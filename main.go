package main

// PortFwd —— 极简端口转发托盘工具
// 托盘右键 -> 打开面板；面板内可新增/编辑/启停/删除转发，支持导入导出 JSON。

import (
	_ "embed"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

//go:embed icons/icon.ico
var icoData []byte

var (
	g32b = syscall.NewLazyDLL("gdi32.dll")
	u32b = syscall.NewLazyDLL("user32.dll")

	pCreateCompatibleDC     = g32b.NewProc("CreateCompatibleDC")
	pCreateCompatibleBitmap = g32b.NewProc("CreateCompatibleBitmap")
	pBitBlt                 = g32b.NewProc("BitBlt")
	pDeleteDC               = g32b.NewProc("DeleteDC")
	pSetBkColor             = g32b.NewProc("SetBkColor")
	pGetDC                  = u32b.NewProc("GetDC")
	pDwmSetAttr             = syscall.NewLazyDLL("dwmapi.dll").NewProc("DwmSetWindowAttribute")
	pReleaseCapture         = u32b.NewProc("ReleaseCapture")
	pShellExecute           = syscall.NewLazyDLL("shell32.dll").NewProc("ShellExecuteW")
	pCreateRoundRectRgn     = g32b.NewProc("CreateRoundRectRgn")
	pSetWindowRgn           = u32b.NewProc("SetWindowRgn")
	_                       = pGetDC
)

// 颜色（COLORREF = 0x00BBGGRR）
const (
	cBg      uint32 = 0x001D1816
	cCard    uint32 = 0x0027211E
	cCardHov uint32 = 0x00312B26
	cLine    uint32 = 0x00372F2A
	cText    uint32 = 0x00EFE9E6
	cSub     uint32 = 0x00A1938B
	cAccent  uint32 = 0x00BFD42D
	cRun     uint32 = 0x0099D334
	cIdle    uint32 = 0x0080726B
	cDanger  uint32 = 0x007171F8
	cBtn     uint32 = 0x00332B26
	cBtnHov  uint32 = 0x00413731
	cWhite   uint32 = 0x00FFFFFF
	cBlack   uint32 = 0x00000000
)

const (
	idNew = 1000 + iota
	idImport
	idExport
	idSave
	idCancel
	idMin
	idClose
	idDlgClose
	idLog
)

const (
	baseCard   = 5000
	baseToggle = 6000
	baseEdit   = 7000
	baseDelete = 8000
)

const (
	trayOpen = 2001 + iota
	trayStartAll
	trayStopAll
	trayExit
	trayOpenLog
	trayInstallSvc
	trayUninstallSvc
)

var (
	hInst      uintptr
	hwndMain   uintptr
	hwndDlg    uintptr
	hIconTray  uintptr
	hIconWin   uintptr
	cfgMu      sync.Mutex
	cfg        Config
	eng        = NewEngine()
	fTitle     uintptr
	fH1        uintptr
	fBody      uintptr
	fSmall     uintptr
	fBtn       uintptr
	fNum       uintptr
	scrollTop  int
	hoverID    int
	dlgEditing = -1
	dlgEdits   [4]uintptr
	trayMsg            uint32
	taskbarCreatedMsg  uint32

	hitSink *[]hitBox
	hitsMain []hitBox
	hitsDlg  []hitBox
	editBrush   uintptr
	staticBrush uintptr
)

// ---------- 图标 ----------

type icoImg struct {
	size int
	data []byte
}

func parseICO(b []byte) []icoImg {
	if len(b) < 6 {
		return nil
	}
	n := int(binary.LittleEndian.Uint16(b[4:6]))
	var out []icoImg
	for i := 0; i < n; i++ {
		off := 6 + i*16
		if off+16 > len(b) {
			break
		}
		w := int(b[off])
		if w == 0 {
			w = 256
		}
		sz := int(binary.LittleEndian.Uint32(b[off+8:]))
		pos := int(binary.LittleEndian.Uint32(b[off+12:]))
		if sz <= 0 || pos+sz > len(b) {
			continue
		}
		out = append(out, icoImg{size: w, data: b[pos : pos+sz]})
	}
	return out
}

func makeIcon(imgs []icoImg, want int) uintptr {
	var pick *icoImg
	for i := range imgs {
		if imgs[i].size == want {
			pick = &imgs[i]
			break
		}
	}
	if pick == nil && len(imgs) > 0 {
		pick = &imgs[0]
	}
	if pick == nil {
		return 0
	}
	h, _, _ := gproc.CreateIconFromResEx.Call(
		uintptr(unsafe.Pointer(&pick.data[0])), uintptr(len(pick.data)),
		1, 0x00030000, uintptr(want), uintptr(want), 0)
	return h
}

// ---------- 入口 ----------

var (
	startHidden bool
	svcManaged  bool // 服务在跑，面板只编辑不监听
)

func main() {
	// 关键：Win32 窗口必须在「创建它的那个 OS 线程」上跑消息循环。
	// Go 的 goroutine 默认会在多个 OS 线程之间迁移，一旦迁移，窗口就会变成没人 pump 消息的
	// 僵尸窗口（面板看起来「卡死」，但转发引擎照常工作）。所以先把主 goroutine 钉死在这个线程。
	runtime.LockOSThread()

	for _, a := range os.Args[1:] {
		if a == "--tray" || a == "--startup" {
			startHidden = true
		}
	}
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--check":
			runCheck()
			return
		case "--dump":
			c := loadConfig()
			b, _ := json.MarshalIndent(c, "", "  ")
			_ = os.WriteFile(filepath.Join(os.TempDir(), "portfwd-dump.json"), b, 0644)
			return
		case "--service":
			runService()
			return
		case "--install":
			initLog()
			if err := installService(); err != nil {
				logf("install service FAIL: %v", err)
				msgBox("安装服务失败：\n"+err.Error()+"\n\n请在管理员窗口里执行。", "PortFwd")
				return
			}
			logf("service installed")
			msgBox("已注册为开机自启服务 PortFwd，并已启动。\n\n配置：%ProgramData%\\PortFwd\\config.json\n面板可以继续用来编辑规则，改完自动生效。", "PortFwd")
			return
		case "--uninstall":
			initLog()
			if err := uninstallService(); err != nil {
				logf("uninstall service FAIL: %v", err)
				msgBox("卸载服务失败：\n"+err.Error(), "PortFwd")
				return
			}
			logf("service uninstalled")
			msgBox("服务已停止并删除。", "PortFwd")
			return
		case "--selftest":
			runSelfTest()
			return
		case "--shot-log":
			p := filepath.Join(os.TempDir(), "portfwd-log.bmp")
			if len(os.Args) > 2 {
				p = os.Args[2]
			}
			runShotLog(p)
			return
		case "--shot":
			p := filepath.Join(os.TempDir(), "portfwd-shot.bmp")
			if len(os.Args) > 2 {
				p = os.Args[2]
			}
			runShot(p)
			return
		}
	}

	initLog()

	mx := utf16ptr("PortFwdTrayMutex.v1")
	_, _, e := kproc.CreateMutexW.Call(0, 0, uintptr(unsafe.Pointer(mx)))
	if errno, ok := e.(syscall.Errno); ok && errno == 183 {
		return // 已经在运行
	}

	hInst, _, _ = proc.GetModuleHandleW.Call(0)
	cfg = loadConfig()
	logf("config loaded: %d rules from %s", len(cfg.Items), configPath())
	for _, it := range cfg.Items {
		logf("  rule %q local=:%d -> %s:%d enabled=%v", it.Name, it.LocalPort, it.TargetHost, it.TargetPort, it.Enabled)
	}

	fTitle = newFont(22, true)
	fH1 = newFont(18, true)
	fBody = newFont(16, false)
	fSmall = newFont(15, false)
	fBtn = newFont(15, false)
	fNum = newFont(15, false)

	imgs := parseICO(icoData)
	hIconTray = makeIcon(imgs, 32)
	hIconWin = makeIcon(imgs, 48)

	cls := utf16ptr("PortFwdMain")
	wc := WNDCLASSEX{
		CbSize:        uint32(unsafe.Sizeof(WNDCLASSEX{})),
		Style:         0x00020000, // CS_DROPSHADOW
		LpfnWndProc:   syscall.NewCallback(wndMain),
		HInstance:     hInst,
		HIcon:         hIconWin,
		HCursor:       mustCursor(),
		LpszClassName: cls,
	}
	proc.RegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	scrW, _, _ := proc.GetSystemMetrics.Call(0)
	scrH, _, _ := proc.GetSystemMetrics.Call(1)
	w, h := int32(900), int32(700)
	x := (int32(scrW) - w) / 2
	y := (int32(scrH) - h) / 2

	hwndMain, _, _ = proc.CreateWindowExW.Call(
		0x00040000, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(utf16ptr("端口转发"))),
		0x80000000, // WS_POPUP：不要系统边框，标题栏自己画
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		0, 0, hInst, 0)
	if hwndMain == 0 {
		return
	}
	darkTitle(hwndMain)
	roundCorners(hwndMain)
	roundRegion(hwndMain, 14)
	proc.SendMessageW.Call(hwndMain, WM_SETICON, 1, hIconWin)
	proc.SendMessageW.Call(hwndMain, WM_SETICON, 0, hIconTray)

	tm, _, _ := proc.RegisterWindowMessageW.Call(uintptr(unsafe.Pointer(utf16ptr("PortFwdTrayMsg"))))
	trayMsg = uint32(tm)
	tb, _, _ := proc.RegisterWindowMessageW.Call(uintptr(unsafe.Pointer(utf16ptr("TaskbarCreated"))))
	taskbarCreatedMsg = uint32(tb)
	addTray()

	notifyUI = func() {
		proc.InvalidateRect.Call(hwndMain, 0, 0)
		if hwndLog != 0 {
			proc.InvalidateRect.Call(hwndLog, 0, 0)
		}
	}

	svcManaged = serviceRunning()
	if svcManaged {
		logf("service PortFwd is running -> panel runs in edit-only mode")
	}
	for _, it := range cfg.Items {
		if it.Enabled && !svcManaged {
			_ = eng.Start(it)
		}
	}

	if !startHidden {
		showMain()
	}
	proc.SetTimer.Call(hwndMain, 1, 1000, 0)

	var msg MSG
	for {
		r, _, _ := proc.GetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		proc.TranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		proc.DispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
	delTray()
}

