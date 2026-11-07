package main

// --shot <文件.bmp>：把面板渲染成图片，用于无人工介入的界面验收。
// 直接把 paintMain 画到内存 DC，不经窗口（避免 PrintWindow 同线程死锁）。

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unsafe"
)

var pGetDIBits = g32b.NewProc("GetDIBits")

func runShot(path string) {
	hInst, _, _ = proc.GetModuleHandleW.Call(0)

	fTitle = newFont(22, true)
	fH1 = newFont(18, true)
	fBody = newFont(16, false)
	fSmall = newFont(15, false)
	fBtn = newFont(15, false)
	fNum = newFont(15, false)

	// 演示数据（不写盘；端口用高位避免与线上转发冲突）
	cfg = Config{Items: []Item{
		{ID: "s1", Name: "内网管理台", TargetHost: "10.0.0.10", TargetPort: 8080, LocalPort: 8080, Enabled: true},
		{ID: "s2", Name: "监控面板", TargetHost: "10.0.0.20", TargetPort: 3000, LocalPort: 3000, Enabled: true},
		{ID: "s3", Name: "远程主机 SSH", TargetHost: "10.0.0.30", TargetPort: 22, LocalPort: 2222, Enabled: false},
	}}
	eng = NewEngine()
	_ = eng.Start(cfg.Items[0])
	_ = eng.Start(cfg.Items[1])
	scrollTop = 0
	hoverID = baseToggle

	rc := RECT{0, 0, 900, 700}
	screenDC, _, _ := proc.GetDC.Call(0)
	mdc, _, _ := pCreateCompatibleDC.Call(screenDC)
	bmp, _, _ := pCreateCompatibleBitmap.Call(screenDC, uintptr(rc.W()), uintptr(rc.H()))
	old, _, _ := proc.SelectObject.Call(mdc, bmp)

	paintMain(mdc, rc)
	_ = fmt.Sprint

	proc.SelectObject.Call(mdc, old) // GetDIBits 要求位图未选入 DC

	type bmiHeader struct {
		BiSize          uint32
		BiWidth         int32
		BiHeight        int32
		BiPlanes        uint16
		BiBitCount      uint16
		BiCompression   uint32
		BiSizeImage     uint32
		BiXPelsPerMeter int32
		BiYPelsPerMeter int32
		BiClrUsed       uint32
		BiClrImportant  uint32
	}
	var bi bmiHeader
	bi.BiSize = uint32(unsafe.Sizeof(bi))
	bi.BiWidth = rc.W()
	bi.BiHeight = -rc.H()
	bi.BiPlanes = 1
	bi.BiBitCount = 32
	buf := make([]byte, int(rc.W())*int(rc.H())*4)
	lines, _, _ := pGetDIBits.Call(mdc, bmp, 0, uintptr(rc.H()),
		uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&bi)), 0)
	if lines != 0 {
		if f, err := os.Create(path); err == nil {
			hdr := make([]byte, 54)
			hdr[0], hdr[1] = 'B', 'M'
			binary.LittleEndian.PutUint32(hdr[2:], uint32(54+len(buf)))
			binary.LittleEndian.PutUint32(hdr[10:], 54)
			binary.LittleEndian.PutUint32(hdr[14:], 40)
			binary.LittleEndian.PutUint32(hdr[18:], uint32(rc.W()))
			binary.LittleEndian.PutUint32(hdr[22:], uint32(rc.H()))
			binary.LittleEndian.PutUint16(hdr[26:], 1)
			binary.LittleEndian.PutUint16(hdr[28:], 32)
			binary.LittleEndian.PutUint32(hdr[34:], uint32(len(buf)))
			_, _ = f.Write(hdr)
			rowBytes := int(rc.W()) * 4
			for y := int(rc.H()) - 1; y >= 0; y-- { // 翻成 bottom-up 写
				_, _ = f.Write(buf[y*rowBytes : (y+1)*rowBytes])
			}
			_ = f.Close()
		}
	}

	pDeleteDC.Call(mdc)
	proc.DeleteObject.Call(bmp)
	proc.ReleaseDC.Call(0, screenDC)
	eng.StopAll(cfg.Items)
}

// ---------- 自检：转发 + 连接记录 ----------

func runSelfTest() {
	cfg = Config{Items: []Item{{ID: "t1", Name: "自测", TargetHost: "10.0.0.10", TargetPort: 8080, LocalPort: 18999, Enabled: true}}}
	eng = NewEngine()
	err := eng.Start(cfg.Items[0])
	c, derr := net.Dial("tcp", "127.0.0.1:18999")
	if derr == nil {
		_, _ = c.Write([]byte("GET / HTTP/1.0\r\n\r\n"))
		time.Sleep(400 * time.Millisecond)
		_ = c.Close()
	}
	time.Sleep(400 * time.Millisecond)
	sb := &strings.Builder{}
	fmt.Fprintf(sb, "listen_err=%v dial_err=%v\n", err, derr)
	for _, e := range eventsSnapshot() {
		fmt.Fprintf(sb, "ev rule=%s client=%s target=%s bytes=%d open=%v err=%q dur=%v\n",
			e.Rule, e.Client, e.Target, e.Bytes, e.Open, e.Err, e.Dur.Round(time.Millisecond))
	}
	_ = os.WriteFile(filepath.Join(os.TempDir(), "portfwd-selftest.txt"), []byte(sb.String()), 0644)
	eng.StopAll(cfg.Items)
}

