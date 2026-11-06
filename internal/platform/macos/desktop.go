//go:build darwin && !ios

package macos

import (
	"math"
	"strings"

	"github.com/ebitengine/purego/objc"

	"github.com/oneo-me/nsgo/desktop/locales"
	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/platform/apple"
	"github.com/oneo-me/nsgo/internal/platform/menutext"
)

// The window's chrome, as NSH's macOS chrome.rs does it. A hidden title
// bar gives the window a full-size content view under a transparent,
// untitled title bar, with the traffic lights centered in the app's bar,
// where Onevra puts them: 16 points in from the left edge, or where the
// app's WindowControls put them, as MyGo's TrafficLightPosition does.
// Env.TitleBar reports the bar's height and the traffic lights' width,
// nothing in full screen. A left press on a window drag handle moves the
// window with AppKit's own drag, and a double click does what the user
// chose in System Settings. The application menu is an NSMenu bar: role
// items send AppKit's standard actions down the responder chain, the
// others report their ID with EventMenu.

// chrome is a surface's window state.
type chrome struct {
	titleBar    driver.TitleBarStyle
	titleHeight driver.Points
	controls    driver.WindowControls
	// placed is set while the traffic lights are where layoutTrafficLights
	// put them rather than where AppKit does.
	placed bool
	// constraints are the app's limits on the window.
	constraints driver.WindowConstraints

	appName     string
	menu        objc.ID // the app's menu bar
	menuEntries []driver.MenuEntry
	language    string // what the menus say (driver.ItemDirection)
	menuTarget  objc.ID
	// notInText are the IDs of the items whose shortcuts a field or a
	// web view with the keyboard focus keeps (MenuEntry.NotInText).
	notInText  map[uint32]bool
	popup      objc.ID // the context menu shown last
	tracking   bool    // whether a context menu tracks
	nextPopups []func()
}

// The title bar's height and the traffic lights' width when AppKit
// cannot tell, and where the traffic lights start in the app's bar.
const titleBarHeight, trafficLightsWidth = 28, 78
const trafficLightsLeft = 16

const (
	maskFullScreen = 1 << 14
	maskFullSize   = 1 << 15
	zoomButton     = 2
	titleHidden    = 1
	titleVisible   = 0

	// fullScreenNone is the collection behavior of a window that never
	// goes full screen.
	fullScreenNone = 1 << 9
)

var (
	selStyleMask           = sel("styleMask")
	selStandardButton      = sel("standardWindowButton:")
	selSetFrameOrigin      = sel("setFrameOrigin:")
	selContentRectForFrame = sel("contentRectForFrameRect:styleMask:")
)

// rawHit makes the views' hitTest: return the deepest view that takes
// the pointer, by its pointer events, rather than the pressable ancestor a
// press goes to: for the window drag's decision, drops, selections and the
// scroll views under the mouse.
var rawHit bool

var selMenuChosen = sel("nsgoMenuChosen:")

// The action of the full screen role's item, in place of AppKit's
// toggleFullScreen:, whose validation titles the item in the language of
// the app's bundle.
var selMenuFullScreen = sel("nsgoMenuFullScreen:")

// fullScreenTitled is the tag of a full screen item the app did not label,
// which says what choosing it does.
const fullScreenTitled = -1

// chromeItem applies an item of the window's own: its title, a window
// command or the menu bar.
func (s *surface) chromeItem(it *driver.MountItem) {
	switch it.Kind {
	case driver.ItemTitle:
		s.window.Send(sel("setTitle:"), apple.NSString(it.Value))
		s.layoutTrafficLights()
	case driver.ItemWindow:
		s.command(it.Window)
	case driver.ItemMenu:
		s.setMenu(it.Menu)
	}
}

