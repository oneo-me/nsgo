// Package filepicker is a modal file and folder picker, or a browser for
// copying paths, over a file system the app lists, possibly on another
// machine: places and disks in a sidebar, the browsed folder's entries in
// an icon or list view, and the folder's address, shown as breadcrumbs and
// typed into on request. The app loads folders and supplies every label,
// icon and place, and decides what may be chosen.
package filepicker

import (
	"fmt"
	"runtime"
	"time"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"
	"github.com/oneo-me/nsgo/ui"
)

// ViewMode is how a folder's entries are shown.
type ViewMode int

const (
	// Icons shows entries as a grid of large icons with their names.
	Icons ViewMode = iota
	// List shows entries as rows with their modification time and size.
	List
)

// Entry is a file or folder of a listed folder.
type Entry struct {
	Name, Path string
	Dir        bool
	Size       int64
	Modified   time.Time
	// Hidden marks an entry the system hides, shown only on request.
	Hidden bool
	// Data is the app's own value for the entry, for its Icon.
	Data any
}

// Crumb is a folder of the address's breadcrumbs.
type Crumb struct{ Name, Path string }

// Folder is a listed folder. Parent is empty at a root; Trail leads from
// the root to the folder itself, and without it the address shows the path.
type Folder struct {
	Path, Parent string
	Trail        []Crumb
	Entries      []Entry
}

// Place is a folder the sidebar opens.
type Place struct {
	Name, Path string
	Icon       func() *core.SVG
}

// Section is a headed group of the sidebar's places.
type Section struct {
	Label  string
	Places []Place
}

// Labels are the picker's texts. Title and Address name the dialog and the
// address field. Close names a browser's close button; CopyPath, when set,
// is the menu item copying an entry's or the folder's path.
type Labels struct {
	Title, Address        string
	Icons, List, Hidden   string
	Name, Modified, Size  string
	Empty, Cancel, Choose string
	Close, CopyPath       string
}

// State keeps a picker's folder, selection and view. Make it with NewState.
type State struct {
	// Load lists path, the app's starting folder when empty, and calls done
	// with the result on the engine.
	Load func(path string, done func(Folder, error))

	view       *core.Signal[ViewMode]
	hidden     *core.Signal[bool]
	width      *core.Signal[core.Points]
	folder     *core.Signal[*Folder]
	loading    *core.Signal[bool]
	failure    *core.Signal[string]
	selected   *ui.Selection
	editing    *core.Signal[bool]
	address    *core.Signal[string]
	generation uint64
}

// NewState makes a picker's state, showing view with hidden entries as
// shown and a sidebar width wide, its default when not positive.
func NewState(cx *core.Scope, view ViewMode, hidden bool, width core.Points) *State {
	if width <= 0 {
		width = 192
	}
	return &State{view: core.NewSignal(cx, view), hidden: core.NewSignal(cx, hidden), width: core.NewSignal(cx, width),
		folder: core.NewSignal(cx, &Folder{}), loading: core.NewSignal(cx, false), failure: core.NewSignal(cx, ""),
		selected: ui.NewSelection(cx, false), editing: core.NewSignal(cx, false), address: core.NewSignal(cx, "")}
}

// View, ShowHidden and SidebarWidth are what the app keeps of the picker.
func (s *State) View() ViewMode            { return s.view.GetUntracked() }
func (s *State) ShowHidden() bool          { return s.hidden.GetUntracked() }
func (s *State) SidebarWidth() core.Points { return s.width.GetUntracked() }

// selectedPath is the selected entry's path, empty for none.
func (s *State) selectedPath() string {
	if keys := s.selected.Keys(); len(keys) > 0 {
		return keys[0].(string)
	}
	return ""
}

// selectPath selects the entry of path, none when empty.
func (s *State) selectPath(path string) {
	if path == "" {
		s.selected.Clear()
		return
	}
	s.selected.Set(path)
}

// Open lists path, the starting folder when empty.
func (s *State) Open(path string) { s.load(path, "") }

