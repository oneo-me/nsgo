//go:build ios

package ios

import (
	"fmt"
	"math"
	"runtime"
	"strings"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/platform/apple"
)

// Accessibility: VoiceOver reads each view's UIAccessibility properties,
// set here from its props, events and text.
//
// - A view is an element when its role says so; without a role, text and
//   pressable, labelled or Accessible views are. An element's subviews
//   are not read, so an Accessible view is one element (the core labels
//   it with its text). AccessibleNo views are none, and hidden ones hide
//   their subtree.
// - The value is Text, else a range's percentage, then the checked and
//   expanded states, which UIKit has no traits for.
// - Increment and decrement are the adjustable trait's swipes, escape
//   and magic tap VoiceOver's gestures; custom actions are
//   UIAccessibilityCustomActions.
// - Live regions are announced from here: a changed label, or a changed
//   text inside one.

var (
	selSetAccessibilityHint           = sel("setAccessibilityHint:")
	selSetAccessibilityElementsHidden = sel("setAccessibilityElementsHidden:")
	selCustomAction                   = sel("nsgoAccessibilityAction:")
	layoutChanged                     uint32 // UIAccessibilityLayoutChangedNotification
)

var traitNames = []string{"Button", "Link", "Header", "Image", "StaticText", "Selected", "NotEnabled", "TabBar",
	"Adjustable", "UpdatesFrequently"}

func loadAccessibility(uikit uintptr) {
	for _, name := range []string{"UIAccessibilityAnnouncementNotification", "UIAccessibilityLayoutChangedNotification"} {
		if addr, err := purego.Dlsym(uikit, name); err == nil {
			n := **(**uint32)(unsafe.Pointer(&addr))
			if name == "UIAccessibilityAnnouncementNotification" {
				announcement = n
			} else {
				layoutChanged = n
			}
		}
	}
	for _, name := range traitNames {
		addr, err := purego.Dlsym(uikit, "UIAccessibilityTrait"+name)
		if err != nil {
			panic(err)
		}
		traits[name] = **(**uint64)(unsafe.Pointer(&addr))
	}
}

// accessMethods are what every view answers VoiceOver's actions with.
func accessMethods() []objc.MethodDef {
	standard := func(name string) func(objc.ID, objc.SEL) bool {
		return func(self objc.ID, _ objc.SEL) bool {
			v := views[self]
			return v != nil && v.action(name)
		}
	}
	return []objc.MethodDef{
		{Cmd: sel("accessibilityActivate"), Fn: accessibilityActivate},
		{Cmd: sel("accessibilityIncrement"), Fn: func(self objc.ID, _ objc.SEL) {
			if v := views[self]; v != nil {
				v.action(driver.ActionIncrement)
			}
		}},
		{Cmd: sel("accessibilityDecrement"), Fn: func(self objc.ID, _ objc.SEL) {
			if v := views[self]; v != nil {
				v.action(driver.ActionDecrement)
			}
		}},
		{Cmd: sel("accessibilityPerformEscape"), Fn: accessibilityEscape},
		{Cmd: sel("accessibilityPerformMagicTap"), Fn: standard(driver.ActionMagicTap)},
		{Cmd: selCustomAction, Fn: accessibilityCustom},
		{Cmd: sel("accessibilityLabel"), Fn: accessibilityLabel},
	}
}

func nsOrNil(s string) objc.ID {
	if s == "" {
		return 0
	}
	return apple.NSString(s)
}

// setProps applies new props, announcing a live region's new label.
func (v *view) setProps(p *driver.ViewProps) {
	old := v.props
	v.props = *p
	if p.TestID != old.TestID {
		v.native.Send(selSetAccessibilityIdentifier, nsOrNil(p.TestID))
	}
	if p.Appearance != old.Appearance {
		v.native.Send(sel("setOverrideUserInterfaceStyle:"), uiStyle(p.Appearance))
	}
	v.updateAccessibility()
	if v.propsSet && p.Label != old.Label {
		v.announceLive(p.Label)
	}
	v.propsSet = true
}

// setLabel is ItemAccessibilityLabel.
func (v *view) setLabel(label string) {
	if label == v.props.Label {
		return
	}
	v.props.Label = label
	v.announceLive(label)
}

// label is what VoiceOver reads for v: its label, else a text's text.
func (v *view) label() string {
	if v.props.Label == "" && v.kind == driver.KindText {
		return v.plain
	}
	return v.props.Label
}

// spokenLabel is what VoiceOver reads for v: its label, a text's text,
// or the texts inside it, as React Native's elements without a label.
func (v *view) spokenLabel() string {
	if l := v.label(); l != "" || v.kind == driver.KindText || !v.axElement {
		return l
	}
	var parts []string
	collectText(v.native, &parts)
	return strings.Join(parts, " ")
}

