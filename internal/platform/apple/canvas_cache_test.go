//go:build darwin

package apple

import (
	"fmt"
	"image/color"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/canvastest"
	"github.com/nsgo-dev/nsgo/internal/platform/imageload/imageloadtest"
)

func isolatedCanvasImages(t *testing.T) {
	t.Helper()
	saved := canvasImages
	canvasImages = map[string]*loadedImage{}
	t.Cleanup(func() {
		for key, im := range canvasImages {
			if im.cancel != nil {
				im.cancel()
			}
			im.release()
			im.image = 0
			delete(canvasImages, key)
		}
		canvasImages = saved
		runMain()
	})
}

func TestCanvasImageCacheBoundAndReload(t *testing.T) {
	isolatedCanvasImages(t)
	sources := canvastest.CacheSources(1000)
	load := func(src driver.ImageSource) *loadedImage {
		t.Helper()
		canvasImage(src, CanvasHost{Ratio: 1})
		im := canvasImages[src.Key()]
		for deadline := time.Now().Add(5 * time.Second); im.cancel != nil; {
			runMain()
			if time.Now().After(deadline) {
				t.Fatal("image load timed out")
			}
			time.Sleep(time.Millisecond)
		}
		if im.image == 0 {
			t.Fatal("no decoded image")
		}
		return im
	}
	first := load(sources[0])
	old := load(sources[1])
	for i := 2; i < len(sources); i++ {
		load(sources[i])
		if i%32 == 0 {
			canvasImage(sources[0], CanvasHost{})
		}
		if len(canvasImages) > maxCanvasImages {
			t.Fatalf("%d cached images", len(canvasImages))
		}
	}
	if canvasImages[sources[0].Key()] != first {
		t.Fatal("recently drawn image evicted")
	}
	if old.image != 0 {
		t.Fatal("evicted native image retained")
	}
	reloaded := load(sources[1])
	if reloaded == old {
		t.Fatal("evicted image slot reused")
	}
	d := (&driver.Drawing{}).DrawImageIn(sources[1], 0, 0, 2, 2)
	img := play(t, "cache-reload", d, 2, 2, false)
	if got := img.RGBAAt(1, 1); got != (color.RGBA{R: 255, A: 255}) {
		t.Fatalf("reloaded pixel %v", got)
	}
}

// Loads under way are never evicted, even by later passes: the cache exceeds its bound only by them, and shrinks back once they have loaded. Failed loads are evicted as loaded ones are.
func TestCanvasImageCacheKeepsLoadsUnderWayAndEvictsFailed(t *testing.T) {
	isolatedCanvasImages(t)
	runMain()
	sources := canvastest.CacheSources(2 * maxCanvasImages)
	for _, src := range sources {
		endCanvasPass()
		canvasImage(src, CanvasHost{Ratio: 1})
	}
	if len(canvasImages) != len(sources) {
		t.Fatalf("%d of %d loads under way kept", len(canvasImages), len(sources))
	}
	kept := map[*loadedImage]bool{}
	for _, src := range sources {
		kept[canvasImages[src.Key()]] = true
	}
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(time.Millisecond) {
		runMain()
		loaded := 0
		for im := range kept {
			if im.image != 0 {
				loaded++
			}
		}
		if loaded == len(kept) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("%d of %d loaded", loaded, len(kept))
		}
	}
	endCanvasPass()
	canvasImage(canvastest.CacheSources(1)[0], CanvasHost{})
	if len(canvasImages) != maxCanvasImages {
		t.Fatalf("%d cached once the loads under way loaded", len(canvasImages))
	}
	evicted := 0
	for im := range kept {
		if im.image == 0 {
			evicted++
		}
	}
	if evicted != len(kept)-maxCanvasImages+1 {
		t.Fatalf("%d evicted images released their native image", evicted)
	}
	// Failed loads must be evictable too, even with no decoded native image.
	for key, im := range canvasImages {
		if im.cancel != nil {
			im.cancel()
		}
		im.release()
		delete(canvasImages, key)
	}
	for i := range maxCanvasImages {
		canvasImages[fmt.Sprint(i)] = &loadedImage{failed: true, used: uint64(i)}
	}
	canvasImage(sources[0], CanvasHost{})
	if len(canvasImages) != maxCanvasImages {
		t.Fatal("failed entries prevented eviction")
	}
	for deadline := time.Now().Add(5 * time.Second); canvasImages[sources[0].Key()].cancel != nil; {
		runMain()
		if time.Now().After(deadline) {
			t.Fatal("reload timed out")
		}
		time.Sleep(time.Millisecond)
	}
}

