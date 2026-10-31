package core

import (
	"encoding/binary"
	"math"
	"reflect"
	"strings"
	"unicode"

	"github.com/oneo-me/nsgo/internal/driver"
)

// Style is a node's layout, look and text. Every field's zero value means
// unset, so styles compose with Merge and text fields inherit from the
// parent as in CSS. A style with nothing set draws nothing: NSGo has no
// look of its own.
type Style struct {
	// Layout, with React Native's defaults: a column flexbox, items that
	// do not shrink and stretch on the cross axis, border-box sizes, no
	// automatic minimum sizes. Grid and block layout are CSS's.
	Display        Display
	Position       Position
	Direction      FlexDirection
	Wrap           FlexWrap
	JustifyContent Justify
	AlignItems     Align
	AlignSelf      Align
	AlignContent   Align
	// JustifyItems and JustifySelf align grid items (and block children)
	// along the row.
	JustifyItems Align
	JustifySelf  Align
	FlexGrow     float32
	FlexShrink   float32
	FlexBasis    Length

	Width, Height       Length
	MinWidth, MinHeight Length
	MaxWidth, MaxHeight Length
	// AspectRatio is width divided by height; 0 is unset.
	AspectRatio float32
	// BoxSizing says whether sizes include padding and borders, which they
	// do unless it is ContentBox.
	BoxSizing BoxSizing

	// Top, Right, Bottom and Left place an absolutely positioned node in
	// its parent, or offset a relative one. Right to left, left and right
	// swap, as in React Native, so they are its start and end, as are the
	// left and right of margins, paddings, borders and corners.
	Top, Right, Bottom, Left Length
	// LayoutDirection lays the node and what it holds out left to right
	// or right to left, over the app's direction; the zero value inherits
	// the parent's.
	LayoutDirection LayoutDirection
	// ZIndex stacks the node above its siblings with lower ones, keeping
	// the order of equals; it changes neither layout nor what sees the
	// node first in the tree.
	ZIndex int

	Margin Edges
	// AutoMargin makes the margins on those sides auto: they take the free
	// space, centering a node between two of them.
	AutoMargin Sides
	Padding    Edges
	// PaddingEnv and MarginEnv add the safe area's insets or the software
	// keyboard's height to the padding and margin on their edges.
	PaddingEnv, MarginEnv EnvEdges
	// Gap separates children; RowGap and ColumnGap override it per axis.
	Gap, RowGap, ColumnGap Points

	// GridTemplateColumns and GridTemplateRows are a grid's tracks in CSS
	// syntax, such as "repeat(3, 1fr)" or "[full-start] 100px minmax(0, 1fr)";
	// GridAutoColumns and GridAutoRows size the tracks it adds.
	GridTemplateColumns, GridTemplateRows GridTracks
	GridAutoColumns, GridAutoRows         GridTracks
	GridAutoFlow                          GridFlow
	// GridRow and GridColumn place the node in its parent grid.
	GridRow, GridColumn GridLines

	// Look.
	Background Color
	// Gradients are drawn over Background, the first on top. A state
	// style that sets Background and no Gradients replaces them, so press
	// feedback shows.
	Gradients    []Gradient
	CornerRadius Points
	// CornerRadii, when set, gives each corner its own radius instead of
	// CornerRadius.
	CornerRadii Corners
	// BorderCurve Continuous eases the corners into their edges, as iOS's
	// continuous corners and Figma's corner smoothing do; pills stay pills.
	BorderCurve BorderCurve
	// BorderWidth is the width of the border on every side. Borders, when
	// set, gives each side's width to layout instead.
	BorderWidth Points
	Borders     Edges
	BorderColor Color
	// BorderColors, when set, colors each side; a side left 0 takes
	// BorderColor.
	BorderColors EdgeColors
	// BorderStyle dashes or dots the border; BorderNone and BorderHidden
	// take it away, in layout too.
	BorderStyle BorderStyle
	// OutlineWidth draws a line around the border box, OutlineOffset away
	// (inside when negative), following its corners, outside layout.
	OutlineWidth  Points
	OutlineOffset Points
	OutlineColor  Color
	OutlineStyle  BorderStyle
	Opacity       Opacity
	// Visibility Hidden hides the node and what it holds from sight, the
	// pointer and screen readers, keeping its place in layout.
	Visibility Visibility
	// Overflow Hidden clips children to the node's bounds and corners, and
	// keeps their overflow out of an enclosing scroll view's content.
	// OverflowX and OverflowY override it per axis.
	Overflow             Overflow
	OverflowX, OverflowY Overflow
	// ShadowColor, ShadowOffset and ShadowRadius are React Native's
	// shadow: ShadowRadius is the blur's standard deviation.
	ShadowColor  Color
	ShadowOffset Point
	ShadowRadius Points
	// Shadows are CSS's box shadows, the first on top; Ring makes
	// Tailwind's rings.
	Shadows []BoxShadow
	// Filters change what the node and its children draw, in order.
	Filters []Filter
	// BackdropFilters change what is behind the node, seen through its
	// background: Blur frosts it, with a view's IOS.BackdropMaterial on
	// iOS. GlassEffect draws Liquid Glass in its place on iOS and macOS
	// 26; give a glass view a blur to fall back to.
	BackdropFilters []Filter
	GlassEffect     GlassEffect
	// Mask draws the node, backdrop included, as much as the gradient's
	// alpha says.
	Mask *Gradient
	// BlendMode mixes the node's pixels with what is drawn behind it.
	BlendMode BlendMode
	// TintColor recolors an image's opaque pixels.
	TintColor Color
	// Cursor is the pointer's look over the view; Auto inherits the
	// nearest ancestor's, and a pressable view's is a pointer on the web.
	Cursor Cursor
	// ObjectPosition places an image in its view once ResizeMode has
	// sized it; centered when unset.
	ObjectPosition ObjectPosition
	// VerticalAlign places an attachment in its text's line, set on the
	// span or on the attachment's view.
	VerticalAlign VerticalAlign
	Transform     Transform
	// TransformOrigin is where Transform acts: its zero value is the
	// center, and its percentages are of the node's size.
	TransformOrigin ObjectPosition
	// Perspective is the depth children's rotations out of their plane
	// are seen from, at this node's center, as CSS's perspective.
	Perspective Points
	// BackfaceHidden hides the node while its rotations turn its back to
	// the viewer.
	BackfaceHidden bool
	// Transition animates changes of the look on the platform.
	Transition Transition
	// Animation runs keyframes over the node on the platform, on top of
	// its transform and opacity; Spin, Ping, Pulse and Bounce make
	// Tailwind's.
	Animation KeyframeAnimation
	// Animated binds properties to animated values the platform moves
	// every frame, over the ones above.
	Animated *AnimatedStyle

	// Text, inherited by descendants that set none.
	Color Color
	// FontFamily names a font family; empty is the system font, and
	// FontMonospace the system's fixed-pitch font.
	FontFamily string
	FontSize   Points
	FontWeight FontWeight
	FontStyle  FontStyle
	// FontVariant turns on small capitals and the forms and widths of
	// figures; FontVariantNormal undoes an inherited one. A node's
	// variant replaces its parent's.
	FontVariant FontVariant
	// LineHeight is the height of each line. LineHeightMultiple is one in
	// multiples of the font size, which descendants inherit as a multiple
	// of theirs; LineHeightEm is one too, resolved where it is set and
	// inherited as a length. Whichever the nearest node sets wins.
	LineHeight         Points
	LineHeightMultiple float32
	LineHeightEm       float32
	LetterSpacing      Points
	TextAlign          TextAlign
	TextDecoration     TextDecoration
	// TextDecorationStyle draws the decoration's lines solid, double,
	// dotted, dashed or wavy; TextDecorationColor colors them, the text's
	// color when 0.
	TextDecorationStyle TextDecorationStyle
	TextDecorationColor Color
	// TextDecorationThickness is how thick the decoration's lines are,
	// and TextUnderlineOffset how far below the baseline an underline's
	// top is; percentages are of the font size, and auto is the font's.
	TextDecorationThickness Length
	TextUnderlineOffset     Length
	// TextTransform changes the case of the characters, as written in the
	// text it is set on.
	TextTransform TextTransform
	// TextShadowColor, TextShadowOffset and TextShadowRadius draw a shadow
	// behind a text's glyphs, the text's own style's for its whole block.
	// An offset or radius without a color is black at a third of opacity.
	TextShadowColor  Color
	TextShadowOffset Point
	TextShadowRadius Points
	// TextAlignVertical moves text shorter than its box down within it.
	TextAlignVertical TextAlignVertical
	// SelectionColor is the highlight of selected text, and an input's
	// caret; 0 is the platform's.
	SelectionColor Color
	// PlaceholderColor colors an input's placeholder; 0 is the platform's.
	PlaceholderColor Color
	// Fill, Stroke, StrokeWidth and StrokeDash are SVG's paint properties,
	// inherited as CSS inherits them. Views draw nothing with them; a
	// canvas's drawing paints with them where it takes its view's paint
	// (Drawing.FillFromView and the like), so a component's shapes follow
	// the app's styles. A StrokeWidth of 0 is unset; a StrokeDash that is
	// empty but not nil is solid.
	Fill, Stroke Paint
	StrokeWidth  Points
	StrokeDash   []Points
	// WhiteSpace NoWrap keeps text on one line per paragraph, overflowing
	// its box.
	WhiteSpace WhiteSpace
	// UserSelect lets the user select the texts inside, unless a text's
	// own Selectable says otherwise.
	UserSelect UserSelect
	// WritingDirection is the base direction of text, which orders runs of
	// mixed directions and says where TextAlignStart is; the zero value is
	// the layout direction.
	WritingDirection WritingDirection

	// Group makes the node a group, named so, whose states the GroupStyle
	// of nodes inside it follows.
	Group string
	// GroupStyle merges styles over the rest while the nearest group
	// around the node is in their states, under the node's own states.
	GroupStyle *GroupStyle

	// State styles merge over the rest while the node is in the state, in
	// NSH's order of precedence, lowest first: checked, indeterminate and
	// selected, which the app sets, then hovered, focused (text fields) and
	// pressed, which the platform reports, then disabled over all.
	Hover, Focus, Active, Selected, Checked, Indeterminate, Disabled *Style
}

