// Package diff is base's DiffView, a port of NSH's: the changes between
// two texts, or the files of a patch, side by side or one over the other,
// the words changed within lines marked, the unchanged lines between
// changes collapsed into separators that expand, and the text colored by
// the Editor's highlighting. It has no look of its own: Styles gives every
// part, the added and deleted tints among them, its look.
//
// It is a package of its own because its highlighting links chroma's
// lexers.
package diff

import (
	"fmt"
	"math"
	"path"
	"slices"
	"strings"

	"github.com/alecthomas/chroma/v2"

	"github.com/oneo-me/nsgo/base/internal/syntax"
	"github.com/oneo-me/nsgo/core"
)

// Indicators is how changed lines are marked beside their numbers.
type Indicators uint8

const (
	// IndicatorBars is a bar at the gutter's edge.
	IndicatorBars Indicators = iota
	// IndicatorClassic is + and −.
	IndicatorClassic
	IndicatorNone
)

// Highlighter is the app's highlighting of a language, in place of the
// built-in one, as the Editor takes it.
type Highlighter = syntax.Highlighter

// Token is a run of a line a Highlighter colors.
type Token = syntax.Token

// Side is a side of a diff.
type Side uint8

const (
	Old Side = iota
	New
)

// Hunk is a change: its index, its file's index in a patch (0 for two
// texts), and its lines as 1-based numbers and counts on each side.
type Hunk struct {
	Index, File                            int
	OldStart, OldLines, NewStart, NewLines int
}

// Stats is the lines added and deleted and the number of changes.
type Stats struct{ Added, Deleted, Hunks int }

// File is a file of a patch: its index among the patch's files, its
// paths (OldPath for renames and copies), status, counts, its changes from
// FirstHunk, and whether it is collapsed.
type File struct {
	Index            int
	Path, OldPath    string
	Status           FileStatus
	Binary           bool
	Added, Deleted   int
	Hunks, FirstHunk int
	Collapsed        bool
}

// Separator is what RenderSeparator draws: the gap of unchanged lines it
// collapses (0 before the first change, i between changes i-1 and i), the
// lines it hides (0 for a patch's hunk header), whether it is before the
// first change or after the last, and a patch's hunk header.
type Separator struct {
	Gap, Lines  int
	First, Last bool
	Header      string
}

// Expansion is which of a separator's lines Ref.Expand shows.
type Expansion uint8

const (
	// ExpandEvery shows every line it hides.
	ExpandEvery Expansion = iota
	// ExpandUp shows the lines just above the change below it.
	ExpandUp
	// ExpandDown shows the lines just below the change above it.
	ExpandDown
)

// Styles are the styles of a DiffView's parts, its whole look.
type Styles struct {
	// Side is each side (split) or the one column (unified); Gutter a
	// side's numbers and indicators; LineNumber every number, with
	// LineNumberAdded and LineNumberDeleted after it on changed lines.
	Side, Gutter, LineNumber, LineNumberAdded, LineNumberDeleted core.Style
	// Content is a side's text beside its gutter; Line every row, with
	// LineAdded and LineDeleted after it; Filler the empty half of a split
	// row beside a line the other side lacks.
	Content, Line, LineAdded, LineDeleted, Filler core.Style
	// WordAdded and WordDeleted mark the words changed within lines.
	WordAdded, WordDeleted core.Style
	// IndicatorAdded and IndicatorDeleted are the bars, or the + and −.
	IndicatorAdded, IndicatorDeleted core.Style
	// Separator is a separator of collapsed lines or a patch's hunk
	// header; HunkActions the box of the app's actions on a change;
	// FileHeader a patch's file's header.
	Separator, HunkActions, FileHeader core.Style
	// LineSelected is over the selected lines' rows of their side, gutter
	// and text; Annotation the box of an annotation; AddAnnotation the box
	// of the add control, the gutter's width and a line tall.
	LineSelected, Annotation, AddAnnotation core.Style
}

// Classes are Tailwind classes of a DiffView's parts, by Styles' names,
// each under its part's style.
type Classes struct {
	Side, Gutter, LineNumber, LineNumberAdded, LineNumberDeleted string
	Content, Line, LineAdded, LineDeleted, Filler                string
	WordAdded, WordDeleted                                       string
	IndicatorAdded, IndicatorDeleted                             string
	Separator, HunkActions, FileHeader                           string
	LineSelected, Annotation, AddAnnotation                      string
}

// DiffView shows the changes between OldValue and NewValue, or the files
// of Patch when they are nil.
type DiffView struct {
	OldValue, NewValue func() string
	// Patch is a unified diff of one file or several: git diff's, git
	// format-patch's, diff -u's. Each hunk shows under a separator of its
	// header, its lines numbered as in the file.
	Patch func() string
	// Language colors the text; a patch's files by their paths without
	// it.
	Language string
	// Highlighter gives the app's highlighter for a language's name or a
	// file's path, nil for the built-in one.
	Highlighter func(language string) Highlighter
	// Unified shows one column, the deleted lines over the added ones,
	// rather than two sides.
	Unified    bool
	Inline     Inline
	Indicators Indicators
	// ContextLines is the unchanged lines shown around each change: 3 when
	// 0, none when negative. ExpandAll shows every one.
	ContextLines int
	ExpandAll    bool
	// NoLineNumbers hides the numbers; SoftWrap wraps long lines at the
	// side's width.
	NoLineNumbers, SoftWrap bool
	// FontFamily (the monospaced font when empty), FontSize (13) and
	// LineHeight (20).
	FontFamily           string
	FontSize, LineHeight core.Points
	// RenderFileHeader draws a patch's file headers, a header above a
	// patch of one file too; its controls call Ref.ToggleFile. Without it
	// a header shows the file's path and counts, and a press toggles it.
	RenderFileHeader func(cx *core.Scope, f File) core.Node
	// RenderSeparator draws the separators of collapsed lines and a
	// patch's hunk headers; its controls call Ref.Expand. Without it a
	// separator says what it hides, and a press shows every line.
	RenderSeparator func(cx *core.Scope, s Separator) core.Node
	// RenderHunkActions draws the app's actions on a change (revert,
	// keep), at its first row's right.
	RenderHunkActions func(cx *core.Scope, h Hunk) core.Node
	// Annotations are the app's content under lines, each drawn by
	// RenderAnnotation across its line's side (the view unified).
	Annotations      func() []Annotation
	RenderAnnotation func(cx *core.Scope, a Annotation) core.Node
	// RenderAddAnnotation draws the app's control to add an annotation,
	// beside the line the pointer is over and the last selected one: lines
	// are the selected ones when its line is one of them, else its line.
	RenderAddAnnotation func(cx *core.Scope, lines LineRange) core.Node
	// StylesFunc replaces Styles, again when what it reads changes;
	// Classes go under both, and Class under Style.
	Classes    Classes
	Styles     Styles
	StylesFunc func() Styles
	Class      string
	Style      core.Style
	StyleFunc  func() core.Style
	Ref        *Ref
	TestID     string

	// OnChange receives the texts after a change was reverted or kept.
	OnChange          func(old, new string)
	OnLineNumberPress func(side Side, line int)
	// OnLineSelectionChange receives the lines selected on the numbers, nil
	// for none: a press selects a line, a Shift-press extends to it, and a
	// mouse drag over the numbers selects those it goes over.
	OnLineSelectionChange func(lines *LineRange)
	// OnStatsChange receives the counts once and whenever a diff counts
	// otherwise.
	OnStatsChange func(Stats)
	OnFileToggle  func(File)
	// OnError receives a patch that cannot be read.
	OnError func(error)
}

