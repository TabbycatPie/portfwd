// icongen 生成应用图标（多尺寸 ICO）：深色圆角方块 + 青蓝渐变 + 双向箭头。
// 纯标准库实现，支持 4x 超采样抗锯齿。输出 icons/icon.ico。
package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

var sizes = []int{16, 20, 24, 32, 40, 48, 64, 128, 256}

// headRight=true 时箭头头为直角三角形（背边竖直、尖角在轴外沿），false 为等腰三角
var headRight = false

const ss = 4 // supersample

type pt struct{ x, y float64 }

// 圆角矩形内点判定
func inRoundRect(x, y, w, h, r float64) bool {
	// 以左上角为原点
	if x < 0 || y < 0 || x > w || y > h {
		return false
	}
	cx := math.Min(math.Max(x, r), w-r)
	cy := math.Min(math.Max(y, r), h-r)
	dx, dy := x-cx, y-cy
	return dx*dx+dy*dy <= r*r
}

// 凸多边形内点判定
func inPoly(x, y float64, p []pt) bool {
	sign := 0
	for i := range p {
		a, b := p[i], p[(i+1)%len(p)]
		cross := (b.x-a.x)*(y-a.y) - (b.y-a.y)*(x-a.x)
		if cross != 0 {
			s := 1
			if cross < 0 {
				s = -1
			}
			if sign == 0 {
				sign = s
			} else if sign != s {
				return false
			}
		}
	}
	return true
}

func lerp(a, b, t float64) float64 { return a + (b-a)*t }

// 渲染某一尺寸的图像（已超采样后降采样）
func render(size int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	big := size * ss
	w := float64(size)
	pad := w * 0.035
	side := w - pad*2
	radius := side * 0.26

	// 箭头：只定义「向右」那一个，另一个用 180° 旋转生成，保证严格对称
	const half, headH, base, tailX, tipX = 0.055, 0.10, 0.56, 0.245, 0.795
	var outline []pt2
	if headRight {
		// 直角三角形头：上沿直通箭尖，下沿斜边收回后下方
		outline = []pt2{
			{tailX, 0.365 - half},
			{tipX, 0.365 - half},
			{base, 0.365 + half},
			{tailX, 0.365 + half},
		}
	} else {
		outline = []pt2{
			{tailX, 0.365 - half}, {base, 0.365 - half}, {base, 0.365 - headH},
			{tipX, 0.365}, {base, 0.365 + headH}, {base, 0.365 + half}, {tailX, 0.365 + half},
		}
	}
	upShape := roundPoly(outline, 0.022)
	upTail := pt2{tailX, 0.365}
	// 旋转 180°
	rot := func(v []pt2) []pt2 {
		out := make([]pt2, len(v))
		for i, q := range v {
			out[i] = pt2{1 - q.x, 1 - q.y}
		}
		return out
	}
	dnShape := rot(upShape)
	dnTail := pt2{1 - upTail.x, 1 - upTail.y}

	inArrow := func(shape []pt2, tail pt2, r, x, y float64) bool {
		dx, dy := x-tail.x, y-tail.y
		if dx*dx+dy*dy <= r*r {
			return true
		}
		return inPoly2(shape, x, y)
	}

	// 背景渐变：薄荷青 -> 靛蓝
	top := [3]float64{0x34, 0xD3, 0x99}
	bot := [3]float64{0x1D, 0x4E, 0xD8}

	for py := 0; py < size; py++ {
		for px := 0; px < size; px++ {
			var bgCov, fgCov float64
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					x := float64(px*ss+sx)/float64(big) * w
					y := float64(py*ss+sy)/float64(big) * w
					if inRoundRect(x-pad, y-pad, side, side, radius) {
						bgCov++
						if inArrow(upShape, upTail, half, x/w, y/w) || inArrow(dnShape, dnTail, half, x/w, y/w) {
							fgCov++
						}
					}
				}
			}
			total := float64(ss * ss)
			if bgCov == 0 {
				continue
			}
			t := float64(py) / float64(size-1)
			r := lerp(top[0], bot[0], t)
			g := lerp(top[1], bot[1], t)
			b := lerp(top[2], bot[2], t)
			// 左上角柔和高光
			hl := 0.16 * math.Max(0, 1-(float64(px)+float64(py))/float64(size)*1.15)
			r = lerp(r, 255, hl*0.35)
			g = lerp(g, 255, hl*0.35)
			b = lerp(b, 255, hl*0.35)

			f := fgCov / bgCov
			r = lerp(r, 255, f)
			g = lerp(g, 255, f)
			b = lerp(b, 255, f)

			img.SetNRGBA(px, py, color.NRGBA{
				R: uint8(math.Round(r)), G: uint8(math.Round(g)), B: uint8(math.Round(b)),
				A: uint8(math.Round(bgCov / total * 255)),
			})
		}
	}
	return img
}

