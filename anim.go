package main

// 悬停过渡动画：每个可交互元素一个 0→1 的进度，由 UI 线程上的 15ms 定时器推进。
// 只在有元素正在过渡时才跑定时器，静止时零开销。

import (
	"math"
	"unsafe"
)

var animVals = map[int]float64{}

func animOf(id int) float64 { return animVals[id] }

// animTick 推进一帧，返回是否还有动画在跑
func animTick() bool {
	busy := false
	for id, v := range animVals {
		var tgt float64
		if id == hoverID {
			tgt = 1
		}
		nv := v + (tgt-v)*0.22
		if math.Abs(nv-tgt) < 0.02 {
			nv = tgt
		} else {
			busy = true
		}
		if nv == 0 && tgt == 0 {
			delete(animVals, id)
		} else {
			animVals[id] = nv
		}
	}
	if hoverID != 0 {
		if _, ok := animVals[hoverID]; !ok {
			animVals[hoverID] = 0.08
			busy = true
		}
	}
	return busy
}

// lerpColor 在两个 COLORREF(0x00BBGGRR) 之间插值
func lerpColor(a, b uint32, t float64) uint32 {
	if t <= 0 {
		return a
	}
	if t >= 1 {
		return b
	}
	ar, ag, ab := float64(a&0xFF), float64((a>>8)&0xFF), float64((a>>16)&0xFF)
	br, bg, bb := float64(b&0xFF), float64((b>>8)&0xFF), float64((b>>16)&0xFF)
	r := uint32(ar + (br-ar)*t + 0.5)
	g := uint32(ag + (bg-ag)*t + 0.5)
	bl := uint32(ab + (bb-ab)*t + 0.5)
	return r | g<<8 | bl<<16
}

// hoverTint 计算某个底色的悬停色：往白色提亮一点（绿按钮同样适用）
func hoverTint(bg uint32) uint32 {
	return lerpColor(bg, 0x00FFFFFF, 0.16)
}

// hoverBg 返回某元素当前应使用的底色（带过渡）
func hoverBg(id int, normal, hover uint32) uint32 {
	t := animOf(id)
	if t <= 0 {
		return normal
	}
	return lerpColor(normal, hover, t)
}

// startHoverAnim 启动动画帧定时器
func startHoverAnim(hwnd uintptr) {
	proc.SetTimer.Call(hwnd, 2, 15, 0)
}

// trackMouseLeave 让窗口在鼠标离开时收到 WM_MOUSELEAVE，避免悬停状态卡住
func trackMouseLeave(hwnd uintptr) {
	t := TRACKMOUSEEVENT{
		CbSize:    uint32(unsafe.Sizeof(TRACKMOUSEEVENT{})),
		DwFlags:   TME_LEAVE,
		HwndTrack: hwnd,
	}
	proc.TrackMouseEvent.Call(uintptr(unsafe.Pointer(&t)))
}
