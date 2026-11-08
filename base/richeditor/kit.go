package richeditor

import (
	"cmp"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// DefaultPriority is an extension's unless it gives one: higher runs first.
const DefaultPriority = 100

// CommandFactory makes a named command from its arguments.
type CommandFactory func(args Attrs) Command

// GlobalAttrs add attributes to other types.
type GlobalAttrs struct {
	Types []string
	Attrs []AttrSpec
}

// Extension contributes to an editor in one unit, as Tiptap's do: to the
// schema, the keys, the rules, the plugins and the named commands.
type Extension struct {
	Name     string
	Priority int
	Nodes    []NodeSpec
	Marks    []MarkSpec
	Global   []GlobalAttrs
	Keys     func(s *Schema) map[string]Command
	Rules    func(s *Schema) []InputRule
	Plugins  func(s *Schema) []*Plugin
	Commands func(s *Schema) map[string]CommandFactory
}

// Built is what a list of extensions makes.
type Built struct {
	Schema   *Schema
	Plugins  []*Plugin
	Commands map[string]CommandFactory
}

func (e *Extension) priority() int { return cmp.Or(e.Priority, DefaultPriority) }

// Build resolves extensions, refusing duplicate names: the schema in their
// order by priority, and the runtime in Tiptap's, the list reversed then by
// priority, so an app's extension after the kit binds before it, under one
// keymap merging every extension's bindings.
func Build(exts ...Extension) (*Built, error) {
	sorted := slices.Clone(exts)
	slices.SortStableFunc(sorted, func(a, b Extension) int { return b.priority() - a.priority() })
	seen := map[string]bool{}
	spec := SchemaSpec{}
	for _, e := range sorted {
		if seen[e.Name] {
			return nil, fmt.Errorf("richeditor: two extensions named %q", e.Name)
		}
		seen[e.Name] = true
		spec.Nodes = append(spec.Nodes, e.Nodes...)
		spec.Marks = append(spec.Marks, e.Marks...)
	}
	for _, e := range sorted {
		for _, g := range e.Global {
			for i := range spec.Nodes {
				if slices.Contains(g.Types, spec.Nodes[i].Name) {
					spec.Nodes[i].Attrs = append(slices.Clone(spec.Nodes[i].Attrs), g.Attrs...)
				}
			}
		}
	}
	schema, err := NewSchema(spec)
	if err != nil {
		return nil, err
	}
	runtimeOrder := slices.Clone(exts)
	slices.Reverse(runtimeOrder)
	slices.SortStableFunc(runtimeOrder, func(a, b Extension) int { return b.priority() - a.priority() })
	built := &Built{Schema: schema, Commands: map[string]CommandFactory{}}
	var rules []InputRule
	var keymaps []map[string]Command
	var plugins []*Plugin
	for _, e := range runtimeOrder {
		if e.Rules != nil {
			rules = append(rules, e.Rules(schema)...)
		}
		if e.Keys != nil {
			keymaps = append(keymaps, e.Keys(schema))
		}
		if e.Plugins != nil {
			plugins = append(plugins, e.Plugins(schema)...)
		}
		if e.Commands != nil {
			for name, f := range e.Commands(schema) {
				built.Commands[name] = f
			}
		}
	}
	keymaps = append(keymaps, BaseKeymap())
	if len(rules) > 0 {
		built.Plugins = append(built.Plugins, InputRules(rules))
	}
	// Plugins come before the keymap: a suggestion's list and a cell
	// selection take their keys before Enter and Backspace edit.
	built.Plugins = append(built.Plugins, plugins...)
	built.Plugins = append(built.Plugins, Keymap("keymap", MergeBindings(keymaps...)))
	return built, nil
}

func bind(name string, c Command) map[string]Command { return map[string]Command{name: c} }

func noArgs(c Command) CommandFactory { return func(Attrs) Command { return c } }

func group(kw ...string) string { return strings.Join(kw, " ") }

func listExtra(marker string) map[string]any { return map[string]any{"listMarker": marker} }

func has[T any](m map[string]*T, name string, f func(*T) Command) Command {
	if t := m[name]; t != nil {
		return f(t)
	}
	return func(*State, func(*Transaction)) bool { return false }
}

// IsMarkActive says the selection is in a mark of t with attrs.
func IsMarkActive(s *State, t *MarkType, attrs Attrs) bool {
	sel := s.Selection
	match := func(marks []*Mark) bool {
		m := t.IsInSet(marks)
		if m == nil {
			return false
		}
		for k, v := range attrs {
			if !attrsEqual(Attrs{k: m.Attrs[k]}, Attrs{k: normalize(v)}) {
				return false
			}
		}
		return true
	}
	if sel.Empty() {
		return match(orMarksHere(s, sel.FromPos()))
	}
	found, all := false, true
	for _, r := range sel.Ranges() {
		s.Doc.NodesBetween(r.From.Pos, r.To.Pos, func(n *Node, _ int, _ *Node, _ int) bool {
			if n.IsInline() && (n.IsText() || n.IsLeaf()) {
				if match(n.Marks) {
					found = true
				} else if n.IsText() && strings.TrimSpace(n.Text) != "" {
					all = false
				}
			}
			return true
		}, 0)
	}
	return found && all
}

// IsNodeActive says the selection is in a node of t with attrs.
func IsNodeActive(s *State, t *NodeType, attrs Attrs) bool {
	return FindParentNode(s, func(n *Node) bool {
		if n.Type != t {
			return false
		}
		for k, v := range attrs {
			if !attrsEqual(Attrs{k: n.Attrs[k]}, Attrs{k: normalize(v)}) {
				return false
			}
		}
		return true
	}) >= 0 || isNodeSelected(s, t)
}

func isNodeSelected(s *State, t *NodeType) bool {
	n, ok := s.Selection.(*NodeSelection)
	return ok && n.Node.Type == t
}

// FindParentNode is the position before the nearest ancestor of the
// selection pred accepts, -1 for none.
func FindParentNode(s *State, pred func(*Node) bool) int {
	r := s.Selection.FromPos()
	for d := r.Depth; d > 0; d-- {
		if pred(r.Node(d)) {
			return r.Before(d)
		}
	}
	return -1
}

// IsActive says the node or mark of name, with attrs, is at the selection.
func IsActive(s *State, name string, attrs Attrs) bool {
	if t := s.Schema.Marks[name]; t != nil {
		return IsMarkActive(s, t, attrs)
	}
	if t := s.Schema.Nodes[name]; t != nil {
		return IsNodeActive(s, t, attrs)
	}
	return false
}

// ToggleBlockType sets the textblock to t, or back to paragraph-like
// default when it is t.
func ToggleBlockType(t *NodeType, attrs Attrs) Command {
	return func(s *State, dispatch func(*Transaction)) bool {
		if IsNodeActive(s, t, attrs) {
			return SetBlockType(s.Schema.DefaultBlock(), nil)(s, dispatch)
		}
		return SetBlockType(t, attrs)(s, dispatch)
	}
}

// ToggleWrap wraps the selection in t, or lifts it out.
func ToggleWrap(t *NodeType, attrs Attrs) Command {
	return func(s *State, dispatch func(*Transaction)) bool {
		if IsNodeActive(s, t, attrs) {
			return Lift(s, dispatch)
		}
		return WrapIn(t, attrs)(s, dispatch)
	}
}

func isListType(t *NodeType) bool {
	return t.Spec.Extra["listMarker"] != nil || t.Spec.Extra["list"] == true
}

// ToggleList wraps the selection in a list of t, lifts it out, or turns the
// list it is in into one of t.
func ToggleList(t, item *NodeType) Command {
	return func(s *State, dispatch func(*Transaction)) bool {
		r := s.Selection.FromPos()
		for d := r.Depth; d > 0; d-- {
			n := r.Node(d)
			if !isListType(n.Type) {
				continue
			}
			if n.Type == t {
				return LiftListItem(item)(s, dispatch)
			}
			if t.ContentMatch.MatchType(n.FirstChild().Type) != nil {
				if dispatch != nil {
					dispatch(s.Tr().SetNodeMarkup(r.Before(d), t, nil, nil).ScrollIntoView())
				}
				return true
			}
			break
		}
		return WrapInList(t, nil)(s, dispatch)
	}
}

// SetMark marks the selection, or the stored marks at a cursor.
func SetMark(t *MarkType, attrs Attrs) Command {
	return func(s *State, dispatch func(*Transaction)) bool {
		if dispatch != nil {
			tr := s.Tr()
			if s.Selection.Empty() {
				tr.AddStoredMark(t.Create(attrs))
			} else {
				for _, r := range s.Selection.Ranges() {
					tr.AddMark(r.From.Pos, r.To.Pos, t.Create(attrs))
				}
			}
			dispatch(tr)
		}
		return true
	}
}

// MarkRange is the range of the mark of t around pos, its marks' run.
func MarkRange(r *ResolvedPos, t *MarkType) (from, to int, ok bool) {
	parent := r.Parent()
	index := r.Index(r.Depth)
	child := parent.MaybeChild(index)
	if child == nil || t.IsInSet(child.Marks) == nil {
		if index == 0 {
			return 0, 0, false
		}
		index--
		child = parent.Child(index)
		if t.IsInSet(child.Marks) == nil {
			return 0, 0, false
		}
	}
	mark := t.IsInSet(child.Marks)
	start := r.Start(r.Depth)
	for i := range index {
		start += parent.Child(i).NodeSize()
	}
	end := start + child.NodeSize()
	for i := index - 1; i >= 0 && mark.IsInSet(parent.Child(i).Marks); i-- {
		start -= parent.Child(i).NodeSize()
	}
	for i := index + 1; i < parent.ChildCount() && mark.IsInSet(parent.Child(i).Marks); i++ {
		end += parent.Child(i).NodeSize()
	}
	return start, end, true
}

// UnsetMark removes t from the selection, the run around a cursor too.
func UnsetMark(t *MarkType) Command {
	return func(s *State, dispatch func(*Transaction)) bool {
		if dispatch != nil {
			tr := s.Tr()
			from, to := s.Selection.From(), s.Selection.To()
			if s.Selection.Empty() {
				if a, b, ok := MarkRange(s.Selection.FromPos(), t); ok {
					from, to = a, b
				}
			}
			tr.RemoveMark(from, to, nil, t)
			tr.RemoveStoredMark(t)
			dispatch(tr)
		}
		return true
	}
}

// UpdateAttributes sets attrs on the blocks of t in the selection.
func UpdateAttributes(t *NodeType, attrs Attrs) Command {
	return func(s *State, dispatch func(*Transaction)) bool {
		tr := s.Tr()
		changed := false
		for _, r := range s.Selection.Ranges() {
			s.Doc.NodesBetween(r.From.Pos, r.To.Pos, func(n *Node, pos int, _ *Node, _ int) bool {
				if n.Type == t {
					for k, v := range attrs {
						tr.SetNodeAttribute(pos, k, v)
						changed = true
					}
				}
				return true
			}, 0)
		}
		if changed && dispatch != nil {
			dispatch(tr)
		}
		return changed
	}
}

// DeleteList leaves a list with Backspace at an item's start: lifting it.
func listBackspace(item *NodeType) Command {
	return func(s *State, dispatch func(*Transaction)) bool {
		cursor := cursorAtStart(s)
		if cursor == nil || cursor.Depth < 2 || cursor.Node(-1).Type != item || cursor.Index(-1) != 0 {
			return false
		}
		return LiftListItem(item)(s, dispatch)
	}
}

func markExtension(name string, spec MarkSpec, keys []string, command string, rules func(*MarkType) []InputRule) Extension {
	spec.Name = name
	return Extension{Name: name, Marks: []MarkSpec{spec},
		Keys: func(s *Schema) map[string]Command {
			out := map[string]Command{}
			for _, k := range keys {
				out[k] = ToggleMark(s.Marks[name], nil)
			}
			return out
		},
		Rules: func(s *Schema) []InputRule {
			if rules == nil {
				return nil
			}
			return rules(s.Marks[name])
		},
		Commands: func(s *Schema) map[string]CommandFactory {
			t := s.Marks[name]
			return map[string]CommandFactory{
				"toggle" + command: func(a Attrs) Command { return ToggleMark(t, a) },
				"set" + command:    func(a Attrs) Command { return SetMark(t, a) },
				"unset" + command:  noArgs(UnsetMark(t)),
			}
		}}
}

func levelRe(pattern string) *regexp.Regexp { return regexp.MustCompile(pattern) }

// StarterOptions leave parts of StarterKit out, or tune them.
type StarterOptions struct {
	// Levels are the headings' levels, 1 to 6 when nil.
	Levels                                              []int
	History                                             HistoryOptions
	NoHistory, NoTrailingNode, NoTaskList, NoTypography bool
	Link                                                LinkOptions
}

// StarterKit is a rich text editor's usual types with their commands, keys
// and rules, by Tiptap's names.
func StarterKit(o StarterOptions) []Extension {
	levels := o.Levels
	if levels == nil {
		levels = []int{1, 2, 3, 4, 5, 6}
	}
	listRoles := func(name, content, group, marker string, attrs []AttrSpec) NodeSpec {
		return NodeSpec{Name: name, Content: content, Group: group, Attrs: attrs, Extra: listExtra(marker)}
	}
	out := []Extension{
		{Name: "document", Nodes: []NodeSpec{{Name: "doc", Content: "block+"}},
			Keys: func(*Schema) map[string]Command { return nil },
			Commands: func(s *Schema) map[string]CommandFactory {
				return map[string]CommandFactory{"selectAll": noArgs(SelectAll), "deleteSelection": noArgs(DeleteSelection), "liftEmptyBlock": noArgs(LiftEmptyBlock),
					"splitBlock": noArgs(SplitBlock), "lift": noArgs(Lift), "joinBackward": noArgs(JoinBackward), "joinForward": noArgs(JoinForward)}
			}},
		{Name: "paragraph", Nodes: []NodeSpec{{Name: "paragraph", Content: "inline*", Group: "block"}},
			Keys: func(s *Schema) map[string]Command {
				return bind("Mod-Alt-0", SetBlockType(s.Nodes["paragraph"], nil))
			},
			Commands: func(s *Schema) map[string]CommandFactory {
				return map[string]CommandFactory{"setParagraph": noArgs(SetBlockType(s.Nodes["paragraph"], nil))}
			}},
		{Name: "heading", Nodes: []NodeSpec{{Name: "heading", Content: "inline*", Group: "block", Defining: true,
			Attrs: []AttrSpec{{Name: "level", Default: 1, HasDefault: true}}}},
			Keys: func(s *Schema) map[string]Command {
				out := map[string]Command{}
				for _, l := range levels {
					out[fmt.Sprintf("Mod-Alt-%d", l)] = ToggleBlockType(s.Nodes["heading"], Attrs{"level": l})
				}
				return out
			},
			Rules: func(s *Schema) []InputRule {
				top := slices.Max(levels)
				return []InputRule{TextblockTypeInputRule(levelRe(fmt.Sprintf(`^(#{1,%d})\s$`, top)), s.Nodes["heading"], func(m []string) Attrs { return Attrs{"level": len(m[1])} })}
			},
			Commands: func(s *Schema) map[string]CommandFactory {
				t := s.Nodes["heading"]
				return map[string]CommandFactory{"setHeading": func(a Attrs) Command { return SetBlockType(t, a) },
					"toggleHeading": func(a Attrs) Command { return ToggleBlockType(t, a) }}
			}},
		{Name: "blockquote", Nodes: []NodeSpec{{Name: "blockquote", Content: "block+", Group: "block", Defining: true}},
			Keys: func(s *Schema) map[string]Command { return bind("Mod-Shift-b", ToggleWrap(s.Nodes["blockquote"], nil)) },
			Rules: func(s *Schema) []InputRule {
				return []InputRule{WrappingInputRule(levelRe(`^\s*>\s$`), s.Nodes["blockquote"], nil, nil)}
			},
			Commands: func(s *Schema) map[string]CommandFactory {
				return map[string]CommandFactory{"toggleBlockquote": noArgs(ToggleWrap(s.Nodes["blockquote"], nil))}
			}},
		{Name: "bulletList", Nodes: []NodeSpec{listRoles("bulletList", "listItem+", "block list", "bullet", nil)},
			Keys: func(s *Schema) map[string]Command {
				return bind("Mod-Shift-8", ToggleList(s.Nodes["bulletList"], s.Nodes["listItem"]))
			},
			Rules: func(s *Schema) []InputRule {
				return []InputRule{WrappingInputRule(levelRe(`^\s*([-+*])\s$`), s.Nodes["bulletList"], nil, nil)}
			},
			Commands: func(s *Schema) map[string]CommandFactory {
				return map[string]CommandFactory{"toggleBulletList": noArgs(ToggleList(s.Nodes["bulletList"], s.Nodes["listItem"]))}
			}},
		{Name: "orderedList", Nodes: []NodeSpec{listRoles("orderedList", "listItem+", "block list", "ordered", []AttrSpec{{Name: "start", Default: 1, HasDefault: true}})},
			Keys: func(s *Schema) map[string]Command {
				return bind("Mod-Shift-7", ToggleList(s.Nodes["orderedList"], s.Nodes["listItem"]))
			},
			Rules: func(s *Schema) []InputRule {
				t := s.Nodes["orderedList"]
				return []InputRule{WrappingInputRule(levelRe(`^(\d+)\.\s$`), t, func(m []string) Attrs { return Attrs{"start": atoi(m[1])} },
					func(m []string, n *Node) bool { return n.ChildCount()+int(n.AttrNum("start")) == atoi(m[1]) })}
			},
			Commands: func(s *Schema) map[string]CommandFactory {
				return map[string]CommandFactory{"toggleOrderedList": noArgs(ToggleList(s.Nodes["orderedList"], s.Nodes["listItem"]))}
			}},
		{Name: "listItem", Nodes: []NodeSpec{{Name: "listItem", Content: "paragraph block*", Defining: true}},
			Keys: func(s *Schema) map[string]Command {
				item := s.Nodes["listItem"]
				return map[string]Command{"Enter": SplitListItem(item, nil), "Tab": SinkListItem(item), "Shift-Tab": LiftListItem(item), "Backspace": listBackspace(item)}
			},
			Commands: func(s *Schema) map[string]CommandFactory {
				item := s.Nodes["listItem"]
				return map[string]CommandFactory{"sinkListItem": noArgs(SinkListItem(item)), "liftListItem": noArgs(LiftListItem(item)), "splitListItem": noArgs(SplitListItem(item, nil))}
			}},
	}
	if !o.NoTaskList {
		out = append(out, Extension{Name: "taskList", Nodes: []NodeSpec{{Name: "taskList", Content: "taskItem+", Group: "block list", Extra: map[string]any{"list": true}}},
			Keys: func(s *Schema) map[string]Command {
				return bind("Mod-Shift-9", ToggleList(s.Nodes["taskList"], s.Nodes["taskItem"]))
			},
			Commands: func(s *Schema) map[string]CommandFactory {
				return map[string]CommandFactory{"toggleTaskList": noArgs(ToggleList(s.Nodes["taskList"], s.Nodes["taskItem"]))}
			}},
			Extension{Name: "taskItem", Nodes: []NodeSpec{{Name: "taskItem", Content: "paragraph block*", Defining: true, Attrs: []AttrSpec{{Name: "checked", Default: false, HasDefault: true}}}},
				Keys: func(s *Schema) map[string]Command {
					item := s.Nodes["taskItem"]
					return map[string]Command{"Enter": SplitListItem(item, Attrs{"checked": false}), "Tab": SinkListItem(item), "Shift-Tab": LiftListItem(item), "Backspace": listBackspace(item)}
				},
				Rules: func(s *Schema) []InputRule {
					t := s.Nodes["taskList"]
					return []InputRule{WrappingInputRule(levelRe(`^\s*\[([ xX]?)\]\s$`), t, nil, nil)}
				}})
	}
	out = append(out,
		Extension{Name: "codeBlock", Nodes: []NodeSpec{{Name: "codeBlock", Content: "text*", Group: "block", Code: true, Defining: true, Marks: "none", Whitespace: "pre",
			Attrs: []AttrSpec{{Name: "language", HasDefault: true}}}},
			Keys: func(s *Schema) map[string]Command {
				return map[string]Command{"Mod-Alt-c": ToggleBlockType(s.Nodes["codeBlock"], nil), "Backspace": codeBlockBackspace(s.Nodes["codeBlock"]), "Enter": codeBlockExit(s.Nodes["codeBlock"])}
			},
			Rules: func(s *Schema) []InputRule {
				return []InputRule{TextblockTypeInputRule(levelRe("^```([a-z]+)?[\\s\\n]$"), s.Nodes["codeBlock"], func(m []string) Attrs {
					if m[1] == "" {
						return nil
					}
					return Attrs{"language": m[1]}
				})}
			},
			Commands: func(s *Schema) map[string]CommandFactory {
				t := s.Nodes["codeBlock"]
				return map[string]CommandFactory{"toggleCodeBlock": func(a Attrs) Command { return ToggleBlockType(t, a) }}
			}},
		Extension{Name: "text", Nodes: []NodeSpec{{Name: "text", Group: "inline"}}},
		Extension{Name: "hardBreak", Nodes: []NodeSpec{{Name: "hardBreak", Inline: true, Group: "inline", NoSelect: true, LeafText: func(*Node) string { return "\n" }}},
			Keys: func(s *Schema) map[string]Command {
				c := ChainCommands(ExitCode, InsertHardBreak(s.Nodes["hardBreak"]))
				return map[string]Command{"Mod-Enter": c, "Shift-Enter": c}
			},
			Commands: func(s *Schema) map[string]CommandFactory {
				return map[string]CommandFactory{"setHardBreak": noArgs(InsertHardBreak(s.Nodes["hardBreak"]))}
			}},
		Extension{Name: "horizontalRule", Nodes: []NodeSpec{{Name: "horizontalRule", Group: "block"}},
			Rules: func(s *Schema) []InputRule {
				t := s.Nodes["horizontalRule"]
				return []InputRule{{Match: levelRe(`^(?:---|—-|___\s|\*\*\*\s)$`), Handler: func(st *State, _ []string, start, end int) *Transaction {
					return insertBlock(st, t, start, end)
				}}}
			},
			Commands: func(s *Schema) map[string]CommandFactory {
				t := s.Nodes["horizontalRule"]
				return map[string]CommandFactory{"setHorizontalRule": noArgs(func(st *State, dispatch func(*Transaction)) bool {
					if dispatch != nil {
						dispatch(insertBlock(st, t, st.Selection.From(), st.Selection.To()).ScrollIntoView())
					}
					return true
				})}
			}},
		Extension{Name: "image", Nodes: []NodeSpec{{Name: "image", Group: "block", Attrs: []AttrSpec{{Name: "src"}, {Name: "alt", HasDefault: true}, {Name: "title", HasDefault: true}}}},
			Commands: func(s *Schema) map[string]CommandFactory {
				t := s.Nodes["image"]
				return map[string]CommandFactory{"setImage": func(a Attrs) Command {
					return func(st *State, dispatch func(*Transaction)) bool {
						if dispatch != nil {
							dispatch(st.Tr().ReplaceSelectionWith(t.Create(a, nil, nil), false).ScrollIntoView())
						}
						return true
					}
				}}
			}},
		markExtension("bold", MarkSpec{}, []string{"Mod-b", "Mod-B"}, "Bold", func(t *MarkType) []InputRule {
			return []InputRule{MarkInputRule(levelRe(`(^|\s)(?:\*\*|__)([^*_\s](?:[^*_]*[^*_\s])?)(?:\*\*|__)$`), t, nil)}
		}),
		markExtension("italic", MarkSpec{}, []string{"Mod-i", "Mod-I"}, "Italic", func(t *MarkType) []InputRule {
			return []InputRule{MarkInputRule(levelRe(`(^|\s)[*_]([^*_\s](?:[^*_]*[^*_\s])?)[*_]$`), t, nil)}
		}),
		markExtension("strike", MarkSpec{}, []string{"Mod-Shift-s"}, "Strike", func(t *MarkType) []InputRule {
			return []InputRule{MarkInputRule(levelRe(`(^|\s)~~([^~\s](?:[^~]*[^~\s])?)~~$`), t, nil)}
		}),
		markExtension("code", MarkSpec{Excludes: "_"}, []string{"Mod-e"}, "Code", func(t *MarkType) []InputRule {
			return []InputRule{MarkInputRule(levelRe("(^|\\s)`([^`]+)`$"), t, nil)}
		}),
		markExtension("underline", MarkSpec{}, []string{"Mod-u", "Mod-U"}, "Underline", nil),
		markExtension("highlight", MarkSpec{}, []string{"Mod-Shift-h"}, "Highlight", func(t *MarkType) []InputRule {
			return []InputRule{MarkInputRule(levelRe(`(^|\s)==([^=\s](?:[^=]*[^=\s])?)==$`), t, nil)}
		}),
		markExtension("subscript", MarkSpec{Excludes: "superscript"}, []string{"Mod-,"}, "Subscript", nil),
		markExtension("superscript", MarkSpec{Excludes: "subscript"}, []string{"Mod-."}, "Superscript", nil),
		Link(o.Link),
		Extension{Name: "textAlign", Global: []GlobalAttrs{{Types: []string{"heading", "paragraph"}, Attrs: []AttrSpec{{Name: "textAlign", HasDefault: true}}}},
			Keys: func(s *Schema) map[string]Command {
				set := func(a string) Command { return setTextAlign(s, a) }
				return map[string]Command{"Mod-Shift-l": set("left"), "Mod-Shift-e": set("center"), "Mod-Shift-r": set("right"), "Mod-Shift-j": set("justify")}
			},
			Commands: func(s *Schema) map[string]CommandFactory {
				return map[string]CommandFactory{"setTextAlign": func(a Attrs) Command {
					v, _ := a["alignment"].(string)
					return setTextAlign(s, v)
				}}
			}},
	)
	if !o.NoHistory {
		out = append(out, Extension{Name: "history", Plugins: func(*Schema) []*Plugin { return []*Plugin{History(o.History)} },
			Keys: func(*Schema) map[string]Command {
				return map[string]Command{"Mod-z": Undo, "Shift-Mod-z": Redo, "Mod-y": Redo}
			},
			Commands: func(*Schema) map[string]CommandFactory {
				return map[string]CommandFactory{"undo": noArgs(Undo), "redo": noArgs(Redo)}
			}})
	}
	if !o.NoTrailingNode {
		out = append(out, Extension{Name: "trailingNode", Plugins: func(s *Schema) []*Plugin { return []*Plugin{trailingNode(s)} }})
	}
	if !o.NoTypography {
		out = append(out, Extension{Name: "typography", Rules: func(*Schema) []InputRule { return typographyRules() }})
	}
	return out
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		n = n*10 + int(c-'0')
	}
	return n
}

