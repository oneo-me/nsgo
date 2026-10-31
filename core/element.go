package core

import (
	"time"

	"github.com/oneo-me/nsgo/internal/driver"
)

// Node is what a tree is made of: a host element (View, Text, Input,
// Scroll), a region (Group, Dynamic, Show, Each, Catch) or a component.
//
// A component is any type whose Render returns its nodes; its fields are
// its props. Render runs once, under a scope of the component's own, and
// signals keep what it returned up to date. Host elements and regions
// return themselves.
type Node interface {
	Render(cx *Scope) Node
}

// Func is a component written as a function.
type Func func(cx *Scope) Node

// Render implements Node.
func (f Func) Render(cx *Scope) Node { return f(cx) }

// Group is several nodes in a row, laid out as children of the parent.
type Group []Node

// Render implements Node.
func (g Group) Render(*Scope) Node { return g }

// View is a container. Its look is its style; it has none of its own.
type View struct {
	// Class is Tailwind CSS classes ("flex-row gap-4 p-4 bg-white
	// dark:bg-zinc-900"), meaning what they mean in Tailwind CSS 4, under
	// Style; an unknown class is logged once.
	Class string
	// ClassFunc's classes follow Class's, again when what it reads
	// changes.
	ClassFunc func() string
	Style     Style
	// StyleFunc's result is merged over Style, again when what it reads
	// changes.
	StyleFunc func() Style
	Children  []Node

	// OnPress runs on a completed press: tap, click or accessibility
	// action. Pressable views capture presses on their descendants.
	OnPress     func(PressEvent)
	OnLongPress func(PressEvent)
	OnPressDown func(PressEvent)
	OnPressUp   func(PressEvent)
	// OnHoverIn and OnHoverOut follow the mouse over the view or a view
	// inside it, and OnHoverMove where it is while it moves there with no
	// button down. Touches never hover.
	OnHoverIn   func()
	OnHoverOut  func()
	OnHoverMove func(HoverEvent)
	// OnMouseMove receives each mouse move over the view in its
	// coordinates, also with a button down, on desktops and the web.
	OnMouseMove func(Point)
	// OnContextMenu receives a secondary click in the view.
	OnContextMenu func(ContextMenuEvent)
	// OnTouch hears the fingers on the surface while one that went down
	// on the view or inside it is down; Responder makes the view take part
	// in the touch responder.
	OnTouch   func(TouchEvent)
	Responder *Responder
	// NoFocus keeps a view that takes presses out of the keyboard
	// focus's Tab stops on macOS and the web: a backdrop that closes a
	// popover, rows a list moves through with the arrow keys.
	NoFocus bool
	// OnFocus and OnBlur follow the keyboard focus to and from a view that
	// takes presses, on macOS and the web; a click presses without
	// focusing.
	OnFocus func()
	OnBlur  func()
	// PointerEvents says which of the view and its children take
	// touches and the pointer.
	PointerEvents PointerEvents
	// HitSlop extends where the view takes touches beyond its bounds.
	HitSlop Edges
	// DelayLongPress is how long a press is held to be a long press; 0
	// is 500 ms.
	DelayLongPress time.Duration
	// OnLayout receives the view's frame after each layout that moves or
	// sizes it.
	OnLayout func(Bounds)

	// OnPan follows a drag on the view, once PanActivation activates it;
	// an active pan takes the touch from presses and scroll views.
	OnPan         func(PanEvent)
	PanActivation PanActivation
	// PanScrollHandoff lets a vertical pan share a drag with the scroll
	// view inside the view where it starts, on iOS, as a bottom sheet's
	// does: the drag scrolls it back to its top first, and only what goes
	// on past that moves the pan. Other platforms leave the drag to the
	// scroll view.
	PanScrollHandoff bool
	// OnPinch follows a two-finger pinch (a trackpad's on desktops);
	// OnRotation a two-finger rotation.
	OnPinch    func(GestureEvent)
	OnRotation func(GestureEvent)
	// PanValues are animated values a drag on the view moves on the
	// platform's main thread, by its translation from where they were as
	// it began, with no engine work per move. They need no OnPan.
	PanValues AnimatedXY
	// PinchValue is scaled by pinches on the view, and RotationValue turned
	// by rotations, in radians, on the main thread as PanValues are: from
	// where they were as the gesture began. They need no OnPinch or
	// OnRotation.
	PinchValue    *AnimatedValue
	RotationValue *AnimatedValue
	// OnFileDrop receives the file: URLs of files dropped on the view from
	// another app, on desktops and the web; OnFileDragEnter and
	// OnFileDragLeave follow them while they are dragged over it. The
	// innermost view with OnFileDrop under the pointer takes them, unless
	// it is Disabled.
	OnFileDrop      func(uris []string)
	OnFileDragEnter func()
	OnFileDragLeave func()
	// IOS, Android and Desktop hold the props only iOS, Android or the
	// desktops have; other platforms ignore them.
	IOS     IOSViewProps
	Android AndroidViewProps
	Desktop DesktopViewProps
	// Selectable makes the texts inside select together, as a web page's
	// do: one selection runs from a place in one text to a place in
	// another, across views. On desktops it is made by dragging, a double
	// click (a word) or a triple click (a whole text), and extended by
	// Shift-clicks; on touch screens by a long press. Cmd or Ctrl+A
	// selects the view's texts, Cmd or Ctrl+C copies and Escape clears.
	// Texts with SelectableOff stay out, and a selectable view inside
	// another is part of the outer one. The highlight is each text's
	// SelectionColor.
	Selectable bool

	Ref *Ref
	// Disabled stops presses and is reported to assistive technology.
	Disabled func() bool
	Selected func() bool
	Checked  func() bool
	// Indeterminate is neither checked nor unchecked, as a "select all"
	// over a partial selection.
	Indeterminate func() bool
	// PartStates styles the view in the states it returns without
	// reporting them, as a part of a control in them: a checkbox's
	// indicator follows the checkbox. Assistive technology hears the
	// states from the control alone.
	PartStates func() PartStates
	// Expanded makes the element expandable, as a disclosure or a menu
	// button, and expanded while it returns true.
	Expanded func() bool
	Role     Role
	// Label is what assistive technology reads; LabelFunc replaces it,
	// again when what it reads changes.
	Label     string
	LabelFunc func() string
	// Hint says what activating the element does, read after its label.
	Hint string
	// AccessibilityValue is a range's or a textual element's value, as
	// a slider's or a progress bar's.
	AccessibilityValue func() AccessibilityValue
	// AccessibilityActions are the actions assistive technology offers
	// on the element: the standard ones (ActionActivate, ActionIncrement,
	// ...) through the screen reader's own gestures, others in its
	// actions menu. OnAccessibilityAction receives their names.
	AccessibilityActions  []AccessibilityAction
	OnAccessibilityAction func(name string)
	// Accessible makes the view one element: its descendants are not
	// exposed separately, and without a label it reads their text.
	Accessible                bool
	ImportantForAccessibility Importance
	// AccessibilityElementsHidden hides the view and its subtree from
	// assistive technology while it returns true, as content behind a
	// drawer.
	AccessibilityElementsHidden func() bool
	// LiveRegion has screen readers announce changes of the label and
	// text inside the view.
	LiveRegion LiveRegion
	// Tooltip shows above the view, 4 points away, once the pointer has
	// rested on it for half a second on desktops, until the pointer leaves
	// or presses; it never takes the pointer. Touch screens show none.
	Tooltip Node
	TestID  string
}

