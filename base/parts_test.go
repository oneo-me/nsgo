package base

import (
	"reflect"
	"testing"
)

func TestInitials(t *testing.T) {
	for name, want := range map[string]string{"Jason Lee": "JL", "Foo Bar Dar": "FB", "huacnlee": "HU", "  ada   lovelace ": "AL",
		"Ω": "Ω", "": "", "王小明": "王", "さくら": "さ", "José Álvarez": "JÁ"} {
		if got := initials(name); got != want {
			t.Errorf("initials(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestPaginationItems(t *testing.T) {
	pages := func(items []PaginationItem) []int {
		var out []int
		for _, i := range items {
			if i.Kind == PaginationEllipsis {
				out = append(out, -i.Start)
			} else {
				out = append(out, i.Page)
			}
		}
		return out
	}
	for _, c := range []struct {
		page, total, visible int
		want                 []int
	}{
		{1, 1, 5, nil},
		{2, 4, 5, []int{1, 2, 3, 4}},
		{5, 20, 7, []int{1, -2, 3, 4, 5, 6, 7, -8, 20}},
		{1, 10000, 5, []int{1, 2, 3, -4, 10000}},
		{10000, 10000, 5, []int{1, -2, 9998, 9999, 10000}},
		{99, 20, 7, []int{1, -2, 17, 18, 19, 20}},
	} {
		if got := pages(paginationItems(c.page, c.total, c.visible)); !reflect.DeepEqual(got, c.want) {
			t.Errorf("page %d of %d shows %v, want %v", c.page, c.total, got, c.want)
		}
	}
}

func TestOtpEdits(t *testing.T) {
	for _, c := range []struct {
		shown    string
		deleted  int
		inserted string
		length   int
		want     string
	}{
		{"12", 1, "", 4, "1"}, {"12", 0, "３", 4, "123"}, {"1234", 0, "5", 4, "1234"}, {"12", 0, "a", 4, "12"}, {"", 1, "", 4, ""},
		{"12", 0, "987-654", 6, "987654"}, {"123456", 0, "9876543", 6, "987654"}, {"12", 0, "3-4", 6, "1234"}, {"1234", 0, "5678", 6, "123456"},
	} {
		if got := otpEdit(c.shown, c.deleted, c.inserted, c.length); got != c.want {
			t.Errorf("otpEdit(%q, %d, %q, %d) = %q, want %q", c.shown, c.deleted, c.inserted, c.length, got, c.want)
		}
	}
	for _, c := range []struct {
		before, after string
		lost          int
		gained        string
	}{{"12", "123", 0, "3"}, {"123", "12", 1, ""}, {"123", "", 3, ""}, {"1２", "13", 1, "3"}, {"1234", "5", 4, "5"}} {
		if lost, gained := otpDiff(c.before, c.after); lost != c.lost || gained != c.gained {
			t.Errorf("otpDiff(%q, %q) = %d %q", c.before, c.after, lost, gained)
		}
	}
	echoes := []string{"1", "13", "135"}
	if !otpEchoed(&echoes, "1") || len(echoes) != 2 || !otpEchoed(&echoes, "135") || len(echoes) != 0 || otpEchoed(&echoes, "") {
		t.Errorf("echoes left %v", echoes)
	}
	if otpGroups(6, 0) != 1 || otpGroups(6, 20) != 6 || otpGroups(0, 0) != 1 || otpGroups(5, 2) != 2 {
		t.Error("group counts are not clamped")
	}
}

func TestTreeModel(t *testing.T) {
	f := newTreeFlat([]TreeItem{
		{ID: "a", Label: "Apple", Children: []TreeItem{{ID: "a/1", Label: "One"}, {ID: "a/2", Label: "Two", Disabled: true}}},
		{ID: "b", Label: "Berry", Disabled: true, Children: []TreeItem{{ID: "b/1", Label: "Hidden"}}},
		{ID: "c", Label: "Cherry"},
	})
	rows := treeRows(f, f.open([]string{"a", "missing", "c"}))
	if !reflect.DeepEqual(rows, []int{0, 1, 2, 3, 5}) {
		t.Fatalf("rows %v", rows)
	}
	if got := treeStep(f, rows, 1, stepNext); got != 5 {
		t.Errorf("the step past a disabled row reached node %d", got)
	}
	if got := treeStep(f, rows, 5, stepNext); got != 0 {
		t.Errorf("the step past the last reached node %d", got)
	}
	if got := treeStep(f, rows, -1, stepLast); got != 5 {
		t.Errorf("the last row is node %d", got)
	}
	// A selection hidden under a collapsed item is acted on from its row.
	if got := treeCursor(f, treeRows(f, nil), f.find("a/1")); got != 0 {
		t.Errorf("the cursor of a hidden selection is node %d", got)
	}
	if got := treeTyped(f, rows, 0, "c"); got != 5 {
		t.Errorf("typing c reached node %d", got)
	}
	if got := treeTyped(f, rows, 0, "aa"); got != 0 {
		t.Errorf("typing aa from Apple reached node %d", got)
	}
}

func TestCarouselTrack(t *testing.T) {
	loop := carouselTrack{snaps: []float64{0, 210, 420}, period: 630, runway: 620, first: 630, viewport: 200}
	if !loop.ready() || (carouselTrack{period: 630, first: 0}).ready() {
		t.Error("a loop is ready once its runway is laid out")
	}
	if r, _ := loop.rest(2); r != 1050 {
		t.Errorf("rest = %v", r)
	}
	if i, at, cycle, _ := loop.nearest(1270); i != 0 || at != 1260 || cycle != 1 {
		t.Errorf("nearest past the seam = %d %v %d", i, at, cycle)
	}
	// Slides resting where the last does are one stop, the first of them.
	end := carouselTrack{snaps: []float64{0, 100, 100}}
	if i, _, _, _ := end.nearest(99); i != 1 {
		t.Errorf("nearest of the end = %d", i)
	}
}