// load lists path, selecting reveal once it does; listings of earlier loads
// are discarded.
func (s *State) load(path, reveal string) {
	s.generation++
	generation := s.generation
	s.loading.Set(true)
	s.failure.Set("")
	if s.Load == nil {
		return
	}
	s.Load(path, func(folder Folder, err error) {
		if generation != s.generation {
			return
		}
		s.loading.Set(false)
		if err != nil {
			folder = Folder{Path: path}
			s.failure.Set(err.Error())
		}
		s.folder.Set(&folder)
		s.selectPath(reveal)
	})
}

// entries are the folder's entries ShowHidden allows.
func (s *State) entries() []Entry {
	var shown []Entry
	for _, e := range s.folder.Get().Entries {
		if !e.Hidden || s.hidden.Get() {
			shown = append(shown, e)
		}
	}
	return shown
}

// chosen is the path Choose takes, if any.
func (p Dialog) chosen() (string, bool) {
	s := p.State
	folder := s.folder.Get()
	if s.loading.Get() || s.failure.Get() != "" || folder.Path == "" {
		return "", false
	}
	if id := s.selectedPath(); id != "" {
		for _, e := range folder.Entries {
			if e.Path == id && (e.Dir || p.Files) {
				return e.Path, true
			}
		}
	}
	return folder.Path, !p.Files
}

// copied is the path copying takes: the selected entry's, or else the
// folder's.
func (s *State) copied() string {
	if id := s.selectedPath(); id != "" {
		return id
	}
	if s.loading.GetUntracked() || s.failure.GetUntracked() != "" {
		return ""
	}
	return s.folder.GetUntracked().Path
}

// Dialog shows the picker in a modal dialog while Open. Choosing closes it
// and calls Choose. With CopyPath labeled, Command-C (Control-C elsewhere)
// copies the selected entry's path, or the folder's.
type Dialog struct {
	State  *State
	Open   func() bool
	OnOpen func(open bool)
	// OnClose follows the dialog being gone, once its motion ended.
	OnClose func()
	Labels  Labels
	// Device names the machine browsed, leading the breadcrumbs.
	Device string
	// Hint is a sentence beside the dialog's buttons, such as what the
	// chosen folder is for.
	Hint     string
	Sections []Section
	// SectionsFunc, when set, gives Sections as they change, which keeps
	// the dialog as they load.
	SectionsFunc func() []Section
	// Files lets files be chosen. Otherwise only folders are: Choose takes
	// the selected folder, or the one shown, and files show dimmed.
	Files bool
	// Icon draws an entry's icon at size; nil draws a folder or file icon.
	Icon func(cx *core.Scope, e Entry, size core.Points) core.Node
	// FormatTime and FormatSize format an entry's details; nil uses a
	// numeric date and decimal units.
	FormatTime func(time.Time) string
	FormatSize func(int64) string
	// Choose receives the chosen path; the dialog closes first. Without it
	// the dialog only browses: its header closes it, it has no buttons and
	// no entry shows dimmed.
	Choose func(path string)
	TestID string
}