// Display says whether a node takes part in layout.
type Display uint8

const (
	DisplayFlex Display = iota + 1
	// DisplayNone hides the node and takes it out of layout.
	DisplayNone
	// DisplayGrid lays children out in a CSS grid.
	DisplayGrid
	// DisplayBlock stacks children as CSS block boxes, collapsing their
	// vertical margins.
	DisplayBlock
)

// BoxSizing is the CSS box-sizing property.
type BoxSizing uint8

const (
	BorderBox BoxSizing = iota + 1
	ContentBox
)

// Sides is a set of sides of a box.
type Sides uint8

const (
	SideTop Sides = 1 << iota
	SideRight
	SideBottom
	SideLeft
	// SidesHorizontal and SidesVertical are left and right, top and bottom.
	SidesHorizontal = SideLeft | SideRight
	SidesVertical   = SideTop | SideBottom
	SidesAll        = SidesHorizontal | SidesVertical
)

// Position says how a node is placed in its parent.
type Position uint8

const (
	// Relative nodes flow in their parent; Top and Left offset them.
	Relative Position = iota + 1
	// Absolute nodes leave the flow and are placed by Top, Right, Bottom
	// and Left in the padding box of their nearest positioned ancestor.
	Absolute
	// Static nodes flow in their parent and ignore Top, Right, Bottom and
	// Left, so absolute nodes inside them are placed by an ancestor.
	Static
	// Fixed nodes leave the flow and are placed by Top, Right, Bottom and
	// Left in the root view, or the modal they are in, whatever scrolls.
	Fixed
	// Sticky nodes flow as relative ones, then stick in the nearest
	// scroll view around them as it scrolls them out: Top (Left in a
	// horizontal one) from its start, as far as their parent's end. They
	// draw above their siblings with the same ZIndex.
	Sticky
)