func collectText(native objc.ID, out *[]string) {
	for _, c := range apple.Subviews(native) {
		if objc.Send[bool](c, sel("isHidden")) {
			continue
		}
		if cv := views[c]; cv != nil && cv.kind == driver.KindText {
			if l := cv.label(); l != "" {
				*out = append(*out, l)
			}
			continue
		}
		collectText(c, out)
	}
}

// accessibilityLabel answers VoiceOver with spokenLabel, read when asked
// so that it follows the texts inside.
func accessibilityLabel(self objc.ID, cmd objc.SEL) objc.ID {
	if v := views[self]; v != nil && v.id != 0 {
		return nsOrNil(v.spokenLabel())
	}
	return apple.SendSuper[objc.ID](self, cmd)
}

// textChanged follows ItemText: a text inside a live region is announced.
func (v *view) textChanged(text string) {
	if text == v.plain {
		return
	}
	first := !v.textSet
	v.plain, v.textSet = text, true
	if first {
		return
	}
	for p := v; p != nil; p = p.parent() {
		if p.props.LiveRegion != driver.LiveRegionNone {
			uiAccessibilityPost(announcement, apple.NSString(text))
			return
		}
	}
}

// parent returns the view v is mounted in, if any.
func (v *view) parent() *view {
	for n := v.native.Send(selSuperview); n != 0; n = n.Send(selSuperview) {
		if p := views[n]; p != nil {
			if p.id == 0 {
				return nil // the surface
			}
			return p
		}
	}
	return nil
}

func (v *view) announceLive(text string) {
	if v.props.LiveRegion != driver.LiveRegionNone && v.kind != driver.KindText && text != "" {
		uiAccessibilityPost(announcement, apple.NSString(text))
	}
}

// updateAccessibility sets v's properties from its props, events and text.
func (v *view) updateAccessibility() {
	native, p := v.native, &v.props
	// VoiceOver keeps to a dialog while it shows.
	native.Send(sel("setAccessibilityViewIsModal:"), p.Role == driver.RoleDialog)
	native.Send(selSetAccessibilityElementsHidden, p.ElementsHidden)
	const noContainer, semanticGroup = 0, 4
	pressable := v.events.Has(driver.EventPress) || v.events.Has(driver.EventLongPress)
	var t uint64
	switch p.Role {
	case driver.RoleRadioGroup, driver.RoleTabList, driver.RoleMenu, driver.RoleDialog, driver.RoleAlert,
		driver.RoleTable, driver.RoleGrid, driver.RoleRowGroup, driver.RoleRow, driver.RoleListbox, driver.RoleGroup,
		driver.RoleRegion, driver.RoleNavigation, driver.RoleTree, driver.RoleToolbar:
		// Groups are containers, so their children stay accessible.
		native.Send(selSetIsAccessibilityElement, false)
		v.axElement = false
		native.Send(selSetAccessibilityContainerType, semanticGroup)
		if p.Role == driver.RoleTabList {
			native.Send(selSetAccessibilityTraits, traits["TabBar"])
		}
		return
	case driver.RoleNone:
		if pressable {
			t = traits["Button"]
		}
	case driver.RoleButton, driver.RoleCheckbox, driver.RoleSwitch, driver.RoleRadio, driver.RoleTab, driver.RoleMenuItem,
		driver.RoleMenuItemCheckbox, driver.RoleMenuItemRadio, driver.RoleOption, driver.RoleTreeItem, driver.RoleCombobox:
		// UIKit has no option or tree item trait: they are activated, and
		// the chosen one carries Selected.
		t = traits["Button"]
	case driver.RoleLink:
		t = traits["Link"]
	case driver.RoleHeader, driver.RoleColumnHeader:
		t = traits["Header"]
	case driver.RoleImage:
		t = traits["Image"]
	case driver.RoleText:
		t = traits["StaticText"]
	case driver.RoleAdjustable:
		t = traits["Adjustable"]
	case driver.RoleProgressBar:
		t = traits["UpdatesFrequently"]
	}
	if p.HasAction(driver.ActionIncrement) || p.HasAction(driver.ActionDecrement) {
		t |= traits["Adjustable"]
	}
	// A toggle button that is on reads as selected, as WebKit reads
	// aria-pressed.
	if p.Selected || p.Pressed {
		t |= traits["Selected"]
	}
	if p.Busy {
		t |= traits["UpdatesFrequently"]
	}
	if p.Disabled {
		t |= traits["NotEnabled"]
	}
	element := p.Role != driver.RoleNone || v.kind == driver.KindText || pressable || p.Label != "" ||
		p.Accessible == driver.AccessibleYes
	if p.Accessible == driver.AccessibleNo || p.Role == driver.RoleTextField {
		element = false
	}
	v.axElement = element
	if len(v.axNodes) > 0 {
		// A canvas with named parts is their container.
		element = false
	}
	native.Send(selSetIsAccessibilityElement, element)
	native.Send(selSetAccessibilityContainerType, noContainer)
	if !element {
		return
	}
	native.Send(selSetAccessibilityTraits, t)
	native.Send(selSetAccessibilityHint, nsOrNil(p.Hint))
	native.Send(selSetAccessibilityValue, nsOrNil(spokenValue(p)))
	var custom []objc.ID
	for _, a := range p.ActionList() {
		if driver.IsStandardAction(a.Name) {
			continue
		}
		label := a.Label
		if label == "" {
			label = a.Name
		}
		act := objc.ID(objc.GetClass("UIAccessibilityCustomAction")).Send(selAlloc).Send(
			sel("initWithName:target:selector:"), apple.NSString(label), native, selCustomAction)
		custom = append(custom, act)
	}
	if len(custom) > 0 || v.axCustom {
		v.axCustom = len(custom) > 0
		arr := objc.ID(0)
		if len(custom) > 0 {
			arr = objc.ID(objc.GetClass("NSArray")).Send(sel("arrayWithObjects:count:"),
				unsafe.Pointer(&custom[0]), uint(len(custom)))
			runtime.KeepAlive(custom)
		}
		native.Send(sel("setAccessibilityCustomActions:"), arr)
		for _, a := range custom {
			a.Send(selRelease)
		}
	}
}