// Render implements core.Node.
func (p Dialog) Render(cx *core.Scope) core.Node {
	s := p.State
	env := cx.Env()
	closeIt := func() { p.OnOpen(false) }
	choose := func(path string) {
		closeIt()
		if p.Choose != nil {
			p.Choose(path)
		}
	}
	mod := "ctrl"
	if runtime.GOOS == "darwin" {
		mod = "meta"
	}
	cx.OnKey(mod+"+shift+.", func() { s.hidden.Set(!s.hidden.GetUntracked()) })
	if p.Labels.CopyPath != "" {
		cx.OnKey(mod+"+c", func() {
			if path := s.copied(); !s.editing.GetUntracked() && path != "" {
				cx.SetClipboard(path)
			}
		})
	}
	children := []core.Node{p.header(cx, closeIt), divider(cx),
		core.View{Style: core.Style{Direction: core.Row, FlexGrow: 1, MinHeight: core.Pt(0)}, Children: []core.Node{
			core.View{Style: core.Style{Width: core.Pt(s.width.GetUntracked()), FlexShrink: 0, Height: core.Pct(100)}, Children: []core.Node{p.places(cx)}},
			core.View{Style: core.Style{Width: core.Pt(1), Height: core.Pct(100), FlexShrink: 0}, StyleFunc: ui.Filled(ui.ColorsOf(cx).Border)},
			p.content(cx, choose),
		}}}
	if p.Choose != nil {
		children = append(children, divider(cx), p.footer(cx, closeIt, choose))
	}
	return ui.Dialog{Open: p.Open, OnOpenChange: func(open bool, _ base.Reason) { p.OnOpen(open) }, OnClose: p.OnClose, Label: p.Labels.Title, TestID: p.TestID, Bare: true,
		Children: []core.Node{core.View{StyleFunc: func() core.Style {
			w := env.Window()
			return core.Style{Width: core.Pt(min(840, w.Width-80)), Height: core.Pt(min(560, w.Height-80)), MinHeight: core.Pt(0)}
		}, Children: children}}}
}

func divider(cx *core.Scope) core.Node {
	return core.View{Style: core.Style{Height: core.Pt(1), FlexShrink: 0}, StyleFunc: ui.Filled(ui.ColorsOf(cx).Border)}
}

// footer shows the hint beside Cancel and Choose.
func (p Dialog) footer(cx *core.Scope, closeIt func(), choose func(string)) core.Node {
	colors := ui.ColorsOf(cx)
	hint := ui.Text(p.Hint, 12, ui.Regular, colors.Muted)
	hint.MaxLines = 1
	hint.Style.FlexGrow, hint.Style.FlexShrink, hint.Style.MinWidth = 1, 1, core.Pt(0)
	return core.View{Style: core.Style{Direction: core.Row, Padding: core.Symmetric(12, 16), Gap: 8, AlignItems: core.AlignCenter}, Children: []core.Node{
		hint,
		ui.Button{Text: p.Labels.Cancel, Variant: ui.ButtonSecondary, OnPress: closeIt},
		ui.Button{Text: p.Labels.Choose, Disabled: func() bool { _, ok := p.chosen(); return !ok }, TestID: "filepicker-choose", OnPress: func() {
			if path, ok := p.chosenNow(cx); ok {
				choose(path)
			}
		}},
	}}
}

func (p Dialog) chosenNow(cx *core.Scope) (path string, ok bool) {
	cx.Untrack(func() { path, ok = p.chosen() })
	return path, ok
}

// header shows the folder's address beside the hidden entries and view
// switches, and a browser's close button.
func (p Dialog) header(cx *core.Scope, closeIt func()) core.Node {
	s := p.State
	address := core.Show{When: s.editing.Get,
		Then: func(cx *core.Scope) core.Node { return p.addressField(cx) },
		Else: func(cx *core.Scope) core.Node { return p.crumbs(cx) }}
	views := func(mode ViewMode, icon func() *core.SVG, label string) core.Node {
		return ui.IconButton{Icon: icon, Label: label, Size: 28, Selected: func() bool { return s.view.Get() == mode }, OnPress: func() { s.view.Set(mode) }}
	}
	children := []core.Node{
		core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), Margin: core.Edges{Right: 4}}, Children: []core.Node{address}},
		ui.IconButton{Icon: lucide.Eye, Label: p.Labels.Hidden, Size: 28, Selected: s.hidden.Get, OnPress: func() { s.hidden.Set(!s.hidden.GetUntracked()) }},
		views(Icons, lucide.LayoutGrid, p.Labels.Icons), views(List, lucide.List, p.Labels.List),
	}
	if p.Choose == nil {
		children = append(children, ui.IconButton{Icon: lucide.X, Label: p.Labels.Close, Size: 28, OnPress: closeIt})
	}
	return core.View{Style: core.Style{Direction: core.Row, Padding: core.Symmetric(12, 16), Gap: 4, AlignItems: core.AlignCenter}, Children: children}
}