// Render implements Node.
func (v View) Render(*Scope) Node { return v }

// PartStates are the states a part of a control is styled in: its Style's
// Checked, Indeterminate, Selected and Disabled merge in while they hold.
type PartStates struct {
	Checked, Indeterminate, Selected, Disabled bool
}

// Text is a run of text the engine lays out and the platform draws.
type Text struct {
	// Class is Tailwind CSS classes, as View's, and ClassFunc's follow
	// them, again when what it reads changes.
	Class     string
	ClassFunc func() string
	Content   string
	// ContentFunc replaces Content, again when what it reads changes.
	ContentFunc func() string
	Style       Style
	StyleFunc   func() Style
	// MaxLines limits the lines shown, cutting the rest as EllipsizeMode
	// says: an ellipsis at the last line's end by default.
	MaxLines      int
	EllipsizeMode EllipsizeMode
	// NoFontScaling keeps the text's sizes whatever the user's text size
	// setting; MaxFontSizeMultiplier, when 1 or more, caps the setting's
	// multiplier for it.
	NoFontScaling         bool
	MaxFontSizeMultiplier float32
	// AdjustsFontSizeToFit shrinks the text's fonts, line heights and
	// letter spacing, down to MinimumFontScale of them (at least 0.1),
	// until it fits its box and MaxLines.
	AdjustsFontSizeToFit bool
	MinimumFontScale     float32
	// Spans, when set, are the text: runs in order, each styled over the
	// Text's style. A span with OnPress is a link; a press on another
	// span is the Text's OnPress.
	Spans []Span
	// SpansFunc replaces Spans, again when what it reads changes: runs of
	// text, class and style only, as a code editor's rows recolor as their
	// highlighting comes.
	SpansFunc func() []Span
	// Selectable lets the user select and copy the text; the highlight is
	// the style's SelectionColor.
	Selectable Selectable

	OnPress  func(PressEvent)
	OnLayout func(Bounds)
	// OnTextLayout receives the text's lines each time it is laid out
	// anew, after the commit.
	OnTextLayout func(lines []TextLine)
	Ref          *Ref
	Role         Role
	// Label replaces the text for assistive technology; LabelFunc
	// replaces Label, again when what it reads changes.
	Label     string
	LabelFunc func() string
	Hint      string
	// LiveRegion has screen readers announce the text as it changes.
	LiveRegion LiveRegion
	TestID     string
}