// ---- ICO 封装 ----

type icoEntry struct {
	size int
	data []byte
}

func bmpEntry(img *image.NRGBA) []byte {
	size := img.Bounds().Dx()
	h := img.Bounds().Dy()
	// BITMAPINFOHEADER：高度为 2 倍（图像 + AND 掩码）
	hdr := make([]byte, 40)
	binary.LittleEndian.PutUint32(hdr[0:], 40)
	binary.LittleEndian.PutUint32(hdr[4:], uint32(size))
	binary.LittleEndian.PutUint32(hdr[8:], uint32(h*2))
	binary.LittleEndian.PutUint16(hdr[12:], 1)
	binary.LittleEndian.PutUint16(hdr[14:], 32)
	binary.LittleEndian.PutUint32(hdr[20:], uint32(size*h*4))

	buf := bytes.NewBuffer(hdr)
	// 像素：自下而上，BGRA
	for y := h - 1; y >= 0; y-- {
		for x := 0; x < size; x++ {
			c := img.NRGBAAt(x, y)
			buf.Write([]byte{c.B, c.G, c.R, c.A})
		}
	}
	// AND 掩码：全 0（不透明区域由 alpha 决定）
	maskRow := ((size + 31) / 32) * 4
	buf.Write(make([]byte, maskRow*h))
	return buf.Bytes()
}

func pngEntry(img *image.NRGBA) []byte {
	var b bytes.Buffer
	_ = png.Encode(&b, img)
	return b.Bytes()
}

func writeICO(path string, imgs map[int]*image.NRGBA) error {
	var entries []icoEntry
	for _, s := range sizes {
		img := imgs[s]
		if img == nil {
			continue
		}
		if s <= 64 {
			entries = append(entries, icoEntry{s, bmpEntry(img)})
		} else {
			entries = append(entries, icoEntry{s, pngEntry(img)})
		}
	}
	var out bytes.Buffer
	le := binary.LittleEndian
	tmp := make([]byte, 6)
	le.PutUint16(tmp[0:], 0)
	le.PutUint16(tmp[2:], 1)
	le.PutUint16(tmp[4:], uint16(len(entries)))
	out.Write(tmp)

	offset := 6 + 16*len(entries)
	for _, e := range entries {
		d := make([]byte, 16)
		w := e.size
		if w >= 256 {
			w = 0
		}
		d[0] = byte(w)
		d[1] = byte(w)
		d[2] = 0
		d[3] = 0
		le.PutUint16(d[4:], 1)
		le.PutUint16(d[6:], 32)
		le.PutUint32(d[8:], uint32(len(e.data)))
		le.PutUint32(d[12:], uint32(offset))
		out.Write(d)
		offset += len(e.data)
	}
	for _, e := range entries {
		out.Write(e.data)
	}
	return os.WriteFile(path, out.Bytes(), 0644)
}

