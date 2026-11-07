package main

// Windows 服务模式：把转发跑在服务里（开机自启、不依赖登录会话），面板只负责编辑配置。
//   PortFwd.exe --service      由 SCM 调用
//   PortFwd.exe --install      注册为自动启动服务并运行（需管理员）
//   PortFwd.exe --uninstall    停止并删除服务（需管理员）

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var (
	adv = syscall.NewLazyDLL("advapi32.dll")

	pStartServiceCtrlDispatcherW = adv.NewProc("StartServiceCtrlDispatcherW")
	pRegisterServiceCtrlHandlerExW = adv.NewProc("RegisterServiceCtrlHandlerExW")
	pSetServiceStatus            = adv.NewProc("SetServiceStatus")
	pOpenSCManagerW              = adv.NewProc("OpenSCManagerW")
	pOpenServiceW                = adv.NewProc("OpenServiceW")
	pQueryServiceStatus          = adv.NewProc("QueryServiceStatus")
	pCreateServiceW              = adv.NewProc("CreateServiceW")
	pDeleteService               = adv.NewProc("DeleteService")
	pCloseServiceHandle          = adv.NewProc("CloseServiceHandle")
	pStartServiceW               = adv.NewProc("StartServiceW")
	pControlService              = adv.NewProc("ControlService")
)

const (
	svcName = "PortFwd"

	SERVICE_WIN32_OWN_PROCESS = 0x10
	SERVICE_ACCEPT_STOP       = 0x1
	SERVICE_ACCEPT_SHUTDOWN   = 0x4

	SERVICE_STOPPED        = 1
	SERVICE_START_PENDING  = 2
	SERVICE_STOP_PENDING   = 3
	SERVICE_RUNNING        = 4
	SERVICE_CONTROL_STOP   = 1
	SERVICE_CONTROL_SHUTDOWN = 5

	SC_MANAGER_CONNECT        = 0x0001
	SC_MANAGER_CREATE_SERVICE = 0x0002
	SERVICE_QUERY_STATUS      = 0x0004
	SERVICE_START             = 0x0010
	SERVICE_STOP              = 0x0020
	SERVICE_ALL_ACCESS        = 0xF01FF
	SERVICE_AUTO_START        = 0x00000002
	SERVICE_ERROR_NORMAL      = 0x00000001
)

type SERVICE_STATUS struct {
	DwServiceType             uint32
	DwCurrentState            uint32
	DwControlsAccepted        uint32
	DwWin32ExitCode           uint32
	DwServiceSpecificExitCode uint32
	DwCheckPoint              uint32
	DwWaitHint                uint32
}

type SERVICE_TABLE_ENTRY struct {
	LpServiceName *uint16
	LpServiceProc uintptr
}

var (
	svcStatusHandle uintptr
	svcStopCh       = make(chan struct{})
	svcStopOnce     sync.Once
	svcStateMu      sync.Mutex
	svcState        SERVICE_STATUS
)

// ---------- 服务主流程 ----------

func runService() {
	display := utf16ptr(svcName)
	table := []SERVICE_TABLE_ENTRY{
		{LpServiceName: display, LpServiceProc: syscall.NewCallback(serviceMain)},
		{LpServiceName: nil, LpServiceProc: 0},
	}
	pStartServiceCtrlDispatcherW.Call(uintptr(unsafe.Pointer(&table[0])))
}

func serviceMain(argc uintptr, argv uintptr) uintptr {
	svcStatusHandle, _, _ = pRegisterServiceCtrlHandlerExW.Call(
		uintptr(unsafe.Pointer(utf16ptr(svcName))),
		syscall.NewCallback(serviceCtrlHandler), 0)
	if svcStatusHandle == 0 {
		return 0
	}
	reportStatus(SERVICE_START_PENDING, 3000)

	// 服务也写日志，路径在 ProgramData 那边
	initServiceLog()
	logf("==== service start v%s", appVer)

	cfg = loadConfig()
	logf("service config: %d rules from %s", len(cfg.Items), configPath())
	applyAll()

	reportStatus(SERVICE_RUNNING, 0)

	// 监视配置文件变化，自动重载
	last := fileModTime(configPath())
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-svcStopCh:
			eng.StopAll(cfg.Items)
			logf("service stopped")
			reportStatus(SERVICE_STOPPED, 0)
			return 0
		case <-tick.C:
			if m := fileModTime(configPath()); m != last {
				last = m
				logf("config changed, reloading")
				eng.StopAll(cfg.Items)
				cfg = loadConfig()
				applyAll()
			}
		}
	}
}

func applyAll() {
	for _, it := range cfg.Items {
		if it.Enabled {
			_ = eng.Start(it)
		}
	}
}

