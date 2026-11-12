package main

import (
	"fmt"
	"strconv"

	baseflow "github.com/nsgo-dev/nsgo/base/flow"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/icons/lucide"

	"github.com/nsgo-dev/nsgo/examples/gallery/locales"
	"github.com/nsgo-dev/nsgo/ui"
	uiflow "github.com/nsgo-dev/nsgo/ui/flow"
)

type flowKind uint8

const (
	flowTask flowKind = iota
	flowExperiment
	flowFinding
)

type flowStatus uint8

const (
	flowNoStatus flowStatus = iota
	flowDone
	flowRunning
	flowPlanned
	flowWaiting
)

type flowText = func(locales.Messages) string

// flowStep is a step of the sample plan.
type flowStep struct {
	id            string
	kind          flowKind
	status        flowStatus
	title, detail flowText
	experiment    int
	notes, files  int
	// checked of checks shows as progress in the card's body.
	checked, checks int
	muted           bool
}

type flowLink struct {
	from, to string
	dashed   bool
	label    flowText
}

type fm = locales.Messages

var flowSteps = []flowStep{
	{id: "goal", status: flowRunning, title: fm.FlowGoal, detail: fm.FlowGoalDetail, notes: 3},
	{id: "audit", status: flowDone, title: fm.FlowAudit, detail: fm.FlowAuditDetail, notes: 2, files: 1},
	{id: "content", status: flowDone, title: fm.FlowContent, detail: fm.FlowContentDetail, notes: 1},
	{id: "navigation", kind: flowExperiment, status: flowRunning, title: fm.FlowNavigation, detail: fm.FlowNavigationDetail, experiment: 1},
	{id: "speed", status: flowDone, title: fm.FlowSpeed, detail: fm.FlowSpeedDetail, files: 2},
	{id: "menus", kind: flowFinding, title: fm.FlowMenus, detail: fm.FlowMenusDetail},
	{id: "retest", status: flowWaiting, title: fm.FlowRetest, detail: fm.FlowRetestDetail, muted: true},
	{id: "images", kind: flowFinding, title: fm.FlowImages, detail: fm.FlowImagesDetail},
	{id: "review", status: flowRunning, title: fm.FlowReview, detail: fm.FlowReviewDetail, notes: 1, checked: 3, checks: 5},
	{id: "ready", kind: flowFinding, title: fm.FlowReady, detail: fm.FlowReadyDetail},
	{id: "notes", status: flowPlanned, title: fm.FlowNotes, detail: fm.FlowNotesDetail, muted: true},
}

var flowLinks = []flowLink{
	{from: "goal", to: "audit"},
	{from: "goal", to: "content"},
	{from: "goal", to: "speed"},
	{from: "goal", to: "review"},
	{from: "audit", to: "navigation", dashed: true, label: fm.FlowInforms},
	{from: "audit", to: "speed", label: fm.FlowDependsOn},
	{from: "navigation", to: "menus"},
	{from: "navigation", to: "retest", dashed: true},
	{from: "speed", to: "images"},
	{from: "content", to: "review"},
	{from: "menus", to: "review", dashed: true},
	{from: "images", to: "review", dashed: true, label: fm.FlowInforms},
	{from: "review", to: "ready"},
	{from: "review", to: "notes", dashed: true},
}

// flowAddition is a step added after another, to watch the layout adapt.
type flowAddition struct{ id, after string }

// flowSample maps the plan, and the steps added to it, onto diagram data.
func flowSample(tr locales.Messages, added []flowAddition) uiflow.Data {
	var data uiflow.Data
	add := func(step flowStep, title, detail string) {
		data.Nodes = append(data.Nodes, uiflow.Node{ID: step.id, Label: title, Muted: step.muted,
			Content: flowCard{step: step, title: title, detail: detail}})
	}
	for _, step := range flowSteps {
		add(step, step.title(tr), step.detail(tr))
	}
	for _, l := range flowLinks {
		e := uiflow.Edge{Source: l.from, Target: l.to, Dashed: l.dashed}
		if l.label != nil {
			e.Label = l.label(tr)
		}
		data.Edges = append(data.Edges, e)
	}
	for i, a := range added {
		add(flowStep{id: a.id, status: flowPlanned}, tr.FlowNewStep(i+1), tr.FlowNewStepDetail())
		data.Edges = append(data.Edges, uiflow.Edge{Source: a.after, Target: a.id})
	}
	return data
}

// flowCard draws a step as a card with its status, kind and activity.
type flowCard struct {
	step          flowStep
	title, detail string
}

