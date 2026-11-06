package ui

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/oneo-me/nsgo/base/toast"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"
)

// ToastType picks a toast's colored status icon; ToastPlain shows none.
type ToastType uint8

const (
	ToastPlain ToastType = iota
	ToastInfo
	ToastSuccess
	ToastWarning
	ToastError
)

// ToastContent is a toast: a Title, a quieter Description and, with
// Action, a button that runs OnAction and closes it. Leading replaces the
// Type's status icon, as with an image or a spinner. Duration is how long
// it stays, 5 s when 0 and until it is dismissed when negative. OnClose
// runs once it has gone, whatever closed it. A toast with the ID of one
// shown replaces it.
type ToastContent struct {
	ID          string
	Type        ToastType
	Title       string
	Description string
	Leading     core.Node
	Action      string
	OnAction    func()
	OnClose     func()
	Duration    time.Duration
}

type toastEntry = toast.Entry[string, ToastContent]

const (
	toastLimit = 3                     // how many show at once by default
	toastTick  = 50 * time.Millisecond // how often the lifecycle advances while one shows
)

// Toaster keeps the app's toasts: the app makes one near its root with
// NewToaster, shows and dismisses toasts through it from anywhere, and
// places a Toasts of it once over its content. NSGo's ToastManager keeps
// their lifecycle; a toast's timeout pauses while the stack is open
// under the pointer or a finger. Limit is how many show at once, 3 when
// 0, older ones waiting behind them.
type Toaster struct {
	Limit    int
	cx       *core.Scope
	manager  toast.Manager[string, ToastContent]
	shown    *core.Signal[[]toastEntry]
	expanded *core.Signal[bool]
	ticking  bool
	count    int
}

// NewToaster makes a Toaster where cx is.
func NewToaster(cx *core.Scope) *Toaster {
	return &Toaster{cx: cx, shown: core.NewSignalFunc(cx, []toastEntry(nil), func(a, b []toastEntry) bool { return false }),
		expanded: core.NewSignal(cx, false)}
}

// Show shows c and returns its ID, made up when c has none.
func (t *Toaster) Show(c ToastContent) string {
	if c.ID == "" {
		t.count++
		c.ID = fmt.Sprintf("toast-%d", t.count)
	}
	if c.Duration == 0 {
		c.Duration = 5 * time.Second
	}
	if old, ok := t.manager.Push(c.ID, c, max(c.Duration, 0), t.cx.Now()); ok && old.OnClose != nil {
		old.OnClose()
	}
	t.run()
	return c.ID
}

// Dismiss closes the toast id.
func (t *Toaster) Dismiss(id string) {
	t.manager.Dismiss(id, t.cx.Now())
	t.run()
}

// DismissAll closes every toast.
func (t *Toaster) DismissAll() {
	t.manager.DismissAll(t.cx.Now())
	t.run()
}

// visible is the toasts to show.
func (t *Toaster) visible() []toastEntry {
	limit := t.Limit
	if limit == 0 {
		limit = toastLimit
	}
	return t.manager.Visible(limit)
}

func (t *Toaster) run() {
	t.shown.Set(t.visible())
	if !t.ticking {
		t.ticking = true
		t.cx.AfterFunc(toastTick, t.tick)
	}
}

func (t *Toaster) tick() {
	a := t.manager.Advance(t.cx.Now(), t.expanded.GetUntracked())
	if a.Changed {
		t.shown.Set(t.visible())
	}
	for _, gone := range a.Removed {
		if gone.Value.OnClose != nil {
			gone.Value.OnClose()
		}
	}
	if t.manager.Len() == 0 {
		t.ticking = false
		return
	}
	t.cx.AfterFunc(toastTick, t.tick)
}

// ToastPlacement is the edge and side of the window toasts stack at;
// the newest is nearest the edge. The zero value is the top center.
type ToastPlacement uint8

const (
	ToastTop ToastPlacement = iota
	ToastTopStart
	ToastTopEnd
	ToastBottom
	ToastBottomStart
	ToastBottomEnd
	// ToastStart and ToastEnd are the middle of the window's sides.
	ToastStart
	ToastEnd
)

func (p ToastPlacement) top() bool { return p <= ToastTopEnd }

func (p ToastPlacement) side() bool { return p >= ToastStart }

