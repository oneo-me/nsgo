package driver

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// NodeID identifies a view of one tree. IDs start at 1, stay dense and
// are reused after a Delete.
type NodeID uint32

// ViewKind is the kind of native view a node mounts.
type ViewKind uint8

const (
	// KindView is a container: background, border, corners and presses.
	KindView ViewKind = iota + 1
	// KindText draws a TextLayout the engine prepared.
	KindText
	// KindInput is the platform's text field.
	KindInput
	// KindScroll scrolls its children; ItemScrollContent sizes them.
	KindScroll
	// KindImage shows an image the platform loads (ItemImage).
	KindImage
	// KindModal is a layer covering the root view or an outer modal,
	// presented when inserted and dismissed when removed (ItemModal).
	KindModal
	// KindCanvas plays a Drawing (ItemCanvas).
	KindCanvas
	// KindWeb hosts the platform's web view, filling it (ItemWeb).
	KindWeb
	// KindVideo hosts the platform's video player, filling it (ItemVideo).
	KindVideo
	// KindNative hosts a view of a component the app registered, filling
	// it (ItemNative).
	KindNative
	// KindShader fills itself with a fragment shader on the GPU
	// (ItemShader).
	KindShader
)

// ItemKind is what a mount item does.
type ItemKind uint8

const (
	// ItemCreate makes a detached view of Kind for ID.
	ItemCreate ItemKind = iota + 1
	// ItemInsert inserts or moves ID into Parent, below sibling Before
	// (on top when 0). Siblings rather than indices, so views a platform
	// adds of its own cannot shift positions.
	ItemInsert
	// ItemRemove detaches ID from its parent and keeps it.
	ItemRemove
	// ItemDelete detaches and destroys ID; its ID may be created again
	// later in the same batch.
	ItemDelete
	// ItemFrame moves and sizes ID relative to its parent (Frame).
	ItemFrame
	// ItemViewStyle sets what any view draws (ViewStyle).
	ItemViewStyle
	// ItemText sets a text view's content (Text).
	ItemText
	// ItemInput configures a text field (Input).
	ItemInput
	// ItemInputValue sets a text field's value (Value) unless the user
	// edited it after the edit numbered Count.
	ItemInputValue
	// ItemEvents sets which events the view reports (Events).
	ItemEvents
	// ItemViewProps sets accessibility and identity (ViewProps).
	ItemViewProps
	// ItemScrollConfig configures a scroll view (Scroll).
	ItemScrollConfig
	// ItemScrollContent sets the size of a scroll view's content (Size).
	ItemScrollContent
	// ItemFocus moves keyboard focus to ID.
	ItemFocus
	// ItemBlur takes keyboard focus away from ID.
	ItemBlur
	// ItemImage sets an image view's source and look (Image).
	ItemImage
	// ItemModal configures a modal (Modal).
	ItemModal
	// ItemAlert shows a native alert (Alert); the platform answers with
	// EventAlert.
	ItemAlert
	// ItemDismissKeyboard takes focus away from the focused text field.
	ItemDismissKeyboard
	// ItemScrollTo scrolls ID to Point (its content's offset, UIKit's
	// contentOffset: ContentInset makes it negative at the start),
	// animated when Animated; the platform clamps it and reports
	// EventScroll. Platforms count the ItemScrollTo and ItemScrollBy
	// items they applied to each scroll view and report the count with
	// its offsets.
	ItemScrollTo
	// ItemCanvas replaces a canvas's drawing (Drawing), in the view's
	// coordinates.
	ItemCanvas
	// ItemSetClipboard puts Value on the clipboard.
	ItemSetClipboard
	// ItemReadClipboard asks for what Clipboard names; the platform
	// answers with EventClipboard carrying Count.
	ItemReadClipboard
	// ItemAnnounce has the screen reader say Value.
	ItemAnnounce
	// ItemKeymap replaces the surface's keyboard shortcuts (Keys); the
	// platform consumes matching key presses and reports EventKey.
	ItemKeymap
	// ItemStatusBar sets the status bar (StatusBar): the layers of
	// settings merged.
	ItemStatusBar
	// ItemTitle sets the window's or page's title (Value).
	ItemTitle
	// ItemBackUnhandled says nothing took the last EventBack: Android
	// leaves the app, as its back button does; others ignore it.
	ItemBackUnhandled
	// ItemWindow asks the desktop window to change (Window); platforms
	// without one ignore it.
	ItemWindow
	// ItemMenu replaces the application menu (Menu; nil restores the
	// platform's); a chosen item is reported with EventMenu.
	ItemMenu
	// ItemWeb configures a web view (Web).
	ItemWeb
	// ItemWebCommand has a web view do WebCommand, with Value for the
	// message or script.
	ItemWebCommand
	// ItemVideo configures a video (Video).
	ItemVideo
	// ItemVideoSeek jumps a video to Time.
	ItemVideoSeek
	// ItemNative names a native view's component and sets its changed
	// props (Native).
	ItemNative
	// ItemNativeCommand has a native view's component run the command
	// Value with Data.
	ItemNativeCommand
	// ItemAnimation changes the native animation graph (Animation), after
	// the batch's views exist.
	ItemAnimation
	// ItemZoomTo zooms a zoomable scroll view (Zoom), animated when
	// Animated; the platform clamps it and reports EventScroll.
	ItemZoomTo
	// ItemImageInfo loads Image's Source, keeping its bytes for the views
	// that show it next; the platform answers with EventImageInfo carrying
	// Count.
	ItemImageInfo
	// ItemSelection selects Selection in text field ID, after the batch's
	// value; the platform clamps it to the text.
	ItemSelection
	// ItemScrollBy moves scroll view ID's offset by Point without
	// clamping it, carrying a drag or glide in flight along, so the
	// visible content stays in place as rows above it change size.
	ItemScrollBy
	// ItemStickyCells replaces the sticky cells of scroll view ID
	// (Sticky): on every scroll the platform moves each along the main
	// axis by clamp(offset - Start, 0, End - Start) from its frame.
	ItemStickyCells
	// ItemAccessibilityLabel changes the label (Value) of a view whose
	// ViewProps were sent before, and nothing else: a label following app
	// state changes alone, so the rest is not sent again.
	ItemAccessibilityLabel
	// ItemAccessibilityFocus moves the screen reader's focus to ID.
	ItemAccessibilityFocus
	// ItemScreen configures a screen's system header (Screen) on the
	// platforms that report header heights, before the screen is
	// inserted and when it changes.
	ItemScreen
	// ItemOverflow gives the area ID's children cover outside its bounds
	// (Frame, in its own coordinates), or zero Bounds when they stay
	// inside, for platforms to look for a touch's target there; see
	// Reach. Sent only when it changes.
	ItemOverflow
	// ItemBlockNative stops the scroll views around ID, the touch
	// responder, from taking the touches until they end, or lets them
	// again with no ID.
	ItemBlockNative
	// ItemShader sets a shader view's program and uniforms (Shader).
	ItemShader
	// ItemSetClipboardImage puts the encoded image Data ([]byte) on the
	// clipboard.
	ItemSetClipboardImage
	// ItemOpenURL opens Value outside the app; the platform answers with
	// EventOpenURL carrying Count.
	ItemOpenURL
	// ItemPickImages shows the system's photo picker for Pick; the
	// platform answers with EventPick carrying Count.
	ItemPickImages
	// ItemAppearance shows the app in Appearance whatever the system's,
	// native chrome included.
	ItemAppearance
	// ItemPopupMenu shows Menu as the platform's context menu at Point,
	// in the root's points; desktops report a chosen item with EventMenu
	// and answer with EventMenuClosed carrying Count.
	ItemPopupMenu
	// ItemQuit ends the app: desktops close the window and return from
	// Run, Android finishes its activity; iOS and the web ignore it.
	ItemQuit
	// ItemBackPreview answers EventBackStarted with the screen a back now
	// pops (ID), for the platform to move it with the gesture; 0 when back
	// goes elsewhere first or nothing pops.
	ItemBackPreview
	// ItemHistory is how many entries back can go through (Count): the
	// screens above each stack's first and the modals shown. The web
	// keeps that many browser history entries; others ignore it.
	ItemHistory
)

