package media

import (
	"bytes"
	"fmt"
	"golang.org/x/image/draw"
	"image"
	"image/jpeg"
	_ "image/png"
)

func Process(data []byte) ([]byte, []byte, error) {
	cfg, format, e := image.DecodeConfig(bytes.NewReader(data))
	if e != nil || (format != "jpeg" && format != "png") || cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width*cfg.Height > 25000000 {
		return nil, nil, fmt.Errorf("unsupported image or more than 25 megapixels")
	}
	src, _, e := image.Decode(bytes.NewReader(data))
	if e != nil {
		return nil, nil, e
	}
	encode := func(max int) ([]byte, error) {
		w, h := src.Bounds().Dx(), src.Bounds().Dy()
		if w > max || h > max {
			if w >= h {
				h = h * max / w
				w = max
			} else {
				w = w * max / h
				h = max
			}
		}
		if w < 1 {
			w = 1
		}
		if h < 1 {
			h = 1
		}
		dst := image.NewRGBA(image.Rect(0, 0, w, h))
		draw.Draw(dst, dst.Bounds(), image.White, image.Point{}, draw.Src)
		draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)
		var out bytes.Buffer
		e := jpeg.Encode(&out, dst, &jpeg.Options{Quality: 85})
		return out.Bytes(), e
	}
	full, e := encode(1920)
	if e != nil {
		return nil, nil, e
	}
	thumb, e := encode(480)
	return full, thumb, e
}
