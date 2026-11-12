package commitgraph

import (
	"time"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/i18n"
)

// FormatDate writes a commit's date in local time, numerically in the
// language the app shows.
func FormatDate(cx *core.Scope, date time.Time) string {
	return i18n.FormatDate(i18n.Language(cx), date.Local(), i18n.DateOptions{Date: i18n.ShortStyle, Time: i18n.ShortStyle})
}

// Item is a shown commit of a History: its index in the history's commits
// and its graph.
type Item struct {
	Index int
	Hash  string
	Row   Row
	// Merge reports that the commit has commits to collapse.
	Merge bool
}

// History is a list of commits with its merges collapsed or expanded, and
// the graph of the commits that leaves shown. The app makes it with
// NewHistory and lists its rows with At.
type History struct {
	commits   func() []Commit
	selection *base.Selection
	collapsed *core.Signal[map[string]bool]
	shown     *core.Memo[shown]
}

// shown is what a History lists, and which merge hides each other commit.
type shown struct {
	items  []Item
	folded map[string]string
}

// NewHistory returns the history of commits, which it reads as signals,
// with every merge expanded. With a selection, one a collapsing merge hides
// moves to the merge.
func NewHistory(cx *core.Scope, commits func() []Commit, selection *base.Selection) *History {
	h := &History{commits: commits, selection: selection, collapsed: core.NewSignalFunc(cx, map[string]bool(nil), func(a, b map[string]bool) bool { return false })}
	h.shown = core.NewMemoFunc(cx, h.lay, func(a, b shown) bool { return false })
	return h
}

// lay folds the collapsed merges and lays out the commits that are left.
func (h *History) lay() shown {
	commits := h.commits()
	folded := Fold(commits, h.collapsed.Get())
	indexes := make([]int, 0, len(commits)-len(folded))
	list := make([]Commit, 0, len(commits)-len(folded))
	for i, commit := range commits {
		if folded[commit.Hash] != "" {
			continue
		}
		if len(folded) > 0 {
			parents := commit.Parents[:0:0]
			for _, parent := range commit.Parents {
				if folded[parent] == "" {
					parents = append(parents, parent)
				}
			}
			commit.Parents = parents
		}
		indexes = append(indexes, i)
		list = append(list, commit)
	}
	rows := Layout(list)
	items := make([]Item, len(rows))
	for i, row := range rows {
		// A collapsed merge has lost the parents it folds, not its merge.
		merge := len(commits[indexes[i]].Parents) > 1
		items[i] = Item{Index: indexes[i], Hash: list[i].Hash, Row: row, Merge: merge}
	}
	return shown{items, folded}
}

// Len returns how many commits show.
func (h *History) Len() int { return len(h.shown.Get().items) }

// At returns the i-th shown commit, the zero Item past the end as a list's
// cell may read it while the history shrinks.
func (h *History) At(i int) Item {
	items := h.shown.Get().items
	if i < 0 || i >= len(items) {
		return Item{Index: -1, Row: Row{Column: -1, Line: -1}}
	}
	return items[i]
}

// Collapsed reports whether the merge of hash hides the commits it brings
// in.
func (h *History) Collapsed(hash string) bool { return h.collapsed.Get()[hash] }

// Toggle collapses or expands the merge of hash.
func (h *History) Toggle(hash string) {
	next := map[string]bool{}
	for merge := range h.collapsed.GetUntracked() {
		next[merge] = true
	}
	if next[hash] {
		delete(next, hash)
	} else {
		next[hash] = true
	}
	h.collapsed.Set(next)
	h.keepSelection()
}

// keepSelection moves chosen commits a merge now hides to that merge.
func (h *History) keepSelection() {
	if h.selection == nil {
		return
	}
	folded := h.shown.GetUntracked().folded
	keys := h.selection.Keys()
	moved := false
	for i, key := range keys {
		hash, _ := key.(string)
		if merge := folded[hash]; merge != "" {
			keys[i], moved = merge, true
		}
	}
	if moved {
		h.selection.Set(keys...)
	}
}
