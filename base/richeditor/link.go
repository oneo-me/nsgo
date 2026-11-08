package richeditor

import (
	"regexp"
	"strings"
	"unicode"
)

// LinkOptions tune Link.
type LinkOptions struct {
	// NoAutolink keeps URLs typed from becoming links, and NoOpenOnClick
	// presses from opening them.
	NoAutolink, NoOpenOnClick bool
	// Open opens a link pressed with Mod, or in a read-only editor.
	Open func(href string)
	// Protocols are allowed beside http, https, ftp, mailto and tel.
	Protocols []string
}

var (
	urlPattern = regexp.MustCompile(`(?i)\b(?:(?:https?|ftp)://[^\s<>"']+|www\.[^\s<>"']+\.[^\s<>"']+|[a-z0-9.-]+\.(?:com|org|net|io|dev|app|ai|co|me|edu|gov|cn|uk|de|jp|fr)(?:/[^\s<>"']*)?|[a-z0-9._%+-]+@[a-z0-9.-]+\.[a-z]{2,})`)
	allowedURI = regexp.MustCompile(`(?i)^(?:(?:https?|ftps?|mailto|tel|callto|sms|cid|xmpp):|[^a-z]|[a-z+.\-]+(?:[^a-z+.\-:]|$))`)
)

// FoundLink is a link in text: its byte range and address.
type FoundLink struct {
	Start, End int
	Href       string
}

// FindLinks detects URLs, host names and e-mail addresses in text, leaving
// trailing punctuation and unbalanced closing brackets out, as linkify does.
func FindLinks(text string) []FoundLink {
	var out []FoundLink
	for _, m := range urlPattern.FindAllStringIndex(text, -1) {
		start, end := m[0], m[1]
		for end > start {
			c := rune(text[end-1])
			if strings.ContainsRune(".,:;!?'\"", c) {
				end--
				continue
			}
			if open := strings.IndexRune(")]}", c); open >= 0 && strings.Count(text[start:end], string("([{"[open])) < strings.Count(text[start:end], string(c)) {
				end--
				continue
			}
			break
		}
		word := text[start:end]
		href := word
		switch {
		case strings.Contains(word, "://"):
		case strings.Contains(word, "@"):
			href = "mailto:" + word
		default:
			href = "https://" + word
		}
		out = append(out, FoundLink{start, end, href})
	}
	return out
}

// IsAllowedURI is Tiptap's check of a link's address.
func IsAllowedURI(href string, protocols []string) bool {
	for _, p := range protocols {
		if strings.HasPrefix(strings.ToLower(href), strings.ToLower(p)+":") {
			return true
		}
	}
	return href == "" || allowedURI.MatchString(strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return -1
		}
		return r
	}, href))
}

// SetLink links the selection.
func SetLink(t *MarkType, href string, protocols []string) Command {
	return func(s *State, dispatch func(*Transaction)) bool {
		if !IsAllowedURI(href, protocols) {
			return false
		}
		return SetMark(t, Attrs{"href": href})(s, dispatch)
	}
}

// Link is the link mark with autolinking, opening on Mod-click and linking a
// selection when a URL is pasted over it, not growing as text is typed after
// it.
func Link(o LinkOptions) Extension {
	return Extension{Name: "link",
		Marks: []MarkSpec{{Name: "link", NotInclusive: true, Attrs: []AttrSpec{{Name: "href"}, {Name: "target", HasDefault: true}, {Name: "rel", HasDefault: true}, {Name: "class", HasDefault: true}}}},
		Plugins: func(s *Schema) []*Plugin {
			t := s.Marks["link"]
			plugin := &Plugin{Key: "link",
				HandleClick: func(v *Session, pos, clicks int, mod bool) bool {
					if o.NoOpenOnClick || clicks != 1 || !mod && v.Editable() {
						return false
					}
					m := t.IsInSet(v.State().Doc.Resolve(pos).MarksHere())
					if m == nil {
						if after := v.State().Doc.Resolve(pos).NodeAfter(); after != nil {
							m = t.IsInSet(after.Marks)
						}
					}
					if m == nil {
						return false
					}
					if o.Open != nil {
						o.Open(m.Attrs["href"].(string))
					}
					return true
				},
				HandlePaste: func(v *Session, slice *Slice) bool {
					st := v.State()
					text := slice.Content.TextBetween(0, slice.Content.Size, "", "")
					links := FindLinks(text)
					if st.Selection.Empty() || len(links) != 1 || links[0].Start != 0 || links[0].End != len(text) {
						return false
					}
					v.Dispatch(st.Tr().AddMark(st.Selection.From(), st.Selection.To(), t.Create(Attrs{"href": links[0].Href})))
					return true
				},
			}
			if !o.NoAutolink {
				plugin.AppendTransaction = func(trs []*Transaction, old, n *State) *Transaction { return autolink(t, o, trs, old, n) }
			}
			return []*Plugin{plugin}
		},
		Commands: func(s *Schema) map[string]CommandFactory {
			t := s.Marks["link"]
			href := func(a Attrs) string {
				v, _ := a["href"].(string)
				return v
			}
			return map[string]CommandFactory{
				"setLink":   func(a Attrs) Command { return SetLink(t, href(a), o.Protocols) },
				"unsetLink": noArgs(UnsetMark(t)),
				"toggleLink": func(a Attrs) Command {
					return func(st *State, dispatch func(*Transaction)) bool {
						if IsMarkActive(st, t, nil) {
							return UnsetMark(t)(st, dispatch)
						}
						return SetLink(t, href(a), o.Protocols)(st, dispatch)
					}
				},
			}
		}}
}