// anchor and align are the stack's anchor and where it sits across.
func (p ToastPlacement) anchor() (toast.Anchor, core.Align) {
	switch p {
	case ToastTopStart:
		return toast.AnchorTopLeft, core.AlignStart
	case ToastTopEnd:
		return toast.AnchorTopRight, core.AlignEnd
	case ToastBottom:
		return toast.AnchorBottomCenter, core.AlignCenter
	case ToastBottomStart:
		return toast.AnchorBottomLeft, core.AlignStart
	case ToastBottomEnd:
		return toast.AnchorBottomRight, core.AlignEnd
	case ToastStart:
		return toast.AnchorLeftCenter, core.AlignStart
	case ToastEnd:
		return toast.AnchorRightCenter, core.AlignEnd
	}
	return toast.AnchorTopCenter, core.AlignCenter
}

// ToastSwipe is the way toasts are swiped away. The zero value is toward
// the nearest edge of the window.
type ToastSwipe uint8

const (
	ToastSwipeEdge ToastSwipe = iota
	ToastSwipeLeft
	ToastSwipeRight
	ToastSwipeUp
	ToastSwipeDown
	// ToastSwipeOff keeps them from being swiped.
	ToastSwipeOff
)

// direction is the swipe of toasts at p, in a layout right to left or not.
func (s ToastSwipe) direction(p ToastPlacement, rtl bool) toast.SwipeDirection {
	switch s {
	case ToastSwipeLeft:
		return toast.SwipeLeft
	case ToastSwipeRight:
		return toast.SwipeRight
	case ToastSwipeUp:
		return toast.SwipeUp
	case ToastSwipeDown:
		return toast.SwipeDown
	case ToastSwipeOff:
		return toast.SwipeOff
	}
	switch {
	case p.side() && (p == ToastStart) != rtl:
		return toast.SwipeLeft
	case p.side():
		return toast.SwipeRight
	case p.top():
		return toast.SwipeUp
	}
	return toast.SwipeDown
}

// Toasts is Onevra's toasts, a Toaster's, over the window inside the safe
// area: glass cards up to 360 points wide, the newest in front and older
// ones tucked behind it, fanning out under the pointer or a finger.
// Reverse puts the newest farthest from the edge, as a list growing away
// from it. Swiping one away, Swipe's way, or Escape closes it. Inset adds
// room at the placement's edge, as for a shell's header. CloseLabel names
// each toast's close button for assistive technology.
type Toasts struct {
	Toaster    *Toaster
	Placement  ToastPlacement
	Reverse    bool
	Swipe      ToastSwipe
	Inset      core.Points
	CloseLabel string
	TestID     string
}

// Render implements core.Node.
func (ts Toasts) Render(cx *core.Scope) core.Node {
	t := ts.Toaster
	anchor, align := ts.Placement.anchor()
	// An overlay sizes to its content: the window's width gives align room.
	place := core.Style{Left: core.Pt(0), Width: core.Pct(100), AlignItems: align, Padding: core.All(16)}
	switch {
	case ts.Placement.side():
		place.Top, place.Height, place.JustifyContent = core.Pt(0), core.Pct(100), core.JustifyCenter
		place.PaddingEnv = core.EnvEdges{Top: core.EnvSafeArea, Bottom: core.EnvSafeArea}
	case ts.Placement.top():
		place.Top, place.Padding.Top, place.PaddingEnv = core.Pt(0), 16+ts.Inset, core.EnvEdges{Top: core.EnvSafeArea}
	default:
		place.Bottom, place.Padding.Bottom, place.PaddingEnv = core.Pt(0), 16+ts.Inset, core.EnvEdges{Bottom: core.EnvSafeArea}
	}
	return core.Overlay{PointerEvents: core.PointerEventsBoxNone, Style: place,
		Children: []core.Node{toast.Stack[toastEntry, string]{TestID: ts.TestID, Placement: anchor, Reverse: ts.Reverse,
			Motion: toast.Motion{CollapsedPeek: 14, ExpandedGap: 8},
			Toasts: t.shown.Get, Key: func(e toastEntry) string { return e.ID },
			Toast: func(cx *core.Scope, e func() toastEntry) core.Node {
				return ts.item(cx, e)
			},
			Style:            core.Style{Width: core.Pt(360), MaxWidth: core.Pct(100)},
			OnExpandedChange: t.expanded.Set,
			OnCancel: func() {
				shown := t.shown.GetUntracked()
				for i := len(shown) - 1; i >= 0; i-- {
					if shown[i].Status != toast.Ending {
						t.Dismiss(shown[i].ID)
						return
					}
				}
			}}}}
}

