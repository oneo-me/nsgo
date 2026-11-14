package core

import "strings"

// SuggestItem is a row a Suggest offers while a trigger query is active.
type SuggestItem struct {
	ID, Label, Detail string
	Icon              func() *SVG
	// Insert replaces the trigger and query. Label is used when Insert is empty.
	Insert   string
	Disabled bool
}

// SuggestOptions tune a Suggest.
type SuggestOptions struct {
	// Trigger starts a query, "@" when empty.
	Trigger string
	// Items returns rows for a query. It is called once per distinct query.
	Items func(query string) []SuggestItem
	// AllowSpaces lets a query contain spaces.
	AllowSpaces bool
	// AllowedPrefixes are what may precede Trigger. When nil, a line start or
	// a space is accepted.
	AllowedPrefixes []string
	OnOpen, OnClose func()
}

// Suggest tracks a trigger query in a text field and accepts one of its rows.
// It is created in the field's owner scope and is safe on that scope's
// engine goroutine, like signals and the rest of the reactive graph.
type Suggest struct {
	cx   *Scope
	opts SuggestOptions

	open     *Signal[bool]
	query    *Signal[string]
	items    *Signal[[]SuggestItem]
	selected *Signal[int]
	anchor   *Signal[Bounds]

	value     string
	selection TextRange
	from, to  int
	lastQuery string
	onChange  func(string)
	ref       *Ref
}

// NewSuggest creates a suggestion state for a field in cx's scope.
func NewSuggest(cx *Scope, o SuggestOptions) *Suggest {
	if o.Trigger == "" {
		o.Trigger = "@"
	}
	s := &Suggest{cx: cx, opts: o, open: NewSignal(cx, false), query: NewSignal(cx, ""),
		items:    NewSignalFunc[[]SuggestItem](cx, nil, func(a, b []SuggestItem) bool { return false }),
		selected: NewSignal(cx, 0), anchor: NewSignal(cx, Bounds{})}
	return s
}

// Open reports whether the suggestion list is active.
func (s *Suggest) Open() bool { return s != nil && s.open.Get() }

// Query is the text after the trigger and before the caret.
func (s *Suggest) Query() string { return s.query.Get() }

// Items returns the current rows, including an empty result.
func (s *Suggest) Items() []SuggestItem { return s.items.Get() }

// Selected reports the highlighted row index, or zero when there are no rows.
func (s *Suggest) Selected() int { return s.selected.Get() }

// SetSelected highlights i, clamped to the current rows.
func (s *Suggest) SetSelected(i int) {
	if !s.Open() {
		return
	}
	rows := s.items.GetUntracked()
	if len(rows) == 0 {
		i = 0
	} else if i < 0 {
		i = len(rows) - 1
	} else if i >= len(rows) {
		i = 0
	}
	s.selected.Set(i)
}

// Anchor is the caret's latest page bounds, suitable for Overlay.AnchorBounds.
func (s *Suggest) Anchor() Bounds { return s.anchor.Get() }

// Accept inserts the selected row and closes the list. Disabled rows are not
// accepted.
func (s *Suggest) Accept() {
	if !s.Open() {
		return
	}
	rows := s.items.GetUntracked()
	i := s.selected.GetUntracked()
	if i < 0 || i >= len(rows) || rows[i].Disabled {
		return
	}
	item := rows[i]
	insert := item.Insert
	if insert == "" {
		insert = item.Label
	}
	start := UTF16Offset(s.value, s.from)
	end := UTF16Offset(s.value, s.to)
	if start < 0 || end < start || start > len(s.value) || end > len(s.value) {
		return
	}
	value := s.value[:start] + insert + s.value[end:]
	s.value = value
	if s.onChange != nil {
		s.onChange(value)
	}
	caret := s.from + UTF16Len(insert)
	s.selection = TextRange{Start: caret, End: caret}
	if s.ref != nil {
		s.ref.SetSelection(caret, caret)
	}
	s.close()
}