// MountItem is one change to the native views. Kind says which fields
// are set; the large groups are pointers so items stay small.
type MountItem struct {
	Kind   ItemKind
	ID     NodeID
	View   ViewKind // ItemCreate
	Parent NodeID   // ItemInsert
	Before NodeID   // ItemInsert
	Frame  Bounds   // ItemFrame
	Size   Size     // ItemScrollContent
	Value  string   // ItemInputValue
	Count  uint32   // ItemInputValue
	// Point and Animated are ItemScrollTo's.
	Point    Point
	Animated bool
	// Clipboard is what an ItemReadClipboard reads.
	Clipboard ClipboardRead
	// Appearance is ItemAppearance's.
	Appearance Appearance
	Events     EventMask

	ViewStyle *ViewStyle
	Text      *TextContent
	Input     *InputConfig
	ViewProps *ViewProps
	Scroll    *ScrollConfig
	Image     *ImageConfig
	Modal     *ModalConfig
	Alert     *AlertConfig
	Drawing   *Drawing
	Shader    *Shader
	// Keys are ItemKeymap's bindings, oldest first; see FindKey.
	Keys   []KeyBinding
	Window *WindowCommand
	Menu   []MenuEntry
	Web    *WebConfig
	// WebCommand is ItemWebCommand's.
	WebCommand WebCommand
	Video      *VideoConfig
	Time       time.Duration // ItemVideoSeek
	Native     *NativeConfig
	// Data is ItemNativeCommand's argument.
	Data      any
	Animation []AnimationOp
	Zoom      *ZoomRequest
	// Selection is ItemSelection's range.
	Selection TextRange
	// Sticky are ItemStickyCells' cells.
	Sticky    []StickyCell
	StatusBar *StatusBarConfig
	Screen    *ScreenConfig
	// Pick is ItemPickImages' request.
	Pick *PickRequest
}