// Ref is a DiffView's methods, bound once it mounts.
type Ref struct{ v *dv }

// rowKind is what a row shows.
type rowKind uint8

const (
	rowEqual rowKind = iota
	rowModified
	rowDeleted
	rowAdded
	rowSeparator
	rowFile
)

// row is a row of the view: lines of each side (-1 for none), or a
// separator of a gap or a hunk's header, or a file's header.
type row struct {
	kind     rowKind
	old, new int
	hunk     int // the change a line is in, -1 for none
	gap      int // a separator's gap
	hidden   int
	header   string
	file     int
	// first and last mark a separator before the first change or after
	// the last.
	first, last bool
}

// segment is a file's part of the joined texts.
type segment struct {
	file             File
	oldAt, newAt     int
	oldLen, newLen   int
	hunk0, hunk1     int
	oldNums, newNums []int
	starts           [][2]int // a patch's hunks' first lines, file-local
	headers          []string
	lexer            chroma.Lexer
	app              Highlighter
	patch            bool
}

type dv struct {
	// looks are the parts' styles, Styles or what StylesFunc returns.
	looks        *core.Memo[*Styles]
	p            DiffView
	cx           *core.Scope
	m            *model
	segs         []segment
	rows         []row
	inl          map[[2]int][2][][2]int
	hlOld, hlNew [][]syntax.Tok
	expand       map[int][2]int
	collapsed    map[string]bool
	rev, hlRev   *core.Signal[int]
	scrollY      *core.Signal[core.Points]
	viewport     *core.Signal[core.Size]
	vs           *core.Ref
	window       *core.Memo[[2]int]
	font         core.Font
	charW, lh    float32
	widest       int
	stats        Stats
	reported     bool
	nav          *nav
	job          int
	// tops is every row's top, and the rows' height last; geo counts
	// moves of the rows; wrapCols is the columns lines wrap at.
	tops     []float32
	geo      *core.Signal[int]
	wrapCols int
	// oldRow and newRow are the row showing each line, -1 for none.
	oldRow, newRow []int
	// The split sides' horizontal scroll views and where they are.
	hs      []*core.Ref
	scrollX float32
	// The lines selected on the numbers and the press they extend from.
	sel     *LineRange
	anchor  int
	selRev  *core.Signal[int]
	hovered *core.Signal[LineRange]
	// hoverStop ends the hover a moment after the pointer left.
	hoverStop   func() bool
	annotations []Annotation
	annH        map[string]float32
}

// Render implements core.Node.
func (p DiffView) Render(cx *core.Scope) core.Node {
	v := &dv{p: p, cx: cx, looks: stylesMemo(cx, p.Styles, p.StylesFunc), rev: core.NewSignal(cx, 0), hlRev: core.NewSignal(cx, 0),
		scrollY: core.NewSignal(cx, core.Points(0)), viewport: core.NewSignal(cx, core.Size{}), vs: &core.Ref{},
		expand: map[int][2]int{}, collapsed: map[string]bool{}, m: &model{},
		geo: core.NewSignal(cx, 0), selRev: core.NewSignal(cx, 0), hovered: core.NewSignal(cx, LineRange{Start: -1, End: -1}),
		annH: map[string]float32{}}
	v.font = core.Font{Family: p.FontFamily, Size: p.FontSize}
	if v.font.Family == "" {
		v.font.Family = core.FontMonospace
	}
	if v.font.Size == 0 {
		v.font.Size = 13
	}
	v.lh = float32(p.LineHeight)
	if v.lh == 0 {
		v.lh = 20
	}
	v.charW = max(cx.MeasureText("0000000000", v.font).Width/10, 1)
	if p.Ref != nil {
		p.Ref.v = v
	}
	v.window = core.NewMemo(cx, v.measureWindow)
	v.followWidth()
	if p.Annotations != nil {
		cx.Effect(func() {
			list := p.Annotations()
			cx.Untrack(func() {
				v.annotations = list
				if v.rows != nil {
					v.relayout()
				}
			})
		})
	}
	cx.Effect(func() {
		var oldText, newText, patch string
		if p.OldValue != nil || p.NewValue != nil {
			if p.OldValue != nil {
				oldText = p.OldValue()
			}
			if p.NewValue != nil {
				newText = p.NewValue()
			}
		} else if p.Patch != nil {
			patch = p.Patch()
		}
		cx.Untrack(func() { v.load(oldText, newText, patch) })
	})
	return core.View{
		Class:     p.Class,
		Style:     merge(core.Style{Overflow: core.Hidden, FontFamily: v.font.Family, FontSize: v.font.Size}, p.Style),
		StyleFunc: p.StyleFunc, TestID: p.TestID,
		Children: []core.Node{core.Scroll{
			Ref: v.vs, TestID: "diff-scroll",
			Style: core.Style{Position: core.Absolute, Left: core.Pt(0), Top: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0)},
			OnScroll: func(e core.ScrollEvent) {
				v.scrollY.Set(e.Offset.Y)
				v.scrolledTo(float32(e.Offset.Y))
			},
			OnLayout: func(b core.Bounds) { v.viewport.Set(b.Size) },
			Children: []core.Node{v.box()},
		}},
	}
}

func merge(styles ...core.Style) core.Style {
	var out core.Style
	for _, s := range styles {
		out = out.Merge(s)
	}
	return out
}

