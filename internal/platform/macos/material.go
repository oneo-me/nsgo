//go:build darwin && !ios

package macos

import (
	"github.com/ebitengine/purego/objc"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/platform/apple"
)

// The window's background material is an NSVisualEffectView behind the
// app's views, blending with what is behind the window. The window itself
// gives up its opaque background so the material shows where the app draws
// nothing opaque, and takes it back without one. NSVisualEffectMaterial
// values follow AppKit's.

var (
	selSetMaterial         = sel("setMaterial:")
	selSetBlendingMode     = sel("setBlendingMode:")
	selSetState            = sel("setState:")
	selSetAutoresizingMask = sel("setAutoresizingMask:")
	selSetOpaque           = sel("setOpaque:")
	selClearColor          = sel("clearColor")
)

const (
	blendingBehindWindow = 0
	effectStateActive    = 1
	widthHeightSizable   = 2 | 16 // NSViewWidthSizable | NSViewHeightSizable
	viewBelow            = -1     // NSWindowBelow
)

// materialCode returns m's NSVisualEffectMaterial, and whether macOS has
// it.
func materialCode(m driver.WindowMaterial) (int, bool) {
	switch m {
	case driver.WindowMaterialWindow:
		return 12, true // windowBackground
	case driver.WindowMaterialContent:
		return 18, true // contentBackground
	case driver.WindowMaterialSidebar:
		return 7, true // sidebar
	case driver.WindowMaterialHeader:
		return 10, true // headerView
	case driver.WindowMaterialTitlebar:
		return 3, true // titlebar
	case driver.WindowMaterialUnderWindow:
		return 21, true // underWindowBackground
	case driver.WindowMaterialUnderPage:
		return 22, true // underPageBackground
	case driver.WindowMaterialMenu:
		return 5, true // menu
	case driver.WindowMaterialPopover:
		return 6, true // popover
	case driver.WindowMaterialHUD:
		return 13, true // hudWindow
	case driver.WindowMaterialSheet:
		return 11, true // sheet
	case driver.WindowMaterialTooltip:
		return 17, true // toolTip
	case driver.WindowMaterialSelection:
		return 4, true // selection
	case driver.WindowMaterialFullScreenUI:
		return 15, true // fullScreenUI
	// Windows 11's backdrops: the closest materials.
	case driver.WindowMaterialMica, driver.WindowMaterialTabbed:
		return 21, true // underWindowBackground
	case driver.WindowMaterialAcrylic:
		return 13, true // hudWindow
	}
	return 0, false
}

// setMaterial puts m behind the app's views, or takes the material away
// and makes the window opaque again for WindowMaterialNone.
func (s *surface) setMaterial(m driver.WindowMaterial) {
	code, ok := materialCode(m)
	if !ok {
		if s.effect != 0 {
			s.effect.Send(sel("removeFromSuperview"))
			s.effect = 0
			s.window.Send(selSetOpaque, true)
			s.window.Send(selSetBackgroundColor, objc.ID(apple.Class("NSColor")).Send(sel("windowBackgroundColor")))
		}
		s.material = driver.WindowMaterialNone
		return
	}
	if s.effect == 0 {
		s.effect = objc.ID(classMaterial).Send(selAlloc).Send(sel("initWithFrame:"),
			objc.Send[apple.CGRect](s.native, selBounds))
		s.effect.Send(selSetBlendingMode, blendingBehindWindow)
		s.effect.Send(selSetState, effectStateActive)
		s.effect.Send(selSetAutoresizingMask, widthHeightSizable)
		s.native.Send(selAddSubviewPositioned, s.effect, viewBelow, objc.ID(0))
		s.effect.Send(selRelease)
		s.window.Send(selSetOpaque, false)
		s.window.Send(selSetBackgroundColor, objc.ID(apple.Class("NSColor")).Send(selClearColor))
	}
	s.effect.Send(selSetMaterial, code)
	s.material = m
}

