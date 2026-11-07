package main

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/base/toast"
	"github.com/oneo-me/nsgo/core"

	"github.com/oneo-me/nsgo/examples/gallery/locales"
	"github.com/oneo-me/nsgo/ui"
)

// The base forms: each component of package base as an app without package
// ui writes it, behavior and accessibility from base and every part's style
// spelled out here, above its ui form in the same Playground and set by the
// same signals. Each stays the simplest thing that shows the component, in
// the text's color and grays: color is ui's.

// bareBox is a plain pressable's shape.
var bareBox = core.Style{Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Gap: 6,
	MinHeight: core.Pt(30), Padding: core.Symmetric(0, 12), CornerRadius: 6, BorderWidth: 1, Disabled: &core.Style{Opacity: core.Alpha(0.4)}}

// bareRow is a row of a list or a tree, which has no hover or press.
var bareRow = core.Style{Direction: core.Row, AlignItems: core.AlignCenter, MinHeight: core.Pt(30), Padding: core.Symmetric(0, 8), CornerRadius: 6}

// bareInk is what a plain pressable shows in each of its states.
func bareInk(cx *core.Scope) func() core.Style {
	colors := ui.ColorsOf(cx)
	return func() core.Style {
		on := &core.Style{Background: colors.Pressed()}
		return core.Style{Color: colors.Text(), BorderColor: colors.Line(), Hover: &core.Style{Background: colors.Hover()}, Active: on,
			Selected: on, Pressed: on, Expanded: on, Focus: &core.Style{BorderColor: colors.Text()}}
	}
}

// bareRowInk is what a row shows: default, selected and focused.
func bareRowInk(cx *core.Scope) func() core.Style {
	colors := ui.ColorsOf(cx)
	return func() core.Style {
		return core.Style{Selected: &core.Style{Background: colors.Pressed()}, Focus: &core.Style{Background: colors.Hover()}}
	}
}

// bareField is a plain text field's look.
func bareField(cx *core.Scope) func() core.Style {
	colors := ui.ColorsOf(cx)
	return func() core.Style {
		return core.Style{Color: colors.Text(), BorderColor: colors.Line(), Focus: &core.Style{BorderColor: colors.Text()}}
	}
}

// barePopup is a plain popup's fill, over the page in both appearances.
func barePopup(cx *core.Scope) func() core.Style {
	colors := ui.ColorsOf(cx)
	return func() core.Style {
		return core.Style{Background: colors.Surface(), BorderColor: colors.Line(), Color: colors.Text()}
	}
}

// barePanel is a popup's shape around its content.
var barePanel = core.Style{Padding: core.All(12), Gap: 8, CornerRadius: 8, BorderWidth: 1, MinWidth: core.Pt(200), MaxWidth: core.Pt(320)}

// bareBackdrop dims what is under a modal.
var bareBackdrop = core.Style{Background: ui.Black(0.45)}

// bareIndicator marks the selected tab: a Tabs' indicator takes one style,
// which an appearance does not restyle, so a gray that reads on both.
var bareIndicator = core.RGB(0x808080)

// bareText is a base form's words.
func bareText(cx *core.Scope, s string) core.Text {
	return ui.Text(s, 13, ui.Regular, ui.ColorsOf(cx).Text)
}

// bareTextf is a base form's words that change.
func bareTextf(cx *core.Scope, s func() string) core.Text {
	return ui.Textf(s, 13, ui.Regular, ui.ColorsOf(cx).Text)
}

// bareButton is a base.Button with words, to open what a stage shows.
func bareButton(cx *core.Scope, text string, press func()) core.Node {
	return base.Button{Label: text, Style: bareBox, StyleFunc: bareInk(cx), OnPress: func(core.PressEvent) { press() },
		Children: []core.Node{bareText(cx, text)}}
}

func baseButtonForm(cx *core.Scope, last *core.Signal[string], selected, disabled *core.Signal[bool]) Form {
	tr := locales.For(cx)
	return Form{Name: "base.Button", Stage: func() core.Node {
		return base.Button{Label: tr.OverlaySave(), Selected: selected.Get, Disabled: disabled.Get, TestID: "base-button", Style: bareBox, StyleFunc: bareInk(cx),
			OnPress: func(core.PressEvent) { pressed(cx, last, "base.Button")() }, Children: []core.Node{bareText(cx, tr.OverlaySave())}}
	}}
}