// load diffs the texts, or the patch's files, joined into two texts.
func (v *dv) load(oldText, newText, patch string) {
	// Other texts clear the selected lines.
	v.setSelection(nil, true)
	v.segs = nil
	v.expand = map[int][2]int{}
	v.nav = nil
	var oldLines, newLines []string
	var hunks []hunk
	add := func(s segment, old, new []string, ops []op) {
		s.oldAt, s.newAt, s.oldLen, s.newLen = len(oldLines), len(newLines), len(old), len(new)
		oldLines, newLines = append(oldLines, old...), append(newLines, new...)
		s.hunk0 = len(hunks)
		i, j := 0, 0
		for k := 0; k < len(ops); {
			if ops[k] == opEqual {
				i, j, k = i+1, j+1, k+1
				continue
			}
			h := hunk{oldStart: i, newStart: j}
			for k < len(ops) && ops[k] != opEqual {
				if ops[k] == opDelete {
					i++
				} else {
					j++
				}
				k++
			}
			h.oldEnd, h.newEnd = i, j
			h.pairs = pair(old[h.oldStart:h.oldEnd], new[h.newStart:h.newEnd], s.oldAt+h.oldStart, s.newAt+h.newStart)
			h.oldStart, h.oldEnd, h.newStart, h.newEnd = h.oldStart+s.oldAt, h.oldEnd+s.oldAt, h.newStart+s.newAt, h.newEnd+s.newAt
			hunks = append(hunks, h)
			s.file.Added += h.newEnd - h.newStart
			s.file.Deleted += h.oldEnd - h.oldStart
		}
		s.hunk1 = len(hunks)
		s.file.Hunks, s.file.FirstHunk = s.hunk1-s.hunk0, s.hunk0
		v.segs = append(v.segs, s)
	}
	if patch == "" {
		old, new := splitLines(oldText), splitLines(newText)
		add(segment{lexer: syntax.Lexer(v.p.Language), app: v.appHighlighter(v.p.Language)}, old, new, myers(old, new, maxSteps))
	} else {
		files, err := parsePatch(patch)
		if err != nil && v.p.OnError != nil {
			v.p.OnError(err)
		}
		for i, f := range files {
			oldT, newT, on, nn := f.texts()
			var ops []op
			var starts [][2]int
			var headers []string
			oi, ni := 0, 0
			for _, h := range f.hunks {
				starts = append(starts, [2]int{oi, ni})
				headers = append(headers, h.header)
				for _, l := range h.lines {
					switch l[0] {
					case '-':
						ops = append(ops, opDelete)
						oi++
					case '+':
						ops = append(ops, opInsert)
						ni++
					default:
						ops = append(ops, opEqual)
						oi, ni = oi+1, ni+1
					}
				}
			}
			lang := v.p.Language
			if lang == "" {
				lang = path.Base(f.path)
			}
			file := File{Index: i, Path: f.path, OldPath: f.oldPath, Status: f.status, Binary: f.binary, Collapsed: v.collapsed[f.path]}
			if file.OldPath == file.Path {
				file.OldPath = ""
			}
			add(segment{file: file, oldNums: on, newNums: nn, starts: starts, headers: headers, patch: true,
				lexer: syntax.Lexer(lang), app: v.appHighlighter(lang)}, splitLines(oldT), splitLines(newT), ops)
		}
		for path := range v.collapsed {
			if !slices.ContainsFunc(v.segs, func(s segment) bool { return s.file.Path == path }) {
				delete(v.collapsed, path)
			}
		}
	}
	v.m = &model{old: oldLines, new: newLines, hunks: hunks}
	v.inl = map[[2]int][2][][2]int{}
	v.widest = 0
	for _, l := range append(slices.Clone(oldLines), newLines...) {
		v.widest = max(v.widest, columns(l))
	}
	v.build()
	v.highlight()
	st := Stats{Hunks: len(hunks)}
	for _, s := range v.segs {
		st.Added += s.file.Added
		st.Deleted += s.file.Deleted
	}
	if (!v.reported || st != v.stats) && v.p.OnStatsChange != nil {
		v.p.OnStatsChange(st)
	}
	v.stats, v.reported = st, true
}

// appHighlighter is the app's highlighter for lang, nil for none.
func (v *dv) appHighlighter(lang string) Highlighter {
	if v.p.Highlighter == nil {
		return nil
	}
	return v.p.Highlighter(lang)
}

// columns is a line's width in columns, tabs as 4.
func columns(l string) int {
	n := 0
	for _, r := range strings.TrimSuffix(l, "\n") {
		if r == '\t' {
			n += 4
		} else {
			n++
		}
	}
	return n
}

// context is the unchanged lines shown around a change.
func (v *dv) context() int {
	switch {
	case v.p.ContextLines < 0:
		return 0
	case v.p.ContextLines == 0:
		return 3
	}
	return v.p.ContextLines
}

// build makes the rows: file headers, separators and lines.
func (v *dv) build() {
	var rows []row
	many := len(v.segs) > 1
	gap := 0
	for fi, s := range v.segs {
		if many || s.patch && v.p.RenderFileHeader != nil {
			rows = append(rows, row{kind: rowFile, file: fi, old: -1, new: -1, hunk: -1})
		}
		if s.file.Collapsed {
			continue
		}
		if s.patch {
			rows = v.patchRows(rows, s, &gap)
			continue
		}
		rows = v.textRows(rows, s, &gap)
	}
	v.rows = rows
	v.oldRow, v.newRow = lineRows(v.oldRow, len(v.m.old)), lineRows(v.newRow, len(v.m.new))
	for i, r := range rows {
		if r.kind == rowSeparator || r.kind == rowFile {
			continue
		}
		if r.old >= 0 {
			v.oldRow[r.old] = i
		}
		if r.new >= 0 {
			v.newRow[r.new] = i
		}
	}
	v.layoutRows()
	v.cx.Batch(func() {
		v.rev.Update(func(n int) int { return n + 1 })
		v.geo.Update(func(n int) int { return n + 1 })
	})
}

// hunkRows are a change's rows: side by side in pairs, or the deleted
// lines over the added ones.
func (v *dv) hunkRows(rows []row, h hunk, index int) []row {
	if v.p.Unified {
		for i := h.oldStart; i < h.oldEnd; i++ {
			rows = append(rows, row{kind: rowDeleted, old: i, new: -1, hunk: index})
		}
		for j := h.newStart; j < h.newEnd; j++ {
			rows = append(rows, row{kind: rowAdded, old: -1, new: j, hunk: index})
		}
		return rows
	}
	oi, ni := h.oldStart, h.newStart
	for _, pr := range append(h.pairs, [2]int{h.oldEnd, h.newEnd}) {
		for ; oi < pr[0]; oi++ {
			rows = append(rows, row{kind: rowDeleted, old: oi, new: -1, hunk: index})
		}
		for ; ni < pr[1]; ni++ {
			rows = append(rows, row{kind: rowAdded, old: -1, new: ni, hunk: index})
		}
		if pr[0] < h.oldEnd && pr[1] < h.newEnd {
			rows = append(rows, row{kind: rowModified, old: pr[0], new: pr[1], hunk: index})
			oi, ni = pr[0]+1, pr[1]+1
		}
	}
	return rows
}

// textRows are two texts' rows: the changes with their context, and
// separators for the unchanged lines between them, as the user expanded
// them.
func (v *dv) textRows(rows []row, s segment, gap *int) []row {
	ctx := v.context()
	equal := func(o, n int) row { return row{kind: rowEqual, old: o, new: n, hunk: -1} }
	oi, ni := s.oldAt, s.newAt
	for hi := s.hunk0; hi <= s.hunk1; hi++ {
		// The unchanged lines before change hi (or after the last).
		endO := s.oldAt + s.oldLen
		if hi < s.hunk1 {
			endO = v.m.hunks[hi].oldStart
		}
		count := endO - oi
		top, bottom := ctx, ctx
		if hi == s.hunk0 {
			top = 0
		}
		if hi == s.hunk1 {
			bottom = 0
		}
		if e, ok := v.expand[*gap]; ok {
			top, bottom = top+e[0], bottom+e[1]
		}
		if v.p.ExpandAll || top+bottom >= count-1 {
			top, bottom = count, 0
		}
		for k := 0; k < min(top, count); k++ {
			rows = append(rows, equal(oi+k, ni+k))
		}
		if hidden := count - top - bottom; hidden > 0 {
			rows = append(rows, row{kind: rowSeparator, gap: *gap, hidden: hidden, old: oi + top, new: ni + top, hunk: -1,
				first: hi == s.hunk0, last: hi == s.hunk1})
		}
		for k := max(count-bottom, top); k < count; k++ {
			rows = append(rows, equal(oi+k, ni+k))
		}
		*gap++
		oi, ni = oi+count, ni+count
		if hi < s.hunk1 {
			h := v.m.hunks[hi]
			rows = v.hunkRows(rows, h, hi)
			oi, ni = h.oldEnd, h.newEnd
		}
	}
	return rows
}

