package main

import (
	"time"

	"github.com/oneo-me/nsgo/base/toast"
	"github.com/oneo-me/nsgo/core"

	"github.com/oneo-me/nsgo/examples/gallery/locales"
	"github.com/oneo-me/nsgo/ui"
)

// What says how things stand: each ui form under its base form, with their
// props to set.

func progressCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	value := core.NewSignal(cx, 40.0)
	indeterminate := core.NewSignal(cx, false)
	percent := func() float32 { return float32(value.Get()) }
	return Playground{Title: "Progress", Fill: true, Base: baseProgressForm(cx, value, indeterminate), Stage: func() core.Node {
		return ui.Progress{Label: tr.LibProgress(), Value: percent, Indeterminate: indeterminate.Get}
	}, BaseKnobs: []core.Node{numberKnob("Value", value, 0, 100, 1), boolKnob("Indeterminate", indeterminate)}}
}

func progressRingCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	value, size := core.NewSignal(cx, 60.0), core.NewSignal(cx, 44.0)
	indeterminate := core.NewSignal(cx, false)
	return Playground{Title: "ProgressRing", Stage: func() core.Node {
		return ui.ProgressRing{Label: tr.LibProgress(), Size: core.Points(size.Get()), Indeterminate: indeterminate.Get,
			Value: func() float32 { return float32(value.Get()) }}
	}, Knobs: []core.Node{numberKnob("Value", value, 0, 100, 1), numberKnob("Size", size, 20, 72, 4), boolKnob("Indeterminate", indeterminate)}}
}

func spinnerPlayground(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	size := core.NewSignal(cx, 24.0)
	return Playground{Title: "Spinner", UIOnly: true, Stage: func() core.Node {
		s := core.Points(size.Get())
		return stageRow(ui.Spinner{Label: tr.LibLoading(), Variant: ui.SpinnerRing, Size: s}, ui.Spinner{Label: tr.LibLoading(), Variant: ui.SpinnerSpokes, Size: s})
	}, Knobs: []core.Node{numberKnob("Size", size, 12, 48, 4)}}
}

func meterCard(cx *core.Scope) core.Node {
	value, marker := core.NewSignal(cx, 0.6), core.NewSignal(cx, 0.8)
	variant := core.NewSignal(cx, ui.MeterAccent)
	return Playground{Title: "Meter", UIOnly: true, Fill: true, Stage: func() core.Node {
		return ui.Meter{Value: value.Get, Variant: variant.Get(), Marker: marker.Get()}
	}, Knobs: []core.Node{
		numberKnob("Value", value, 0, 1, 0.05), numberKnob("Marker", marker, 0, 1, 0.05),
		choiceKnob("Variant", variant, options([]ui.MeterVariant{ui.MeterAccent, ui.MeterInfo, ui.MeterWarn, ui.MeterDanger}, "Accent", "Info", "Warn", "Danger")...),
	}}
}

func skeletonCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	lines := core.NewSignal(cx, 3.0)
	avatar, loading := core.NewSignal(cx, true), core.NewSignal(cx, true)
	return Playground{Title: "SkeletonContent · Skeleton", UIOnly: true, Fill: true, Stage: func() core.Node {
		return ui.SkeletonContent{Loading: loading.Get, Placeholder: ui.Skeleton{Lines: int(lines.Get()), Avatar: avatar.Get(), Label: tr.LibLoading()},
			Children: []core.Node{ui.Text(tr.ControlsDeployedToProduction(), 13, ui.Regular, colors.Text)}}
	}, Knobs: []core.Node{numberKnob("Lines", lines, 1, 6, 1), boolKnob("Avatar", avatar), boolKnob("Loading", loading)}}
}

// secondsOf is s seconds.
func secondsOf(s float64) time.Duration { return time.Duration(s * float64(time.Second)) }

// toastPlayground sets how the gallery's toasts show, for all of them, and
// shows one with what it holds.
func toastPlayground(cx *core.Scope) core.Node {
	last := core.NewSignal(cx, "")
	tr := locales.For(cx)
	kind := core.NewSignal(cx, ui.ToastSuccess)
	seconds := core.NewSignal(cx, 5.0)
	description, action, sticky := core.NewSignal(cx, true), core.NewSignal(cx, false), core.NewSignal(cx, false)
	swipe := core.NewSignal(cx, toast.SwipeRight)
	limit := core.NewSignal(cx, float64(shell.toasts.Limit))
	if limit.GetUntracked() == 0 {
		limit.Set(3)
	}
	cx.Effect(func() { shell.toasts.Limit = int(limit.Get()) })
	show := func() {
		c := ui.ToastContent{Type: kind.GetUntracked(), Title: tr.LibChangesSaved()}
		if description.GetUntracked() {
			c.Description = tr.LibThreeFiles()
		}
		undone := false
		if action.GetUntracked() {
			c.Action = tr.OverlayUndo()
			c.OnAction = func() { undone = true }
		}
		c.OnClose = func() {
			if undone {
				last.Set(tr.OverlayToastUndone())
			} else {
				last.Set(tr.OverlayToastGone())
			}
		}
		c.Duration = secondsOf(seconds.GetUntracked())
		if sticky.GetUntracked() {
			c.Duration = -1
		}
		shell.toasts.Show(c)
	}
	return Playground{Title: "Toasts · ToastContent", Event: last.Get, Base: baseToastForm(cx, last, swipe), Stage: func() core.Node {
		return ui.HStack{Gap: 8, Children: []core.Node{
			ui.Button{Text: tr.KitShow(), TestID: "toast-show", OnPress: show},
			ui.Button{Text: tr.OverlayDismissAll(), Variant: ui.ButtonGhost, OnPress: shell.toasts.DismissAll},
		}}
	}, BaseKnobs: []core.Node{
		choiceKnob("SwipeDirection", swipe, options([]toast.SwipeDirection{toast.SwipeRight, toast.SwipeLeft, toast.SwipeOff}, "Right", "Left", "Off")...),
	}, Knobs: []core.Node{
		choiceKnob("Placement", shell.toastPlacement, options([]ui.ToastPlacement{ui.ToastTopStart, ui.ToastTop, ui.ToastTopEnd, ui.ToastStart, ui.ToastEnd,
			ui.ToastBottomStart, ui.ToastBottom, ui.ToastBottomEnd}, "TopStart", "Top", "TopEnd", "Start", "End", "BottomStart", "Bottom", "BottomEnd")...),
		boolKnob("Reverse", shell.toastReverse),
		choiceKnob("Swipe", shell.toastSwipe, options([]ui.ToastSwipe{ui.ToastSwipeEdge, ui.ToastSwipeLeft, ui.ToastSwipeRight, ui.ToastSwipeUp, ui.ToastSwipeDown, ui.ToastSwipeOff},
			"Edge", "Left", "Right", "Up", "Down", "Off")...),
		numberKnob("Limit", limit, 1, 6, 1),
		choiceKnob("Type", kind, options([]ui.ToastType{ui.ToastPlain, ui.ToastInfo, ui.ToastSuccess, ui.ToastWarning, ui.ToastError},
			"Plain", "Info", "Success", "Warning", "Error")...),
		numberKnob("Duration", seconds, 1, 15, 1),
		boolKnob("Duration < 0", sticky), boolKnob("Description", description), boolKnob("Action", action),
	}}
}