func insertBlock(s *State, t *NodeType, start, end int) *Transaction {
	tr := s.Tr()
	r := s.Doc.Resolve(start)
	node := t.Create(nil, nil, nil)
	if r.ParentOffset == 0 && r.Depth > 0 && end == r.End(r.Depth) {
		before := r.Before(r.Depth)
		tr.ReplaceRangeWith(before, r.After(r.Depth), node)
	} else {
		tr.ReplaceRangeWith(start, end, node)
	}
	return tr
}

func setTextAlign(s *Schema, align string) Command {
	return func(st *State, dispatch func(*Transaction)) bool {
		tr := st.Tr()
		changed := false
		for _, r := range st.Selection.Ranges() {
			st.Doc.NodesBetween(r.From.Pos, r.To.Pos, func(n *Node, pos int, _ *Node, _ int) bool {
				if _, ok := n.Attrs["textAlign"]; ok {
					var v any
					if align != "" {
						v = align
					}
					tr.SetNodeAttribute(pos, "textAlign", v)
					changed = true
				}
				return true
			}, 0)
		}
		if changed && dispatch != nil {
			dispatch(tr)
		}
		return changed
	}
}

func codeBlockBackspace(t *NodeType) Command {
	return func(s *State, dispatch func(*Transaction)) bool {
		cursor := cursorAtStart(s)
		if cursor == nil || cursor.Parent().Type != t || cursor.Parent().Content.Size != 0 {
			return false
		}
		return SetBlockType(s.Schema.DefaultBlock(), nil)(s, dispatch)
	}
}

