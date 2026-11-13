package widget

import (
	"time"

	"github.com/nsgo-dev/nsgo/core"
)

// View is an element of what a widget shows, which the widget extension
// lays out and draws with SwiftUI: Text, Symbol, Image, Date, Column, Row,
// Stack, Spacer, Divider, Rectangle, Progress and Gauge. A nil View shows
// nothing.
//
// Widgets lay out as SwiftUI does, not as core's flexbox: a view is as
// large as its content unless Style fills or sizes it, and a container
// centers what it holds unless AlignItems and Justify say otherwise.
type View interface {
	node() *node
}

// Text shows a text.
type Text struct {
	Content string
	Style   Style
}

// Symbol shows one of Apple's SF Symbols by name, as "drop.fill", sized
// by the font and colored by Style.Color.
type Symbol struct {
	Name  string
	Style Style
}

// Image shows an image of PNG, JPEG, HEIF or any format Apple's systems
// read: Data, or the file at File, which Update reads. It fits its space
// keeping its aspect ratio, or covers it when Cover is set, cropped. The
// extension reads it at no more than 1024 pixels across, as the system
// bounds the memory of widgets.
type Image struct {
	Data  []byte
	File  string
	Cover bool
	Style Style
}

// Date shows a time in the language and formats of the system.
type Date struct {
	Time   time.Time
	Format DateFormat
	Style  Style
}

// DateFormat is how Date shows a time. Relative, Offset and Timer change
// by themselves as time passes, without an entry for each second.
type DateFormat uint8

const (
	// TimeOfDay shows the time, as "11:23 PM".
	TimeOfDay DateFormat = iota
	// Day shows the date, as "June 3, 2026".
	Day
	// Relative shows the time from now, as "2 hr, 23 min".
	Relative
	// Offset shows the time from now with its sign, as "+2 hours".
	Offset
	// Timer counts the time from or to it, as "36:59:01".
	Timer
)

// Column stacks its children from top to bottom.
type Column struct {
	Style    Style
	Children []View
}

// Row stacks its children from the start to the end of the line.
type Row struct {
	Style    Style
	Children []View
}

// Stack lays its children over each other, the last in front.
type Stack struct {
	Style    Style
	Children []View
}

// Spacer takes the space left along the axis of the row or column holding
// it, pushing its siblings apart.
type Spacer struct{}

// Divider draws a thin line across the row or column holding it.
type Divider struct {
	Style Style
}

// Rectangle fills its space with a color, its corners rounded by
// Style.CornerRadius.
type Rectangle struct {
	Fill  Color
	Style Style
}

// Progress shows a bar filled to Value, from 0 to 1, in Style.Color, as
// wide as its space.
type Progress struct {
	Value float64
	Style Style
}

// Gauge shows a ring filled to Value, from 0 to 1, in Style.Color, 44
// points across unless sized.
type Gauge struct {
	Value float64
	Style Style
}

// Button stacks its children as a Column does and, pressed in a Live
// Activity from iOS 17, runs the handler of OnAction with Action, without
// opening the app. Elsewhere, in widgets and on Android, it shows its
// children alone.
type Button struct {
	Action   string
	Style    Style
	Children []View
}

// Style is how a view lays out and looks. Its text fields (Color and the
// font's) pass to the views it holds, which change parts of them.
type Style struct {
	// Gap separates the children of a Column or a Row.
	Gap core.Points
	// AlignItems places the children of a Column across it, those of a Row
	// up and down and those of a Stack both ways, and the content of a
	// view that fills more space than it needs across; Justify places that
	// content along its axis. Both center by default.
	AlignItems Align
	Justify    Align
	// Padding is the space inside the view's edges, inside its background;
	// Left and Right are its start and end.
	Padding core.Edges
	// Width and Height size the view, its padding included; 0 fits its
	// content. FillWidth and FillHeight make it as large as its space.
	Width, Height         core.Points
	FillWidth, FillHeight bool
	// Background fills the view's box, its padding included, its corners
	// rounded by CornerRadius, which also clips the view.
	Background   Color
	CornerRadius core.Points
	// Color colors the text, the symbols, the bars and the rings.
	Color Color
	// TextStyle sizes text with one of the system's styles, which follow
	// the text size the user chose; FontSize sizes it in points instead.
	TextStyle  TextStyle
	FontSize   core.Points
	FontWeight core.FontWeight
	FontDesign FontDesign
	Italic     bool
	// FontVariant turns on core.FontTabularNums, which keeps the width of
	// numbers that change, and core.FontSmallCaps.
	FontVariant core.FontVariant
	// TextAlign aligns the lines of a text of several lines; MaxLines cuts
	// it with an ellipsis, after MinScale let it shrink down to that
	// fraction of its size.
	TextAlign core.TextAlign
	MaxLines  int
	MinScale  float32
	// Link opens with the app when the view is tapped, as Entry.URL does
	// for the whole widget. Small widgets and those of the Lock Screen
	// open only their entry's URL.
	Link string
	// Accented puts the view among those that take the accent color where
	// the system tints widgets, as on the Lock Screen and the tinted Home
	// Screen, where the others take its secondary color.
	Accented bool
}

// Align places views at the center, the start or the end of their space.
type Align uint8

const (
	AlignCenter Align = iota
	AlignStart
	AlignEnd
)

// TextStyle is one of the system's text styles, from the largest.
type TextStyle uint8

const (
	// NoTextStyle leaves the size to FontSize, or to the view holding it.
	NoTextStyle TextStyle = iota
	LargeTitle
	Title
	Title2
	Title3
	Headline
	Subheadline
	Body
	Callout
	Footnote
	Caption
	Caption2
)

// FontDesign is a design of the system's font.
type FontDesign uint8

const (
	DefaultDesign FontDesign = iota
	Rounded
	Serif
	Monospaced
)

// Color is a color of a widget: an sRGB color, with another in dark mode,
// or one of the system's, which follow the appearance and the tinting of
// widgets. The zero Color is none: text takes its container's.
type Color struct {
	light, dark core.Color
	system      string
}

// The system's colors.
var (
	// Primary is the color of text, black or white.
	Primary = Color{system: "primary"}
	// Secondary is the color of text of less importance.
	Secondary = Color{system: "secondary"}
	// Accent is the app's accent color, the system's blue by default.
	Accent = Color{system: "accent"}
)

// RGB returns an opaque color from 0xRRGGBB, as core.RGB does.
func RGB(hex uint32) Color { return Color{light: core.RGB(hex)} }

// RGBA returns a color from 0xRRGGBB and an alpha from 0 to 1, as
// core.RGBA does.
func RGBA(hex uint32, alpha float32) Color { return Color{light: core.RGBA(hex, alpha)} }

// Dark returns the color with d in its place in dark mode.
func (c Color) Dark(d Color) Color {
	c.dark = d.light
	return c
}