// patchRows are a patched file's rows: each hunk under a separator of its
// header, every line shown.
func (v *dv) patchRows(rows []row, s segment, gap *int) []row {
	hi := s.hunk0
	oi, ni := s.oldAt, s.newAt
	next := 0
	for oi < s.oldAt+s.oldLen || ni < s.newAt+s.newLen {
		if next < len(s.starts) && oi-s.oldAt == s.starts[next][0] && ni-s.newAt == s.starts[next][1] {
			rows = append(rows, row{kind: rowSeparator, gap: *gap, header: s.headers[next], old: oi, new: ni, hunk: -1, first: next == 0})
			*gap++
			next++
		}
		if hi < s.hunk1 && v.m.hunks[hi].oldStart == oi && v.m.hunks[hi].newStart == ni {
			h := v.m.hunks[hi]
			rows = v.hunkRows(rows, h, hi)
			oi, ni, hi = h.oldEnd, h.newEnd, hi+1
			continue
		}
		rows = append(rows, row{kind: rowEqual, old: oi, new: ni, hunk: -1})
		oi, ni = oi+1, ni+1
	}
	return rows
}

// highlight lexes each file's sides on a goroutine of its own.
func (v *dv) highlight() {
	v.hlOld, v.hlNew = make([][]syntax.Tok, len(v.m.old)), make([][]syntax.Tok, len(v.m.new))
	v.job++
	job := v.job
	for _, s := range v.segs {
		if s.lexer == nil && s.app == nil {
			continue
		}
		strip := func(lines []string) []string {
			out := make([]string, len(lines))
			for i, l := range lines {
				out[i] = strings.TrimSuffix(l, "\n")
			}
			return out
		}
		old := strip(v.m.old[s.oldAt : s.oldAt+s.oldLen])
		new := strip(v.m.new[s.newAt : s.newAt+s.newLen])
		lexer, app, at := s.lexer, s.app, [2]int{s.oldAt, s.newAt}
		go func() {
			var o, n [][]syntax.Tok
			if app != nil {
				o, n = syntax.HighlightLines(app, old, 0, len(old)), syntax.HighlightLines(app, new, 0, len(new))
			} else {
				o, n = syntax.LexLines(lexer, old), syntax.LexLines(lexer, new)
			}
			v.cx.Post(func() {
				if v.job != job {
					return
				}
				copy(v.hlOld[at[0]:], o)
				copy(v.hlNew[at[1]:], n)
				v.hlRev.Update(func(n int) int { return n + 1 })
			})
		}()
	}
}

// inlineOf is a modified row's changed ranges, computed once.
func (v *dv) inlineOf(o, n int) [2][][2]int {
	k := [2]int{o, n}
	if r, ok := v.inl[k]; ok {
		return r
	}
	a, b := inline(v.m.old[o], v.m.new[n], v.p.Inline)
	v.inl[k] = [2][][2]int{a, b}
	return v.inl[k]
}

// windowRows is the rows made (tracked).
func (v *dv) windowRows() (int, int) {
	w := v.window.Get()
	return w[0], w[1]
}

// measureWindow is the rows in view and half a viewport each way, at
// least 8: the window slides with the view, so a scroll makes only the
// rows coming into it, a few a frame, where a window moved a page ahead
// at a time would make them all in one. Rows are kept by index, not
// reused as a table's slots are, since each kind of row has parts of its
// own. None until the view has its size, so the first rows are made once,
// wrapped at its width.
func (v *dv) measureWindow() [2]int {
	v.rev.Get()
	v.geo.Get()
	vp := v.viewport.Get()
	n := len(v.rows)
	if n == 0 || vp.Width <= 0 || vp.Height <= 0 {
		return [2]int{}
	}
	y := float32(v.scrollY.Get())
	first := v.rowAt(y)
	last := min(v.rowAt(y+float32(vp.Height))+1, n)
	reach := max((last-first)/2, 8)
	return [2]int{max(first-reach, 0), min(last+reach, n)}
}

// numberWidth is a number column's width.
func (v *dv) numberWidth() float32 {
	if v.p.NoLineNumbers {
		return 0
	}
	most := max(len(v.m.old), len(v.m.new))
	for _, s := range v.segs {
		if len(s.oldNums) > 0 {
			most = max(most, slices.Max(append(s.oldNums, 0)))
		}
		if len(s.newNums) > 0 {
			most = max(most, slices.Max(append(s.newNums, 0)))
		}
	}
	return float32(max(len(fmt.Sprint(most)), 2))*v.charW + 8
}

// gutterWidth is a side's gutter: its numbers (two unified) and the
// indicator.
func (v *dv) gutterWidth() float32 {
	w := v.numberWidth()
	if v.p.Unified {
		w *= 2
	}
	if v.p.Indicators != IndicatorNone {
		w += v.charW + 4
	}
	return w
}

// viewWidth is the view's width; halfWidth a split side's.
func (v *dv) viewWidth() float32 { return float32(v.viewport.Get().Width) }

func (v *dv) halfWidth() float32 { return float32(math.Floor(float64(v.viewWidth() / 2))) }

// sideWidth is a side's width: half the view split, all of it unified.
func (v *dv) sideWidth() float32 {
	if v.p.Unified {
		return v.viewWidth()
	}
	return v.halfWidth()
}

// textWidth is a side's text's width, past the gutter: the widest line's
// without soft wrap, at least the room the side leaves.
func (v *dv) textWidth() float32 {
	v.rev.Get()
	room := v.sideWidth() - v.gutterWidth()
	if v.p.SoftWrap {
		return room
	}
	return max(float32(v.widest)*v.charW+16, room)
}

// followWidth wraps the lines at the text's width with SoftWrap.
func (v *dv) followWidth() {
	if !v.p.SoftWrap {
		return
	}
	v.cx.Effect(func() {
		if v.viewWidth() <= 0 {
			return
		}
		w := v.sideWidth() - v.gutterWidth() - 16
		v.cx.Untrack(func() {
			cols := max(int(w/v.charW), 8)
			if cols != v.wrapCols {
				v.wrapCols = cols
				v.relayout()
			}
		})
	})
}

// relayout places the rows again: wrapping or an annotation's height
// moved them.
func (v *dv) relayout() {
	v.layoutRows()
	v.geo.Update(func(n int) int { return n + 1 })
}

func (v *dv) box() core.Node {
	var sides []core.Node
	if v.p.Unified {
		sides = []core.Node{v.side(-1)}
	} else {
		sides = []core.Node{v.side(int(Old)), v.side(int(New))}
	}
	return core.View{
		StyleFunc: func() core.Style {
			v.rev.Get()
			v.geo.Get()
			return core.Style{Width: core.Pt(core.Points(v.viewWidth())), Height: core.Pt(core.Points(v.height())), Direction: core.Row}
		},
		Children: append(sides, v.bands(), v.actions(), v.annotationsView(), v.addControls()),
	}
}