func baseToggleForm(cx *core.Scope, on, disabled *core.Signal[bool]) Form {
	tr := locales.For(cx)
	return Form{Name: "base.Toggle", Stage: func() core.Node {
		return base.Toggle{Label: tr.LibBold(), Pressed: on.Get, OnChange: on.Set, Disabled: disabled.Get, Style: bareBox, StyleFunc: bareInk(cx),
			Children: []core.Node{bareText(cx, tr.LibBold())}}
	}}
}

func baseToggleGroupForm(cx *core.Scope, bold, italic, disabled *core.Signal[bool]) Form {
	tr := locales.For(cx)
	toggle := func(text string, on *core.Signal[bool]) core.Node {
		return base.Toggle{Label: text, Pressed: on.Get, OnChange: on.Set, Style: bareBox, StyleFunc: bareInk(cx), Children: []core.Node{bareText(cx, text)}}
	}
	return Form{Name: "base.ToggleGroup", Stage: func() core.Node {
		return base.ToggleGroup{Label: tr.LibFormatting(), Disabled: disabled.Get, Style: core.Style{Direction: core.Row, Gap: 4},
			Children: []core.Node{toggle(tr.LibBold(), bold), toggle(tr.LibItalic(), italic)}}
	}}
}

func baseLinkForm(cx *core.Scope, last *core.Signal[string], disabled *core.Signal[bool]) Form {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	return Form{Name: "base.Link", Stage: func() core.Node {
		return base.Link{Href: "/docs", OpenWith: base.LinkOpenApp, Disabled: disabled.Get, OnOpen: last.Set, Label: tr.LibReadDocs(),
			Style:    core.Style{Disabled: &core.Style{Opacity: core.Alpha(0.4)}},
			Children: []core.Node{ui.Text(tr.LibReadDocs(), 13, ui.Regular, colors.Text)}}
	}}
}

func baseToolbarForm(cx *core.Scope, last *core.Signal[string], axis *core.Signal[base.Axis], bold, disabled *core.Signal[bool]) Form {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	return Form{Name: "base.Toolbar", Stage: func() core.Node {
		direction, rule := core.Row, core.Style{Width: core.Pt(1), Height: core.Pt(20)}
		if axis.Get() == base.Vertical {
			direction, rule = core.Column, core.Style{Width: core.Pt(20), Height: core.Pt(1)}
		}
		return base.Toolbar{Axis: axis.Get(), Label: tr.LibFormatting(), Disabled: disabled.Get, Style: core.Style{Direction: direction, AlignItems: core.AlignCenter, Gap: 4},
			Children: []core.Node{
				base.ToolbarGroup{Label: tr.LibHistory(), Style: core.Style{Direction: direction, Gap: 4}, Children: []core.Node{
					bareButton(cx, tr.LibUndo(), pressed(cx, last, tr.LibUndo())),
					bareButton(cx, tr.LibRedo(), pressed(cx, last, tr.LibRedo())),
				}},
				base.ToolbarSeparator{Style: rule, StyleFunc: ui.Filled(colors.Line)},
				base.Toggle{Label: tr.LibBold(), Pressed: bold.Get, OnChange: bold.Set, Style: bareBox, StyleFunc: bareInk(cx), Children: []core.Node{bareText(cx, tr.LibBold())}},
			}}
	}}
}

func baseInputForm(cx *core.Scope, value *core.Signal[string], multiline, masked, disabled *core.Signal[bool]) Form {
	tr := locales.For(cx)
	return Form{Name: "base.Input", Fill: true, Stage: func() core.Node {
		return base.Input{Value: value.Get, OnChange: value.Set, Placeholder: tr.CompTypeHere(), Multiline: multiline.Get(), Masked: masked.Get(), Disabled: disabled.Get,
			Style: core.Style{Height: core.Pt(32), Padding: core.Symmetric(0, 8), FontSize: 13, CornerRadius: 6, BorderWidth: 1}, StyleFunc: bareField(cx)}
	}}
}