// crumbs shows the address as breadcrumbs in a field's frame; an ancestor
// opens with the folder left selected, and the blank space edits the
// address as text.
func (p Dialog) crumbs(cx *core.Scope) core.Node {
	s := p.State
	colors := ui.ColorsOf(cx)
	return core.Dynamic(func(cx *core.Scope) core.Node {
		folder := s.folder.Get()
		trail := folder.Trail
		if len(trail) == 0 && folder.Path != "" {
			trail = []Crumb{{Name: folder.Path, Path: folder.Path}}
		}
		// The device leads, opening the starting folder; long paths keep
		// their nearest folders.
		const shown = 4
		var nodes []core.Node
		add := func(name string, open func()) {
			if len(nodes) > 0 {
				nodes = append(nodes, ui.Text("/", 12, ui.Regular, colors.Faint))
			}
			if open == nil {
				t := ui.Text(name, 12, ui.Medium, colors.Text)
				t.MaxLines = 1
				nodes = append(nodes, t)
				return
			}
			t := ui.Text(name, 12, ui.Regular, colors.Muted)
			t.MaxLines = 1
			nodes = append(nodes, base.Button{Label: name, OnPress: func(core.PressEvent) { open() }, Style: core.Style{FlexShrink: 0}, Children: []core.Node{t}})
		}
		if p.Device != "" {
			add(p.Device, func() { s.Open("") })
		}
		first := max(0, len(trail)-shown)
		ancestor := func(i int) func() { return func() { s.load(trail[i].Path, trail[i+1].Path) } }
		if first > 0 {
			add("…", ancestor(first-1))
		}
		for i := first; i < len(trail); i++ {
			if i == len(trail)-1 {
				add(trail[i].Name, nil)
			} else {
				add(trail[i].Name, ancestor(i))
			}
		}
		return base.Button{Label: p.Labels.Address, OnPress: func(core.PressEvent) {
			s.address.Set(folder.Path)
			s.editing.Set(true)
		}, Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 6, Height: core.Pt(ui.ButtonHeight), Padding: core.Symmetric(0, 8), Overflow: core.Hidden,
			CornerRadius: ui.RadiusControl, BorderWidth: 1},
			StyleFunc: func() core.Style {
				return core.Style{BorderColor: colors.Border(), Background: ui.Alpha(colors.Surface(), .5)}
			}, Children: nodes}
	})
}

// addressField edits the address as text: Enter opens it, and Escape or
// leaving the field returns to the breadcrumbs.
func (p Dialog) addressField(cx *core.Scope) core.Node {
	s := p.State
	return ui.Input{Value: s.address.Get, OnChange: s.address.Set, Label: p.Labels.Address, AutoFocus: true,
		OnBlur: func() { s.editing.Set(false) },
		OnSubmit: func(text string) {
			if text != "" && text != s.folder.GetUntracked().Path {
				s.Open(text)
			}
			s.editing.Set(false)
		}}
}

// places lists the sidebar's sections, following SectionsFunc.
func (p Dialog) places(cx *core.Scope) core.Node {
	if p.SectionsFunc != nil {
		return core.Dynamic(func(cx *core.Scope) core.Node { return p.sidebar(p.SectionsFunc()) })
	}
	return p.sidebar(p.Sections)
}

func (p Dialog) sidebar(list []Section) core.Node {
	s := p.State
	sections := make([]ui.SidebarSection, len(list))
	for i, section := range list {
		sections[i].Title = section.Label
		for _, place := range section.Places {
			sections[i].Items = append(sections[i].Items, ui.SidebarItem{ID: place.Path, Text: place.Name, Icon: place.Icon})
		}
	}
	return ui.Sidebar{Sections: sections, Selected: func() string { return s.folder.Get().Path }, OnSelect: func(path string) {
		if s.folder.GetUntracked().Path != path {
			s.Open(path)
		}
	}}
}

// message is text centered in the content.
func message(text string, color func() core.Color) core.Node {
	t := ui.Text(text, 13, ui.Regular, color)
	return core.View{Style: core.Style{FlexGrow: 1, Padding: core.All(24), JustifyContent: core.JustifyCenter, AlignItems: core.AlignCenter}, Children: []core.Node{t}}
}