// Close dismisses an active list without changing the field.
func (s *Suggest) Close() { s.close() }

func (s *Suggest) close() {
	if !s.open.GetUntracked() {
		return
	}
	s.open.Set(false)
	if s.opts.OnClose != nil {
		s.opts.OnClose()
	}
}

// Keys returns a node that captures navigation and acceptance keys while the
// suggestion is open. Put it beside the field that owns the Suggest.
func (s *Suggest) Keys() Node {
	return Show{When: func() bool { return s != nil && s.Open() }, Then: func(cx *Scope) Node {
		for _, key := range []string{"up", "down", "pageup", "pagedown", "enter", "tab", "escape"} {
			cx.OnKeyWith(key, KeyOptions{When: KeyAlways, Repeat: key == "up" || key == "down" || key == "pageup" || key == "pagedown"}, func() {
				s.key(key)
			})
		}
		return nil
	}}
}

func (s *Suggest) key(key string) {
	if !s.Open() {
		return
	}
	switch key {
	case "up":
		s.SetSelected(s.selected.GetUntracked() - 1)
	case "down":
		s.SetSelected(s.selected.GetUntracked() + 1)
	case "pageup":
		s.SetSelected(s.selected.GetUntracked() - 5)
	case "pagedown":
		s.SetSelected(s.selected.GetUntracked() + 5)
	case "enter", "tab":
		s.Accept()
	case "escape":
		s.Close()
	}
}

// bind attaches a field's controlled value, change callback and ref.
func (s *Suggest) bind(value func() string, onChange func(string), ref *Ref) {
	s.onChange, s.ref = onChange, ref
	if value != nil {
		v := value()
		if v != s.value {
			s.changed(v)
		}
	}
}

func (s *Suggest) changed(value string) {
	s.value = value
	if s.selection.End > UTF16Len(value) {
		s.selection = TextRange{Start: UTF16Len(value), End: UTF16Len(value)}
	}
	s.update()
}

func (s *Suggest) selectionChanged(r TextRange) {
	s.selection = r
	s.update()
}

func (s *Suggest) caretMoved(e CaretEvent) { s.anchor.Set(e.PageBounds) }

func (s *Suggest) update() {
	if s.selection.Start != s.selection.End {
		s.close()
		return
	}
	from, to, query, ok := suggestionRange(s.value, s.selection.End, s.opts)
	if !ok {
		s.close()
		return
	}
	s.from, s.to = from, to
	if s.open.GetUntracked() && query == s.lastQuery {
		s.query.Set(query)
		return
	}
	s.lastQuery = query
	s.query.Set(query)
	rows := []SuggestItem(nil)
	if s.opts.Items != nil {
		rows = s.opts.Items(query)
	}
	s.items.Set(rows)
	s.selected.Set(0)
	if !s.open.GetUntracked() {
		s.open.Set(true)
		if s.opts.OnOpen != nil {
			s.opts.OnOpen()
		}
	}
}

func suggestionRange(value string, caret int, o SuggestOptions) (from, to int, query string, ok bool) {
	if o.Trigger == "" {
		o.Trigger = "@"
	}
	if caret < 0 || caret > UTF16Len(value) {
		return
	}
	before := value[:UTF16Offset(value, caret)]
	at := strings.LastIndex(before, o.Trigger)
	if at < 0 {
		return
	}
	prefix := before[:at]
	allowed := false
	if o.AllowedPrefixes == nil {
		allowed = prefix == "" || strings.HasSuffix(prefix, " ") || strings.HasSuffix(prefix, "\n")
	} else {
		for _, p := range o.AllowedPrefixes {
			if strings.HasSuffix(prefix, p) {
				allowed = true
				break
			}
		}
	}
	if !allowed {
		return
	}
	query = before[at+len(o.Trigger):]
	if !o.AllowSpaces && strings.ContainsAny(query, " \n\t") {
		query = ""
		return
	}
	return UTF16Len(before[:at]), caret, query, true
}
