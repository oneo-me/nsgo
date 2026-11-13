//go:build darwin && !ios

package macos

import (
	"runtime"
	"strings"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
)

// Accessibility: VoiceOver reads each view's NSAccessibility attributes,
// set here from its props, events and text.
//
// - A view is an element when its role says so; without a role, text is
//   static text, a pressable view a button, and a labelled or Accessible
//   view a group. AccessibleNo and hidden views are none.
// - An Accessible view or a hidden one has no accessibility children, so
//   VoiceOver reads it as one element (the core labels it with its text).
// - Ranges set the value and its bounds, Text its description; the
//   increment, decrement and escape actions are AXIncrement, AXDecrement
//   and AXCancel, custom ones NSAccessibilityCustomActions.
// - Live regions are announced from here: a changed label, or a changed
//   text inside one.

const appKitPath = "/System/Library/Frameworks/AppKit.framework/AppKit"

var (
	selSetAccessibilityHidden   = sel("setAccessibilityHidden:")
	selSetAccessibilityHelp     = sel("setAccessibilityHelp:")
	selSetAccessibilityExpanded = sel("setAccessibilityExpanded:")
	selCustomAction             = sel("nsgoAccessibilityAction:")

	axPerform = map[objc.SEL]string{
		sel("accessibilityPerformIncrement"): driver.ActionIncrement,
		sel("accessibilityPerformDecrement"): driver.ActionDecrement,
		sel("accessibilityPerformCancel"):    driver.ActionEscape,
	}
)

// axPost is NSAccessibilityPostNotificationWithUserInfo.
var axPost = sync.OnceValue(func() func(element, name, info objc.ID) {
	lib, err := purego.Dlopen(appKitPath, purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return func(_, _, _ objc.ID) {}
	}
	var post func(element, name, info objc.ID)
	purego.RegisterLibFunc(&post, lib, "NSAccessibilityPostNotificationWithUserInfo")
	return post
})

// accessMethods are what every view answers VoiceOver's actions with.
func accessMethods() []objc.MethodDef {
	defs := []objc.MethodDef{
		{Cmd: sel("accessibilityPerformPress"), Fn: accessibilityPress},
		{Cmd: sel("isAccessibilitySelectorAllowed:"), Fn: accessibilityAllowed},
		{Cmd: selCustomAction, Fn: accessibilityCustom},
		{Cmd: sel("accessibilityLabel"), Fn: accessibilityLabel},
		{Cmd: sel("accessibilityChildren"), Fn: accessibilityChildren},
	}
	for s, name := range axPerform {
		defs = append(defs, objc.MethodDef{Cmd: s, Fn: func(self objc.ID, _ objc.SEL) bool {
			v := views[self]
			return v != nil && v.action(name)
		}})
	}
	return defs
}

// AppKit roles of core roles: role and subrole.
var roles = map[driver.Role][2]string{
	driver.RoleButton:      {"AXButton", ""},
	driver.RoleLink:        {"AXLink", ""},
	driver.RoleHeader:      {"AXHeading", ""},
	driver.RoleImage:       {"AXImage", ""},
	driver.RoleCheckbox:    {"AXCheckBox", ""},
	driver.RoleSwitch:      {"AXCheckBox", "AXSwitch"},
	driver.RoleRadio:       {"AXRadioButton", ""},
	driver.RoleTab:         {"AXRadioButton", "AXTabButton"},
	driver.RoleText:        {"AXStaticText", ""},
	driver.RoleRadioGroup:  {"AXRadioGroup", ""},
	driver.RoleTabList:     {"AXTabGroup", ""},
	driver.RoleMenu:        {"AXMenu", ""},
	driver.RoleMenuItem:    {"AXMenuItem", ""},
	driver.RoleAdjustable:  {"AXSlider", ""},
	driver.RoleSpinButton:  {"AXIncrementor", ""},
	driver.RoleCombobox:    {"AXPopUpButton", ""},
	driver.RoleProgressBar: {"AXProgressIndicator", ""},
	// WebKit's roles for ARIA's tables.
	driver.RoleTable:        {"AXTable", ""},
	driver.RoleGrid:         {"AXTable", ""},
	driver.RoleRowGroup:     {"AXGroup", ""},
	driver.RoleRow:          {"AXRow", ""},
	driver.RoleColumnHeader: {"AXCell", ""},
	driver.RoleCell:         {"AXCell", ""},
	// WebKit's subroles for ARIA's dialog and alert.
	driver.RoleDialog: {"AXGroup", "AXApplicationDialog"},
	driver.RoleAlert:  {"AXGroup", "AXApplicationAlert"},
	// And for alertdialog.
	driver.RoleAlertDialog: {"AXGroup", "AXApplicationAlertDialog"},
	// A listbox and a tree are lists, as WebKit maps ARIA's. Their items
	// are static text that reports its state: AppKit's rows need a
	// table's columns, and its menu items a menu.
	driver.RoleListbox:  {"AXList", ""},
	driver.RoleTree:     {"AXList", ""},
	driver.RoleOption:   {"AXStaticText", ""},
	driver.RoleTreeItem: {"AXStaticText", ""},
	driver.RoleGroup:    {"AXGroup", ""},
	// ARIA's landmark subroles, as WebKit maps them.
	driver.RoleRegion:           {"AXGroup", "AXLandmarkRegion"},
	driver.RoleNavigation:       {"AXGroup", "AXLandmarkNavigation"},
	driver.RoleToolbar:          {"AXToolbar", ""},
	driver.RoleMenuItemCheckbox: {"AXMenuItem", ""},
	driver.RoleMenuItemRadio:    {"AXMenuItem", ""},
}

