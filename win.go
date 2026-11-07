package main

// Win32 薄封装：只用 user32/gdi32/shell32/comdlg32/kernel32，不引入第三方依赖。

import (
	"syscall"
	"unsafe"
)

var (
	u32  = syscall.NewLazyDLL("user32.dll")
	g32  = syscall.NewLazyDLL("gdi32.dll")
	s32  = syscall.NewLazyDLL("shell32.dll")
	c32  = syscall.NewLazyDLL("comdlg32.dll")
	k32  = syscall.NewLazyDLL("kernel32.dll")
	uxt  = syscall.NewLazyDLL("uxtheme.dll")
	_    = uxt
	proc = struct {
		RegisterClassExW, CreateWindowExW, DefWindowProcW, DestroyWindow                    *syscall.LazyProc
		GetMessageW, TranslateMessage, DispatchMessageW, PostQuitMessage                    *syscall.LazyProc
		PostMessageW, ShowWindow, UpdateWindow, InvalidateRect, BeginPaint, EndPaint        *syscall.LazyProc
		GetClientRect, SendMessageW, SetWindowTextW, GetWindowTextW, GetWindowTextLengthW    *syscall.LazyProc
		LoadCursorW, CreatePopupMenu, AppendMenuW, TrackPopupMenu, DestroyMenu              *syscall.LazyProc
		SetForegroundWindow, GetCursorPos, SetTimer, KillTimer, GetSystemMetrics            *syscall.LazyProc
		MessageBoxW, SetFocus, GetFocus, IsWindowVisible, MoveWindow, EnableWindow          *syscall.LazyProc
		RegisterWindowMessageW, SetWindowLongPtrW, GetWindowLongPtrW, GetDC, ReleaseDC      *syscall.LazyProc
		DrawTextW, SetBkMode, SetTextColor, CreateSolidBrush, CreatePen, SelectObject       *syscall.LazyProc
		DeleteObject, CreateFontW, Rectangle, Ellipse, RoundRect, GetStockObject            *syscall.LazyProc
		SetTextAlign, SetWindowPos, FillRect, GetModuleHandleW, ShowCursor                  *syscall.LazyProc
		ReleaseCapture, GetForegroundWindow, AttachThreadInput, BringWindowToTop            *syscall.LazyProc
		SetProcessDpiAwarenessContext, GetDpiForSystem, GetDpiForWindow, SetProcessDPIAware  *syscall.LazyProc
		SetMapMode, SetWindowExtEx, SetViewportExtEx                                       *syscall.LazyProc
		GetWindowThreadProcessId                                                            *syscall.LazyProc
	}{}
	gproc = struct {
		ShellNotifyIcon     *syscall.LazyProc
		CreateIconFromResEx *syscall.LazyProc
		DestroyIcon         *syscall.LazyProc
		GetOpenFileNameW    *syscall.LazyProc
		GetSaveFileNameW    *syscall.LazyProc
	}{}
	kproc = struct {
		CreateMutexW, GetLastError, GetCurrentThreadId *syscall.LazyProc
	}{}
)