// StickyCell is a sticky list cell and its travel, in its scroll view's
// main-axis offsets: from Start it moves with the offset, as far as End
// (which may be infinite).
type StickyCell struct {
	Cell       NodeID
	Start, End Points
	// Node, when set, is the animated node the cell's inset from the
	// viewport's start follows, in place of Inset, which Start counts.
	Node  uint32
	Inset Points
}

// StickyShift is how far a sticky cell moves from its frame along the
// main axis at offset.
func (c StickyCell) StickyShift(offset Points) Points {
	return min(max(offset-c.Start, 0), max(c.End-c.Start, 0))
}

// StickyShift is c.StickyShift with the cell's inset the value its node
// has, as backends place a cell bound to an animated node. A nil
// animator keeps the inset c was laid out with.
func (a *Animator) StickyShift(c StickyCell, offset Points) Points {
	if c.Node != 0 && a != nil {
		if v, _, ok := a.Value(c.Node); ok {
			offset += Points(v) - c.Inset
		}
	}
	return c.StickyShift(offset)
}

// CheckCommits has every commit compare the whole tree again after it and
// panic if that finds changes its marks missed. Tests set it; it doubles
// what commits cost.
var CheckCommits bool

// MountBatch is one commit's changes, applied in order on the main thread.
// Root is the view the platform shows in its surface.
type MountBatch struct {
	Root  NodeID
	Items []MountItem
	// LayoutAnimation, when set, animates the batch's frame changes,
	// insertions and removals on the platform (Scope.AnimateLayout).
	LayoutAnimation *LayoutAnimation
	// Sent is when the engine handed the batch to the platform, for the
	// performance monitor's latency reading; the zero time when no monitor
	// measures.
	Sent time.Time
}