// spokenValue is the value VoiceOver reads: the value's text, else a
// range's percentage, then the checked and expanded states.
func spokenValue(p *driver.ViewProps) string {
	var parts []string
	val := p.Value
	switch {
	case val.Text != "":
		parts = append(parts, val.Text)
	case val.IsRange():
		parts = append(parts, fmt.Sprintf("%g%%", math.Round(float64((val.Now-val.Min)/(val.Max-val.Min)*100))))
	}
	switch p.Role {
	case driver.RoleCheckbox, driver.RoleSwitch, driver.RoleRadio, driver.RoleMenuItemCheckbox, driver.RoleMenuItemRadio:
		switch {
		case p.Indeterminate:
			parts = append(parts, "mixed")
		case p.Checked || p.Selected:
			parts = append(parts, "1")
		default:
			parts = append(parts, "0")
		}
	}
	if p.Expandable {
		if p.Expanded {
			parts = append(parts, "expanded")
		} else {
			parts = append(parts, "collapsed")
		}
	}
	return strings.Join(parts, ", ")
}

// action reports the accessibility action name if v offers it.
func (v *view) action(name string) bool {
	if !v.props.HasAction(name) || !v.events.Has(driver.EventAccessibilityAction) {
		return false
	}
	v.report(driver.Event{Kind: driver.EventAccessibilityAction, Text: name})
	return true
}

// accessibilityEscape answers VoiceOver's escape, the two-finger scrub,
// which UIKit asks of the focused element and then of the views around
// it: a view with the escape action takes it, and the outermost view of
// the surface, the root or a modal UIKit presents apart, goes back as
// Escape and the back button do, closing popups and popping screens.
func accessibilityEscape(self objc.ID, _ objc.SEL) bool {
	v := views[self]
	if v == nil {
		return false
	}
	if v.action(driver.ActionEscape) {
		return true
	}
	if up := views[self.Send(selSuperview)]; up != nil && up.surface == v.surface && up.id != 0 {
		return false
	}
	v.surface.engine.Dispatch(driver.Event{Kind: driver.EventBack})
	return true
}

func accessibilityActivate(self objc.ID, _ objc.SEL) bool {
	v := views[self]
	switch {
	case v == nil:
		return false
	case v.action(driver.ActionActivate):
		return true
	case !v.events.Has(driver.EventPress):
		return false
	}
	v.report(driver.Event{Kind: driver.EventPress, Index: -1})
	return true
}

// accessibilityCustom runs the custom action whose name VoiceOver chose.
func accessibilityCustom(self objc.ID, _ objc.SEL, action objc.ID) bool {
	v := views[self]
	if v == nil {
		return false
	}
	label := apple.GoString(action.Send(sel("name")))
	for _, a := range v.props.ActionList() {
		if a.Label == label || (a.Label == "" && a.Name == label) {
			return v.action(a.Name)
		}
	}
	return false
}

// focusAccessibility moves VoiceOver to v.
func (v *view) focusAccessibility() {
	target := v.native
	if v.input != nil {
		target = v.input.field
	}
	uiAccessibilityPost(layoutChanged, target)
}