// codeBlockExit leaves a code block with Enter after two empty lines.
func codeBlockExit(t *NodeType) Command {
	return func(s *State, dispatch func(*Transaction)) bool {
		cursor := cursorAtEnd(s)
		if cursor == nil || cursor.Parent().Type != t || !strings.HasSuffix(cursor.Parent().TextContent(), "\n\n") {
			return false
		}
		if dispatch != nil {
			tr := s.Tr().Delete(cursor.Pos-2, cursor.Pos)
			pos := tr.Mapping.Map(cursor.After(cursor.Depth), 1)
			tr.Insert(pos, s.Schema.DefaultBlock().CreateAndFill(nil, nil, nil))
			tr.SetSelection(SelectionNear(tr.Doc.Resolve(pos+1), 1))
			dispatch(tr.ScrollIntoView())
		}
		return true
	}
}

func trailingNode(s *Schema) *Plugin {
	node := s.DefaultBlock()
	return &Plugin{Key: "trailingNode", AppendTransaction: func(_ []*Transaction, _, n *State) *Transaction {
		last := n.Doc.LastChild()
		if last == nil || last.Type == node {
			return nil
		}
		return n.Tr().Insert(n.Doc.Content.Size, node.CreateAndFill(nil, nil, nil)).SetMeta(addToHistory, false)
	}}
}

