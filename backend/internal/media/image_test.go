package media

import (
	"bytes"
	"image"
	"image/png"
	"testing"
)

func TestProcess(t *testing.T) {
	var buf bytes.Buffer
	_ = png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 2400, 1200)))
	full, thumb, e := Process(buf.Bytes())
	if e != nil {
		t.Fatal(e)
	}
	for i, b := range [][]byte{full, thumb} {
		c, f, e := image.DecodeConfig(bytes.NewReader(b))
		if e != nil || f != "jpeg" {
			t.Fatal(e)
		}
		want := 1920
		if i == 1 {
			want = 480
		}
		if c.Width != want {
			t.Fatalf("got %d", c.Width)
		}
	}
	if _, _, e := Process([]byte("<svg/>")); e == nil {
		t.Fatal("accepted invalid format")
	}
}
