package ui

import (
	"slices"
	"testing"
	"time"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
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
			Button{Text: "Primary"}, Button{Text: "Accent", Variant: ButtonAccent}, Button{Icon: IconPlus, Label: "Add", Variant: ButtonGhost},
			RoundButton{Icon: IconPlay, Label: "Play"}, IconButton{Icon: IconSearch, Label: "Search", Selected: on.Get},
			Badge{Text: "New", Tone: ToneAccent}, Avatar{Name: "Ada Lovelace", Status: AvatarOnline},
			AvatarGroup{Members: []AvatarGroupMember{{Name: "Ada"}, {Name: "Alan"}, {Name: "Grace"}}, Max: 2},
		}},
		Card{Title: "Card", Description: "On the surface", OnPress: func() {}, Children: []core.Node{
			Checkbox{Text: "Checked", Checked: on.Get, OnChange: on.Set},
			RadioGroup{Children: []core.Node{Radio{Text: "Radio", Checked: on.Get}}},
			Switch{Checked: on.Get, OnChange: on.Set, Label: "Switch"},
			Segmented[int]{Options: []Option[int]{{Value: 0, Label: "Day"}, {Value: 1, Label: "Week"}}, Value: tab.Get, OnChange: tab.Set},
			Tabs{Items: []TabItem{{Text: "One"}, {Text: "Two"}}, Selected: tab.Get, OnChange: tab.Set},
			Tabs{Variant: TabsSegmented, Items: []TabItem{{Text: "One"}, {Text: "Two"}}, Selected: tab.Get, OnChange: tab.Set},
			Progress{Label: "Progress", Value: func() float32 { return 40 }}, Meter{Value: func() float64 { return 0.4 }},
			Input{Input: base.Input{Label: "Name", Placeholder: "Name"}},
			NumberInput{Label: "Count", Value: value.Get, OnChange: value.Set, LeadingText: "W"},
			TagInput{Label: "Tags", Tags: tags.Get, OnChange: tags.Set}, Rating{Label: "Rating", Value: func() int { return 3 }},
		}},
		Accordion{Items: []AccordionItem{{Text: "Open", Content: core.Text{Content: "Body"}}, {Text: "Closed"}},
			Open: func(i int) bool { return i == 0 }},
		Collapsible{Text: "Collapsible", Open: on.Get, OnOpenChange: on.Set, Content: core.Text{Content: "Body"}},
		Breadcrumbs{Items: []string{"Home", "Docs", "Guide", "Page"}, MaxItems: 3, MoreLabel: "More"},
		NavHeader{Title: "Header", OnBack: func() {}, BackLabel: "Back"},
		Skeleton{Lines: 2, Avatar: true, Label: "Loading"},
		Image{Label: "Picture"},
		ImageCompare{Label: "Compare", Before: core.View{}, After: core.View{}, BeforeLabel: "Before"},
		ScrollView{Style: core.Style{Height: core.Pt(80)}, BottomBar: []core.Node{IconButton{Icon: IconPlus, Label: "Add"}},
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