func (s *surface) command(c *driver.WindowCommand) {
	w := s.window
	switch c.Action {
	case driver.WindowSetTitleBar:
		s.titleBar, s.titleHeight = c.TitleBar, c.Height
		s.applyTitleBar()
		// A title bar's change keeps the content's size, not the frame's.
		if s.openFrame.Move {
			s.setWindowFrame(s.openFrame)
		}
	case driver.WindowMinimize:
		w.Send(sel("miniaturize:"), objc.ID(0))
	case driver.WindowToggleMaximize:
		w.Send(sel("zoom:"), objc.ID(0))
	case driver.WindowToggleFullScreen:
		w.Send(sel("toggleFullScreen:"), objc.ID(0))
	case driver.WindowClose:
		w.Send(sel("performClose:"), objc.ID(0))
	case driver.WindowSetFrame:
		s.setWindowFrame(c.Frame)
	case driver.WindowSetLevel:
		s.setLevel(c.Level)
	case driver.WindowOrderFront:
		if s.kind == driver.SurfacePanel {
			s.showPanel(s.anchor)
		} else {
			s.orderFront()
		}
	case driver.WindowShow:
		s.show(c.Anchor)
	case driver.WindowHide:
		if s.kind == driver.SurfacePanel {
			s.hidePanel(false)
		} else {
			w.Send(sel("orderOut:"), objc.ID(0))
		}
	case driver.WindowSetControls:
		s.controls = c.Controls
		s.applyTitleBar()
	case driver.WindowSetMaterial:
		s.setMaterial(c.Material)
		s.refresh()
	case driver.WindowSetConstraints:
		s.setConstraints(c.Constraints)
	}
}

// setConstraints limits the window, a panel being the app's to size.
// miniaturize: does nothing without Miniaturizable, and zoom: nothing
// once windowShouldZoom:toFrame: declines, so the app's commands, the
// title bar's double click and the menus all follow.
func (s *surface) setConstraints(c driver.WindowConstraints) {
	w := s.window
	if w == 0 || s.kind != driver.SurfaceWindow {
		return
	}
	s.constraints = c
	w.Send(sel("setContentMinSize:"), apple.CGSize{Width: float64(c.MinWidth), Height: float64(c.MinHeight)})
	limit := func(p driver.Points) float64 {
		if p > 0 {
			return float64(p)
		}
		return math.MaxFloat32
	}
	w.Send(sel("setContentMaxSize:"), apple.CGSize{Width: limit(c.MaxWidth), Height: limit(c.MaxHeight)})
	behavior := objc.Send[uint](w, sel("collectionBehavior")) &^ fullScreenNone
	if c.NoMaximize {
		behavior |= fullScreenNone
	}
	w.Send(sel("setCollectionBehavior:"), behavior)
	mask := objc.Send[uint](w, selStyleMask)
	style := mask | styleResizable | styleMiniaturizable
	if c.NoResize {
		style &^= styleResizable
	}
	if c.NoMinimize {
		style &^= styleMiniaturizable
	}
	if style != mask {
		w.Send(sel("setStyleMask:"), style)
		s.applyTitleBar()
	}
	if zoom := w.Send(selStandardButton, zoomButton); zoom != 0 {
		zoom.Send(sel("setEnabled:"), !c.NoMaximize)
	}
	if mask&maskFullScreen != 0 {
		return
	}
	size := objc.Send[apple.CGRect](s.native, selBounds).Size
	shown := driver.Size{Width: driver.Points(size.Width), Height: driver.Points(size.Height)}
	if fit := c.Clamp(shown); fit != shown {
		w.Send(sel("setContentSize:"), apple.CGSize{Width: float64(fit.Width), Height: float64(fit.Height)})
	}
}

