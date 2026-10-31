package core

import "github.com/oneo-me/nsgo/internal/driver"

// The values apps share with the platforms. core/driver, the platform
// protocol, defines them; core aliases them so apps import core alone.

type (
	// AndroidEventMapper is implemented by an Android component that
	// receives the listener callbacks of its view itself.
	AndroidEventMapper = driver.AndroidEventMapper
	// AndroidPropMapper is implemented by an Android component that chooses
	// the setter calls a prop makes, in order.
	AndroidPropMapper = driver.AndroidPropMapper
	// AndroidViewProp is one setter call on a component's view:
	// set<Name>(Value), Name's first letter upper-cased.
	AndroidViewProp = driver.AndroidViewProp
	// AccessibilityAction is an action assistive technology offers on an
	// element; OnAccessibilityAction receives its Name.
	AccessibilityAction = driver.AccessibilityAction
	// AccessibilityValue is a range's or a textual element's value
	// (accessibilityValue).
	AccessibilityValue = driver.AccessibilityValue
	// Accessible says whether a view is one element for assistive technology.
	Accessible = driver.Accessible
	// AlertAction is one button of an alert.
	AlertAction = driver.AlertAction
	// AlertConfig is a native alert: a title, a message and its buttons.
	AlertConfig = driver.AlertConfig
	// AlertRole says what an alert's button does.
	AlertRole = driver.AlertRole
	// Animation is a description of how values move: Timing, Spring and Decay
	// move one value, and Sequence, Parallel, Stagger, Delay and Loop compose
	// them.
	Animation = driver.Animation
	// AnimationKind is what an Animation does.
	AnimationKind = driver.AnimationKind
	// AutoCapitalize is a field's automatic capitalization; sentences are the
	// default.
	AutoCapitalize = driver.AutoCapitalize
	// AutoComplete says what a field holds, for the system's autofill and
	// suggestions: React Native's autoComplete, which iOS maps to its
	// textContentType.
	AutoComplete = driver.AutoComplete
	// AudioSession is what a video's sound does to the rest of the
	// device's audio while it plays.
	AudioSession = driver.AudioSession
	// AnimatedPropSetter is implemented by a native component that takes
	// its animated props as numbers and colors.
	AnimatedPropSetter = driver.AnimatedPropSetter
	// Bounds is a rectangle: its origin relative to the parent's and its size.
	Bounds = driver.Bounds
	// Color is an sRGB color packed as 0xRRGGBBAA, the wire form of mount
	// items.
	Color = driver.Color
	// Cursor is the pointer's look over a view (cursor).
	Cursor = driver.Cursor
	// Curve is an easing curve: CSS's cubic Bézier from (0,0) to (1,1) through
	// two control points.
	Curve = driver.Curve
	// DecayConfig slows a value from Velocity (units per second; the value's
	// current velocity when 0) by Deceleration per millisecond (0.998 when 0).
	DecayConfig = driver.DecayConfig
	// DrawKind is a drawing operation.
	DrawKind = driver.DrawKind
	// DrawOp is one operation; Kind says which fields it uses.
	DrawOp = driver.DrawOp
	// Drawing is a recording of 2D drawing operations, modeled on the web's
	// CanvasRenderingContext2D: a current path, fill and stroke settings, and a
	// transform, saved and restored as a stack.
	Drawing = driver.Drawing
	// EaseMode applies a curve as is, mirrored, or both halves.
	EaseMode = driver.EaseMode
	// Easing is how a timing animation or an interpolation segment progresses:
	// a cubic Bézier, or one of React Native's curves.
	Easing = driver.Easing
	// EasingKind is an easing's shape.
	EasingKind = driver.EasingKind
	// StepPosition is where a steps easing jumps, as CSS's step positions.
	StepPosition = driver.StepPosition
	// EdgeColors is a color for each side of a box.
	EdgeColors = driver.EdgeColors
	// Edges holds a length per side.
	Edges = driver.Edges
	// EllipsizeMode is where a text cut by its line limit loses the rest.
	EllipsizeMode = driver.EllipsizeMode
	// Environment is what the platform reports about where the app runs.
	Environment = driver.Environment
	// Extrapolate is what an interpolation does outside its input range; 0
	// clamps.
	Extrapolate = driver.Extrapolate
	// Font is what a text engine needs to pick a font.
	Font = driver.Font
	// FontWeight is a CSS font weight from 100 to 900; 0 is unset.
	FontWeight = driver.FontWeight
	// HeaderSystemItem is a button iOS draws in a navigation bar in
	// UIKit's own look, its symbol or label.
	HeaderSystemItem = driver.HeaderSystemItem
	// FontVariant is a set of OpenType features, as CSS's
	// font-variant-caps and font-variant-numeric set them; the values
	// combine.
	FontVariant = driver.FontVariant
	// GesturePhase is where a gesture is in its course.
	GesturePhase = driver.GesturePhase
	// CanvasGradient is a linear, radial or conic color gradient that a
	// Drawing fills and strokes with.
	CanvasGradient = driver.CanvasGradient
	// CanvasNode is a named part of a drawing, which assistive technology
	// and tests see as a child of the canvas.
	CanvasNode = driver.CanvasNode
	// CompositeOp is how drawing mixes with what a canvas holds, as the
	// web's globalCompositeOperation.
	CompositeOp = driver.CompositeOp
	// DrawArg is an argument of a DrawOp that Drawing.Bind binds to an
	// animated node.
	DrawArg = driver.DrawArg
	// DrawSlot binds an argument of a drawing's operation to an animated
	// node.
	DrawSlot = driver.DrawSlot
	// AnimatedArg is an animated node a drawing binds an argument to.
	AnimatedArg = driver.AnimatedArg
	// FillRule is how a fill or a clip decides what is inside a path.
	FillRule = driver.FillRule
	// LineDash is the lengths of a stroke's dashes and gaps in turn.
	LineDash = driver.LineDash
	// Path is a path to fill, stroke or clip with, as the web's Path2D.
	Path = driver.Path
	// Paint is what Fill and Stroke paint with: a color, nothing or the
	// text color. The zero Paint is unset, so the property inherits.
	Paint = driver.Paint
	// PaintStyle is the SVG paint properties a canvas view inherits.
	PaintStyle = driver.PaintStyle
	// ViewPaint is an op of a drawing whose value is its view's paint.
	ViewPaint = driver.ViewPaint
	// ViewPaintKind is the paint property a ViewPaint takes.
	ViewPaintKind = driver.ViewPaintKind
	// Caret is a caret position in a text: a UTF-16 offset and the side it
	// is on.
	Caret = driver.Caret
	// Affinity says which line a caret offset is on where one offset is two
	// places on screen: the end of a wrapped line and the start of the
	// next.
	Affinity = driver.Affinity
	// TextBaseline is the line of a canvas text that the point it is drawn
	// at sits on.
	TextBaseline = driver.TextBaseline
	// TextMetrics are a text's measurements in a font.
	TextMetrics = driver.TextMetrics
	// GradientSpread is how a CanvasGradient paints beyond offsets 0 and 1.
	GradientSpread = driver.GradientSpread
	// GradientStop is a color at an offset of a Gradient.
	GradientStop = driver.GradientStop
	// Idiom is the kind of device, which decides how components present.
	Idiom = driver.Idiom
	// ImageScales is a set of ImageScaleValues: the variants an image asset
	// ships in.
	ImageScales = driver.ImageScales
	// ImageSource is where an image comes from: a URI (https, file path, file:,
	// data:) or bytes, such as a file embedded with go:embed.
	ImageSource = driver.ImageSource
	// Interpolation maps a node through piecewise-linear ranges.
	Interpolation = driver.Interpolation
	// KeyboardAppearance is the look of iOS's keyboard; the default follows the
	// app's appearance.
	KeyboardAppearance = driver.KeyboardAppearance
	// KeyboardDismissMode is whether dragging a scroll view dismisses the
	// software keyboard.
	KeyboardDismissMode = driver.KeyboardDismissMode
	// KeyboardTaps is what a tap in a scroll view does while the software
	// keyboard shows.
	KeyboardTaps = driver.KeyboardTaps
	// KeyboardType picks a software keyboard.
	KeyboardType = driver.KeyboardType
	// LayoutAnimation animates the frame changes, insertions and removals of
	// one commit on the platform, as React Native's LayoutAnimation does.
	LayoutAnimation = driver.LayoutAnimation
	// LayoutCurve is how a layout change progresses.
	LayoutCurve = driver.LayoutCurve
	// LayoutProperty is what appearing and disappearing views animate.
	LayoutProperty = driver.LayoutProperty
	// LayoutTransition is how one kind of change animates.
	LayoutTransition = driver.LayoutTransition
	// Length is a style's length.
	Length = driver.Length
	// LineCap ends open strokes.
	LineCap = driver.LineCap
	// LineJoin joins stroke segments.
	LineJoin = driver.LineJoin
	// LiveRegion says how screen readers announce changes of a view's label and
	// text (accessibilityLiveRegion).
	LiveRegion = driver.LiveRegion
	// MenuEntry is a MenuItem as an ItemMenu carries it: the item's handler
	// replaced by an ID the platform reports with EventMenu (0: none), its
	// shortcut normalized and its role's label filled in.
	MenuEntry = driver.MenuEntry
	// MenuRole is a standard menu item the platform carries out, labelling it
	// and giving it its usual shortcut when the item has none of its own.
	MenuRole = driver.MenuRole
	// ModalAnimation is how a modal appears.
	ModalAnimation = driver.ModalAnimation
	// ModalPresentation is how a modal covers the screen on iOS.
	ModalPresentation = driver.ModalPresentation
	// Modifiers are the modifier keys held during an event.
	Modifiers = driver.Modifiers
	// NativeComponent is one Native element's platform view.
	NativeComponent = driver.NativeComponent
	// NativeComponents are the components an app registers with its platform,
	// by name.
	NativeComponents = driver.NativeComponents
	// NativeContext is how a component talks back to its element.
	NativeContext = driver.NativeContext
	// NativeFactory makes a component's view for one Native element, on the
	// platform's main thread; nil leaves the box empty.
	NativeFactory = driver.NativeFactory
	// NodeID identifies a view of one tree.
	NodeID = driver.NodeID
	// OS names the operating system the app runs on.
	OS = driver.OS
	// ObjectPosition is where an image sits in its view once its ResizeMode has
	// sized it, as CSS's object-position places it.
	ObjectPosition = driver.ObjectPosition
	// Orientation is a set of interface orientations.
	Orientation = driver.Orientation
	// PanActivation decides on the main thread when a drag on a view becomes
	// its pan, so it can share touches with the scroll views around it, as
	// react-native-gesture-handler's offsets do.
	PanActivation = driver.PanActivation
	// PasteBehavior says whether an input inserts the text the user pastes.
	PasteBehavior = driver.PasteBehavior
	// PasteEvent is what the user pasted into an input, read from the clipboard
	// as they pasted: its plain text (empty without any), files with their
	// bytes, such as an image copied in another app, and the file: URLs of
	// local files copied in a file manager.
	PasteEvent = driver.PasteEvent
	// PastedFile is a file the clipboard held: its name when it has one, its
	// MIME type and its bytes.
	PastedFile = driver.PastedFile
	// Point is a position in points.
	Point = driver.Point
	// PointerEvents says which of a view and its children take touches and the
	// pointer.
	PointerEvents = driver.PointerEvents
	// Points is a length in platform-independent points (iOS points, Android
	// dp, CSS pixels at a zoom of 1).
	Points = driver.Points
	// PositionAxis is an ObjectPosition along one axis: Offset from the start
	// edge (left, top), or from the end edge (right, bottom) when FromEnd.
	PositionAxis = driver.PositionAxis
	// ResizeMode fits an image or a video into its view.
	ResizeMode = driver.ResizeMode
	// ReturnKeyType labels the software keyboard's return key.
	ReturnKeyType = driver.ReturnKeyType
	// Ripple is Android's ripple over a pressed view (AndroidViewProps).
	Ripple = driver.Ripple
	// Role is a view's accessibility role.
	Role = driver.Role
	// SVG is a parsed SVG document: vector shapes that stay sharp at any size.
	SVG = driver.SVG
	// ScrollBehavior is how a scroll view scrolls, as NSH's Scrollable props
	// (React Native's ScrollView props) say: Scroll's Behavior.
	ScrollBehavior = driver.ScrollBehavior
	// Selectable says whether the user can select a text.
	Selectable = driver.Selectable
	// Size is a width and a height in points.
	Size = driver.Size
	// SnapAlignment is where a snapped item sits in the viewport.
	SnapAlignment = driver.SnapAlignment
	// SpringConfig is a damped spring's: Stiffness 100, Damping 10 and Mass 1
	// when 0.
	SpringConfig = driver.SpringConfig
	// StatusBarStyle is the color of a phone's status bar content.
	StatusBarStyle = driver.StatusBarStyle
	// StickyCell is a sticky list cell and its travel, in its scroll view's
	// main-axis offsets: from Start it moves with the offset, as far as End
	// (which may be infinite).
	StickyCell = driver.StickyCell
	// StorageChange is a change to the entries, for the backend to keep.
	StorageChange = driver.StorageChange
	// StorageOp says what a StorageChange does.
	StorageOp = driver.StorageOp
	// SubmitBehavior is what Return does in a text field.
	SubmitBehavior = driver.SubmitBehavior
	// TextAlign aligns lines of text.
	TextAlign = driver.TextAlign
	// TextDecoration draws lines on text; the values combine.
	TextDecoration = driver.TextDecoration
	// TextDecorationStyle is how a text's decoration lines are drawn; the zero
	// value is solid.
	TextDecorationStyle = driver.TextDecorationStyle
	// TextLine is a line of laid-out text, as Text.OnTextLayout reports it.
	TextLine = driver.TextLine
	// TextRange is a selection in a text field, in UTF-16 units as the
	// platforms' text APIs count; Start equals End for a caret.
	TextRange = driver.TextRange
	// TimingConfig is a timing animation's: Duration (500 ms when 0) along
	// Easing, after Delay.
	TimingConfig = driver.TimingConfig
	// TitleBarStyle is whether the system draws the window's title bar.
	TitleBarStyle = driver.TitleBarStyle
	// Touch is one finger of a TouchEvent.
	Touch = driver.Touch
	// Transform moves, scales and rotates a view without changing its layout,
	// around its center: translate, then rotate, then scale.
	Transform = driver.Transform
	// Transition animates a view's look when its style changes: the platform
	// runs it on the main thread, with no work for the engine per frame.
	Transition = driver.Transition
	// TransitionProperty is a set of animatable properties.
	TransitionProperty = driver.TransitionProperty
	// VerticalAlign is where an attachment sits in its line, as CSS's
	// vertical-align places a replaced inline box, against the font of the text
	// around it.
	VerticalAlign = driver.VerticalAlign
	// WebNavigation is a web view's page and history.
	WebNavigation = driver.WebNavigation
	// WebSource is what a web view shows: the page at Src, or HTML, whose
	// relative links resolve against BaseURL, instead when it is set.
	WebSource = driver.WebSource
	// WindowAction is what an ItemWindow asks of the window.
	WindowAction = driver.WindowAction
	// WindowFrame is where a window moves and how large it becomes, in
	// points from the main screen's top left.
	WindowFrame = driver.WindowFrame
	// WindowLevel is where a window stacks among the others.
	WindowLevel = driver.WindowLevel
	// WindowControls is how a hidden title bar shows the system's window
	// controls: hidden, or macOS's traffic lights moved.
	WindowControls = driver.WindowControls
	// MenuBarMode says when Windows and Linux show the menu bar.
	MenuBarMode = driver.MenuBarMode
	// Appearance is the light or dark appearance an app asks for, or the
	// system's.
	Appearance = driver.Appearance
	// ClipboardImage is an image read from the clipboard: a PNG and its
	// size in pixels.
	ClipboardImage = driver.ClipboardImage
	// OSVersion is an operating system's version.
	OSVersion = driver.OSVersion
	// PickFormat says which formats picked images come in.
	PickFormat = driver.PickFormat
	// PickRequest is a pick as the platform carries it out, PickOptions
	// resolved.
	PickRequest = driver.PickRequest
	// PickEncoding is what a platform does with one picked image.
	PickEncoding = driver.PickEncoding
	// PickedFile is an image the user picked: its name, type, bytes, size
	// in pixels and modification time.
	PickedFile = driver.PickedFile
	// WindowCommand is an ItemWindow's request.
	WindowCommand = driver.WindowCommand
	// WindowDrag marks a view as a handle that moves the desktop window, or
	// keeps a view inside such a handle clickable.
	WindowDrag = driver.WindowDrag
	// WindowMaterial is a translucent material behind a desktop window's
	// content, where the app draws nothing opaque.
	WindowMaterial = driver.WindowMaterial
)

