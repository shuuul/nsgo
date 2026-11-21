//go:build android && nsgotest

package android

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/testhost"
)

func TestPixelBuffersInImageAndCanvas(t *testing.T) {
	const side = 48
	frame := image.NewRGBA(image.Rect(0, 0, side, side))
	paint := func(c color.RGBA) {
		draw.Draw(frame, frame.Bounds(), image.NewUniform(c), image.Point{}, draw.Src)
		draw.Draw(frame, image.Rect(side/2, 0, side, side), image.NewUniform(color.RGBA{B: 255, A: 255}), image.Point{}, draw.Src)
	}
	paint(color.RGBA{R: 128, A: 128})

	var scope *core.Scope
	var version *core.Signal[uint64]
	var loads atomic.Int32
	loaded := func() { loads.Add(1) }
	window, err := testhost.Current().Open(func(cx *core.Scope) core.Node {
		scope = cx
		version = core.NewSignal(cx, uint64(0))
		src := func() core.ImageSource { return core.ImageSource{Pixels: frame, Version: version.Get()} }
		half := core.Style{Width: core.Pct(100), Height: core.Pct(50)}
		return core.View{
			Style: core.Style{
				Width: core.Pct(100), Height: core.Pct(100),
				Direction: core.Column, Background: core.RGB(0xffffff),
			},
			Children: []core.Node{
				core.Image{Style: half, ResizeMode: core.Stretch, SourceFunc: src,
					OnLoadStart: loaded, OnLoad: func(core.Size) { loaded() }, OnLoadEnd: loaded,
					OnError: func(error) { loaded() }},
				core.Canvas{Style: half, Draw: func(d *core.Drawing, size core.Size) {
					w, h := float32(size.Width), float32(size.Height)
					y := (h - side) / 2
					d.DrawImage(src(), w/6-side/2, y)
					d.DrawImageIn(src(), w/2-side/2, y, side, side)
					d.DrawImageFrom(src(), side/2, 0, side/2, side, w*5/6-side/2, y, side, side)
				}},
			},
		}
	}, testhost.Options{})
	if err != nil {
		t.Fatal(err)
	}
	s := window.(*testSurface)

	check := func(previous, want color.RGBA) {
		t.Helper()
		s.Sync()
		ratio := float64(math.Float32frombits(s.s.ratio.Load()))
		deadline := time.Now().Add(4 * time.Second)
		for {
			data, err := s.Screenshot()
			if err != nil {
				t.Fatal(err)
			}
			img, err := png.Decode(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			w, h := img.Bounds().Dx(), img.Bounds().Dy()
			left := int(side / 4 * ratio)
			points := []image.Point{{X: w / 4, Y: h / 4}, {X: w/6 - left, Y: h * 3 / 4},
				{X: w/2 - left, Y: h * 3 / 4}, {X: w * 5 / 6, Y: h * 3 / 4}}
			ready := true
			for i, at := range points {
				expected := want
				if i == 3 {
					expected = color.RGBA{B: 255, A: 255}
				}
				got := color.RGBAModel.Convert(img.At(at.X, at.Y)).(color.RGBA)
				if nearPixelColor(got, expected) {
					continue
				}
				ready = false
				if time.Now().After(deadline) || previous.A != 0 && !nearPixelColor(got, previous) {
					t.Fatalf("sample %d at %v = %v, want %v (previous %v)", i, at, got, expected, previous)
				}
			}
			if ready {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}

	red := color.RGBA{R: 255, G: 127, B: 127, A: 255}
	green := color.RGBA{R: 127, G: 255, B: 127, A: 255}
	check(color.RGBA{}, red)
	scope.Post(func() {
		paint(color.RGBA{G: 128, A: 128})
		version.Set(1)
	})
	check(red, green)
	if n := loads.Load(); n != 0 {
		t.Fatalf("pixel updates delivered %d load events", n)
	}
}

func nearPixelColor(got, want color.RGBA) bool {
	near := func(a, b uint8) bool { return int(a)-int(b) >= -3 && int(a)-int(b) <= 3 }
	return near(got.R, want.R) && near(got.G, want.G) && near(got.B, want.B) && got.A == want.A
}
