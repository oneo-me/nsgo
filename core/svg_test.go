package core

import (
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

func TestIconInheritsColorAndSize(t *testing.T) {
	s := MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="8"/></svg>`))
	var tint *Signal[Color]
	h := newHost(t, Size{Width: 200, Height: 100}, func(cx *Scope) Node {
		tint = NewSignal(cx, RGB(0x112233))
		return View{Style: Style{AlignItems: AlignStart}, Children: []Node{
			View{
				StyleFunc: func() Style {
					return Style{Color: tint.Get(), FontSize: 20, AlignItems: AlignStart}
				},
				Children: []Node{
					Icon{SVG: s, TestID: "icon"},
					Icon{SVG: s, TestID: "wide", Style: Style{Width: Pt(48), Height: Pt(24)}},
				},
			},
		}}
	})
	drawing := func(b *driver.MountBatch, id NodeID) *Drawing {
		for _, it := range b.Items {
			if it.Kind == driver.ItemCanvas && it.ID == id {
				return it.Drawing
			}
		}
		return nil
	}
	b := h.commit()
	icon := h.find("icon")
	if got := icon.frame.Size; got != (Size{Width: 20, Height: 20}) {
		t.Fatalf("icon size = %v, want its font size", got)
	}
	d := drawing(b, icon.id)
	if d == nil {
		t.Fatal("the icon did not draw")
	}
	found := false
	for _, op := range d.Ops {
		if op.Kind == DrawStrokeColor {
			found = true
			if op.Color != RGB(0x112233) {
				t.Errorf("stroke color = %#x, want the inherited text color", op.Color)
			}
		}
	}
	if !found {
		t.Fatal("no stroke color")
	}
	wide := h.find("wide")
	d = drawing(b, wide.id)
	if d == nil {
		t.Fatal("the wide icon did not draw")
	}
	// 24 units in a 24-point-high box, centered in the 48-point width.
	if op := d.Ops[1]; op.Kind != DrawTranslate || op.X != 12 || op.Y != 0 {
		t.Errorf("wide translate = %+v", op)
	}
	if op := d.Ops[2]; op.Kind != DrawScale || op.X != 1 || op.Y != 1 {
		t.Errorf("wide scale = %+v", op)
	}

	tint.Set(RGB(0xff8800))
	b = h.commit()
	d = drawing(b, icon.id)
	if d == nil {
		t.Fatal("a color change did not redraw")
	}
	for _, op := range d.Ops {
		if op.Kind == DrawStrokeColor && op.Color != RGB(0xff8800) {
			t.Errorf("stroke color after change = %#x", op.Color)
		}
	}
}

func TestIconTakesClasses(t *testing.T) {
	s := MustParseSVG([]byte(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor"><circle cx="12" cy="12" r="8"/></svg>`))
	h := newHost(t, Size{Width: 200, Height: 100}, func(cx *Scope) Node {
		return View{Style: Style{AlignItems: AlignStart}, Children: []Node{Icon{SVG: s, Class: "size-8 text-[#112233]", TestID: "icon"}}}
	})
	b := h.commit()
	icon := h.find("icon")
	if got := icon.frame.Size; got != (Size{Width: 32, Height: 32}) {
		t.Fatalf("icon size = %v, want size-8's", got)
	}
	for _, it := range b.Items {
		if it.Kind != driver.ItemCanvas || it.ID != icon.id {
			continue
		}
		for _, op := range it.Drawing.Ops {
			if op.Kind == DrawStrokeColor && op.Color != RGB(0x112233) {
				t.Errorf("stroke color = %#x, want the class's", op.Color)
			}
		}
		return
	}
	t.Fatal("the icon did not draw")
}

func TestIconChangesDocument(t *testing.T) {
	square := MustParseSVG([]byte(`<svg viewBox="0 0 24 24"><rect width="24" height="24"/></svg>`))
	wide := MustParseSVG([]byte(`<svg viewBox="0 0 48 24"><rect width="48" height="24"/></svg>`))
	var doc *Signal[*SVG]
	h := newHost(t, Size{Width: 200, Height: 100}, func(cx *Scope) Node {
		doc = NewSignal(cx, square)
		return View{Style: Style{AlignItems: AlignStart}, Children: []Node{
			Icon{SVGFunc: doc.Get, TestID: "icon"},
		}}
	})
	b := h.commit()
	icon := h.find("icon")
	if got, want := icon.frame.Size, (Size{Width: DefaultFontSize, Height: DefaultFontSize}); got != want {
		t.Fatalf("icon = %v, want its font size %v", got, want)
	}
	doc.Set(wide)
	b = h.commit()
	if got, want := icon.frame.Size, (Size{Width: DefaultFontSize * 2, Height: DefaultFontSize}); got != want {
		t.Fatalf("after the document changed, icon = %v, want %v", got, want)
	}
	found := false
	for _, it := range b.Items {
		if it.Kind == driver.ItemCanvas && it.ID == icon.id {
			found = true
		}
	}
	if !found {
		t.Fatal("a new document did not draw")
	}
}

func TestSVGViewTakesTheDocumentsSize(t *testing.T) {
	s := MustParseSVG([]byte(`<svg width="30" height="10" viewBox="0 0 3 1"><rect width="3" height="1" fill="red"/></svg>`))
	h := newHost(t, Size{Width: 200, Height: 100}, func(cx *Scope) Node {
		return View{Style: Style{AlignItems: AlignStart, FontSize: 20}, Children: []Node{SVGView{SVG: s, TestID: "doc"}}}
	})
	h.commit()
	if got := h.find("doc").frame.Size; got != (Size{Width: 30, Height: 10}) {
		t.Errorf("the view is %v, want the document's size", got)
	}
}
