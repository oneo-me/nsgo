package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/oneo-me/nsgo/base/diff"
	"github.com/oneo-me/nsgo/core"

	"github.com/oneo-me/nsgo/examples/gallery/locales"
	"github.com/oneo-me/nsgo/ui"
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

// diffStyles is the gallery's diff look, base having none, for its
// StylesFunc.
func diffStyles(cx *core.Scope) func() diff.Styles {
	return ui.Themed(cx, func() diff.Styles { return diffParts(cx) })
}

// diffParts are the diff's styles, read where ui.ColorsOf's colors are.
func diffParts(cx *core.Scope) diff.Styles {
	colors := ui.ColorsOf(cx).Now()
	added, deleted := colors.Up, colors.Down
	return diff.Styles{
		Side:             core.Style{Borders: core.Edges{Right: 1}, BorderColor: colors.Line},
		LineNumber:       core.Style{Color: colors.Faint, FontSize: 12},
		LineAdded:        core.Style{Background: ui.Alpha(added, 0.12)},
		LineDeleted:      core.Style{Background: ui.Alpha(deleted, 0.12)},
		Filler:           core.Style{Background: ui.Alpha(colors.Inset, 0.5)},
		WordAdded:        core.Style{Background: ui.Alpha(added, 0.30)},
		WordDeleted:      core.Style{Background: ui.Alpha(deleted, 0.30)},
		IndicatorAdded:   core.Style{Background: added, Color: added},
		IndicatorDeleted: core.Style{Background: deleted, Color: deleted},
		Content:          core.Style{Color: colors.Text},
		Separator:        core.Style{Background: colors.Inset, Color: colors.Muted, Padding: core.Symmetric(0, 12), Cursor: core.CursorPointer},
		FileHeader: core.Style{Background: colors.Raised, Color: colors.Text, Padding: core.Symmetric(0, 12), Borders: core.Edges{Bottom: 1},
			BorderColor: colors.Border, Cursor: core.CursorPointer},
		LineSelected:  core.Style{Background: ui.Alpha(colors.Accent, 0.18)},
		Annotation:    core.Style{Padding: core.Symmetric(6, 12)},
		AddAnnotation: core.Style{Padding: core.Edges{Right: 2}},
	}
}

