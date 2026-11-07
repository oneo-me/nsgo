package main

import (
	"fmt"
	"strings"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/base/diff"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"

	"github.com/oneo-me/nsgo/examples/gallery/locales"
	"github.com/oneo-me/nsgo/ui"
	uidiff "github.com/oneo-me/nsgo/ui/diff"
)

const diffOld = `package store

import (
	"errors"
	"sync"
)

// Store keeps values by key.
type Store struct {
	mu     sync.Mutex
	values map[string]string
}

// New returns an empty store.
func New() *Store {
	return &Store{values: map[string]string{}}
}

// Get returns the value of key.
func (s *Store) Get(key string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.values[key]
	if !ok {
		return "", errors.New("not found")
	}
	return v, nil
}

// Set sets the value of key.
func (s *Store) Set(key, value string) {
	s.mu.Lock()
	s.values[key] = value
	s.mu.Unlock()
}

// Len counts the keys.
func (s *Store) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.values)
}
`

const diffNew = `package store

import (
	"errors"
	"sync"
)

// ErrNotFound is what Get returns for a key the store lacks.
var ErrNotFound = errors.New("store: not found")

// Store keeps values by key, safe for concurrent use.
type Store struct {
	mu     sync.RWMutex
	values map[string]string
}

// New returns an empty store.
func New() *Store {
	return &Store{values: map[string]string{}}
}

// Get returns the value of key.
func (s *Store) Get(key string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.values[key]
	if !ok {
		return "", ErrNotFound
	}
	return v, nil
}

// Set sets the value of key.
func (s *Store) Set(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[key] = value
}

// Len counts the keys.
func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.values)
}
`

const diffPatch = `diff --git a/README.md b/README.md
index 3b18e51..a7c4f2d 100644
--- a/README.md
+++ b/README.md
@@ -1,6 +1,7 @@
 # Store

-A map with a lock.
+A map with a lock, safe for concurrent use.

 ## Use

+Get returns ErrNotFound for a key the store lacks.
diff --git a/store.go b/store.go
index 1f2e3d4..5a6b7c8 100644
--- a/store.go
+++ b/store.go
@@ -9,7 +9,7 @@ import (
 // Store keeps values by key.
 type Store struct {
-	mu     sync.Mutex
+	mu     sync.RWMutex
 	values map[string]string
 }

@@ -40,6 +40,6 @@ func (s *Store) Set(key, value string) {
 // Len counts the keys.
 func (s *Store) Len() int {
-	s.mu.Lock()
-	defer s.mu.Unlock()
+	s.mu.RLock()
+	defer s.mu.RUnlock()
 	return len(s.values)
 }
diff --git a/old_test.go b/store_test.go
similarity index 92%
rename from old_test.go
rename to store_test.go
`