// axRole returns the AppKit role of v, or none.
func (v *view) axRole() [2]string {
	p := &v.props
	if len(v.axNodes) > 0 {
		// A canvas with named parts holds them.
		return [2]string{"AXGroup", ""}
	}
	if p.Role == driver.RoleButton && p.Pressed {
		// A button that stays on, as browsers make ARIA's aria-pressed.
		return [2]string{"AXCheckBox", "AXToggle"}
	}
	if r, ok := roles[p.Role]; ok {
		return r
	}
	if r, ok := v.axScrollRole(); ok {
		return r
	}
	switch {
	case v.kind == driver.KindText:
		return roles[driver.RoleText]
	case p.Accessible == driver.AccessibleNo || p.Role == driver.RoleTextField:
		return [2]string{}
	case v.events.Has(driver.EventPress) || v.events.Has(driver.EventLongPress):
		return roles[driver.RoleButton]
	case p.Label != "" || p.Accessible == driver.AccessibleYes:
		return [2]string{"AXGroup", ""}
	}
	return [2]string{}
}

func nsNumber(f float64) objc.ID {
	return objc.ID(apple.Class("NSNumber")).Send(sel("numberWithDouble:"), f)
}

func nsOrNil(s string) objc.ID {
	if s == "" {
		return 0
	}
	return apple.NSString(s)
}

// nsArray returns an autoreleased array of items.
func nsArray(items []objc.ID) objc.ID {
	class := objc.ID(apple.Class("NSArray"))
	if len(items) == 0 {
		return class.Send(sel("array"))
	}
	a := class.Send(sel("arrayWithObjects:count:"), unsafe.Pointer(&items[0]), uint(len(items)))
	runtime.KeepAlive(items)
	return a
}

