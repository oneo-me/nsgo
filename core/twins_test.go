package core

import (
	"strings"
	"testing"
)

func TestReplacingTwinSetTwicePanics(t *testing.T) {
	var caught string
	h := newHost(t, Size{Width: 100, Height: 100}, func(cx *Scope) Node {
		return View{Children: []Node{
			// A style and classes stack, so both halves are fine.
			View{Style: Style{Width: Pt(10)}, StyleFunc: func() Style { return Style{Height: Pt(10)} },
				Class: "p-1", ClassFunc: func() string { return "m-1" }, TestID: "stacked"},
			Catch{
				Try: func(cx *Scope) Node {
					return Text{Content: "a", ContentFunc: func() string { return "b" }}
				},
				Fallback: func(cx *Scope, err error, _ func()) Node {
					caught = err.Error()
					return nil
				},
			},
		}}
	})
	h.commit()
	if h.find("stacked") == nil {
		t.Fatal("a View with Style, StyleFunc, Class and ClassFunc did not mount")
	}
	if !strings.Contains(caught, "core.Text sets both Content and ContentFunc") {
		t.Fatalf("caught %q", caught)
	}
}