// ViewStyle is everything a platform draws for a view itself.
type ViewStyle struct {
	Background Color
	// CornerRadius rounds every corner, unless Corners is set: then each
	// corner has its own radius. Platforms fit radii that do not fit
	// with Corners.Fit.
	CornerRadius Points
	Corners      Corners
	// ContinuousCorners eases the corners into the edges (RoundedRect)
	// instead of drawing quarter circles.
	ContinuousCorners bool
	// BorderWidth and BorderColor are the border on every side, unless
	// Borders is set: then each side has its width there and its color
	// in BorderColors.
	BorderWidth  Points
	BorderColor  Color
	Borders      Edges
	BorderColors EdgeColors
	// BorderStyle dashes or dots the border; 0 is solid.
	BorderStyle BorderStyle
	Outline     Outline
	Opacity     float32
	// Clip clips children to the bounds and corners.
	Clip         bool
	ShadowColor  Color
	ShadowOffset Point
	ShadowRadius Points
	// TintColor recolors an image's opaque pixels; 0 keeps its colors.
	TintColor Color
	// Cursor is the pointer's look over the view; Auto inherits the
	// nearest ancestor's, and a pressable view's is a pointer on the web.
	Cursor Cursor
	// Transform moves, scales and rotates the view around its transform
	// origin without moving its children's frames; hit testing follows
	// it. The core has resolved its percentages.
	Transform Transform
	// Origin is where the transform acts, from the view's center.
	Origin Point
	// BackfaceHidden hides the view while its rotations turn its back to
	// the viewer.
	BackfaceHidden bool
	// ChildPerspective, when set, is the depth children's rotations out
	// of the plane are seen from, at the view's center (ChildPerspective
	// the function).
	ChildPerspective Points
	BlendMode        BlendMode
	// Material and Glass are how iOS draws a backdrop blur.
	Material BackdropMaterial
	Glass    GlassEffect
	// Effects are shadows, gradients, filters and masks, nil without any.
	Effects *Effects
	// Transition, when it has a duration, animates the change from the
	// previous ViewStyle to this one.
	Transition Transition
	// Animation runs keyframes over the view while it is set; a change
	// starts it again.
	Animation KeyframeAnimation
	// Hidden hides the view and its children (display none, visibility
	// hidden).
	Hidden bool
}

// Radii returns the radius of each corner, fitted to size.
func (vs *ViewStyle) Radii(size Size) Corners {
	c := vs.Corners
	if c == (Corners{}) {
		c = AllCorners(vs.CornerRadius)
	}
	return c.Fit(size)
}

// Rounded reports whether some corner is rounded.
func (vs *ViewStyle) Rounded() bool { return vs.CornerRadius > 0 || vs.Corners != (Corners{}) }

// Matrix returns the view's transform for a box of size, around its
// origin.
func (vs *ViewStyle) Matrix(size Size) Matrix3D {
	return vs.Transform.Matrix(Point{X: size.Width/2 + vs.Origin.X, Y: size.Height/2 + vs.Origin.Y})
}

// ImageConfig is an image view's source and look.
type ImageConfig struct {
	Source       ImageSource
	Placeholder  ImageSource
	ResizeMode   ResizeMode
	BlurRadius   Points
	FadeDuration time.Duration
	// Position is the style's ObjectPosition: where the fitted image sits
	// (ResizeMode.Fit). It travels with the image, not the ViewStyle.
	Position ObjectPosition
}

// ModalConfig is how a modal appears and goes.
type ModalConfig struct {
	Animation ModalAnimation
	// Presentation, NoCloseOnSwipe and Orientations are iOS's. A sheet
	// reports its size with EventModalSize, its swipes with
	// EventRequestClose.
	Presentation   ModalPresentation
	NoCloseOnSwipe bool
	Orientations   Orientation
}

// ScreenConfig is a screen's system header, which the platform draws
// over the top of the screen's view, HeaderHeight tall.
type ScreenConfig struct {
	Title string
	// HeaderHidden hides the header; HeaderHeight is then 0.
	HeaderHidden      bool
	HeaderTransparent bool
	LargeTitle        bool
	// CanGoBack says a screen is below; BackHidden hides the back button
	// to it, and BackTitle is its label.
	CanGoBack  bool
	BackHidden bool
	BackTitle  string
	// NoPopOnSwipe turns the swipe back off, as NoPopOnBack does with the
	// back button's popping: a press still reports EventBack.
	NoPopOnSwipe bool
	NoPopOnBack  bool
	// Background, TitleColor and TintColor are the header's colors; 0
	// keeps the platform's.
	Background, TitleColor, TintColor Color
	// Left, Right and TitleView are the header views, each a layout root
	// at its content's size that no view holds, or 0.
	Left, Right, TitleView NodeID
	// RightSystemItem, with a Right view, is the UIKit button iOS shows
	// in Right's place; a press on it presses Right's first view that
	// takes presses.
	RightSystemItem HeaderSystemItem
	// HeaderHeight is the area the header covers from the screen's top:
	// the bar and the part of the status bar the screen reaches under.
	HeaderHeight Points
}