// LayoutDirection is the direction a node's content flows in.
type LayoutDirection uint8

const (
	DirectionLTR LayoutDirection = iota + 1
	DirectionRTL
)

// WritingDirection is the base direction of a text's paragraphs.
type WritingDirection uint8

const (
	WritingLTR WritingDirection = iota + 1
	WritingRTL
	// WritingAuto takes the direction of the text's first strong
	// character, as HTML's dir="auto".
	WritingAuto
)

// rtlText reports whether text written s starts right to left: its first
// letter is Hebrew, Arabic or of another right-to-left script.
func rtlText(s string) bool {
	for _, r := range s {
		switch {
		case r >= 0x0590 && r <= 0x08ff, r >= 0xfb1d && r <= 0xfdff, r >= 0xfe70 && r <= 0xfefc,
			r >= 0x10800 && r <= 0x10fff, r >= 0x1e800 && r <= 0x1efff:
			if unicode.IsLetter(r) {
				return true
			}
		case unicode.IsLetter(r):
			return false
		}
	}
	return false
}

// BorderCurve is the shape of rounded corners.
type BorderCurve uint8

const (
	BorderCurveCircular BorderCurve = iota + 1
	BorderCurveContinuous
)

// Visibility says whether a node is seen while it keeps its place.
type Visibility uint8

const (
	VisibilityVisible Visibility = iota + 1
	VisibilityHidden
)

// WhiteSpace says whether text wraps.
type WhiteSpace uint8

const (
	WhiteSpaceNormal WhiteSpace = iota + 1
	WhiteSpaceNoWrap
)

// UserSelect says whether the user may select text.
type UserSelect uint8

const (
	UserSelectText UserSelect = iota + 1
	UserSelectNone
)

// GroupStyle styles a node by the states of its nearest group: the
// nearest ancestor whose Group is Name, or any group when Name is empty,
// as Tailwind's group-hover and its kin do.
type GroupStyle struct {
	Name                                                             string
	Hover, Focus, Active, Selected, Checked, Indeterminate, Disabled *Style
}

// FlexDirection is the main axis of a container, as in CSS.
type FlexDirection uint8

const (
	// Column lays children out top to bottom, the default as in React Native.
	Column FlexDirection = iota + 1
	Row
	ColumnReverse
	RowReverse
)

func (d FlexDirection) row() bool     { return d == Row || d == RowReverse }
func (d FlexDirection) reverse() bool { return d == ColumnReverse || d == RowReverse }

// FlexWrap says whether children wrap onto more lines.
type FlexWrap uint8

const (
	NoWrap FlexWrap = iota + 1
	Wrap
	WrapReverse
)

// Align places children on the cross axis (align-items, align-self,
// align-content) and grid items along the row (justify-items,
// justify-self). Start and end are React Native's flex-start and flex-end.
type Align uint8

