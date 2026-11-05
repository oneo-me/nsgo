package toast_test

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/oneo-me/nsgo/base/toast"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/nsgotest"
)

func ids[K comparable, V any](es []toast.Entry[K, V]) []K {
	var out []K
	for _, e := range es {
		out = append(out, e.ID)
	}
	return out
}

func TestToastTimeoutPausesAndRemovalWaitsForTheExit(t *testing.T) {
	motion := toast.SonnerMotion()
	start := time.Unix(0, 0)
	var m toast.Manager[string, int]
	m.Push("a", 1, 5*time.Second, start)
	m.Advance(start.Add(motion.Duration), false)
	m.Advance(start.Add(motion.Duration+4*time.Second), true)
	if s := m.Entries()[0].Status; s != toast.Present {
		t.Fatalf("status = %v, want present while paused", s)
	}
	if a := m.Advance(start.Add(motion.Duration+9*time.Second), false); !slices.Equal(a.Ending, []string{"a"}) || len(a.Removed) > 0 {
		t.Fatalf("advance = %+v, want a ending and nothing removed", a)
	}
	a := m.Advance(start.Add(motion.Duration+motion.ExitDuration+9*time.Second), false)
	if len(a.Removed) != 1 || a.Removed[0].ID != "a" || a.Removed[0].Value != 1 || m.Len() != 0 {
		t.Fatalf("advance = %+v, want a removed after its exit", a)
	}
}

func TestToastLimitKeepsEndingToastsAndPushReplaces(t *testing.T) {
	now := time.Unix(0, 0)
	var m toast.Manager[string, int]
	for i, id := range []string{"a", "b", "c"} {
		m.Push(id, i, 0, now)
	}
	m.Dismiss("a", now)
	if got := ids(m.Visible(1)); !slices.Equal(got, []string{"a", "c"}) {
		t.Errorf("visible = %v, want the ending toast and the newest", got)
	}
	if old, ok := m.Push("b", 9, 0, now); !ok || old != 1 {
		t.Errorf("push = %v, %v, want the replaced value", old, ok)
	}
	if got := ids(m.Entries()); !slices.Equal(got, []string{"a", "c", "b"}) {
		t.Errorf("entries = %v, want the replaced toast the newest", got)
	}
	if got := m.DismissAll(now); !slices.Equal(got, []string{"c", "b"}) {
		t.Errorf("dismiss all = %v, want those not ending yet", got)
	}
}

func TestToastStack(t *testing.T) {
	type entry = toast.Entry[int, string]
	var dismissed []int
	var expanded []bool
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		shown := core.NewSignalFunc(cx, []entry{{1, "one", toast.Present}, {2, "two", toast.Present}}, slices.Equal)
		return core.View{Style: core.Style{FlexGrow: 1, Padding: core.All(20)}, Children: []core.Node{
			toast.Stack[entry, int]{TestID: "stack", Toasts: shown.Get, Key: func(e entry) int { return e.ID },
				Style:            core.Style{Width: core.Pt(200)},
				OnExpandedChange: func(e bool) { expanded = append(expanded, e) },
				Toast: func(cx *core.Scope, e func() entry) core.Node {
					id := core.Untracked(cx, e).ID
					return toast.Toast{TestID: fmt.Sprint("toast-", id), Label: core.Untracked(cx, e).Value,
						Status:           func() toast.Status { return e().Status },
						OnDismissRequest: func() { dismissed = append(dismissed, id) },
						Style:            core.Style{Height: core.Pt(core.Points(30 + 10*id))}}
				}},
		}}
	})
	// Collapsed: the newest (50) in front, the older (40) 14 behind it.
	if h := app.Find("id:stack").Frame.Size.Height; h != 64 {
		t.Errorf("collapsed height = %v, want 64", h)
	}
	if v := app.Find("id:toast-1"); v.Props.Role != core.RoleAlert {
		t.Errorf("role = %v, want alert", v.Props.Role)
	}
	app.Hover("id:stack")
	if h := app.Find("id:stack").Frame.Size.Height; h != 40+14+50 {
		t.Errorf("expanded height = %v, want both toasts and the gap", h)
	}
	app.HoverOut()
	if !slices.Equal(expanded, []bool{true, false}) {
		t.Errorf("expanded changes = %v", expanded)
	}
	app.Drag("id:toast-2", core.Point{X: 30}, core.Point{X: 80})
	if !slices.Equal(dismissed, []int{2}) {
		t.Errorf("dismissed = %v, want the swiped toast", dismissed)
	}
	app.Drag("id:toast-2", core.Point{Y: 80})
	if len(dismissed) != 1 {
		t.Errorf("a drag across the swipe dismissed %v", dismissed)
	}
}