// autolink links the word before the white space a change typed.
func autolink(t *MarkType, o LinkOptions, trs []*Transaction, old, n *State) *Transaction {
	changed := false
	for _, tr := range trs {
		if tr.DocChanged() && tr.Origin != OriginHistory && tr.Meta(preventAutolink) == nil {
			changed = true
		}
	}
	if !changed || old.Doc.Eq(n.Doc) {
		return nil
	}
	sel, ok := n.Selection.(*TextSelection)
	if !ok || sel.Cursor() == nil {
		return nil
	}
	cursor := sel.Cursor()
	parent := cursor.Parent()
	if parent.Type.Spec.Code {
		return nil
	}
	if cursor.ParentOffset == 0 {
		// Enter after a word: the block before ends with it.
		if cursor.Depth == 0 || cursor.Index(-1) == 0 {
			return nil
		}
		before := cursor.Node(-1).Child(cursor.Index(-1) - 1)
		if !before.IsTextblock() {
			return nil
		}
		return linkLastWord(t, n, before, cursor.Before(cursor.Depth)-before.NodeSize()+1, before.Content.Size, n.Tr())
	}
	text := parent.TextBetween(0, cursor.ParentOffset, "", "￼")
	runes := []rune(text)
	if !unicode.IsSpace(runes[len(runes)-1]) {
		return nil
	}
	return linkLastWord(t, n, parent, cursor.Start(cursor.Depth), cursor.ParentOffset-1, n.Tr())
}

func linkLastWord(t *MarkType, n *State, parent *Node, start, end int, tr *Transaction) *Transaction {
	text := parent.TextBetween(0, end, "", "￼")
	word := strings.TrimRightFunc(text, unicode.IsSpace)
	at := strings.LastIndexFunc(word, unicode.IsSpace) + 1
	if at > 0 {
		at += len(string([]rune(word[at-1:])[0])) - 1
	}
	word = word[at:]
	links := FindLinks(word)
	if len(links) != 1 {
		return nil
	}
	l := links[0]
	from := start + utf16Len(text[:at]) + utf16Len(word[:l.Start])
	to := from + utf16Len(word[l.Start:l.End])
	if n.Doc.RangeHasMark(from, to, t) || n.Schema.Marks["code"] != nil && n.Doc.RangeHasMark(from, to, n.Schema.Marks["code"]) {
		return nil
	}
	return tr.AddMark(from, to, t.Create(Attrs{"href": l.Href})).SetMeta(preventAutolink, true)
}

const preventAutolink metaKey = "preventAutolink"

// CharacterCountOptions tune CharacterCount.
type CharacterCountOptions struct {
	// Limit is the most characters, none when 0.
	Limit int
	// CountNodes counts every position rather than text.
	CountNodes bool
}

const characterCountKey = "characterCount"

// Characters counts a state's characters.
func Characters(s *State, nodes bool) int {
	if nodes {
		return s.Doc.Content.Size
	}
	return len([]rune(s.Doc.TextBetween(0, s.Doc.Content.Size, " ", " ")))
}

// Words counts a state's words.
func Words(s *State) int {
	return len(strings.Fields(s.Doc.TextBetween(0, s.Doc.Content.Size, " ", " ")))
}