// rowsOf is the rows in the window, made by piece in a column of a side.
func (v *dv) rowsOf(piece func(i int, r row) core.Node) core.Node {
	return core.Each(func() []int {
		s, e := v.windowRows()
		out := make([]int, e-s)
		for i := range out {
			out[i] = s + i
		}
		return out
	}, func(i int) int { return i }, func(cx *core.Scope, i func() int, _ func() int) core.Node {
		return core.Dynamic(func(cx *core.Scope) core.Node {
			v.rev.Get()
			v.geo.Get()
			if i() >= len(v.rows) || v.rows[i()].kind == rowSeparator || v.rows[i()].kind == rowFile {
				return core.Group{}
			}
			return piece(i(), v.rows[i()])
		})
	})
}

// place is row i's box in a column: its top and its lines' height.
func (v *dv) place(i int, r row) core.Style {
	return core.Style{Position: core.Absolute, Left: core.Pt(0), Right: core.Pt(0), Top: core.Pt(core.Points(v.tops[i])),
		Height: core.Pt(core.Points(float32(v.lineCount(r)) * v.lh))}
}

// side is a side's column: its gutter beside its text, the text scrolling
// sideways on its own (side -1: unified's one column).
func (v *dv) side(side int) core.Node {
	desktop := core.Untracked(v.cx, v.cx.Env().Desktop)
	gutter := core.View{
		Class: v.p.Classes.Gutter,
		StyleFunc: func() core.Style {
			v.rev.Get()
			return merge(core.Style{Width: core.Pt(core.Points(v.gutterWidth())), FlexShrink: 0}, v.styles().Gutter)
		},
		Children: []core.Node{v.rowsOf(func(i int, r row) core.Node { return v.gutterPiece(i, r, side) })},
	}
	if desktop {
		// A drag over the numbers selects the lines it goes over.
		gutter.Responder = &core.Responder{Claims: []core.TouchClaim{{OnMove: true}},
			OnMove: func(ev core.ResponderEvent) { v.dragLines(float32(ev.Position.Y)) }}
	}
	rows := core.View{
		StyleFunc: func() core.Style {
			v.geo.Get()
			return core.Style{Width: core.Pt(core.Points(v.textWidth())), Height: core.Pt(core.Points(v.height()))}
		},
		Children: []core.Node{v.rowsOf(func(i int, r row) core.Node { return v.textPiece(i, r, side) })},
	}
	var text core.Node
	if v.p.SoftWrap {
		text = core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1}, Children: []core.Node{rows}}
	} else {
		ref := &core.Ref{}
		v.hs = append(v.hs, ref)
		text = core.Scroll{Horizontal: true, Ref: ref, Style: core.Style{FlexGrow: 1, FlexShrink: 1},
			OnScroll: func(e core.ScrollEvent) { v.scrolledX(ref, float32(e.Offset.X)) }, Children: []core.Node{rows}}
	}
	return core.View{
		Class: v.p.Classes.Side,
		StyleFunc: func() core.Style {
			v.geo.Get()
			return merge(core.Style{Width: core.Pt(core.Points(v.sideWidth())), Height: core.Pt(core.Points(v.height())),
				Direction: core.Row}, v.styles().Side)
		},
		Children: []core.Node{gutter, text},
	}
}

// scrolledX keeps the split sides scrolled sideways together: a side's
// scroll scrolls the other, a move under half a point ending the echo.
func (v *dv) scrolledX(from *core.Ref, x float32) {
	if math.Abs(float64(x-v.scrollX)) < 0.5 {
		return
	}
	v.scrollX = x
	for _, r := range v.hs {
		if r != from {
			r.ScrollTo(core.Point{X: core.Points(x)}, false)
		}
	}
}

// pieceSide is the side row r's piece in column side shows: the column's,
// or unified the row's.
func (r row) pieceSide(side int) (Side, int) {
	switch {
	case side == int(Old), side < 0 && r.kind == rowDeleted:
		return Old, r.old
	}
	return New, r.new
}

// rowSelected reports whether row r is selected in column side (tracked).
func (v *dv) rowSelected(r row, side int) bool {
	if side >= 0 {
		return v.selected(Side(side), r.lineOf(Side(side)))
	}
	return v.selected(Old, r.old) || v.selected(New, r.new)
}

// tints are the row's changed state in column side.
func (v *dv) tints(r row, side int) (added, deleted bool) {
	isOld := side == int(Old) || side < 0 && r.kind == rowDeleted
	added = r.kind == rowAdded || r.kind == rowModified && !isOld
	deleted = r.kind == rowDeleted || r.kind == rowModified && isOld
	if side < 0 && r.kind == rowModified {
		added, deleted = false, false
	}
	return added, deleted
}

// hoverable sets the handlers that show the add control over row r.
func (v *dv) hoverable(view *core.View, r row, side int) {
	if v.p.RenderAddAnnotation == nil {
		return
	}
	sd, line := r.pieceSide(side)
	view.OnHoverIn = func() { v.hover(sd, line) }
	view.OnHoverOut = func() { v.hover(sd, -1) }
}

// gutterPiece is row r's numbers and indicator in column side.
func (v *dv) gutterPiece(i int, r row, side int) core.Node {
	s, c := v.styles, &v.p.Classes
	added, deleted := v.tints(r, side)
	numClass, tint := c.LineNumber, ""
	switch {
	case added:
		numClass, tint = joinClasses(numClass, c.LineNumberAdded), c.IndicatorAdded
	case deleted:
		numClass, tint = joinClasses(numClass, c.LineNumberDeleted), c.IndicatorDeleted
	}
	numPlace := core.Style{Width: core.Pt(core.Points(v.numberWidth())), TextAlign: core.TextAlignEnd,
		Padding: core.Edges{Right: 8}, LineHeight: core.Points(v.lh)}
	numStyle := func() core.Style {
		st := s().LineNumber
		if added {
			st = merge(st, s().LineNumberAdded)
		} else if deleted {
			st = merge(st, s().LineNumberDeleted)
		}
		return st
	}
	number := func(sd Side, at int) core.Node {
		if v.p.NoLineNumbers {
			return core.Group{}
		}
		text := ""
		if at >= 0 {
			text = fmt.Sprint(v.number(sd, at))
		}
		return core.Text{Content: text, MaxLines: 1, Class: numClass, Style: numPlace, StyleFunc: numStyle}
	}
	var children []core.Node
	switch {
	case side < 0:
		children = append(children, number(Old, r.old), number(New, r.new))
	case side == int(Old):
		children = append(children, number(Old, r.old))
	default:
		children = append(children, number(New, r.new))
	}
	switch v.p.Indicators {
	case IndicatorBars:
		st := func() core.Style {
			switch {
			case added:
				return s().IndicatorAdded
			case deleted:
				return s().IndicatorDeleted
			}
			return core.Style{}
		}
		children = append(children, core.View{Style: core.Style{Width: core.Pt(core.Points(v.charW + 4))},
			Children: []core.Node{core.View{Class: tint, Style: core.Style{Width: core.Pt(3), FlexGrow: 1}, StyleFunc: st}}})
	case IndicatorClassic:
		mark := " "
		if added {
			mark = "+"
		} else if deleted {
			mark = "−"
		}
		st := func() core.Style {
			switch {
			case added:
				return s().IndicatorAdded
			case deleted:
				return s().IndicatorDeleted
			}
			return core.Style{}
		}
		children = append(children, core.Text{Content: mark, Class: tint, Style: core.Style{Width: core.Pt(core.Points(v.charW + 4)),
			LineHeight: core.Points(v.lh)}, StyleFunc: st})
	}
	desktop := core.Untracked(v.cx, v.cx.Env().Desktop)
	press := func(ev core.PressEvent) {
		sd := v.gutterSide(r, side, float32(ev.Position.X))
		v.selectLine(sd, r.lineOf(sd), ev.Modifiers.Shift)
	}
	view := core.View{
		ClassFunc: v.selectedClass(r, side),
		StyleFunc: func() core.Style {
			st := merge(v.place(i, r), core.Style{Direction: core.Row})
			if v.rowSelected(r, side) {
				st = merge(st, s().LineSelected)
			}
			return st
		},
		NoFocus:  true,
		Children: children,
		OnPress: func(ev core.PressEvent) {
			sd := v.gutterSide(r, side, float32(ev.Position.X))
			if !desktop {
				press(ev)
			}
			if line := r.lineOf(sd); line >= 0 && v.p.OnLineNumberPress != nil {
				v.p.OnLineNumberPress(sd, v.number(sd, line))
			}
		},
	}
	if desktop {
		view.OnPressDown = press
	}
	v.hoverable(&view, r, side)
	return view
}

