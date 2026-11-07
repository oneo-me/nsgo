package base

import (
	"slices"
	"testing"
)

func TestFilterCommandsRanks(t *testing.T) {
	items := []CommandItem{
		{ID: "keyword", Label: "Preferences", Keywords: []string{"open"}},
		{ID: "holds", Label: "Reopen"},
		{ID: "prefix", Label: "Open file"},
		{ID: "exact", Label: "Open"},
		{ID: "both", Label: "Open recent file"},
	}
	ids := func(items []CommandItem) []string {
		out := make([]string, len(items))
		for i, item := range items {
			out[i] = item.ID
		}
		return out
	}
	if got := ids(FilterCommands(items, "OPEN")); !slices.Equal(got, []string{"exact", "prefix", "both", "holds", "keyword"}) {
		t.Fatalf("ranked %v", got)
	}
	if got := ids(FilterCommands(items, "  file open ")); !slices.Equal(got, []string{"prefix", "both"}) {
		t.Fatalf("every word must match: %v", got)
	}
	if got := ids(FilterCommands(items, " ")); !slices.Equal(got, []string{"keyword", "holds", "prefix", "exact", "both"}) {
		t.Fatalf("no words reorder: %v", got)
	}
}

func TestCommandGathersGroupsAsTheyRank(t *testing.T) {
	x := newCommandIndex([]CommandItem{
		{ID: "a", Label: "Alpha", Group: "One"},
		{ID: "b", Label: "Open", Group: "Two"},
		{ID: "c", Label: "Reopen", Group: "One"},
		{ID: "d", Label: "Open door", Group: "Two"},
	})
	shown := newCommandShown(x, x.matching("open", nil), true)
	var rows []string
	for _, r := range shown.rows {
		switch r.kind {
		case commandHeaderRow:
			rows = append(rows, "# "+r.heading)
		case commandSeparatorRow:
			rows = append(rows, "-")
		default:
			rows = append(rows, x.items[r.item].ID)
		}
	}
	if !slices.Equal(rows, []string{"# Two", "b", "d", "-", "# One", "c"}) {
		t.Fatalf("rows %v", rows)
	}
	if next := stepAmong(shown.enabled, shown.rowOf(3), listMove{kind: moveNext}); next != 2 {
		t.Fatalf("a move past the section went to %d", next)
	}
}
