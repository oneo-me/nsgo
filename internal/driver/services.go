package driver

import (
	"bytes"
	"image/png"
)

// AlertConfig is a native alert: a title, a message and its buttons.
type AlertConfig struct {
	Title   string
	Message string
	Actions []AlertAction
	// Sheet presents the actions as an action sheet from the bottom
	// (UIAlertController's; a list dialog on Android) instead of an alert.
	Sheet bool
	// Cancelable lets Escape, back and a press outside close it without
	// an action, answered with -1.
	Cancelable bool
	// ID numbers the alert; the engine sets it.
	ID uint32
}

// AlertAction is one button of an alert.
type AlertAction struct {
	Label string
	Role  AlertRole
}

// AlertRole says what an alert's button does.
type AlertRole uint8

const (
	AlertDefault AlertRole = iota
	// AlertCancel is the button Escape and back choose.
	AlertCancel
	// AlertDestructive is drawn as dangerous.
	AlertDestructive
)

// TitleBarStyle is whether the system draws the window's title bar.
type TitleBarStyle uint8

const (
	// TitleBarDefault is the system's title bar, above the app.
	TitleBarDefault TitleBarStyle = iota
	// TitleBarHidden lets the app fill the window up to its top edge and
	// draw its own title bar. The system's window controls stay, in the
	// area Env.TitleBar reports.
	TitleBarHidden
)

// WindowDrag marks a view as a handle that moves the desktop window, or
// keeps a view inside such a handle clickable. The nearest view that
// sets it decides; inputs and views with press or gesture handlers
// nearer than it stay clickable either way.
type WindowDrag uint8

const (
	// WindowDragInherit follows the views around it.
	WindowDragInherit WindowDrag = iota
	// WindowDragOn drags the window, and a double click zooms it as the
	// system's title bar does.
	WindowDragOn
	// WindowDragOff keeps presses on the view for the app.
	WindowDragOff
)

// WindowMaterial is a translucent material behind a desktop window's
// content, where the app draws nothing opaque. macOS shows a system
// vibrancy material; Windows 11 22H2 and later shows Mica, Acrylic or Mica
// Alt, choosing the closest backdrop for the other names. Other platforms
// keep the window opaque.
//
// The window starts with nsgo.WindowMaterial's and changes it with
// WindowSetMaterial; Windows shows one only in a window started with
// one. Env.WindowMaterial reports the one the window actually shows.
// What the app draws over the material decides how much shows: a view
// with an opaque background hides it, and one with no background, or a
// translucent one, lets it through.
type WindowMaterial uint8

const (
	// WindowMaterialNone keeps the window opaque.
	WindowMaterialNone WindowMaterial = iota
	// The materials of window areas.
	WindowMaterialWindow
	WindowMaterialContent
	WindowMaterialSidebar
	WindowMaterialHeader
	WindowMaterialTitlebar
	WindowMaterialUnderWindow
	WindowMaterialUnderPage
	// The materials of transient surfaces: menus, popovers, HUD windows,
	// sheets, tooltips, selections and full-screen UI.
	WindowMaterialMenu
	WindowMaterialPopover
	WindowMaterialHUD
	WindowMaterialSheet
	WindowMaterialTooltip
	WindowMaterialSelection
	WindowMaterialFullScreenUI
	// Windows 11's system backdrops. Mica and Tabbed use the under-window
	// material on macOS; Acrylic uses the HUD one.
	WindowMaterialMica
	WindowMaterialAcrylic
	WindowMaterialTabbed
)

// WindowAction is what an ItemWindow asks of the window.
type WindowAction uint8

const (
	// WindowSetTitleBar shows or hides the title bar (TitleBar, Height).
	WindowSetTitleBar WindowAction = iota + 1
	WindowMinimize
	// WindowToggleMaximize maximizes the window, or restores it.
	WindowToggleMaximize
	WindowToggleFullScreen
	// WindowClose closes the window as its close button does.
	WindowClose
	// WindowSetFrame moves and sizes the window (Frame).
	WindowSetFrame
	// WindowSetLevel puts the window at Level.
	WindowSetLevel
	// WindowOrderFront brings the window to the front of its level,
	// showing it, and activates the app.
	WindowOrderFront
	// WindowSetMenuBar says when Windows and Linux show the menu bar
	// (MenuBar).
	WindowSetMenuBar
	// WindowSetControls places or hides the window controls of a hidden
	// title bar (Controls).
	WindowSetControls
	// WindowSetMaterial puts Material behind the window's content.
	WindowSetMaterial
)