// textPiece is row r's text in column side: a Text per wrapped line.
func (v *dv) textPiece(i int, r row, side int) core.Node {
	s, c := v.styles, &v.p.Classes
	sd, line := r.pieceSide(side)
	added, deleted := v.tints(r, side)
	class := c.Line
	switch {
	case line < 0:
		class = joinClasses(class, c.Filler)
	case added:
		class = joinClasses(class, c.LineAdded)
	case deleted:
		class = joinClasses(class, c.LineDeleted)
	}
	lineStyle := func() core.Style {
		st := s().Line
		switch {
		case line < 0:
			st = merge(st, s().Filler)
		case added:
			st = merge(st, s().LineAdded)
		case deleted:
			st = merge(st, s().LineDeleted)
		}
		return st
	}
	var texts []core.Node
	if line >= 0 {
		isOld := sd == Old
		starts := []int{0}
		if v.p.SoftWrap {
			starts = v.wrapped(isOld, line)
		}
		full := len(strings.TrimSuffix(v.lineText(isOld, line), "\n"))
		for k, a := range starts {
			b := full
			if k+1 < len(starts) {
				b = starts[k+1]
			}
			texts = append(texts, core.Text{MaxLines: 1, Style: core.Style{WhiteSpace: core.WhiteSpaceNoWrap, LineHeight: core.Points(v.lh),
				Height: core.Pt(core.Points(v.lh))},
				SpansFunc: func() []core.Span { return v.spans(r, isOld, line, a, b) }})
		}
	}
	view := core.View{
		Class:     joinClasses(class, c.Content),
		ClassFunc: v.selectedClass(r, side),
		StyleFunc: func() core.Style {
			st := merge(v.place(i, r), core.Style{Padding: core.Edges{Left: 8, Right: 8}}, lineStyle(), s().Content)
			if v.rowSelected(r, side) {
				st = merge(st, s().LineSelected)
			}
			return st
		},
		Children: texts,
	}
	v.hoverable(&view, r, side)
	return view
}

// selectedClass is LineSelected's class while row r is selected in
// column side.
func (v *dv) selectedClass(r row, side int) func() string {
	if v.p.Classes.LineSelected == "" {
		return nil
	}
	return func() string {
		if v.rowSelected(r, side) {
			return v.p.Classes.LineSelected
		}
		return ""
	}
}

// joinClasses returns a's classes then b's, which win.
func joinClasses(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	return a + " " + b
}

// lineText is a line of a side.
func (v *dv) lineText(isOld bool, line int) string {
	if isOld {
		return v.m.old[line]
	}
	return v.m.new[line]
}

// bands are the rows across the view in the window: separators of
// collapsed lines or a patch's hunk headers, and files' headers.
func (v *dv) bands() core.Node {
	return core.Each(func() []int {
		// The rows change under a window that stays.
		v.rev.Get()
		s, e := v.windowRows()
		var out []int
		for i := s; i < e && i < len(v.rows); i++ {
			if k := v.rows[i].kind; k == rowSeparator || k == rowFile {
				out = append(out, i)
			}
		}
		return out
	}, func(i int) int { return i }, func(cx *core.Scope, i func() int, _ func() int) core.Node {
		return core.Dynamic(func(cx *core.Scope) core.Node {
			v.rev.Get()
			v.geo.Get()
			v.viewport.Get()
			if i() >= len(v.rows) || v.rows[i()].kind != rowSeparator && v.rows[i()].kind != rowFile {
				return core.Group{}
			}
			return v.band(cx, i(), v.rows[i()])
		})
	})
}

// band is separator or file header row r at index i: the app's, or a
// press toggling the file or showing the gap's lines.
func (v *dv) band(cx *core.Scope, i int, r row) core.Node {
	s, c := v.styles, &v.p.Classes
	place := core.Style{Position: core.Absolute, Left: core.Pt(0), Width: core.Pt(core.Points(core.Untracked(v.cx, v.viewWidth))),
		Top: core.Pt(core.Points(v.tops[i])), Height: core.Pt(core.Points(v.lh)), Direction: core.Row, AlignItems: core.AlignCenter}
	if r.kind == rowFile {
		seg := v.segs[r.file]
		f := seg.file
		if v.p.RenderFileHeader != nil {
			return core.View{Class: c.FileHeader, Style: place, StyleFunc: func() core.Style { return s().FileHeader },
				Children: []core.Node{v.p.RenderFileHeader(cx, f)}}
		}
		name := f.Path
		if f.OldPath != "" && f.OldPath != f.Path {
			name = f.OldPath + " → " + f.Path
		}
		mark := "▾ "
		if f.Collapsed {
			mark = "▸ "
		}
		return core.View{Class: c.FileHeader, Style: place, StyleFunc: func() core.Style { return s().FileHeader }, Role: core.RoleButton, Label: name,
			OnPress: func(core.PressEvent) { v.toggleFile(f.Path) },
			Children: []core.Node{core.Text{MaxLines: 1, Spans: []core.Span{{Content: mark + name + "  "},
				{Content: fmt.Sprintf("+%d", f.Added), StyleFunc: func() core.Style { return core.Style{Color: s().IndicatorAdded.Color} }}, {Content: " "},
				{Content: fmt.Sprintf("−%d", f.Deleted), StyleFunc: func() core.Style { return core.Style{Color: s().IndicatorDeleted.Color} }}}}}}
	}
	if v.p.RenderSeparator != nil {
		return core.View{Class: c.Separator, Style: place, StyleFunc: func() core.Style { return s().Separator },
			Children: []core.Node{v.p.RenderSeparator(cx, Separator{Gap: r.gap, Lines: r.hidden, First: r.first, Last: r.last,
				Header: r.header})}}
	}
	label := r.header
	if label == "" {
		label = fmt.Sprintf("⋯ %d unchanged lines", r.hidden)
		if r.hidden == 1 {
			label = "⋯ 1 unchanged line"
		}
	}
	gap := r.gap
	return core.View{Class: c.Separator, Style: place, StyleFunc: func() core.Style { return s().Separator }, Role: core.RoleButton, Label: label,
		OnPress: func(core.PressEvent) {
			if r.header == "" {
				v.expandGap(gap, math.MaxInt32, math.MaxInt32)
			}
		},
		Children: []core.Node{core.Text{Content: label, MaxLines: 1}}}
}