// Render implements Node.
func (t Text) Render(*Scope) Node { return t }

// Span is a run of a Text with a style of its own.
type Span struct {
	// Class is Tailwind CSS classes under Style, as View's.
	Class       string
	Content     string
	ContentFunc func() string
	// Style's text fields merge over the Text's. Its Background and
	// CornerRadius draw behind the span's characters, a box per line.
	// StyleFunc's result merges over Style, again when what it reads
	// changes.
	Style     Style
	StyleFunc func() Style
	OnPress   func(PressEvent)
	// Attachment, when set, is a view in the text's lines in place of
	// Content: laid out at its content size, it wraps with the words
	// around it and takes its own presses. Style is then its view's
	// style, and Style.VerticalAlign places it in its line, else the
	// VerticalAlign of the one view Attachment makes. Copying text leaves
	// attachments out.
	Attachment Node
}

// Input is the platform's text field, a single line unless Multiline:
// the core's part of base.Input, which apps use. With Value it is
// controlled: the app updates the value in OnChange.
// Its style's PlaceholderColor colors the placeholder and SelectionColor
// the caret and the selection.
type Input struct {
	// Class is Tailwind CSS classes, as View's.
	Class        string
	Value        func() string
	DefaultValue string
	Placeholder  string
	// Masked hides the characters, for passwords.
	Masked       bool
	Readonly     bool
	Disabled     func() bool
	KeyboardType KeyboardType
	AutoFocus    bool
	// Multiline wraps the text in lines and grows with them unless its
	// style sizes it; Return inserts a line break unless SubmitBehavior
	// says else.
	Multiline bool
	// MaxLength caps the text the user enters, in UTF-16 units; 0 sets
	// no cap.
	MaxLength int
	// ReturnKeyType labels the software keyboard's return key.
	ReturnKeyType  ReturnKeyType
	SubmitBehavior SubmitBehavior
	AutoCapitalize AutoCapitalize
	// NoAutoCorrect turns the system's autocorrection off.
	NoAutoCorrect bool
	// AutoComplete says what the field holds, for the system's autofill.
	AutoComplete AutoComplete
	// IOS holds the props only iOS has.
	IOS IOSInputProps
	// HideCaret draws no caret.
	HideCaret bool
	// SelectedRange, when set, selects its range whenever it changes.
	SelectedRange func() TextRange
	// InputAccessoryViewID names the InputAccessoryView shown above the
	// software keyboard while the field has focus.
	InputAccessoryViewID string
	Style                Style
	StyleFunc            func() Style

	OnChange func(text string)
	// OnSubmit runs on Return, as SubmitBehavior says.
	OnSubmit func(text string)
	OnFocus  func()
	OnBlur   func()
	// OnSelectionChange receives the selection, or the caret, as the user
	// moves it.
	OnSelectionChange func(TextRange)
	// OnKeyPress receives each key the user types: the text it enters,
	// "Backspace" or "Enter".
	OnKeyPress func(key string)
	// OnCompositionChange follows an input method composing text: as the
	// composition starts and changes, and as it ends, after the OnChange
	// of the text it commits. While one lasts, the field holds a Value the
	// app sets and shows it when the composition ends, unless the user
	// edited the text since.
	OnCompositionChange func(Composition)
	// OnContentSizeChange receives the size of the value's text, wrapped
	// at the content width when Multiline, after a layout that changed
	// it. An empty field's is one empty line's.
	OnContentSizeChange func(Size)
	// OnCaretMove receives where the caret is while the input has the
	// focus, whenever it moves: with the selection, and when the text
	// scrolls or wraps again. Something that opens at the caret, such as
	// an @-mention's suggestions, anchors to its PageBounds.
	OnCaretMove func(CaretEvent)
	// OnPaste receives what the user pastes, before the text it inserts
	// is reported; PasteBehavior PasteReport keeps the text out. Images
	// and files are never inserted.
	OnPaste       func(PasteEvent)
	PasteBehavior PasteBehavior
	OnLayout      func(Bounds)
	Ref           *Ref
	Label         string
	TestID        string
}

