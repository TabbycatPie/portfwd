package main

// 高分屏适配：所有布局都按「逻辑坐标」写（980×780 这套），
// 真正绘制时在离屏 DC 上设各向异性映射，把逻辑坐标放大到物理像素。
// 这样字号、间距、圆角随 DPI 一起变，字不会再被系统拉伸成糊的。
//
// 鼠标坐标是物理像素，命中测试前要 toLogical 换回逻辑坐标。

import "math"

var (
	dpiScale  float64 = 1.0 // 窗口所在显示器的 DPI / 96
	userScale float64 = 1.0 // 用户自己选的界面缩放
)

func uiTotal() float64 {
	s := dpiScale * userScale
	if s <= 0.4 {
		s = 0.4
	}
	if s > 3 {
		s = 3
	}
	return s
}

// iscale 逻辑尺寸 -> 物理像素（用于窗口/控件这些真实尺寸）
func iscale(v float64) int32 {
	return int32(math.Round(v * uiTotal()))
}

// logicalRect 物理客户区 -> 逻辑矩形，布局代码全部用这个
func logicalRect(prc RECT) RECT {
	s := uiTotal()
	return RECT{0, 0,
		int32(math.Round(float64(prc.W()) / s)),
		int32(math.Round(float64(prc.H()) / s))}
}

// toLogical 鼠标物理坐标 -> 逻辑坐标
func toLogical(x, y int32) (int32, int32) {
	s := uiTotal()
	return int32(math.Round(float64(x) / s)), int32(math.Round(float64(y) / s))
}

// newGfxDC 建离屏 DC 并设置映射模式（逻辑 -> 物理）
func newGfxDC(hdc uintptr, prc RECT) (mdc, bmp, old uintptr) {
	mdc, _, _ = pCreateCompatibleDC.Call(hdc)
	bmp, _, _ = pCreateCompatibleBitmap.Call(hdc, uintptr(prc.W()), uintptr(prc.H()))
	old, _, _ = proc.SelectObject.Call(mdc, bmp)
	if s := uiTotal(); s != 1 {
		lw := int32(math.Round(float64(prc.W()) / s))
		lh := int32(math.Round(float64(prc.H()) / s))
		proc.SetMapMode.Call(mdc, MM_ANISOTROPIC)
		proc.SetWindowExtEx.Call(mdc, uintptr(lw), uintptr(lh), 0)
		proc.SetViewportExtEx.Call(mdc, uintptr(prc.W()), uintptr(prc.H()), 0)
	}
	return
}

// blitGfxDC 把离屏内容按物理像素拷到窗口并释放资源
func blitGfxDC(hdc uintptr, prc RECT, mdc, bmp, old uintptr) {
	proc.SetMapMode.Call(mdc, MM_TEXT)
	pBitBlt.Call(hdc, 0, 0, uintptr(prc.W()), uintptr(prc.H()), mdc, 0, 0, 0x00CC0020)
	proc.SelectObject.Call(mdc, old)
	pDeleteDC.Call(mdc)
	proc.DeleteObject.Call(bmp)
}

// initDPI 进程级声明：Per-Monitor V2（Win10 1703+），老系统退回 SetProcessDPIAware
func initDPI() {
	ok, _, _ := proc.SetProcessDpiAwarenessContext.Call(^uintptr(3)) // DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 = -4
	if ok == 0 {
		proc.SetProcessDPIAware.Call()
	}
	d, _, _ := proc.GetDpiForSystem.Call()
	if d == 0 {
		d = 96
	}
	dpiScale = float64(d) / 96.0
}

// refreshDPI 窗口换到别的显示器后重新取 DPI
func refreshDPI(hwnd uintptr) {
	if hwnd == 0 {
		return
	}
	if d, _, _ := proc.GetDpiForWindow.Call(hwnd); d > 0 {
		dpiScale = float64(d) / 96.0
	}
}