func baseNumberInputForm(cx *core.Scope, value, step *core.Signal[float64], disabled *core.Signal[bool]) Form {
	tr := locales.For(cx)
	button := func(s string) core.Node { return bareText(cx, s) }
	return Form{Name: "base.NumberInput", Stage: func() core.Node {
		return base.NumberInput{Label: tr.LibFontSize(), Step: step.Get, Disabled: disabled.Get, IncrementLabel: "+1", DecrementLabel: "-1",
			Value: func() *float64 { v := value.Get(); return &v }, OnChange: func(v *float64) {
				if v != nil {
					value.Set(*v)
				}
			},
			Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 4}, ButtonLook: func() core.Style { return bareInk(cx)() },
			InputStyle:     core.Style{Width: core.Pt(72), Height: core.Pt(30), Padding: core.Symmetric(0, 8), FontSize: 13, TextAlign: core.TextAlignCenter, CornerRadius: 6, BorderWidth: 1},
			InputStyleFunc: bareField(cx), DecrementButton: button("-"), IncrementButton: button("+"),
			DecrementButtonStyle: bareBox, IncrementButtonStyle: bareBox}
	}}
}

func baseOtpInputForm(cx *core.Scope, last *core.Signal[string], masked, disabled *core.Signal[bool]) Form {
	tr := locales.For(cx)
	return Form{Name: "base.OtpInput", Stage: func() core.Node {
		return base.OtpInput{Length: 6, Masked: masked.Get(), Disabled: disabled.Get, Label: tr.LibOneTimeCode(), OnComplete: last.Set,
			Style: core.Style{Direction: core.Row, Gap: 4}, GroupStyle: core.Style{Direction: core.Row, Gap: 4},
			CellStyle: core.Style{Width: core.Pt(32), Height: core.Pt(36), FontSize: 14, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, CornerRadius: 6, BorderWidth: 1}, CellStyleFunc: bareField(cx)}
	}}
}

func baseCheckboxForm(cx *core.Scope, checked, partly, disabled *core.Signal[bool]) Form {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	return Form{Name: "base.Checkbox", Stage: func() core.Node {
		return base.Checkbox{Label: tr.LibStarred(), Checked: checked.Get, Indeterminate: partly.Get, Disabled: disabled.Get, OnChange: checked.Set,
			Style:          core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, Disabled: &core.Style{Opacity: core.Alpha(0.4)}},
			IndicatorStyle: core.Style{Width: core.Pt(16), Height: core.Pt(16), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, CornerRadius: 4, BorderWidth: 1},
			IndicatorStyleFunc: func() core.Style {
				on := &core.Style{Background: colors.Text(), BorderColor: colors.Text()}
				return core.Style{BorderColor: colors.Line(), Checked: on, Indeterminate: &core.Style{Background: colors.Line()}}
			},
			Indicator: ui.Text("✓", 11, ui.Semibold, colors.Background), Children: []core.Node{bareText(cx, tr.LibStarred())}}
	}}
}

func baseRadioForm(cx *core.Scope, picked *core.Signal[int], disabled *core.Signal[bool]) Form {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	return Form{Name: "base.RadioGroup · base.Radio", Stage: func() core.Node {
		var radios []core.Node
		for i, name := range []string{tr.LibApple(), tr.LibBanana(), tr.LibCherry()} {
			radios = append(radios, base.Radio{Label: name, Disabled: disabled.Get, Checked: func() bool { return picked.Get() == i }, OnChange: func(bool) { picked.Set(i) },
				Style: bareBox, StyleFunc: func() core.Style {
					return bareInk(cx)().Merge(core.Style{Checked: &core.Style{BorderColor: colors.Text()}})
				}, Children: []core.Node{bareText(cx, name)}})
		}
		return base.RadioGroup{Label: tr.LibFruit(), Style: core.Style{Direction: core.Row, Gap: 4}, Children: radios}
	}}
}