// HeaderSystemItem is a button iOS draws in a navigation bar in UIKit's
// own look, its symbol or label.
type HeaderSystemItem uint8

const (
	HeaderItemAdd HeaderSystemItem = iota + 1
	HeaderItemDone
	HeaderItemClose
	HeaderItemEdit
	HeaderItemCompose
	HeaderItemSearch
	HeaderItemShare
	HeaderItemTrash
	HeaderItemSave
	HeaderItemCancel
)

// TextContent is a text view's content, laid out by the engine.
type TextContent struct {
	// Text is the plain text, for accessibility.
	Text string
	// Layout is the platform's TextLayout, ready to draw. The platform
	// owns it from now on and releases the one it replaces.
	Layout TextLayout
	// Content is where Layout goes in the view: inside padding and border.
	Content Bounds
	// Style and Spans are the text's resolved style and its spans', as
	// it was prepared, for tests to read; platforms draw Layout and read
	// Spans' links for presses.
	Style TextStyle
	Spans []TextSpan
}

// InputConfig is a text field's look and behavior.
type InputConfig struct {
	Style        TextStyle
	Placeholder  string
	Masked       bool
	Readonly     bool
	Disabled     bool
	KeyboardType KeyboardType
	// Content is where the text goes in the view: inside padding and border.
	Content Bounds
	// Accessory is the InputAccessoryView that rides above the keyboard
	// while the field has focus, or 0.
	Accessory NodeID
	// Multiline takes several lines in a multi-line control.
	Multiline bool
	// MaxLength caps what the user enters, in UTF-16 units; 0 sets none.
	MaxLength int
	// SubmitBehavior is as set; platforms read Submit.
	SubmitBehavior SubmitBehavior
	ReturnKey      ReturnKeyType
	AutoCapitalize AutoCapitalize
	NoAutoCorrect  bool
	AutoComplete   AutoComplete
	// KeyboardAppearance is iOS's.
	KeyboardAppearance KeyboardAppearance
	// PlaceholderColor is 0 for the platform's: the text color at half
	// strength where the platform styles placeholders itself.
	PlaceholderColor Color
	// SelectionColor colors the caret and the selection; 0 is the
	// platform's.
	SelectionColor Color
	HideCaret      bool
	// PasteBehavior says whether a paste inserts its text.
	PasteBehavior PasteBehavior
}

// KeyboardType picks a software keyboard.
type KeyboardType uint8

const (
	KeyboardDefault KeyboardType = iota
	KeyboardEmail
	KeyboardNumber
	KeyboardDecimal
	KeyboardPhone
	KeyboardURL
)

// ViewProps is a view's accessibility and identity.
type ViewProps struct {
	Role  Role
	Label string
	// Hint says what activating the element does.
	Hint     string
	Disabled bool
	// Selected, Checked and Indeterminate (mixed) are reported to
	// assistive technology, and Expanded when Expandable.
	Selected      bool
	Checked       bool
	Indeterminate bool
	Expandable    bool
	Expanded      bool
	// Value is a range's or a textual element's value.
	Value AccessibilityValue
	// Accessible makes the view one element, or none.
	Accessible Accessible
	// ElementsHidden hides the view and its subtree from assistive
	// technology.
	ElementsHidden bool
	LiveRegion     LiveRegion
	// Actions are the actions offered to assistive technology, reported
	// with EventAccessibilityAction. The list is shared and never changed,
	// so ViewProps stay comparable; read it with ActionList.
	Actions *[]AccessibilityAction
	TestID  string
	// WindowDrag, PanActivation and selections are decided on the main
	// thread.
	WindowDrag    WindowDrag
	PanActivation PanActivation
	// PanScrollHandoff: the view's pan shares a drag with the scroll view
	// inside it where the drag starts, on iOS (View.PanScrollHandoff).
	PanScrollHandoff bool
	// SelectionArea is a view's, Selectable and SelectionColor a text's.
	SelectionArea  bool
	Selectable     Selectable
	SelectionColor Color
	// NoFocus keeps a pressable view out of the Tab stops.
	NoFocus bool
	// Appearance is the one its subtree forces (Scope.SetSubtreeAppearance),
	// for the platform's views in it; AppearanceSystem follows the window.
	Appearance Appearance
	// PointerEvents, HitSlop and DelayLongPress are the view's; a zero
	// DelayLongPress is the platform's 500 ms.
	PointerEvents  PointerEvents
	HitSlop        Edges
	DelayLongPress time.Duration
	// Ripple is Android's press ripple, nil for none. It is shared and
	// never changed, as Actions is.
	Ripple *Ripple
}