func (s *surface) applyTitleBar() {
	w := s.window
	hidden := s.titleBar == driver.TitleBarHidden
	mask := objc.Send[uint](w, sel("styleMask"))
	if hidden {
		mask |= maskFullSize
	} else {
		mask &^= maskFullSize
	}
	w.Send(sel("setStyleMask:"), mask)
	w.Send(sel("setTitlebarAppearsTransparent:"), hidden)
	visibility := titleVisible
	if hidden {
		visibility = titleHidden
	}
	w.Send(sel("setTitleVisibility:"), visibility)
	for i := 0; i < 3; i++ {
		if b := w.Send(selStandardButton, i); b != 0 {
			b.Send(sel("setHidden:"), hidden && s.controls.Hidden)
		}
	}
	s.layoutTrafficLights()
	s.refresh()
	// AppKit can restore the old content size after setStyleMask:.
	apple.OnMain(func() {
		content := w.Send(sel("contentView"))
		size := objc.Send[apple.CGRect](w, sel("contentRectForFrameRect:"), objc.Send[apple.CGRect](w, sel("frame"))).Size
		if objc.Send[apple.CGRect](content, sel("frame")).Size != size {
			content.Send(selSetFrameSize, size)
		}
		s.layoutTrafficLights()
		s.refresh()
	})
}

// layoutTrafficLights puts the traffic lights in the app's bar: the
// close button's top left corner at the app's TrafficLights, by default
// 16 points in from the window's left edge as Onevra places them and
// centered in the bar. Without a bar height or a position of the app's
// they stay where AppKit lays them out. AppKit puts them back whenever it
// lays the title bar out, so this runs after each of the window's layout
// passes (NsgoWindow), and changes only what moved.
func (s *surface) layoutTrafficLights() {
	w := s.window
	at := s.controls.TrafficLights
	// An embedding host's toolbar lays the title bar out itself.
	if w == 0 || s.titleBar != driver.TitleBarHidden || s.titleHeight <= 0 && at == (driver.Point{}) ||
		w.Send(sel("toolbar")) != 0 {
		s.restoreTrafficLights()
		return
	}
	if objc.Send[uint](w, selStyleMask)&maskFullScreen != 0 {
		return
	}
	close := w.Send(selStandardButton, 0)
	if close == 0 {
		return
	}
	bar := close.Send(selSuperview)
	container := bar.Send(selSuperview)
	if container == 0 {
		return
	}
	button := objc.Send[apple.CGRect](close, sel("frame"))
	height := float64(s.barHeight())
	frame := objc.Send[apple.CGRect](container, sel("frame"))
	if y := objc.Send[apple.CGRect](w, sel("frame")).Size.Height - height; frame.Size.Height != height || frame.Origin.Y != y {
		frame.Size.Height, frame.Origin.Y = height, y
		container.Send(selSetFrame, frame)
	}
	left, top := float64(at.X), float64(at.Y)
	if left <= 0 {
		left = trafficLightsLeft
	}
	if top <= 0 {
		top = max((height-button.Size.Height)/2, 0)
	}
	if !objc.Send[bool](bar, sel("isFlipped")) {
		top = objc.Send[apple.CGRect](bar, selBounds).Size.Height - top - button.Size.Height
	}
	mini := w.Send(selStandardButton, 1)
	gap := objc.Send[apple.CGRect](mini, sel("frame")).Origin.X - button.Origin.X
	for i := 0; i < 3; i++ {
		b := w.Send(selStandardButton, i)
		origin := apple.CGPoint{X: left + float64(i)*gap, Y: top}
		if b != 0 && objc.Send[apple.CGRect](b, sel("frame")).Origin != origin {
			b.Send(selSetFrameOrigin, origin)
		}
	}
	s.placed = true
}

// restoreTrafficLights gives the traffic lights back to AppKit after
// layoutTrafficLights placed them. A layout pass of the window leaves
// them where they are; hiding and showing them again makes AppKit lay
// the title bar out, as Electron's window_buttons_proxy.mm notes.
func (s *surface) restoreTrafficLights() {
	if !s.placed || s.window == 0 {
		return
	}
	s.placed = false
	for i := 0; i < 3; i++ {
		if b := s.window.Send(selStandardButton, i); b != 0 {
			hidden := objc.Send[bool](b, sel("isHidden"))
			b.Send(sel("setHidden:"), !hidden)
			b.Send(sel("setHidden:"), hidden)
		}
	}
}

