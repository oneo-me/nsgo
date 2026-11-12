package main

import (
	"strings"
	"testing"

	"github.com/nsgo-dev/nsgo/nsgotest"
)

// TestCommitGraph lists the fictional history, collapses a merge and
// activates a commit.
func TestCommitGraph(t *testing.T) {
	app := startDesktop(t, nsgotest.Size(1200, 900))
	open(app, "CommitGraph")
	app.Find("commits >> text:origin/main")
	app.Screenshot("commit-graph")
	app.Press("commits >> label:Collapse Merge >> nth=0")
	app.Find("commits >> label:Expand Merge")
	app.Press("commits >> text:/^Merge feature/ >> nth=0")
	app.Key("enter")
	if got := app.Find("id:commit-activated").Text; !strings.HasPrefix(got, "Merge feature") {
		t.Fatalf("activated %q", got)
	}
	app.Screenshot("commit-graph-collapsed")
}
