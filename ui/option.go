package ui

// Option is one segment of a segmented control; Count shows a quiet
// number after the label when set.
type Option[T comparable] struct {
	Value T
	Label string
	Count func() int
}