// barHeight is the hidden title bar's height: the app's, or the one
// AppKit gives the window's own title bar.
func (s *surface) barHeight() driver.Points {
	if s.titleHeight > 0 {
		return s.titleHeight
	}
	w := s.window
	mask := objc.Send[uint](w, selStyleMask)
	frame := objc.Send[apple.CGRect](w, sel("frame"))
	content := objc.Send[apple.CGRect](objc.ID(objc.GetClass("NSWindow")), selContentRectForFrame,
		frame, mask&^maskFullSize)
	if h := frame.Size.Height - content.Size.Height; h > 0 {
		return driver.Points(h)
	}
	return titleBarHeight
}

// titleBarArea is what a hidden title bar covers: its height on top and
// the traffic lights on the left, unless the app hid them.
func (s *surface) titleBarArea() driver.Edges {
	w := s.window
	if w == 0 || s.titleBar != driver.TitleBarHidden {
		return driver.Edges{}
	}
	if objc.Send[uint](w, selStyleMask)&maskFullScreen != 0 {
		return driver.Edges{}
	}
	height := s.barHeight()
	if w.Send(sel("toolbar")) != 0 {
		// The toolbar an embedding host put there, as AppKit laid it out.
		frame := objc.Send[apple.CGRect](w, sel("frame"))
		layout := objc.Send[apple.CGRect](w, sel("contentLayoutRect"))
		if h := frame.Size.Height - (layout.Origin.Y + layout.Size.Height); h > 0 {
			height = driver.Points(h)
		}
	}
	if s.controls.Hidden {
		return driver.Edges{Top: height}
	}
	left := driver.Points(trafficLightsWidth)
	if zoom := w.Send(selStandardButton, zoomButton); zoom != 0 {
		f := objc.Send[apple.CGRect](zoom, sel("frame"))
		if r := f.Origin.X + f.Size.Width + 8; r > 8 {
			left = driver.Points(r)
		}
	}
	return driver.Edges{Top: height, Left: left}
}

// takePress moves the window from a drag handle, or starts or extends a
// selection, at a left press the view has been given; it reports whether
// it took the press from the view. AppKit has already activated the
// window, so the press reaches the views it hit.
func (s *surface) takePress(event objc.ID) bool {
	return s.dragWindow(event) || s.selectPress(event)
}

// dragWindow moves the window, or zooms it on a double click, when a
// left mouse down lands on a window drag handle; it reports whether it
// took the press. The nearest view from the one hit up that sets
// WindowDrag decides; an input, a scroll view or a view with press or
// gesture handlers nearer than it keeps the press.
func (s *surface) dragWindow(event objc.ID) bool {
	loc := objc.Send[apple.CGPoint](event, selLocationInWindow)
	up := s.native.Send(selSuperview)
	if up == 0 {
		return false
	}
	p := objc.Send[apple.CGPoint](up, selConvertPoint, loc, objc.ID(0))
	rawHit = true
	hit := s.native.Send(sel("hitTest:"), p)
	rawHit = false
	if !dragsWindow(hit, up) {
		return false
	}
	if objc.Send[int](event, sel("clickCount")) == 2 {
		defaults := objc.ID(objc.GetClass("NSUserDefaults")).Send(sel("standardUserDefaults"))
		switch apple.GoString(defaults.Send(sel("stringForKey:"), apple.NSString("AppleActionOnDoubleClick"))) {
		case "Minimize":
			s.window.Send(sel("miniaturize:"), objc.ID(0))
		case "None":
		default:
			s.window.Send(sel("zoom:"), objc.ID(0))
		}
		return true
	}
	s.window.Send(sel("performWindowDragWithEvent:"), event)
	return true
}

