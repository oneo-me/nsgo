package main

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/oneo-me/nsgo/base/toast"
	"github.com/oneo-me/nsgo/core"

	"github.com/oneo-me/nsgo/examples/gallery/locales"
	"github.com/oneo-me/nsgo/ui"
)

// The gallery's toasts: showToast(...) from anywhere, shown by the toaster the
// root places once, styled by the gallery. A styled layer built on base's
// unstyled pieces, as NSH's gallery builds its toast.tsx: a ToastManager
// keeps the toasts and their lifecycle, a ToastStack lays them out, a
// Toast is each one's root.

// toastContent is what a toast shows.
type toastContent struct {
	// id names it: showing another with the same id replaces it.
	id          string
	title       string
	description string
	// action is a button that closes it; done receives its label.
	action string
	// close is the label of a close button.
	close string
	// duration is how long it stays (5 s); negative until dismissed.
	duration time.Duration
	// done receives the action that closed it, "" when none did.
	done func(answer string)
}

type toastEntry = toast.Entry[string, toastContent]

const (
	toastLimit = 3                     // how many show at once
	toastTick  = 50 * time.Millisecond // how often the lifecycle advances while one shows
)

// toaster is the gallery's toasts, kept across the rebuilds of its root.
type toaster struct {
	cx       *core.Scope
	manager  toast.Manager[string, toastContent]
	shown    *core.Signal[[]toastEntry]
	expanded *core.Signal[bool]
	chosen   map[string]string
	ticking  bool
	count    int
}

func newToaster(cx *core.Scope) *toaster {
	return &toaster{cx: cx, chosen: map[string]string{},
		shown:    core.NewSignalFunc(cx, []toastEntry(nil), func(a, b []toastEntry) bool { return false }),
		expanded: core.NewSignal(cx, false)}
}

func (t *toaster) sync() { t.shown.Set(t.manager.Visible(toastLimit)) }

func (t *toaster) tick() {
	a := t.manager.Advance(t.cx.Now(), t.expanded.GetUntracked())
	for _, gone := range a.Removed {
		if gone.Value.done != nil {
			gone.Value.done(t.chosen[gone.ID])
		}
		delete(t.chosen, gone.ID)
	}
	if a.Changed {
		t.sync()
	}
	if t.manager.Len() == 0 {
		t.ticking = false
		return
	}
	t.cx.AfterFunc(toastTick, t.tick)
}

func (t *toaster) run() {
	t.sync()
	if !t.ticking {
		t.ticking = true
		t.cx.AfterFunc(toastTick, t.tick)
	}
}

// show shows a toast.
func (t *toaster) show(c toastContent) {
	if c.id == "" {
		t.count++
		c.id = fmt.Sprintf("toast-%d", t.count)
	}
	if c.duration == 0 {
		c.duration = 5 * time.Second
	}
	timeout := max(c.duration, 0)
	if old, ok := t.manager.Push(c.id, c, timeout, t.cx.Now()); ok && old.done != nil {
		old.done("")
	}
	t.run()
}

// dismiss closes the toast id, every toast when id is "".
func (t *toaster) dismiss(id string) {
	if id == "" {
		t.manager.DismissAll(t.cx.Now())
	} else {
		t.manager.Dismiss(id, t.cx.Now())
	}
	t.run()
}

func (t *toaster) choose(id, action string) {
	t.chosen[id] = action
	t.dismiss(id)
}

// showToast shows a toast from anywhere in the gallery.
func showToast(c toastContent) { shell.toasts.show(c) }

// item is one toast, the gallery's look.
func (t *toaster) item(cx *core.Scope, e func() toastEntry) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	fin := ui.FinOf(cx)
	first := core.Untracked(cx, e)
	id, c := first.ID, first.Value
	label := strings.Join(slices.DeleteFunc([]string{c.title, c.description}, func(s string) bool { return s == "" }), ". ")
	texts := []core.Node{core.Text{Content: c.title, Style: core.Style{FontSize: 13, FontWeight: core.FontWeightSemibold}, StyleFunc: ui.Colored(colors.Text)}}
	if c.description != "" {
		texts = append(texts, core.Text{Content: c.description, Style: core.Style{FontSize: 13}, StyleFunc: ui.Colored(colors.Muted)})
	}
	children := []core.Node{core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 2}, Children: texts}}
	if c.action != "" {
		children = append(children, core.View{Role: core.RoleButton, Label: c.action, OnPress: func(core.PressEvent) { t.choose(id, c.action) },
			Style: core.Style{Padding: core.Edges{Left: 10, Right: 10}, Height: core.Pt(28), JustifyContent: core.JustifyCenter,
				CornerRadius: ui.RadiusControl, Active: &core.Style{Opacity: core.Alpha(0.7)}}, StyleFunc: ui.Filled(colors.Light),
			Children: []core.Node{core.Text{Content: c.action, Style: core.Style{FontSize: 13, FontWeight: core.FontWeightMedium}, StyleFunc: ui.Colored(colors.OnLight)}}})
	}
	if c.close != "" {
		children = append(children, core.View{Role: core.RoleButton, Label: tr.ToastClose(), OnPress: func(core.PressEvent) { t.dismiss(id) },
			Style:    core.Style{Width: core.Pt(24), Height: core.Pt(24), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, CornerRadius: 12},
			Children: []core.Node{core.Text{Content: c.close, Style: core.Style{FontSize: 16}, StyleFunc: ui.Colored(colors.Muted)}}})
	}
	fade := core.Transition{Duration: ms(200), Easing: core.EaseOut}
	return toast.Toast{TestID: id, Label: label,
		Status:           func() toast.Status { return e().Status },
		OnDismissRequest: func() { t.dismiss(id) },
		StartingStyle:    core.Style{Opacity: core.Alpha(0), Transform: core.Transform{TranslateY: 16}},
		EndingStyle:      core.Style{Opacity: core.Alpha(0), Transform: core.Transform{Scale: 0.95}},
		SwipingStyle:     core.Style{Transition: core.Transition{}},
		StyleFunc: func() core.Style {
			return fin.Raised().Over(core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12,
				Padding: core.Edges{Top: 12, Right: 14, Bottom: 12, Left: 14}, CornerRadius: ui.RadiusInset, BorderCurve: core.BorderCurveContinuous,
				BorderWidth: 1, BorderColor: colors.Border(), Background: colors.Raised(), Transition: fade})
		},
		Children: children}
}

// view is where the toasts show: at the bottom, inside the safe area.
func (t *toaster) view(wide bool) core.Node {
	placement, align := toast.AnchorBottomCenter, core.AlignCenter
	if wide {
		placement, align = toast.AnchorBottomRight, core.AlignEnd
	}
	return core.Overlay{PointerEvents: core.PointerEventsBoxNone,
		Style: core.Style{Left: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0), AlignItems: align,
			Padding: core.All(16), PaddingEnv: core.EnvEdges{Bottom: core.EnvSafeArea}},
		Children: []core.Node{toast.Stack[toastEntry, string]{TestID: "toasts", Placement: placement,
			Toasts: t.shown.Get, Key: func(e toastEntry) string { return e.ID }, Toast: t.item,
			Style:            core.Style{Width: core.Pt(340), MaxWidth: core.Pct(100)},
			OnExpandedChange: t.expanded.Set,
			OnCancel: func() {
				shown := t.shown.GetUntracked()
				for i := len(shown) - 1; i >= 0; i-- {
					if shown[i].Status != toast.Ending {
						t.dismiss(shown[i].ID)
						return
					}
				}
			}}}}
}