// Render implements Node.
func (i Input) Render(*Scope) Node { return i }

// Scroll scrolls its children, laid out as a column (or a row when
// Horizontal) as long as they need. Like React Native's ScrollView, it
// shrinks to fit its parent (FlexShrink 1) unless its style says else.
type Scroll struct {
	// Class is Tailwind CSS classes, as View's.
	Class         string
	Horizontal    bool
	HideIndicator bool
	// Behavior is how it scrolls: paging, snapping, deceleration,
	// bounces, insets and the keyboard; BehaviorFunc, when set, replaces
	// it as it changes.
	Behavior     ScrollBehavior
	BehaviorFunc func() ScrollBehavior
	// ContentOffset is where it starts.
	ContentOffset Point
	Style         Style
	StyleFunc     func() Style
	Children      []Node
	// OnScroll receives where it is as it scrolls, at most once a turn,
	// and at most once per ScrollEventThrottle when that is set: then the
	// latest offset waits for the interval to pass.
	OnScroll            func(ScrollEvent)
	ScrollEventThrottle time.Duration
	// OnScrollDragStart and OnScrollDragEnd bracket the user dragging it
	// (its content or its scroll bar); OnMomentumStart and
	// OnMomentumEnd the glide or snap after. A glide a new drag catches
	// ends with OnMomentumEnd.
	OnScrollDragStart func(ScrollEvent)
	OnScrollDragEnd   func(ScrollEvent)
	OnMomentumStart   func(ScrollEvent)
	OnMomentumEnd     func(ScrollEvent)
	// OnRefresh makes it refreshable: pulling the content down past its
	// top calls it and shows the platform's spinner while Refreshing
	// returns true. Set Refreshing in OnRefresh, clear it when done.
	// Behavior.RefreshIndicator replaces the spinner with the app's own
	// indicator, which PullValue animates.
	OnRefresh  func()
	Refreshing func() bool
	// PullValue follows how far the content is pulled past its top on
	// the platform's main thread: 0 at rest, the indicator's room while
	// it refreshes. Draw an indicator of the app's own over the top of
	// the scroll view with styles bound to it.
	PullValue *AnimatedValue
	// MaxZoomScale above MinZoomScale (both 1 when 0) lets pinches zoom
	// the content, which then scrolls both ways. ZoomScale is the scale
	// it starts at, and ZoomScaleFunc zooms to its value when it
	// changes; the user's zoom stays otherwise. NoBouncesZoom stops
	// pinches at the limits, NoPinchZoom ignores them, and CenterContent
	// centers content smaller than the viewport.
	MinZoomScale  float32
	MaxZoomScale  float32
	ZoomScale     float32
	ZoomScaleFunc func() float32
	NoBouncesZoom bool
	NoPinchZoom   bool
	CenterContent bool
	// OffsetValues follow the offset on the platform's main thread, in
	// the same frame as it scrolls.
	OffsetValues AnimatedXY
	OnLayout     func(Bounds)
	Ref          *Ref
	TestID       string
}