func mustCursor() uintptr {
	c, _, _ := proc.LoadCursorW.Call(0, IDC_ARROW)
	return c
}

func addTray() {
	var nid NOTIFYICONDATA
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	nid.Hwnd = hwndMain
	nid.UID = 1
	nid.UFlags = NIF_MESSAGE | NIF_ICON | NIF_TIP
	nid.UCallbackMessage = trayMsg
	nid.HIcon = hIconTray
	copy(nid.SzTip[:], syscall.StringToUTF16("端口转发"))
	r, _, _ := gproc.ShellNotifyIcon.Call(NIM_ADD, uintptr(unsafe.Pointer(&nid)))
	logf("tray NIM_ADD ok=%d cbSize=%d", r, nid.CbSize)
}

func delTray() {
	var nid NOTIFYICONDATA
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	nid.Hwnd = hwndMain
	nid.UID = 1
	gproc.ShellNotifyIcon.Call(NIM_DELETE, uintptr(unsafe.Pointer(&nid)))
}

func roundCorners(hwnd uintptr) {
	pref := int32(2) // DWMWCP_ROUND
	pDwmSetAttr.Call(hwnd, 33, uintptr(unsafe.Pointer(&pref)), 4)
}

// 给无边框窗口裁出圆角（不依赖 DWM 是否接管）
func roundRegion(hwnd uintptr, radius int32) {
	var rc RECT
	proc.GetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
	if rc.W() <= 0 || rc.H() <= 0 {
		return
	}
	rgn, _, _ := pCreateRoundRectRgn.Call(0, 0, uintptr(rc.W()+1), uintptr(rc.H()+1), uintptr(radius*2), uintptr(radius*2))
	pSetWindowRgn.Call(hwnd, rgn, 1)
}