// WindowCommand is an ItemWindow's request.
type WindowCommand struct {
	Action   WindowAction
	TitleBar TitleBarStyle
	// Height is the custom title bar's height, in which the platform
	// centers its window controls; 0 is the platform's.
	Height   Points
	Frame    WindowFrame
	Level    WindowLevel
	MenuBar  MenuBarMode
	Controls WindowControls
	Material WindowMaterial
}

// WindowControls is how a hidden title bar shows the system's window
// controls: macOS's traffic lights and the caption buttons NSGo draws on
// Windows. Linux draws none.
type WindowControls struct {
	// Hidden takes the controls away for the app to draw its own, which
	// call MinimizeWindow, ToggleMaximize and CloseWindow; Env.TitleBar
	// then reports no width at its ends.
	Hidden bool
	// TrafficLights puts the top left corner of macOS's close button
	// this far from the window's top left corner, in points, the other
	// two after it. A zero X keeps them 16 points in, and a zero Y
	// centers them in the title bar.
	TrafficLights Point
}

// WindowFrame is where a window moves and how large it becomes, in
// points. X and Y place its top left corner from the main screen's top
// left, y growing down, on every platform; without Move the corner stays
// where it is, so a window that changes height keeps its top edge.
// Width and Height are its outer size, borders and title bar included
// (the default size GTK keeps on Linux); 0 keeps the current one.
type WindowFrame struct {
	X, Y          Points
	Move          bool
	Width, Height Points
	// Animated animates the change with macOS's resize animation.
	Animated bool
}

// WindowLevel is where a window stacks among the others.
type WindowLevel uint8

const (
	// WindowLevelNormal is the level of ordinary windows.
	WindowLevelNormal WindowLevel = iota
	// WindowLevelFloating stays above normal windows, as a palette does.
	WindowLevelFloating
	// WindowLevelDesktop sits with the desktop's icons on macOS, on every
	// space and out of Mission Control; Windows puts the window at the
	// bottom of the others, where activating it raises it again.
	WindowLevelDesktop
)

// MenuBarMode says when Windows and Linux show the menu bar the window
// draws for the application menu. macOS's menu bar is the system's and
// always shows.
type MenuBarMode uint8

const (
	// MenuBarShown always shows it, above the app.
	MenuBarShown MenuBarMode = iota
	// MenuBarAutoHide shows it once Alt is pressed and released alone,
	// until Alt alone, a press elsewhere or a chosen item hides it again,
	// as Windows and GNOME apps do. F10 and Alt with a menu's mnemonic
	// still open a menu, and the bar shows while it is open.
	MenuBarAutoHide
)

// Appearance is the light or dark appearance an app asks for, or the
// system's.
type Appearance uint8

const (
	// AppearanceSystem follows the system's appearance.
	AppearanceSystem Appearance = iota
	AppearanceLight
	AppearanceDark
)

// ClipboardRead is what an ItemReadClipboard asks of the clipboard.
type ClipboardRead uint8

const (
	// ClipboardReadText reads its plain text, "" when it has none.
	ClipboardReadText ClipboardRead = iota
	// ClipboardReadHasText reports whether it holds text, without reading it,
	// so iOS shows no paste notice.
	ClipboardReadHasText
	// ClipboardReadImage reads its image as a PNG.
	ClipboardReadImage
	// ClipboardReadHasImage reports whether it holds an image, without
	// reading it.
	ClipboardReadHasImage
	// ClipboardReadFiles reads the file: URLs of the local files it holds,
	// such as files copied in a file manager.
	ClipboardReadFiles
)

// MaxClipboardImage is the most bytes of a PNG a clipboard read returns:
// a larger image reads as none, since the app's memory may not hold it.
const MaxClipboardImage = 48 << 20

// ClipboardImage is an image read from the clipboard.
type ClipboardImage struct {
	// PNG is the image encoded as a PNG, which an ImageSource's Data
	// shows.
	PNG []byte
	// Size is its size in pixels.
	Size Size
}

// MenuRole is a standard menu item the platform carries out, labelling it
// and giving it its usual shortcut when the item has none of its own. The
// app is not told when it is chosen.
type MenuRole uint8

const (
	MenuRoleNone MenuRole = iota
	// MenuAbout, MenuHide, MenuHideOthers and MenuShowAll are macOS's.
	MenuAbout
	MenuHide
	MenuHideOthers
	MenuShowAll
	MenuQuit
	// The edit roles act on the focused text field.
	MenuUndo
	MenuRedo
	MenuCut
	MenuCopy
	MenuPaste
	MenuSelectAll
	MenuMinimize
	MenuZoom
	MenuToggleFullScreen
	MenuClose
)

var menuRoleLabels = [...]string{"", "About", "Hide", "Hide Others", "Show All", "Quit", "Undo", "Redo",
	"Cut", "Copy", "Paste", "Select All", "Minimize", "Zoom", "Toggle Full Screen", "Close"}

