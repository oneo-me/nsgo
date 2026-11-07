package ui

import (
	"slices"
	"testing"
	"time"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"
)

// showcase is a component of each kind the gallery shares, popups
// closed, in a column that scrolls.
func showcase(cx *core.Scope) core.Node {
	on, value, tab := core.NewSignal(cx, true), core.NewSignal(cx, 2.0), core.NewSignal(cx, 1)
	tags := core.NewSignalFunc(cx, []string{"go", "ui"}, slices.Equal)
	sel := NewSelection(cx, false)
	rows := []string{"One", "Two", "Three"}
	return core.Scroll{TestID: "showcase", Style: core.Style{FlexGrow: 1}, Children: []core.Node{core.View{Style: core.Style{Gap: 12}, Children: []core.Node{
		HStack{Gap: 8, Children: []core.Node{
			Button{Text: "Primary"}, Button{Text: "Accent", Variant: ButtonAccent}, Button{Icon: lucide.Plus, Label: "Add", Variant: ButtonGhost},
			RoundButton{Icon: lucide.Play, Label: "Play"}, IconButton{Icon: lucide.Search, Label: "Search", Selected: on.Get},
			Badge{Text: "New", Tone: ToneAccent}, Avatar{Name: "Ada Lovelace", Status: AvatarOnline},
			AvatarGroup{Members: []AvatarGroupMember{{Name: "Ada"}, {Name: "Alan"}, {Name: "Grace"}}, Max: 2},
		}},
		core.View{Style: core.Style{Gap: 12, Padding: core.All(14)}, StyleFunc: func() core.Style { return CardStyle(cx) }, Children: []core.Node{
			Checkbox{Text: "Checked", Checked: on.Get, OnChange: on.Set},
			RadioGroup{Children: []core.Node{Radio{Text: "Radio", Checked: on.Get}}},
			Switch{Checked: on.Get, OnChange: on.Set, Label: "Switch"},
			Tabs[int]{Options: []Option[int]{{Value: 0, Label: "One"}, {Value: 1, Label: "Two"}}, Value: tab.Get, OnChange: tab.Set},
			Tabs[int]{Variant: TabsUnderline, Options: []Option[int]{{Value: 0, Label: "One"}, {Value: 1, Label: "Two"}}, Value: tab.Get, OnChange: tab.Set},
			Progress{Label: "Progress", Value: func() float32 { return 40 }}, Meter{Value: func() float64 { return 0.4 }},
			Input{Input: base.Input{Label: "Name", Placeholder: "Name"}},
			NumberInput{Label: "Count", Value: value.Get, OnChange: value.Set, LeadingText: "W"},
			TagInput{Label: "Tags", Tags: tags.Get, OnChange: tags.Set}, Rating{Label: "Rating", Value: func() int { return 3 }},
			HStack{Gap: 8, Children: []core.Node{
				ToggleButton{Text: "Bold", On: on.Get, OnChange: on.Set}, ToggleButton{Icon: lucide.Star, Label: "Star"},
				HoldToConfirm{Text: "Hold", Tone: HoldDanger}, HoldToConfirm{Text: "Done", ConfirmedText: "Done", Confirmed: on.Get},
			}},
			Stepper{Current: func() int { return 1 }, OnSelect: func(int) {},
				Steps: []StepperStep{{Label: "One"}, {Label: "Two", Description: "Now"}, {Label: "Three", Error: "Failed"}}},
			Stepper{Current: func() int { return 1 }, Vertical: true, Steps: []StepperStep{{Label: "One"}, {Label: "Two"}}},
			Slider{Label: "Level", Value: func() float64 { return 30 }, Step: 10},
			Slider{Label: "Range", Value: func() float64 { return 20 }, High: func() float64 { return 70 }},
			HStack{Gap: 8, Children: []core.Node{
				ProgressRing{Value: func() float32 { return 60 }}, Spinner{}, Spinner{Variant: SpinnerSpokes},
				Select[int]{Placeholder: "Pick", Options: []Option[int]{{Value: 1, Label: "One"}}},
			}},
		}},
		Accordion{Items: []AccordionItem{{Text: "Open", Content: core.Text{Content: "Body"}}, {Text: "Closed"}},
			Open: func(i int) bool { return i == 0 }},
		Collapsible{Text: "Collapsible", Open: on.Get, OnOpenChange: on.Set, Content: core.Text{Content: "Body"}},
		Breadcrumbs{Items: []string{"Home", "Docs", "Guide", "Page"}, MaxItems: 3, MoreLabel: "More"},
		NavHeader{Title: "Header", OnBack: func() {}, BackLabel: "Back"},
		Skeleton{Lines: 2, Avatar: true, Label: "Loading"},
		Image{Label: "Picture"},
		ImageCompare{Label: "Compare", Before: core.View{}, After: core.View{}, BeforeLabel: "Before"},
		ScrollView{Style: core.Style{Height: core.Pt(80)}, BottomBar: []core.Node{IconButton{Icon: lucide.Plus, Label: "Add"}},
			Children: []core.Node{core.View{Style: core.Style{Height: core.Pt(400)}}}},
		List{Label: "List", Count: func() int { return len(rows) }, Selection: sel, RowLabel: func(i int) string { return rows[i] },
			Row: func(cx *core.Scope, r core.ListRow) core.Node {
				return core.Text{ContentFunc: func() string { return rows[r.Index()] }}
			}, Style: core.Style{Height: core.Pt(120)}},
		TextView{Markdown: "# Title\n\nSome `code`, a [link](https://example.com) and\n\n> a quote.\n"},
		ResizablePanelGroup{HandleLabel: "Resize", Style: core.Style{Height: core.Pt(60)},
			Panels: []base.ResizablePanel{{MinSize: -1}, {MinSize: -1}}},
		Menu{Trigger: Button{Text: "Menu"}, Children: []core.Node{MenuItem{Text: "Item"}}},
		Popover{Trigger: Button{Text: "Popover", Variant: ButtonSecondary}},
	}}}}
}

