package main

import (
	"fmt"
	"slices"
	"time"

	"github.com/oneo-me/nsgo/base/commitgraph"
	"github.com/oneo-me/nsgo/core"

	"github.com/oneo-me/nsgo/examples/gallery/locales"
	"github.com/oneo-me/nsgo/ui"
	uigraph "github.com/oneo-me/nsgo/ui/commitgraph"
)

var historySubjects = []string{"Fix the empty state", "Keep the scroll position", "Add keyboard shortcuts", "Tidy the settings page",
	"Speed up the first load", "Support dark mode", "Update dependencies", "Handle a lost connection", "Write the upgrade notes", "Trim the build"}

var historyAuthors = []string{"Ada Lovelace", "Ken Thompson", "Grace Hopper", "Linus Torvalds"}

// demoHistory is a fictional repository of three hundred commits: a main
// line with branches leaving it, some side by side, and merging back. It
// grows oldest first, so its reverse lists each commit after its children.
func demoHistory() []uigraph.Commit {
	random := seeded(7)
	pick := func(n int) int { return int(random() * float64(n)) }
	when := time.Date(2026, 3, 2, 9, 0, 0, 0, time.UTC)
	var commits []uigraph.Commit
	add := func(parents ...string) *uigraph.Commit {
		when = when.Add(time.Duration(40+pick(600)) * time.Minute)
		i := len(commits)
		commits = append(commits, uigraph.Commit{Hash: fmt.Sprintf("%08x%04d", uint32(i+1)*2654435761, i), Parents: parents, Date: when,
			Subject: historySubjects[pick(len(historySubjects))], Author: historyAuthors[pick(len(historyAuthors))], More: pick(5) == 0})
		return &commits[i]
	}
	type branch struct {
		name string
		tip  int
		left int
	}
	main := 0
	add()
	var open []branch
	for len(commits) < 300 {
		switch step := pick(10); {
		case step < 2 && len(open) < 4:
			name := fmt.Sprintf("feature/%d", len(commits))
			add(commits[main].Hash)
			open = append(open, branch{name, len(commits) - 1, 2 + pick(8)})
		case step < 7 && len(open) > 0:
			at := pick(len(open))
			b := &open[at]
			if b.left > 0 {
				b.left--
				add(commits[b.tip].Hash)
				b.tip = len(commits) - 1
				continue
			}
			merged := add(commits[main].Hash, commits[b.tip].Hash)
			merged.Subject = "Merge " + b.name
			main = len(commits) - 1
			open = slices.Delete(open, at, at+1)
		default:
			add(commits[main].Hash)
			main = len(commits) - 1
		}
	}
	for i, b := range open {
		commits[b.tip].Refs = append(commits[b.tip].Refs, uigraph.Ref{Name: b.name, Kind: uigraph.RefBranch})
		if i == 0 {
			commits[b.tip].Refs = append(commits[b.tip].Refs, uigraph.Ref{Name: "origin/" + b.name, Kind: uigraph.RefRemote})
		}
	}
	commits[main].Head = true
	commits[main].Refs = append(commits[main].Refs, uigraph.Ref{Name: "main", Kind: uigraph.RefHead},
		uigraph.Ref{Name: "origin/main", Kind: uigraph.RefRemote, Current: true}, uigraph.Ref{Name: "v1.4.0", Kind: uigraph.RefTag})
	for i, j := 0, len(commits)-1; i < j; i, j = i+1, j-1 {
		commits[i], commits[j] = commits[j], commits[i]
	}
	return commits
}

// graphCommits is what base's graph knows of commits.
func graphCommits(commits []uigraph.Commit) []commitgraph.Commit {
	out := make([]commitgraph.Commit, len(commits))
	for i, c := range commits {
		out[i] = commitgraph.Commit{Hash: c.Hash, Parents: c.Parents}
	}
	return out
}

// commitGraphDemo is the history filling the page, with its base form in
// the panel.
func commitGraphDemo(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	fin := ui.FinOf(cx)
	last := core.NewSignal(cx, "—")
	commits := demoHistory()
	selection := ui.NewSelection(cx, false)
	return stagePage(cx,
		core.View{StyleFunc: func() core.Style {
			return fin.Card().Over(core.Style{FlexGrow: 1, FlexShrink: 1, MinHeight: core.Pt(0), CornerRadius: ui.RadiusInset,
				BorderWidth: 1, BorderColor: colors.Border(), Background: colors.Surface(), Overflow: core.Hidden})
		}, Children: []core.Node{uigraph.View{Label: tr.CommitGraphHistory(), TestID: "commits", Selection: selection,
			Commits:    func() []uigraph.Commit { return commits },
			OnActivate: func(c uigraph.Commit) { last.Set(c.Subject) }}}},
		statusLine(cx, tr.CommitGraphActivated(), "commit-activated", last.Get),
		baseCommitGraphForm(cx, last).stage("base"))
}
