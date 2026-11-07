// trayping：给 PortFwd 的隐藏窗口发一条托盘消息，用来无人工验证托盘点击链路。
package main

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"
)

var (
	u32 = syscall.NewLazyDLL("user32.dll")
	k32 = syscall.NewLazyDLL("kernel32.dll")

	pRegisterWindowMessageW = u32.NewProc("RegisterWindowMessageW")
	pFindWindowW            = u32.NewProc("FindWindowW")
	pPostMessageW           = u32.NewProc("PostMessageW")
	pIsWindowVisible        = u32.NewProc("IsWindowVisible")
	pGetModuleHandleW       = k32.NewProc("GetModuleHandleW")
	_                       = pGetModuleHandleW
)

func p(s string) *uint16 {
	q, _ := syscall.UTF16PtrFromString(s)
	return q
}

func main() {
	msg, _, _ := pRegisterWindowMessageW.Call(uintptr(unsafe.Pointer(p("PortFwdTrayMsg"))))
	fmt.Printf("trayMsg=%#x\n", msg)

	hwnd, _, _ := pFindWindowW.Call(uintptr(unsafe.Pointer(p("PortFwdMain"))), 0)
	if hwnd == 0 {
		fmt.Println("window not found (PortFwdMain)")
		return
	}
	vis, _, _ := pIsWindowVisible.Call(hwnd)
	fmt.Printf("hwnd=%d visible(before)=%v\n", hwnd, vis != 0)

	// 0x0203 = WM_LBUTTONDBLCLK, 0x0202 = WM_LBUTTONUP
	for _, click := range []uintptr{0x0203, 0x0202} {
		r, _, _ := pPostMessageW.Call(hwnd, msg, 1, click)
		fmt.Printf("post lp=%#x -> %v\n", click, r != 0)
	}
	// 给窗口一点时间处理
	time.Sleep(600 * time.Millisecond)
	vis2, _, _ := pIsWindowVisible.Call(hwnd)
	fmt.Printf("visible(after)=%v\n", vis2 != 0)
}
