package commitgraph

import (
	"fmt"
	"strings"
	"testing"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/nsgotest"
)

func c(hash string, parents ...string) Commit { return Commit{Hash: hash, Parents: parents} }

func TestLayoutJoinsBranchesAndMerges(t *testing.T) {
	var got []string
	for _, row := range Layout([]Commit{c("m", "a", "b"), c("b", "r"), c("a", "r"), c("r"), c("x")}) {
		got = append(got, fmt.Sprintf("%d:%d/%d %v", row.Column, row.Line, row.Columns, row.Lines))
	}
	// The merge opens a line to its second parent, which ends at the root
	// beside the first parent's line. A later root needs only its own
	// column, and takes column 0's line ID and color.
	want := []string{
		"0:0/2 [{0 -1 0 0} {1 -1 0 1}]",
		"1:1/2 [{0 0 0 0} {1 1 1 1}]",
		"0:0/2 [{0 0 0 0} {1 1 1 1}]",
		"0:0/2 [{0 0 0 -1} {1 1 0 -1}]",
		"0:0/1 []",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("graph:\n%s", strings.Join(got, "\n"))
	}
}

func TestLayoutJoinsAMergeToAnOpenLine(t *testing.T) {
	rows := Layout([]Commit{c("t", "p"), c("m", "a", "p"), c("a"), c("p")})
	// The merge's second parent is the line t already opened: no new line.
	if got := fmt.Sprint(rows[1].Lines); got != "[{0 0 0 0} {1 -1 1 1} {0 -1 1 0}]" {
		t.Fatalf("merge: %s", got)
	}
}

func TestLayoutBoundsOpenLines(t *testing.T) {
	var commits []Commit
	for i := range MaxLines + 2 {
		commits = append(commits, c(fmt.Sprint("tip", i), "root"))
	}
	rows := Layout(commits)
	if last := rows[len(rows)-1]; last.Column != -1 || last.Columns != MaxLines {
		t.Fatalf("past the limit: column %d of %d", last.Column, last.Columns)
	}
}

func TestFoldHidesMergedCommits(t *testing.T) {
	commits := []Commit{c("m2", "m1", "t"), c("t", "s1"), c("m1", "a", "s2"), c("s2", "s1"), c("s1", "r"), c("a", "r"), c("r")}
	// m1 folds s2, but not s1, which t builds on outside the merge.
	if got := Fold(commits, map[string]bool{"m1": true}); fmt.Sprint(got) != "map[s2:m1]" {
		t.Fatalf("m1: %v", got)
	}
	// m2 folds only t: its first parent reaches s1 through m1.
	if got := Fold(commits, map[string]bool{"m2": true}); fmt.Sprint(got) != "map[t:m2]" {
		t.Fatalf("m2: %v", got)
	}
}

// TestGraphTogglesAMerge collapses a merge from its button: its commit goes,
// the choice moves to the merge and the graph lays out what is left.
func TestGraphTogglesAMerge(t *testing.T) {
	commits := []Commit{c("m", "a", "b"), c("b", "a"), c("a")}
	var history *History
	var selection *base.Selection
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		selection = base.NewSelection(cx, false)
		selection.Set("b")
		history = NewHistory(cx, func() []Commit { return commits }, selection)
		return core.View{Style: core.Style{FlexGrow: 1}, Children: []core.Node{
			Graph{History: history, Item: func() Item { return history.At(0) }}}}
	}, nsgotest.Size(200, 100))
	app.Settle()
	app.Press("label:Collapse Merge")
	if got := fmt.Sprint(selection.Keys()); got != "[m]" {
		t.Fatalf("selection: %s", got)
	}
	if !app.Exists("label:Expand Merge") {
		t.Fatal(app.Dump())
	}
	if n := history.Len(); n != 2 {
		t.Fatalf("shown: %d", n)
	}
	if i := history.At(0); !i.Merge || i.Row.Columns != 1 {
		t.Fatalf("collapsed merge: %+v", i)
	}
	app.Press("label:Expand Merge")
	if n := history.Len(); n != 3 {
		t.Fatalf("shown: %d", n)
	}
}
