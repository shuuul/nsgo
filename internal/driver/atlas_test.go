package driver

import "testing"

func TestDrawAtlasCopiesTheSpriteList(t *testing.T) {
	crop := Bounds{Origin: Point{4, 8}, Size: Size{12, 16}}
	rects := []Bounds{crop, {}, crop}
	placement := RSXform{SCos: 2, TX: 11, TY: 23}
	xforms := []RSXform{placement, placement, placement}
	alphas := []float32{.5}
	src := ImageSource{URI: "asset:atlas.png"}
	d := NewDrawing(nil).DrawAtlas(src, rects, xforms, alphas)
	rects[0], xforms[0], alphas[0] = Bounds{}, RSXform{}, 0
	if len(d.Ops) != 1 || d.Ops[0].Kind != DrawAtlas || d.Ops[0].Image.URI != src.URI {
		t.Fatalf("got %+v, want one atlas op", d.Ops)
	}
	got := *d.Ops[0].Atlas
	if len(got) != 2 || got[0] != (AtlasSprite{crop, placement, .5}) || got[1] != (AtlasSprite{crop, placement, 1}) {
		t.Fatalf("recorded %+v", got)
	}
}

func TestCanvasDamageIncludesAnAtlasUnderChangingTransform(t *testing.T) {
	at := Bounds{Origin: Point{5, 4}, Size: Size{2, 3}}
	d := NewDrawing(nil).Translate(3, 4).DrawAtlas(ImageSource{URI: "asset:atlas.png"}, []Bounds{at},
		[]RSXform{{SSin: 2, TX: 11, TY: 7}}, nil)
	after := append([]DrawOp(nil), d.Ops...)
	after[0].X = 6
	want := Bounds{Origin: Point{8, 11}, Size: Size{9, 4}}
	if got, ok := CanvasDamage(d.Ops, after); !ok || got != want {
		t.Fatalf("got %v, %v, want %v", got, ok, want)
	}
}