// ---------- 连接记录窗口截图 ----------

func runShotLog(path string) {
	hInst, _, _ = proc.GetModuleHandleW.Call(0)
	fTitle = newFont(22, true)
	fH1 = newFont(18, true)
	fBody = newFont(16, false)
	fSmall = newFont(15, false)
	fBtn = newFont(15, false)
	fNum = newFont(15, false)

	now := time.Now()
	demo := []ConnEvent{
		{At: now.Add(-95 * time.Second), Rule: "内网管理台", Client: "192.168.1.23:52114", Target: "10.0.0.10:8080", Bytes: 1843200, Dur: 62 * time.Second},
		{At: now.Add(-84 * time.Second), Rule: "监控面板", Client: "192.168.1.23:52120", Target: "10.0.0.20:3000", Bytes: 96453, Dur: 21 * time.Second},
		{At: now.Add(-40 * time.Second), Rule: "内网管理台", Client: "192.168.1.56:60122", Target: "10.0.0.10:8080", Bytes: 524288, Dur: 12 * time.Second},
		{At: now.Add(-9 * time.Second), Rule: "远程主机 SSH", Client: "192.168.1.23:52199", Target: "10.0.0.30:22", Bytes: 0, Dur: 3 * time.Second, Err: "目标不可达: dial tcp: i/o timeout"},
		{At: now.Add(-2 * time.Second), Rule: "内网管理台", Client: "192.168.1.88:53001", Target: "10.0.0.10:8080", Open: true},
	}
	evMu.Lock()
	for i := range demo {
		e := demo[i]
		events = append(events, &e)
	}
	evMu.Unlock()

	rc := RECT{0, 0, 900, 600}
	screenDC, _, _ := proc.GetDC.Call(0)
	mdc, _, _ := pCreateCompatibleDC.Call(screenDC)
	bmp, _, _ := pCreateCompatibleBitmap.Call(screenDC, uintptr(rc.W()), uintptr(rc.H()))
	old, _, _ := proc.SelectObject.Call(mdc, bmp)
	paintLog(mdc, rc)
	proc.SelectObject.Call(mdc, old)
	saveBMP(path, rc, mdc, bmp)
	pDeleteDC.Call(mdc)
	proc.DeleteObject.Call(bmp)
	proc.ReleaseDC.Call(0, screenDC)
}

func saveBMP(path string, rc RECT, mdc, bmp uintptr) {
	type bmiHeader struct {
		BiSize          uint32
		BiWidth         int32
		BiHeight        int32
		BiPlanes        uint16
		BiBitCount      uint16
		BiCompression   uint32
		BiSizeImage     uint32
		BiXPelsPerMeter int32
		BiYPelsPerMeter int32
		BiClrUsed       uint32
		BiClrImportant  uint32
	}
	var bi bmiHeader
	bi.BiSize = uint32(unsafe.Sizeof(bi))
	bi.BiWidth = rc.W()
	bi.BiHeight = -rc.H()
	bi.BiPlanes = 1
	bi.BiBitCount = 32
	buf := make([]byte, int(rc.W())*int(rc.H())*4)
	lines, _, _ := pGetDIBits.Call(mdc, bmp, 0, uintptr(rc.H()),
		uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&bi)), 0)
	if lines == 0 {
		return
	}
	f, err := os.Create(path)
	if err != nil {
		return
	}
	defer f.Close()
	hdr := make([]byte, 54)
	hdr[0], hdr[1] = 'B', 'M'
	binary.LittleEndian.PutUint32(hdr[2:], uint32(54+len(buf)))
	binary.LittleEndian.PutUint32(hdr[10:], 54)
	binary.LittleEndian.PutUint32(hdr[14:], 40)
	binary.LittleEndian.PutUint32(hdr[18:], uint32(rc.W()))
	binary.LittleEndian.PutUint32(hdr[22:], uint32(rc.H()))
	binary.LittleEndian.PutUint16(hdr[26:], 1)
	binary.LittleEndian.PutUint16(hdr[28:], 32)
	binary.LittleEndian.PutUint32(hdr[34:], uint32(len(buf)))
	_, _ = f.Write(hdr)
	rowBytes := int(rc.W()) * 4
	for y := int(rc.H()) - 1; y >= 0; y-- {
		_, _ = f.Write(buf[y*rowBytes : (y+1)*rowBytes])
	}
}