// number is line at's number on side: as in the file for a patch,
// counted in its file otherwise.
func (v *dv) number(side Side, at int) int {
	for _, s := range v.segs {
		if side == Old && at >= s.oldAt && at < s.oldAt+s.oldLen {
			if s.oldNums != nil {
				return s.oldNums[at-s.oldAt]
			}
			return at - s.oldAt + 1
		}
		if side == New && at >= s.newAt && at < s.newAt+s.newLen {
			if s.newNums != nil {
				return s.newNums[at-s.newAt]
			}
			return at - s.newAt + 1
		}
	}
	return at + 1
}

// spans is bytes from to to of a line in runs of one token and one inline
// state, tabs as four spaces.
func (v *dv) spans(r row, isOld bool, line, from, to int) []core.Span {
	v.hlRev.Get()
	dark := v.cx.Dark()
	palette := &syntax.Light
	if dark {
		palette = &syntax.Dark
	}
	text := v.m.new[line]
	toks := v.hlNew[line]
	word, wordClass := v.styles().WordAdded, v.p.Classes.WordAdded
	if isOld {
		text, toks, word, wordClass = v.m.old[line], v.hlOld[line], v.styles().WordDeleted, v.p.Classes.WordDeleted
	}
	text = strings.TrimSuffix(text, "\n")
	to = min(to, len(text))
	var marks [][2]int
	if r.kind == rowModified {
		in := v.inlineOf(r.old, r.new)
		marks = in[1]
		if isOld {
			marks = in[0]
		}
	}
	cuts := []int{from, to}
	for _, t := range toks {
		cuts = append(cuts, t.Start, t.End)
	}
	for _, m := range marks {
		cuts = append(cuts, m[0], m[1])
	}
	slices.Sort(cuts)
	cuts = slices.Compact(cuts)
	var out []core.Span
	for i := 0; i+1 < len(cuts); i++ {
		a, b := cuts[i], cuts[i+1]
		if a < from || b > to || a >= b {
			continue
		}
		var st core.Style
		var class string
		for _, t := range toks {
			if t.Start <= a && b <= t.End {
				st = palette[t.Kind]
			}
		}
		for _, m := range marks {
			if m[0] <= a && b <= m[1] {
				st, class = merge(st, word), wordClass
			}
		}
		out = append(out, core.Span{Content: strings.ReplaceAll(text[a:b], "\t", "    "), Class: class, Style: st})
	}
	if len(out) == 0 {
		out = []core.Span{{Content: ""}}
	}
	return out
}

// actions are the app's actions on each change in view, at its first
// row's right.
func (v *dv) actions() core.Node {
	if v.p.RenderHunkActions == nil {
		return core.Group{}
	}
	return core.Each(func() []int {
		v.rev.Get()
		s, e := v.windowRows()
		var out []int
		for i := s; i < e && i < len(v.rows); i++ {
			r := v.rows[i]
			if r.hunk >= 0 && (i == 0 || v.rows[i-1].hunk != r.hunk) {
				out = append(out, r.hunk)
			}
		}
		return out
	}, func(h int) int { return h }, func(cx *core.Scope, h func() int, _ func() int) core.Node {
		hk := v.publicHunk(core.Untracked(cx, h))
		return core.View{
			PointerEvents: core.PointerEventsBoxNone,
			Class:         v.p.Classes.HunkActions,
			StyleFunc: func() core.Style {
				v.rev.Get()
				v.geo.Get()
				top := float32(0)
				for i, r := range v.rows {
					if r.hunk == h() {
						top = v.tops[i]
						break
					}
				}
				return merge(core.Style{Position: core.Absolute, Left: core.Pt(0), Width: core.Pt(core.Points(v.viewWidth() - 8)),
					Top: core.Pt(core.Points(top)), Direction: core.Row, JustifyContent: core.JustifyEnd}, v.styles().HunkActions)
			},
			Children: []core.Node{v.p.RenderHunkActions(cx, hk)},
		}
	})
}

func (v *dv) publicHunk(i int) Hunk {
	h := v.m.hunks[i]
	file := 0
	for fi, s := range v.segs {
		if i >= s.hunk0 && i < s.hunk1 {
			file = fi
		}
	}
	return Hunk{Index: i, File: file, OldStart: v.number(Old, min(h.oldStart, max(len(v.m.old)-1, 0))), OldLines: h.oldEnd - h.oldStart,
		NewStart: v.number(New, min(h.newStart, max(len(v.m.new)-1, 0))), NewLines: h.newEnd - h.newStart}
}

// lineRows is a slice of n rows, every one -1.
func lineRows(buf []int, n int) []int {
	if cap(buf) < n {
		buf = make([]int, n)
	}
	buf = buf[:n]
	for i := range buf {
		buf[i] = -1
	}
	return buf
}

func (v *dv) expandGap(gap, up, down int) {
	e := v.expand[gap]
	v.expand[gap] = [2]int{min(e[0]+up, math.MaxInt32), min(e[1]+down, math.MaxInt32)}
	v.build()
}

func (v *dv) toggleFile(path string) {
	v.collapsed[path] = !v.collapsed[path]
	for i := range v.segs {
		if v.segs[i].file.Path == path {
			v.segs[i].file.Collapsed = v.collapsed[path]
			if v.p.OnFileToggle != nil {
				v.p.OnFileToggle(v.segs[i].file)
			}
		}
	}
	v.build()
}

// nav is the change gone to last, while the view stays where it went:
// y is where, and reached whether the view got there.
type nav struct {
	hunk    int
	y       float32
	reached bool
}

// goTo scrolls row i to the top, as far as the rows go, going to change
// hunk (-1 for none).
func (v *dv) goTo(i, hunk int) {
	y := min(v.tops[i], max(v.height()-float32(v.viewport.GetUntracked().Height), 0))
	y = max(y, 0)
	v.nav = nil
	if hunk >= 0 {
		v.nav = &nav{hunk: hunk, y: y, reached: math.Abs(float64(float32(v.scrollY.GetUntracked())-y)) < 1}
	}
	v.vs.ScrollTo(core.Point{Y: core.Points(y)}, false)
	v.scrollY.Set(core.Points(y))
}

// scrolledTo follows the view to y: a change gone to was reached, or the
// user scrolled away from it once there, or far on the way.
func (v *dv) scrolledTo(y float32) {
	n := v.nav
	if n == nil {
		return
	}
	off := math.Abs(float64(n.y - y))
	switch {
	case off < 1:
		n.reached = true
	case n.reached || off > math.Max(float64(v.viewport.GetUntracked().Height), 1):
		v.nav = nil
	}
}

// span is a change's rows: from its first row's top to its last's bottom.
type span struct {
	hunk        int
	top, bottom float32
}