const (
	OSMacOS   = driver.OSMacOS
	OSIOS     = driver.OSIOS
	OSAndroid = driver.OSAndroid
	OSWindows = driver.OSWindows
	OSLinux   = driver.OSLinux
	OSWeb     = driver.OSWeb
)

const (
	IdiomPhone   = driver.IdiomPhone
	IdiomTablet  = driver.IdiomTablet
	IdiomDesktop = driver.IdiomDesktop
	IdiomMac     = driver.IdiomMac
	IdiomTV      = driver.IdiomTV
)

const (
	WindowLevelNormal   = driver.WindowLevelNormal
	WindowLevelFloating = driver.WindowLevelFloating
	WindowLevelDesktop  = driver.WindowLevelDesktop
)

const (
	MenuBarShown    = driver.MenuBarShown
	MenuBarAutoHide = driver.MenuBarAutoHide
)

const (
	AppearanceSystem = driver.AppearanceSystem
	AppearanceLight  = driver.AppearanceLight
	AppearanceDark   = driver.AppearanceDark
)

const (
	PickCompatible = driver.PickCompatible
	PickCurrent    = driver.PickCurrent
)

// MaxPicked is the most images one PickImages answers with.
const MaxPicked = driver.MaxPicked

// MaxClipboardImage is the most bytes of a PNG ReadClipboardImage gives.
const MaxClipboardImage = driver.MaxClipboardImage