func baseSwitchForm(cx *core.Scope, checked, disabled *core.Signal[bool]) Form {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	return Form{Name: "base.Switch", Stage: func() core.Node {
		return base.Switch{Label: tr.LibStarred(), Checked: checked.Get, Disabled: disabled.Get, OnChange: checked.Set,
			Style:      core.Style{Disabled: &core.Style{Opacity: core.Alpha(0.4)}},
			TrackStyle: core.Style{Width: core.Pt(36), Height: core.Pt(20), Padding: core.All(2), CornerRadius: 10},
			TrackStyleFunc: func() core.Style {
				return core.Style{Background: colors.Line(), Checked: &core.Style{Background: colors.Text(), AlignItems: core.AlignEnd}}
			},
			ThumbStyle: core.Style{Width: core.Pt(16), Height: core.Pt(16), CornerRadius: 8}, ThumbStyleFunc: ui.Filled(colors.Surface)}
	}}
}

func baseSliderForm(cx *core.Scope, start, end, step *core.Signal[float64], ranged, disabled *core.Signal[bool]) Form {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	return Form{Name: "base.Slider", Fill: true, Stage: func() core.Node {
		return base.Slider{Label: tr.LibVolume(), StartLabel: tr.LibMinimum(), Step: step.Get, Disabled: disabled.Get,
			Value: func() base.SliderValue {
				if ranged.Get() {
					return base.RangeValue(start.Get(), end.Get())
				}
				return base.SingleValue(start.Get())
			},
			OnChange: func(v base.SliderValue) {
				start.Set(v.Start)
				if v.Range {
					end.Set(v.End)
				}
			},
			Style:          core.Style{Height: core.Pt(20), Disabled: &core.Style{Opacity: core.Alpha(0.4)}},
			TrackStyle:     core.Style{Height: core.Pt(20)},
			IndicatorStyle: core.Style{Height: core.Pt(4), CornerRadius: 2, Margin: core.Edges{Top: 8}}, IndicatorStyleFunc: ui.Filled(colors.Line),
			RangeStyle: core.Style{CornerRadius: 2}, RangeStyleFunc: ui.Filled(colors.Text),
			ThumbStyle: core.Style{Width: core.Pt(14), Height: core.Pt(14), CornerRadius: 7}, ThumbStyleFunc: ui.Filled(colors.Text)}
	}}
}

func baseAvatarForm(cx *core.Scope, picture *core.Signal[bool]) Form {
	colors := ui.ColorsOf(cx)
	return Form{Name: "base.Avatar", Stage: func() core.Node {
		a := base.Avatar{Name: demoPeople[0], Style: core.Style{Width: core.Pt(48), Height: core.Pt(48), CornerRadius: 24, Overflow: core.Hidden},
			FallbackStyle: core.Style{AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}, StyleFunc: ui.Filled(colors.Pressed),
			ImageStyle: core.Style{Width: core.Pt(48), Height: core.Pt(48)}, Fallback: bareText(cx, "AL")}
		if picture.Get() {
			a.Src = demoImage()
		}
		return a
	}}
}

func baseAvatarGroupForm(cx *core.Scope, limit *core.Signal[float64]) Form {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	shape := core.Style{Width: core.Pt(36), Height: core.Pt(36), CornerRadius: 18, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Overflow: core.Hidden}
	return Form{Name: "base.AvatarGroup", Stage: func() core.Node {
		var avatars []core.Node
		for _, name := range demoPeople {
			avatars = append(avatars, base.Avatar{Name: name, Style: shape, StyleFunc: ui.Filled(colors.Pressed), Fallback: bareText(cx, name[:1]),
				FallbackStyle: core.Style{AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}})
		}
		return base.AvatarGroup{Label: tr.ControlsMembers(), Limit: int(limit.Get()), Style: core.Style{Direction: core.Row, Gap: 4},
			OverflowStyle: shape, Overflow: bareText(cx, fmt.Sprintf("+%d", len(demoPeople)-int(limit.Get()))), OverflowLabel: tr.KitMore(), Children: avatars}
	}}
}

