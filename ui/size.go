package ui

import "github.com/oneo-me/nsgo/core"

// Size is a control's size, the same on every device. Its zero value is
// the standard size.
type Size uint8

const (
	// SizeDefault is the standard size, 28 pt tall in 13 pt text.
	SizeDefault Size = iota
	// SizeSmall is 24 pt tall in 12 pt text, for dense rows.
	SizeSmall
	// SizeLarge is 32 pt tall in 14 pt text, for what leads a page.
	SizeLarge
)

// sized picks a value by the size.
func sized[T any](s Size, standard, small, large T) T {
	switch s {
	case SizeSmall:
		return small
	case SizeLarge:
		return large
	}
	return standard
}

// Height is the height of a control of the size, to line others up with
// it.
func (s Size) Height() core.Points { return sized[core.Points](s, ButtonHeight, 24, 32) }

// text is the type of a control of the size.
func (s Size) text() core.Style {
	return sized(s, TypeBody, TypeCaption, core.Style{FontSize: 14, LineHeight: 20})
}

// glyph is an icon's size in a button of the size.
func (s Size) glyph() core.Points { return sized[core.Points](s, 14, 12, 16) }

// mark is an icon's size beside text of the size, and a selection
// control's.
func (s Size) mark() core.Points { return sized(s, iconSize, 14, 18) }

// inset is what a control of the size keeps either side of its text.
func (s Size) inset() core.Points { return sized[core.Points](s, 14, 10, 18) }