const (
	AlignStretch Align = iota + 1
	AlignStart
	AlignCenter
	AlignEnd
	AlignSpaceBetween // align-content only
	AlignSpaceAround  // align-content only
	AlignBaseline     // items only
	AlignSpaceEvenly  // align-content only
)

// Justify places children on the main axis (justify-content).
type Justify uint8

const (
	JustifyStart Justify = iota + 1
	JustifyCenter
	JustifyEnd
	JustifySpaceBetween
	JustifySpaceAround
	JustifySpaceEvenly
	// JustifyStretch stretches a grid's auto tracks across the row, which
	// grids do when it is unset.
	JustifyStretch
)

// Overflow says whether a node clips its children.
type Overflow uint8

const (
	Visible Overflow = iota + 1
	Hidden
	// OverflowScroll makes the node a scroll container in layout, as
	// CSS's overflow: scroll, and clips it; only Scroll views scroll.
	OverflowScroll
)

// Opacity is a node's opacity from 0 to 1. Its zero value is unset,
// which is opaque; make one with Alpha.
type Opacity struct {
	value float32
	set   bool
}

// Alpha returns an opacity from 0 (invisible) to 1 (opaque).
func Alpha(a float32) Opacity { return Opacity{min(max(a, 0), 1), true} }

// Value returns the opacity, 1 when unset.
func (o Opacity) Value() float32 {
	if !o.set {
		return 1
	}
	return o.value
}

// FontStyle is upright or italic.
type FontStyle uint8

const (
	FontStyleNormal FontStyle = iota + 1
	FontStyleItalic
)

// TextAlignVertical is where text shorter than its box sits in it.
type TextAlignVertical uint8

const (
	TextAlignVerticalTop TextAlignVertical = iota + 1
	TextAlignVerticalCenter
	TextAlignVerticalBottom
)

// fraction is how far down the free space the text moves.
func (a TextAlignVertical) fraction() Points {
	switch a {
	case TextAlignVerticalCenter:
		return 0.5
	case TextAlignVerticalBottom:
		return 1
	}
	return 0
}

// TextTransform changes the case of text as it is laid out.
type TextTransform uint8

const (
	TextTransformNone TextTransform = iota + 1
	TextTransformUppercase
	TextTransformLowercase
	// TextTransformCapitalize upper-cases the first letter or digit of
	// each word.
	TextTransformCapitalize
)

// Apply returns s in the case x asks for.
func (x TextTransform) Apply(s string) string {
	switch x {
	case TextTransformUppercase:
		return strings.ToUpper(s)
	case TextTransformLowercase:
		return strings.ToLower(s)
	case TextTransformCapitalize:
		var b strings.Builder
		b.Grow(len(s))
		start := true
		for _, r := range s {
			if start && (unicode.IsLetter(r) || unicode.IsDigit(r)) {
				b.WriteRune(unicode.ToTitle(r))
				start = false
				continue
			}
			b.WriteRune(r)
			if unicode.IsSpace(r) {
				start = true
			}
		}
		return b.String()
	}
	return s
}

// IsZero reports whether s sets nothing.
func (s Style) IsZero() bool { return reflect.ValueOf(s).IsZero() }