func baseCarouselForm(cx *core.Scope, last *core.Signal[string], count *core.Signal[float64], looping *core.Signal[bool]) Form {
	tr := locales.For(cx)
	return Form{Name: "base.Carousel", Fill: true, Stage: func() core.Node {
		var slides []core.Node
		for i := range int(count.Get()) {
			slides = append(slides, base.CarouselItem{Label: fmt.Sprintf("%d / %d", i+1, int(count.Get())), Style: core.Style{Width: core.Pt(200), Margin: core.Edges{Right: 8}},
				Children: []core.Node{swatch(cx, ToneNeutral, itoa(i+1))}})
		}
		return base.Carousel{Label: tr.DemoImage(), Looping: looping.Get(), Style: core.Style{Gap: 8}, OnChange: func(i int) { last.Set(fmt.Sprintf("Carousel: %d", i+1)) },
			Children: []core.Node{
				base.CarouselContent{Style: core.Style{AlignItems: core.AlignStretch}, Children: slides},
				core.View{Style: core.Style{Direction: core.Row, JustifyContent: core.JustifyCenter, Gap: 4}, Children: []core.Node{
					base.CarouselPrevious{Label: tr.LibBack(), Style: bareBox, StyleFunc: bareInk(cx), Children: []core.Node{bareText(cx, "‹")}},
					base.CarouselNext{Label: tr.LibNext(), Style: bareBox, StyleFunc: bareInk(cx), Children: []core.Node{bareText(cx, "›")}},
				}},
			}}
	}}
}

func baseListForm(cx *core.Scope, last *core.Signal[string]) Form {
	tr := locales.For(cx)
	selection := base.NewSelection(cx, false)
	return Form{Name: "base.List", Fill: true, Stage: func() core.Node {
		return base.List{Label: tr.ControlsMembers(), Selection: selection, EstimatedSize: 30,
			Count: func() int { return len(demoPeople) }, Key: func(i int) any { return demoPeople[i] },
			RowLabel:   func(i int) string { return demoPeople[i] },
			OnActivate: func(i int) { last.Set("base.List: " + demoPeople[i]) },
			Row: func(cx *core.Scope, row core.ListRow) core.Node {
				return bareTextf(cx, func() string { return demoPeople[row.Index()] })
			},
			Style: core.Style{Height: core.Pt(180)}, ItemStyle: bareRow, ItemStyleFunc: bareRowInk(cx)}
	}}
}

func baseTreeForm(cx *core.Scope, last *core.Signal[string]) Form {
	tr := locales.For(cx)
	items := []base.TreeItem{
		{ID: "source", Label: "source", Children: []base.TreeItem{{ID: "main.go", Label: "main.go"}, {ID: "view.go", Label: "view.go"}}},
		{ID: "docs", Label: "docs", Children: []base.TreeItem{{ID: "guide.md", Label: "guide.md"}}},
		{ID: "README.md", Label: "README.md"},
	}
	return Form{Name: "base.Tree", Fill: true, Stage: func() core.Node {
		return base.Tree{Label: tr.LibFiles(), Items: func() []base.TreeItem { return items }, DefaultExpanded: []string{"source"}, EstimatedSize: 30,
			OnAction: func(id string) { last.Set("base.Tree: " + id) }, Style: core.Style{Height: core.Pt(180), MinHeight: core.Pt(0)}, ListStyle: core.Style{FlexGrow: 1, MinHeight: core.Pt(0)},
			ItemStyle: bareRow, ItemStyleFunc: bareRowInk(cx),
			RenderItem: func(e base.TreeEntry) core.Node {
				return core.View{Style: core.Style{Padding: core.Edges{Left: core.Points(e.Depth) * 14}}, Children: []core.Node{bareText(cx, e.Label)}}
			}}
	}}
}

func baseProgressForm(cx *core.Scope, value *core.Signal[float64], indeterminate *core.Signal[bool]) Form {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	return Form{Name: "base.Progress", Fill: true, Stage: func() core.Node {
		return base.Progress{Label: tr.LibProgress(), Value: func() float32 { return float32(value.Get()) }, Indeterminate: indeterminate.Get,
			TrackStyle: core.Style{Height: core.Pt(6), CornerRadius: 3, Overflow: core.Hidden}, TrackStyleFunc: ui.Filled(colors.Line),
			IndicatorStyle: core.Style{Height: core.Pct(100)}, IndicatorStyleFunc: ui.Filled(colors.Text)}
	}}
}