// Ripple is Android's ripple over a view while it is pressed.
type Ripple struct {
	// Color is the ripple's; 0 takes the theme's.
	Color Color
	// Borderless spreads it past the view's shape.
	Borderless bool
	// Radius is 0 for the platform's.
	Radius Points
	// Foreground draws it over the view's content, else behind it.
	Foreground bool
}

// Role is a view's accessibility role.
type Role uint8

const (
	RoleNone Role = iota
	RoleButton
	RoleLink
	RoleHeader
	RoleImage
	RoleCheckbox
	RoleSwitch
	RoleRadio
	RoleTab
	RoleText
	RoleTextField
	RoleRadioGroup
	RoleTabList
	RoleMenu
	RoleMenuItem
	RoleDialog
	RoleAlert
	// RoleAdjustable is a slider: its Value is a range that the
	// increment and decrement actions adjust.
	RoleAdjustable
	// RoleProgressBar shows progress, its Value a range.
	RoleProgressBar
	// RoleTable is a table of the app's rows; RoleGrid one whose cells
	// the user selects and moves between. RoleRowGroup holds a header's,
	// a body's or a footer's rows, RoleRow a row's cells, and
	// RoleColumnHeader and RoleCell are a row's cells.
	RoleTable
	RoleGrid
	RoleRowGroup
	RoleRow
	RoleColumnHeader
	RoleCell
)

// ScrollConfig configures a scroll view.
type ScrollConfig struct {
	Horizontal bool
	// HideIndicator hides the scroll bar.
	HideIndicator bool
	Behavior      ScrollBehavior
	// HeaderInset is the part of Behavior.ContentInset's top that a
	// transparent or large system header covers, which a large title
	// collapses into as the content scrolls under it.
	HeaderInset Points
	// Refreshable says the app listens for EventRefresh: pulling the
	// content down past its top refreshes. Refreshing shows the spinner;
	// a config after EventRefresh says whether the app keeps it.
	Refreshable bool
	Refreshing  bool
	// MinZoomScale and MaxZoomScale bound pinches (1 and at least
	// MinZoomScale); the content zooms when Zooms. ZoomScale is the app's
	// scale, which the platform zooms to when it changes.
	MinZoomScale, MaxZoomScale float32
	ZoomScale                  float32
	// NoBouncesZoom stops pinches at the limits; NoPinchZoom ignores
	// pinches; CenterContent centers content smaller than the viewport.
	NoBouncesZoom bool
	NoPinchZoom   bool
	CenterContent bool
	// Edges are the effects at its sides, in the order of Edges: top,
	// right, bottom, left.
	Edges [4]ScrollEdge
}

// TextEngine prepares text for layout. The engine calls it on its own
// goroutine, so implementations must not touch main-thread-only objects.
type TextEngine interface {
	// Prepare shapes text once; the result measures it at any number of
	// widths and lays it out at its final size.
	Prepare(text *AttributedText) PreparedText
}

// PreparedText is text a TextEngine shaped.
type PreparedText interface {
	// Measure returns the size of the text wrapped at maxWidth (Auto: no
	// wrapping).
	Measure(maxWidth Points) Size
	// Layout returns the text laid out in size, for the main thread to
	// draw.
	Layout(size Size) TextLayout
	// Release frees what the prepared text holds.
	Release()
}

// TextLayout is a platform's laid-out text, drawn on the main thread and
// released there when replaced or deleted.
type TextLayout interface {
	Release()
}

// Selectable says whether the user can select a text.
type Selectable uint8

const (
	// SelectableAuto selects the text with its selection area when it is
	// in one. Elsewhere only the web selects it, on desktops, as a page's
	// text.
	SelectableAuto Selectable = iota
	// SelectableOn selects the text, alone when it is in no area.
	SelectableOn
	// SelectableOff keeps the text out of selections, its area's too.
	SelectableOff
)