func dragsWindow(hit, stop objc.ID) bool {
	for n := hit; n != 0 && n != stop; n = n.Send(selSuperview) {
		v := views[n]
		if v == nil {
			continue
		}
		if v.native != n {
			return false // an input's field
		}
		switch v.windowDrag {
		case driver.WindowDragOn:
			return true
		case driver.WindowDragOff:
			return false
		}
		if v.takesPointer() {
			return false
		}
	}
	return false
}

// takesPointer reports whether presses on v are the app's.
func (v *view) takesPointer() bool {
	return v.kind == driver.KindInput || v.kind == driver.KindScroll || v.kind == driver.KindWeb || v.pressable() ||
		v.events.Has(driver.EventPan) || v.events.Has(driver.EventPinch) || v.events.Has(driver.EventRotation) ||
		v.events.Has(driver.EventContextMenu)
}

// setMenu makes entries the menu bar, or restores the default one.
func (s *surface) setMenu(entries []driver.MenuEntry) {
	nsApp := objc.ID(objc.GetClass("NSApplication")).Send(sel("sharedApplication"))
	if s.menu != 0 {
		defer s.menu.Send(selRelease)
		s.menu = 0
	}
	s.menuEntries = entries
	if entries == nil {
		nsApp.Send(sel("setMainMenu:"), defaultMenu)
		return
	}
	s.ensureMenuTarget()
	clear(s.notInText)
	bar := objc.ID(objc.GetClass("NSMenu")).Send(selNew)
	for _, e := range entries {
		if e.Separator {
			continue
		}
		items := e.Items
		if len(items) == 0 {
			// A top-level action gets a menu of its own.
			items = []driver.MenuEntry{e}
		}
		holder := s.holder(e.Label, e.Disabled, items)
		bar.Send(sel("addItem:"), holder)
		holder.Send(selRelease)
	}
	s.directMenu(bar)
	nsApp.Send(sel("setMainMenu:"), bar)
	s.menu = bar
}

// relabelMenus says the menu bar again in the surface's language.
func (s *surface) relabelMenus() {
	if s.language != defaultLanguage {
		nsApp := objc.ID(objc.GetClass("NSApplication")).Send(sel("sharedApplication"))
		old := defaultMenu
		defaultLanguage, defaultMenu = s.language, mainMenu(s.language)
		if nsApp.Send(sel("mainMenu")) == old {
			nsApp.Send(sel("setMainMenu:"), defaultMenu)
		}
		old.Send(selRelease)
	}
	if s.menu != 0 {
		s.setMenu(s.menuEntries)
	}
}

// The action of disabled items: nothing answers it, so AppKit disables
// them as it validates the menu.
var selMenuDisabled = sel("nsgoMenuItemDisabled:")

// holder returns a new item holding a submenu of items.
func (s *surface) holder(label string, disabled bool, items []driver.MenuEntry) objc.ID {
	var action objc.SEL
	if disabled {
		action = selMenuDisabled
	}
	it := objc.ID(objc.GetClass("NSMenuItem")).Send(selAlloc).Send(sel("initWithTitle:action:keyEquivalent:"),
		apple.NSString(label), action, apple.NSString(""))
	sub := s.submenu(label, items)
	it.Send(sel("setSubmenu:"), sub)
	sub.Send(selRelease)
	return it
}

// submenu returns a new menu of items.
func (s *surface) submenu(title string, items []driver.MenuEntry) objc.ID {
	m := objc.ID(objc.GetClass("NSMenu")).Send(selAlloc).Send(sel("initWithTitle:"), apple.NSString(title))
	lastSeparator := true
	for _, e := range items {
		switch {
		case e.Separator:
			if !lastSeparator {
				m.Send(sel("addItem:"), objc.ID(objc.GetClass("NSMenuItem")).Send(sel("separatorItem")))
				lastSeparator = true
			}
			continue
		case len(e.Items) > 0:
			holder := s.holder(e.Label, e.Disabled, e.Items)
			m.Send(sel("addItem:"), holder)
			holder.Send(selRelease)
		default:
			it := s.item(e)
			m.Send(sel("addItem:"), it)
			it.Send(selRelease)
		}
		lastSeparator = false
	}
	if n := objc.Send[int](m, sel("numberOfItems")); n > 0 && lastSeparator {
		m.Send(sel("removeItemAtIndex:"), n-1)
	}
	return m
}