func baseToastForm(cx *core.Scope, last *core.Signal[string], swipe *core.Signal[toast.SwipeDirection]) Form {
	tr := locales.For(cx)
	shown := core.NewSignal(cx, false)
	return Form{Name: "toast.Toast", Stage: func() core.Node {
		direction := swipe.Get()
		return core.View{Style: core.Style{Gap: 8, AlignItems: core.AlignCenter}, Children: []core.Node{
			bareButton(cx, tr.KitShow(), func() { shown.Set(true) }),
			core.Show{When: shown.Get, Then: func(cx *core.Scope) core.Node {
				return toast.Toast{Label: tr.LibChangesSaved(), SwipeDirection: direction, OnDismissRequest: func() { shown.Set(false); last.Set(tr.OverlayToastGone()) },
					Style: barePanel, StyleFunc: barePopup(cx), Children: []core.Node{bareText(cx, tr.LibChangesSaved())}}
			}},
		}}
	}}
}

func baseDialogForm(cx *core.Scope, last *core.Signal[string], noEscape, noBackdrop *core.Signal[bool]) Form {
	tr := locales.For(cx)
	open := core.NewSignal(cx, false)
	return Form{Name: "base.Dialog", Stage: func() core.Node {
		return core.Group{bareButton(cx, tr.KitOpen(), func() { open.Set(true) }),
			base.Dialog{Open: open.Get, Label: tr.KitDialogTitle(), TestID: "base-dialog", NoCloseOnEscape: noEscape.Get(), NoCloseOnBackdropPress: noBackdrop.Get(),
				OnOpenChange:  func(o bool, reason base.Reason) { open.Set(o); last.Set("base.Dialog: " + reason.String()) },
				BackdropStyle: bareBackdrop, PopupStyle: barePanel, PopupStyleFunc: barePopup(cx),
				Children: []core.Node{bareText(cx, tr.KitDialogTitle()), bareButton(cx, tr.OverlayDone(), func() { open.Set(false) })}}}
	}}
}

func baseAlertDialogForm(cx *core.Scope, last *core.Signal[string]) Form {
	tr := locales.For(cx)
	return Form{Name: "base.ShowAlertDialog", Stage: func() core.Node {
		return bareButton(cx, tr.KitOpen(), func() {
			base.ShowAlertDialog(cx, base.AlertDialog{Title: tr.KitDialogTitle(), Description: tr.KitDialogNote(),
				Actions: []core.AlertAction{{Label: tr.KitCancel()}, {Label: tr.OverlayDone()}}}, func(label string) { last.Set("AlertDialog: " + label) })
		})
	}}
}

func baseSheetForm(cx *core.Scope, last *core.Signal[string], side *core.Signal[core.Placement], swipe *core.Signal[bool]) Form {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	open := core.NewSignal(cx, false)
	return Form{Name: "base.Sheet", Stage: func() core.Node {
		return core.Group{bareButton(cx, tr.OverlayShowTheSheet(), func() { open.Set(true) }),
			base.Sheet{Open: open.Get, Side: side.Get(), CloseOnSwipe: swipe.Get(), Label: tr.OverlaySheet(),
				OnOpenChange:  func(next bool, reason base.Reason) { open.Set(next); last.Set("base.Sheet: " + reason.String()) },
				BackdropStyle: bareBackdrop, PopupStyle: core.Style{Padding: core.All(16), Gap: 12}, PopupStyleFunc: ui.Filled(colors.Surface),
				Children: []core.Node{bareText(cx, tr.OverlayTheBackdropEscapeAndBack()), bareButton(cx, tr.OverlayDone(), func() { open.Set(false) })}}}
	}}
}

func basePopoverForm(cx *core.Scope, last *core.Signal[string], placement *core.Signal[core.Placement], align *core.Signal[core.Align]) Form {
	tr := locales.For(cx)
	return Form{Name: "base.Popover", Stage: func() core.Node {
		return base.Popover{Placement: placement.Get(), Align: align.Get(), Offset: 4, Margin: 8, Label: tr.OverlayPopover(), Style: bareBox, StyleFunc: bareInk(cx),
			Trigger:      bareText(cx, tr.OverlayShowThePopover()),
			OnOpenChange: func(_ bool, reason base.Reason) { last.Set("base.Popover: " + reason.String()) },
			PopupStyle:   barePanel, PopupStyleFunc: barePopup(cx), Children: []core.Node{bareText(cx, tr.OverlayAPressOutsideEscapeOr())}}
	}}
}

