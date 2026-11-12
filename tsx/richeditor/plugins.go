package richeditor

import (
	"github.com/nsgo-dev/nsgo/base/richeditor"
	"github.com/nsgo-dev/nsgo/core"
)

// RichPluginSpec is a plugin whose behavior is the app's: a keymap and hooks
// the editor calls with itself, asked in plugin order, the first true
// taking the event.
type RichPluginSpec struct {
	// Key names it among the editor's plugins.
	Key string
	// Keys bind keys by ProseMirror's names ("Mod-k") to commands.
	Keys            map[string]*Command
	HandleKeyDown   func(editor *Editor, key string) bool
	HandleTextInput func(editor *Editor, event RichTextInputEvent) bool
	HandleClick     func(editor *Editor, event RichClickEvent) bool
	HandlePaste     func(editor *Editor, event RichPasteEvent) bool
	// Decorations are what the view draws over the editor's state.
	Decorations func(editor *Editor) []RichDecoration
	// FilterTransaction refuses a transaction before it applies, the
	// editor's state still the one before it.
	FilterTransaction func(editor *Editor, tr RichTransactionInfo) bool
	// AppendTransaction may answer transactions that applied with a
	// command, run on the state they left.
	AppendTransaction func(editor *Editor, tr RichTransactionInfo) *Command
}

// RichTextInputEvent is text typed over from to to.
type RichTextInputEvent struct {
	From, To int
	Text     string
}

// RichClickEvent is a press at a position, Clicks in a row, Mod held or not.
type RichClickEvent struct {
	Pos, Clicks int
	Mod         bool
}

// RichPasteEvent is what is pasted, as text.
type RichPasteEvent struct{ Text string }

// RichTransactionInfo is what JS reads of a transaction.
type RichTransactionInfo struct {
	DocChanged, SelectionSet bool
	Origin                   string
	Steps                    int
}

func infoOf(tr *richeditor.Transaction) RichTransactionInfo {
	return RichTransactionInfo{DocChanged: tr.DocChanged(), SelectionSet: tr.SelectionSet(), Origin: originName(tr.Origin), Steps: len(tr.Steps)}
}

// RichDecoration styles a range without changing the document: Class on its
// text, or on the block at From with Node; with Widget, a view in the text
// at From, before a cursor there when Side is below zero, kept while Key
// stays.
type RichDecoration struct {
	From, To int
	Class    string
	Node     bool
	Widget   func() core.Node
	Side     int
	Key      string
}

func (p RichPluginSpec) plugin(e *Editor, key string) *richeditor.Plugin {
	if p.Key != "" {
		key = p.Key
	}
	out := &richeditor.Plugin{Key: key}
	var keymap *richeditor.Plugin
	if len(p.Keys) > 0 {
		bindings := make(map[string]richeditor.Command, len(p.Keys))
		for k, c := range p.Keys {
			if c != nil {
				bindings[k] = e.late(c)
				e.check = append(e.check, checked{key + ": " + k, c})
			}
		}
		keymap = richeditor.Keymap(key+"Keys", bindings)
	}
	if keymap != nil || p.HandleKeyDown != nil {
		out.HandleKeyDown = func(_ *richeditor.Session, name string) bool {
			if keymap != nil && keymap.HandleKeyDown(e.s, name) {
				return true
			}
			return p.HandleKeyDown != nil && p.HandleKeyDown(e, name)
		}
	}
	if f := p.HandleTextInput; f != nil {
		out.HandleTextInput = func(_ *richeditor.Session, from, to int, text string) bool {
			return f(e, RichTextInputEvent{from, to, text})
		}
	}
	if f := p.HandleClick; f != nil {
		out.HandleClick = func(_ *richeditor.Session, pos, clicks int, mod bool) bool {
			return f(e, RichClickEvent{pos, clicks, mod})
		}
	}
	if f := p.HandlePaste; f != nil {
		out.HandlePaste = func(_ *richeditor.Session, slice *richeditor.Slice) bool {
			return f(e, RichPasteEvent{richeditor.SliceToText(slice)})
		}
	}
	if f := p.Decorations; f != nil {
		out.Decorations = func(*richeditor.State) []richeditor.Decoration {
			list := f(e)
			decos := make([]richeditor.Decoration, len(list))
			for i, d := range list {
				decos[i] = richeditor.Decoration{From: d.From, To: d.To, Class: d.Class, Node: d.Node, Side: d.Side, Key: d.Key}
				if w := d.Widget; w != nil {
					decos[i].Widget = func(*core.Scope) core.Node { return w() }
				}
			}
			return decos
		}
	}
	if f := p.FilterTransaction; f != nil {
		out.FilterTransaction = func(tr *richeditor.Transaction, _ *richeditor.State) bool { return f(e, infoOf(tr)) }
	}
	if f := p.AppendTransaction; f != nil {
		out.AppendTransaction = func(trs []*richeditor.Transaction, _, n *richeditor.State) *richeditor.Transaction {
			c := f(e, infoOf(trs[len(trs)-1]))
			if c == nil {
				return nil
			}
			cmd, err := c.resolve(e)
			if err != nil {
				return nil
			}
			var got *richeditor.Transaction
			cmd(n, func(tr *richeditor.Transaction) { got = tr })
			return got
		}
	}
	return out
}
