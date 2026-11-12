// Package richeditor gives TSX nsgo/base's RichEditor and its sessions,
// which link packages other apps should not: an app whose TSX uses them
// imports this package for its effect.
//
// An app makes a session with createRichEditor from extensions (the
// built-in StarterKit, TableKit, Mention and CharacterCount, and its own
// from defineExtension) and plugins whose hooks are its functions, drives
// it with commands as data (command, chain, toggleMark, …), and shows it
// with <RichEditor editor={editor} />.
package richeditor

import (
	"strings"

	"github.com/nsgo-dev/nsgo/base/richeditor"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/tsx"
)

// View is RichEditor in TSX: base/richeditor's View over a
// RichEditorSession, else over a session of its own from Markdown.
type View struct {
	Editor              *Editor
	Markdown            string
	Placeholder         string
	BubbleMenu          func() core.Node
	BubbleMenuPlacement core.Placement
	BubbleMenuOffset    core.Points
	OnChange            func(revision int)
	OnSelectionChange   func(selection RichSelection)
	OnFocus, OnBlur     func()
	AutoFocus           bool
	Label               string
	Classes             richeditor.Classes
	Styles              richeditor.Styles
	Class               string
	Style               core.Style
	TestID              string
	OnLayout            func(core.Bounds)
	Tooltip             core.Node
}

// Base is the Go view it shows, for a styled form to give its look.
func (v View) Base() richeditor.View {
	out := richeditor.View{Markdown: v.Markdown, Placeholder: v.Placeholder, BubbleMenu: v.BubbleMenu,
		BubbleMenuPlacement: v.BubbleMenuPlacement, BubbleMenuOffset: v.BubbleMenuOffset, OnChange: v.OnChange,
		AutoFocus: v.AutoFocus, Label: v.Label, Classes: v.Classes, Styles: v.Styles, Class: v.Class, Style: v.Style,
		TestID: v.TestID, OnLayout: v.OnLayout, Tooltip: v.Tooltip}
	e := v.Editor
	if e != nil {
		out.Editor = e.session()
	}
	if f := v.OnSelectionChange; f != nil {
		out.OnSelectionChange = func(richeditor.Selection) {
			if e != nil {
				f(e.Selection())
			}
		}
	}
	out.OnFocus = func() {
		if e != nil {
			e.setFocused(true)
		}
		if v.OnFocus != nil {
			v.OnFocus()
		}
	}
	out.OnBlur = func() {
		if e != nil {
			e.setFocused(false)
		}
		if v.OnBlur != nil {
			v.OnBlur()
		}
	}
	return out
}

// Render implements core.Node.
func (v View) Render(*core.Scope) core.Node { return v.Base() }

// openLink opens a link pressed with Mod, through the app's openLink.
func (e *Editor) openLink(href string) {
	if e.open != nil {
		e.open(href)
	}
}

// validate reports what createRichEditor found wrong once the schema was
// known: commands bound to unknown names or types, rules of unknown types.
func (e *Editor) validate() error {
	problems := e.problems
	for _, c := range e.check {
		if _, err := c.c.resolve(e); err != nil {
			problems = append(problems, c.where+": "+err.Error())
		}
	}
	if len(problems) > 0 {
		return typeError("createRichEditor: %s", strings.Join(problems, "; "))
	}
	return nil
}

func init() {
	const module = "nsgo/base"
	tsx.Register(module, map[string]core.Node{"RichEditor": View{}}, tsx.Event("OnChange", "revision"))
	tsx.RegisterClass(module, "RichEditorSession", tsx.Class[Editor]{Getters: []string{"Revision", "Editable", "IsFocused", "Selection", "Size"}})
	tsx.RegisterClass(module, "RichCommand", tsx.Class[Command]{})
	tsx.Optional[RichExtensionSpec]("Priority")
	tsx.Optional[RichDecoration]("Side")
	tsx.RegisterClass(module, "RichExtension", tsx.Class[Extension]{Getters: []string{"Name", "Options"}})
	funcs := map[string]any{
		"createRichEditor": createRichEditor, "defineExtension": defineExtension,
		"StarterKit": StarterKit, "TableKit": TableKit, "Mention": Mention, "CharacterCount": CharacterCount,
	}
	for k, f := range kernelCommands {
		funcs[k] = f
	}
	tsx.RegisterFunctions(module, funcs)
}