const (
	SubmitBehaviorDefault       = driver.SubmitBehaviorDefault
	SubmitBehaviorSubmit        = driver.SubmitBehaviorSubmit
	SubmitBehaviorBlurAndSubmit = driver.SubmitBehaviorBlurAndSubmit
	SubmitBehaviorNewline       = driver.SubmitBehaviorNewline
)

const (
	ReturnKeyDefault = driver.ReturnKeyDefault
	ReturnKeyDone    = driver.ReturnKeyDone
	ReturnKeyGo      = driver.ReturnKeyGo
	ReturnKeyNext    = driver.ReturnKeyNext
	ReturnKeySearch  = driver.ReturnKeySearch
	ReturnKeySend    = driver.ReturnKeySend
)

const (
	AutoCapitalizeSentences  = driver.AutoCapitalizeSentences
	AutoCapitalizeNone       = driver.AutoCapitalizeNone
	AutoCapitalizeWords      = driver.AutoCapitalizeWords
	AutoCapitalizeCharacters = driver.AutoCapitalizeCharacters
)

const (
	AutoCompleteDefault                = driver.AutoCompleteDefault
	AutoCompleteOff                    = driver.AutoCompleteOff
	AutoCompleteUsername               = driver.AutoCompleteUsername
	AutoCompletePassword               = driver.AutoCompletePassword
	AutoCompleteNewPassword            = driver.AutoCompleteNewPassword
	AutoCompleteOneTimeCode            = driver.AutoCompleteOneTimeCode
	AutoCompleteEmail                  = driver.AutoCompleteEmail
	AutoCompleteTel                    = driver.AutoCompleteTel
	AutoCompleteName                   = driver.AutoCompleteName
	AutoCompleteGivenName              = driver.AutoCompleteGivenName
	AutoCompleteFamilyName             = driver.AutoCompleteFamilyName
	AutoCompleteMiddleName             = driver.AutoCompleteMiddleName
	AutoCompleteNickname               = driver.AutoCompleteNickname
	AutoCompleteOrganization           = driver.AutoCompleteOrganization
	AutoCompleteStreetAddress          = driver.AutoCompleteStreetAddress
	AutoCompleteAddressLine1           = driver.AutoCompleteAddressLine1
	AutoCompleteAddressLine2           = driver.AutoCompleteAddressLine2
	AutoCompleteCity                   = driver.AutoCompleteCity
	AutoCompleteRegion                 = driver.AutoCompleteRegion
	AutoCompletePostalCode             = driver.AutoCompletePostalCode
	AutoCompleteCountry                = driver.AutoCompleteCountry
	AutoCompleteCreditCardNumber       = driver.AutoCompleteCreditCardNumber
	AutoCompleteCreditCardExpiration   = driver.AutoCompleteCreditCardExpiration
	AutoCompleteCreditCardSecurityCode = driver.AutoCompleteCreditCardSecurityCode
	AutoCompleteCreditCardName         = driver.AutoCompleteCreditCardName
	AutoCompleteURL                    = driver.AutoCompleteURL
	AutoCompleteBirthdate              = driver.AutoCompleteBirthdate
)

