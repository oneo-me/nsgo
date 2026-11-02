package main

import (
	"fmt"
	"slices"
	"time"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"

	ui "github.com/oneo-me/nsgo/examples/gallery-ui"
	"github.com/oneo-me/nsgo/examples/gallery/locales"
)

// base's controls own presses, keys, the disabled state and what screen
// readers say; every value is the app's, and so is the look: the gallery's
// own here, with its hover and keyboard focus ring on every control, and
// no motion with reduced motion.

// controlSizes are the sizes of the page. Touch screens take the standard
// tier (44 pt and more to press); a pointer the compact one desktops'
// settings and filters use.
type controlSizes struct {
	row, padding, gap, label, detail, control, corner, check, dash, dot, tabs, line core.Points
	small                                                                           bool
}

var (
	pointerSizes = controlSizes{row: 40, padding: 6, gap: 10, label: 14, detail: 12, control: 16, corner: 5, check: 12, dash: 8, dot: 6, tabs: 30, line: 32, small: true}
	touchSizes   = controlSizes{row: 56, padding: 8, gap: 12, label: 15, detail: 13, control: 22, corner: 7, check: 16, dash: 10, dot: 8, tabs: 40, line: 44}
)

// bleed is how far a row's hover fill reaches past its content on each
// side, and its corners.
const bleed core.Points = 8

// controlRow is a control's row: the control and its label, the whole row
// pressable.
// Its hairline's color is rowLine's.
func (z controlSizes) controlRow(cx *core.Scope, first bool) core.Style {
	s := core.Style{MinHeight: core.Pt(z.row), Direction: core.Row, AlignItems: core.AlignCenter, Gap: z.gap, Padding: core.Symmetric(z.padding, 0)}
	if !first {
		s.Borders = core.Edges{Top: 1}
	}
	return s
}

// rowLine is the color of a control row's hairline, for its StyleFunc;
// the first row has none.
func rowLine(cx *core.Scope, first bool) func() core.Style {
	if first {
		return nil
	}
	return func() core.Style { return core.Style{BorderColors: core.EdgeColors{Top: ui.ColorsOf(cx).Line()}} }
}

// hoverRow is a row that takes the pointer's hover: its fill reaching
// bleed past the content on both sides with rounded corners, and the
// content where it was (start: more room before it). The hairline above
// it is a divider, so nothing moves.
func (z controlSizes) hoverRow(cx *core.Scope, first bool, start core.Points, disabled bool) core.Style {
	s := z.controlRow(cx, true)
	s.Margin = core.Edges{Left: -bleed, Right: -bleed}
	top := z.padding
	if !first {
		top++
	}
	s.Padding = core.Edges{Left: bleed + start, Right: bleed, Top: top, Bottom: z.padding}
	s.CornerRadius = bleed
	s.Active = &core.Style{Opacity: core.Alpha(0.7)}
	s.Disabled = &core.Style{Opacity: core.Alpha(0.4)}
	return s
}

// hoverPaint is a hover row's focus ring and hover fill, for its
// StyleFunc.
func hoverPaint(cx *core.Scope, disabled bool) func() core.Style {
	return func() core.Style {
		s := core.Style{Focus: ui.FocusRing(cx, -2)}
		if !disabled {
			s.Hover = ui.Hovered(cx)
		}
		return s
	}
}

// divider is the hairline above a hover row that is not the first.
func divider(cx *core.Scope, show bool) core.Node {
	colors := ui.ColorsOf(cx)
	if !show {
		return core.Group{}
	}
	return core.View{Style: core.Style{Position: core.Absolute, Top: core.Pt(0), Left: core.Pt(bleed), Right: core.Pt(bleed), Height: core.Pt(1)}, StyleFunc: ui.Filled(colors.Line)}
}

