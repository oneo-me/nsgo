//go:build darwin && !ios

package macos

import "github.com/ebitengine/purego/objc"

// NSUserInterfaceLayoutDirection's directions.
const (
	layoutLTR = 0
	layoutRTL = 1
)

var selSetLayoutDirection = sel("setUserInterfaceLayoutDirection:")

func (s *surface) layoutDirection() int {
	if s.rtl {
		return layoutRTL
	}
	return layoutLTR
}

// directTree lays a view and the views inside it out in the app's
// direction, as its language is written, whatever the system's: the
// fields, scroll bars and native components the surface shows.
func (s *surface) directTree(v objc.ID) {
	if !s.directed || v == 0 {
		return
	}
	v.Send(selSetLayoutDirection, s.layoutDirection())
	subs := v.Send(sel("subviews"))
	n := objc.Send[uint](subs, sel("count"))
	for i := uint(0); i < n; i++ {
		s.directTree(subs.Send(sel("objectAtIndex:"), i))
	}
}

// directMenu lays a menu the surface shows, and its submenus, out in the
// app's direction.
func (s *surface) directMenu(m objc.ID) {
	if !s.directed || m == 0 {
		return
	}
	m.Send(selSetLayoutDirection, s.layoutDirection())
	items := m.Send(sel("itemArray"))
	n := objc.Send[uint](items, sel("count"))
	for i := uint(0); i < n; i++ {
		s.directMenu(items.Send(sel("objectAtIndex:"), i).Send(sel("submenu")))
	}
}