func baseTooltipForm(cx *core.Scope, placement *core.Signal[core.Placement]) Form {
	tr := locales.For(cx)
	return Form{Name: "base.Tooltip", Stage: func() core.Node {
		return base.Tooltip{Placement: placement.Get(), Offset: 4, Margin: 8, Trigger: bareText(cx, tr.OverlaySave()), Style: bareBox, StyleFunc: bareInk(cx),
			PopupStyle: core.Style{Padding: core.Symmetric(4, 8), CornerRadius: 6, BorderWidth: 1}, PopupStyleFunc: barePopup(cx),
			Children: []core.Node{bareText(cx, tr.OverlaySavesTheDocument())}}
	}}
}

// bareMenuItems are the items of the base forms of Menu and ContextMenu.
func bareMenuItems(cx *core.Scope, last *core.Signal[string]) []core.Node {
	tr := locales.For(cx)
	item := func(text string) core.Node {
		return base.MenuItem{Label: text, OnSelect: func() { last.Set("base.Menu: " + text) }, Style: bareRow, StyleFunc: bareInk(cx),
			Children: []core.Node{bareText(cx, text)}}
	}
	colors := ui.ColorsOf(cx)
	return []core.Node{item(tr.TerminalCopy()), item(tr.TerminalPaste()),
		base.MenuSeparator{Style: core.Style{Height: core.Pt(1), Margin: core.Symmetric(4, 0)}, StyleFunc: ui.Filled(colors.Line)},
		base.Submenu{Label: tr.KitMore(), Trigger: bareText(cx, tr.KitMore()), Back: bareText(cx, tr.LibBack()), Style: bareRow, StyleFunc: bareInk(cx),
			PopupStyle: bareMenuPanel, PopupStyleFunc: barePopup(cx), Children: []core.Node{item(tr.LibApple()), item(tr.LibBanana())}}}
}

// bareMenuPanel is a menu's popup: its rows reach its edges.
var bareMenuPanel = core.Style{Padding: core.All(4), Gap: 2, CornerRadius: 8, BorderWidth: 1, MinWidth: core.Pt(160)}

func baseMenuForm(cx *core.Scope, last *core.Signal[string], placement *core.Signal[core.Placement], align *core.Signal[core.Align]) Form {
	tr := locales.For(cx)
	return Form{Name: "base.Menu", Stage: func() core.Node {
		return base.Menu{Placement: placement.Get(), Align: align.Get(), Offset: 4, Margin: 8, Label: tr.DemoMenu(), Trigger: bareText(cx, tr.KitOpen()), Style: bareBox, StyleFunc: bareInk(cx),
			PopupStyle: bareMenuPanel, PopupStyleFunc: barePopup(cx), BackdropStyle: bareBackdrop, Children: bareMenuItems(cx, last)}
	}}
}