func init() {
	proc.RegisterClassExW = u32.NewProc("RegisterClassExW")
	proc.CreateWindowExW = u32.NewProc("CreateWindowExW")
	proc.DefWindowProcW = u32.NewProc("DefWindowProcW")
	proc.DestroyWindow = u32.NewProc("DestroyWindow")
	proc.GetMessageW = u32.NewProc("GetMessageW")
	proc.TranslateMessage = u32.NewProc("TranslateMessage")
	proc.DispatchMessageW = u32.NewProc("DispatchMessageW")
	proc.PostQuitMessage = u32.NewProc("PostQuitMessage")
	proc.PostMessageW = u32.NewProc("PostMessageW")
	proc.ShowWindow = u32.NewProc("ShowWindow")
	proc.UpdateWindow = u32.NewProc("UpdateWindow")
	proc.InvalidateRect = u32.NewProc("InvalidateRect")
	proc.BeginPaint = u32.NewProc("BeginPaint")
	proc.EndPaint = u32.NewProc("EndPaint")
	proc.GetClientRect = u32.NewProc("GetClientRect")
	proc.SendMessageW = u32.NewProc("SendMessageW")
	proc.SetWindowTextW = u32.NewProc("SetWindowTextW")
	proc.GetWindowTextW = u32.NewProc("GetWindowTextW")
	proc.GetWindowTextLengthW = u32.NewProc("GetWindowTextLengthW")
	proc.LoadCursorW = u32.NewProc("LoadCursorW")
	proc.CreatePopupMenu = u32.NewProc("CreatePopupMenu")
	proc.AppendMenuW = u32.NewProc("AppendMenuW")
	proc.TrackPopupMenu = u32.NewProc("TrackPopupMenu")
	proc.DestroyMenu = u32.NewProc("DestroyMenu")
	proc.SetForegroundWindow = u32.NewProc("SetForegroundWindow")
	proc.GetCursorPos = u32.NewProc("GetCursorPos")
	proc.SetTimer = u32.NewProc("SetTimer")
	proc.KillTimer = u32.NewProc("KillTimer")
	proc.GetSystemMetrics = u32.NewProc("GetSystemMetrics")
	proc.MessageBoxW = u32.NewProc("MessageBoxW")
	proc.SetFocus = u32.NewProc("SetFocus")
	proc.GetFocus = u32.NewProc("GetFocus")
	proc.IsWindowVisible = u32.NewProc("IsWindowVisible")
	proc.MoveWindow = u32.NewProc("MoveWindow")
	proc.EnableWindow = u32.NewProc("EnableWindow")
	proc.RegisterWindowMessageW = u32.NewProc("RegisterWindowMessageW")
	proc.SetWindowLongPtrW = u32.NewProc("SetWindowLongPtrW")
	proc.GetWindowLongPtrW = u32.NewProc("GetWindowLongPtrW")
	proc.GetDC = u32.NewProc("GetDC")
	proc.ReleaseDC = u32.NewProc("ReleaseDC")
	proc.DrawTextW = u32.NewProc("DrawTextW")
	proc.SetBkMode = g32.NewProc("SetBkMode")
	proc.SetTextColor = g32.NewProc("SetTextColor")
	proc.CreateSolidBrush = g32.NewProc("CreateSolidBrush")
	proc.CreatePen = g32.NewProc("CreatePen")
	proc.SelectObject = g32.NewProc("SelectObject")
	proc.DeleteObject = g32.NewProc("DeleteObject")
	proc.CreateFontW = g32.NewProc("CreateFontW")
	proc.Rectangle = g32.NewProc("Rectangle")
	proc.Ellipse = g32.NewProc("Ellipse")
	proc.RoundRect = g32.NewProc("RoundRect")
	proc.GetStockObject = g32.NewProc("GetStockObject")
	proc.SetTextAlign = g32.NewProc("SetTextAlign")
	proc.SetWindowPos = u32.NewProc("SetWindowPos")
	proc.FillRect = u32.NewProc("FillRect")
	proc.GetModuleHandleW = k32.NewProc("GetModuleHandleW")
	proc.ShowCursor = u32.NewProc("ShowCursor")
	proc.ReleaseCapture = u32.NewProc("ReleaseCapture")
	proc.GetForegroundWindow = u32.NewProc("GetForegroundWindow")
	proc.AttachThreadInput = u32.NewProc("AttachThreadInput")
	proc.BringWindowToTop = u32.NewProc("BringWindowToTop")
	proc.GetWindowThreadProcessId = u32.NewProc("GetWindowThreadProcessId")
	proc.SetProcessDpiAwarenessContext = u32.NewProc("SetProcessDpiAwarenessContext")
	proc.GetDpiForSystem = u32.NewProc("GetDpiForSystem")
	proc.GetDpiForWindow = u32.NewProc("GetDpiForWindow")
	proc.SetProcessDPIAware = u32.NewProc("SetProcessDPIAware")
	proc.SetMapMode = g32.NewProc("SetMapMode")
	proc.SetWindowExtEx = g32.NewProc("SetWindowExtEx")
	proc.SetViewportExtEx = g32.NewProc("SetViewportExtEx")

	gproc.ShellNotifyIcon = s32.NewProc("Shell_NotifyIconW")
	gproc.CreateIconFromResEx = u32.NewProc("CreateIconFromResourceEx")
	gproc.DestroyIcon = u32.NewProc("DestroyIcon")
	gproc.GetOpenFileNameW = c32.NewProc("GetOpenFileNameW")
	gproc.GetSaveFileNameW = c32.NewProc("GetSaveFileNameW")

	kproc.CreateMutexW = k32.NewProc("CreateMutexW")
	kproc.GetLastError = k32.NewProc("GetLastError")
	kproc.GetCurrentThreadId = k32.NewProc("GetCurrentThreadId")
}