// Merge returns s with every field o sets replacing s's.
func (s Style) Merge(o Style) Style {
	set(&s.Display, o.Display)
	set(&s.Position, o.Position)
	set(&s.Direction, o.Direction)
	set(&s.Wrap, o.Wrap)
	set(&s.JustifyContent, o.JustifyContent)
	set(&s.AlignItems, o.AlignItems)
	set(&s.AlignSelf, o.AlignSelf)
	set(&s.AlignContent, o.AlignContent)
	set(&s.JustifyItems, o.JustifyItems)
	set(&s.JustifySelf, o.JustifySelf)
	set(&s.FlexGrow, o.FlexGrow)
	set(&s.FlexShrink, o.FlexShrink)
	set(&s.FlexBasis, o.FlexBasis)
	set(&s.Width, o.Width)
	set(&s.Height, o.Height)
	set(&s.MinWidth, o.MinWidth)
	set(&s.MinHeight, o.MinHeight)
	set(&s.MaxWidth, o.MaxWidth)
	set(&s.MaxHeight, o.MaxHeight)
	set(&s.AspectRatio, o.AspectRatio)
	set(&s.BoxSizing, o.BoxSizing)
	set(&s.Top, o.Top)
	set(&s.Right, o.Right)
	set(&s.Bottom, o.Bottom)
	set(&s.Left, o.Left)
	set(&s.LayoutDirection, o.LayoutDirection)
	set(&s.ZIndex, o.ZIndex)
	set(&s.Margin, o.Margin)
	set(&s.AutoMargin, o.AutoMargin)
	set(&s.Padding, o.Padding)
	set(&s.PaddingEnv, o.PaddingEnv)
	set(&s.MarginEnv, o.MarginEnv)
	set(&s.Gap, o.Gap)
	set(&s.RowGap, o.RowGap)
	set(&s.ColumnGap, o.ColumnGap)
	set(&s.GridTemplateColumns, o.GridTemplateColumns)
	set(&s.GridTemplateRows, o.GridTemplateRows)
	set(&s.GridAutoColumns, o.GridAutoColumns)
	set(&s.GridAutoRows, o.GridAutoRows)
	set(&s.GridAutoFlow, o.GridAutoFlow)
	set(&s.GridRow, o.GridRow)
	set(&s.GridColumn, o.GridColumn)
	set(&s.Background, o.Background)
	setSlice(&s.Gradients, o.Gradients)
	set(&s.CornerRadius, o.CornerRadius)
	set(&s.CornerRadii, o.CornerRadii)
	set(&s.BorderCurve, o.BorderCurve)
	set(&s.BorderWidth, o.BorderWidth)
	set(&s.Borders, o.Borders)
	set(&s.BorderColor, o.BorderColor)
	set(&s.BorderColors, o.BorderColors)
	set(&s.BorderStyle, o.BorderStyle)
	set(&s.OutlineWidth, o.OutlineWidth)
	set(&s.OutlineOffset, o.OutlineOffset)
	set(&s.OutlineColor, o.OutlineColor)
	set(&s.OutlineStyle, o.OutlineStyle)
	set(&s.Opacity, o.Opacity)
	set(&s.Visibility, o.Visibility)
	set(&s.Overflow, o.Overflow)
	set(&s.OverflowX, o.OverflowX)
	set(&s.OverflowY, o.OverflowY)
	set(&s.ShadowColor, o.ShadowColor)
	set(&s.ShadowOffset, o.ShadowOffset)
	set(&s.ShadowRadius, o.ShadowRadius)
	setSlice(&s.Shadows, o.Shadows)
	setSlice(&s.Filters, o.Filters)
	setSlice(&s.BackdropFilters, o.BackdropFilters)
	set(&s.GlassEffect, o.GlassEffect)
	set(&s.Mask, o.Mask)
	set(&s.BlendMode, o.BlendMode)
	set(&s.TintColor, o.TintColor)
	set(&s.Cursor, o.Cursor)
	set(&s.ObjectPosition, o.ObjectPosition)
	set(&s.VerticalAlign, o.VerticalAlign)
	set(&s.Transform, o.Transform)
	set(&s.TransformOrigin, o.TransformOrigin)
	set(&s.Perspective, o.Perspective)
	set(&s.BackfaceHidden, o.BackfaceHidden)
	set(&s.Transition, o.Transition)
	set(&s.Animation, o.Animation)
	set(&s.Animated, o.Animated)
	s.mergeText(o)
	set(&s.Group, o.Group)
	set(&s.GroupStyle, o.GroupStyle)
	set(&s.Hover, o.Hover)
	set(&s.Focus, o.Focus)
	set(&s.Active, o.Active)
	set(&s.Selected, o.Selected)
	set(&s.Checked, o.Checked)
	set(&s.Indeterminate, o.Indeterminate)
	set(&s.Disabled, o.Disabled)
	return s
}

// Interaction states a node's style depends on.
const (
	statePressed uint8 = 1 << iota
	stateHovered
	stateFocused
)

// groupState is what a node's GroupStyle follows: the states of its
// group.
type groupState struct {
	states                                     uint8
	checked, indeterminate, selected, disabled bool
}

// withStates returns s with the styles of the states n is in merged in,
// its group's first. A state's Background without Gradients replaces the
// gradients, so press feedback shows.
func (s Style) withStates(states uint8, p driver.ViewProps, g groupState) Style {
	out := s
	merge := func(on bool, st *Style) {
		if !on || st == nil {
			return
		}
		if st.Background != 0 && st.Gradients == nil {
			out.Gradients = nil
		}
		out = out.Merge(*st)
	}
	if gs := s.GroupStyle; gs != nil {
		merge(g.checked, gs.Checked)
		merge(g.indeterminate, gs.Indeterminate)
		merge(g.selected, gs.Selected)
		merge(g.states&stateHovered != 0, gs.Hover)
		merge(g.states&stateFocused != 0, gs.Focus)
		merge(g.states&statePressed != 0, gs.Active)
		merge(g.disabled, gs.Disabled)
	}
	merge(p.Checked, s.Checked)
	merge(p.Indeterminate, s.Indeterminate)
	merge(p.Selected, s.Selected)
	merge(states&stateHovered != 0, s.Hover)
	merge(states&stateFocused != 0, s.Focus)
	merge(states&statePressed != 0, s.Active)
	merge(p.Disabled, s.Disabled)
	out.Hover, out.Focus, out.Active = s.Hover, s.Focus, s.Active
	out.Selected, out.Checked, out.Indeterminate, out.Disabled = s.Selected, s.Checked, s.Indeterminate, s.Disabled
	out.GroupStyle = s.GroupStyle
	return out
}

