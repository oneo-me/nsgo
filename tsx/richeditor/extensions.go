package richeditor

import (
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"time"

	"github.com/nsgo-dev/nsgo/base/richeditor"
	"github.com/nsgo-dev/nsgo/tsx"
)

func typeError(format string, args ...any) error {
	return &tsx.JSError{Name: "TypeError", Message: fmt.Sprintf(format, args...)}
}

func rangeError(format string, args ...any) error {
	return &tsx.JSError{Name: "RangeError", Message: fmt.Sprintf(format, args...)}
}

// Extension is RichExtension: Tiptap's extension, a built-in one or the
// app's, made into the kernel's as createRichEditor builds the editor.
type Extension struct {
	name    string
	options map[string]any
	make    func(e *Editor, options map[string]any) ([]richeditor.Extension, error)
}

// Name is its name, unique in an editor.
func (x *Extension) Name() string { return x.name }

// Options are its options, defaults under what was configured.
func (x *Extension) Options() map[string]any { return maps.Clone(x.options) }

// Configure is a copy with options merged deeply over its own, as
// Tiptap's mergeDeep.
func (x *Extension) Configure(options map[string]any) *Extension {
	return &Extension{name: x.name, options: mergeDeep(x.options, options), make: x.make}
}

func mergeDeep(base, over map[string]any) map[string]any {
	out := maps.Clone(base)
	if out == nil {
		out = map[string]any{}
	}
	for k, v := range over {
		if m, ok := v.(map[string]any); ok {
			if b, ok := out[k].(map[string]any); ok {
				out[k] = mergeDeep(b, m)
				continue
			}
		}
		out[k] = v
	}
	return out
}

// off says an option is false, as {history: false} leaves an extension out.
func off(o map[string]any, key string) bool {
	v, ok := o[key].(bool)
	return ok && !v
}

func sub(o map[string]any, key string) map[string]any {
	m, _ := o[key].(map[string]any)
	return m
}

func number(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int64:
		return float64(n)
	case int:
		return float64(n)
	}
	return 0
}

func stringList(v any) []string {
	list, _ := v.([]any)
	var out []string
	for _, s := range list {
		if t, ok := s.(string); ok {
			out = append(out, t)
		}
	}
	return out
}

// StarterKit is the kit of Tiptap's usual types: levels for headings,
// false for history, trailingNode, taskList, typography or gapcursor to
// leave one out, history's depth and newGroupDelay, and link's autolink,
// openOnClick and protocols.
func StarterKit(options map[string]any) *Extension {
	return &Extension{name: "starterKit", options: mergeDeep(nil, options), make: func(e *Editor, o map[string]any) ([]richeditor.Extension, error) {
		so := richeditor.StarterOptions{NoHistory: off(o, "history"), NoTrailingNode: off(o, "trailingNode"), NoTaskList: off(o, "taskList"),
			NoTypography: off(o, "typography"), NoGapCursor: off(o, "gapcursor")}
		if raw, ok := o["levels"].([]any); ok {
			for _, l := range raw {
				so.Levels = append(so.Levels, int(number(l)))
			}
		}
		if h := sub(o, "history"); h != nil {
			so.History = richeditor.HistoryOptions{Depth: int(number(h["depth"])), NewGroupDelay: time.Duration(number(h["newGroupDelay"]) * float64(time.Millisecond))}
		}
		if l := sub(o, "link"); l != nil {
			so.Link = richeditor.LinkOptions{NoAutolink: off(l, "autolink"), NoOpenOnClick: off(l, "openOnClick"), Protocols: stringList(l["protocols"])}
		}
		so.Link.Open = func(href string) { e.openLink(href) }
		return richeditor.StarterKit(so), nil
	}}
}

// TableKit is tables: cellMinWidth, and false for resizable or
// lastColumnResizable.
func TableKit(options map[string]any) *Extension {
	return &Extension{name: "tableKit", options: mergeDeep(nil, options), make: func(_ *Editor, o map[string]any) ([]richeditor.Extension, error) {
		return []richeditor.Extension{richeditor.TableKit(richeditor.TableOptions{CellMinWidth: number(o["cellMinWidth"]),
			NotResizable: off(o, "resizable"), LastColumnNotResizable: off(o, "lastColumnResizable")})}, nil
	}}
}

// Mention is Tiptap's mention and its suggestion: char, key, allowSpaces,
// class and the captureKeys its list takes.
func Mention(options map[string]any) *Extension {
	return &Extension{name: "mention", options: mergeDeep(nil, options), make: func(_ *Editor, o map[string]any) ([]richeditor.Extension, error) {
		s := richeditor.SuggestionOptions{CaptureKeys: stringList(o["captureKeys"])}
		s.Char, _ = o["char"].(string)
		s.Key, _ = o["key"].(string)
		s.Class, _ = o["class"].(string)
		s.AllowSpaces, _ = o["allowSpaces"].(bool)
		return []richeditor.Extension{richeditor.Mention(richeditor.MentionOptions{Suggestion: s})}, nil
	}}
}