func main() {
	out := "icons/icon.ico"
	if len(os.Args) > 1 {
		out = os.Args[1]
	}
	if len(os.Args) > 2 && os.Args[2] == "right" {
		headRight = true
	}
	imgs := map[int]*image.NRGBA{}
	for _, s := range sizes {
		imgs[s] = render(s)
	}
	_ = os.MkdirAll(dirOf(out), 0755)
	if err := writeICO(out, imgs); err != nil {
		panic(err)
	}
	// 另存一张 256 PNG 供预览
	preview := dirOf(out) + "/../icon-preview.png"
	if headRight {
		preview = dirOf(out) + "/../icon-preview-right.png"
	}
	f, _ := os.Create(preview)
	_ = png.Encode(f, render(512))
	_ = f.Close()
	println("wrote", out)
}

func dirOf(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' || p[i] == '\\' {
			return p[:i]
		}
	}
	return "."
}

// ---------- 轮廓圆角化 ----------

func lerpPt(a, b pt2, t float64) pt2 { return pt2{a.x + (b.x-a.x)*t, a.y + (b.y-a.y)*t} }

type pt2 struct{ x, y float64 }

// roundPoly 把折线每个角用半径 r 的圆弧替代（返回稠密顶点）
func roundPoly(v []pt2, r float64) []pt2 {
	n := len(v)
	// 多边形朝向（有向面积符号）
	var area float64
	for i := 0; i < n; i++ {
		a, b := v[i], v[(i+1)%n]
		area += a.x*b.y - b.x*a.y
	}
	orient := 1.0
	if area < 0 {
		orient = -1
	}
	out := make([]pt2, 0, n*10)
	for i := 0; i < n; i++ {
		p := v[(i-1+n)%n]
		c := v[i]
		q := v[(i+1)%n]
		// 角平分线方向的内缩距离
		d1 := math.Hypot(c.x-p.x, c.y-p.y)
		d2 := math.Hypot(q.x-c.x, q.y-c.y)
		if d1 < 1e-9 || d2 < 1e-9 {
			out = append(out, c)
			continue
		}
		// 凹角（外凸拐点）不做圆角，直接保留，避免外鼓
		cr := (c.x-p.x)*(q.y-c.y) - (c.y-p.y)*(q.x-c.x)
		if cr*orient < 0 {
			out = append(out, c)
			continue
		}
		u1 := pt2{(c.x - p.x) / d1, (c.y - p.y) / d1}
		u2 := pt2{(q.x - c.x) / d2, (q.y - c.y) / d2}
		cosA := u1.x*u2.x + u1.y*u2.y
		if cosA > 0.999 {
			out = append(out, c)
			continue
		}
		ang := math.Acos(math.Max(-1, math.Min(1, cosA)))
		cut := r / math.Tan(ang/2)
		cut = math.Min(cut, math.Min(d1, d2)*0.48)
		if cut <= 1e-9 {
			out = append(out, c)
			continue
		}
		rr := cut * math.Tan(ang/2)
		a := pt2{c.x - u1.x*cut, c.y - u1.y*cut}
		b := pt2{c.x + u2.x*cut, c.y + u2.y*cut}
		// 以 a、b 为端点的圆弧近似（用二次贝塞尔采样）
		ctrl := c
		steps := 8
		for k := 0; k <= steps; k++ {
			t := float64(k) / float64(steps)
			x := (1-t)*(1-t)*a.x + 2*(1-t)*t*ctrl.x + t*t*b.x
			y := (1-t)*(1-t)*a.y + 2*(1-t)*t*ctrl.y + t*t*b.y
			out = append(out, pt2{x, y})
		}
		_ = rr
	}
	return out
}

// inPoly2 射线法（支持凹多边形）
func inPoly2(v []pt2, x, y float64) bool {
	in := false
	n := len(v)
	for i, j := 0, n-1; i < n; j, i = i, i+1 {
		vi, vj := v[i], v[j]
		if (vi.y > y) != (vj.y > y) {
			xt := (vj.x-vi.x)*(y-vi.y)/(vj.y-vi.y) + vi.x
			if x < xt {
				in = !in
			}
		}
	}
	return in
}