// A pass drawing more sources than the bound keeps them all: evicting one would load it again at every pass, forever.
func TestCanvasImageCacheKeepsAWidePass(t *testing.T) {
	isolatedCanvasImages(t)
	runMain()
	sources := canvastest.CacheSources(2 * maxCanvasImages)
	for _, src := range sources {
		canvasImage(src, CanvasHost{Ratio: 1})
	}
	if len(canvasImages) != len(sources) {
		t.Fatalf("a pass of %d sources kept %d", len(sources), len(canvasImages))
	}
	kept := map[*loadedImage]bool{}
	for _, src := range sources {
		kept[canvasImages[src.Key()]] = true
	}
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(time.Millisecond) {
		runMain()
		loaded := 0
		for im := range kept {
			if im.image != 0 {
				loaded++
			}
		}
		if loaded == len(kept) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("%d of %d loaded", loaded, len(kept))
		}
	}
	for _, src := range sources {
		if im := canvasImage(src, CanvasHost{}); !kept[im] || im.image == 0 {
			t.Fatal("the next pass loaded a wide pass's image anew")
		}
	}
	runMain()
	for _, src := range canvastest.CacheSources(maxCanvasImages) {
		canvasImage(src, CanvasHost{})
		endCanvasPass()
	}
	if len(canvasImages) > maxCanvasImages {
		t.Fatalf("%d cached after later passes", len(canvasImages))
	}
	for im := range kept {
		if im.image != 0 {
			t.Fatal("a wide pass's evicted image kept its native image")
		}
	}
}

// Canvases drawn in passes of their own whose images together exceed the bound and load more slowly than a pass all get every image: a canvas's pass must not cancel another's loads, which would draw again and cancel the first's, forever.
func TestCanvasImageCacheCanvasesDrawnApartLoadSlowImages(t *testing.T) {
	isolatedCanvasImages(t)
	runMain()
	var sources []driver.ImageSource
	for _, uri := range imageloadtest.Slow(t, 88, 150*time.Millisecond, canvastest.RedPNG()) {
		sources = append(sources, driver.ImageSource{URI: uri})
	}
	type canvas struct {
		dirty   bool
		sources []driver.ImageSource
	}
	canvases := []*canvas{{sources: sources[:40]}, {sources: sources[40:80]}, {sources: sources[80:]}}
	draws := 0
	draw := func(c *canvas) {
		c.dirty = false
		draws++
		for _, src := range c.sources {
			canvasImage(src, CanvasHost{Owner: c, Ratio: 1, Redraw: func() { c.dirty = true }})
		}
		// The pass ends before the next canvas draws in a pass of its own.
		runMain()
	}
	for _, c := range canvases {
		draw(c)
	}
	for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(time.Millisecond) {
		runMain()
		for _, c := range canvases {
			if c.dirty {
				draw(c)
			}
		}
		loaded := 0
		for _, src := range sources {
			if im := canvasImages[src.Key()]; im != nil && im.image != 0 {
				loaded++
			}
		}
		if loaded == len(sources) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("%d of %d loaded after %d draws, %d cached", loaded, len(sources), draws, len(canvasImages))
		}
	}
	if draws > 3+len(sources) {
		t.Fatalf("%d draws for %d images", draws, len(sources))
	}
}