// CharacterCount refuses changes that grow past the limit, a document over it
// already may shrink, and trims what is pasted to what fits.
func CharacterCount(o CharacterCountOptions) Extension {
	count := func(doc *Node) int {
		if o.CountNodes {
			return doc.Content.Size
		}
		return len([]rune(doc.TextBetween(0, doc.Content.Size, " ", " ")))
	}
	return Extension{Name: characterCountKey, Plugins: func(*Schema) []*Plugin {
		return []*Plugin{{Key: characterCountKey,
			Init: func(*State) any { return o.Limit },
			Apply: func(tr *Transaction, v any, _, _ *State) any {
				if l, ok := tr.Meta(characterCountKey).(int); ok {
					return l
				}
				return v
			},
			FilterTransaction: func(tr *Transaction, s *State) bool {
				limit, _ := s.PluginState(characterCountKey).(int)
				if limit == 0 || !tr.DocChanged() {
					return true
				}
				was, now := count(tr.Before()), count(tr.Doc)
				return now <= limit || now <= was
			},
			TransformPasted: func(v *Session, slice *Slice) *Slice {
				st := v.State()
				limit, _ := st.PluginState(characterCountKey).(int)
				if limit == 0 {
					return slice
				}
				room := limit - count(st.Doc)
				if !st.Selection.Empty() {
					room += count(st.Doc) - count(mustDelete(st))
				}
				if room >= slice.Size() {
					return slice
				}
				lo, hi := 0, slice.Size()
				for lo < hi {
					mid := (lo + hi + 1) / 2
					trimmed := NewSlice(slice.Content.Cut(0, mid+slice.OpenStart), slice.OpenStart, 0)
					if len([]rune(trimmed.Content.TextBetween(0, trimmed.Content.Size, " ", " "))) <= room {
						lo = mid
					} else {
						hi = mid - 1
					}
				}
				return NewSlice(slice.Content.Cut(0, lo+slice.OpenStart), slice.OpenStart, 0)
			},
		}}
	}, Commands: func(*Schema) map[string]CommandFactory {
		return map[string]CommandFactory{"setCharacterLimit": func(a Attrs) Command {
			return func(s *State, dispatch func(*Transaction)) bool {
				if dispatch != nil {
					dispatch(s.Tr().SetMeta(characterCountKey, int(num(a["limit"]))))
				}
				return true
			}
		}}
	}}
}

func mustDelete(s *State) *Node { return s.Tr().DeleteSelection().Doc }

// SuggestionOptions tune a Suggestion.
type SuggestionOptions struct {
	// Key names it among an editor's, "suggestion" when "".
	Key string
	// Char is what starts it, "@" when "".
	Char string
	// AllowSpaces lets a query hold spaces.
	AllowSpaces bool
	// AllowedPrefixes are what may go before Char, " " and a line's start
	// when nil.
	AllowedPrefixes []string
	// Allow says a match may become active.
	Allow func(s *State, from, to int) bool
	// Class goes on the active range.
	Class string
	// CaptureKeys are taken while it is active, for the app's list.
	CaptureKeys []string
	// OnKey hears keys while it is active, and those it returns true for
	// are taken.
	OnKey func(key string) bool
}

// SuggestionState is what a Suggestion's plugin holds.
type SuggestionState struct {
	Active   bool
	From, To int
	Query    string
	// Text is the trigger and the query.
	Text string
	// dismissedFrom is the range Escape closed, until the trigger moves or
	// white space is typed.
	dismissedFrom int
	// LastKey and KeyCount say what was captured, for an app reading them
	// after the fact.
	LastKey  string
	KeyCount int
}

const suggestionExit metaKey = "suggestionExit"

// suggestionKey is the meta key of a captured key.
type suggestionKey struct{ key string }

func findSuggestion(r *ResolvedPos, o SuggestionOptions) (from, to int, query, text string, ok bool) {
	parent := r.Parent()
	if !parent.IsTextblock() || parent.Type.Spec.Code {
		return
	}
	before := parent.TextBetween(0, r.ParentOffset, "", "￼")
	at := strings.LastIndex(before, o.Char)
	if at < 0 {
		return
	}
	prefix := before[:at]
	allowed := o.AllowedPrefixes == nil && (prefix == "" || strings.HasSuffix(prefix, " ") || strings.HasSuffix(prefix, "\n"))
	if prefix == "" {
		allowed = true
	}
	for _, p := range o.AllowedPrefixes {
		if strings.HasSuffix(prefix, p) {
			allowed = true
		}
	}
	rest := before[at+len(o.Char):]
	if !allowed || !o.AllowSpaces && strings.ContainsAny(rest, " \n￼") || strings.Contains(rest, "￼") {
		return
	}
	start := r.Start(r.Depth) + utf16Len(before[:at])
	return start, r.Pos, rest, o.Char + rest, true
}