// The standard actions of the menu roles, sent down the responder chain.
var roleActions = map[driver.MenuRole]string{
	driver.MenuAbout: "orderFrontStandardAboutPanel:", driver.MenuHide: "hide:",
	driver.MenuHideOthers: "hideOtherApplications:", driver.MenuShowAll: "unhideAllApplications:",
	driver.MenuQuit: "terminate:", driver.MenuUndo: "undo:", driver.MenuRedo: "redo:", driver.MenuCut: "cut:",
	driver.MenuCopy: "copy:", driver.MenuPaste: "paste:", driver.MenuSelectAll: "selectAll:",
	driver.MenuMinimize: "performMiniaturize:", driver.MenuZoom: "performZoom:", driver.MenuClose: "performClose:",
}

// item returns a new menu item of e.
func (s *surface) item(e driver.MenuEntry) objc.ID {
	title := menutext.Label(s.language, &e, s.name())
	fullScreen := e.Role == driver.MenuToggleFullScreen
	titled := fullScreen && (e.Label == "" || e.Label == e.Role.Label())
	if titled {
		title = locales.In(s.language).EnterFullScreen() // validateMenu retitles it as the window changes
	}
	shortcut := e.Shortcut
	if shortcut == "" && e.Role != driver.MenuRoleNone {
		shortcut = driver.NormalizeKey(e.Role.Shortcut(true))
	}
	key, mods := keyEquivalent(shortcut)
	var action objc.SEL
	switch {
	case e.Disabled:
		action = selMenuDisabled
	case fullScreen:
		action = selMenuFullScreen
	case e.Role != driver.MenuRoleNone:
		action = sel(roleActions[e.Role])
	case e.ID != 0:
		action = selMenuChosen
	}
	it := objc.ID(objc.GetClass("NSMenuItem")).Send(selAlloc).Send(sel("initWithTitle:action:keyEquivalent:"),
		apple.NSString(title), action, apple.NSString(key))
	if key != "" {
		it.Send(sel("setKeyEquivalentModifierMask:"), mods)
	}
	if action == selMenuFullScreen {
		it.Send(selSetTarget, s.menuTarget)
		if titled {
			it.Send(sel("setTag:"), fullScreenTitled)
		}
	}
	if action == selMenuChosen {
		it.Send(selSetTarget, s.menuTarget)
		it.Send(sel("setTag:"), int(e.ID))
		if e.NotInText && key != "" {
			if s.notInText == nil {
				s.notInText = map[uint32]bool{}
			}
			s.notInText[e.ID] = true
		}
	}
	if e.Checkable {
		state := 0
		if e.Checked {
			state = 1
		}
		it.Send(sel("setState:"), state)
	}
	return it
}

// name is the app's name, for the app menu's items.
func (s *surface) name() string {
	if s.appName == "" {
		s.appName = apple.GoString(objc.ID(objc.GetClass("NSRunningApplication")).
			Send(sel("currentApplication")).Send(sel("localizedName")))
	}
	return s.appName
}

// AppKit's function key characters of the keys core.NormalizeKey names.
var keyEquivalents = map[string]rune{
	"enter": '\r', "escape": 0x1b, "tab": '\t', "space": ' ', "backspace": 0x08, "delete": 0xF728,
	"up": 0xF700, "down": 0xF701, "left": 0xF702, "right": 0xF703, "insert": 0xF727,
	"home": 0xF729, "end": 0xF72B, "pageup": 0xF72C, "pagedown": 0xF72D,
	"plus": '=', "minus": '-',
}