// ImageSource is where an image comes from: a URI (https, file path,
// file:, data:) or bytes, such as a file embedded with go:embed.
type ImageSource struct {
	URI  string
	Data []byte
	// Size is the image's size in points, when known up front.
	Size Size
	// Scales are the pixel densities an "asset:" image ships in (a.png,
	// a@2x.png, ...): the core mounts the one for the screen's pixel
	// ratio. Empty for a single file.
	Scales ImageScales
}

// Density returns the pixels a point of the image s names has: the
// density an "asset:" file's name gives (a@2x.png is 2), else 1.
func (s ImageSource) Density() float32 {
	if !strings.HasPrefix(s.URI, "asset:") {
		return 1
	}
	name := s.URI[strings.LastIndexByte(s.URI, '/')+1:]
	if dot := strings.LastIndexByte(name, '.'); dot > 0 {
		name = name[:dot]
	}
	at := strings.LastIndexByte(name, '@')
	if at < 0 || !strings.HasSuffix(name, "x") {
		return 1
	}
	d, err := strconv.ParseFloat(name[at+1:len(name)-1], 32)
	if err != nil || d <= 0 {
		return 1
	}
	return float32(d)
}

// Key returns a string naming the image s loads, for caches: its URI, or
// where its bytes are.
func (s ImageSource) Key() string {
	if len(s.Data) > 0 {
		return fmt.Sprintf("data:%p:%d", &s.Data[0], len(s.Data))
	}
	return s.URI
}

// Same reports whether s and o load the same image: inline data must
// be the same bytes, not equal ones.
func (s ImageSource) Same(o ImageSource) bool {
	return s.URI == o.URI && s.Size == o.Size && s.Scales == o.Scales && len(s.Data) == len(o.Data) &&
		(len(s.Data) == 0 || &s.Data[0] == &o.Data[0])
}

// ResizeMode fits an image or a video into its view. The zero value is
// the element's default: Cover for an image, Contain for a video, as in
// NSH. Platforms receive the resolved mode.
type ResizeMode uint8

const (
	// Cover fills the view, cropping what overflows.
	Cover ResizeMode = iota + 1
	// Contain fits the whole image, leaving space.
	Contain
	// Stretch fills the view, distorting the image.
	Stretch
	// Center keeps the image's natural size, cropped where it is larger
	// than the view (CSS's object-fit: none).
	Center
	// ScaleDown keeps the natural size, shrunk to fit where it is larger
	// than the view (CSS's object-fit: scale-down, React Native's center).
	ScaleDown
)

// ModalPresentation is how a modal covers the screen on iOS.
type ModalPresentation uint8

const (
	// ModalFullScreen covers the whole screen with the modal's own look.
	ModalFullScreen ModalPresentation = iota
	// ModalPageSheet is the system's sheet, which leaves the content
	// behind it in sight.
	ModalPageSheet
	// ModalFormSheet is a smaller sheet, centered on large screens.
	ModalFormSheet
)

// Orientation is a set of interface orientations.
type Orientation uint8

const (
	OrientationPortrait Orientation = 1 << iota
	OrientationPortraitUpsideDown
	OrientationLandscapeLeft
	OrientationLandscapeRight

	OrientationLandscape = OrientationLandscapeLeft | OrientationLandscapeRight
)

// ModalAnimation is how a modal appears.
type ModalAnimation uint8

const (
	ModalSlide ModalAnimation = iota
	ModalFade
	ModalNone
)

// Same reports whether a and b show the same image the same way.
func (a ImageConfig) Same(b ImageConfig) bool {
	return a.Source.Same(b.Source) && a.Placeholder.Same(b.Placeholder) && a.ResizeMode == b.ResizeMode &&
		a.BlurRadius == b.BlurRadius && a.FadeDuration == b.FadeDuration && a.Position == b.Position
}

// ObjectPosition is where an image sits in its view once its ResizeMode
// has sized it, as CSS's object-position places it. The parts of a
// covering image that overflow are cropped. The zero value centers it.
type ObjectPosition struct {
	X, Y PositionAxis
}

// PositionAxis is an ObjectPosition along one axis: Offset from the start
// edge (left, top), or from the end edge (right, bottom) when FromEnd.
// A percentage is of the free space, the view's size less the image's, so
// Pct(0) is the start, Pct(50) the center and Pct(100) the end; an auto
// Offset, the zero value, is the center.
type PositionAxis struct {
	Offset  Length
	FromEnd bool
}