// Label returns the label of an item with role r and no label of its own.
func (r MenuRole) Label() string {
	if int(r) < len(menuRoleLabels) {
		return menuRoleLabels[r]
	}
	return ""
}

// Shortcut returns the shortcut of an item with role r and no shortcut of
// its own, in OnKey's form, on a Mac (mac) or elsewhere; "" for none.
func (r MenuRole) Shortcut(mac bool) string {
	pick := func(onMac, other string) string {
		if mac {
			return onMac
		}
		return other
	}
	switch r {
	case MenuHide:
		return pick("meta+h", "")
	case MenuHideOthers:
		return pick("alt+meta+h", "")
	case MenuQuit:
		return pick("meta+q", "alt+f4")
	case MenuUndo:
		return "mod+z"
	case MenuRedo:
		return pick("shift+meta+z", "ctrl+y")
	case MenuCut:
		return "mod+x"
	case MenuCopy:
		return "mod+c"
	case MenuPaste:
		return "mod+v"
	case MenuSelectAll:
		return "mod+a"
	case MenuMinimize:
		return pick("meta+m", "")
	case MenuToggleFullScreen:
		return pick("ctrl+meta+f", "f11")
	case MenuClose:
		return "mod+w"
	}
	return ""
}

// EditsText reports whether r acts on the focused text field, whose own
// shortcut it then is.
func (r MenuRole) EditsText() bool { return r >= MenuUndo && r <= MenuSelectAll }

// MenuEntry is a MenuItem as an ItemMenu carries it: the item's handler
// replaced by an ID the platform reports with EventMenu (0: none), its
// shortcut normalized and its role's label filled in. A role item without
// a shortcut takes its role's, which the platform fills in.
type MenuEntry struct {
	ID        uint32
	Label     string
	Shortcut  string
	Role      MenuRole
	Disabled  bool
	Checkable bool
	Checked   bool
	Separator bool
	Items     []MenuEntry
	// NotInText leaves the shortcut to a text input or a web view with
	// the keyboard focus: the key goes to it rather than choosing the
	// item.
	NotInText bool
}

// StorageOp says what a StorageChange does.
type StorageOp uint8

const (
	// StorageSet sets Key to Value.
	StorageSet StorageOp = iota
	// StorageRemove removes Key.
	StorageRemove
	// StorageClear removes every entry.
	StorageClear
)

// StorageChange is a change to the entries, for the backend to keep.
type StorageChange struct {
	Op         StorageOp
	Key, Value string
}

// StorageBackend is where a platform keeps an app's entries: a file in
// the app's data directory, the browser's localStorage.
type StorageBackend interface {
	// Load returns the entries kept, read once when the app first uses
	// its storage. An error leaves the app with none, logged.
	Load() (map[string]string, error)
	// Write keeps change. It is called in order, on the goroutine of the
	// graph that made the change; a backend that blocks writes elsewhere.
	Write(change StorageChange)
}

// MemoryClipboard is a clipboard of a test's own: it takes the clipboard
// items an app sends and answers its reads, so the machine's clipboard
// is never touched.
type MemoryClipboard struct {
	Text  string
	Image []byte // PNG
	Files []string
}

// Take applies it when it is a clipboard item and reports whether it is:
// a write changes the clipboard, and a read returns the event that
// answers it, with answer true.
func (c *MemoryClipboard) Take(it *MountItem) (ev Event, answer, taken bool) {
	switch it.Kind {
	case ItemSetClipboard:
		c.Text, c.Image, c.Files = it.Value, nil, nil
	case ItemSetClipboardImage:
		data, _ := it.Data.([]byte)
		c.Text, c.Image, c.Files = "", data, nil
	case ItemReadClipboard:
		ev = Event{Kind: EventClipboard, Count: it.Count}
		switch it.Clipboard {
		case ClipboardReadText:
			ev.Text = c.Text
		case ClipboardReadHasText:
			ev.Finished = c.Text != ""
		case ClipboardReadImage:
			if len(c.Image) > 0 {
				img := &ClipboardImage{PNG: c.Image}
				if cfg, err := png.DecodeConfig(bytes.NewReader(c.Image)); err == nil {
					img.Size = Size{Width: Points(cfg.Width), Height: Points(cfg.Height)}
				}
				ev.Data = img
			}
		case ClipboardReadHasImage:
			ev.Finished = len(c.Image) > 0
		case ClipboardReadFiles:
			ev.URIs = append([]string(nil), c.Files...)
		}
		return ev, true, true
	default:
		return Event{}, false, false
	}
	return Event{}, false, true
}