// A view's backdrop blur is an NSVisualEffectView under its children that
// blends with what is behind it in the window, or Liquid Glass on macOS 26
// and later. AppKit sets the blur's strength by material, not by radius:
// the radius only turns it on. The material is opaque enough to hide the
// view's own layer, so a view inside it shows the background over the blur,
// where CSS has it.

const blendingWithinWindow = 1

// backdropMaterial is the NSVisualEffectMaterial of a backdrop material.
func backdropMaterial(m driver.BackdropMaterial) int {
	switch m {
	case driver.BackdropThin:
		return 6 // popover
	case driver.BackdropRegular:
		return 5 // menu
	case driver.BackdropThick:
		return 11 // sheet
	case driver.BackdropChrome:
		return 3 // titlebar
	}
	return 15 // fullScreenUI
}

// keepGlassActive keeps Liquid Glass as it is in a window in the back or
// an app not in front, as a blur does at effectStateActive: AppKit redraws
// it flat from the window's active appearance, and NSGlassEffectView has
// no state of its own to set. The appearance is the window's, so its title
// bar stays active too, for as long as it lives. A host's window is the
// host's to say.
func (s *surface) keepGlassActive() {
	force := sel("_setForceMainAppearance:")
	if s.glassActive || s.embedded || s.window == 0 || !objc.Send[bool](s.window, sel("respondsToSelector:"), force) {
		return
	}
	s.window.Send(force, true)
	s.glassActive = true
}

// fitBackdrop lays v's blur out for style vs.
func (v *view) fitBackdrop(vs *driver.ViewStyle) {
	b := objc.Send[apple.CGRect](v.native, selBounds)
	apple.FitBackdrop(v.backdrop, vs, b.Size.Width, b.Size.Height)
	if v.tint != 0 {
		v.tint.Send(selSetFrame, objc.Send[apple.CGRect](v.backdrop, selBounds))
	}
}

// setBackdrop gives v the backdrop its style asks for, or takes it away.
func (v *view) setBackdrop(vs *driver.ViewStyle) {
	glass := vs.Glass != driver.GlassNone && apple.Class("NSGlassEffectView") != 0
	want := vs.Effects.BackdropBlur() > 0 || glass
	if !want || glass != v.glass {
		if v.backdrop != 0 {
			v.backdrop.Send(sel("removeFromSuperview"))
			v.backdrop, v.tint = 0, 0
		}
		if !want {
			return
		}
	}
	if v.backdrop == 0 {
		class := classMaterial
		if glass {
			class = apple.Class("NSGlassEffectView")
		}
		v.backdrop = objc.ID(class).Send(selAlloc).Send(sel("initWithFrame:"), objc.Send[apple.CGRect](v.native, selBounds))
		v.backdrop.Send(selSetAutoresizingMask, widthHeightSizable)
		v.native.Send(selAddSubviewPositioned, v.backdrop, viewBelow, objc.ID(0))
		v.backdrop.Send(selRelease)
		v.glass = glass
	}
	if glass {
		v.surface.keepGlassActive()
		v.backdrop.Send(sel("setStyle:"), int(vs.Glass)-1)
		v.backdrop.Send(sel("setCornerRadius:"), float64(max(vs.CornerRadius, vs.Corners.TopLeft)))
		return
	}
	v.backdrop.Send(selSetMaterial, backdropMaterial(vs.Material))
	v.backdrop.Send(selSetBlendingMode, blendingWithinWindow)
	v.backdrop.Send(selSetState, effectStateActive)
	v.backdrop.Send(sel("setWantsLayer:"), true)
	v.fitBackdrop(vs)
	if v.tint == 0 {
		v.tint = objc.ID(apple.Class("NSView")).Send(selAlloc).Send(sel("initWithFrame:"), objc.Send[apple.CGRect](v.backdrop, selBounds))
		v.tint.Send(selSetAutoresizingMask, widthHeightSizable)
		v.tint.Send(sel("setWantsLayer:"), true)
		v.backdrop.Send(sel("addSubview:"), v.tint)
		v.tint.Send(selRelease)
	}
	c := apple.CGColor(vs.Background)
	v.tint.Send(selLayer).Send(selSetBackgroundColor, c)
	apple.Release(c)
}
