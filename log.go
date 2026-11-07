package main

// 常驻日志：release 版也写，出问题能查。
// 位置 %APPDATA%\PortFwd\portfwd.log；单文件超 2 MB 自动滚成 portfwd.log.1。

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	logMaxSize = 2 << 20 // 2 MB
	logKeep    = 1       // 保留一个历史文件
)

var (
	logMu   sync.Mutex
	logFile string
	appVer  = "1.0.2"
)

func initLog() {
	dir, err := os.UserConfigDir()
	if err != nil || dir == "" {
		dir = os.Getenv("APPDATA")
	}
	base := filepath.Join(dir, "PortFwd")
	_ = os.MkdirAll(base, 0755)
	logFile = filepath.Join(base, "portfwd.log")
	rotateLog()
	logf("==== start PortFwd v%s (pid %d) args=%v", appVer, os.Getpid(), os.Args[1:])
}

func logf(format string, a ...any) {
	if logFile == "" {
		return
	}
	logMu.Lock()
	defer logMu.Unlock()
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = fmt.Fprintf(f, "%s %s\n", time.Now().Format("2006-01-02 15:04:05.000"), fmt.Sprintf(format, a...))
}

func rotateLog() {
	st, err := os.Stat(logFile)
	if err != nil || st.Size() < logMaxSize {
		return
	}
	bak := logFile + ".1"
	_ = os.Remove(bak)
	_ = os.Rename(logFile, bak)
}

// logPathFor 供 UI 打开日志用
func logPathFor() string { return logFile }