const (
	DrawSave           = driver.DrawSave
	DrawRestore        = driver.DrawRestore
	DrawTranslate      = driver.DrawTranslate
	DrawScale          = driver.DrawScale
	DrawRotate         = driver.DrawRotate
	DrawBeginPath      = driver.DrawBeginPath
	DrawMoveTo         = driver.DrawMoveTo
	DrawLineTo         = driver.DrawLineTo
	DrawQuadTo         = driver.DrawQuadTo
	DrawCubicTo        = driver.DrawCubicTo
	DrawArc            = driver.DrawArc
	DrawRect           = driver.DrawRect
	DrawRoundRect      = driver.DrawRoundRect
	DrawEllipse        = driver.DrawEllipse
	DrawClosePath      = driver.DrawClosePath
	DrawFill           = driver.DrawFill
	DrawStroke         = driver.DrawStroke
	DrawClip           = driver.DrawClip
	DrawFillColor      = driver.DrawFillColor
	DrawStrokeColor    = driver.DrawStrokeColor
	DrawLineWidth      = driver.DrawLineWidth
	DrawLineCap        = driver.DrawLineCap
	DrawLineJoin       = driver.DrawLineJoin
	DrawAlpha          = driver.DrawAlpha
	DrawFillText       = driver.DrawFillText
	DrawClear          = driver.DrawClear
	DrawFillGradient   = driver.DrawFillGradient
	DrawStrokeGradient = driver.DrawStrokeGradient
	DrawTransform      = driver.DrawTransform
	DrawSetTransform   = driver.DrawSetTransform
	DrawComposite      = driver.DrawComposite
	DrawLineDash       = driver.DrawLineDash
	DrawLineDashOffset = driver.DrawLineDashOffset
	DrawMiterLimit     = driver.DrawMiterLimit
	DrawShadow         = driver.DrawShadow
	DrawStrokeText     = driver.DrawStrokeText
	DrawImage          = driver.DrawImage
	DrawTextAlign      = driver.DrawTextAlign
	DrawTextBaseline   = driver.DrawTextBaseline
)