func dragWindow(hwnd uintptr) {
	pReleaseCapture.Call()
	proc.SendMessageW.Call(hwnd, 0xA1, 2, 0) // WM_NCLBUTTONDOWN / HTCAPTION
}

func darkTitle(hwnd uintptr) {
	on := int32(1)
	pDwmSetAttr.Call(hwnd, 20, uintptr(unsafe.Pointer(&on)), 4) // DWMWA_USE_IMMERSIVE_DARK_MODE
}

func showTrayMenu() {
	hm, _, _ := proc.CreatePopupMenu.Call()
	proc.AppendMenuW.Call(hm, MF_STRING, trayOpen, uintptr(unsafe.Pointer(utf16ptr("打开面板"))))
	proc.AppendMenuW.Call(hm, MF_SEPARATOR, 0, 0)
	proc.AppendMenuW.Call(hm, MF_STRING, trayStartAll, uintptr(unsafe.Pointer(utf16ptr("全部启动"))))
	proc.AppendMenuW.Call(hm, MF_STRING, trayStopAll, uintptr(unsafe.Pointer(utf16ptr("全部停止"))))
	proc.AppendMenuW.Call(hm, MF_SEPARATOR, 0, 0)
	proc.AppendMenuW.Call(hm, MF_STRING, trayOpenLog, uintptr(unsafe.Pointer(utf16ptr("打开日志"))))
	if serviceInstalled() {
		proc.AppendMenuW.Call(hm, MF_STRING, trayUninstallSvc, uintptr(unsafe.Pointer(utf16ptr("卸载后台服务"))))
	} else {
		proc.AppendMenuW.Call(hm, MF_STRING, trayInstallSvc, uintptr(unsafe.Pointer(utf16ptr("安装为后台服务"))))
	}
	proc.AppendMenuW.Call(hm, MF_SEPARATOR, 0, 0)
	proc.AppendMenuW.Call(hm, MF_STRING, trayExit, uintptr(unsafe.Pointer(utf16ptr("退出"))))

	var pt POINT
	proc.GetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	proc.SetForegroundWindow.Call(hwndMain)
	cmd, _, _ := proc.TrackPopupMenu.Call(hm, TPM_RIGHTBUTTON|TPM_RETURNCMD,
		uintptr(pt.X), uintptr(pt.Y), 0, hwndMain, 0)
	proc.PostMessageW.Call(hwndMain, WM_NULL, 0, 0)
	proc.DestroyMenu.Call(hm)

	switch int(cmd) {
	case trayOpen:
		showMain()
	case trayStartAll:
		startAll()
	case trayStopAll:
		stopAll()
	case trayOpenLog:
		pShellExecute.Call(0,
			uintptr(unsafe.Pointer(utf16ptr("open"))),
			uintptr(unsafe.Pointer(utf16ptr(logPathFor()))),
			0, 0, 1)
	case trayInstallSvc:
		if err := installService(); err != nil {
			msgBox("安装服务失败：\n"+err.Error()+"\n\n请以管理员身份运行本程序。", "PortFwd")
		} else {
			svcManaged = true
			eng.StopAll(cfg.Items)
			msgBox("已安装为开机自启服务，转发交由服务执行。\n配置：%ProgramData%\\PortFwd\\config.json", "PortFwd")
		}
		proc.InvalidateRect.Call(hwndMain, 0, 0)
	case trayUninstallSvc:
		if err := uninstallService(); err != nil {
			msgBox("卸载服务失败：\n"+err.Error(), "PortFwd")
		} else {
			svcManaged = false
			msgBox("后台服务已卸载，恢复由面板直接转发。", "PortFwd")
		}
		proc.InvalidateRect.Call(hwndMain, 0, 0)
	case trayExit:
		logf("quit by user")
		delTray()
		proc.DestroyWindow.Call(hwndMain)
	}
}

