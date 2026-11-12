package core

import (
	"slices"
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

func propsOf(b *driver.MountBatch) map[NodeID]driver.ViewProps {
	out := map[NodeID]driver.ViewProps{}
	for _, it := range b.Items {
		if it.Kind == driver.ItemViewProps {
			out[it.ID] = *it.ViewProps
		}
	}
	return out
}

func TestAccessibilityPropsFollowTheirSignals(t *testing.T) {
	var now *Signal[float32]
	var open, hidden *Signal[bool]
	actions := []AccessibilityAction{{Name: ActionIncrement}, {Name: "reset", Label: "Reset"}}
	h := newHost(t, Size{Width: 100, Height: 100}, func(cx *Scope) Node {
		now = NewSignal(cx, float32(40))
		open = NewSignal(cx, false)
		hidden = NewSignal(cx, false)
		return View{Children: []Node{
			View{TestID: "bar", Role: RoleProgressBar, Hint: "Downloads the file",
				AccessibilityValue:   func() AccessibilityValue { return AccessibilityValue{Max: 100, Now: now.Get()} },
				AccessibilityActions: actions, LiveRegion: LiveRegionPolite},
			View{TestID: "menu", Role: RoleButton, Expanded: open.Get},
			View{TestID: "hidden", AccessibilityElementsHidden: hidden.Get},
			View{TestID: "nohide", ImportantForAccessibility: ImportantNoHideDescendants},
			View{TestID: "no", ImportantForAccessibility: ImportantNo},
		}}
	})
	props := propsOf(h.commit())
	bar := props[h.find("bar").id]
	if bar.Hint != "Downloads the file" || bar.Value != (AccessibilityValue{Max: 100, Now: 40}) || !bar.Value.IsRange() ||
		bar.LiveRegion != LiveRegionPolite || !slices.Equal(bar.ActionList(), actions) || !bar.HasAction("reset") {
		t.Errorf("bar props = %+v", bar)
	}
	if m := props[h.find("menu").id]; !m.Expandable || m.Expanded {
		t.Errorf("menu props = %+v, want expandable and collapsed", m)
	}
	if p := props[h.find("nohide").id]; !p.ElementsHidden || p.Accessible != driver.AccessibleNo {
		t.Errorf("no-hide-descendants props = %+v", p)
	}
	if p := props[h.find("no").id]; p.ElementsHidden || p.Accessible != driver.AccessibleNo {
		t.Errorf("importantForAccessibility no props = %+v", p)
	}

	now.Set(60)
	open.Set(true)
	hidden.Set(true)
	props = propsOf(h.commit())
	if len(props) != 3 {
		t.Errorf("changes sent %d props, want the three that changed", len(props))
	}
	if p := props[h.find("bar").id]; p.Value.Now != 60 || p.Actions != bar.Actions {
		t.Errorf("bar props = %+v after the value changed, keeping its actions", p)
	}
	if !props[h.find("menu").id].Expanded || !props[h.find("hidden").id].ElementsHidden {
		t.Error("expanded and hidden states were not sent")
	}
	if k := kinds(h.commit()); k[driver.ItemViewProps] != 0 {
		t.Errorf("an idle commit sent %v", k)
	}
}

func TestALabelChangingAloneGoesAlone(t *testing.T) {
	var count *Signal[int]
	var off *Signal[bool]
	h := newHost(t, Size{Width: 100, Height: 100}, func(cx *Scope) Node {
		count = NewSignal(cx, 1)
		off = NewSignal(cx, false)
		return View{TestID: "v", Role: RoleButton, Disabled: off.Get,
			LabelFunc: func() string { return string(rune('0' + count.Get())) }}
	})
	if p := propsOf(h.commit())[h.find("v").id]; p.Label != "1" {
		t.Fatalf("label = %q", p.Label)
	}
	count.Set(2)
	b := h.commit()
	if k := kinds(b); k[driver.ItemViewProps] != 0 || k[driver.ItemAccessibilityLabel] != 1 {
		t.Fatalf("a label change sent %v, want one ItemAccessibilityLabel", k)
	}
	for _, it := range b.Items {
		if it.Kind == driver.ItemAccessibilityLabel && (it.ID != h.find("v").id || it.Value != "2") {
			t.Errorf("label item = %+v", it)
		}
	}
	count.Set(3)
	off.Set(true)
	if p, ok := propsOf(h.commit())[h.find("v").id]; !ok || p.Label != "3" || !p.Disabled {
		t.Errorf("a label changing with other props sent %+v, want the whole props", p)
	}
}

func TestAGroupReadsItsTexts(t *testing.T) {
	var price *Signal[string]
	h := newHost(t, Size{Width: 200, Height: 100}, func(cx *Scope) Node {
		price = NewSignal(cx, "$10")
		return View{Children: []Node{
			View{TestID: "row", Accessible: true, Children: []Node{
				Text{Content: "Apple"},
				View{Children: []Node{Text{ContentFunc: price.Get}}},
				Text{Content: "secret", Style: Style{Display: DisplayNone}},
				Icon{Label: "on sale"},
			}},
			View{TestID: "labelled", Accessible: true, Label: "Own", Children: []Node{Text{Content: "x"}}},
		}}
	})
	props := propsOf(h.commit())
	if p := props[h.find("row").id]; p.Label != "Apple $10 on sale" || p.Accessible != AccessibleYes {
		t.Errorf("group props = %+v, want its texts as the label", p)
	}
	if p := props[h.find("labelled").id]; p.Label != "Own" {
		t.Errorf("a labelled group reads %q", p.Label)
	}
	price.Set("$12")
	b := h.commit()
	found := false
	for _, it := range b.Items {
		if it.Kind == driver.ItemAccessibilityLabel && it.ID == h.find("row").id {
			found = it.Value == "Apple $12 on sale"
		}
	}
	if !found {
		t.Errorf("the group's label did not follow its text: %v", kinds(b))
	}
}

func TestAccessibilityActionsAndFocus(t *testing.T) {
	var got []string
	var off *Signal[bool]
	var ref Ref
	h := newHost(t, Size{Width: 100, Height: 100}, func(cx *Scope) Node {
		off = NewSignal(cx, false)
		return View{TestID: "v", Ref: &ref, Disabled: off.Get, Role: RoleAdjustable,
			AccessibilityActions:  []AccessibilityAction{{Name: ActionIncrement}, {Name: ActionDecrement}},
			OnAccessibilityAction: func(name string) { got = append(got, name) }}
	})
	b := h.commit()
	id := h.find("v").id
	for _, it := range b.Items {
		if it.Kind == driver.ItemEvents && it.ID == id && !it.Events.Has(driver.EventAccessibilityAction) {
			t.Error("the view does not listen for accessibility actions")
		}
	}
	h.tree.Dispatch(driver.Event{Kind: driver.EventAccessibilityAction, ID: id, Text: ActionIncrement})
	off.Set(true)
	h.commit()
	h.tree.Dispatch(driver.Event{Kind: driver.EventAccessibilityAction, ID: id, Text: ActionDecrement})
	if !slices.Equal(got, []string{ActionIncrement}) {
		t.Errorf("actions = %v, want the one before the view was disabled", got)
	}
	ref.SetAccessibilityFocus()
	b = h.commit()
	if k := kinds(b); k[driver.ItemAccessibilityFocus] != 1 || b.Items[len(b.Items)-1].ID != id {
		t.Errorf("SetAccessibilityFocus sent %v", k)
	}
	if !driver.IsStandardAction(ActionMagicTap) || driver.IsStandardAction("reset") {
		t.Error("IsStandardAction")
	}
}