const (
	ArgX     = driver.ArgX
	ArgY     = driver.ArgY
	ArgX1    = driver.ArgX1
	ArgY1    = driver.ArgY1
	ArgX2    = driver.ArgX2
	ArgY2    = driver.ArgY2
	ArgW     = driver.ArgW
	ArgH     = driver.ArgH
	ArgR     = driver.ArgR
	ArgA     = driver.ArgA
	ArgA0    = driver.ArgA0
	ArgA1    = driver.ArgA1
	ArgRY    = driver.ArgRY
	ArgColor = driver.ArgColor
)

const (
	CompositeSourceOver      = driver.CompositeSourceOver
	CompositeSourceIn        = driver.CompositeSourceIn
	CompositeSourceOut       = driver.CompositeSourceOut
	CompositeSourceAtop      = driver.CompositeSourceAtop
	CompositeDestinationOver = driver.CompositeDestinationOver
	CompositeDestinationIn   = driver.CompositeDestinationIn
	CompositeDestinationOut  = driver.CompositeDestinationOut
	CompositeDestinationAtop = driver.CompositeDestinationAtop
	CompositeLighter         = driver.CompositeLighter
	CompositeCopy            = driver.CompositeCopy
	CompositeXor             = driver.CompositeXor
	CompositeMultiply        = driver.CompositeMultiply
	CompositeScreen          = driver.CompositeScreen
	CompositeOverlay         = driver.CompositeOverlay
	CompositeDarken          = driver.CompositeDarken
	CompositeLighten         = driver.CompositeLighten
	CompositeColorDodge      = driver.CompositeColorDodge
	CompositeColorBurn       = driver.CompositeColorBurn
	CompositeHardLight       = driver.CompositeHardLight
	CompositeSoftLight       = driver.CompositeSoftLight
	CompositeDifference      = driver.CompositeDifference
	CompositeExclusion       = driver.CompositeExclusion
	CompositeHue             = driver.CompositeHue
	CompositeSaturation      = driver.CompositeSaturation
	CompositeColor           = driver.CompositeColor
	CompositeLuminosity      = driver.CompositeLuminosity
)

const (
	BaselineAlphabetic  = driver.BaselineAlphabetic
	BaselineTop         = driver.BaselineTop
	BaselineHanging     = driver.BaselineHanging
	BaselineMiddle      = driver.BaselineMiddle
	BaselineIdeographic = driver.BaselineIdeographic
	BaselineBottom      = driver.BaselineBottom
)

const (
	NonZero = driver.NonZero
	EvenOdd = driver.EvenOdd
)

const (
	JumpEnd   = driver.JumpEnd
	JumpStart = driver.JumpStart
	JumpNone  = driver.JumpNone
	JumpBoth  = driver.JumpBoth
)

const (
	CapButt   = driver.CapButt
	CapRound  = driver.CapRound
	CapSquare = driver.CapSquare
)

const (
	JoinMiter = driver.JoinMiter
	JoinRound = driver.JoinRound
	JoinBevel = driver.JoinBevel
)

const (
	AlertDefault     = driver.AlertDefault
	AlertCancel      = driver.AlertCancel
	AlertDestructive = driver.AlertDestructive
)

const (
	KeyboardDefault = driver.KeyboardDefault
	KeyboardEmail   = driver.KeyboardEmail
	KeyboardNumber  = driver.KeyboardNumber
	KeyboardDecimal = driver.KeyboardDecimal
	KeyboardPhone   = driver.KeyboardPhone
	KeyboardURL     = driver.KeyboardURL
)