func (s *Style) mergeText(o Style) {
	set(&s.Color, o.Color)
	set(&s.FontFamily, o.FontFamily)
	set(&s.FontSize, o.FontSize)
	set(&s.FontWeight, o.FontWeight)
	set(&s.FontStyle, o.FontStyle)
	if o.LineHeight != 0 || o.LineHeightMultiple != 0 || o.LineHeightEm != 0 {
		s.LineHeight, s.LineHeightMultiple, s.LineHeightEm = o.LineHeight, o.LineHeightMultiple, o.LineHeightEm
	}
	set(&s.LetterSpacing, o.LetterSpacing)
	set(&s.TextAlign, o.TextAlign)
	set(&s.TextDecoration, o.TextDecoration)
	set(&s.TextDecorationStyle, o.TextDecorationStyle)
	set(&s.TextDecorationColor, o.TextDecorationColor)
	set(&s.TextDecorationThickness, o.TextDecorationThickness)
	set(&s.TextUnderlineOffset, o.TextUnderlineOffset)
	set(&s.FontVariant, o.FontVariant)
	set(&s.TextTransform, o.TextTransform)
	set(&s.TextShadowColor, o.TextShadowColor)
	set(&s.TextShadowOffset, o.TextShadowOffset)
	set(&s.TextShadowRadius, o.TextShadowRadius)
	set(&s.TextAlignVertical, o.TextAlignVertical)
	set(&s.SelectionColor, o.SelectionColor)
	set(&s.PlaceholderColor, o.PlaceholderColor)
	set(&s.Fill, o.Fill)
	set(&s.Stroke, o.Stroke)
	set(&s.StrokeWidth, o.StrokeWidth)
	setSlice(&s.StrokeDash, o.StrokeDash)
	set(&s.WhiteSpace, o.WhiteSpace)
	set(&s.UserSelect, o.UserSelect)
	set(&s.WritingDirection, o.WritingDirection)
	set(&s.LayoutDirection, o.LayoutDirection)
}

func set[T comparable](dst *T, v T) {
	var zero T
	if v != zero {
		*dst = v
	}
}

// setSlice sets dst to v unless v is nil: an empty slice that is not nil
// clears what dst had.
func setSlice[T any](dst *[]T, v []T) {
	if v != nil {
		*dst = v
	}
}

// textFields are a style's text fields, which views pass the views inside
// them: comparable, and small enough to copy down the tree.
type textFields struct {
	Color                            Color
	FontFamily                       string
	FontSize                         Points
	FontWeight                       FontWeight
	FontStyle                        FontStyle
	LineHeight, LetterSpacing        Points
	TextAlign                        TextAlign
	TextDecoration                   TextDecoration
	SelectionColor                   Color
	TextDecorationStyle              TextDecorationStyle
	TextDecorationColor              Color
	TextDecorationThickness          Length
	TextUnderlineOffset              Length
	FontVariant                      FontVariant
	TextTransform                    TextTransform
	TextShadowColor                  Color
	TextShadowOffset                 Point
	TextShadowRadius                 Points
	TextAlignVertical                TextAlignVertical
	PlaceholderColor                 Color
	LineHeightMultiple, LineHeightEm float32
	WhiteSpace                       WhiteSpace
	UserSelect                       UserSelect
	WritingDirection                 WritingDirection
	LayoutDirection                  LayoutDirection
	Fill, Stroke                     Paint
	StrokeWidth                      Points
	// StrokeDash holds the dashes' bits, so the fields stay comparable;
	// empty is unset.
	StrokeDash string
}

// inherit returns s's text fields, falling back to parent's. A line
// height in em becomes a length where it is set.
func (s *Style) inherit(parent textFields) textFields {
	t := parent
	set(&t.Color, s.Color)
	set(&t.FontFamily, s.FontFamily)
	set(&t.FontSize, s.FontSize)
	set(&t.FontWeight, s.FontWeight)
	set(&t.FontStyle, s.FontStyle)
	if s.LineHeight != 0 || s.LineHeightMultiple != 0 || s.LineHeightEm != 0 {
		t.LineHeight, t.LineHeightMultiple, t.LineHeightEm = s.LineHeight, s.LineHeightMultiple, s.LineHeightEm
	}
	set(&t.LetterSpacing, s.LetterSpacing)
	set(&t.TextAlign, s.TextAlign)
	set(&t.TextDecoration, s.TextDecoration)
	set(&t.TextDecorationStyle, s.TextDecorationStyle)
	set(&t.TextDecorationColor, s.TextDecorationColor)
	set(&t.TextDecorationThickness, s.TextDecorationThickness)
	set(&t.TextUnderlineOffset, s.TextUnderlineOffset)
	set(&t.FontVariant, s.FontVariant)
	set(&t.TextTransform, s.TextTransform)
	set(&t.TextShadowColor, s.TextShadowColor)
	set(&t.TextShadowOffset, s.TextShadowOffset)
	set(&t.TextShadowRadius, s.TextShadowRadius)
	set(&t.TextAlignVertical, s.TextAlignVertical)
	set(&t.SelectionColor, s.SelectionColor)
	set(&t.PlaceholderColor, s.PlaceholderColor)
	set(&t.Fill, s.Fill)
	set(&t.Stroke, s.Stroke)
	set(&t.StrokeWidth, s.StrokeWidth)
	if s.StrokeDash != nil {
		t.StrokeDash = dashKey(s.StrokeDash)
	}
	set(&t.WhiteSpace, s.WhiteSpace)
	set(&t.UserSelect, s.UserSelect)
	set(&t.WritingDirection, s.WritingDirection)
	set(&t.LayoutDirection, s.LayoutDirection)
	if t.LineHeightEm != 0 {
		size := t.FontSize
		if size == 0 {
			size = DefaultFontSize
		}
		t.LineHeight, t.LineHeightEm = Points(t.LineHeightEm)*size, 0
	}
	return t
}