func baseContextMenuForm(cx *core.Scope, last *core.Signal[string], off *core.Signal[bool]) Form {
	tr := locales.For(cx)
	return Form{Name: "base.ContextMenu", Fill: true, Stage: func() core.Node {
		return base.ContextMenu{Disabled: off.Get(), PopupStyle: bareMenuPanel, PopupStyleFunc: barePopup(cx), BackdropStyle: bareBackdrop, Children: bareMenuItems(cx, last),
			Trigger: core.View{Style: core.Style{Height: core.Pt(64), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
				Children: []core.Node{bareText(cx, Hint(cx, tr.KitLongPressHere(), tr.KitRightClickHere()))}}}
	}}
}

func baseTabsForm(cx *core.Scope, last *core.Signal[string], selected *core.Signal[int], disabled *core.Signal[bool]) Form {
	tr := locales.For(cx)
	return Form{Name: "base.Tabs · base.Tab", Stage: func() core.Node {
		var tabs []core.Node
		for i, name := range []string{tr.ControlsOverview(), tr.ControlsActivity(), tr.ControlsBilling()} {
			tab := base.Tab{Label: name, Selected: func() bool { return selected.Get() == i }, OnPress: func(core.PressEvent) { selected.Set(i); last.Set(fmt.Sprintf("base.Tabs: %d", i)) },
				Style: bareBox.Merge(core.Style{BorderWidth: 0}), StyleFunc: bareInk(cx), Children: []core.Node{bareText(cx, name)}}
			if i == 2 {
				tab.Disabled = disabled.Get
			}
			tabs = append(tabs, tab)
		}
		return base.Tabs{Label: tr.DemoTabs(), Style: core.Style{Direction: core.Row, Gap: 4, Padding: core.Edges{Bottom: 2}}, Children: tabs,
			IndicatorStyle: core.Style{Position: core.Absolute, Bottom: core.Pt(0), Height: core.Pt(2), Background: bareIndicator}}
	}}
}

func basePaginationForm(cx *core.Scope, page, total *core.Signal[float64], compact, disabled *core.Signal[bool]) Form {
	tr := locales.For(cx)
	return Form{Name: "base.Pagination", Stage: func() core.Node {
		return base.Pagination{Label: tr.LibPages(), PreviousLabel: tr.LibPreviousPage(), NextLabel: tr.LibNextPage(), VisiblePages: 7, Compact: compact.Get, Disabled: disabled.Get,
			Page: func() int { return int(page.Get()) }, TotalPages: func() int { return int(total.Get()) }, OnChange: func(p int) { page.Set(float64(p)) },
			Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 2}, ItemStyle: bareBox.Merge(core.Style{Padding: core.Symmetric(0, 6), MinWidth: core.Pt(30)}), ItemStyleFunc: bareInk(cx),
			RenderItem: func(i base.PaginationItem) core.Node {
				switch i.Kind {
				case base.PaginationPrevious:
					return bareText(cx, "‹")
				case base.PaginationNext:
					return bareText(cx, "›")
				case base.PaginationEllipsis:
					return bareText(cx, "…")
				}
				return bareText(cx, strconv.Itoa(i.Page))
			}}
	}}
}

// demoSections are the Sidebar demo's sections.
func demoSections(tr locales.Messages) []string {
	return []string{tr.ControlsOverview(), tr.ControlsActivity(), tr.ControlsBilling()}
}

func baseAccordionForm(cx *core.Scope, multiple, disabled *core.Signal[bool]) Form {
	tr := locales.For(cx)
	open := core.NewSignalFunc(cx, []string{"overview"}, slices.Equal)
	item := func(value, title, body string) core.Node {
		return base.AccordionItem{Value: value, Style: core.Style{Gap: 4}, Children: []core.Node{
			base.AccordionHeader{Children: []core.Node{base.AccordionTrigger{Label: title, Style: bareBox.Merge(core.Style{JustifyContent: core.JustifyStart}), StyleFunc: bareInk(cx),
				Children: []core.Node{bareText(cx, title)}}}},
			base.AccordionPanel{Style: core.Style{Padding: core.Symmetric(4, 12)}, Children: []core.Node{bareText(cx, body)}},
		}}
	}
	return Form{Name: "base.Accordion", Fill: true, Stage: func() core.Node {
		return base.Accordion{Value: open.Get, Multiple: multiple.Get, Disabled: disabled.Get, OnChange: open.Set, Style: core.Style{Gap: 4}, Children: []core.Node{
			item("overview", tr.ControlsOverview(), tr.ControlsDeployedToProduction()),
			item("activity", tr.ControlsActivity(), tr.ControlsInvoicePaid()),
		}}
	}}
}

func baseCollapsibleForm(cx *core.Scope, open, disabled *core.Signal[bool]) Form {
	tr := locales.For(cx)
	return Form{Name: "base.Collapsible", Fill: true, Stage: func() core.Node {
		return base.Collapsible{Open: open.Get, Style: core.Style{Gap: 4}, Content: core.View{Style: core.Style{Padding: core.Symmetric(4, 12)}, Children: []core.Node{bareText(cx, tr.ControlsDeployedToProduction())}},
			Children: []core.Node{base.Button{Label: tr.ControlsActivity(), Expanded: open.Get, Disabled: disabled.Get, OnPress: func(core.PressEvent) { toggled(open) }, Style: bareBox, StyleFunc: bareInk(cx),
				Children: []core.Node{bareText(cx, tr.ControlsActivity())}}}}
	}}
}