// diffDemo is base/diff's DiffView: two texts side by side or unified,
// the words changed marked, unchanged lines collapsed, changes navigated,
// reverted and kept, and a patch of several files.
func diffDemo(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	fin := ui.FinOf(cx)
	oldText, newText := core.NewSignal(cx, diffOld), core.NewSignal(cx, diffNew)
	unified, patch, wrap := core.NewSignal(cx, false), core.NewSignal(cx, false), core.NewSignal(cx, false)
	// The review's comments, under the lines they are on.
	type comment struct {
		diff.Annotation
		lines diff.LineRange
	}
	comments := core.NewSignalFunc(cx, []comment(nil), slices.Equal)
	inline := core.NewSignal(cx, diff.InlineWordAlt)
	stats := core.NewSignal(cx, diff.Stats{})
	last := core.NewSignal(cx, "—")
	ref := &diff.Ref{}
	pad := core.Points(16)
	if shell.desktop() {
		pad = 24
	}
	toggle := func(s *core.Signal[bool]) func() { return func() { s.Set(!s.GetUntracked()) } }
	small := func(label string, onPress func()) core.Node {
		return ui.Button{Text: label, Variant: ui.ButtonSecondary, Small: true, OnPress: onPress}
	}
	modes := []ui.Option[diff.Inline]{{Value: diff.InlineWordAlt, Label: tr.DiffWords()}, {Value: diff.InlineWord, Label: tr.DiffEachWord()},
		{Value: diff.InlineChar, Label: tr.DiffCharacters()}, {Value: diff.InlineNone, Label: tr.DiffNone()}}
	return core.View{StyleFunc: func() core.Style {
		return fillPage.Merge(core.Style{Background: colors.Background(), Padding: core.All(pad), Gap: 12})
	}, Children: []core.Node{
		core.View{Style: core.Style{Direction: core.Row, Wrap: core.Wrap, AlignItems: core.AlignCenter, Gap: 8}, Children: []core.Node{
			Tool{Label: tr.DiffShowAPatch(), Text: tr.DiffPatch(), On: patch.Get, OnPress: toggle(patch)},
			Tool{Label: tr.DiffOneColumn(), Text: tr.DiffUnified(), On: unified.Get, OnPress: toggle(unified)},
			Tool{Label: tr.DiffSoftWrap(), Text: tr.DiffWrap(), On: wrap.Get, OnPress: toggle(wrap)},
			core.View{Style: core.Style{Width: core.Pt(360)}, Children: []core.Node{ui.Segmented[diff.Inline]{Options: modes, Value: inline.Get, OnChange: inline.Set, Small: true}}},
			Tool{Label: tr.DiffPreviousChange(), Text: "↑", OnPress: func() {
				i, _ := ref.PreviousHunk()
				last.Set(tr.DiffChange(i + 1))
			}},
			Tool{Label: tr.DiffNextChange(), Text: "↓", OnPress: func() {
				i, _ := ref.NextHunk()
				last.Set(tr.DiffChange(i + 1))
			}},
			Tool{Label: tr.DiffExpandEveryLine(), Text: tr.DiffExpand(), OnPress: ref.ExpandAll},
			Tool{Label: tr.DiffCollapseUnchangedLines(), Text: tr.DiffCollapse(), OnPress: ref.CollapseAll},
			Tool{Label: tr.DiffStartAgain(), Text: tr.DiffReset(), OnPress: func() { oldText.Set(diffOld); newText.Set(diffNew) }},
		}},
		core.View{StyleFunc: func() core.Style {
			return fin.Card().Over(core.Style{FlexGrow: 1, FlexShrink: 1, MinHeight: core.Pt(0), CornerRadius: ui.RadiusInset,
				BorderWidth: 1, BorderColor: colors.Border(), Background: colors.Surface(), Overflow: core.Hidden})
		}, Children: []core.Node{
			core.Dynamic(func(cx *core.Scope) core.Node {
				colors := ui.ColorsOf(cx)
				d := diff.DiffView{
					Ref: ref, TestID: "diff", Language: "go", Unified: unified.Get(), Inline: inline.Get(), SoftWrap: wrap.Get(),
					Annotations: func() []diff.Annotation {
						var out []diff.Annotation
						for _, c := range comments.Get() {
							out = append(out, c.Annotation)
						}
						return out
					},
					RenderAnnotation: func(cx *core.Scope, a diff.Annotation) core.Node {
						tr := locales.For(cx)
						colors := ui.ColorsOf(cx)
						fin := ui.FinOf(cx)
						var c comment
						for _, x := range comments.GetUntracked() {
							if x.ID == a.ID {
								c = x
							}
						}
						text := tr.DiffCommentOnLine(c.lines.Start + 1)
						if c.lines.End > c.lines.Start {
							text = tr.DiffCommentOnLines(c.lines.Start+1, c.lines.End+1)
						}
						return core.View{StyleFunc: func() core.Style {
							return fin.Raised().Over(core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, Padding: core.All(8),
								CornerRadius: 6, BorderWidth: 1, BorderColor: colors.Border(), Background: colors.Raised()})
						}, Children: []core.Node{
							core.Text{Content: text, Style: core.Style{FlexGrow: 1, FontSize: 13}, StyleFunc: ui.Colored(colors.Text)},
							small(tr.DiffResolve(), func() {
								comments.Set(slices.DeleteFunc(slices.Clone(comments.GetUntracked()), func(x comment) bool { return x.ID == a.ID }))
							}),
						}}
					},
					RenderAddAnnotation: func(cx *core.Scope, lines diff.LineRange) core.Node {
						tr := locales.For(cx)
						colors := ui.ColorsOf(cx)
						return core.View{Role: core.RoleButton, Label: tr.DiffAddAComment(), Style: core.Style{Width: core.Pt(16), Height: core.Pt(16),
							CornerRadius: 4, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Cursor: core.CursorPointer}, StyleFunc: ui.Filled(colors.Accent),
							OnPress: func(core.PressEvent) {
								list := comments.GetUntracked()
								id := fmt.Sprint("c", len(list)+1)
								comments.Set(append(slices.Clone(list), comment{diff.Annotation{ID: id, Side: lines.Side, Line: lines.End}, lines}))
								last.Set(tr.DiffCommented())
							},
							Children: []core.Node{core.Text{Content: "+", Style: core.Style{FontSize: 12, FontWeight: ui.Semibold}, StyleFunc: ui.Colored(colors.OnAccent)}}}
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
					RenderHunkActions: func(cx *core.Scope, h diff.Hunk) core.Node {
						tr := locales.For(cx)
						return core.View{Style: core.Style{Direction: core.Row, Gap: 4}, Children: []core.Node{
							small(tr.DiffRevert(), func() { ref.RevertHunk(h.Index); last.Set(tr.DiffReverted(h.Index + 1)) }),
							small(tr.DiffKeep(), func() { ref.AcceptHunk(h.Index); last.Set(tr.DiffKept(h.Index + 1)) }),
						}}
					},
					StylesFunc: diffStyles(cx),
					StyleFunc:  func() core.Style { return core.Style{FlexGrow: 1, Color: colors.Text()} },
				}
				if patch.Get() {
					d.Language = ""
					d.RenderHunkActions = nil
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
		core.View{Style: core.Style{Direction: core.Row, Gap: 16, AlignItems: core.AlignCenter}, Children: []core.Node{
			core.Text{Style: core.Style{FontSize: 12, FontFamily: ui.Mono}, SpansFunc: func() []core.Span {
				s := stats.Get()
				return []core.Span{{Content: fmt.Sprintf("+%d", s.Added), StyleFunc: ui.Colored(colors.Up)},
					{Content: " "}, {Content: fmt.Sprintf("−%d", s.Deleted), StyleFunc: ui.Colored(colors.Down)},
					{Content: " · " + tr.DiffChanges(s.Hunks), StyleFunc: ui.Colored(colors.Muted)}}
			}},
			core.Text{ContentFunc: last.Get, MaxLines: 1, Style: core.Style{FlexShrink: 1, FontSize: 12, FontFamily: ui.Mono}, StyleFunc: ui.Colored(colors.Muted)},
		}},
	}}
}