// Render implements core.Node.
func (f flowCard) Render(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	step := f.step
	tile, icon := colors.Accent, lucide.ListChecks
	switch step.kind {
	case flowExperiment:
		tile, icon = colors.Warn, lucide.FlaskConical
	case flowFinding:
		tile, icon = colors.Up, lucide.Lightbulb
	}
	if step.id == "goal" {
		icon = lucide.Target
	}
	card := uiflow.Card{Title: f.title, Description: f.detail,
		Leading: core.View{Style: core.Style{Width: core.Pt(30), Height: core.Pt(30), CornerRadius: 8, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
			StyleFunc: func() core.Style { return core.Style{Background: tile().Faded(.16)} },
			Children:  []core.Node{ui.Icon(icon, 16, tile)}}}
	if step.checks > 0 {
		card.Body = core.Group{
			core.Text{Content: f.title, MaxLines: 1, Style: ui.TypeBody.Merge(core.Style{FontWeight: ui.Semibold}), StyleFunc: ui.Colored(colors.Text)},
			core.Text{Content: f.detail, MaxLines: 1, Style: core.Style{FontSize: 11, LineHeight: 15}, StyleFunc: ui.Colored(colors.Muted)},
			core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, Padding: core.Edges{Top: 6}}, Children: []core.Node{
				ui.Progress{Value: func() float32 { return 100 * float32(step.checked) / float32(step.checks) }, Style: core.Style{FlexGrow: 1}},
				ui.Text(fmt.Sprintf("%d/%d", step.checked, step.checks), 10, ui.Regular, colors.Muted),
			}},
		}
	}
	if step.status == flowRunning {
		card.Variant = uiflow.CardAccent
	}
	if step.status != flowNoStatus {
		color, icon, label := colors.Up, lucide.CircleCheck, tr.FlowDone()
		switch step.status {
		case flowRunning:
			color, icon, label = colors.Accent, lucide.LoaderCircle, tr.FlowRunning()
		case flowPlanned:
			color, icon, label = colors.Muted, lucide.CircleDashed, tr.FlowPlanned()
		case flowWaiting:
			color, icon, label = colors.Warn, lucide.Clock, tr.FlowWaiting()
		}
		header := core.Group{
			core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 4}, Children: []core.Node{
				ui.Icon(icon, 12, color), ui.Text(label, 11, ui.Medium, color)}},
			ui.Spacer{},
		}
		if step.experiment > 0 {
			header = append(header, ui.Text(tr.FlowExperiment(step.experiment), 11, ui.Regular, colors.Muted))
		}
		card.Header = header
	}
	if step.notes > 0 || step.files > 0 {
		var counts core.Group
		count := func(icon func() *core.SVG, n int) {
			if n > 0 {
				counts = append(counts, core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 4}, Children: []core.Node{
					ui.Icon(icon, 12, colors.Muted), ui.Text(strconv.Itoa(n), 11, ui.Regular, colors.Muted)}})
			}
		}
		count(lucide.MessageSquare, step.notes)
		count(lucide.Paperclip, step.files)
		card.Footer = counts
	}
	return card
}

// flowShown is the camera of the page shown last, which tests read.
var flowShown *uiflow.State