// onFill is the fill of a box or ring that is on, and of one that is off.
func onFill(cx *core.Scope, on bool, s core.Style) core.Style {
	colors := ui.ColorsOf(cx)
	fin := ui.FinOf(cx)
	if on {
		s.Background, s.BorderColor, s.Shadows = colors.AccentFill(), colors.AccentFill(), fin.Chip().Shadows
	} else {
		s.Background, s.BorderColor, s.Shadows = colors.Field(), colors.FieldBorder(), fin.Field().Shadows
	}
	return s
}

// mark is what a checkbox's box shows.
type mark uint8

const (
	markNone mark = iota
	markCheck
	markDash
)

// box is a checkbox's box: empty, or orange with a check or a dash.
func (z controlSizes) box(cx *core.Scope, m mark) core.Style {
	return onFill(cx, m != markNone, core.Style{Width: core.Pt(z.control), Height: core.Pt(z.control), CornerRadius: z.corner,
		AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, BorderWidth: 1})
}

// boxMark is the mark inside a checkbox's box.
func (z controlSizes) boxMark(cx *core.Scope, m func() mark) core.Node {
	colors := ui.ColorsOf(cx)
	return core.Dynamic(func(*core.Scope) core.Node {
		switch m() {
		case markCheck:
			return ui.Icon(ui.IconCheck, z.check, colors.OnAccent)
		case markDash:
			return core.View{Style: core.Style{Width: core.Pt(z.dash), Height: core.Pt(2), CornerRadius: 1}, StyleFunc: ui.Filled(colors.OnAccent)}
		}
		return core.Group{}
	})
}

// ring is a radio's ring: empty, or orange with a dot.
func (z controlSizes) ring(cx *core.Scope, on func() bool) core.Node {
	colors := ui.ColorsOf(cx)
	return core.View{StyleFunc: func() core.Style {
		return onFill(cx, on(), core.Style{Width: core.Pt(z.control), Height: core.Pt(z.control), CornerRadius: z.control / 2,
			AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, BorderWidth: 1})
	}, Children: []core.Node{core.Show{When: on, Then: func(*core.Scope) core.Node {
		return core.View{Style: core.Style{Width: core.Pt(z.dot), Height: core.Pt(z.dot), CornerRadius: z.dot / 2}, StyleFunc: ui.Filled(colors.OnAccent)}
	}}}}
}

// label is a label and a quieter line under it.
func (z controlSizes) labelled(cx *core.Scope, label string, detail func() string) core.Node {
	colors := ui.ColorsOf(cx)
	children := []core.Node{ui.Text(label, z.label, ui.Regular, colors.Text)}
	if detail != nil {
		children = append(children, ui.Textf(detail, z.detail, ui.Regular, colors.Muted))
	}
	return core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 2}, Children: children}
}

type kind struct {
	name  string
	count int
}

type plan struct {
	name, detail, price string
	disabled            bool
}

type tabSection struct {
	name     string
	rows     [][2]string
	disabled bool
}

// controlsData is the demo's sample data in tr's language.
func controlsData(tr locales.Messages) (kinds []kind, plans []plan, tabSections []tabSection, usage [][2]string) {
	kinds = []kind{{tr.ControlsPhotos(), 1204}, {tr.ControlsVideos(), 86}, {tr.ControlsScreenshots(), 312}, {tr.ControlsLivePhotos(), 48}}
	plans = []plan{
		{tr.ControlsFree(), tr.ControlsOneDevice5Gb(), "$0", false},
		{tr.ControlsPlus(), tr.ControlsThreeDevices200Gb(), tr.Controls3Mo(), false},
		{tr.ControlsTeam(), tr.ControlsASharedLibrary2Tb(), tr.Controls12Mo(), false},
		{tr.ControlsEnterprise(), tr.ControlsSetUpByYourOrganization(), "—", true},
	}
	tabSections = []tabSection{
		{tr.ControlsOverview(), [][2]string{{tr.ControlsVisitors(), "12,480"}, {tr.ControlsSignUps(), "342"}, {tr.ControlsConversion(), "2.74%"}}, false},
		{tr.ControlsActivity(), [][2]string{{tr.ControlsDeployedToProduction(), "09:41"}, {tr.ControlsInvoicePaid(), "08:15"}, {tr.ControlsAMemberJoined(), "07:02"}}, false},
		{tr.ControlsMembers(), [][2]string{{"Ada Lovelace", tr.ControlsOwner()}, {"Grace Hopper", tr.ControlsAdmin()}, {"Alan Turing", tr.ControlsMember()}}, false},
		{tr.ControlsBilling(), nil, true},
	}
	usage = [][2]string{{tr.ControlsPhotos(), "120 GB"}, {tr.ControlsApps(), "54 GB"}, {tr.ControlsSystem(), "38 GB"}}
	return
}