// Render implements Node.
func (s Scroll) Render(*Scope) Node { return s }

// Ref refers to a mounted element: an overlay's anchor, focus, its
// bounds. Its zero value is ready to use; pass its address in an
// element's Ref field.
type Ref struct {
	t *Tree
	n *node
}

// Bounds returns the element's frame in surface points after the last
// commit, and false while it is not mounted.
func (r *Ref) Bounds() (Bounds, bool) {
	if r.n == nil || r.n.deleted {
		return Bounds{}, false
	}
	return r.t.surfaceBounds(r.n), true
}

// Focus moves keyboard focus to the element, a text field.
func (r *Ref) Focus() {
	if r.n != nil && !r.n.deleted {
		r.t.command(driver.ItemFocus, r.n)
	}
}

// ScrollTo scrolls the element, a Scroll or List, so its content's
// offset is at.
func (r *Ref) ScrollTo(at Point, animated bool) {
	if r.n == nil || r.n.deleted {
		return
	}
	if ln := r.n.list; ln != nil {
		r.t.listCommand(ln, listCommand{to: at, animated: animated})
		return
	}
	if r.n.scroll != nil {
		r.t.scrollTo(r.n, at, animated)
	}
}

// ScrollToEnd scrolls the element, a Scroll or List, to the end of its
// content. A list keeps following the end as the rows there are
// measured, until the user scrolls away.
func (r *Ref) ScrollToEnd(animated bool) {
	if r.n == nil || r.n.deleted {
		return
	}
	if ln := r.n.list; ln != nil {
		r.t.listCommand(ln, listCommand{pin: &listPin{end: true}, animated: animated})
		return
	}
	if sc := r.n.scroll; sc != nil {
		d := sc.config.Behavior.ContentInset
		end := Point{X: max(sc.content.Width-r.n.frame.Size.Width+d.Right, -d.Left)}
		if !sc.config.Horizontal {
			end = Point{Y: max(sc.content.Height-r.n.frame.Size.Height+d.Bottom, -d.Top)}
		}
		r.t.scrollTo(r.n, end, animated)
	}
}

// ZoomTo zooms the element, a Scroll that zooms, to scale (clamped to
// its limits), with content point center (unzoomed) in the middle of the
// viewport, or keeping the middle when center is nil.
func (r *Ref) ZoomTo(scale float32, center *Point, animated bool) {
	if r.n == nil || r.n.deleted || r.n.scroll == nil {
		return
	}
	z := &driver.ZoomRequest{Scale: r.n.scroll.config.ClampZoom(zoomOr(scale, 1))}
	if center != nil {
		c := *center
		z.Center = &c
	}
	r.t.commands = append(r.t.commands, driver.MountItem{Kind: driver.ItemZoomTo, ID: r.n.id, Zoom: z, Animated: animated})
	r.t.changed = true
}