// dashKey encodes dashes as a comparable string: "d" and the bits of
// each length.
func dashKey(dashes []Points) string {
	b := make([]byte, 1, 1+4*len(dashes))
	b[0] = 'd'
	for _, l := range dashes {
		b = binary.LittleEndian.AppendUint32(b, math.Float32bits(float32(l)))
	}
	return string(b)
}

// paint returns the SVG paint properties the text fields carry.
func (s *textFields) paint() driver.PaintStyle {
	p := driver.PaintStyle{Fill: s.Fill, Stroke: s.Stroke, StrokeWidth: s.StrokeWidth}
	if k := s.StrokeDash; k != "" {
		p.StrokeDash = make([]Points, 0, (len(k)-1)/4)
		for i := 1; i+4 <= len(k); i += 4 {
			p.StrokeDash = append(p.StrokeDash, Points(math.Float32frombits(binary.LittleEndian.Uint32([]byte(k[i:i+4])))))
		}
	}
	return p
}

// textShadow is the shadow the text fields describe, if any.
func (s *textFields) textShadow() driver.TextShadow {
	if s.TextShadowColor == 0 && s.TextShadowOffset == (Point{}) && s.TextShadowRadius == 0 {
		return driver.TextShadow{}
	}
	c := s.TextShadowColor
	if c == 0 {
		c = Color(0x00000055)
	}
	if c&0xff == 0 {
		return driver.TextShadow{}
	}
	return driver.TextShadow{Color: c, Offset: s.TextShadowOffset, Radius: max(s.TextShadowRadius, 0)}
}

// textStyle resolves inherited text fields into what a text engine needs.
func (s *textFields) textStyle() driver.TextStyle {
	t := driver.TextStyle{
		Font: Font{
			Family: s.FontFamily,
			Size:   s.FontSize,
			Weight: s.FontWeight,
			Italic: s.FontStyle == FontStyleItalic,
			// The normal bit only marks a variant as set.
			Variant: s.FontVariant &^ FontVariantNormal,
		},
		Color:         s.Color,
		LineHeight:    s.LineHeight,
		LetterSpacing: s.LetterSpacing,
		Align:         s.TextAlign,
		Decoration:    s.TextDecoration &^ TextDecorationNone,
	}
	if t.Decoration != 0 {
		if s.TextDecorationStyle > DecorationSolid {
			t.DecorationStyle = s.TextDecorationStyle
		}
		t.DecorationColor = s.TextDecorationColor
	}
	if t.Font.Size == 0 {
		t.Font.Size = DefaultFontSize
	}
	if t.Decoration != 0 {
		// Percentages are of the font size, as CSS's are of 1em.
		em := func(l Length) (Points, bool) {
			if p, ok := l.Points(); ok {
				return p, true
			}
			if pct, ok := l.Percent(); ok {
				return t.Font.Size * Points(pct) / 100, true
			}
			return 0, false
		}
		if p, ok := em(s.TextDecorationThickness); ok {
			t.DecorationThickness = max(p, 0)
		}
		if t.Decoration&TextDecorationUnderline != 0 {
			if p, ok := em(s.TextUnderlineOffset); ok {
				t.UnderlineOffset = Pt(p)
			}
		}
	}
	if s.LineHeightMultiple > 0 {
		t.LineHeight = Points(s.LineHeightMultiple) * t.Font.Size
	}
	if t.Font.Weight == 0 {
		t.Font.Weight = FontWeightNormal
	}
	if t.Color == 0 {
		t.Color = RGB(0x000000)
	}
	if t.Align == 0 {
		t.Align = TextAlignStart
	}
	return t
}

