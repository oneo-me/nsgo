package base

import (
	"slices"
	"testing"
	"time"
)

func listItems() []SelectItem {
	return []SelectItem{
		{Value: "apple", Title: "Apple", Group: "Fruits"},
		{Value: "banana", Title: "Banana", Group: "Fruits", Disabled: true},
		{Value: "blueberry", Title: "Blueberry", Group: "Fruits"},
		{Value: "carrot", Title: "Carrot", Group: "Vegetables"},
		{Value: "celery", Title: "Celery", Group: "Vegetables"},
		{Value: "rice", Title: "Rice"},
	}
}

func shownFor(items []SelectItem, query string) *listShown {
	x := newListIndex(items)
	return newListShown(x, x.matching(query, nil))
}

// rowNames are a list's rows as "Header" or the item's index.
func rowItems(s *listShown) []int {
	out := []int{}
	for _, r := range s.rows {
		if r.header {
			out = append(out, -1)
		} else {
			out = append(out, r.item)
		}
	}
	return out
}

func TestListSectionsAreAdjacentItemsOfAGroup(t *testing.T) {
	if got := rowItems(shownFor(listItems(), "")); !slices.Equal(got, []int{-1, 0, 1, 2, -1, 3, 4, 5}) {
		t.Fatalf("rows = %v", got)
	}
	split := []SelectItem{{Value: "a", Group: "G"}, {Value: "b", Group: "H"}, {Value: "c", Group: "G"}}
	if got := rowItems(shownFor(split, "")); !slices.Equal(got, []int{-1, 0, -1, 1, -1, 2}) {
		t.Fatalf("the same group after another is a section of its own: %v", got)
	}
}

func TestListQueryKeepsMatchesAndDropsEmptySections(t *testing.T) {
	if got := rowItems(shownFor(listItems(), "er")); !slices.Equal(got, []int{-1, 2, -1, 4}) {
		t.Fatalf("rows = %v", got)
	}
	if got := rowItems(shownFor(listItems(), "rice")); !slices.Equal(got, []int{5}) {
		t.Fatalf("rows = %v", got)
	}
	if got := shownFor(listItems(), "zucchini").rows; len(got) != 0 {
		t.Fatalf("rows = %v", got)
	}
}

func TestListMovesSkipDisabledItemsAndWrap(t *testing.T) {
	s := shownFor(listItems(), "")
	for _, c := range []struct {
		from int
		by   moveKind
		want int
	}{
		{-1, moveNext, 0}, {-1, movePrevious, 5}, {0, moveNext, 2}, {2, movePrevious, 0}, {5, moveNext, 0},
		{0, movePrevious, 5}, {3, moveFirst, 0}, {0, moveLast, 5}, {1, moveNext, 2}, {1, movePrevious, 0},
	} {
		if got := s.step(c.from, listMove{kind: c.by}); got != c.want {
			t.Errorf("from %d by %d = %d, want %d", c.from, c.by, got, c.want)
		}
	}
	if got := shownFor([]SelectItem{{Value: "x", Disabled: true}}, "").step(-1, listMove{}); got != -1 {
		t.Errorf("nothing enabled: %d", got)
	}
}

func TestListPagesStopAtTheEnds(t *testing.T) {
	s := shownFor(listItems(), "")
	for _, c := range []struct {
		from int
		by   listMove
		want int
	}{
		{0, listMove{movePageDown, 2}, 3}, {3, listMove{movePageDown, 10}, 5}, {5, listMove{movePageUp, 3}, 2},
		{2, listMove{movePageUp, 10}, 0}, {-1, listMove{movePageDown, 3}, 0}, {-1, listMove{movePageUp, 3}, 5},
		{1, listMove{movePageDown, 1}, 2}, {1, listMove{movePageUp, 1}, 0},
	} {
		if got := s.step(c.from, c.by); got != c.want {
			t.Errorf("from %d by %+v = %d, want %d", c.from, c.by, got, c.want)
		}
	}
}

func TestTypeaheadFindsTheNextTitle(t *testing.T) {
	items := listItems()
	s, x := shownFor(items, ""), newListIndex(items)
	find := func(from int, typed string) int { return findTyped(s.enabled, s.rowOf(from), x.folded, typed) }
	for _, c := range []struct {
		from  int
		typed string
		want  int
	}{
		{-1, "c", 3}, {3, "c", 4}, {4, "cc", 3}, {3, "ce", 4}, {4, "cel", 4}, {0, "b", 2}, {-1, "r", 5}, {-1, "z", -1},
	} {
		if got := find(c.from, c.typed); got != c.want {
			t.Errorf("from %d typing %q = %d, want %d", c.from, c.typed, got, c.want)
		}
	}
}

func TestTypeaheadStartsAgainAfterAPause(t *testing.T) {
	start := time.Unix(100, 0)
	var ahead typeahead
	for _, c := range []struct {
		r    rune
		at   time.Duration
		want string
	}{{'C', 0, "c"}, {'e', 500 * time.Millisecond, "ce"}, {'l', 2 * time.Second, "l"}} {
		if got := ahead.push(c.r, start.Add(c.at)); got != c.want {
			t.Errorf("push(%c) = %q, want %q", c.r, got, c.want)
		}
	}
}

func TestListHighlightFollowsItsValue(t *testing.T) {
	items := listItems()
	s := shownFor(items, "")
	if got := s.follow(items, "celery"); got != 4 {
		t.Errorf("follow = %d", got)
	}
	if got := s.follow(items, "banana"); got != 0 {
		t.Errorf("a disabled value: %d", got)
	}
	if got := shownFor(items, "b").follow(items, "apple"); got != 2 {
		t.Errorf("filtered out: %d", got)
	}
}

func TestLongerQueriesSearchTheLastMatches(t *testing.T) {
	items := make([]SelectItem, 300)
	for i := range items {
		items[i] = SelectItem{Value: time.Duration(i).String(), Title: "Item " + time.Duration(i).String()}
	}
	x := newListIndex(items)
	last := x.matching("1", nil)
	for _, q := range []string{"12", "item 12", "item 120"} {
		narrowed := x.matching(q, last)
		if !slices.Equal(narrowed, x.matching(q, nil)) {
			t.Fatalf("%q narrowed differently", q)
		}
		last = narrowed
	}
}

func TestSelectedTitlesKeepTheirOrder(t *testing.T) {
	items := listItems()
	for _, c := range []struct {
		values []string
		want   string
	}{{[]string{"rice", "apple"}, "Rice, Apple"}, {[]string{"kiwi", "celery"}, "Celery"}, {nil, ""}} {
		if got := selectedTitles(items, c.values).Text; got != c.want {
			t.Errorf("%v = %q, want %q", c.values, got, c.want)
		}
	}
}
