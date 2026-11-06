package main

import "github.com/oneo-me/nsgo/examples/gallery/locales"

// mark is what a menu item's checkbox shows.
type mark uint8

const (
	markNone mark = iota
	markCheck
	markDash
)

// onOff is a setting's state in words.
func onOff(tr locales.Messages, on bool) string {
	if on {
		return tr.CommonOn()
	}
	return tr.CommonOff()
}