// Resolve returns where content starts along the axis, from the start
// edge, given the free space: the box's size less the content's.
func (a PositionAxis) Resolve(free Points) Points {
	var d Points
	switch a.Offset.unit {
	case unitPoints:
		d = Points(a.Offset.value)
	case unitPercent:
		d = free * Points(a.Offset.value) / 100
	default:
		d = free / 2
	}
	if a.FromEnd {
		return free - d
	}
	return d
}

// Place returns the top left corner of content of the given size placed
// in a box of size.
func (p ObjectPosition) Place(box, content Size) Point {
	return Point{X: p.X.Resolve(box.Width - content.Width), Y: p.Y.Resolve(box.Height - content.Height)}
}

// Fit returns where an image of natural size, in points, goes in a view
// of size box: scaled as m scales it (Center keeps the natural size, as
// CSS's none; ScaleDown shrinks an image larger than the box, as CSS's
// scale-down), then placed at pos. The rectangle may reach outside the
// box, which crops it. An image without a size fills the box. Every
// backend fits images and videos with it, as NSH's ResizeMode::fit.
func (m ResizeMode) Fit(natural, box Size, pos ObjectPosition) Bounds {
	nw, nh := natural.Width, natural.Height
	if nw <= 0 || nh <= 0 || m == Stretch {
		return Bounds{Size: box}
	}
	contain := min(box.Width/nw, box.Height/nh)
	var scale Points
	switch m {
	case Contain:
		scale = contain
	case Center:
		scale = 1
	case ScaleDown:
		scale = min(contain, 1)
	default: // Cover
		scale = max(box.Width/nw, box.Height/nh)
	}
	content := Size{Width: nw * scale, Height: nh * scale}
	return Bounds{Origin: pos.Place(box, content), Size: content}
}

// ImageScaleValues are the pixel densities an image asset can ship in, as
// its file name's suffix says ("@1.5x" to "@4x"; the file without one is
// 1x).
var ImageScaleValues = [...]float32{1, 1.5, 2, 3, 4}

// ImageScales is a set of ImageScaleValues: the variants an image asset
// ships in.
type ImageScales uint8

// Pick returns the scale to load at ratio, as React Native picks: the
// smallest at least the ratio, else the largest; 0 for an empty set.
func (set ImageScales) Pick(ratio float32) float32 {
	var pick float32
	for i, s := range ImageScaleValues {
		if set&(1<<i) != 0 {
			pick = s
			if s >= ratio {
				break
			}
		}
	}
	return pick
}

// StatusBarStyle is the color of a phone's status bar content.
type StatusBarStyle uint8

const (
	// StatusBarInherit keeps the style of the layers below.
	StatusBarInherit StatusBarStyle = iota
	// StatusBarAuto follows the appearance: dark content on light.
	StatusBarAuto
	StatusBarDark
	StatusBarLight
)

// StatusBarConfig is the status bar the platform shows (ItemStatusBar).
type StatusBarConfig struct {
	// Style is never StatusBarInherit.
	Style    StatusBarStyle
	Hidden   bool
	Animated bool
}

// VideoConfig is everything a video plays with (ItemVideo), with the
// defaults applied: Volume in 0 to 1, Rate above 0, a ResizeMode and
// ProgressInterval at least 16 milliseconds.
type VideoConfig struct {
	Source           string
	Paused           bool
	Muted            bool
	Volume           float32
	Rate             float32
	Repeat           bool
	ResizeMode       ResizeMode
	Controls         bool
	ProgressInterval time.Duration
	AudioSession     AudioSession
}

// AudioSession is what a video's sound does to the rest of the device's
// audio while it plays, as NSH's audioSession.
type AudioSession uint8

const (
	// AudioSessionDefault leaves the app's audio settings to decide: on
	// iOS the device's silent mode silences it.
	AudioSessionDefault AudioSession = iota
	// AudioSessionPlayback plays in iOS's silent mode as speech, pausing
	// other apps' audio until it pauses, ends or goes; Android holds the
	// audio focus while it plays.
	AudioSessionPlayback
	// AudioSessionMixWithOthers plays in iOS's silent mode as speech, with
	// other apps' audio going on.
	AudioSessionMixWithOthers
)