// viewStyle is what platforms draw for a view of size whose text color
// is color, laid out right to left when rtl.
func (s *Style) viewStyle(size Size, color Color, rtl bool) driver.ViewStyle {
	cc := func(c Color) Color {
		if c == CurrentColor {
			return color
		}
		return c
	}
	v := driver.ViewStyle{
		Background:        cc(s.Background),
		CornerRadius:      s.CornerRadius,
		ContinuousCorners: s.BorderCurve == BorderCurveContinuous,
		BorderWidth:       s.BorderWidth,
		BorderColor:       cc(s.BorderColor),
		Opacity:           s.Opacity.Value(),
		Clip:              s.clips(),
		ShadowColor:       cc(s.ShadowColor),
		ShadowOffset:      s.ShadowOffset,
		ShadowRadius:      s.ShadowRadius,
		TintColor:         cc(s.TintColor),
		Cursor:            s.Cursor,
		Transform:         s.Transform,
		BackfaceHidden:    s.BackfaceHidden,
		ChildPerspective:  max(s.Perspective, 0),
		BlendMode:         s.BlendMode,
		Glass:             s.GlassEffect,
		Transition:        s.Transition,
		Animation:         s.Animation,
		Hidden:            s.Visibility == VisibilityHidden,
	}
	if c := s.CornerRadii; c != (Corners{}) {
		if rtl {
			c.TopLeft, c.TopRight, c.BottomLeft, c.BottomRight = c.TopRight, c.TopLeft, c.BottomRight, c.BottomLeft
		}
		if c.Uniform() {
			v.CornerRadius = c.TopLeft
		} else {
			v.CornerRadius, v.Corners = 0, c
		}
	}
	if s.Borders != (Edges{}) || s.BorderColors != (EdgeColors{}) {
		w, c := s.borders(), s.BorderColors.Or(s.BorderColor)
		for _, side := range []*Color{&c.Top, &c.Right, &c.Bottom, &c.Left} {
			*side = cc(*side)
		}
		if rtl {
			w.Left, w.Right = w.Right, w.Left
			c.Left, c.Right = c.Right, c.Left
		}
		if w == All(w.Top) && c == AllColors(c.Top) {
			v.BorderWidth, v.BorderColor = w.Top, c.Top
		} else {
			v.BorderWidth, v.BorderColor, v.Borders, v.BorderColors = 0, 0, w, c
		}
	}
	switch s.BorderStyle {
	case BorderNone, BorderHidden:
		v.BorderWidth, v.Borders, v.BorderColors = 0, Edges{}, EdgeColors{}
	case BorderDashed, BorderDotted:
		v.BorderStyle = s.BorderStyle
	}
	if s.OutlineWidth > 0 {
		o := driver.Outline{Width: s.OutlineWidth, Offset: s.OutlineOffset, Color: cc(s.OutlineColor), Style: s.OutlineStyle}
		if s.OutlineColor == 0 {
			o.Color = color
		}
		if o.Style == BorderSolid {
			o.Style = 0
		}
		if o.Visible() {
			v.Outline = o
		}
	}
	t := &v.Transform
	t.TranslateX += Points(t.TranslateXPercent) * size.Width / 100
	t.TranslateY += Points(t.TranslateYPercent) * size.Height / 100
	t.TranslateXPercent, t.TranslateYPercent = 0, 0
	if s.TransformOrigin != (ObjectPosition{}) {
		at := s.TransformOrigin.Place(size, Size{})
		v.Origin = Point{X: at.X - size.Width/2, Y: at.Y - size.Height/2}
	}
	v.Effects = s.effects(size, cc)
	return v
}

// effects resolves the effects s draws on a view of size, nil when it
// draws none.
func (s *Style) effects(size Size, cc func(Color) Color) *driver.Effects {
	if s.Shadows == nil && s.Gradients == nil && s.Filters == nil && s.BackdropFilters == nil && s.Mask == nil {
		return nil
	}
	e := &driver.Effects{}
	for _, sh := range s.Shadows {
		if sh.Color = cc(sh.Color); sh.Color&0xff != 0 {
			e.Shadows = append(e.Shadows, sh)
		}
	}
	// An animated background replaces the gradients, as a state's does.
	if s.Animated == nil || s.Animated.Background == nil {
		for i := range s.Gradients {
			if p, ok := s.Gradients[i].Paint(size); ok {
				e.Gradients = append(e.Gradients, p)
			}
		}
	}
	for _, list := range [2]struct {
		in  []Filter
		out *[]Filter
	}{{s.Filters, &e.Filters}, {s.BackdropFilters, &e.BackdropFilters}} {
		for _, f := range list.in {
			f.Shadow.Color = cc(f.Shadow.Color)
			*list.out = append(*list.out, f)
		}
	}
	if s.Mask != nil {
		if p, ok := s.Mask.Paint(size); ok {
			e.Mask = &p
		}
	}
	if e.Empty() {
		return nil
	}
	return e
}

// overflow returns the overflow on each axis.
func (s *Style) overflow() (x, y Overflow) {
	x, y = s.Overflow, s.Overflow
	if s.OverflowX != 0 {
		x = s.OverflowX
	}
	if s.OverflowY != 0 {
		y = s.OverflowY
	}
	return x, y
}

// clips reports whether the node clips its children when drawn.
func (s *Style) clips() bool {
	x, y := s.overflow()
	return x == Hidden || x == OverflowScroll || y == Hidden || y == OverflowScroll
}