// keyEquivalent returns a normalized shortcut's key equivalent and its
// modifier mask; mod is Command.
func keyEquivalent(shortcut string) (string, uint) {
	if shortcut == "" {
		return "", 0
	}
	parts := strings.Split(shortcut, "+")
	name := parts[len(parts)-1]
	if name == "" && len(parts) > 1 {
		name = "+" // "mod++"
	}
	const shift, control, option, command = 1 << 17, 1 << 18, 1 << 19, 1 << 20
	var mods uint
	for _, m := range parts[:len(parts)-1] {
		switch m {
		case "mod", "meta":
			mods |= command
		case "ctrl":
			mods |= control
		case "alt":
			mods |= option
		case "shift":
			mods |= shift
		}
	}
	key := name
	if r, ok := keyEquivalents[name]; ok {
		key = string(r)
	} else if len(name) > 1 && name[0] == 'f' {
		n := 0
		for _, c := range name[1:] {
			if c < '0' || c > '9' {
				n = -1
				break
			}
			n = n*10 + int(c-'0')
		}
		if n >= 1 && n <= 35 {
			key = string(rune(0xF704 + n - 1))
		}
	}
	return key, mods
}

// menuTargets are the surfaces of the menu items' targets.
var menuTargets = map[objc.ID]*surface{}

// ensureMenuTarget makes the target of the surface's menu items.
func (s *surface) ensureMenuTarget() {
	if s.menuTarget == 0 {
		s.menuTarget = objc.ID(classMenuTarget).Send(selNew)
		menuTargets[s.menuTarget] = s
	}
}

// validateMenu disables an item whose shortcut a field with the keyboard
// focus keeps while a key press looks for it, so AppKit hands the key to
// the field; clicked, the item is enabled.
func validateMenu(self objc.ID, _ objc.SEL, item objc.ID) bool {
	s := menuTargets[self]
	if s != nil && objc.Send[objc.SEL](item, sel("action")) == selMenuFullScreen {
		return s.validateFullScreen(item)
	}
	if s == nil || !s.notInText[uint32(objc.Send[int](item, sel("tag")))] {
		return true
	}
	const keyDown = 10 // NSEventTypeKeyDown
	nsApp := objc.ID(objc.GetClass("NSApplication")).Send(sel("sharedApplication"))
	e := nsApp.Send(sel("currentEvent"))
	return e == 0 || objc.Send[uint](e, sel("type")) != keyDown || !s.editing()
}

// validateFullScreen enables a full screen item while a window has the
// keyboard focus, and titles it by what choosing it does there.
func (s *surface) validateFullScreen(item objc.ID) bool {
	w := objc.ID(objc.GetClass("NSApplication")).Send(sel("sharedApplication")).Send(sel("keyWindow"))
	if w == 0 {
		return false
	}
	if objc.Send[int](item, sel("tag")) == fullScreenTitled {
		msgs, title := locales.In(s.language), ""
		if objc.Send[uint](w, selStyleMask)&maskFullScreen != 0 {
			title = msgs.ExitFullScreen()
		} else {
			title = msgs.EnterFullScreen()
		}
		item.Send(sel("setTitle:"), apple.NSString(title))
	}
	return true
}

// menuFullScreen toggles the window with the keyboard focus.
func menuFullScreen(objc.ID, objc.SEL, objc.ID) {
	objc.ID(objc.GetClass("NSApplication")).Send(sel("sharedApplication")).Send(sel("keyWindow")).Send(sel("toggleFullScreen:"), objc.ID(0))
}

// menuChosen reports a chosen menu item's ID to its surface.
func menuChosen(self objc.ID, _ objc.SEL, item objc.ID) {
	s := menuTargets[self]
	if id := objc.Send[int](item, sel("tag")); id > 0 && s != nil && s.engine != nil && !s.closed {
		s.engine.Dispatch(driver.Event{Kind: driver.EventMenu, Count: uint32(id)})
	}
}