// CharacterCount limits the characters to limit, counting every position
// with mode "nodes".
func CharacterCount(options map[string]any) *Extension {
	return &Extension{name: "characterCount", options: mergeDeep(nil, options), make: func(_ *Editor, o map[string]any) ([]richeditor.Extension, error) {
		return []richeditor.Extension{richeditor.CharacterCount(richeditor.CharacterCountOptions{Limit: int(number(o["limit"])), CountNodes: o["mode"] == "nodes"})}, nil
	}}
}

// RichExtensionSpec is defineExtension's, Tiptap's extension config: a node or
// a mark named after it, and functions of its options run once as an
// editor is made, returning data.
type RichExtensionSpec struct {
	Name     string
	Priority int
	Node     *RichNodeSpec
	Mark     *RichMarkSpec
	// AddOptions gives the defaults under what configure sets.
	AddOptions            func() map[string]any
	AddGlobalAttributes   func(options map[string]any) []RichGlobalAttributes
	AddKeyboardShortcuts  func(options map[string]any) map[string]*Command
	AddCommands           func(options map[string]any) map[string]func(args map[string]any) *Command
	AddInputRules         func(options map[string]any) []RichInputRule
	AddProseMirrorPlugins func(options map[string]any) []RichPluginSpec
}

// RichNodeSpec is a node type: ProseMirror's spec, attrs by name with their
// defaults ({ level: { default: 1 } }), and what it is as text.
type RichNodeSpec struct {
	Content, Marks, Group string
	Inline, Atom, Code    bool
	Defining, Isolating   bool
	// Selectable false keeps node selections off it.
	Selectable *bool
	Whitespace string
	Attrs      map[string]map[string]any
	// LeafText is a leaf as text, RenderText what the view shows for an
	// inline leaf, of its attributes.
	LeafText, RenderText func(attrs map[string]any) string
	Extra                map[string]any
}

// RichMarkSpec is a mark type.
type RichMarkSpec struct {
	Attrs map[string]map[string]any
	// Inclusive false keeps text typed at its end out of it.
	Inclusive       *bool
	Excludes, Group string
	Extra           map[string]any
}

// RichGlobalAttributes add attributes to other types.
type RichGlobalAttributes struct {
	Types      []string
	Attributes map[string]map[string]any
}

// RichInputRule changes text as it is typed: Find, ending at the cursor
// ("$"), becomes a mark ("mark", its last group the text), a textblock
// ("textblock"), wraps the block ("wrapping") or is replaced by Replace
// ("text"). Name is the type, the extension's own when "".
type RichInputRule struct {
	Find    *regexp.Regexp
	Type    string
	Name    string
	Replace string
	Attrs   map[string]any
}

func attrSpecs(attrs map[string]map[string]any) []richeditor.AttrSpec {
	names := slices.Sorted(maps.Keys(attrs))
	out := make([]richeditor.AttrSpec, len(names))
	for i, n := range names {
		def, has := attrs[n]["default"]
		out[i] = richeditor.AttrSpec{Name: n, Default: def, HasDefault: has}
	}
	return out
}

func textOf(f func(map[string]any) string) func(*richeditor.Node) string {
	if f == nil {
		return nil
	}
	return func(n *richeditor.Node) string { return f(maps.Clone(n.Attrs)) }
}

// defineExtension declares the app's extension.
func defineExtension(spec RichExtensionSpec) (*Extension, error) {
	if spec.Name == "" {
		return nil, typeError("defineExtension: an extension needs a name")
	}
	if spec.Node != nil && spec.Mark != nil {
		return nil, typeError("defineExtension: %s is a node or a mark, not both", spec.Name)
	}
	var defaults map[string]any
	if spec.AddOptions != nil {
		defaults = spec.AddOptions()
	}
	return &Extension{name: spec.Name, options: mergeDeep(nil, defaults), make: func(e *Editor, o map[string]any) ([]richeditor.Extension, error) {
		return spec.extension(e, o)
	}}, nil
}