func reportStatus(state uint32, waitHint uint32) {
	svcStateMu.Lock()
	svcState.DwServiceType = SERVICE_WIN32_OWN_PROCESS
	svcState.DwCurrentState = state
	svcState.DwControlsAccepted = 0
	if state == SERVICE_RUNNING {
		svcState.DwControlsAccepted = SERVICE_ACCEPT_STOP | SERVICE_ACCEPT_SHUTDOWN
	}
	svcState.DwWin32ExitCode = 0
	svcState.DwWaitHint = waitHint
	svcStateMu.Unlock()
	pSetServiceStatus.Call(svcStatusHandle, uintptr(unsafe.Pointer(&svcState)))
}

func serviceCtrlHandler(ctrl uintptr, eventType uintptr, eventData uintptr, ctx uintptr) uintptr {
	switch ctrl {
	case SERVICE_CONTROL_STOP, SERVICE_CONTROL_SHUTDOWN:
		reportStatus(SERVICE_STOP_PENDING, 3000)
		svcStopOnce.Do(func() { close(svcStopCh) })
	}
	return 0
}

// ---------- 安装 / 卸载 / 查询 ----------

func serviceExePath() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return exe
}

func installService() error {
	scm, _, err := pOpenSCManagerW.Call(0, 0, SC_MANAGER_CREATE_SERVICE|SC_MANAGER_CONNECT)
	if scm == 0 {
		return fmt.Errorf("%s", T("svc.err.open", err))
	}
	defer pCloseServiceHandle.Call(scm)

	cmd := fmt.Sprintf(`"%s" --service`, serviceExePath())
	h, _, e := pCreateServiceW.Call(scm,
		uintptr(unsafe.Pointer(utf16ptr(svcName))),
		uintptr(unsafe.Pointer(utf16ptr(T("svc.display")))),
		SERVICE_ALL_ACCESS, SERVICE_WIN32_OWN_PROCESS, SERVICE_AUTO_START, SERVICE_ERROR_NORMAL,
		uintptr(unsafe.Pointer(utf16ptr(cmd))),
		0, 0, 0, 0, 0)
	if h == 0 {
		return fmt.Errorf("%s", T("svc.err.create", e))
	}
	defer pCloseServiceHandle.Call(h)
	pStartServiceW.Call(h, 0, 0)
	return nil
}

func uninstallService() error {
	scm, _, err := pOpenSCManagerW.Call(0, 0, SC_MANAGER_CONNECT)
	if scm == 0 {
		return fmt.Errorf("%s", T("svc.err.open", err))
	}
	defer pCloseServiceHandle.Call(scm)
	h, _, e := pOpenServiceW.Call(scm, uintptr(unsafe.Pointer(utf16ptr(svcName))), SERVICE_STOP|SERVICE_QUERY_STATUS|0x10000)
	if h == 0 {
		return fmt.Errorf("%s", T("svc.err.notfound", e))
	}
	defer pCloseServiceHandle.Call(h)
	var st SERVICE_STATUS
	pControlService.Call(h, SERVICE_CONTROL_STOP, uintptr(unsafe.Pointer(&st)))
	time.Sleep(800 * time.Millisecond)
	if r, _, e2 := pDeleteService.Call(h); r == 0 {
		return fmt.Errorf("%s", T("svc.err.delete", e2))
	}
	return nil
}

// serviceState 返回 SCM 里的状态；0 表示未安装
func serviceState() uint32 {
	scm, _, _ := pOpenSCManagerW.Call(0, 0, SC_MANAGER_CONNECT)
	if scm == 0 {
		return 0
	}
	defer pCloseServiceHandle.Call(scm)
	h, _, _ := pOpenServiceW.Call(scm, uintptr(unsafe.Pointer(utf16ptr(svcName))), SERVICE_QUERY_STATUS)
	if h == 0 {
		return 0
	}
	defer pCloseServiceHandle.Call(h)
	var st SERVICE_STATUS
	if r, _, _ := pQueryServiceStatus.Call(h, uintptr(unsafe.Pointer(&st))); r == 0 {
		return 0
	}
	return st.DwCurrentState
}

func serviceInstalled() bool { return serviceState() != 0 }
func serviceRunning() bool   { return serviceState() == SERVICE_RUNNING }

// ---------- 服务侧路径 ----------

var programDataDir = func() string {
	return filepath.Join(os.Getenv("ProgramData"), "PortFwd")
}

func initServiceLog() {
	_ = os.MkdirAll(programDataDir(), 0755)
	logFile = filepath.Join(programDataDir(), "portfwd.log")
	rotateLog()
}

func fileModTime(p string) int64 {
	st, err := os.Stat(p)
	if err != nil {
		return 0
	}
	return st.ModTime().UnixNano()
}
