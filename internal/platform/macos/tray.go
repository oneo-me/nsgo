//go:build darwin && !ios

package macos

import (
	"time"
	"unsafe"

	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
)

// Tray icons are status items in the menu bar. Their button reports
// primary and secondary clicks (a Control-click is secondary) to the
// surface that added them; an icon that draws itself holds its view's
// surface in the button, which lets the clicks through.

// tray is a status item.
type tray struct {
	cfg   driver.TrayConfig
	item  objc.ID
	owner *surface // the surface that added it, which hears its clicks
	view  *surface // the surface it draws itself with
	// blurred is when a panel under it hid as it lost the focus: the
	// click on the icon that made it hide does not show it again.
	blurred time.Time
}

// trays are the status items by ID; trayButtons by their button. Only
// the main thread uses them.
var (
	trays       = map[uint32]*tray{}
	trayButtons = map[objc.ID]*tray{}
	classTray   objc.Class
	trayTarget  objc.ID
	selTray     = sel("nsgoTrayClicked:")
)

// applyTray adds, changes or removes the tray icon cfg describes.
func (s *surface) applyTray(cfg driver.TrayConfig) {
	tr := trays[cfg.ID]
	if cfg.Removed {
		if tr != nil {
			tr.remove()
		}
		return
	}
	if tr == nil {
		tr = &tray{owner: s}
		tr.item = statusBar().Send(sel("statusItemWithLength:"), float64(-1)).Send(sel("retain"))
		if trayTarget == 0 {
			classTray = register("NsgoTrayTarget", apple.Class("NSObject"), nil, []objc.MethodDef{
				{Cmd: selTray, Fn: trayClicked},
			})
			trayTarget = objc.ID(classTray).Send(selNew)
		}
		button := tr.item.Send(sel("button"))
		button.Send(selSetTarget, trayTarget)
		button.Send(selSetAction, selTray)
		const leftUp, rightUp = 1 << 2, 1 << 4
		button.Send(sel("sendActionOn:"), uint64(leftUp|rightUp))
		trays[cfg.ID] = tr
		trayButtons[button] = tr
	}
	tr.cfg = cfg
	button := tr.item.Send(sel("button"))
	var image objc.ID
	if len(cfg.Icon) > 0 && cfg.View == 0 {
		data := objc.ID(apple.Class("NSData")).Send(sel("dataWithBytes:length:"),
			unsafe.Pointer(&cfg.Icon[0]), uint(len(cfg.Icon)))
		image = objc.ID(apple.Class("NSImage")).Send(selAlloc).Send(sel("initWithData:"), data)
		if image != 0 {
			// As tall as the menu bar's icons, keeping its proportions.
			size := objc.Send[apple.CGSize](image, sel("size"))
			if size.Height > 0 {
				const height = 18
				image.Send(sel("setSize:"), apple.CGSize{Width: size.Width * height / size.Height, Height: height})
			}
			image.Send(sel("setTemplate:"), cfg.Template)
			defer image.Send(selRelease)
		}
	}
	button.Send(sel("setImage:"), image)
	button.Send(sel("setTitle:"), apple.NSString(cfg.Title))
	const imageLeft = 2
	button.Send(sel("setImagePosition:"), imageLeft)
	if cfg.Tooltip != "" {
		button.Send(sel("setToolTip:"), apple.NSString(cfg.Tooltip))
	} else {
		button.Send(sel("setToolTip:"), objc.ID(0))
	}
	if tr.view != nil {
		tr.show(tr.view)
	}
}

// show puts the surface the icon draws itself with in its button.
func (tr *tray) show(view *surface) {
	tr.view = view
	button := tr.item.Send(sel("button"))
	size := objc.Send[apple.CGRect](view.native, selBounds).Size
	tr.item.Send(sel("setLength:"), size.Width)
	if view.native.Send(selSuperview) != button {
		button.Send(selAddSubview, view.native)
	}
	bounds := objc.Send[apple.CGRect](button, selBounds)
	view.native.Send(selSetFrame, apple.Rect(0, (bounds.Size.Height-size.Height)/2, size.Width, size.Height))
}

// remove takes the icon out of the menu bar.
func (tr *tray) remove() {
	delete(trays, tr.cfg.ID)
	delete(trayButtons, tr.item.Send(sel("button")))
	statusBar().Send(sel("removeStatusItem:"), tr.item)
	tr.item.Send(selRelease)
	tr.item = 0
}

// removeTrays takes every icon away as the app ends.
func removeTrays() {
	for _, tr := range trays {
		tr.remove()
	}
}

func statusBar() objc.ID {
	return objc.ID(apple.Class("NSStatusBar")).Send(sel("systemStatusBar"))
}

// trayClicked reports a click on an icon's button: a right click or a
// Control-click is secondary.
func trayClicked(_ objc.ID, _ objc.SEL, button objc.ID) {
	tr := trayButtons[button]
	if tr == nil || tr.owner.engine == nil || tr.owner.closed {
		return
	}
	event := nsApp().Send(sel("currentEvent"))
	const rightUp, control = 4, 1 << 18
	index := 0
	if event != 0 && (objc.Send[uint](event, sel("type")) == rightUp ||
		objc.Send[uint](event, sel("modifierFlags"))&control != 0) {
		index = 1
	}
	if index == 0 && time.Since(tr.blurred) < 300*time.Millisecond {
		return
	}
	tr.owner.engine.Dispatch(driver.Event{Kind: driver.EventTray, Count: tr.cfg.ID, Index: index})
}

// trayMenu shows entries as icon id's menu, and reports it closed as a
// context menu.
func (s *surface) trayMenu(id uint32, entries []driver.MenuEntry, popup uint32) {
	tr := trays[id]
	if tr == nil {
		s.engine.Dispatch(driver.Event{Kind: driver.EventMenuClosed, Count: popup})
		return
	}
	s.ensureMenuTarget()
	menu := s.submenu("", entries)
	apple.OnMain(func() {
		// The status item tracks its own menu, under its button, which
		// returns once it closed.
		if tr.item != 0 && !finished {
			tr.item.Send(sel("setMenu:"), menu)
			tr.item.Send(sel("button")).Send(sel("performClick:"), objc.ID(0))
			tr.item.Send(sel("setMenu:"), objc.ID(0))
		}
		menu.Send(selRelease)
		s.engine.Dispatch(driver.Event{Kind: driver.EventMenuClosed, Count: popup})
	})
}
