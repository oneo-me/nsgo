package richeditor

import (
	"github.com/nsgo-dev/nsgo/base/richeditor"
)

// Command is RichCommand: an edit as data, resolved against the editor it
// runs in, so a named command is that editor's and a type name its
// schema's.
type Command struct {
	resolve func(e *Editor) (richeditor.Command, error)
}

func commandOf(f func(e *Editor) (richeditor.Command, error)) *Command { return &Command{resolve: f} }

func plain(c richeditor.Command) *Command {
	return commandOf(func(*Editor) (richeditor.Command, error) { return c, nil })
}

func resolveAll(e *Editor, cmds []*Command) ([]richeditor.Command, error) {
	out := make([]richeditor.Command, len(cmds))
	for i, c := range cmds {
		if c == nil {
			return nil, typeError("command %d is missing", i)
		}
		r, err := c.resolve(e)
		if err != nil {
			return nil, err
		}
		out[i] = r
	}
	return out, nil
}

// command is the editor's named command with its arguments: its
// extensions' (toggleBold, toggleHeading, insertTable, setLink).
func command(name string, args map[string]any) *Command {
	return commandOf(func(e *Editor) (richeditor.Command, error) {
		if c := e.s.Command(name, args); c != nil {
			return c, nil
		}
		return nil, typeError("the editor has no command %q", name)
	})
}

// chain runs commands one after another as one transaction, and applies
// when every one does.
func chain(cmds ...*Command) *Command {
	return commandOf(func(e *Editor) (richeditor.Command, error) {
		list, err := resolveAll(e, cmds)
		if err != nil {
			return nil, err
		}
		return richeditor.Chain(list...), nil
	})
}

// first runs the first of commands that applies, ProseMirror's
// chainCommands.
func first(cmds ...*Command) *Command {
	return commandOf(func(e *Editor) (richeditor.Command, error) {
		list, err := resolveAll(e, cmds)
		if err != nil {
			return nil, err
		}
		return richeditor.ChainCommands(list...), nil
	})
}

func markType(e *Editor, name string) (*richeditor.MarkType, error) {
	if t := e.s.State().Schema.Marks[name]; t != nil {
		return t, nil
	}
	return nil, typeError("the editor has no mark type %q", name)
}

func nodeType(e *Editor, name string) (*richeditor.NodeType, error) {
	if t := e.s.State().Schema.Nodes[name]; t != nil {
		return t, nil
	}
	return nil, typeError("the editor has no node type %q", name)
}

// toggleMark toggles a mark over the selection, or the marks typed next.
func toggleMark(name string, attrs map[string]any) *Command {
	return commandOf(func(e *Editor) (richeditor.Command, error) {
		t, err := markType(e, name)
		if err != nil {
			return nil, err
		}
		return richeditor.ToggleMark(t, attrs), nil
	})
}

// setBlockType makes the selected textblocks nodes of name.
func setBlockType(name string, attrs map[string]any) *Command {
	return commandOf(func(e *Editor) (richeditor.Command, error) {
		t, err := nodeType(e, name)
		if err != nil {
			return nil, err
		}
		return richeditor.SetBlockType(t, attrs), nil
	})
}

// wrapIn wraps the selected blocks in a node of name.
func wrapIn(name string, attrs map[string]any) *Command {
	return commandOf(func(e *Editor) (richeditor.Command, error) {
		t, err := nodeType(e, name)
		if err != nil {
			return nil, err
		}
		return richeditor.WrapIn(t, attrs), nil
	})
}

// insertText types text over the selection, with the marks there.
func insertText(text string) *Command {
	return plain(func(s *richeditor.State, dispatch func(*richeditor.Transaction)) bool {
		if dispatch != nil {
			tr := s.Tr().InsertText(text, -1, -1)
			tr.Origin = richeditor.OriginAPI
			dispatch(tr.ScrollIntoView())
		}
		return true
	})
}

// insertContent puts a document's JSON, or nodes', over the selection.
func insertContent(content map[string]any) *Command {
	return commandOf(func(e *Editor) (richeditor.Command, error) {
		node, err := richeditor.NodeFromJSON(e.s.State().Schema, content)
		if err != nil {
			return nil, err
		}
		return func(s *richeditor.State, dispatch func(*richeditor.Transaction)) bool {
			if dispatch != nil {
				tr := s.Tr()
				if node.Type == s.Schema.TopNode {
					tr.ReplaceSelection(richeditor.MaxOpen(node.Content, false))
				} else {
					tr.ReplaceSelectionWith(node, node.IsInline())
				}
				tr.Origin = richeditor.OriginAPI
				dispatch(tr.ScrollIntoView())
			}
			return true
		}, nil
	})
}

// setTextSelection selects from anchor to head, a caret without head.
func setTextSelection(anchor int, head *int) *Command {
	return plain(func(s *richeditor.State, dispatch func(*richeditor.Transaction)) bool {
		h := anchor
		if head != nil {
			h = *head
		}
		size := s.Doc.Content.Size
		if anchor < 0 || anchor > size || h < 0 || h > size {
			return false
		}
		if dispatch != nil {
			dispatch(s.Tr().SetSelection(richeditor.TextSelectionAt(s.Doc, anchor, h)).ScrollIntoView())
		}
		return true
	})
}

// setNodeSelection selects the node after pos.
func setNodeSelection(pos int) *Command {
	return plain(func(s *richeditor.State, dispatch func(*richeditor.Transaction)) bool {
		if pos < 0 || pos >= s.Doc.Content.Size || s.Doc.Resolve(pos).NodeAfter() == nil {
			return false
		}
		if dispatch != nil {
			dispatch(s.Tr().SetSelection(richeditor.NodeSelectionAt(s.Doc, pos)))
		}
		return true
	})
}

// moveBlocks moves the blocks from from to to to target, between blocks.
func moveBlocks(from, to, target int) *Command { return plain(richeditor.MoveBlocks(from, to, target)) }

var kernelCommands = map[string]any{
	"command": command, "chain": chain, "first": first,
	"toggleMark": toggleMark, "setBlockType": setBlockType, "wrapIn": wrapIn,
	"insertText": insertText, "insertContent": insertContent,
	"setTextSelection": setTextSelection, "setNodeSelection": setNodeSelection, "moveBlocks": moveBlocks,
	"selectAll":       func() *Command { return plain(richeditor.SelectAll) },
	"deleteSelection": func() *Command { return plain(richeditor.DeleteSelection) },
	"lift":            func() *Command { return plain(richeditor.Lift) },
	"undo":            func() *Command { return plain(richeditor.Undo) },
	"redo":            func() *Command { return plain(richeditor.Redo) },
}