// setProps applies new props, announcing a live region's new label.
func (v *view) setProps(p *driver.ViewProps) {
	old := v.props
	v.props = *p
	if p.TestID != old.TestID {
		v.native.Send(selSetAccessibilityIdentifier, nsOrNil(p.TestID))
	}
	if p.Appearance != old.Appearance {
		v.native.Send(sel("setAppearance:"), nsAppearance(p.Appearance))
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

// accessibilityChildren hides a group's or a hidden view's subtree.
func accessibilityChildren(self objc.ID, cmd objc.SEL) objc.ID {
	if v := views[self]; v != nil && v.axGrouped {
		return nsArray(nil)
	} else if v != nil && len(v.axNodes) > 0 {
		return nsArray(v.axNodes)
	} else if v != nil && v.sc != nil && v.axElement {
		// A scroll area's bar goes with its content.
		kids := apple.SendSuper[objc.ID](self, cmd)
		if kids == 0 {
			return nsArray([]objc.ID{v.axScrollBar()})
		}
		return kids.Send(sel("arrayByAddingObject:"), v.axScrollBar())
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
	for p := v; p != nil; p = p.up {
		if p.props.LiveRegion != driver.LiveRegionNone {
			announceWith(text, p.props.LiveRegion)
			return
		}
	}
}

func (v *view) announceLive(text string) {
	if v.props.LiveRegion != driver.LiveRegionNone && v.kind != driver.KindText && text != "" {
		announceWith(text, v.props.LiveRegion)
	}
}

// updateAccessibility sets v's attributes from its props, events and text.
func (v *view) updateAccessibility() {
	native, p := v.native, &v.props
	role := v.axRole()
	element := role[0] != "" && !p.ElementsHidden
	native.Send(selSetAccessibilityElement, element)
	v.axElement = element
	native.Send(selSetAccessibilityHidden, p.ElementsHidden)
	v.axGrouped = p.ElementsHidden || (p.Accessible == driver.AccessibleYes && v.kind != driver.KindText)
	if !element {
		return
	}
	native.Send(selSetAccessibilityRole, apple.NSString(role[0]))
	native.Send(selSetAccessibilitySubrole, nsOrNil(role[1]))
	native.Send(selSetAccessibilityHelp, nsOrNil(p.HelpText()))
	native.Send(selSetAccessibilityEnabled, !p.Disabled)
	native.Send(selSetAccessibilitySelected, p.Selected)
	if p.Expandable {
		native.Send(selSetAccessibilityExpanded, p.Expanded)
	}
	val := p.Value
	switch {
	case val.IsRange() || p.Role == driver.RoleAdjustable || p.Role == driver.RoleProgressBar:
		native.Send(sel("setAccessibilityMinValue:"), nsNumber(float64(val.Min)))
		native.Send(sel("setAccessibilityMaxValue:"), nsNumber(float64(val.Max)))
		native.Send(selSetAccessibilityValue, nsNumber(float64(val.Now)))
		native.Send(sel("setAccessibilityValueDescription:"), nsOrNil(val.Text))
	case p.Role == driver.RoleCheckbox || p.Role == driver.RoleSwitch || p.Role == driver.RoleRadio ||
		p.Role == driver.RoleMenuItemCheckbox || p.Role == driver.RoleMenuItemRadio || p.Pressed:
		n := 0
		switch {
		case p.Indeterminate:
			n = 2 // mixed
		case p.Checked || p.Selected || p.Pressed:
			n = 1
		}
		native.Send(selSetAccessibilityValue, objc.ID(apple.Class("NSNumber")).Send(sel("numberWithInt:"), n))
	case val.Text != "":
		native.Send(selSetAccessibilityValue, apple.NSString(val.Text))
	}
	var custom []objc.ID
	for _, a := range p.ActionList() {
		if driver.IsStandardAction(a.Name) {
			continue
		}
		label := a.Label
		if label == "" {
			label = a.Name
		}
		act := objc.ID(apple.Class("NSAccessibilityCustomAction")).Send(selAlloc).Send(
			sel("initWithName:target:selector:"), apple.NSString(label), native, selCustomAction)
		custom = append(custom, act)
	}
	if len(custom) > 0 || v.axCustom {
		v.axCustom = len(custom) > 0
		native.Send(sel("setAccessibilityCustomActions:"), nsArray(custom))
		for _, a := range custom {
			a.Send(selRelease)
		}
	}
}

// action reports the accessibility action name if v offers it.
func (v *view) action(name string) bool {
	if !v.props.HasAction(name) || !v.events.Has(driver.EventAccessibilityAction) {
		return false
	}
	v.report(driver.Event{Kind: driver.EventAccessibilityAction, Text: name})
	return true
}

func accessibilityPress(self objc.ID, _ objc.SEL) bool {
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

// accessibilityAllowed offers VoiceOver only the actions a view has.
func accessibilityAllowed(self objc.ID, cmd objc.SEL, selector objc.SEL) bool {
	if name, ok := axPerform[selector]; ok {
		v := views[self]
		return v != nil && v.props.HasAction(name)
	}
	return apple.SendSuper[bool](self, cmd, selector)
}

// accessibilityCustom runs the custom action whose label VoiceOver chose.
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

// focusAccessibility moves VoiceOver to v, and keyboard focus to a field.
func (v *view) focusAccessibility() {
	target := v.native
	if v.input != nil {
		target = v.input.field
		if w := v.native.Send(selWindow); w != 0 {
			w.Send(selMakeFirstResponer, v.input.field)
		}
	}
	info := objc.ID(apple.Class("NSDictionary")).Send(sel("dictionaryWithObject:forKey:"),
		nsArray([]objc.ID{target}), apple.Global(appKitPath, "NSAccessibilityUIElementsKey"))
	axPost()(target, apple.Global(appKitPath, "NSAccessibilityLayoutChangedNotification"), info)
}

// announce has VoiceOver say text.
func announce(text string) { announceWith(text, driver.LiveRegionAssertive) }

func announceWith(text string, live driver.LiveRegion) {
	priority := 50 // NSAccessibilityPriorityMedium
	if live == driver.LiveRegionAssertive {
		priority = 90 // NSAccessibilityPriorityHigh
	}
	keys := []objc.ID{apple.Global(appKitPath, "NSAccessibilityAnnouncementKey"), apple.Global(appKitPath, "NSAccessibilityPriorityKey")}
	values := []objc.ID{apple.NSString(text), objc.ID(apple.Class("NSNumber")).Send(sel("numberWithInt:"), priority)}
	info := objc.ID(apple.Class("NSDictionary")).Send(sel("dictionaryWithObjects:forKeys:count:"),
		unsafe.Pointer(&values[0]), unsafe.Pointer(&keys[0]), uint(len(keys)))
	app := objc.ID(apple.Class("NSApplication")).Send(sel("sharedApplication"))
	axPost()(app, apple.Global(appKitPath, "NSAccessibilityAnnouncementRequestedNotification"), info)
	runtime.KeepAlive(keys)
	runtime.KeepAlive(values)
}