// TestThemes keeps the default accent and neutrals free of a hue, and has
// what floats follow the theme shown.
func TestThemes(t *testing.T) {
	headless(t)
	th := Theme()
	if th.Light.Accent != core.RGB(0x007aff) || th.Dark.Accent != core.RGB(0x0a84ff) || th.Light.Colors["accentFill"] != th.Light.Accent {
		t.Fatalf("default accent = %v", th.Light.Accent)
	}
	for _, p := range []core.Palette{th.Light, th.Dark} {
		for _, c := range []core.Color{p.Background, p.Surface, p.SurfaceHover, p.SurfacePressed, p.Border, p.Text, p.TextMuted,
			p.Colors["inset"], p.Colors["raised"], p.Colors["field"], p.Colors["line"], p.Colors["faint"], p.Colors["track"]} {
			if r, g, b, _ := c.Components(); r != g || g != b {
				t.Errorf("neutral %v has a hue", c)
			}
		}
	}
	if d := WarmDuskTheme(); d.Light.Accent != core.RGB(0xbd481b) || d.Dark.Accent != core.RGB(0xed986e) {
		t.Fatalf("Warm Dusk accents = %v, %v", d.Light.Accent, d.Dark.Accent)
	}
	for _, theme := range []*core.Theme{th, WarmDuskTheme()} {
		var menu, pill Finish
		start(t, desktopEnv, func(cx *core.Scope) core.Node {
			cx.SetTheme(theme)
			return core.View{TestID: "finish", StyleFunc: func() core.Style {
				menu, pill = FinOf(cx).Menu(), FinOf(cx).Pill()
				return core.Style{}
			}}
		})
		if menu.Background != Alpha(theme.Light.Background, 0.92) || menu.Border != theme.Light.Border || pill.Background != theme.Light.Colors["raised"] {
			t.Fatalf("finishes %v, %v do not follow the theme", menu, pill)
		}
	}
}

// TestAppearanceRestylesInPlace turns the components dark: each restyles
// in place, built once, and looks as it does in a dark build.
func TestAppearanceRestylesInPlace(t *testing.T) {
	headless(t)
	for _, e := range []core.Environment{desktopEnv, phoneEnv} {
		dark := e
		dark.Dark = true
		app, _ := start(t, e, showcase)
		app.FastForward(time.Second)
		before, _ := looks(rootOf(app.Find("showcase")))
		app.SetEnvironment(dark)
		app.FastForward(time.Second)
		ids, got := looks(rootOf(app.Find("showcase")))
		kept := map[uint64]bool{}
		for _, id := range ids {
			kept[id] = true
		}
		for _, id := range before {
			if !kept[id] {
				t.Fatalf("%v: view %d was built again", e.Idiom, id)
			}
		}
		fresh, _ := start(t, dark, showcase)
		fresh.FastForward(time.Second)
		_, want := looks(rootOf(fresh.Find("showcase")))
		if len(got) != len(want) {
			t.Fatalf("%v: %d views after the switch, %d in a dark build", e.Idiom, len(got), len(want))
		}
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("%v: view %d after the switch:\n%s\nin a dark build:\n%s", e.Idiom, i, got[i], want[i])
			}
		}
	}
}