// Suggestion follows a trigger and the query after it, for the app's list,
// which reads SuggestionStateOf and replaces the range.
func Suggestion(o SuggestionOptions) *Plugin {
	o.Key, o.Char = cmpOr(o.Key, "suggestion"), cmpOr(o.Char, "@")
	return &Plugin{Key: o.Key,
		Init: func(*State) any { return &SuggestionState{dismissedFrom: -1} },
		Apply: func(tr *Transaction, v any, _, n *State) any {
			prev := v.(*SuggestionState)
			next := &SuggestionState{dismissedFrom: prev.dismissedFrom, LastKey: prev.LastKey, KeyCount: prev.KeyCount}
			if next.dismissedFrom >= 0 {
				next.dismissedFrom = tr.Mapping.Map(next.dismissedFrom, 1)
			}
			if k, ok := tr.Meta(suggestionKey{o.Key}).(string); ok {
				next.LastKey, next.KeyCount = k, prev.KeyCount+1
			}
			if tr.Meta(suggestionExit) == o.Key {
				next.dismissedFrom = prev.From
				return next
			}
			sel, ok := n.Selection.(*TextSelection)
			if !ok || sel.Cursor() == nil {
				return next
			}
			from, to, query, text, found := findSuggestion(sel.Cursor(), o)
			if !found {
				next.dismissedFrom = -1
				return next
			}
			if from == next.dismissedFrom {
				return next
			}
			next.dismissedFrom = -1
			if o.Allow != nil && !o.Allow(n, from, to) {
				return next
			}
			next.Active, next.From, next.To, next.Query, next.Text = true, from, to, query, text
			return next
		},
		HandleKeyDown: func(v *Session, key string) bool {
			st, _ := v.State().PluginState(o.Key).(*SuggestionState)
			if st == nil || !st.Active {
				return false
			}
			if key == "Escape" {
				v.Dispatch(v.State().Tr().SetMeta(suggestionExit, o.Key).SetMeta(addToHistory, false))
				return true
			}
			if o.OnKey != nil && o.OnKey(key) {
				return true
			}
			for _, c := range o.CaptureKeys {
				if c == key {
					v.Dispatch(v.State().Tr().SetMeta(suggestionKey{o.Key}, key))
					return true
				}
			}
			return false
		},
		Decorations: func(s *State) []Decoration {
			st, _ := s.PluginState(o.Key).(*SuggestionState)
			if st == nil || !st.Active || o.Class == "" {
				return nil
			}
			return []Decoration{{From: st.From, To: st.To, Class: o.Class}}
		}}
}

// SuggestionStateOf is the state of the Suggestion keyed key.
func SuggestionStateOf(s *State, key string) *SuggestionState {
	st, _ := s.PluginState(cmpOr(key, "suggestion")).(*SuggestionState)
	return st
}

// ExitSuggestion dismisses the Suggestion keyed key.
func ExitSuggestion(key string) Command {
	key = cmpOr(key, "suggestion")
	return func(s *State, dispatch func(*Transaction)) bool {
		st := SuggestionStateOf(s, key)
		if st == nil || !st.Active {
			return false
		}
		if dispatch != nil {
			dispatch(s.Tr().SetMeta(suggestionExit, key))
		}
		return true
	}
}

// MentionOptions tune Mention.
type MentionOptions struct {
	Suggestion SuggestionOptions
}

// Mention is an inline node showing @name with its one position, and the
// Suggestion that finds it. Id identifies who, so nothing goes by the text.
func Mention(o MentionOptions) Extension {
	char := cmpOr(o.Suggestion.Char, "@")
	o.Suggestion.Key = cmpOr(o.Suggestion.Key, "mention")
	return Extension{Name: "mention",
		Nodes: []NodeSpec{{Name: "mention", Inline: true, Group: "inline", Atom: true, Attrs: []AttrSpec{{Name: "id"}, {Name: "label", HasDefault: true}},
			LeafText:   func(n *Node) string { return char + cmpOr(n.AttrStr("label"), n.AttrStr("id")) },
			RenderText: func(n *Node) string { return char + cmpOr(n.AttrStr("label"), n.AttrStr("id")) }}},
		Plugins: func(*Schema) []*Plugin { return []*Plugin{Suggestion(o.Suggestion)} },
		Commands: func(s *Schema) map[string]CommandFactory {
			return map[string]CommandFactory{"insertMention": func(a Attrs) Command { return InsertMention(s.Nodes["mention"], o.Suggestion.Key, a) }}
		}}
}

// InsertMention replaces the active query with a mention and a space after
// it, one already there taken in.
func InsertMention(t *NodeType, key string, attrs Attrs) Command {
	return func(s *State, dispatch func(*Transaction)) bool {
		st := SuggestionStateOf(s, key)
		from, to := s.Selection.From(), s.Selection.To()
		if st != nil && st.Active {
			from, to = st.From, st.To
		}
		if dispatch != nil {
			if after := s.Doc.Resolve(to).NodeAfter(); after != nil && after.IsText() && strings.HasPrefix(after.Text, " ") {
				to++
			}
			node := t.Create(attrs, nil, nil)
			tr := s.Tr().ReplaceWith(from, to, node, s.Schema.Text(" "))
			end := from + node.NodeSize() + 1
			tr.SetSelection(TextSelectionAt(tr.Doc, end, end))
			dispatch(tr.ScrollIntoView())
		}
		return true
	}
}