const (
	RoleNone        = driver.RoleNone
	RoleButton      = driver.RoleButton
	RoleLink        = driver.RoleLink
	RoleHeader      = driver.RoleHeader
	RoleImage       = driver.RoleImage
	RoleCheckbox    = driver.RoleCheckbox
	RoleSwitch      = driver.RoleSwitch
	RoleRadio       = driver.RoleRadio
	RoleTab         = driver.RoleTab
	RoleText        = driver.RoleText
	RoleTextField   = driver.RoleTextField
	RoleRadioGroup  = driver.RoleRadioGroup
	RoleTabList     = driver.RoleTabList
	RoleMenu        = driver.RoleMenu
	RoleMenuItem    = driver.RoleMenuItem
	RoleDialog      = driver.RoleDialog
	RoleAlert       = driver.RoleAlert
	RoleAdjustable  = driver.RoleAdjustable
	RoleProgressBar = driver.RoleProgressBar
	// RoleTable and RoleGrid are a table and an interactive one, around
	// RoleRowGroup, RoleRow, RoleColumnHeader and RoleCell.
	RoleTable        = driver.RoleTable
	RoleGrid         = driver.RoleGrid
	RoleRowGroup     = driver.RoleRowGroup
	RoleRow          = driver.RoleRow
	RoleColumnHeader = driver.RoleColumnHeader
	RoleCell         = driver.RoleCell
)

const (
	PasteInsert = driver.PasteInsert
	PasteReport = driver.PasteReport
)

const (
	DecelerationFast = driver.DecelerationFast
	ActionActivate   = driver.ActionActivate
	ActionIncrement  = driver.ActionIncrement
	ActionDecrement  = driver.ActionDecrement
	ActionEscape     = driver.ActionEscape
	ActionMagicTap   = driver.ActionMagicTap
	FontMonospace    = driver.FontMonospace
)

const (
	SnapStart  = driver.SnapStart
	SnapCenter = driver.SnapCenter
	SnapEnd    = driver.SnapEnd
)

const (
	KeyboardTapsNever   = driver.KeyboardTapsNever
	KeyboardTapsAlways  = driver.KeyboardTapsAlways
	KeyboardTapsHandled = driver.KeyboardTapsHandled
)

const (
	LiveRegionNone      = driver.LiveRegionNone
	LiveRegionPolite    = driver.LiveRegionPolite
	LiveRegionAssertive = driver.LiveRegionAssertive
)

const (
	PointerEventsAuto    = driver.PointerEventsAuto
	PointerEventsNone    = driver.PointerEventsNone
	PointerEventsBoxNone = driver.PointerEventsBoxNone
	PointerEventsBoxOnly = driver.PointerEventsBoxOnly
)

const (
	ExtrapolateClamp    = driver.ExtrapolateClamp
	ExtrapolateExtend   = driver.ExtrapolateExtend
	ExtrapolateIdentity = driver.ExtrapolateIdentity
)

const (
	EasingBezier  = driver.EasingBezier
	EasingQuad    = driver.EasingQuad
	EasingCubic   = driver.EasingCubic
	EasingPoly    = driver.EasingPoly
	EasingSin     = driver.EasingSin
	EasingCircle  = driver.EasingCircle
	EasingExp     = driver.EasingExp
	EasingElastic = driver.EasingElastic
	EasingBack    = driver.EasingBack
	EasingBounce  = driver.EasingBounce
	EasingSteps   = driver.EasingSteps
)

const (
	EaseModeIn    = driver.EaseModeIn
	EaseModeOut   = driver.EaseModeOut
	EaseModeInOut = driver.EaseModeInOut
)

const (
	AnimationTiming   = driver.AnimationTiming
	AnimationSpring   = driver.AnimationSpring
	AnimationDecay    = driver.AnimationDecay
	AnimationSequence = driver.AnimationSequence
	AnimationParallel = driver.AnimationParallel
	AnimationStagger  = driver.AnimationStagger
	AnimationDelay    = driver.AnimationDelay
	AnimationLoop     = driver.AnimationLoop
)

const (
	CursorAuto       = driver.CursorAuto
	CursorDefault    = driver.CursorDefault
	CursorPointer    = driver.CursorPointer
	CursorText       = driver.CursorText
	CursorWait       = driver.CursorWait
	CursorProgress   = driver.CursorProgress
	CursorHelp       = driver.CursorHelp
	CursorCrosshair  = driver.CursorCrosshair
	CursorMove       = driver.CursorMove
	CursorGrab       = driver.CursorGrab
	CursorGrabbing   = driver.CursorGrabbing
	CursorNotAllowed = driver.CursorNotAllowed
	CursorZoomIn     = driver.CursorZoomIn
	CursorZoomOut    = driver.CursorZoomOut
	CursorColResize  = driver.CursorColResize
	CursorRowResize  = driver.CursorRowResize
	CursorNone       = driver.CursorNone
)

const (
	TitleBarDefault = driver.TitleBarDefault
	TitleBarHidden  = driver.TitleBarHidden
)

const (
	WindowDragInherit = driver.WindowDragInherit
	WindowDragOn      = driver.WindowDragOn
	WindowDragOff     = driver.WindowDragOff
)