// flowDemo is the plan filling the page, its toolbar over it, with its
// options, what it reports and its base form in the panel.
func flowDemo(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	fin := ui.FinOf(cx)
	state := &uiflow.State{}
	flowShown = state
	selected, opened := core.NewSignal(cx, ""), core.NewSignal(cx, "")
	leftToRight, empty := core.NewSignal(cx, false), core.NewSignal(cx, false)
	added := core.NewSignalFunc(cx, []flowAddition(nil), func(a, b []flowAddition) bool { return len(a) == len(b) })
	data := func() uiflow.Data {
		if empty.Get() {
			return uiflow.Data{}
		}
		return flowSample(locales.For(cx), added.Get())
	}
	direction := func() uiflow.Direction {
		if leftToRight.Get() {
			return uiflow.LeftToRight
		}
		return uiflow.TopToBottom
	}
	tool := func(icon func() *core.SVG, label string, enabled func() bool, press func()) core.Node {
		return ui.IconButton{Icon: icon, Label: label, OnPress: press, Disabled: func() bool { return enabled != nil && !enabled() }}
	}
	full := func() bool { return !empty.Get() }
	toolbar := core.View{Style: core.Style{Direction: core.Row, Gap: 2, Padding: core.All(4), CornerRadius: ui.RadiusControl + 4, BorderWidth: 1},
		StyleFunc: func() core.Style {
			return fin.Raised().Over(core.Style{Background: colors.Surface(), BorderColor: colors.Border()})
		},
		Children: []core.Node{
			tool(lucide.Plus, tr.FlowAdd(), full, func() {
				// A step follows the selected one, or the previous addition.
				list := added.GetUntracked()
				after := "review"
				if n := len(list); n > 0 {
					after = list[n-1].id
				}
				if s := selected.GetUntracked(); s != "" {
					after = s
				}
				id := fmt.Sprintf("added-%d", len(list)+1)
				added.Set(append(list[:len(list):len(list)], flowAddition{id: id, after: after}))
				selected.Set(id)
			}),
			core.Show{When: func() bool { return len(added.Get()) > 0 }, Then: func(*core.Scope) core.Node {
				return tool(lucide.RotateCcw, tr.FlowReset(), nil, func() {
					added.Set(nil)
					selected.Set("")
				})
			}},
			tool(lucide.ZoomOut, tr.FlowZoomOut(), full, state.ZoomOut),
			tool(lucide.ZoomIn, tr.FlowZoomIn(), full, state.ZoomIn),
			tool(lucide.Scan, tr.FlowFit(), full, state.Fit),
		}}
	return stagePage(cx,
		core.View{StyleFunc: func() core.Style {
			return fin.Card().Over(core.Style{FlexGrow: 1, FlexShrink: 1, MinHeight: core.Pt(0), CornerRadius: ui.RadiusInset,
				BorderWidth: 1, BorderColor: colors.Border(), Background: colors.Background(), Overflow: core.Hidden})
		}, Children: []core.Node{PixelGrid{Style: core.Style{FlexGrow: 1, MinHeight: core.Pt(0)}, Children: []core.Node{uiflow.View{TestID: "flow", State: state, Data: data, Direction: direction,
			LabelFunc: func() string {
				d := data()
				return tr.FlowLabel(len(d.Nodes), len(d.Edges))
			},
			EmptyText: tr.FlowEmpty(), Selected: selected.Get, OnSelect: selected.Set, OnOpen: opened.Set, Overlay: toolbar}}}}},
		Section{Title: tr.TerminalOptions(), Note: "flow.View", Children: []core.Node{
			SwitchRow{Label: tr.FlowLeftToRight(), Code: "LeftToRight", Checked: leftToRight.Get, OnChange: leftToRight.Set},
			SwitchRow{Label: tr.FlowEmptyState(), Code: "Empty", Checked: empty.Get, OnChange: empty.Set},
		}},
		Section{Title: tr.TerminalStatus(), Note: "OnSelect · OnOpen", Children: []core.Node{
			statusLine(cx, tr.FlowSelected(), "flow-selected", dashIfEmpty(selected.Get)),
			statusLine(cx, tr.FlowOpened(), "flow-opened", dashIfEmpty(opened.Get)),
		}},
		baseFlowForm(cx).panelStage(cx, "base"))
}

// dashIfEmpty is fn's text, a dash while it has none.
func dashIfEmpty(fn func() string) func() string {
	return func() string {
		if s := fn(); s != "" {
			return s
		}
		return "—"
	}
}

// baseFlowForm is base's diagram in its simplest form: plain boxes in the
// text's color and grays.
func baseFlowForm(cx *core.Scope) Form {
	colors := ui.ColorsOf(cx)
	box := func(title string) core.Node {
		return core.View{Style: core.Style{Padding: core.Symmetric(8, 12), BorderWidth: 1, CornerRadius: 6},
			StyleFunc: func() core.Style { return core.Style{BorderColor: colors.Border(), Background: colors.Surface()} },
			Children:  []core.Node{bareText(cx, title)}}
	}
	data := baseflow.Data{
		Nodes: []baseflow.Node{{ID: "survey", Label: "Survey", Content: box("Survey")}, {ID: "design", Label: "Design", Content: box("Design")},
			{ID: "prototype", Label: "Prototype", Content: box("Prototype")}, {ID: "review", Label: "Review", Content: box("Review")}},
		Edges: []baseflow.Edge{{Source: "survey", Target: "design"}, {Source: "survey", Target: "prototype", Dashed: true},
			{Source: "design", Target: "review"}, {Source: "prototype", Target: "review"}},
	}
	return Form{Name: "flow.View", Fill: true, Stage: func() core.Node {
		return core.Func(func(cx *core.Scope) core.Node {
			selected := core.NewSignal(cx, "")
			return baseflow.View{Data: func() baseflow.Data { return data }, Selected: selected.Get, OnSelect: selected.Set, NodeGap: 24, RankGap: 40,
				Style: core.Style{Height: core.Pt(220)}}
		})
	}}
}
