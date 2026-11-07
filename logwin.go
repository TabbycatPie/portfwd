package main

// 连接记录窗口：谁在什么时候连进来、连的哪条规则、用了多少流量、持续多久。

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"
)

var (
	hwndLog   uintptr
	logScroll int
	hitsLog   []hitBox
	logClsReg bool
)

const (
	idLogClose = 3100
	idLogClear = 3101
)

func openLog() {
	if hwndLog != 0 {
		proc.ShowWindow.Call(hwndLog, SW_RESTORE)
		proc.SetForegroundWindow.Call(hwndLog)
		return
	}
	if !logClsReg {
		cls := utf16ptr("PortFwdLog")
		wc := WNDCLASSEX{
			CbSize:        uint32(unsafe.Sizeof(WNDCLASSEX{})),
			Style:         0x00020000,
			LpfnWndProc:   syscall.NewCallback(wndLogProc),
			HInstance:     hInst,
			HCursor:       mustCursor(),
			LpszClassName: cls,
		}
		proc.RegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
		logClsReg = true
	}
	hwndLog, _, _ = proc.CreateWindowExW.Call(
		0x00040000, uintptr(unsafe.Pointer(utf16ptr("PortFwdLog"))), uintptr(unsafe.Pointer(utf16ptr("连接记录"))),
		0x80000000, 320, 200, 900, 600,
		hwndMain, 0, hInst, 0)
	roundCorners(hwndLog)
	roundRegion(hwndLog, 14)
	proc.ShowWindow.Call(hwndLog, SW_SHOW)
	forceForeground(hwndLog)
	proc.SetForegroundWindow.Call(hwndLog)
}

func destroyLog() {
	if hwndLog != 0 {
		proc.DestroyWindow.Call(hwndLog)
		hwndLog = 0
	}
	hitSink = &hitsMain
}

func wndLogProc(hwnd, msg, wp, lp uintptr) uintptr {
	switch msg {
	case WM_ERASEBKGND:
		return 1
	case WM_PAINT:
		var ps PAINTSTRUCT
		hdc, _, _ := proc.BeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		var rc RECT
		proc.GetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
		paintLog(hdc, rc)
		proc.EndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		return 0
	case WM_MOUSEMOVE:
		id := hitTest(hitsLog, loword(lp), hiword(lp))
		if id != hoverID {
			hoverID = id
			proc.InvalidateRect.Call(hwnd, 0, 0)
		}
		return 0
	case WM_LBUTTONDOWN:
		x, y := toLogical(loword(lp), hiword(lp))
		switch hitTest(hitsLog, x, y) {
		case idLogClose:
			destroyLog()
			return 0
		case idLogClear:
			clearEvents()
			logScroll = 0
			return 0
		}
		if y < 54 {
			dragWindow(hwndLog)
		}
		return 0
	case WM_MOUSEWHEEL:
		d := int(int16(uint16(wp >> 16)))
		if d < 0 {
			logScroll++
		} else if logScroll > 0 {
			logScroll--
		}
		proc.InvalidateRect.Call(hwnd, 0, 0)
		return 0
	case WM_CLOSE:
		destroyLog()
		return 0
	}
	r, _, _ := proc.DefWindowProcW.Call(hwnd, msg, wp, lp)
	return r
}

func shortDur(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
}

func humanBytes(n int64) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%d B", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	default:
		return fmt.Sprintf("%.1f MB", float64(n)/1024/1024)
	}
}

func paintLog(hdc uintptr, prc RECT) {
	mdc, bmp, old := newGfxDC(hdc, prc)
	rc := logicalRect(prc)
	g := gfx{mdc}
	g.fill(rc, cBg)
	hitsLog = hitsLog[:0]
	hitSink = &hitsLog

	// 标题栏
	g.text("连接记录", RECT{28, 2, rc.Right - 80, 50}, cText, fTitle, DT_LEFT|DT_VCENTER|DT_SINGLELINE)
	closeR := RECT{rc.Right - 58, 11, rc.Right - 18, 43}
	if hoverID == idLogClose {
		g.round(closeR, 8, cBtnHov)
	}
	g.text("×", closeR, cText, fTitle, DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	hitsLog = append(hitsLog, hitBox{id: idLogClose, rect: closeR})

	// 列头
	hy := int32(60)
	cols := []struct {
		x, w int32
		t    string
	}{
		{28, 90, "时间"},
		{120, 200, "规则"},
		{326, 160, "来源"},
		{492, 190, "目标"},
		{688, 100, "流量"},
		{794, 80, "状态"},
	}
	for _, c := range cols {
		g.text(c.t, RECT{c.x, hy, c.x + c.w, hy + 26}, cSub, fSmall, DT_LEFT|DT_VCENTER|DT_SINGLELINE)
	}
	g.fill(RECT{0, 90, rc.Right, 91}, cLine)

	evs := eventsSnapshot()
	maxRows := int((rc.H() - 96 - 72) / 42)
	if maxRows < 1 {
		maxRows = 1
	}
	if logScroll > len(evs)-maxRows {
		logScroll = len(evs) - maxRows
	}
	if logScroll < 0 {
		logScroll = 0
	}

	if len(evs) == 0 {
		g.text("还没有连接记录。有人访问你转发的端口时会出现在这里。",
			RECT{28, 120, rc.Right - 28, 160}, cSub, fBody, DT_LEFT|DT_VCENTER|DT_SINGLELINE)
	}

	top := int32(96)
	for i := 0; i < maxRows && logScroll+i < len(evs); i++ {
		e := evs[logScroll+i]
		r := RECT{0, top + int32(i)*42, rc.Right, top + int32(i)*42 + 40}
		if i%2 == 1 {
			g.fill(r, 0x00241E1B)
		}
		cy := r.Top + 20
		g.text(e.At.Format("15:04:05"), RECT{28, r.Top, 118, r.Top + 40}, cText, fNum, DT_LEFT|DT_VCENTER|DT_SINGLELINE)
		g.text(e.Rule, RECT{120, r.Top, 320, r.Top + 40}, cText, fNum, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
		g.text(e.Client, RECT{326, r.Top, 486, r.Top + 40}, cAccent, fNum, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
		g.text(e.Target, RECT{492, r.Top, 682, r.Top + 40}, cSub, fNum, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)

		state, color := "", cSub
		switch {
		case e.Err != "":
			state, color = "失败", cDanger
		case e.Open:
			state, color = "传输中", cRun
		default:
			state, color = "完成 · "+shortDur(e.Dur), cSub
		}
		g.text(humanBytes(e.Bytes), RECT{688, r.Top, 788, r.Top + 40}, cSub, fNum, DT_LEFT|DT_VCENTER|DT_SINGLELINE)
		g.text(state, RECT{794, r.Top, rc.Right - 24, r.Top + 40}, color, fNum, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
		_ = cy
	}

	// 底部
	fy := rc.Bottom - 52
	g.fill(RECT{0, fy - 12, rc.Right, fy - 11}, cLine)
	g.text(fmt.Sprintf("共 %d 条（滚动查看，最多保留 %d 条）", len(evs), maxEvents),
		RECT{28, fy - 4, rc.Right - 200, fy + 30}, cSub, fSmall, DT_LEFT|DT_VCENTER|DT_SINGLELINE)

	br := RECT{rc.Right - 28 - 88, fy - 6, rc.Right - 28, fy + 32}
	btn(g, br, "清空", idLogClear, cBtn, cDanger)

	blitGfxDC(hdc, prc, mdc, bmp, old)
}