// diffDemo is ui/diff's View: two texts side by side or unified,
// the words changed marked, unchanged lines collapsed, changes navigated,
// their lines staged and discarded, and a patch of several files.
func diffDemo(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	fin := ui.FinOf(cx)
	oldText, newText := core.NewSignal(cx, diffOld), core.NewSignal(cx, diffNew)
	unified, patch, wrap := core.NewSignal(cx, false), core.NewSignal(cx, false), core.NewSignal(cx, false)
	inline := core.NewSignal(cx, diff.InlineWordAlt)
	stats := core.NewSignal(cx, diff.Stats{})
	last := core.NewSignal(cx, "—")
	ref := &diff.Ref{}
	small := func(label string, onPress func()) core.Node {
		return ui.Button{Text: label, Variant: ui.ButtonSecondary, Size: ui.SizeSmall, OnPress: onPress}
	}
	modes := []ui.Option[diff.Inline]{{Value: diff.InlineWordAlt, Label: tr.DiffWords()}, {Value: diff.InlineWord, Label: tr.DiffEachWord()},
		{Value: diff.InlineChar, Label: tr.DiffCharacters()}, {Value: diff.InlineNone, Label: tr.DiffNone()}}
	return stagePage(cx,
		core.View{StyleFunc: func() core.Style {
			return fin.Card().Over(core.Style{FlexGrow: 1, FlexShrink: 1, MinHeight: core.Pt(0), CornerRadius: ui.RadiusInset,
				BorderWidth: 1, BorderColor: colors.Border(), Background: colors.Surface(), Overflow: core.Hidden})
		}, Children: []core.Node{
			core.Dynamic(func(cx *core.Scope) core.Node {
				d := uidiff.View{
					Ref: ref, TestID: "diff", Language: "go", Unified: unified.Get(), Inline: inline.Get(), SoftWrap: wrap.Get(),
					RenderRegionActions: func(cx *core.Scope, r diff.Region) core.Node {
						tr := locales.For(cx)
						return uidiff.RegionActions{
							OnStage: func() { ref.AcceptRegion(r); last.Set(tr.DiffStaged()) },
							OnDiscard: func() {
								base.ShowAlertDialog(cx, base.AlertDialog{Title: tr.DiffDiscardTheseChanges(), Actions: []core.AlertAction{
									{Label: tr.DiffCancel(), Role: core.AlertCancel},
									{Label: tr.DiffDiscard(), Role: core.AlertDestructive},
								}}, func(label string) {
									if label == tr.DiffDiscard() {
										ref.RevertRegion(r)
										last.Set(tr.DiffDiscarded())
									}
								})
							},
						}
					},
					OnLineSelectionChange: func(r *diff.LineRange) {
						if r != nil {
							last.Set(tr.DiffLinesSelected(r.Start+1, r.End+1))
						}
					},
					OnStatsChange: stats.Set,
					OnChange:      func(c diff.Change) { oldText.Set(c.OldValue); newText.Set(c.NewValue) },
					OnLineNumberPress: func(l diff.Line) {
						last.Set(map[diff.Side]string{diff.Old: tr.DiffLineOfOld(l.Line), diff.New: tr.DiffLineOfNew(l.Line)}[l.Side])
					},
					OnFileToggle: func(f diff.File) { last.Set(tr.DiffCollapsed(f.Path, onOff(tr, f.Collapsed))) },
					Style:        core.Style{FlexGrow: 1},
				}
				if patch.Get() {
					d.Language = ""
					d.RenderRegionActions = nil
					d.Patch = func() string { return diffPatch }
					d.RenderSeparator = func(cx *core.Scope, s diff.Separator) core.Node {
						label := tr.DiffLines(s.Lines)
						if s.Header != "" {
							label = "@@ " + strings.TrimSpace(s.Header)
						}
						return core.Text{Content: label, MaxLines: 1, Style: core.Style{FontFamily: ui.Mono, FontSize: 11}, StyleFunc: ui.Colored(ui.ColorsOf(cx).Muted)}
					}
				} else {
					d.OldValue, d.NewValue = oldText.Get, newText.Get
				}
				return d
			}),
		}},
		Section{Title: tr.TerminalOptions(), Note: "diff.View", Children: []core.Node{
			SwitchRow{Label: tr.DiffShowAPatch(), Code: "Patch", Checked: patch.Get, OnChange: patch.Set},
			SwitchRow{Label: tr.DiffOneColumn(), Code: "Unified", Checked: unified.Get, OnChange: unified.Set},
			SwitchRow{Label: tr.DiffSoftWrap(), Code: "SoftWrap", Checked: wrap.Get, OnChange: wrap.Set},
			choiceKnob("Inline", inline, modes...),
		}},
		Section{Title: tr.KitActions(), Note: "diff.Ref", Children: []core.Node{
			ui.Wrap{Children: []core.Node{
				ui.Button{Text: tr.DiffPreviousChange(), Icon: lucide.ArrowUp, Label: tr.DiffPreviousChange(), Variant: ui.ButtonSecondary, Size: ui.SizeSmall, OnPress: func() {
					i, _ := ref.PreviousHunk()
					last.Set(tr.DiffChange(i + 1))
				}},
				ui.Button{Text: tr.DiffNextChange(), Icon: lucide.ArrowDown, Label: tr.DiffNextChange(), Variant: ui.ButtonSecondary, Size: ui.SizeSmall, OnPress: func() {
					i, _ := ref.NextHunk()
					last.Set(tr.DiffChange(i + 1))
				}},
				small(tr.DiffExpandEveryLine(), ref.ExpandAll),
				small(tr.DiffCollapseUnchangedLines(), ref.CollapseAll),
				small(tr.DiffStartAgain(), func() { oldText.Set(diffOld); newText.Set(diffNew) }),
			}},
		}},
		Section{Title: tr.TerminalStatus(), Note: "OnStatsChange", Children: []core.Node{
			core.Text{Style: core.Style{FontSize: 12, FontFamily: ui.Mono}, SpansFunc: func() []core.Span {
				s := stats.Get()
				return []core.Span{{Content: fmt.Sprintf("+%d", s.Added), StyleFunc: ui.Colored(colors.Up)},
					{Content: " "}, {Content: fmt.Sprintf("−%d", s.Deleted), StyleFunc: ui.Colored(colors.Down)},
					{Content: " · " + tr.DiffChanges(s.Hunks), StyleFunc: ui.Colored(colors.Muted)}}
			}},
			core.Text{ContentFunc: last.Get, Style: core.Style{FontSize: 12, FontFamily: ui.Mono}, StyleFunc: ui.Colored(colors.Muted)},
		}})
}