// content shows the folder's entries in the chosen view.
func (p Dialog) content(cx *core.Scope, choose func(string)) core.Node {
	s := p.State
	colors := ui.ColorsOf(cx)
	activate := func(e Entry) {
		switch {
		case e.Dir:
			s.Open(e.Path)
		case p.Files && p.Choose != nil:
			choose(e.Path)
		}
	}
	body := core.Dynamic(func(cx *core.Scope) core.Node {
		entries := s.entries()
		failure := s.failure.Get()
		switch {
		case s.loading.Get():
			return core.View{Style: core.Style{FlexGrow: 1, JustifyContent: core.JustifyCenter, AlignItems: core.AlignCenter}, Children: []core.Node{ui.Spinner{}}}
		case failure != "":
			return message(failure, colors.Danger)
		case len(entries) == 0:
			return message(p.Labels.Empty, colors.Muted)
		case s.view.Get() == List:
			return p.listView(cx, entries, activate)
		}
		return p.iconView(cx, entries, activate)
	})
	return core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), MinHeight: core.Pt(0)}, Children: []core.Node{p.copyMenu(cx, s.copied, body)}}
}

// copyMenu offers to copy a path, when the app names the item.
func (p Dialog) copyMenu(cx *core.Scope, path func() string, trigger core.Node) core.Node {
	if p.Labels.CopyPath == "" {
		return trigger
	}
	return ui.ContextMenu{Trigger: trigger, Style: core.Style{FlexGrow: 1, MinHeight: core.Pt(0)}, Children: []core.Node{
		ui.MenuItem{Text: p.Labels.CopyPath, OnSelect: func() {
			if text := path(); text != "" {
				cx.SetClipboard(text)
			}
		}}}}
}

// entryDim is an entry's dimming.
func (p Dialog) entryDim(e Entry) core.Style {
	if e.Hidden || !e.Dir && !p.Files && p.Choose != nil {
		return core.Style{Opacity: core.Alpha(.45)}
	}
	return core.Style{}
}

// entryList lists entries in rows of the picker's selection: a press selects
// one and a second activates it. Each row's view is entry's.
func (p Dialog) entryList(cx *core.Scope, entries []Entry, activate func(Entry), columns int, rowStyle core.Style, entry func(e Entry) core.Node) core.Node {
	colors := ui.ColorsOf(cx)
	return ui.List{Count: func() int { return len(entries) }, Key: func(i int) any { return entries[i].Path }, Selection: p.State.selected,
		Columns: columns, TapSelects: true, OnActivate: func(i int) { activate(entries[i]) }, RowLabel: func(i int) string { return entries[i].Name },
		Row: func(_ *core.Scope, r core.ListRow) core.Node {
			return core.Dynamic(func(*core.Scope) core.Node {
				if i := r.Index(); i < len(entries) {
					return entry(entries[i])
				}
				return nil
			})
		},
		Style: core.Style{FlexGrow: 1}, RowStyle: rowStyle,
		RowStyleFunc: func() core.Style { return core.Style{Selected: &core.Style{Background: ui.Alpha(colors.Text(), .1)}} }}
}

