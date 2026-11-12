//go:build darwin && !ios

package macos

import (
	"unsafe"

	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
)

// The caret and pastes of text inputs. A field edits in the surface's
// field editor, which is NSGo's own (NsgoFieldEditor, from the field's
// cell, so in a host's window too) so that it hears pastes, as NSH's
// FieldEditor does; a text view hears its own. Secure fields keep
// AppKit's.

var classFieldEditor, classFieldCell, classWindowDelegate objc.Class

func registerPaste() {
	paste := func(self objc.ID, cmd objc.SEL, sender objc.ID) {
		if pasteInto(self) {
			apple.Super(self, cmd, sender)
		}
	}
	classFieldEditor = register("NsgoFieldEditor", apple.Class("NSTextView"), nil, append(editorCursor(), []objc.MethodDef{
		{Cmd: sel("paste:"), Fn: paste},
		{Cmd: sel("pasteAsPlainText:"), Fn: paste},
		{Cmd: sel("validateUserInterfaceItem:"), Fn: validatePaste},
	}...))
	classFieldCell = register("NsgoFieldCell", apple.Class("NSTextFieldCell"), nil, []objc.MethodDef{{
		Cmd: sel("fieldEditorForView:"),
		Fn: func(self objc.ID, _ objc.SEL, control objc.ID) objc.ID {
			v := views[control]
			if v == nil || v.surface == nil {
				return 0
			}
			s := v.surface
			if s.fieldEditor == 0 {
				s.fieldEditor = objc.ID(classFieldEditor).Send(selAlloc).Send(sel("init"))
				s.fieldEditor.Send(sel("setFieldEditor:"), true)
			}
			return s.fieldEditor
		},
	}})
	classWindowDelegate = register("NsgoWindowDelegate", apple.Class("NSObject"),
		[]*objc.Protocol{objc.GetProtocol("NSWindowDelegate")}, []objc.MethodDef{{
			// An opened window's close is the app's to decide
			// (core.Window.OnClosing), which destroys it.
			Cmd: sel("windowShouldClose:"),
			Fn: func(self objc.ID, _ objc.SEL, window objc.ID) bool {
				if s := windowSurface(window); s != nil && s.id != 0 && !finished {
					s.engine.Dispatch(driver.Event{Kind: driver.EventWindowClose})
					return false
				}
				return true
			},
		}, {
			// A window the app keeps from maximizing stays as it is.
			// The rect after the window goes unread.
			Cmd: sel("windowShouldZoom:toFrame:"),
			Fn: func(self objc.ID, _ objc.SEL, window objc.ID) bool {
				s := windowSurface(window)
				return s == nil || !s.constraints.NoMaximize
			},
		}, {
			// A test host's app ends with its window, once AppKit's close
			// returned.
			Cmd: sel("windowWillClose:"),
			Fn: func(self objc.ID, _ objc.SEL, note objc.ID) {
				if s := windowSurface(note.Send(sel("object"))); s == nil || s.id == 0 {
					apple.OnMain(finish)
				}
			},
		}})
}

// validatePaste enables Paste for an input that hears pastes, whatever the
// pasteboard holds: AppKit enables it only for text it could insert.
func validatePaste(self objc.ID, cmd objc.SEL, item objc.ID) bool {
	if a := objc.Send[objc.SEL](item, sel("action")); a == sel("paste:") || a == sel("pasteAsPlainText:") {
		if in := pasteInput(self); in != nil && in.fs.Events.Has(driver.EventPaste) {
			return true // images and files too
		}
	}
	return apple.SendSuper[bool](self, cmd, item)
}

// setWindowDelegate gives s's own window the delegate that ends the app
// with it.
func (s *surface) setWindowDelegate() {
	d := objc.ID(classWindowDelegate).Send(selAlloc).Send(sel("init"))
	s.window.Send(selSetDelegate, d)
}

// pasteInput returns the input that editor, a text view or the field
// editor editing a field, pastes into.
func pasteInput(editor objc.ID) *input {
	if in := inputOf(editor); in != nil {
		return in
	}
	return inputOf(editor.Send(sel("delegate")))
}

// pasteInto reports a paste into editor's input, and whether the editor
// inserts its text: unless the input only reports pastes, there is none,
// or it names the files pasted, as the text of a Finder copy does.
func pasteInto(editor objc.ID) bool {
	in := pasteInput(editor)
	if in == nil || !in.fs.Events.Has(driver.EventPaste) {
		return true
	}
	p := pasted()
	in.fs.Dispatch(driver.Event{Kind: driver.EventPaste, ID: in.fs.ID, Data: p})
	return p.Text != "" && len(p.URIs) == 0 && in.fs.Config.PasteBehavior == driver.PasteInsert
}

// pasted reads the general pasteboard as a paste: its text, its local
// files, or else its image, as a PNG or a JPEG, which is what apps read.
func pasted() *driver.PasteEvent {
	pb := objc.ID(apple.Class("NSPasteboard")).Send(sel("generalPasteboard"))
	p := &driver.PasteEvent{HTML: apple.GoString(pb.Send(sel("stringForType:"), apple.NSString("public.html"))),
		Text: apple.GoString(pb.Send(sel("stringForType:"), apple.NSString(pasteboardText))),
		URIs: fileURLs(pb)}
	if len(p.URIs) > 0 {
		return p // a file's icon is no image of the user's
	}
	t := availableType(pb, imageTypes)
	if t == "" {
		return p
	}
	data := apple.Bytes(pb.Send(sel("dataForType:"), apple.NSString(t)))
	switch t {
	case "public.png":
		p.Files = []driver.PastedFile{{MIME: "image/png", Data: data}}
	case "public.jpeg":
		p.Files = []driver.PastedFile{{MIME: "image/jpeg", Data: data}}
	default:
		// Copying from Preview or Safari gives a TIFF. A paste is one
		// gesture, so its conversion may keep the main thread.
		if png, _, err := apple.PNG(data); err == nil {
			p.Files = []driver.PastedFile{{MIME: "image/png", Data: png}}
		}
	}
	return p
}

// caretMoved reports where in's caret is, while it has the focus.
func (in *input) caretMoved() {
	if in == nil || !in.focused || !in.fs.Events.Has(driver.EventCaretMove) {
		return
	}
	v := views[in.field]
	e := in.editor()
	if v == nil || e == 0 || v.native.Send(selWindow) == 0 {
		return
	}
	r := objc.Send[apple.NSRange](e, selSelectedRange)
	screen := objc.Send[apple.CGRect](e, sel("firstRectForCharacterRange:actualRange:"),
		apple.NSRange{Location: r.Location + r.Length}, unsafe.Pointer(nil))
	var at apple.CGRect
	if screen.Size.Height > 0 {
		w := objc.Send[apple.CGRect](v.native.Send(selWindow), sel("convertRectFromScreen:"), screen)
		at = objc.Send[apple.CGRect](v.native, sel("convertRect:fromView:"), w, objc.ID(0))
	}
	in.fs.Dispatch(driver.Event{Kind: driver.EventCaretMove, ID: in.fs.ID,
		Point: driver.Point{X: driver.Points(at.Origin.X), Y: driver.Points(at.Origin.Y)},
		Size:  driver.Size{Width: driver.Points(max(at.Size.Width, 1)), Height: driver.Points(at.Size.Height)}})
}
