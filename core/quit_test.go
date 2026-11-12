package core

import (
	"context"
	"slices"
	"testing"
	"time"
)

func TestOnQuitRunsLivingHandlersOldestFirstOnce(t *testing.T) {
	var ran []string
	var gone *Signal[bool]
	var deadline time.Time
	h := newHost(t, Size{Width: 390, Height: 844}, func(cx *Scope) Node {
		gone = NewSignal(cx, false)
		cx.OnQuit(func(ctx context.Context) {
			deadline, _ = ctx.Deadline()
			ran = append(ran, "app")
		})
		return View{Children: []Node{
			Show{When: func() bool { return !gone.Get() }, Then: func(cx *Scope) Node {
				cx.OnQuit(func(context.Context) { ran = append(ran, "gone") })
				return View{}
			}},
			Func(func(cx *Scope) Node {
				cx.OnQuit(func(context.Context) { panic("broken") })
				cx.OnQuit(func(context.Context) { ran = append(ran, "after a panic") })
				return View{}
			}),
		}}
	})
	h.commit()
	gone.Set(true)
	h.commit()
	end := time.Now().Add(QuitTimeout)
	if !h.root.Quitting() {
		t.Fatal("Quitting found no handlers")
	}
	if !slices.Equal(ran, []string{"app", "after a panic"}) {
		t.Errorf("ran %v", ran)
	}
	if len(h.errs) != 1 {
		t.Errorf("the panic was reported as %v", h.errs)
	}
	h.errs = nil
	if d := deadline.Sub(end); d < 0 || d > time.Second {
		t.Errorf("the handlers' time ends %v past QuitTimeout", d)
	}
	if h.root.Quitting() || len(ran) != 2 {
		t.Errorf("a second quit ran %v", ran)
	}
}