// item is one toast.
func (ts Toasts) item(cx *core.Scope, e func() toastEntry) core.Node {
	t := ts.Toaster
	colors := ColorsOf(cx)
	first := core.Untracked(cx, e)
	id, c := first.ID, first.Value
	label := strings.Join(slices.DeleteFunc([]string{c.Title, c.Description}, func(s string) bool { return s == "" }), ". ")
	texts := []core.Node{core.Text{Content: c.Title, Style: TypeLabel, StyleFunc: Colored(colors.Text)}}
	if c.Description != "" {
		texts = append(texts, core.Text{Content: c.Description, Style: TypeCaption, StyleFunc: Colored(colors.Muted)})
	}
	var children []core.Node
	if lead := toastLeading(c); lead != nil {
		children = append(children, core.View{Style: core.Style{FlexShrink: 0}, Children: []core.Node{lead}})
	}
	children = append(children, core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), Gap: 2}, Children: texts})
	if c.Action != "" {
		children = append(children, Button{Text: c.Action, Variant: ButtonSecondary, TestID: id + "-action", OnPress: func() {
			if c.OnAction != nil {
				c.OnAction()
			}
			t.Dismiss(id)
		}})
	}
	children = append(children, IconButton{Icon: lucide.X, Label: ts.CloseLabel, Size: 24, TestID: id + "-close", OnPress: func() { t.Dismiss(id) }})
	align := core.AlignCenter
	if c.Description != "" {
		align = core.AlignStart
	}
	swipe := ts.Swipe.direction(ts.Placement, core.Untracked(cx, cx.Env().RTL))
	var enter core.Transform
	switch {
	case ts.Placement.side() && swipe == toast.SwipeRight:
		enter.TranslateX = 16
	case ts.Placement.side():
		enter.TranslateX = -16
	case ts.Placement.top():
		enter.TranslateY = -16
	default:
		enter.TranslateY = 16
	}
	return toast.Toast{TestID: id, Label: label, SwipeDirection: swipe,
		Status:           func() toast.Status { return e().Status },
		OnDismissRequest: func() { t.Dismiss(id) },
		StartingStyle:    core.Style{Opacity: core.Alpha(0), Transform: enter},
		EndingStyle:      core.Style{Opacity: core.Alpha(0), Transform: core.Transform{Scale: 0.96}},
		SwipingStyle:     core.Style{Transition: core.Transition{}},
		StyleFunc: func() core.Style {
			glass := FinOf(cx).Glass()
			return core.Style{Direction: core.Row, AlignItems: align, Gap: 12,
				Padding: core.Edges{Top: 12, Right: 8, Bottom: 12, Left: 16}, CornerRadius: RadiusCard, BorderCurve: core.BorderCurveContinuous,
				BorderWidth: 1, BorderColor: colors.Border(), Background: Alpha(colors.Background(), 0.92), Color: colors.Text(),
				Shadows: glass.Shadows, BackdropFilters: glass.Backdrop,
				Transition: core.Transition{Duration: 200 * time.Millisecond, Easing: core.EaseOut}}
		},
		Children: children}
}

// toastLeading is the app's leading content or the type's status icon.
func toastLeading(c ToastContent) core.Node {
	if c.Leading != nil {
		return c.Leading
	}
	var name func() *core.SVG
	var tint func(Colors) func() core.Color
	switch c.Type {
	case ToastInfo:
		name, tint = lucide.Info, func(c Colors) func() core.Color { return c.Accent }
	case ToastSuccess:
		name, tint = lucide.CircleCheck, func(c Colors) func() core.Color { return c.Up }
	case ToastWarning:
		name, tint = lucide.TriangleAlert, func(c Colors) func() core.Color { return c.Warn }
	case ToastError:
		name, tint = lucide.CircleX, func(c Colors) func() core.Color { return c.Danger }
	default:
		return nil
	}
	return core.Func(func(cx *core.Scope) core.Node {
		return core.View{Style: core.Style{Height: core.Pt(18), JustifyContent: core.JustifyCenter},
			Children: []core.Node{Icon(name, 18, tint(ColorsOf(cx)))}}
	})
}