// forceForeground 把窗口强行提到最前并激活。
// 直接调 SetForegroundWindow 会被 Windows 的前台锁挡住（进程不在前台时调用无效），
// 所以先 AttachThreadInput 到当前前台窗口的线程，再置前，最后用一次 TOPMOST 兜底。
func forceForeground(hwnd uintptr) {
	fg, _, _ := proc.GetForegroundWindow.Call()
	tidFg, _, _ := proc.GetWindowThreadProcessId.Call(fg, 0)
	tidMe, _, _ := kproc.GetCurrentThreadId.Call()

	attached := false
	if fg != 0 && tidFg != 0 && tidFg != tidMe {
		if ok, _, _ := proc.AttachThreadInput.Call(tidMe, tidFg, 1); ok != 0 {
			attached = true
		}
	}
	proc.BringWindowToTop.Call(hwnd)
	proc.SetForegroundWindow.Call(hwnd)
	proc.SetFocus.Call(hwnd)
	if attached {
		proc.AttachThreadInput.Call(tidMe, tidFg, 0)
	}

	// 兜底：短暂置顶再撤掉，保证一定浮到最上层
	hwndTopMost := ^uintptr(0)   // -1
	hwndNoTopMost := ^uintptr(1) // -2
	proc.SetWindowPos.Call(hwnd, hwndTopMost, 0, 0, 0, 0, SWP_NOMOVE|SWP_NOSIZE|SWP_NOACTIVATE)
	proc.SetWindowPos.Call(hwnd, hwndNoTopMost, 0, 0, 0, 0, SWP_NOMOVE|SWP_NOSIZE|SWP_NOACTIVATE)

	logf("forceForeground hwnd=%d fg=%d attached=%v nowFg=%d", hwnd, fg, attached, mustForeground())
}

func mustForeground() uintptr {
	h, _, _ := proc.GetForegroundWindow.Call()
	return h
}

func showMain() {
	vis, _, _ := proc.IsWindowVisible.Call(hwndMain)
	if vis == 0 {
		proc.ShowWindow.Call(hwndMain, SW_SHOW)
	} else {
		proc.ShowWindow.Call(hwndMain, SW_RESTORE)
	}
	roundRegion(hwndMain, 14)
	forceForeground(hwndMain)
	proc.InvalidateRect.Call(hwndMain, 0, 1)
	logf("panel shown")
}

func wndMain(hwnd, msg, wp, lp uintptr) uintptr {
	switch msg {
	case WM_ERASEBKGND:
		return 1
	case WM_PAINT:
		var ps PAINTSTRUCT
		hdc, _, _ := proc.BeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		var rc RECT
		proc.GetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
		paintMain(hdc, rc)
		proc.EndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		return 0
	case WM_LBUTTONDOWN:
		onClick(loword(lp), hiword(lp))
		return 0
	case WM_MOUSEMOVE:
		id := hitTest(hitsMain, loword(lp), hiword(lp))
		if id != hoverID {
			hoverID = id
			proc.InvalidateRect.Call(hwnd, 0, 0)
		}
		return 0
	case WM_MOUSEWHEEL:
		delta := int(int16(uint16(wp >> 16)))
		if delta < 0 {
			scrollTop++
		} else if scrollTop > 0 {
			scrollTop--
		}
		proc.InvalidateRect.Call(hwnd, 0, 0)
		return 0
	case WM_SIZE:
		roundRegion(hwnd, 14)
		return 0
	case WM_TIMER:
		proc.InvalidateRect.Call(hwnd, 0, 0)
		if hwndLog != 0 {
			proc.InvalidateRect.Call(hwndLog, 0, 0)
		}
		return 0
	case WM_CLOSE:
		proc.ShowWindow.Call(hwnd, SW_HIDE)
		logf("panel hidden to tray")
		return 0
	case WM_DESTROY:
		eng.StopAll(cfg.Items)
		proc.KillTimer.Call(hwnd, 1)
		proc.PostQuitMessage.Call(0)
		return 0
	}
	if taskbarCreatedMsg != 0 && msg == uintptr(taskbarCreatedMsg) {
		logf("explorer restarted -> re-add tray icon")
		addTray()
		return 0
	}
	if trayMsg != 0 && msg == uintptr(trayMsg) {
		logf("tray event lp=%#x", lp)
		switch uint32(lp) {
		case WM_LBUTTONUP, WM_LBUTTONDBLCLK:
			showMain()
		case WM_RBUTTONUP:
			showTrayMenu()
		}
		return 0
	}
	r, _, _ := proc.DefWindowProcW.Call(hwnd, msg, wp, lp)
	return r
}

// ---------- 绘制 ----------

func visibleRows(h int32) int32 {
	n := (h - 132 - 64) / 90
	if n < 1 {
		n = 1
	}
	return n
}

type hitBox struct {
	id   int
	rect RECT
}