// ScrollToIndex scrolls the element, a List, so row index is at the
// start of its viewport.
func (r *Ref) ScrollToIndex(index int, animated bool) {
	r.ScrollToIndexAt(index, 0, 0, animated)
}

// ScrollToIndexAt scrolls the element, a List, so row index is at
// viewPosition of its viewport (0 its start, 0.5 its middle, 1 its end),
// viewOffset before it. Sizes of rows never shown are estimates, so the
// list keeps following the row as the rows around it are measured, until
// the user scrolls away or the rows change.
func (r *Ref) ScrollToIndexAt(index int, viewPosition float32, viewOffset Points, animated bool) {
	if r.n == nil || r.n.deleted || r.n.list == nil {
		return
	}
	r.t.listCommand(r.n.list, listCommand{pin: &listPin{index: max(index, 0), viewPosition: viewPosition, viewOffset: viewOffset},
		animated: animated})
}

// Blur takes keyboard focus away from the element.
func (r *Ref) Blur() {
	if r.n != nil && !r.n.deleted {
		r.t.command(driver.ItemBlur, r.n)
	}
}

// Image shows a picture the platform loads, decodes and caches. Layout
// knows only the size Source declares: one dimension derives the other,
// and an image without one is 0 by 0 until its style sizes it.
type Image struct {
	// Class is Tailwind CSS classes, as View's.
	Class        string
	Source       ImageSource
	SourceFunc   func() ImageSource
	Placeholder  ImageSource
	ResizeMode   ResizeMode
	BlurRadius   Points
	FadeDuration time.Duration // 0 is the platform's
	Style        Style
	StyleFunc    func() Style

	OnLoadStart func()
	// OnProgress receives bytes loaded and the total, 0 when unknown.
	OnProgress func(loaded, total int64)
	// OnLoad receives the source's size in pixels.
	OnLoad    func(pixels Size)
	OnError   func(err error)
	OnLoadEnd func()
	OnPress   func(PressEvent)
	OnLayout  func(Bounds)
	Ref       *Ref
	Label     string
	TestID    string
}

// Render implements Node.
func (i Image) Render(*Scope) Node { return i }

// Src returns a source for uri.
func Src(uri string) ImageSource { return ImageSource{URI: uri} }

// Overlay floats its children above everything in the nearest Modal or
// the surface, outside the flow: placed beside Anchor on Placement, or by
// its style's Top, Right, Bottom and Left without one. It never closes
// itself; the app removes it.
type Overlay struct {
	Anchor *Ref
	// AnchorBounds, when set and Anchor is not, places the overlay beside
	// the bounds it returns in the surface, again when they change: an
	// input's caret (CaretEvent.PageBounds), or a point.
	AnchorBounds func() Bounds
	Placement    Placement
	// Align places the overlay along the anchor's side; centered when 0.
	Align Align
	// Offset is the gap to the anchor; Margin keeps the overlay off the
	// edges of its host.
	Offset, Margin Points
	// Class is Tailwind CSS classes, as View's.
	Class     string
	Style     Style
	StyleFunc func() Style
	Children  []Node
	// PointerEvents says which of the overlay and its children take
	// touches and the pointer: None for a tooltip, BoxNone for a layer
	// that passes presses through around its content.
	PointerEvents PointerEvents

	// OnPressOutside receives presses outside the overlay and its anchor.
	OnPressOutside func(at Point)
	// OnCancel receives Escape or back while it is the topmost overlay
	// that listens.
	OnCancel   func()
	OnPosition func(Placement)
	Role       Role
	Label      string
	TestID     string
}

// Render implements Node.
func (o Overlay) Render(*Scope) Node { return o }

// Placement is the side of its anchor an overlay goes on.
type Placement uint8

