package media

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func solid(w, h int, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

func TestResizeKeepsRatioAndNeverUpscales(t *testing.T) {
	src := solid(4000, 3000, color.RGBA{10, 120, 60, 255})
	for _, w := range []int{640, 1024, 1600} {
		out := resize(src, w)
		if out.Bounds().Dx() != w || out.Bounds().Dy() != w*3/4 {
			t.Errorf("width %d: got %v", w, out.Bounds())
		}
		if got := out.RGBAAt(w/2, w/4); got != (color.RGBA{10, 120, 60, 255}) {
			t.Errorf("width %d: color drifted to %v", w, got)
		}
	}
	if out := resize(solid(300, 200, color.RGBA{A: 255}), 640); out.Bounds().Dx() != 300 {
		t.Fatalf("must not upscale, got %v", out.Bounds())
	}
}

func TestHalveAverages(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 2, 2))
	src.SetRGBA(0, 0, color.RGBA{0, 0, 0, 255})
	src.SetRGBA(1, 0, color.RGBA{100, 0, 0, 255})
	src.SetRGBA(0, 1, color.RGBA{100, 0, 0, 255})
	src.SetRGBA(1, 1, color.RGBA{200, 0, 0, 255})
	if got := halve(src).RGBAAt(0, 0); got.R != 100 || got.A != 255 {
		t.Fatalf("got %v", got)
	}
}

func TestOrientation(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 4, 2))
	src.SetRGBA(0, 0, color.RGBA{255, 0, 0, 255})
	out := applyOrientation(src, 6)
	if out.Bounds().Dx() != 2 || out.Bounds().Dy() != 4 {
		t.Fatalf("rotate 90 should swap axes, got %v", out.Bounds())
	}
	if out.RGBAAt(1, 0).R != 255 {
		t.Fatal("top-left pixel must move to the top-right after a clockwise turn")
	}
	if applyOrientation(src, 3).RGBAAt(3, 1).R != 255 {
		t.Fatal("rotate 180 must move top-left to bottom-right")
	}
	rotated := resizeOriented(solid(4000, 3000, color.RGBA{A: 255}), 900, 6)
	if rotated.Bounds().Dx() != 900 || rotated.Bounds().Dy() != 1200 {
		t.Fatalf("portrait variant should be 900x1200, got %v", rotated.Bounds())
	}
}

func TestDecodeRejectsHugeImages(t *testing.T) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewGray(image.Rect(0, 0, 8000, 6000))); err != nil {
		t.Fatal(err)
	}
	if _, _, err := decode(buf.Bytes()); !errors.Is(err, ErrTooManyPixels) {
		t.Fatalf("48 MP image must be rejected, got %v", err)
	}
	if _, _, err := decode([]byte("not an image at all")); !errors.Is(err, ErrUnsupportedType) {
		t.Fatalf("got %v", err)
	}
}