// controlsDemo is base's Switch, Checkbox, Radio and RadioGroup, Tab and
// Tabs, Collapsible and Progress in the settings, filters, pickers,
// section switchers and disclosures apps use them for.
func controlsDemo(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	kinds, plans, tabSections, usage := controlsData(tr)
	z := touchSizes
	pointer := cx.Env().Desktop()
	if pointer {
		z = pointerSizes
	}
	last := core.NewSignal(cx, "—")

	setting := func(cx *core.Scope, label string, detail func() string, on *core.Signal[bool], disabled func() bool, first bool) core.Node {
		look := ui.NewSwitchLook(cx, on.Get, z.small)
		s := z.hoverRow(cx, first, 0, false)
		// The track at the trailing edge, after the label.
		s.Direction = core.RowReverse
		return base.Switch{Checked: on.Get, Disabled: disabled, Label: label,
			OnChange: func(v bool) {
				on.Set(v)
				last.Set(fmt.Sprintf("%s: %s", label, onOff(tr, v)))
			},
			Style: s, StyleFunc: hoverPaint(cx, false), TrackStyleFunc: look.Track, ThumbStyleFunc: look.Thumb,
			Children: []core.Node{divider(cx, !first), z.labelled(cx, label, detail)}}
	}

	settings := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		notifications, badges := core.NewSignal(cx, true), core.NewSignal(cx, true)
		haptics, lowData := core.NewSignal(cx, false), core.NewSignal(cx, false)
		return ui.Card{Icon: ui.IconControls, Title: tr.ControlsSwitch(), Description: tr.ControlsSettingsThatTakeEffectAt(), Children: []core.Node{
			core.View{Children: []core.Node{
				setting(cx, tr.ControlsNotifications(), func() string {
					if notifications.Get() {
						return tr.ControlsBannersSoundsAndBadges()
					}
					return tr.ControlsOff()
				}, notifications, nil, true),
				setting(cx, tr.ControlsBadges(), func() string {
					if notifications.Get() {
						return tr.ControlsTheUnreadCountOnThe()
					}
					return tr.ControlsTurnOnNotificationsFirst()
				}, badges, func() bool { return !notifications.Get() }, false),
				setting(cx, tr.ControlsHaptics(), func() string { return tr.ControlsATapUnderTheFinger() }, haptics, nil, false),
				setting(cx, tr.ControlsLowDataMode(), func() string {
					if lowData.Get() {
						return tr.ControlsImagesLoadAtALower()
					}
					return tr.ControlsImagesLoadInFull()
				}, lowData, nil, false),
			}},
		}}
	}

	filter := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		shown := core.NewSignalFunc(cx, []string{tr.ControlsPhotos(), tr.ControlsVideos(), tr.ControlsLivePhotos()}, slices.Equal)
		has := func(name string) bool { return slices.Contains(shown.Get(), name) }
		all := func() bool { return len(shown.Get()) == len(kinds) }
		boxMark := func() mark {
			switch {
			case all():
				return markCheck
			case len(shown.Get()) > 0:
				return markDash
			}
			return markNone
		}
		rows := []core.Node{base.Checkbox{Checked: all, Indeterminate: func() bool { return boxMark() == markDash }, Label: tr.ControlsSelectAll(),
			OnChange: func(on bool) {
				var names []string
				if on {
					for _, k := range kinds {
						names = append(names, k.name)
					}
				}
				shown.Set(names)
				last.Set(fmt.Sprintf("%s: %s", tr.ControlsSelectAll(), onOff(tr, on)))
			},
			Style: z.hoverRow(cx, true, 0, false), StyleFunc: hoverPaint(cx, false), IndicatorStyleFunc: func() core.Style { return z.box(cx, boxMark()) }, Indicator: z.boxMark(cx, boxMark),
			Children: []core.Node{core.Text{Content: tr.ControlsSelectAll(), Style: core.Style{FlexGrow: 1, FontSize: z.label, FontWeight: ui.Semibold}, StyleFunc: ui.Colored(colors.Text)}}}}
		for _, k := range kinds {
			m := func() mark {
				if has(k.name) {
					return markCheck
				}
				return markNone
			}
			rows = append(rows, base.Checkbox{Checked: func() bool { return has(k.name) }, Label: k.name,
				OnChange: func(on bool) {
					var names []string
					for _, o := range kinds {
						if (o.name == k.name && on) || (o.name != k.name && has(o.name)) {
							names = append(names, o.name)
						}
					}
					shown.Set(names)
					last.Set(fmt.Sprintf("%s: %s", k.name, onOff(tr, on)))
				},
				// One level under "Select all": the boxes line up with its label.
				Style: z.hoverRow(cx, false, z.control+z.gap, false), StyleFunc: hoverPaint(cx, false), IndicatorStyleFunc: func() core.Style { return z.box(cx, m()) }, Indicator: z.boxMark(cx, m),
				Children: []core.Node{divider(cx, true),
					core.Text{Content: k.name, Style: core.Style{FlexGrow: 1, FontSize: z.label}, StyleFunc: ui.Colored(colors.Text)},
					ui.Text(thousands(k.count), z.detail, ui.Regular, colors.Faint)}})
		}
		total := func() int {
			n := 0
			for _, k := range kinds {
				if has(k.name) {
					n += k.count
				}
			}
			return n
		}
		return ui.Card{Icon: ui.IconControls, Title: tr.ControlsCheckbox(), Description: tr.ControlsAFilterSelectAllShows(), Actions: []core.Node{ui.Badge{TextFunc: func() string { return tr.ControlsCountOf(len(shown.Get()), len(kinds)) }}}, Children: []core.Node{
			core.View{Children: rows},
			core.Text{ContentFunc: func() string {
				if len(shown.Get()) == 0 {
					return tr.ControlsNothingToShow()
				}
				return tr.ControlsShowingItems(total())
			}, Style: core.Style{FontSize: 14}, StyleFunc: ui.Colored(colors.Muted)},
		}}
	}

	plansCard := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		chosen := core.NewSignal(cx, tr.ControlsPlus())
		var radios []core.Node
		for i, p := range plans {
			on := func() bool { return chosen.Get() == p.name }
			radios = append(radios, base.Radio{Checked: on, Disabled: func() bool { return p.disabled }, Label: p.name,
				OnChange: func(bool) { chosen.Set(p.name); last.Set(tr.ControlsPlan() + p.name) },
				Style:    z.hoverRow(cx, i == 0, 0, p.disabled), StyleFunc: hoverPaint(cx, p.disabled),
				Children: []core.Node{divider(cx, i != 0), z.ring(cx, on), z.labelled(cx, p.name, func() string { return p.detail }),
					core.Text{Content: p.price, StyleFunc: func() core.Style {
						c := colors.Muted
						if on() {
							c = colors.Text
						}
						return core.Style{FontSize: z.label, Color: c()}
					}}}})
		}
		align := core.AlignStretch
		if pointer {
			align = core.AlignEnd
		}
		// The commit of the picker: under it on a phone, at its trailing
		// edge with a pointer.
		return ui.Card{Icon: ui.IconControls, Title: tr.ControlsRadio(), Description: tr.ControlsAPlanPickerOneOf(), Children: []core.Node{
			base.RadioGroup{Label: tr.ControlsPlan2(), Children: radios},
			core.View{Style: core.Style{AlignItems: align}, Children: []core.Node{core.Dynamic(func(*core.Scope) core.Node {
				name := chosen.Get()
				return ui.Button{Text: tr.ControlsContinueWith() + name, Small: pointer, OnPress: func() {
					for _, p := range plans {
						if p.name == name {
							last.Set(tr.ControlsContinueEvent(p.name, p.price))
						}
					}
				}}
			})}},
		}}
	}

	sections := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		fin := ui.FinOf(cx)
		current := core.NewSignal(cx, tr.ControlsOverview())
		// The track's padding and border: a tab is a pill inside them.
		pill := z.tabs - 2*(3+1)
		var tabs []core.Node
		for _, s := range tabSections {
			selected := func() bool { return current.Get() == s.name }
			tabs = append(tabs, base.Tab{Selected: selected, Disabled: func() bool { return s.disabled }, Label: s.name,
				OnPress: func(core.PressEvent) { current.Set(s.name); last.Set(tr.ControlsTab() + s.name) },
				StyleFunc: func() core.Style {
					st := core.Style{FlexGrow: 1, FlexBasis: core.Pt(0), CornerRadius: pill / 2, BorderWidth: 1, BorderColor: transparent,
						AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter,
						Focus: ui.FocusRing(cx, -2), Disabled: &core.Style{Opacity: core.Alpha(0.4)}}
					if selected() {
						st = fin.Pill().Over(st)
						st.BorderColor = fin.Pill().Border
					} else if !s.disabled {
						st.Hover = ui.Hovered(cx)
					}
					return st
				},
				Children: []core.Node{core.Text{Content: s.name, MaxLines: 1, StyleFunc: func() core.Style {
					c := colors.Muted
					if selected() {
						c = colors.Text
					}
					return core.Style{FontSize: 13, FontWeight: ui.Medium, Color: c()}
				}}}})
		}
		return ui.Card{Icon: ui.IconControls, Title: tr.ControlsTabs(), Description: tr.ControlsASectionSwitcherTheApp(), Children: []core.Node{
			base.Tabs{Label: tr.ControlsSections(), StyleFunc: func() core.Style {
				return core.Style{Height: core.Pt(z.tabs), Direction: core.Row, Gap: 2, Padding: core.All(3), CornerRadius: z.tabs / 2,
					BorderWidth: 1, BorderColor: colors.Border(), Background: colors.Inset(), Shadows: fin.Track().Shadows}
			}, Children: tabs},
			core.Dynamic(func(*core.Scope) core.Node {
				var rows []core.Node
				for _, s := range tabSections {
					if s.name != current.Get() {
						continue
					}
					for i, r := range s.rows {
						st := z.controlRow(cx, i == 0)
						st.MinHeight, st.Padding = core.Pt(z.line), core.Edges{}
						rows = append(rows, core.View{Style: st, StyleFunc: rowLine(cx, i == 0), Children: []core.Node{
							core.Text{Content: r[0], Style: core.Style{FlexGrow: 1, FlexShrink: 1, FontSize: z.label}, StyleFunc: ui.Colored(colors.Text)},
							ui.Text(r[1], z.label, ui.Regular, colors.Muted),
						}})
					}
				}
				return core.View{Children: rows}
			}),
		}}
	}

	disclosure := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		storage, advanced, originals := core.NewSignal(cx, false), core.NewSignal(cx, false), core.NewSignal(cx, false)
		look := ui.NewSwitchLook(cx, originals.Get, z.small)
		reduce := cx.Env().ReduceMotion
		// summary is the always-visible row that flips a section: the
		// app's trigger, which tells screen readers whether it is expanded.
		summary := func(label, detail string, open *core.Signal[bool], first bool) core.Node {
			return core.View{Role: core.RoleButton, Label: label, Expanded: open.Get,
				OnPress: func(core.PressEvent) {
					next := !open.GetUntracked()
					open.Set(next)
					state := "collapsed"
					if next {
						state = "expanded"
					}
					last.Set(label + ": " + state)
				},
				Style: z.hoverRow(cx, first, 0, false), StyleFunc: hoverPaint(cx, false),
				Children: []core.Node{divider(cx, !first), z.labelled(cx, label, func() string { return detail }),
					// A lane as wide as the chevron's stroke, so the chevron
					// ends where the values below it end.
					core.View{StyleFunc: func() core.Style {
						s := core.Style{Width: core.Pt(z.check / 2), AlignItems: core.AlignCenter}
						if !reduce() {
							s.Transition = core.Transition{Duration: 150 * time.Millisecond, Properties: core.TransitionTransform}
						}
						if open.Get() {
							s.Transform.Rotate = 90
						}
						return s
					}, Children: []core.Node{ui.Icon(ui.IconChevron, z.check, colors.Muted)}},
				}}
		}
		var used []core.Node
		for _, u := range usage {
			used = append(used, core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, MinHeight: core.Pt(z.line)}, Children: []core.Node{
				core.Text{Content: u[0], Style: core.Style{FlexGrow: 1, FontSize: z.label}, StyleFunc: ui.Colored(colors.Muted)},
				ui.Text(u[1], z.label, ui.Regular, colors.Muted),
			}})
		}
		keep := z.hoverRow(cx, false, z.gap, false)
		keep.Direction = core.RowReverse
		return ui.Card{Icon: ui.IconControls, Title: tr.ControlsCollapsible(), Description: tr.ControlsDetailsUnderASummaryKept(), Children: []core.Node{
			core.View{Children: []core.Node{
				base.Collapsible{Open: storage.Get,
					Content:  core.View{Style: core.Style{Padding: core.Edges{Left: z.gap, Bottom: z.padding}}, Children: used},
					Children: []core.Node{summary(tr.ControlsStorage(), tr.Controls212GbOf256Gb(), storage, true)}},
				base.Collapsible{Open: advanced.Get,
					Content: base.Switch{Checked: originals.Get, Label: tr.ControlsKeepOriginals(),
						OnChange: func(on bool) { originals.Set(on); last.Set(tr.ControlsKeepOriginals2() + onOff(tr, on)) },
						Style:    keep, StyleFunc: hoverPaint(cx, false), TrackStyleFunc: look.Track, ThumbStyleFunc: look.Thumb,
						Children: []core.Node{divider(cx, true), z.labelled(cx, tr.ControlsKeepOriginals(), func() string {
							if originals.Get() {
								return tr.ControlsEditsAreSavedAsCopies()
							}
							return tr.ControlsEditsReplaceThePhoto()
						})}},
					Children: []core.Node{summary(tr.ControlsAdvanced(), tr.ControlsOptionsMostPeopleLeaveAlone(), advanced, false)}},
			}},
		}}
	}

	transfers := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		sent, running := core.NewSignal(cx, 0), core.NewSignal(cx, false)
		var timer func()
		stop := func() {
			if timer != nil {
				timer()
				timer = nil
			}
			running.Set(false)
		}
		cx.Cleanup(stop)
		// The indeterminate bar's sweep: the app flips its translation, a
		// transition moves it; it rests in the middle with reduced motion.
		end := core.NewSignal(cx, false)
		reduce := cx.Env().ReduceMotion
		every(cx, 800*time.Millisecond, func() {
			if !core.Untracked(cx, reduce) {
				toggled(end)
			}
		})
		start := func() {
			if sent.GetUntracked() >= 100 {
				sent.Set(0)
			}
			running.Set(true)
			last.Set(tr.ControlsUploadStarted())
			timer = every(cx, 300*time.Millisecond, func() {
				sent.Set(min(100, sent.GetUntracked()+10))
				if sent.GetUntracked() >= 100 {
					stop()
					last.Set(tr.ControlsUploadDone())
				}
			})
		}
		// transfer is a label and value over a bar, the value at the
		// trailing edge.
		transfer := func(label string, detail func() string, value func() int, indeterminate, first bool) core.Node {
			// The label row first: the children follow the track.
			st := z.controlRow(cx, first)
			st.Direction, st.AlignItems, st.Gap = core.ColumnReverse, core.AlignStretch, z.padding
			return base.Progress{Value: func() float32 { return float32(value()) }, Indeterminate: func() bool { return indeterminate }, Label: label,
				Style: st, StyleFunc: rowLine(cx, first),
				TrackStyle: core.Style{Height: core.Pt(6), CornerRadius: 3, Overflow: core.Hidden}, TrackStyleFunc: ui.Filled(colors.Raised),
				IndicatorStyleFunc: func() core.Style {
					s := core.Style{Height: core.Pt(6), CornerRadius: 3, Background: colors.Accent()}
					if !indeterminate {
						// The width follows each step of the transfer.
						s.Transition = core.Transition{Duration: 300 * time.Millisecond, Easing: core.EaseOut, Properties: core.TransitionLayout}
						return s
					}
					s.Width = core.Pct(100.0 / 3)
					switch {
					case reduce():
						s.Transform.TranslateXPercent = 100
					default:
						s.Transition = core.Transition{Duration: 700 * time.Millisecond, Easing: core.EaseInOut, Properties: core.TransitionTransform}
						if end.Get() {
							s.Transform.TranslateXPercent = 200
						}
					}
					return s
				},
				Children: []core.Node{core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: z.gap}, Children: []core.Node{
					z.labelled(cx, label, detail),
					core.Text{ContentFunc: func() string {
						if indeterminate {
							return "—"
						}
						return fmt.Sprintf("%d%%", value())
					}, Style: core.Style{FontSize: z.label, TextAlign: core.TextAlignEnd}, StyleFunc: ui.Colored(colors.Muted)},
				}}}}
		}
		var flex float32 = 1
		align := core.AlignStretch
		if pointer {
			flex, align = 0, core.AlignStart
		}
		return ui.Card{Icon: ui.IconControls, Title: tr.ControlsProgress(), Description: tr.ControlsTransfersTheAppOwnsThe(), Children: []core.Node{
			core.View{Children: []core.Node{
				transfer(tr.ControlsUpload(), func() string {
					switch {
					case sent.Get() >= 100:
						return tr.Controls24PhotosSent()
					case running.Get():
						return tr.ControlsSending24Photos()
					}
					return tr.Controls24PhotosToSend()
				}, sent.Get, false, true),
				transfer(tr.ControlsBackup(), func() string { return tr.ControlsPreparingItsSizeIsNot() }, func() int { return 0 }, true, false),
			}},
			core.View{Style: core.Style{Direction: core.Row, Gap: z.gap, Margin: core.Edges{Top: z.padding}, AlignSelf: align}, Children: []core.Node{
				core.Dynamic(func(*core.Scope) core.Node {
					label := tr.ControlsSend()
					switch {
					case running.Get():
						label = tr.ControlsPause()
					case sent.Get() >= 100:
						label = tr.ControlsSendAgain()
					}
					return ui.Button{Text: label, Small: z.small, Flex: flex, OnPress: func() {
						if running.GetUntracked() {
							stop()
							last.Set(tr.ControlsUploadPaused())
						} else {
							start()
						}
					}}
				}),
				ui.Button{Text: tr.ControlsReset(), Variant: ui.ButtonSecondary, Small: z.small, Flex: flex, Disabled: func() bool { return sent.Get() == 0 }, OnPress: func() {
					stop()
					sent.Set(0)
					last.Set(tr.ControlsUploadReset())
				}},
			}},
		}}
	}

	return demoPage(cx, pageProps{}, settings, filter, plansCard, sections, disclosure, transfers, tableCard, lastEventCard(last))
}

// onOff is a setting's state in words.
func onOff(tr locales.Messages, on bool) string {
	if on {
		return tr.CommonOn()
	}
	return tr.CommonOff()
}