const (
	// PlaceDefault is the component's own default: below for overlays.
	PlaceDefault Placement = iota
	PlaceBottom
	PlaceTop
	PlaceLeft
	PlaceRight
)

// Modal covers its host, the surface or an outer modal, with a layer of
// its own laid out at the host's size: the core's part of base.Dialog,
// which apps use. Mounting it presents it, and removing it dismisses it.
type Modal struct {
	Animation ModalAnimation
	// Presentation is how it covers the screen on iOS; other platforms
	// ignore it and cover the host.
	Presentation ModalPresentation
	// NoCloseOnSwipe keeps an iOS sheet from being swiped away; the swipe
	// still asks to close it ("swipe").
	NoCloseOnSwipe bool
	// IOS and Android hold the props only iOS, or only Android, has.
	IOS     IOSModalProps
	Android AndroidModalProps
	// StatusBar, or StatusBarFunc, is set over the app's status bar while
	// the modal is in the tree.
	StatusBar     StatusBarSetting
	StatusBarFunc func() StatusBarSetting
	Style         Style
	StyleFunc     func() Style
	Children      []Node
	// OnRequestClose receives Escape or back ("cancel") while the modal is
	// the topmost, and an iOS sheet's swipe ("swipe"); the app decides
	// whether to remove it. A sheet swiped away has left the screen
	// already and stays hidden until the app removes it.
	OnRequestClose func(reason string)
	Role           Role
	Label          string
	TestID         string
}

// Render implements Node.
func (m Modal) Render(*Scope) Node { return m }

// Stack shows the last of its screens: the core's part of base.Nav's
// Stack, which apps use. Pushing is adding a screen, popping is removing
// it. Back pops the top screen, which reports OnDismiss and stays hidden
// until the app removes it.
type Stack struct {
	Style    Style
	Children []Node
	TestID   string
}

// Render implements Node.
func (s Stack) Render(*Scope) Node { return s }

// Screen is one page of a Stack, filling it: the core's part of
// base.NavPage, which apps use.
//
// On iOS and Android the platform draws the system header, a navigation
// bar or a toolbar: Title, the header views, a back button to the screen
// below, and HeaderStyle's Background, Color (the title's) and TintColor
// (the buttons'). The content is laid out below it. Elsewhere Header, when
// set, is drawn at the top and the header options are ignored.
type Screen struct {
	Title     string
	TitleFunc func() string
	Header    Node
	// HeaderLeft, HeaderRight and HeaderTitle are views the system header
	// shows at its start, at its end and in place of the title, each at
	// its content's size.
	HeaderLeft, HeaderRight, HeaderTitle Node
	// IOS holds what only iOS's header has.
	IOS IOSScreenProps
	// HeaderHidden hides the system header.
	HeaderHidden bool
	// HeaderTransparent lays the content out under a header without a
	// background.
	HeaderTransparent bool
	// HeaderLargeTitle shows iOS's large title.
	HeaderLargeTitle bool
	// HeaderBackHidden hides the back button.
	HeaderBackHidden bool
	// HeaderBackTitle labels the back button leading back to this screen;
	// its title by default.
	HeaderBackTitle string
	HeaderStyle     Style
	HeaderStyleFunc func() Style
	// StatusBar, or StatusBarFunc, is set over the app's status bar while
	// the screen is in the tree and not popped.
	StatusBar     StatusBarSetting
	StatusBarFunc func() StatusBarSetting
	Style         Style
	StyleFunc     func() Style
	Children      []Node
	// NoPopOnBack keeps the screen when back is pressed.
	NoPopOnBack bool
	// NoPopOnSwipe turns off iOS's swipe from the leading edge back to
	// the screen below.
	NoPopOnSwipe bool
	// OnBackPress receives every back press while the screen is on top.
	OnBackPress func()
	// OnDismiss reports that the user popped the screen; remove it.
	OnDismiss func()
	TestID    string
}

// Render implements Node.
func (s Screen) Render(*Scope) Node { return s }