func typographyRules() []InputRule {
	pairs := [][2]string{{`--$`, "—"}, {`\.\.\.$`, "…"}, {`<-$`, "←"}, {`->$`, "→"}, {`\(c\)$`, "©"}, {`\(r\)$`, "®"}, {`\(tm\)$`, "™"}, {`!=$`, "≠"}, {` 1/2 $`, " ½ "}, {` 1/4 $`, " ¼ "}, {` 3/4 $`, " ¾ "}, {`\+/-$`, "±"}}
	out := make([]InputRule, len(pairs))
	for i, p := range pairs {
		out[i] = TextInputRule(regexp.MustCompile(p[0]), p[1])
	}
	return out
}

// Table are a table's Tiptap options.
type TableOptions struct {
	CellMinWidth float64
	// NotResizable keeps tables from resizing by their handles.
	NotResizable, LastColumnNotResizable bool
}

// TableKit is tables with their commands and keys, outside StarterKit.
func TableKit(o TableOptions) Extension {
	if o.CellMinWidth == 0 {
		o.CellMinWidth = 25
	}
	cell := []AttrSpec{{Name: "colspan", Default: 1, HasDefault: true}, {Name: "rowspan", Default: 1, HasDefault: true}, {Name: "colwidth", HasDefault: true}}
	return Extension{Name: "table",
		Nodes: []NodeSpec{
			{Name: "table", Content: "tableRow+", Group: "block", Isolating: true, Extra: map[string]any{"tableRole": "table", "options": o}},
			{Name: "tableRow", Content: "(tableCell | tableHeader)*", Attrs: []AttrSpec{{Name: "height", HasDefault: true}}, Extra: map[string]any{"tableRole": "row"}},
			{Name: "tableCell", Content: "block+", Isolating: true, Attrs: cell, Extra: map[string]any{"tableRole": "cell"}},
			{Name: "tableHeader", Content: "block+", Isolating: true, Attrs: cell, Extra: map[string]any{"tableRole": "header_cell"}},
		},
		Keys: func(s *Schema) map[string]Command {
			return map[string]Command{"Tab": GoToNextCellOrAddRow, "Shift-Tab": GoToNextCell(-1)}
		},
		Plugins: func(*Schema) []*Plugin { return []*Plugin{FixTables(), TableEditing()} },
		Commands: func(*Schema) map[string]CommandFactory {
			return map[string]CommandFactory{
				"insertTable": func(a Attrs) Command {
					rows, cols := int(num(a["rows"])), int(num(a["cols"]))
					header, _ := a["withHeaderRow"].(bool)
					return InsertTable(max(rows, 1), max(cols, 1), header)
				},
				"addColumnBefore": noArgs(AddColumnBefore), "addColumnAfter": noArgs(AddColumnAfter), "deleteColumn": noArgs(DeleteColumn),
				"addRowBefore": noArgs(AddRowBefore), "addRowAfter": noArgs(AddRowAfter), "deleteRow": noArgs(DeleteRow), "deleteTable": noArgs(DeleteTable),
				"mergeCells": noArgs(MergeCells), "splitCell": noArgs(SplitCell),
				"mergeOrSplit":    noArgs(ChainCommands(MergeCells, SplitCell)),
				"toggleHeaderRow": noArgs(ToggleHeaderRow), "toggleHeaderColumn": noArgs(ToggleHeaderColumn), "toggleHeaderCell": noArgs(ToggleHeaderCell),
				"goToNextCell": noArgs(GoToNextCell(1)), "goToPreviousCell": noArgs(GoToNextCell(-1)),
			}
		}}
}