const (
	WindowMaterialNone         = driver.WindowMaterialNone
	WindowMaterialWindow       = driver.WindowMaterialWindow
	WindowMaterialContent      = driver.WindowMaterialContent
	WindowMaterialSidebar      = driver.WindowMaterialSidebar
	WindowMaterialHeader       = driver.WindowMaterialHeader
	WindowMaterialTitlebar     = driver.WindowMaterialTitlebar
	WindowMaterialUnderWindow  = driver.WindowMaterialUnderWindow
	WindowMaterialUnderPage    = driver.WindowMaterialUnderPage
	WindowMaterialMenu         = driver.WindowMaterialMenu
	WindowMaterialPopover      = driver.WindowMaterialPopover
	WindowMaterialHUD          = driver.WindowMaterialHUD
	WindowMaterialSheet        = driver.WindowMaterialSheet
	WindowMaterialTooltip      = driver.WindowMaterialTooltip
	WindowMaterialSelection    = driver.WindowMaterialSelection
	WindowMaterialFullScreenUI = driver.WindowMaterialFullScreenUI
	WindowMaterialMica         = driver.WindowMaterialMica
	WindowMaterialAcrylic      = driver.WindowMaterialAcrylic
	WindowMaterialTabbed       = driver.WindowMaterialTabbed
)

const (
	WindowSetTitleBar      = driver.WindowSetTitleBar
	WindowMinimize         = driver.WindowMinimize
	WindowToggleMaximize   = driver.WindowToggleMaximize
	WindowToggleFullScreen = driver.WindowToggleFullScreen
	WindowClose            = driver.WindowClose
	WindowSetFrame         = driver.WindowSetFrame
	WindowSetLevel         = driver.WindowSetLevel
	WindowOrderFront       = driver.WindowOrderFront
	WindowSetMenuBar       = driver.WindowSetMenuBar
	WindowSetControls      = driver.WindowSetControls
	WindowSetMaterial      = driver.WindowSetMaterial
)

const (
	MenuRoleNone         = driver.MenuRoleNone
	MenuAbout            = driver.MenuAbout
	MenuHide             = driver.MenuHide
	MenuHideOthers       = driver.MenuHideOthers
	MenuShowAll          = driver.MenuShowAll
	MenuQuit             = driver.MenuQuit
	MenuUndo             = driver.MenuUndo
	MenuRedo             = driver.MenuRedo
	MenuCut              = driver.MenuCut
	MenuCopy             = driver.MenuCopy
	MenuPaste            = driver.MenuPaste
	MenuSelectAll        = driver.MenuSelectAll
	MenuMinimize         = driver.MenuMinimize
	MenuZoom             = driver.MenuZoom
	MenuToggleFullScreen = driver.MenuToggleFullScreen
	MenuClose            = driver.MenuClose
)

const (
	SelectableAuto = driver.SelectableAuto
	SelectableOn   = driver.SelectableOn
	SelectableOff  = driver.SelectableOff
)

const (
	AudioSessionDefault       = driver.AudioSessionDefault
	AudioSessionPlayback      = driver.AudioSessionPlayback
	AudioSessionMixWithOthers = driver.AudioSessionMixWithOthers
)

const (
	Cover     = driver.Cover
	Contain   = driver.Contain
	Stretch   = driver.Stretch
	Center    = driver.Center
	ScaleDown = driver.ScaleDown
)

const (
	ModalFullScreen = driver.ModalFullScreen
	ModalPageSheet  = driver.ModalPageSheet
	ModalFormSheet  = driver.ModalFormSheet
)

const (
	OrientationPortrait           = driver.OrientationPortrait
	OrientationPortraitUpsideDown = driver.OrientationPortraitUpsideDown
	OrientationLandscapeLeft      = driver.OrientationLandscapeLeft
	OrientationLandscapeRight     = driver.OrientationLandscapeRight
	OrientationLandscape          = driver.OrientationLandscape
)

const (
	ModalSlide = driver.ModalSlide
	ModalFade  = driver.ModalFade
	ModalNone  = driver.ModalNone
)

const (
	TransitionOpacity    = driver.TransitionOpacity
	TransitionBackground = driver.TransitionBackground
	TransitionTransform  = driver.TransitionTransform
	TransitionBorder     = driver.TransitionBorder
	TransitionShadow     = driver.TransitionShadow
)

const (
	GestureBegan     = driver.GestureBegan
	GestureChanged   = driver.GestureChanged
	GestureEnded     = driver.GestureEnded
	GestureCancelled = driver.GestureCancelled
)

const (
	DefaultPanSlop  = driver.DefaultPanSlop
	DefaultFontSize = driver.DefaultFontSize
)

const (
	StatusBarInherit = driver.StatusBarInherit
	StatusBarAuto    = driver.StatusBarAuto
	StatusBarDark    = driver.StatusBarDark
	StatusBarLight   = driver.StatusBarLight
)

const (
	FontWeightLight    = driver.FontWeightLight
	FontWeightNormal   = driver.FontWeightNormal
	FontWeightMedium   = driver.FontWeightMedium
	FontWeightSemibold = driver.FontWeightSemibold
	FontWeightBold     = driver.FontWeightBold
)

const (
	TextAlignStart   = driver.TextAlignStart
	TextAlignCenter  = driver.TextAlignCenter
	TextAlignEnd     = driver.TextAlignEnd
	TextAlignJustify = driver.TextAlignJustify
)

const (
	VerticalAlignBaseline   = driver.VerticalAlignBaseline
	VerticalAlignTextTop    = driver.VerticalAlignTextTop
	VerticalAlignTextBottom = driver.VerticalAlignTextBottom
	VerticalAlignMiddle     = driver.VerticalAlignMiddle
)

const (
	TextDecorationNone        = driver.TextDecorationNone
	TextDecorationUnderline   = driver.TextDecorationUnderline
	TextDecorationLineThrough = driver.TextDecorationLineThrough
)