// hunkSpans are the shown changes' spans, in order.
func (v *dv) hunkSpans() []span {
	var out []span
	for i, r := range v.rows {
		if r.hunk < 0 || r.kind == rowSeparator || r.kind == rowFile {
			continue
		}
		if n := len(out); n > 0 && out[n-1].hunk == r.hunk {
			out[n-1].bottom = v.tops[i+1]
			continue
		}
		out = append(out, span{r.hunk, v.tops[i], v.tops[i+1]})
	}
	return out
}

// navFrom is the change by places after the one gone to, while the view
// stays there.
func (v *dv) navFrom(spans []span, by int) int {
	if v.nav == nil || len(spans) == 0 {
		return -1
	}
	at := slices.IndexFunc(spans, func(s span) bool { return s.hunk == v.nav.hunk })
	if at < 0 {
		return -1
	}
	n := len(spans)
	return spans[((at+by)%n+n)%n].hunk
}

// currentHunk is the change gone to while the view stays there, else the
// one at the top of the view, else the first starting in view.
func (v *dv) currentHunk() int {
	spans := v.hunkSpans()
	if h := v.navFrom(spans, 0); h >= 0 {
		return h
	}
	y, height := float32(v.scrollY.GetUntracked()), float32(v.viewport.GetUntracked().Height)
	for _, s := range spans {
		if s.top <= y && y < s.bottom {
			return s.hunk
		}
	}
	for _, s := range spans {
		if s.top >= y && s.top < y+height {
			return s.hunk
		}
	}
	return -1
}

// Hunks returns the changes, in order.
func (r *Ref) Hunks() []Hunk {
	if r.v == nil {
		return nil
	}
	out := make([]Hunk, len(r.v.m.hunks))
	for i := range out {
		out[i] = r.v.publicHunk(i)
	}
	return out
}

// Stats returns the lines added and deleted and the number of changes.
func (r *Ref) Stats() Stats {
	if r.v == nil {
		return Stats{}
	}
	return r.v.stats
}

// GoToHunk scrolls change i's first row to the top, expanding a collapsed
// file first.
func (r *Ref) GoToHunk(i int) {
	v := r.v
	if v == nil || i < 0 || i >= len(v.m.hunks) {
		return
	}
	for _, s := range v.segs {
		if i >= s.hunk0 && i < s.hunk1 && s.file.Collapsed {
			v.toggleFile(s.file.Path)
		}
	}
	for row, x := range v.rows {
		if x.hunk == i {
			v.goTo(row, i)
			return
		}
	}
}

// NextHunk scrolls the next change to the top and returns its index,
// after the last the first; -1 for none. It goes on from the change gone
// to while the view stays there, else from the top of the view.
func (r *Ref) NextHunk() int { return r.step(1) }

// PreviousHunk scrolls the previous change to the top and returns its
// index, before the first the last.
func (r *Ref) PreviousHunk() int { return r.step(-1) }

// CurrentHunk returns the change gone to while the view stays there,
// else the one at the top of the view or the first in view; -1 for none.
func (r *Ref) CurrentHunk() int {
	if r.v == nil {
		return -1
	}
	return r.v.currentHunk()
}

func (r *Ref) step(d int) int {
	v := r.v
	if v == nil {
		return -1
	}
	spans := v.hunkSpans()
	if len(spans) == 0 {
		return -1
	}
	next := v.navFrom(spans, d)
	if next < 0 {
		y := float32(v.scrollY.GetUntracked())
		if d > 0 {
			next = spans[0].hunk
			for _, s := range spans {
				if s.top > y+1 {
					next = s.hunk
					break
				}
			}
		} else {
			next = spans[len(spans)-1].hunk
			for _, s := range slices.Backward(spans) {
				if s.top < y-1 {
					next = s.hunk
					break
				}
			}
		}
	}
	r.GoToHunk(next)
	return next
}

// RevertHunk puts change i's old lines back into the new text and reports
// the texts.
func (r *Ref) RevertHunk(i int) {
	if v := r.v; v != nil && i >= 0 && i < len(v.m.hunks) && v.p.OnChange != nil {
		o, n := v.m.revert(v.m.hunks[i])
		v.load(o, n, "")
		v.p.OnChange(o, n)
	}
}

// AcceptHunk makes change i's new lines the old text's and reports the
// texts.
func (r *Ref) AcceptHunk(i int) {
	if v := r.v; v != nil && i >= 0 && i < len(v.m.hunks) && v.p.OnChange != nil {
		o, n := v.m.accept(v.m.hunks[i])
		v.load(o, n, "")
		v.p.OnChange(o, n)
	}
}

// Expand shows lines separator gap hides: every one, or lines (20 when
// 0) up or down.
func (r *Ref) Expand(gap int, how Expansion, lines int) {
	v := r.v
	if v == nil {
		return
	}
	if lines <= 0 {
		lines = 20
	}
	switch how {
	case ExpandUp:
		v.expandGap(gap, 0, lines)
	case ExpandDown:
		v.expandGap(gap, lines, 0)
	default:
		v.expandGap(gap, math.MaxInt32, math.MaxInt32)
	}
}

// ExpandAll shows every unchanged line; CollapseAll collapses them again.
func (r *Ref) ExpandAll() {
	if v := r.v; v != nil {
		for g := 0; g <= len(v.m.hunks)+len(v.segs); g++ {
			v.expand[g] = [2]int{math.MaxInt32, math.MaxInt32}
		}
		v.build()
	}
}

// CollapseAll collapses the unchanged lines again.
func (r *Ref) CollapseAll() {
	if v := r.v; v != nil {
		v.expand = map[int][2]int{}
		v.build()
	}
}

// Files returns a patch's files, in order.
func (r *Ref) Files() []File {
	if r.v == nil {
		return nil
	}
	var out []File
	for _, s := range r.v.segs {
		if s.patch {
			out = append(out, s.file)
		}
	}
	return out
}

// ToggleFile collapses the file at path, or expands it.
func (r *Ref) ToggleFile(path string) {
	if r.v != nil {
		r.v.toggleFile(path)
	}
}

// CollapseFile shows only the header of the file at path.
func (r *Ref) CollapseFile(path string) {
	if r.v != nil && !r.v.collapsed[path] {
		r.v.toggleFile(path)
	}
}

// ExpandFile shows the lines of the file at path again.
func (r *Ref) ExpandFile(path string) {
	if r.v != nil && r.v.collapsed[path] {
		r.v.toggleFile(path)
	}
}

// ScrollToFile scrolls the header of the file at path to the top.
func (r *Ref) ScrollToFile(path string) {
	if v := r.v; v != nil {
		for i, x := range v.rows {
			if x.kind == rowFile && v.segs[x.file].file.Path == path {
				v.goTo(i, -1)
				return
			}
		}
	}
}

// stylesMemo holds the parts' styles: styles, or what fn returns as it
// changes. Reading it is tracked, so a part's style follows.
func stylesMemo(cx *core.Scope, styles Styles, fn func() Styles) *core.Memo[*Styles] {
	return core.NewMemoFunc(cx, func() *Styles {
		if fn == nil {
			return &styles
		}
		s := fn()
		return &s
	}, nil)
}

// styles are the parts' styles, read where a part is styled.
func (v *dv) styles() *Styles { return v.looks.Get() }