const (
	WM_CREATE        = 0x0001
	WM_DESTROY       = 0x0002
	WM_SIZE          = 0x0005
	WM_PAINT         = 0x000F
	WM_CLOSE         = 0x0010
	WM_ERASEBKGND    = 0x0014
	WM_SETICON       = 0x0080
	WM_KEYDOWN       = 0x0100
	WM_COMMAND       = 0x0111
	WM_TIMER         = 0x0113
	WM_CTLCOLOREDIT  = 0x0133
	WM_CTLCOLORSTATIC = 0x0138
	WM_MOUSEMOVE     = 0x0200
	WM_LBUTTONDOWN   = 0x0201
	WM_LBUTTONUP     = 0x0202
	WM_LBUTTONDBLCLK = 0x0203
	WM_RBUTTONUP     = 0x0205
	WM_MOUSEWHEEL    = 0x020A
	WM_APP           = 0x8000
	WM_NULL          = 0x0000

	SW_HIDE    = 0
	SW_SHOW    = 5
	SW_MINIMIZE = 6
	SWP_NOSIZE     = 0x0001
	SWP_NOMOVE     = 0x0002
	SWP_NOACTIVATE = 0x0010
	MM_TEXT        = 1
	MM_ANISOTROPIC = 8
	SW_RESTORE  = 9

	NIM_ADD    = 0
	NIM_MODIFY = 1
	NIM_DELETE = 2

	NIF_MESSAGE = 0x1
	NIF_ICON    = 0x2
	NIF_TIP     = 0x4

	MF_STRING    = 0x0
	MF_SEPARATOR = 0x800
	MF_POPUP     = 0x10
	MF_CHECKED   = 0x8
	MF_GRAYED    = 0x1
	TPM_RIGHTBUTTON = 0x2
	TPM_RETURNCMD   = 0x100

	GWLP_USERDATA = -21

	DT_LEFT     = 0x0
	DT_CENTER   = 0x1
	DT_RIGHT    = 0x2
	DT_VCENTER  = 0x4
	DT_SINGLELINE = 0x20
	DT_END_ELLIPSIS = 0x8000
	DT_NOPREFIX = 0x800

	TRANSPARENT = 1
	PS_SOLID    = 0
	PS_NULL     = 5
	NULL_BRUSH  = 5
	NULL_PEN    = 8

	IDC_ARROW = 32512

	OFN_OVERWRITEPROMPT = 0x2
	OFN_FILEMUSTEXIST   = 0x1000
	OFN_PATHMUSTEXIST   = 0x800
	OFN_HIDEREADONLY    = 0x4

	VK_ESCAPE = 0x1B
	VK_RETURN = 0x0D
)

type POINT struct{ X, Y int32 }
type RECT struct{ Left, Top, Right, Bottom int32 }
type MSG struct {
	Hwnd    uintptr
	Message uint32
	_       uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      POINT
	_       uint32
}
type PAINTSTRUCT struct {
	Hdc        uintptr
	Erase      int32
	RcPaint    RECT
	Restore    int32
	IncUpdate  int32
	Reserved   [32]byte
}
type WNDCLASSEX struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     uintptr
	HIcon         uintptr
	HCursor       uintptr
	HbrBackground uintptr
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       uintptr
}
type NOTIFYICONDATA struct {
	CbSize           uint32
	_                uint32
	Hwnd             uintptr
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	_                uint32
	HIcon            uintptr
	SzTip            [128]uint16
	DwState          uint32
	DwStateMask      uint32
	SzInfo           [256]uint16
	UVersion         uint32
	SzInfoTitle      [64]uint16
	DwInfoFlags      uint32
	GuidItem         [16]byte
	HBalloonIcon     uintptr
}
type OPENFILENAME struct {
	LStructSize       uint32
	_                 uint32
	HwndOwner         uintptr
	HInstance         uintptr
	LpstrFilter       *uint16
	LpstrCustomFilter *uint16
	NMaxCustFilter    uint32
	NFilterIndex      uint32
	LpstrFile         *uint16
	NMaxFile          uint32
	LpstrFileTitle    *uint16
	NMaxFileTitle     uint32
	LpstrInitialDir   *uint16
	LpstrTitle        *uint16
	Flags             uint32
	NFileOffset       uint16
	NFileExtension    uint16
	LpstrDefExt       *uint16
	LCustData         uintptr
	LpfnHook          uintptr
	LpTemplateName    *uint16
	PvReserved        uintptr
	DwReserved        uint32
	FlagsEx           uint32
}

