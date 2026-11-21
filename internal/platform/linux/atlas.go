//go:build linux && !android

package linux

import "github.com/nsgo-dev/nsgo/internal/driver"

func (p *canvasPlayer) atlas(op *driver.DrawOp) {
	if op.Atlas == nil || op.Image == nil {
		return
	}
	slot := p.w.canvasImage(*op.Image, p.v)
	if slot == nil || slot.pic == nil {
		return
	}
	src := slot.surfaceOf(0)
	nat := natural(slot)
	if src == 0 || nat.Width <= 0 {
		return
	}
	pattern := cairoPatternCreateForSurface(src)
	defer cairoPatternDestroy(pattern)
	cr := p.cr
	path := cairoCopyPath(cr)
	k := float64(slot.pic.width) / float64(nat.Width)
	for _, sprite := range *op.Atlas {
		cairoSave(cr)
		t := sprite.Transform
		c, s := float64(t.SCos), float64(t.SSin)
		m := cairoMatrix{xx: c, yx: s, xy: -s, yy: c, x0: float64(t.TX), y0: float64(t.TY)}
		cairoTransform(cr, &m)
		b := sprite.Rect
		cairoNewPath(cr)
		cairoRectangle(cr, 0, 0, float64(b.Size.Width), float64(b.Size.Height))
		source := cairoMatrix{xx: k, yy: k, x0: k * float64(b.Origin.X), y0: k * float64(b.Origin.Y)}
		cairoPatternSetMatrix(pattern, &source)
		p.shadowed(func(cr uintptr) {
			cairoSave(cr)
			cairoClip(cr)
			cairoSetSource(cr, pattern)
			cairoPaintWithAlpha(cr, p.st.alpha*float64(sprite.Alpha))
			cairoRestore(cr)
		})
		cairoRestore(cr)
	}
	cairoNewPath(cr)
	cairoAppendPath(cr, path)
	cairoPathDestroy(path)
}
