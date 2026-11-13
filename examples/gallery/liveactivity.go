package main

import (
	"fmt"
	"math"
	"strings"
	"sync"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/i18n"
	"github.com/nsgo-dev/nsgo/icons/lucide"
	"github.com/nsgo-dev/nsgo/ui"
	"github.com/nsgo-dev/nsgo/widget"

	"github.com/nsgo-dev/nsgo/examples/gallery/locales"
)

// The Widgets demo's Live Activity walks the widgets' steps on the Lock
// Screen and in the Dynamic Island, or as a Live Update on Android. Its
// buttons add steps and end it; each names the whole state it leads to,
// so a press needs nothing of the gallery's views, which do not run when
// iOS starts the app in the background for it.
const walkKind = "walk"

// walkStep is how many steps a press of More adds.
const walkStep = 500

// walkAction is the ID of a button that shows s in language lang, "end"
// for the one that ends the walk.
func walkAction(lang string, s widgetState) string {
	return fmt.Sprintf("walk:%s:%d:%d:%d", lang, s.steps, s.goal, s.accent)
}

// parseWalk reads a walkAction.
func parseWalk(action string) (lang string, s widgetState, ok bool) {
	rest, ok := strings.CutPrefix(action, "walk:")
	if !ok {
		return "", s, false
	}
	lang, nums, _ := strings.Cut(rest, ":")
	_, err := fmt.Sscanf(nums, "%d:%d:%d", &s.steps, &s.goal, &s.accent)
	return lang, s, err == nil
}

// walkContent is what the activity shows of s: the steps toward the goal
// in a bar, with the share in the status bar's chip and beside the camera.
func walkContent(tr locales.Messages, lang string, s widgetState) widget.ActivityContent {
	move, _, _ := s.rings()
	more := s
	more.steps += walkStep
	return widget.ActivityContent{
		Title: tr.WidgetsSteps(s.steps), Body: tr.WidgetsOfGoal(s.goal), Symbol: "figure.walk",
		Status:       tr.WidgetsPercent(int(math.Round(move * 100))),
		ShowProgress: true, Progress: move, Tint: s.color(),
		Actions: []widget.ActivityAction{
			{ID: walkAction(lang, more), Title: tr.WidgetsSteps(walkStep), Symbol: "plus"},
			{ID: "end", Title: tr.WidgetsEndActivity(), Symbol: "stop.fill"},
		},
	}
}

// walks hands the presses of the activity's buttons to the demo while it
// shows; without it they change the activity alone.
var walks struct {
	sync.Mutex
	pressed func(s widgetState, ended bool)
}

// walkPressed carries out a press of the activity's buttons: it updates or
// ends the activity before it returns, as iOS holds it until then.
func walkPressed(p widget.ActivityPress) {
	if p.Activity.Kind() != walkKind {
		return
	}
	ended := p.Action == "end"
	lang, s, ok := parseWalk(p.Action)
	switch {
	case ended:
		p.Activity.End(nil, widget.DismissNow)
	case ok:
		p.Activity.Update(walkContent(locales.In(lang), lang, s))
	default:
		return
	}
	walks.Lock()
	fn := walks.pressed
	walks.Unlock()
	if fn != nil {
		fn(s, ended)
	}
}

// liveActivityCard starts and ends the walk, which follows the demo's
// steps, goal and accent while it runs.
func liveActivityCard(cx *core.Scope, steps, goal *core.Signal[float64], accent *core.Signal[int], state func() widgetState) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	lang := i18n.Language(cx)
	var running *widget.Activity
	for _, a := range widget.Activities() {
		if a.Kind() == walkKind {
			running = a
		}
	}
	activity := core.NewSignal(cx, running)
	status := core.NewSignal(cx, "")
	walks.Lock()
	walks.pressed = func(s widgetState, ended bool) {
		cx.Post(func() {
			if ended {
				activity.Set(nil)
				return
			}
			steps.Set(float64(s.steps))
			goal.Set(float64(s.goal))
			accent.Set(s.accent)
		})
	}
	walks.Unlock()
	cx.Cleanup(func() {
		walks.Lock()
		walks.pressed = nil
		walks.Unlock()
	})
	cx.Effect(func() {
		s := state()
		a := activity.Get()
		if a == nil {
			return
		}
		if err := a.Update(walkContent(tr, lang, s)); err != nil {
			activity.Set(nil)
		}
	})
	toggle := func() {
		if a := activity.Get(); a != nil {
			a.End(nil, widget.DismissNow)
			activity.Set(nil)
			status.Set("")
			return
		}
		a, err := widget.StartActivity(walkKind, walkContent(tr, lang, state()))
		if err != nil {
			status.Set(err.Error())
			return
		}
		status.Set("")
		activity.Set(a)
	}
	return Card{Title: tr.WidgetsLiveActivity(), Description: tr.WidgetsLiveActivityDetail(), Children: []core.Node{
		core.Dynamic(func(*core.Scope) core.Node {
			text, variant := tr.WidgetsStartActivity(), ui.ButtonPrimary
			if activity.Get() != nil {
				text, variant = tr.WidgetsEndActivity(), ui.ButtonSecondary
			}
			return core.View{Style: core.Style{Direction: core.Row}, Children: []core.Node{
				ui.Button{Text: text, Variant: variant, TestID: "activity-toggle", OnPress: toggle},
			}}
		}),
		core.Dynamic(func(*core.Scope) core.Node {
			if status.Get() == "" {
				return nil
			}
			return core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 6}, Children: []core.Node{
				ui.Icon(lucide.CircleAlert, 14, colors.Danger),
				core.Text{ContentFunc: status.Get, TestID: "activity-status", Style: core.Style{FlexGrow: 1, FlexShrink: 1, FontSize: 12}, StyleFunc: ui.Colored(colors.Muted)},
			}}
		}),
		ui.Text(tr.WidgetsLiveActivityHow(), 12, ui.Regular, colors.Faint),
	}}
}