func (spec RichExtensionSpec) extension(e *Editor, o map[string]any) ([]richeditor.Extension, error) {
	x := richeditor.Extension{Name: spec.Name, Priority: spec.Priority}
	if n := spec.Node; n != nil {
		x.Nodes = []richeditor.NodeSpec{{Name: spec.Name, Content: n.Content, Marks: n.Marks, Group: n.Group, Inline: n.Inline, Atom: n.Atom,
			NoSelect: n.Selectable != nil && !*n.Selectable, Code: n.Code, Defining: n.Defining, Isolating: n.Isolating, Whitespace: n.Whitespace,
			Attrs: attrSpecs(n.Attrs), LeafText: textOf(n.LeafText), RenderText: textOf(n.RenderText), Extra: n.Extra}}
	}
	if m := spec.Mark; m != nil {
		x.Marks = []richeditor.MarkSpec{{Name: spec.Name, Attrs: attrSpecs(m.Attrs), NotInclusive: m.Inclusive != nil && !*m.Inclusive,
			Excludes: m.Excludes, Group: m.Group, Extra: m.Extra}}
	}
	if spec.AddGlobalAttributes != nil {
		for _, g := range spec.AddGlobalAttributes(o) {
			x.Global = append(x.Global, richeditor.GlobalAttrs{Types: g.Types, Attrs: attrSpecs(g.Attributes)})
		}
	}
	if spec.AddKeyboardShortcuts != nil {
		keys := spec.AddKeyboardShortcuts(o)
		for k, c := range keys {
			if c == nil {
				return nil, typeError("%s: the key %s has no command", spec.Name, k)
			}
			e.check = append(e.check, checked{spec.Name + ": " + k, c})
		}
		x.Keys = func(*richeditor.Schema) map[string]richeditor.Command {
			out := make(map[string]richeditor.Command, len(keys))
			for k, c := range keys {
				out[k] = e.late(c)
			}
			return out
		}
	}
	if spec.AddCommands != nil {
		cmds := spec.AddCommands(o)
		x.Commands = func(*richeditor.Schema) map[string]richeditor.CommandFactory {
			out := make(map[string]richeditor.CommandFactory, len(cmds))
			for name, f := range cmds {
				if f == nil {
					continue
				}
				out[name] = func(args richeditor.Attrs) richeditor.Command {
					c := f(args)
					if c == nil {
						return func(*richeditor.State, func(*richeditor.Transaction)) bool { return false }
					}
					return e.late(c)
				}
			}
			return out
		}
	}
	if spec.AddInputRules != nil {
		rules := spec.AddInputRules(o)
		for i, r := range rules {
			if r.Find == nil {
				return nil, typeError("%s: input rule %d has no pattern", spec.Name, i)
			}
		}
		x.Rules = func(s *richeditor.Schema) []richeditor.InputRule {
			var out []richeditor.InputRule
			for _, r := range rules {
				if rule, err := r.rule(s, spec.Name); err == nil {
					out = append(out, rule)
				} else {
					e.problems = append(e.problems, fmt.Sprintf("%s: %v", spec.Name, err))
				}
			}
			return out
		}
	}
	if spec.AddProseMirrorPlugins != nil {
		specs := spec.AddProseMirrorPlugins(o)
		x.Plugins = func(*richeditor.Schema) []*richeditor.Plugin {
			out := make([]*richeditor.Plugin, len(specs))
			for i, p := range specs {
				out[i] = p.plugin(e, fmt.Sprintf("%s%d", spec.Name, i))
			}
			return out
		}
	}
	return []richeditor.Extension{x}, nil
}

func (r RichInputRule) rule(s *richeditor.Schema, own string) (richeditor.InputRule, error) {
	name := r.Name
	if name == "" {
		name = own
	}
	attrs := func([]string) richeditor.Attrs { return r.Attrs }
	switch r.Type {
	case "text":
		return richeditor.TextInputRule(r.Find, r.Replace), nil
	case "mark":
		if t := s.Marks[name]; t != nil {
			return richeditor.MarkInputRule(r.Find, t, attrs), nil
		}
		return richeditor.InputRule{}, fmt.Errorf("no mark type %q", name)
	case "textblock", "wrapping":
		t := s.Nodes[name]
		if t == nil {
			return richeditor.InputRule{}, fmt.Errorf("no node type %q", name)
		}
		if r.Type == "textblock" {
			return richeditor.TextblockTypeInputRule(r.Find, t, attrs), nil
		}
		return richeditor.WrappingInputRule(r.Find, t, attrs, nil), nil
	}
	return richeditor.InputRule{}, errors.New("an input rule's type is mark, textblock, wrapping or text")
}

// checked is a command createRichEditor checks once its schema is known.
type checked struct {
	where string
	c     *Command
}

// late is c resolved as it runs, when the editor has its session.
func (e *Editor) late(c *Command) richeditor.Command {
	return func(s *richeditor.State, dispatch func(*richeditor.Transaction)) bool {
		r, err := c.resolve(e)
		return err == nil && r(s, dispatch)
	}
}