func utf16ptr(s string) *uint16 {
	p, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		return nil
	}
	return p
}

func rgb(r, g, b byte) uint32 { return uint32(r) | uint32(g)<<8 | uint32(b)<<16 }

func loword(v uintptr) int32  { return int32(v & 0xFFFF) }
func hiword(v uintptr) int32  { return int32((v >> 16) & 0xFFFF) }

func (r RECT) W() int32 { return r.Right - r.Left }
func (r RECT) H() int32 { return r.Bottom - r.Top }

// 画笔/画刷/字体缓存的简单封装
type gfx struct {
	hdc uintptr
}

func (g gfx) brush(c uint32) uintptr {
	b, _, _ := proc.CreateSolidBrush.Call(uintptr(c))
	return b
}
func (g gfx) fill(r RECT, c uint32) {
	b := g.brush(c)
	proc.FillRect.Call(g.hdc, uintptr(unsafe.Pointer(&r)), b)
	proc.DeleteObject.Call(b)
}
func (g gfx) round(r RECT, radius int32, c uint32) {
	b := g.brush(c)
	p, _, _ := proc.CreatePen.Call(PS_NULL, 0, 0)
	ob, _, _ := proc.SelectObject.Call(g.hdc, b)
	op, _, _ := proc.SelectObject.Call(g.hdc, p)
	proc.RoundRect.Call(g.hdc, uintptr(r.Left), uintptr(r.Top), uintptr(r.Right), uintptr(r.Bottom), uintptr(radius*2), uintptr(radius*2))
	proc.SelectObject.Call(g.hdc, ob)
	proc.SelectObject.Call(g.hdc, op)
	proc.DeleteObject.Call(b)
	proc.DeleteObject.Call(p)
}
func (g gfx) circle(cx, cy, rr int32, c uint32) {
	b := g.brush(c)
	p, _, _ := proc.CreatePen.Call(PS_NULL, 0, 0)
	ob, _, _ := proc.SelectObject.Call(g.hdc, b)
	op, _, _ := proc.SelectObject.Call(g.hdc, p)
	proc.Ellipse.Call(g.hdc, uintptr(cx-rr), uintptr(cy-rr), uintptr(cx+rr), uintptr(cy+rr))
	proc.SelectObject.Call(g.hdc, ob)
	proc.SelectObject.Call(g.hdc, op)
	proc.DeleteObject.Call(b)
	proc.DeleteObject.Call(p)
}
func (g gfx) text(s string, r RECT, color uint32, font uintptr, flags uint32) {
	p, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		return
	}
	of, _, _ := proc.SelectObject.Call(g.hdc, font)
	proc.SetBkMode.Call(g.hdc, TRANSPARENT)
	proc.SetTextColor.Call(g.hdc, uintptr(color))
	proc.DrawTextW.Call(g.hdc, uintptr(unsafe.Pointer(p)), ^uintptr(0), uintptr(unsafe.Pointer(&r)), uintptr(flags|DT_NOPREFIX))
	proc.SelectObject.Call(g.hdc, of)
}

func newFont(size int32, bold bool) uintptr {
	weight := int32(400)
	if bold {
		weight = 600
	}
	face := utf16ptr("Microsoft YaHei UI")
	f, _, _ := proc.CreateFontW.Call(
		uintptr(size), 0, 0, 0, uintptr(weight), 0, 0, 0, 1 /*DEFAULT_CHARSET*/,
		0, 0, 4 /*ANTIALIASED_QUALITY 灰度更干净*/, 0, uintptr(unsafe.Pointer(face)),
	)
	return f
}
