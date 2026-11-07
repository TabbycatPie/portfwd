// bmp2png：把 PortFwd --shot 产出的 32 位 BMP 转成 PNG（纯标准库，无外部依赖）。
package main

import (
	"encoding/binary"
	"image"
	"image/png"
	"math"
	"os"
	"strconv"
)

func main() {
	if len(os.Args) < 3 {
		println("usage: bmp2png in.bmp out.png")
		return
	}
	b, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	off := int(binary.LittleEndian.Uint32(b[10:]))
	w := int(int32(binary.LittleEndian.Uint32(b[18:])))
	h := int(int32(binary.LittleEndian.Uint32(b[22:])))
	bpp := int(binary.LittleEndian.Uint16(b[28:]))
	if bpp != 32 {
		panic("only 32bpp bmp supported")
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	row := w * 4
	for y := 0; y < h; y++ {
		src := off + (h-1-y)*row // 输入是 bottom-up
		for x := 0; x < w; x++ {
			bb := b[src+x*4+0]
			gg := b[src+x*4+1]
			rr := b[src+x*4+2]
			i := img.PixOffset(x, y)
			img.Pix[i+0] = rr
			img.Pix[i+1] = gg
			img.Pix[i+2] = bb
			img.Pix[i+3] = 255
		}
	}
	// 可选第 4 个参数：圆角半径（像素），把四角裁成透明，配合窗口圆角更真实
	radius := 0.0
	if len(os.Args) > 3 {
		if v, e := strconv.Atoi(os.Args[3]); e == nil {
			radius = float64(v)
		}
	}
	if radius > 0 {
		wf, hf := float64(w), float64(h)
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				cx := math.Min(math.Max(float64(x)+0.5, radius), wf-radius)
				cy := math.Min(math.Max(float64(y)+0.5, radius), hf-radius)
				dx, dy := float64(x)+0.5-cx, float64(y)+0.5-cy
				if dx*dx+dy*dy > radius*radius {
					i := img.PixOffset(x, y)
					img.Pix[i+3] = 0
				}
			}
		}
	}

	f, err := os.Create(os.Args[2])
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		panic(err)
	}
	println("wrote", os.Args[2])
}