const (
	HeaderItemAdd     = driver.HeaderItemAdd
	HeaderItemDone    = driver.HeaderItemDone
	HeaderItemClose   = driver.HeaderItemClose
	HeaderItemEdit    = driver.HeaderItemEdit
	HeaderItemCompose = driver.HeaderItemCompose
	HeaderItemSearch  = driver.HeaderItemSearch
	HeaderItemShare   = driver.HeaderItemShare
	HeaderItemTrash   = driver.HeaderItemTrash
	HeaderItemSave    = driver.HeaderItemSave
	HeaderItemCancel  = driver.HeaderItemCancel
)

const (
	FontVariantNormal    = driver.FontVariantNormal
	FontSmallCaps        = driver.FontSmallCaps
	FontOldstyleNums     = driver.FontOldstyleNums
	FontLiningNums       = driver.FontLiningNums
	FontTabularNums      = driver.FontTabularNums
	FontProportionalNums = driver.FontProportionalNums
)

const (
	DecorationSolid  = driver.DecorationSolid
	DecorationDouble = driver.DecorationDouble
	DecorationDotted = driver.DecorationDotted
	DecorationDashed = driver.DecorationDashed
	DecorationWavy   = driver.DecorationWavy
)

const (
	EllipsizeTail   = driver.EllipsizeTail
	EllipsizeHead   = driver.EllipsizeHead
	EllipsizeMiddle = driver.EllipsizeMiddle
	EllipsizeClip   = driver.EllipsizeClip
)

const (
	AccessibleAuto = driver.AccessibleAuto
	AccessibleYes  = driver.AccessibleYes
	AccessibleNo   = driver.AccessibleNo
)

const (
	SpreadPad     = driver.SpreadPad
	SpreadReflect = driver.SpreadReflect
	SpreadRepeat  = driver.SpreadRepeat
)

const (
	LayoutLinear        = driver.LayoutLinear
	LayoutEaseIn        = driver.LayoutEaseIn
	LayoutEaseOut       = driver.LayoutEaseOut
	LayoutEaseInEaseOut = driver.LayoutEaseInEaseOut
	LayoutSpring        = driver.LayoutSpring
	LayoutKeyboard      = driver.LayoutKeyboard
)

const (
	LayoutOpacity = driver.LayoutOpacity
	LayoutScale   = driver.LayoutScale
)

const (
	Downstream = driver.Downstream
	Upstream   = driver.Upstream
)

const (
	StorageSet    = driver.StorageSet
	StorageRemove = driver.StorageRemove
	StorageClear  = driver.StorageClear
)

var (
	Linear    = driver.Linear
	Ease      = driver.Ease
	EaseIn    = driver.EaseIn
	EaseOut   = driver.EaseOut
	EaseInOut = driver.EaseInOut
	Auto      = driver.Auto
	// PaintNone paints nothing; PaintCurrentColor paints with the text
	// color.
	PaintNone         = driver.PaintNone
	PaintCurrentColor = driver.PaintCurrentColor
)

// PaintColor paints with c.
func PaintColor(c Color) Paint { return driver.PaintColor(c) }

// UTF16Len is the length of s in UTF-16 units.
func UTF16Len(s string) int { return driver.UTF16Len(s) }

// All returns edges of the same length on every side.
func All(p Points) Edges { return driver.All(p) }

// Symmetric returns edges with vertical and horizontal lengths.
func Symmetric(vertical, horizontal Points) Edges { return driver.Symmetric(vertical, horizontal) }

// Pt returns a length in points.
func Pt(p Points) Length { return driver.Pt(p) }

// Pct returns a length in percent of the containing block's size.
func Pct(percent float32) Length { return driver.Pct(percent) }

// RGB returns an opaque color from 0xRRGGBB.
func RGB(hex uint32) Color { return driver.RGB(hex) }

// RGBA returns a color from 0xRRGGBB and an alpha from 0 to 1.
func RGBA(hex uint32, alpha float32) Color { return driver.RGBA(hex, alpha) }

// ParseColor reads a CSS color, as a host's Tokens give them: hex
// ("#e11d48", "#fff8"), rgb(), rgba(), hsl(), hsla(), a color name or
// transparent.
func ParseColor(s string) (Color, error) { return driver.ParseColor(s) }

// AllColors returns edge colors of the same color on every side.
func AllColors(c Color) EdgeColors { return driver.AllColors(c) }

// CanvasLinear returns a canvas gradient from (x0, y0) to (x1, y1).
func CanvasLinear(x0, y0, x1, y1 float32, stops ...GradientStop) *CanvasGradient {
	return driver.CanvasLinear(x0, y0, x1, y1, stops...)
}

// CanvasRadial returns a canvas gradient from the circle (x0, y0, r0) to
// the circle (x1, y1, r1).
func CanvasRadial(x0, y0, r0, x1, y1, r1 float32, stops ...GradientStop) *CanvasGradient {
	return driver.CanvasRadial(x0, y0, r0, x1, y1, r1, stops...)
}

// CanvasConic returns a canvas gradient around (x, y) from angle radians,
// turning clockwise.
func CanvasConic(angle, x, y float32, stops ...GradientStop) *CanvasGradient {
	return driver.CanvasConic(angle, x, y, stops...)
}

// ParsePath reads SVG path data into a Path.
func ParsePath(data string) (*Path, error) { return driver.ParsePath(data) }

// MustParsePath is ParsePath for path data that is part of the program:
// it panics when the data is in error.
func MustParsePath(data string) *Path { return driver.MustParsePath(data) }

// ParseSVG parses an SVG document.
func ParseSVG(data []byte) (*SVG, error) { return driver.ParseSVG(data) }