func paintMain(hdc uintptr, rc RECT) {
	mdc, _, _ := pCreateCompatibleDC.Call(hdc)
	bmp, _, _ := pCreateCompatibleBitmap.Call(hdc, uintptr(rc.W()), uintptr(rc.H()))
	old, _, _ := proc.SelectObject.Call(mdc, bmp)

	g := gfx{mdc}
	g.fill(rc, cBg)
	hitsMain = hitsMain[:0]
	hitSink = &hitsMain

	pad := int32(28)
	const titleH = int32(54)

	// —— 自绘标题栏 ——
	g.text("端口转发", RECT{pad, 2, rc.Right - 160, titleH - 8}, cText, fTitle, DT_LEFT|DT_VCENTER|DT_SINGLELINE)
	closeR := RECT{rc.Right - 58, 11, rc.Right - 18, 43}
	minR := RECT{rc.Right - 106, 11, rc.Right - 66, 43}
	if hoverID == idClose {
		g.round(closeR, 8, 0x00322B3B)
	} else if hoverID == idMin {
		g.round(minR, 8, cBtnHov)
	}
	g.text("×", closeR, cText, fTitle, DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	g.text("—", minR, cSub, fBtn, DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	hitsMain = append(hitsMain, hitBox{id: idClose, rect: closeR}, hitBox{id: idMin, rect: minR})

	// —— 工具行 ——
	g.text("本地端口 → 目标服务 · 右键托盘图标可再次打开",
		RECT{pad, titleH - 4, rc.Right - 470, titleH + 32}, cSub, fSmall, DT_LEFT|DT_VCENTER|DT_SINGLELINE)

	bw, bh := int32(96), int32(38)
	bx := rc.Right - pad - bw
	by := titleH + 1
	btn(g, RECT{bx, by, bx + bw, by + bh}, "＋ 新增", idNew, cAccent, cBlack)
	bx -= bw + 10
	btn(g, RECT{bx, by, bx + bw, by + bh}, "导入", idImport, cBtn, cText)
	bx -= bw + 10
	btn(g, RECT{bx, by, bx + bw, by + bh}, "导出", idExport, cBtn, cText)
	bx -= bw + 10
	btn(g, RECT{bx, by, bx + bw, by + bh}, "日志", idLog, cBtn, cText)

	g.fill(RECT{0, 116, rc.Right, 117}, cLine)

	cfgMu.Lock()
	items := append([]Item(nil), cfg.Items...)
	cfgMu.Unlock()

	maxRows := int(visibleRows(rc.H()))
	if scrollTop > len(items)-maxRows {
		scrollTop = len(items) - maxRows
	}
	if scrollTop < 0 {
		scrollTop = 0
	}

	top := int32(132)
	pitch := int32(90)
	rowH := int32(78)

	if len(items) == 0 {
		g.text("还没有转发规则，点右上角「＋ 新增」加一条",
			RECT{pad, top + 24, rc.Right - pad, top + 64}, cSub, fBody, DT_LEFT|DT_VCENTER|DT_SINGLELINE)
	}

	for i := 0; i < maxRows && scrollTop+i < len(items); i++ {
		it := items[scrollTop+i]
		r := RECT{pad, top + int32(i)*pitch, rc.Right - pad, top + int32(i)*pitch + rowH}
		running := eng.Running(it.ID)
		cardBg := cCard
		if hoverID == baseCard+scrollTop+i {
			cardBg = cCardHov
		}
		g.round(r, 14, cardBg)
		hitsMain = append(hitsMain, hitBox{id: baseCard + scrollTop + i, rect: r})

		name := it.Name
		if name == "" {
			name = "未命名"
		}
		g.text(name, RECT{r.Left + 22, r.Top + 10, r.Right - 390, r.Top + 42}, cText, fH1,
			DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
		mapText := it.Target() + "   →   本机 " + strconv.Itoa(it.LocalPort)
		g.text(mapText, RECT{r.Left + 22, r.Top + 42, r.Right - 390, r.Top + 70}, cSub, fNum,
			DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)

		dotC := cIdle
		statusText := "已停止"
		if svcManaged && it.Enabled {
			dotC = cRun
			statusText = "服务托管"
		} else if running {
			dotC = cRun
			statusText = "运行中"
			if n := eng.Conns(it.ID); n > 0 {
				statusText += fmt.Sprintf(" · %d 连接", n)
			}
		} else if e := eng.Err(it.ID); e != "" {
			dotC = cDanger
			statusText = "失败"
		}
		g.circle(r.Right-330, r.Top+38, 5, dotC)
		g.text(statusText, RECT{r.Right - 316, r.Top + 24, r.Right - 200, r.Top + 52}, dotC, fSmall,
			DT_LEFT|DT_VCENTER|DT_SINGLELINE)

		btH := int32(36)
		btY := r.Top + (rowH-btH)/2
		delR := RECT{r.Right - 22 - 60, btY, r.Right - 22, btY + btH}
		editR := RECT{delR.Left - 8 - 60, btY, delR.Left - 8, btY + btH}
		togR := RECT{editR.Left - 10 - 90, btY, editR.Left - 10, btY + btH}

		if running {
			btn(g, togR, "停止", baseToggle+scrollTop+i, cBtn, cSub)
		} else {
			btn(g, togR, "启动", baseToggle+scrollTop+i, cAccent, cBlack)
		}
		btn(g, editR, "编辑", baseEdit+scrollTop+i, cBtn, cText)
		btn(g, delR, "删除", baseDelete+scrollTop+i, cBtn, cDanger)
	}

	runCount := 0
	for _, it := range items {
		if eng.Running(it.ID) {
			runCount++
		}
	}
	fy := rc.Bottom - 44
	g.fill(RECT{0, fy - 12, rc.Right, fy - 11}, cLine)
	g.text(fmt.Sprintf("共 %d 条 · 运行中 %d 条", len(items), runCount),
		RECT{pad, fy - 4, rc.Right - pad, fy + 26}, cSub, fSmall, DT_LEFT|DT_VCENTER|DT_SINGLELINE)
	if len(items) > maxRows {
		g.text(fmt.Sprintf("滚轮翻页 %d/%d", scrollTop+1, len(items)-maxRows+1),
			RECT{pad, fy - 4, rc.Right - pad, fy + 26}, cSub, fSmall, DT_RIGHT|DT_VCENTER|DT_SINGLELINE)
	}
	for _, it := range items {
		if !eng.Running(it.ID) {
			if e := eng.Err(it.ID); e != "" {
				g.text("⚠ "+e, RECT{pad + 180, fy - 4, rc.Right - pad, fy + 26}, cDanger, fSmall,
					DT_RIGHT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
				break
			}
		}
	}

	pBitBlt.Call(hdc, 0, 0, uintptr(rc.W()), uintptr(rc.H()), mdc, 0, 0, 0x00CC0020)
	proc.SelectObject.Call(mdc, old)
	pDeleteDC.Call(mdc)
	proc.DeleteObject.Call(bmp)
}

func btn(g gfx, r RECT, label string, id int, bg, fg uint32) {
	if hoverID == id {
		bg = cBtnHov
	}
	g.round(r, 8, bg)
	g.text(label, r, fg, fBtn, DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	if hitSink != nil {
		*hitSink = append(*hitSink, hitBox{id: id, rect: r})
	}
}

func hitTest(boxes []hitBox, x, y int32) int {
	for _, h := range boxes {
		if x >= h.rect.Left && x < h.rect.Right && y >= h.rect.Top && y < h.rect.Bottom {
			return h.id
		}
	}
	return 0
}

func onClick(x, y int32) {
	id := hitTest(hitsMain, x, y)
	switch {
	case id == 0:
		if y < 54 { // 标题栏空白处拖动窗口
			dragWindow(hwndMain)
		}
		return
	case id == idMin:
		proc.ShowWindow.Call(hwndMain, SW_MINIMIZE)
		return
	case id == idClose:
		proc.ShowWindow.Call(hwndMain, SW_HIDE)
		return
	case id == idNew:
		openDlg(-1)
	case id == idImport:
		doImport()
	case id == idExport:
		doExport()
	case id == idLog:
		openLog()
	case id >= baseDelete && id < baseDelete+1000:
		deleteItem(id - baseDelete)
	case id >= baseEdit && id < baseEdit+1000:
		openDlg(id - baseEdit)
	case id >= baseToggle && id < baseToggle+1000:
		toggleItem(id - baseToggle)
	}
}

func toggleItem(idx int) {
	cfgMu.Lock()
	if idx < 0 || idx >= len(cfg.Items) {
		cfgMu.Unlock()
		return
	}
	it := &cfg.Items[idx]
	if eng.Running(it.ID) {
		eng.Stop(it.ID)
		it.Enabled = false
	} else {
		_ = eng.Start(*it)
		it.Enabled = true
	}
	_ = saveConfig(&cfg)
	cfgMu.Unlock()
	proc.InvalidateRect.Call(hwndMain, 0, 0)
}

func deleteItem(idx int) {
	cfgMu.Lock()
	if idx < 0 || idx >= len(cfg.Items) {
		cfgMu.Unlock()
		return
	}
	eng.Stop(cfg.Items[idx].ID)
	cfg.Items = append(cfg.Items[:idx], cfg.Items[idx+1:]...)
	_ = saveConfig(&cfg)
	cfgMu.Unlock()
	proc.InvalidateRect.Call(hwndMain, 0, 0)
}

func startAll() {
	cfgMu.Lock()
	for i := range cfg.Items {
		cfg.Items[i].Enabled = true
		_ = eng.Start(cfg.Items[i])
	}
	_ = saveConfig(&cfg)
	cfgMu.Unlock()
	proc.InvalidateRect.Call(hwndMain, 0, 0)
}

func stopAll() {
	cfgMu.Lock()
	for i := range cfg.Items {
		cfg.Items[i].Enabled = false
		eng.Stop(cfg.Items[i].ID)
	}
	_ = saveConfig(&cfg)
	cfgMu.Unlock()
	proc.InvalidateRect.Call(hwndMain, 0, 0)
}

// ---------- 导入导出 ----------

func doExport() {
	path, ok := fileDialog(true, "导出转发配置", "portfwd-config.json",
		"JSON 配置 (*.json)\x00*.json\x00所有文件 (*.*)\x00*.*\x00\x00")
	if !ok {
		return
	}
	cfgMu.Lock()
	b, _ := json.MarshalIndent(cfg, "", "  ")
	cfgMu.Unlock()
	if err := os.WriteFile(path, b, 0644); err != nil {
		msgBox("导出失败："+err.Error(), "端口转发")
		return
	}
	msgBox("已导出到\n"+path, "端口转发")
}

func doImport() {
	path, ok := fileDialog(false, "导入转发配置", "",
		"JSON 配置 (*.json)\x00*.json\x00所有文件 (*.*)\x00*.*\x00\x00")
	if !ok {
		return
	}
	b, err := os.ReadFile(path)
	if err != nil {
		msgBox("读取失败："+err.Error(), "端口转发")
		return
	}
	var nc Config
	if err := json.Unmarshal(stripBOM(b), &nc); err != nil {
		msgBox("不是有效的配置文件：\n"+err.Error(), "端口转发")
		return
	}
	eng.StopAll(cfg.Items)
	cfgMu.Lock()
	cfg = nc
	for i := range cfg.Items {
		if cfg.Items[i].ID == "" {
			cfg.Items[i].ID = newID()
		}
	}
	_ = saveConfig(&cfg)
	cfgMu.Unlock()
	for _, it := range cfg.Items {
		if it.Enabled {
			_ = eng.Start(it)
		}
	}
	scrollTop = 0
	proc.InvalidateRect.Call(hwndMain, 0, 0)
}

func fileDialog(save bool, title, defName, filter string) (string, bool) {
	buf := make([]uint16, 1024)
	if defName != "" {
		copy(buf, syscall.StringToUTF16(defName))
	}
	f := syscall.StringToUTF16(filter)
	ofn := OPENFILENAME{
		LStructSize: uint32(unsafe.Sizeof(OPENFILENAME{})),
		HwndOwner:   hwndMain,
		LpstrFilter: &f[0],
		LpstrFile:   &buf[0],
		NMaxFile:    uint32(len(buf)),
		LpstrTitle:  utf16ptr(title),
		Flags:       OFN_OVERWRITEPROMPT | OFN_PATHMUSTEXIST | OFN_HIDEREADONLY,
	}
	var ok uintptr
	if save {
		ofn.LpstrDefExt = utf16ptr("json")
		ok, _, _ = gproc.GetSaveFileNameW.Call(uintptr(unsafe.Pointer(&ofn)))
	} else {
		ofn.Flags |= OFN_FILEMUSTEXIST
		ok, _, _ = gproc.GetOpenFileNameW.Call(uintptr(unsafe.Pointer(&ofn)))
	}
	if ok == 0 {
		return "", false
	}
	return syscall.UTF16ToString(buf), true
}

func msgBox(text, title string) {
	proc.MessageBoxW.Call(hwndMain,
		uintptr(unsafe.Pointer(utf16ptr(text))),
		uintptr(unsafe.Pointer(utf16ptr(title))), 0x40)
}

// ---------- 新增/编辑 ----------

var dlgClsRegistered bool

func openDlg(idx int) {
	if idx >= 0 && idx < len(cfg.Items) {
		dlgEditing = idx
	} else {
		dlgEditing = -1
	}
	if !dlgClsRegistered {
		cls := utf16ptr("PortFwdDlg")
		wc := WNDCLASSEX{
			CbSize:        uint32(unsafe.Sizeof(WNDCLASSEX{})),
			Style:         0x00020000,
			LpfnWndProc:   syscall.NewCallback(wndDlgProc),
			HInstance:     hInst,
			HCursor:       mustCursor(),
			LpszClassName: cls,
		}
		proc.RegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
		dlgClsRegistered = true
	}
	title := "新增转发"
	if dlgEditing >= 0 {
		title = "编辑转发"
	}
	x, y := int32(300), int32(180)
	w, h := int32(560), int32(452)
	hwndDlg, _, _ = proc.CreateWindowExW.Call(
		0x00040000, uintptr(unsafe.Pointer(utf16ptr("PortFwdDlg"))), uintptr(unsafe.Pointer(utf16ptr(title))),
		0x80000000, uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		hwndMain, 0, hInst, 0)
	roundCorners(hwndDlg)
	roundRegion(hwndDlg, 14)
	proc.ShowWindow.Call(hwndDlg, SW_SHOW)
	proc.SetForegroundWindow.Call(hwndDlg)
}

func wndDlgProc(hwnd, msg, wp, lp uintptr) uintptr {
	switch msg {
	case WM_CREATE:
		buildDlg(hwnd)
		return 0
	case WM_ERASEBKGND:
		return 1
	case WM_CTLCOLOREDIT:
		pSetBkColor.Call(wp, uintptr(cCard))
		proc.SetTextColor.Call(wp, uintptr(cText))
		if editBrush == 0 {
			editBrush, _, _ = proc.CreateSolidBrush.Call(uintptr(cCard))
		}
		return editBrush
	case WM_CTLCOLORSTATIC:
		pSetBkColor.Call(wp, uintptr(cBg))
		proc.SetTextColor.Call(wp, uintptr(cSub))
		if staticBrush == 0 {
			staticBrush, _, _ = proc.CreateSolidBrush.Call(uintptr(cBg))
		}
		return staticBrush
	case WM_PAINT:
		var ps PAINTSTRUCT
		hdc, _, _ := proc.BeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		var rc RECT
		proc.GetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
		paintDlg(hdc, rc)
		proc.EndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		return 0
	case WM_LBUTTONDOWN:
		dlgClick(loword(lp), hiword(lp))
		return 0
	case WM_MOUSEMOVE:
		id := hitTest(hitsDlg, loword(lp), hiword(lp))
		if id != hoverID {
			hoverID = id
			proc.InvalidateRect.Call(hwnd, 0, 0)
		}
		return 0
	case WM_CLOSE:
		destroyDlg()
		return 0
	}
	r, _, _ := proc.DefWindowProcW.Call(hwnd, msg, wp, lp)
	return r
}

func buildDlg(hwnd uintptr) {
	var it Item
	if dlgEditing >= 0 && dlgEditing < len(cfg.Items) {
		it = cfg.Items[dlgEditing]
	}
	mkLabel := func(text string, y int32) {
		h, _, _ := proc.CreateWindowExW.Call(
			0, uintptr(unsafe.Pointer(utf16ptr("STATIC"))), uintptr(unsafe.Pointer(utf16ptr(text))),
			0x50000000, 26, uintptr(y), 508, 20, hwnd, 0, hInst, 0)
		proc.SendMessageW.Call(h, 0x0030, fSmall, 0)
	}
	mkEdit := func(y int32, text string) uintptr {
		e, _, _ := proc.CreateWindowExW.Call(
			0, uintptr(unsafe.Pointer(utf16ptr("EDIT"))), uintptr(unsafe.Pointer(utf16ptr(text))),
			0x50000000|0x0080, 26, uintptr(y), 508, 34, hwnd, 0, hInst, 0)
		proc.SendMessageW.Call(e, 0x0030, fBody, 0)
		return e
	}
	mkLabel("名称（留空则用 地址:端口）", 66)
	dlgEdits[0] = mkEdit(88, it.Name)
	mkLabel("目标地址（IP 或域名）", 136)
	dlgEdits[1] = mkEdit(158, it.TargetHost)
	mkLabel("目标端口", 206)
	dlgEdits[2] = mkEdit(228, portText(it.TargetPort))
	mkLabel("本地端口（对外提供服务的端口）", 276)
	dlgEdits[3] = mkEdit(298, portText(it.LocalPort))
	proc.SetFocus.Call(dlgEdits[1])
}

func portText(p int) string {
	if p == 0 {
		return ""
	}
	return strconv.Itoa(p)
}

func paintDlg(hdc uintptr, rc RECT) {
	mdc, _, _ := pCreateCompatibleDC.Call(hdc)
	bmp, _, _ := pCreateCompatibleBitmap.Call(hdc, uintptr(rc.W()), uintptr(rc.H()))
	old, _, _ := proc.SelectObject.Call(mdc, bmp)
	g := gfx{mdc}
	g.fill(rc, cBg)
	hitsDlg = hitsDlg[:0]
	hitSink = &hitsDlg

	title := "新增转发"
	if dlgEditing >= 0 {
		title = "编辑转发"
	}
	g.text(title, RECT{26, 2, rc.Right - 70, 50}, cText, fTitle, DT_LEFT|DT_VCENTER|DT_SINGLELINE)
	closeR := RECT{rc.Right - 54, 10, rc.Right - 18, 42}
	if hoverID == idDlgClose {
		g.round(closeR, 8, cBtnHov)
	}
	g.text("×", closeR, cText, fTitle, DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	hitsDlg = append(hitsDlg, hitBox{id: idDlgClose, rect: closeR})

	bw, bh := int32(104), int32(38)
	by := rc.Bottom - 56
	cancel := RECT{rc.Right - 26 - bw, by, rc.Right - 26, by + bh}
	save := RECT{cancel.Left - 12 - bw, by, cancel.Left - 12, by + bh}
	btn(g, save, "保存", idSave, cAccent, cBlack)
	btn(g, cancel, "取消", idCancel, cBtn, cText)
	g.text("保存后自动启动", RECT{26, by, save.Left - 12, by + bh}, cSub, fSmall,
		DT_LEFT|DT_VCENTER|DT_SINGLELINE)

	pBitBlt.Call(hdc, 0, 0, uintptr(rc.W()), uintptr(rc.H()), mdc, 0, 0, 0x00CC0020)
	proc.SelectObject.Call(mdc, old)
	pDeleteDC.Call(mdc)
	proc.DeleteObject.Call(bmp)
}

func dlgClick(x, y int32) {
	switch hitTest(hitsDlg, x, y) {
	case idSave:
		saveDlg()
	case idCancel, idDlgClose:
		destroyDlg()
	default:
		if y < 50 {
			dragWindow(hwndDlg)
		}
	}
}

func editText(h uintptr) string {
	n, _, _ := proc.GetWindowTextLengthW.Call(h)
	buf := make([]uint16, n+2)
	proc.GetWindowTextW.Call(h, uintptr(unsafe.Pointer(&buf[0])), n+1)
	return strings.TrimSpace(syscall.UTF16ToString(buf))
}

func saveDlg() {
	name := editText(dlgEdits[0])
	host := editText(dlgEdits[1])
	tp, e1 := strconv.Atoi(editText(dlgEdits[2]))
	lp, e2 := strconv.Atoi(editText(dlgEdits[3]))
	if host == "" {
		msgBox("目标地址不能为空", "提示")
		return
	}
	if e1 != nil || tp < 1 || tp > 65535 {
		msgBox("目标端口必须是 1-65535", "提示")
		return
	}
	if e2 != nil || lp < 1 || lp > 65535 {
		msgBox("本地端口必须是 1-65535", "提示")
		return
	}
	cfgMu.Lock()
	for i, it := range cfg.Items {
		if it.LocalPort == lp && i != dlgEditing {
			n := it.Name
			cfgMu.Unlock()
			msgBox(fmt.Sprintf("本地端口 %d 已被「%s」占用", lp, n), "提示")
			return
		}
	}
	if name == "" {
		name = host + ":" + strconv.Itoa(tp)
	}
	if dlgEditing >= 0 && dlgEditing < len(cfg.Items) {
		oldID := cfg.Items[dlgEditing].ID
		eng.Stop(oldID)
		cfg.Items[dlgEditing] = Item{ID: oldID, Name: name, TargetHost: host, TargetPort: tp, LocalPort: lp, Enabled: true}
		_ = eng.Start(cfg.Items[dlgEditing])
	} else {
		it := Item{ID: newID(), Name: name, TargetHost: host, TargetPort: tp, LocalPort: lp, Enabled: true}
		cfg.Items = append(cfg.Items, it)
		_ = eng.Start(it)
	}
	_ = saveConfig(&cfg)
	cfgMu.Unlock()
	destroyDlg()
	proc.InvalidateRect.Call(hwndMain, 0, 0)
}

func destroyDlg() {
	if hwndDlg != 0 {
		proc.DestroyWindow.Call(hwndDlg)
		hwndDlg = 0
	}
	hitSink = &hitsMain
}

// ---------- 自检（无界面） ----------

func runCheck() {
	c := loadConfig()
	sb := &strings.Builder{}
	fmt.Fprintf(sb, "config=%s items=%d\n", configPath(), len(c.Items))
	for _, it := range c.Items {
		if err := eng.Start(it); err != nil {
			fmt.Fprintf(sb, "[fail] %s listen :%d err=%v\n", it.Name, it.LocalPort, err)
			continue
		}
		fmt.Fprintf(sb, "[ok] %s :%d -> %s\n", it.Name, it.LocalPort, it.Target())
	}
	eng.StopAll(c.Items)
	_ = os.WriteFile(filepath.Join(os.TempDir(), "portfwd-check.txt"), []byte(sb.String()), 0644)
}