// listView shows entries as rows under column headings.
func (p Dialog) listView(cx *core.Scope, entries []Entry, activate func(Entry)) core.Node {
	colors := ui.ColorsOf(cx)
	row := func(name, date, size core.Node) []core.Node {
		return []core.Node{
			core.View{Style: core.Style{Direction: core.Row, FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), Gap: 8, AlignItems: core.AlignCenter}, Children: []core.Node{name}},
			core.View{Style: core.Style{Width: core.Pt(160), FlexShrink: 0}, Children: []core.Node{date}},
			core.View{Style: core.Style{Width: core.Pt(80), FlexShrink: 0, AlignItems: core.AlignEnd}, Children: []core.Node{size}},
		}
	}
	head := func(s string) core.Node { return ui.Text(s, 11.5, ui.Regular, colors.Muted) }
	entry := func(e Entry) core.Node {
		date, size := "", ""
		if !e.Modified.IsZero() {
			date = p.formatTime(e.Modified)
		}
		if !e.Dir {
			size = p.formatSize(e.Size)
		}
		name := ui.Text(e.Name, 13, ui.Regular, colors.Text)
		name.MaxLines = 1
		return core.View{TestID: "filepicker-entry-" + e.Name,
			Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 12, FlexGrow: 1, MinWidth: core.Pt(0)}.Merge(p.entryDim(e)),
			Children: row(core.View{Style: core.Style{Direction: core.Row, Gap: 8, AlignItems: core.AlignCenter, MinWidth: core.Pt(0)}, Children: []core.Node{p.icon(cx, e, 18), name}},
				ui.Text(date, 12, ui.Regular, colors.Muted), ui.Text(size, 12, ui.Regular, colors.Muted))}
	}
	return core.View{Style: core.Style{FlexGrow: 1, MinHeight: core.Pt(0)}, Children: []core.Node{
		core.View{Style: core.Style{Direction: core.Row, Gap: 12, Padding: core.Symmetric(8, 20), FlexShrink: 0}, Children: row(head(p.Labels.Name), head(p.Labels.Modified), head(p.Labels.Size))},
		p.entryList(cx, entries, activate, 1, core.Style{Padding: core.Symmetric(6, 8)}, entry),
	}}
}

// iconCell is an icon view's cell's width.
const iconCell = 96

// iconView shows entries as a grid of large icons over their names, as
// many across as fit.
func (p Dialog) iconView(cx *core.Scope, entries []Entry, activate func(Entry)) core.Node {
	colors := ui.ColorsOf(cx)
	columns := core.NewSignal(cx, 1)
	entry := func(e Entry) core.Node {
		name := ui.Text(e.Name, 12, ui.Regular, colors.Text)
		name.MaxLines = 2
		name.Style.TextAlign = core.TextAlignCenter
		return core.View{TestID: "filepicker-entry-" + e.Name,
			Style:    core.Style{Height: core.Pt(88), Gap: 6, AlignItems: core.AlignCenter, FlexGrow: 1, MinWidth: core.Pt(0)}.Merge(p.entryDim(e)),
			Children: []core.Node{p.icon(cx, e, 44), name}}
	}
	return core.View{Style: core.Style{FlexGrow: 1, MinHeight: core.Pt(0)},
		OnLayout: func(b core.Bounds) { columns.Set(max(int(b.Size.Width-24)/iconCell, 1)) },
		Children: []core.Node{core.Dynamic(func(cx *core.Scope) core.Node {
			return p.entryList(cx, entries, activate, columns.Get(), core.Style{Padding: core.Symmetric(8, 4), AlignItems: core.AlignStart}, entry)
		})}}
}

func (p Dialog) icon(cx *core.Scope, e Entry, size core.Points) core.Node {
	if p.Icon != nil {
		return p.Icon(cx, e, size)
	}
	colors := ui.ColorsOf(cx)
	if e.Dir {
		return ui.Icon(lucide.Folder, size, colors.Accent)
	}
	return ui.Icon(lucide.File, size, colors.Muted)
}

func (p Dialog) formatTime(at time.Time) string {
	if p.FormatTime != nil {
		return p.FormatTime(at)
	}
	return at.Local().Format("2006-01-02 15:04")
}

// formatSize uses decimal units, as file managers do.
func (p Dialog) formatSize(n int64) string {
	if p.FormatSize != nil {
		return p.FormatSize(n)
	}
	return Size(n)
}

// Size is n bytes in decimal units.
func Size(n int64) string {
	if n < 1000 {
		return fmt.Sprintf("%d B", n)
	}
	value := float64(n)
	for _, unit := range []string{"KB", "MB", "GB", "TB"} {
		value /= 1000
		if value < 999.95 || unit == "TB" {
			return fmt.Sprintf("%.1f %s", value, unit)
		}
	}
	return ""
}
