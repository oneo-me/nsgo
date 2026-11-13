package ui

import (
	"time"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/desktop"
)

// Dialog is a modal surface over a dimmed app and a sheet from the bottom
// on touch screens: a title, a description, the app's children and its
// Footer of actions. On desktops it pops in and fades out
// does, centered across and with two thirds of the spare height below it.
type Dialog struct {
	Open         func() bool
	OnOpenChange func(open bool, reason base.Reason)
	// OnClose follows it being gone, once its motion ended.
	OnClose     func()
	Title       string
	Description string
	Children    []core.Node
	// Footer holds the actions, at the end of a row on desktops, the
	// last first in a stack across a sheet.
	Footer                 []core.Node
	NoCloseOnEscape        bool
	NoCloseOnBackdropPress bool
	// Inset is the least distance from the window's edges on desktops, 20
	// when 0; a title bar the app draws in takes it further. The strip of
	// that title bar keeps moving the window under the backdrop.
	Inset core.Points
	// Bare leaves out the surface's padding, gap and width, for content
	// that fills it, sizes it and is clipped to its corners.
	Bare bool
	// Label names the dialog for assistive technology, Title when empty,
	// which reads Description after it. Role makes it an alert dialog,
	// one asking for an answer, as a confirmation.
	Label  string
	Role   core.Role
	TestID string
	// Style merges over the surface's.
	Style core.Style
}

// Render implements core.Node.
func (d Dialog) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	label := d.Label
	if label == "" {
		label = d.Title
	}
	var body []core.Node
	if d.Title != "" || d.Description != "" {
		var head []core.Node
		if d.Title != "" {
			head = append(head, core.Text{Content: d.Title, Role: core.RoleHeader, Style: TypeTitle, StyleFunc: Colored(colors.Text)})
		}
		if d.Description != "" {
			head = append(head, core.Text{Content: d.Description, Style: TypeBody, StyleFunc: Colored(colors.Muted)})
		}
		body = append(body, core.View{Style: core.Style{Gap: 4}, Children: head})
	}
	body = append(body, d.Children...)
	if Touch(cx) {
		if d.Footer != nil {
			body = append(body, core.View{Style: core.Style{Direction: core.ColumnReverse, Gap: 8, Margin: core.Edges{Top: 8}},
				Children: d.Footer})
		}
		return base.Sheet{Open: d.Open, OnOpenChange: d.OnOpenChange, NoCloseOnEscape: d.NoCloseOnEscape,
			NoCloseOnBackdropPress: d.NoCloseOnBackdropPress, CloseOnSwipe: !d.NoCloseOnBackdropPress,
			BackdropStyle: scrim, PopupStyleFunc: func() core.Style { return sheetPanel(cx, core.PlaceBottom).Merge(d.Style) },
			Children: sheetContent(cx, core.PlaceBottom, body), Label: label, TestID: d.TestID}
	}
	if d.Footer != nil {
		body = append(body, core.View{Style: core.Style{Direction: core.Row, Wrap: core.Wrap, JustifyContent: core.JustifyEnd,
			Gap: 8, Margin: core.Edges{Top: 8}}, Children: d.Footer})
	}
	popup := core.Style{Width: core.Pt(440), Padding: core.All(20), Gap: 12, Transition: dialogPop}
	if d.Bare {
		// The clip is a view of its own: on Apple's targets a view that
		// clips loses its outer shadow.
		popup = core.Style{Transition: dialogPop}
		body = []core.Node{core.View{Style: core.Style{Overflow: core.Hidden, CornerRadius: RadiusDialog - 1, BorderCurve: core.BorderCurveContinuous,
			FlexGrow: 1, FlexShrink: 1, MinHeight: core.Pt(0)}, Children: body}}
	}
	spacer := func(grow float32) core.View {
		return core.View{Style: core.Style{FlexGrow: grow, MinHeight: core.Pt(0)}, PointerEvents: core.PointerEventsNone}
	}
	return base.Dialog{Open: d.Open, OnOpenChange: d.OnOpenChange, OnClose: d.OnClose, Animation: core.ModalNone, NoCloseOnEscape: d.NoCloseOnEscape,
		Role: d.Role, AccessibilityDescription: d.Description,
		NoCloseOnBackdropPress: d.NoCloseOnBackdropPress, BackdropStyle: core.Style{Background: Black(0.18), Transition: dialogFade},
		BackdropStartingStyle: core.Style{Opacity: core.Alpha(0)}, BackdropEndingStyle: core.Style{Opacity: core.Alpha(0)},
		PopupStyle: popup, StartingStyle: dialogStarting, EndingStyle: dialogEnding,
		PopupStyleFunc: func() core.Style {
			window, inset := cx.Env().Window(), dialogInset(cx, d.Inset)
			finish := FinOf(cx).Dialog()
			return finish.Over(core.Style{Background: Alpha(finish.Background, 0.96), Color: colors.Text(), CornerRadius: RadiusDialog, BorderCurve: core.BorderCurveContinuous, BorderWidth: 1,
				MaxWidth: core.Pt(max(0, window.Width-2*inset)), MaxHeight: core.Pt(max(0, window.Height-2*inset)), FlexShrink: 0}).Merge(d.Style)
		},
		LayerStyle: core.Style{JustifyContent: core.JustifyStart}, Before: []core.Node{spacer(1)}, After: []core.Node{spacer(2), dialogDragStrip(cx)},
		Children: body, Label: label, TestID: d.TestID}
}

// RadiusDialog rounds a dialog on desktops.
const RadiusDialog core.Points = 16

// A dialog's layers move so: the dim fades, and the surface pops
// in and fades out where it is. Under reduced motion they come and go at
// once.
var (
	dialogFade = core.Transition{Duration: 180 * time.Millisecond, Easing: core.EaseOut, Properties: core.TransitionOpacity}
	dialogPop  = core.Transition{Duration: 180 * time.Millisecond, Easing: core.EaseOut,
		Properties: core.TransitionOpacity | core.TransitionTransform}
	dialogStarting = core.Style{Opacity: core.Alpha(0), Transform: core.Transform{Scale: 0.96, TranslateY: 8}}
	dialogEnding   = core.Style{Opacity: core.Alpha(0), Transform: core.Transform{Scale: 0.98}}
)

// dialogInset is how far a dialog stays from the window's edges: inset, 20
// when 0, at least the title bar the app draws in.
func dialogInset(cx *core.Scope, inset core.Points) core.Points {
	if inset <= 0 {
		inset = 20
	}
	return max(inset, desktop.TitleBar(cx).Top)
}

// dialogDragStrip keeps the title bar the app draws in moving the window
// under a dialog's backdrop.
func dialogDragStrip(cx *core.Scope) core.Node {
	return core.View{Desktop: core.DesktopViewProps{WindowDrag: core.WindowDragOn}, StyleFunc: func() core.Style {
		return core.Style{Position: core.Absolute, Top: core.Pt(0), Left: core.Pt(0), Right: core.Pt(0), Height: core.Pt(desktop.TitleBar(cx).Top)}
	}}
}
