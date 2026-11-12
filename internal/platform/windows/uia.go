//go:build windows

package windows

import (
	"strings"
	"sync/atomic"
	"syscall"
	"unsafe"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// Accessibility is a UI Automation provider tree over the views, as
// NSH's Windows backend has: Windows asks the window for it with
// WM_GETOBJECT, and the answer is the root of server-side providers, one
// per exposed view, made on demand and holding only the window and the
// node, so a node that is gone answers UIA_E_ELEMENTNOTAVAILABLE. With
// UseComThreading they are called on the window's thread.
//
// Which views are elements: texts, labelled images, fields, views that
// take presses, labelled views and every view with a Role; other views
// are flattened, their elements becoming children of the nearest element
// above. ElementsHidden and hidden views hide a subtree, Accessible No
// only the view. Buttons, links, checkboxes and other controls, and
// Accessible Yes views, group their content as iOS and Android do: texts
// and images inside them name them and are no elements of their own. A
// canvas's named parts are its children.
//
// Each element has a control type, its TestID as the automation id, its
// Hint as help text, and the patterns its role and state call for:
// Invoke (presses), Toggle (Checked), ExpandCollapse, SelectionItem,
// RangeValue (a range Value) and Value (fields). Live regions raise
// LiveRegionChanged; Announce is a notification.

var (
	uiaCore                       = syscall.NewLazyDLL("uiautomationcore.dll")
	uiaReturnRawElementProvider   = uiaCore.NewProc("UiaReturnRawElementProvider")
	uiaHostProviderFromHwnd       = uiaCore.NewProc("UiaHostProviderFromHwnd")
	uiaRaiseAutomationEvent       = uiaCore.NewProc("UiaRaiseAutomationEvent")
	uiaRaiseStructureChangedEvent = uiaCore.NewProc("UiaRaiseStructureChangedEvent")
	uiaRaiseNotificationEvent     = uiaCore.NewProc("UiaRaiseNotificationEvent")
	uiaClientsAreListening        = uiaCore.NewProc("UiaClientsAreListening")
	uiaDisconnectProvider         = uiaCore.NewProc("UiaDisconnectProvider")
	safeArrayCreateVector         = oleaut32.NewProc("SafeArrayCreateVector")
	safeArrayAccessData           = oleaut32.NewProc("SafeArrayAccessData")
	safeArrayUnaccessData         = oleaut32.NewProc("SafeArrayUnaccessData")
	iidRawElementProviderSimple   = parseGUID("d6dd68d1-86fd-4332-8666-9abedea2d24c")
	iidRawElementProviderFragment = parseGUID("f7063da8-8359-439c-9297-bbc5299a7d87")
	iidRawElementProviderFragRoot = parseGUID("620ce2a5-ab8f-40a9-86cb-de3c75599b58")
	iidInvokeProvider             = parseGUID("54fcb24b-e18e-47a2-b4d3-eccbe77599a2")
	iidToggleProvider             = parseGUID("56d00bd0-c4f4-433c-a836-1a52a57e0892")
	iidRangeValueProvider         = parseGUID("36dc7aef-33e6-4691-afe1-2be7274b3d33")
	iidExpandCollapseProvider     = parseGUID("d847d3a5-cab0-4a98-8c32-ecb45c59ad24")
	iidSelectionItemProvider      = parseGUID("2acad808-b2d4-452d-a407-91ff1ad167b2")
	iidValueProvider              = parseGUID("c7935180-6fb3-4201-b174-7df73adbf64a")
)

const (
	wmGetObject       = 0x003D
	uiaRootObjectID   = -25
	uiaElementGone    = 0x80040201 // UIA_E_ELEMENTNOTAVAILABLE
	providerServer    = 0x1        // ProviderOptions_ServerSideProvider
	providerComThread = 0x20       // ProviderOptions_UseComThreading

	vtEmpty   = 0
	vtI4      = 3
	vtR8      = 5
	vtBSTR    = 8
	vtBool    = 11
	vtUnknown = 13

	patternInvoke         = 10000
	patternValue          = 10002
	patternRangeValue     = 10003
	patternExpandCollapse = 10005
	patternSelectionItem  = 10010
	patternToggle         = 10015

	propRuntimeID           = 30000
	propBoundingRectangle   = 30001
	propProcessID           = 30002
	propControlType         = 30003
	propLocalizedType       = 30004
	propName                = 30005
	propHasKeyboardFocus    = 30008
	propIsKeyboardFocusable = 30009
	propIsEnabled           = 30010
	propAutomationID        = 30011
	propClassName           = 30012
	propHelpText            = 30013
	propIsControlElement    = 30016
	propIsContentElement    = 30017
	propIsOffscreen         = 30022
	propIsPassword          = 30019
	propLiveSetting         = 30135
	propHeadingLevel        = 30173
	propIsDialog            = 30174

	ctButton      = 50000
	ctCheckBox    = 50002
	ctEdit        = 50004
	ctHyperlink   = 50005
	ctImage       = 50006
	ctMenu        = 50009
	ctMenuItem    = 50011
	ctProgressBar = 50012
	ctRadioButton = 50013
	ctSlider      = 50015
	ctSpinner     = 50016
	ctComboBox    = 50003
	ctToolBar     = 50021
	ctTab         = 50018
	ctTabItem     = 50019
	ctText        = 50020
	ctList        = 50008
	ctListItem    = 50007
	ctTree        = 50023
	ctTreeItem    = 50024
	ctGroup       = 50026
	ctDataGrid    = 50028
	ctDataItem    = 50029
	ctHeaderItem  = 50035
	ctTable       = 50036
	ctWindow      = 50032

	eventFocusChanged     = 20005
	eventLiveRegionChange = 20024
	structureInvalidated  = 2 // StructureChangeType_ChildrenInvalidated
	uiaAppendRuntimeID    = 3
	headingLevel1         = 80051
)

// variant is the start of an OLE VARIANT: its type, then its value.
type variant struct {
	vt  uint16
	_   [3]uint16
	val uint64
}

func (v *variant) setInt(i int32) { v.vt, v.val = vtI4, uint64(uint32(i)) }
func (v *variant) setBool(b bool) {
	v.vt, v.val = vtBool, 0
	if b {
		v.val = 0xffff // VARIANT_TRUE
	}
}
func (v *variant) setString(s string) {
	if s == "" {
		return
	}
	b, _, _ := sysAllocString.Call(uintptr(unsafe.Pointer(wide(s))))
	v.vt, v.val = vtBSTR, uint64(b)
}
func (v *variant) setDouble(d float64) {
	v.vt, v.val = vtR8, *(*uint64)(unsafe.Pointer(&d))
}

// uiaRect is UiaRect: the bounds on the screen, in pixels.
type uiaRect struct{ left, top, width, height float64 }

// uiaIface is one interface of an element: its vtable and the element.
type uiaIface struct {
	vtbl *[16]uintptr
	el   *uiaElement
}

// uiaElement is one provider: the window and the view, or a canvas's
// part of it (part 1 is the first), or the root (no view).
type uiaElement struct {
	simple, fragment, root, invoke, toggle, rangeValue, expand, selection, value uiaIface

	refs atomic.Int32
	w    *window
	id   driver.NodeID
	part int
}

// uiaKey is an element's key among the window's.
func uiaKey(id driver.NodeID, part int) uint64 { return uint64(id)<<16 | uint64(part) }

// element returns the provider of view id's part, made once.
func (w *window) element(id driver.NodeID, part int) *uiaElement {
	if w.uia == nil {
		w.uia = map[uint64]*uiaElement{}
	}
	k := uiaKey(id, part)
	if e := w.uia[k]; e != nil {
		return e
	}
	e := &uiaElement{w: w, id: id, part: part}
	for _, f := range []struct {
		i    *uiaIface
		vtbl *[16]uintptr
	}{{&e.simple, &uiaSimpleVtbl}, {&e.fragment, &uiaFragmentVtbl}, {&e.root, &uiaRootVtbl},
		{&e.invoke, &uiaInvokeVtbl}, {&e.toggle, &uiaToggleVtbl}, {&e.rangeValue, &uiaRangeVtbl},
		{&e.expand, &uiaExpandVtbl}, {&e.selection, &uiaSelectionVtbl}, {&e.value, &uiaValueVtbl}} {
		f.i.vtbl, f.i.el = f.vtbl, e
	}
	w.uia[k] = e
	return e
}

// getObject answers WM_GETOBJECT for UI Automation.
func (w *window) getObject(wParam, lParam uintptr) (uintptr, bool) {
	if int32(lParam) != uiaRootObjectID || uiaReturnRawElementProvider.Find() != nil {
		return 0, false
	}
	root := w.element(0, 0)
	r, _, _ := uiaReturnRawElementProvider.Call(w.hwnd, wParam, lParam, uintptr(unsafe.Pointer(&root.simple)))
	return r, true
}

// alive returns the element's view, nil for the root, and whether the
// element still stands for something.
func (e *uiaElement) alive() (*view, bool) {
	if e.id == 0 {
		return nil, e.w.hwnd != 0
	}
	v := e.w.view(e.id)
	if v == nil || v.deleted {
		return nil, false
	}
	if e.part > len(v.canvasNodes()) {
		return nil, false
	}
	return v, true
}

// IUnknown, shared by every interface: QueryInterface answers from the
// element's interfaces, with IUnknown its simple provider.

func uiaQueryInterface(i *uiaIface, iid *guid, out *unsafe.Pointer) uintptr {
	e := i.el
	var to *uiaIface
	switch *iid {
	case iidUnknown, iidRawElementProviderSimple:
		to = &e.simple
	case iidRawElementProviderFragment:
		to = &e.fragment
	case iidRawElementProviderFragRoot:
		if e.id == 0 {
			to = &e.root
		}
	default:
		if p := e.pattern(*iid); p != nil {
			to = p
		}
	}
	if to == nil {
		*out = nil
		return eNoInterface
	}
	*out = unsafe.Pointer(to)
	e.refs.Add(1)
	return sOK
}

func uiaAddRef(i *uiaIface) uintptr { return uintptr(i.el.refs.Add(1)) }

// uiaRelease counts down; the window keeps its elements while they stand
// for a view.
func uiaRelease(i *uiaIface) uintptr { return uintptr(i.el.refs.Add(-1)) }

// pattern returns the interface of pattern iid when the element has it.
func (e *uiaElement) pattern(iid guid) *uiaIface {
	v, ok := e.alive()
	if !ok || v == nil {
		return nil
	}
	s := e.spec(v)
	switch {
	case iid == iidInvokeProvider && s.invoke:
		return &e.invoke
	case iid == iidToggleProvider && s.toggle >= 0:
		return &e.toggle
	case iid == iidRangeValueProvider && s.ranged:
		return &e.rangeValue
	case iid == iidExpandCollapseProvider && s.expandable:
		return &e.expand
	case iid == iidSelectionItemProvider && s.selectable:
		return &e.selection
	case iid == iidValueProvider && v.field != nil && e.part == 0:
		return &e.value
	}
	return nil
}

func patternIID(id uintptr) (guid, bool) {
	switch id {
	case patternInvoke:
		return iidInvokeProvider, true
	case patternToggle:
		return iidToggleProvider, true
	case patternRangeValue:
		return iidRangeValueProvider, true
	case patternExpandCollapse:
		return iidExpandCollapseProvider, true
	case patternSelectionItem:
		return iidSelectionItemProvider, true
	case patternValue:
		return iidValueProvider, true
	}
	return guid{}, false
}

// uiaSpec is what an element exposes.
type uiaSpec struct {
	control                        int32
	name, help, automationID       string
	enabled, focusable, focused    bool
	invoke, ranged, expandable     bool
	selectable, selected, expanded bool
	toggle                         int32 // ToggleState, -1 for none
	min, max, now                  float32
	live                           driver.LiveRegion
	heading, dialog, password      bool
}

// uiaControls are the control types of the roles.
var uiaControls = [...]int32{
	driver.RoleButton: ctButton, driver.RoleLink: ctHyperlink, driver.RoleHeader: ctText, driver.RoleImage: ctImage,
	driver.RoleCheckbox: ctCheckBox, driver.RoleSwitch: ctButton, driver.RoleRadio: ctRadioButton, driver.RoleTab: ctTabItem,
	driver.RoleText: ctText, driver.RoleTextField: ctEdit, driver.RoleRadioGroup: ctGroup, driver.RoleTabList: ctTab,
	driver.RoleMenu: ctMenu, driver.RoleMenuItem: ctMenuItem, driver.RoleDialog: ctWindow, driver.RoleAlert: ctText,
	driver.RoleAdjustable: ctSlider, driver.RoleSpinButton: ctSpinner, driver.RoleCombobox: ctComboBox, driver.RoleProgressBar: ctProgressBar, driver.RoleTable: ctTable,
	driver.RoleGrid: ctDataGrid, driver.RoleRowGroup: ctGroup, driver.RoleRow: ctDataItem,
	driver.RoleColumnHeader: ctHeaderItem, driver.RoleCell: ctText, driver.RoleListbox: ctList,
	driver.RoleOption: ctListItem, driver.RoleGroup: ctGroup, driver.RoleRegion: ctGroup,
	driver.RoleNavigation: ctGroup, driver.RoleTree: ctTree, driver.RoleTreeItem: ctTreeItem,
	driver.RoleToolbar: ctToolBar, driver.RoleMenuItemCheckbox: ctMenuItem, driver.RoleMenuItemRadio: ctMenuItem,
}

// groups reports whether role groups the texts and images inside it.
func groups(p *driver.ViewProps, pressable bool) bool {
	switch p.Role {
	case driver.RoleButton, driver.RoleLink, driver.RoleCheckbox, driver.RoleSwitch, driver.RoleRadio, driver.RoleTab,
		driver.RoleMenuItem, driver.RoleHeader, driver.RoleCell, driver.RoleColumnHeader, driver.RoleOption,
		driver.RoleTreeItem, driver.RoleMenuItemCheckbox, driver.RoleMenuItemRadio:
		return true
	}
	return p.Accessible == driver.AccessibleYes || p.Role == driver.RoleNone && pressable
}

func takesPresses(v *view) bool {
	return v.events.Has(driver.EventPress) || v.events.Has(driver.EventLongPress)
}

// isElement reports whether v is an element of its own.
func isElement(v *view) bool {
	p := &v.props
	switch {
	case p.Accessible == driver.AccessibleNo:
		return false
	case p.Role != driver.RoleNone, p.Label != "", v.field != nil, takesPresses(v):
		return true
	case v.kind == driver.KindText:
		return v.plain != ""
	case v.kind == driver.KindCanvas:
		return len(v.canvasNodes()) > 0
	}
	return false
}

// hidesSubtree reports whether v and what is inside are no elements.
func hidesSubtree(v *view) bool {
	return v.props.ElementsHidden || v.style.Hidden || v.deleted
}

// spec returns what element e of view v exposes.
func (e *uiaElement) spec(v *view) uiaSpec {
	if e.part > 0 {
		n := v.canvasNodes()[e.part-1]
		c := int32(ctGroup)
		if int(n.Role) < len(uiaControls) && uiaControls[n.Role] != 0 {
			c = uiaControls[n.Role]
		}
		return uiaSpec{control: c, name: n.Label, automationID: n.TestID, enabled: true, toggle: -1,
			invoke: n.Role == driver.RoleButton || n.Role == driver.RoleLink, selectable: n.Selected, selected: n.Selected}
	}
	p := &v.props
	s := uiaSpec{name: p.Label, help: p.Hint, automationID: p.TestID, enabled: !p.Disabled, toggle: -1, live: p.LiveRegion}
	pressable := takesPresses(v)
	switch {
	case int(p.Role) < len(uiaControls) && uiaControls[p.Role] != 0:
		s.control = uiaControls[p.Role]
	case v.field != nil:
		s.control = ctEdit
	case v.kind == driver.KindText:
		s.control = ctText
	case pressable:
		s.control = ctButton
	case v.kind == driver.KindImage:
		s.control = ctImage
	case v.kind == driver.KindCanvas:
		s.control = ctImage
	default:
		s.control = ctGroup
	}
	if s.name == "" {
		if v.kind == driver.KindText {
			s.name = v.plain
		} else if groups(p, pressable) {
			s.name = groupedText(v)
		}
	}
	s.heading, s.dialog = p.Role == driver.RoleHeader, p.Role == driver.RoleDialog
	if p.Role == driver.RoleAlert && s.live == driver.LiveRegionNone {
		s.live = driver.LiveRegionAssertive
	}
	s.invoke = pressable || p.HasAction(driver.ActionActivate)
	switch {
	case p.Role == driver.RoleCheckbox || p.Role == driver.RoleSwitch || p.Role == driver.RoleMenuItemCheckbox ||
		p.Role == driver.RoleMenuItemRadio || p.Checked || p.Indeterminate || p.Pressed:
		s.toggle = 0
		if p.Checked || p.Pressed {
			s.toggle = 1
		}
		if p.Indeterminate {
			s.toggle = 2
		}
		s.invoke = s.invoke && p.Role != driver.RoleCheckbox && p.Role != driver.RoleSwitch
	}
	if p.Value.IsRange() {
		s.ranged, s.min, s.max, s.now = true, p.Value.Min, p.Value.Max, p.Value.Now
	}
	s.expandable, s.expanded = p.Expandable, p.Expanded
	s.selectable = p.Role == driver.RoleTab || p.Role == driver.RoleRadio || p.Role == driver.RoleOption || p.Selected
	s.selected = p.Selected || p.Role == driver.RoleRadio && p.Checked
	if in := v.field; in != nil {
		s.focusable, s.focused, s.password = true, in.focused, in.masked()
		if s.name == "" {
			s.name = in.config.Placeholder
		}
	}
	return s
}

// groupedText joins the texts inside a view that groups them.
func groupedText(v *view) string {
	var parts []string
	var walk func(c *view)
	walk = func(c *view) {
		for _, k := range c.children {
			if hidesSubtree(k) {
				continue
			}
			if k.kind == driver.KindText && k.plain != "" {
				parts = append(parts, k.plain)
			} else if k.props.Label != "" && !takesPresses(k) {
				parts = append(parts, k.props.Label)
			} else if !takesPresses(k) && k.props.Role == driver.RoleNone {
				walk(k)
			}
		}
	}
	walk(v)
	return strings.Join(parts, " ")
}

// children returns the elements inside element e: its view's, flattened,
// and a canvas's parts.
func (e *uiaElement) children() []*uiaElement {
	v, ok := e.alive()
	if !ok || e.part > 0 {
		return nil
	}
	w := e.w
	var out []*uiaElement
	if v == nil {
		root := w.view(w.root)
		if root == nil {
			return nil
		}
		v = root
		if isElement(root) && !hidesSubtree(root) {
			return []*uiaElement{w.element(root.id, 0)}
		}
	}
	for i := range v.canvasNodes() {
		out = append(out, w.element(v.id, i+1))
	}
	grouping := e.id != 0 && groups(&v.props, takesPresses(v))
	var walk func(c *view)
	walk = func(c *view) {
		for _, k := range c.children {
			if hidesSubtree(k) {
				continue
			}
			plain := k.kind == driver.KindText || k.kind == driver.KindImage
			if isElement(k) && !(grouping && plain && k.props.Role == driver.RoleNone && !takesPresses(k)) {
				out = append(out, w.element(k.id, 0))
				continue
			}
			walk(k)
		}
	}
	walk(v)
	return out
}

// parent returns the element around e.
func (e *uiaElement) parent() *uiaElement {
	v, ok := e.alive()
	if !ok || v == nil {
		return nil
	}
	if e.part > 0 {
		return e.w.element(v.id, 0)
	}
	for p := v.parent; p != nil; p = p.parent {
		if isElement(p) && !hidesSubtree(p) {
			return e.w.element(p.id, 0)
		}
	}
	return e.w.element(0, 0)
}

// screenBounds returns e's bounds on the screen, in pixels.
func (e *uiaElement) screenBounds() uiaRect {
	w := e.w
	var b driver.Bounds
	if v, ok := e.alive(); ok && v != nil {
		b, _ = w.surfaceBounds(v)
		if e.part > 0 {
			n := v.canvasNodes()[e.part-1].Bounds
			b = driver.Bounds{Origin: driver.Point{X: b.Origin.X + n.Origin.X, Y: b.Origin.Y + n.Origin.Y}, Size: n.Size}
		}
	} else {
		var r rect
		getClientRect.Call(w.hwnd, uintptr(unsafe.Pointer(&r)))
		size := w.point(r.Right-r.Left, r.Bottom-r.Top)
		b.Size = driver.Size{Width: size.X, Height: size.Y}
	}
	origin := point{}
	clientToScreen.Call(w.hwnd, uintptr(unsafe.Pointer(&origin)))
	s := w.scale
	return uiaRect{float64(origin.X) + float64(b.Origin.X)*s, float64(origin.Y) + float64(b.Origin.Y)*s,
		float64(b.Size.Width) * s, float64(b.Size.Height) * s}
}

// IRawElementProviderSimple.

func uiaProviderOptions(i *uiaIface, out *int32) uintptr {
	*out = providerServer | providerComThread
	return sOK
}

func uiaGetPatternProvider(i *uiaIface, id uintptr, out *unsafe.Pointer) uintptr {
	*out = nil
	if iid, ok := patternIID(id); ok {
		if p := i.el.pattern(iid); p != nil {
			p.el.refs.Add(1)
			*out = unsafe.Pointer(p)
		}
	}
	return sOK
}

func uiaGetPropertyValue(i *uiaIface, id uintptr, out *variant) uintptr {
	*out = variant{}
	e := i.el
	v, ok := e.alive()
	if !ok {
		return uiaElementGone
	}
	if v == nil {
		return sOK // the root's are the host provider's: the window's
	}
	s := e.spec(v)
	switch id {
	case propControlType:
		out.setInt(s.control)
	case propName:
		out.setString(s.name)
	case propHelpText:
		out.setString(s.help)
	case propAutomationID:
		out.setString(s.automationID)
	case propClassName:
		out.setString("NsgoView")
	case propIsEnabled:
		out.setBool(s.enabled)
	case propIsKeyboardFocusable:
		out.setBool(s.focusable)
	case propHasKeyboardFocus:
		out.setBool(s.focused)
	case propIsControlElement, propIsContentElement:
		out.setBool(true)
	case propIsPassword:
		out.setBool(s.password)
	case propLiveSetting:
		out.setInt(int32(s.live))
	case propHeadingLevel:
		if s.heading {
			out.setInt(headingLevel1)
		}
	case propIsDialog:
		out.setBool(s.dialog)
	case propIsOffscreen:
		b := e.screenBounds()
		out.setBool(b.width <= 0 || b.height <= 0)
	}
	return sOK
}

func uiaHostProvider(i *uiaIface, out *uintptr) uintptr {
	*out = 0
	if i.el.id != 0 {
		return sOK
	}
	r, _, _ := uiaHostProviderFromHwnd.Call(i.el.w.hwnd, uintptr(unsafe.Pointer(out)))
	return r
}

// IRawElementProviderFragment.

const (
	navParent = iota
	navNext
	navPrevious
	navFirst
	navLast
)

func uiaNavigate(i *uiaIface, dir uintptr, out *unsafe.Pointer) uintptr {
	if out == nil {
		return 0x80070057 // E_INVALIDARG
	}
	*out = nil
	e := i.el
	if _, ok := e.alive(); !ok {
		return uiaElementGone
	}
	var to *uiaElement
	switch dir {
	case navParent:
		to = e.parent()
	case navFirst, navLast:
		if kids := e.children(); len(kids) > 0 {
			to = kids[0]
			if dir == navLast {
				to = kids[len(kids)-1]
			}
		}
	case navNext, navPrevious:
		if p := e.parent(); p != nil {
			kids := p.children()
			for k, c := range kids {
				if c != e {
					continue
				}
				if dir == navNext && k+1 < len(kids) {
					to = kids[k+1]
				} else if dir == navPrevious && k > 0 {
					to = kids[k-1]
				}
			}
		}
	}
	if to != nil {
		to.refs.Add(1)
		*out = unsafe.Pointer(&to.fragment)
	}
	return sOK
}

func uiaRuntimeID(i *uiaIface, out *uintptr) uintptr {
	*out = 0
	e := i.el
	if e.id == 0 {
		return sOK // the host provider's
	}
	ids := []int32{uiaAppendRuntimeID, int32(e.id), int32(e.part)}
	arr, _, _ := safeArrayCreateVector.Call(vtI4, 0, uintptr(len(ids)))
	if arr == 0 {
		return 0x8007000E // E_OUTOFMEMORY
	}
	var data *[3]int32
	safeArrayAccessData.Call(arr, uintptr(unsafe.Pointer(&data)))
	copy(data[:], ids)
	safeArrayUnaccessData.Call(arr)
	*out = arr
	return sOK
}

func uiaBoundingRectangle(i *uiaIface, out *uiaRect) uintptr {
	*out = uiaRect{}
	if v, ok := i.el.alive(); !ok {
		return uiaElementGone
	} else if v == nil {
		return sOK // the host provider's
	}
	*out = i.el.screenBounds()
	return sOK
}

func uiaEmbeddedRoots(i *uiaIface, out *uintptr) uintptr {
	*out = 0
	return sOK
}

func uiaSetFocus(i *uiaIface) uintptr {
	v, ok := i.el.alive()
	if !ok {
		return uiaElementGone
	}
	if v != nil && v.field != nil {
		i.el.w.setFocus(v.field)
	}
	return sOK
}

func uiaFragmentRoot(i *uiaIface, out *unsafe.Pointer) uintptr {
	root := i.el.w.element(0, 0)
	root.refs.Add(1)
	*out = unsafe.Pointer(&root.root)
	return sOK
}

// IRawElementProviderFragmentRoot.

// fromPoint returns the innermost element at a screen pixel.
func (w *window) fromPoint(x, y float64) *uiaElement {
	origin := point{}
	clientToScreen.Call(w.hwnd, uintptr(unsafe.Pointer(&origin)))
	p := w.point(int32(x)-origin.X, int32(y)-origin.Y)
	root := w.view(w.root)
	if root == nil {
		return nil
	}
	for v := pick(root, p); v != nil; v = v.parent {
		if isElement(v) && !hidesSubtree(v) {
			el := w.element(v.id, 0)
			// Its own parts, then itself.
			b, _ := w.surfaceBounds(v)
			for k, n := range v.canvasNodes() {
				nb := n.Bounds
				nb.Origin.X += b.Origin.X
				nb.Origin.Y += b.Origin.Y
				if nb.Contains(p) {
					return w.element(v.id, k+1)
				}
			}
			for p := el.parent(); p != nil && p.id != 0; p = p.parent() {
				// A grouped text names its group, which is the element.
				if pv, ok := p.alive(); ok && pv != nil && groups(&pv.props, takesPresses(pv)) && (v.kind == driver.KindText || v.kind == driver.KindImage) {
					el = p
				}
				break
			}
			return el
		}
	}
	return nil
}

func uiaElementFromPoint(i *uiaIface, x, y float64, out *unsafe.Pointer) uintptr {
	*out = nil
	if e := i.el.w.fromPoint(x, y); e != nil {
		e.refs.Add(1)
		*out = unsafe.Pointer(&e.fragment)
	}
	return sOK
}

func uiaGetFocus(i *uiaIface, out *unsafe.Pointer) uintptr {
	*out = nil
	w := i.el.w
	if in := w.focusedField(); in != nil && in.v != nil {
		e := w.element(in.v.id, 0)
		e.refs.Add(1)
		*out = unsafe.Pointer(&e.fragment)
	}
	return sOK
}

// Patterns.

// activate presses the element as a click would: the activate action, or
// a press.
func (e *uiaElement) activate() uintptr {
	v, ok := e.alive()
	if !ok || v == nil {
		return uiaElementGone
	}
	w := e.w
	if e.part > 0 {
		b, _ := w.surfaceBounds(v)
		n := v.canvasNodes()[e.part-1].Bounds
		w.engine.PressAt(driver.Event{Page: driver.Point{X: b.Origin.X + n.Origin.X + n.Size.Width/2, Y: b.Origin.Y + n.Origin.Y + n.Size.Height/2}})
		return sOK
	}
	switch {
	case v.props.HasAction(driver.ActionActivate) && v.events.Has(driver.EventAccessibilityAction):
		w.engine.Dispatch(driver.Event{Kind: driver.EventAccessibilityAction, ID: v.id, Text: driver.ActionActivate})
	case v.events.Has(driver.EventPress):
		w.engine.Dispatch(driver.Event{Kind: driver.EventPress, ID: v.id, Index: -1})
	}
	return sOK
}

func uiaInvoke(i *uiaIface) uintptr { return i.el.activate() }

func uiaToggleState(i *uiaIface, out *int32) uintptr {
	v, ok := i.el.alive()
	if !ok || v == nil {
		return uiaElementGone
	}
	*out = max(i.el.spec(v).toggle, 0)
	return sOK
}

func (e *uiaElement) number(read func(s uiaSpec) float32, out *float64) uintptr {
	v, ok := e.alive()
	if !ok || v == nil {
		return uiaElementGone
	}
	*out = float64(read(e.spec(v)))
	return sOK
}

func uiaRangeValue(i *uiaIface, out *float64) uintptr {
	return i.el.number(func(s uiaSpec) float32 { return s.now }, out)
}
func uiaRangeMax(i *uiaIface, out *float64) uintptr {
	return i.el.number(func(s uiaSpec) float32 { return s.max }, out)
}
func uiaRangeMin(i *uiaIface, out *float64) uintptr {
	return i.el.number(func(s uiaSpec) float32 { return s.min }, out)
}
func uiaRangeStep(i *uiaIface, out *float64) uintptr {
	return i.el.number(func(s uiaSpec) float32 { return (s.max - s.min) / 10 }, out)
}

// uiaRangeSet moves a range toward value by the increment and decrement
// actions, the only way the app adjusts one.
func (e *uiaElement) setRange(value float64) uintptr {
	v, ok := e.alive()
	if !ok || v == nil {
		return uiaElementGone
	}
	s := e.spec(v)
	action := driver.ActionIncrement
	if value < float64(s.now) {
		action = driver.ActionDecrement
	} else if value == float64(s.now) {
		return sOK
	}
	if v.props.HasAction(action) && v.events.Has(driver.EventAccessibilityAction) {
		e.w.engine.Dispatch(driver.Event{Kind: driver.EventAccessibilityAction, ID: v.id, Text: action})
	}
	return sOK
}

func uiaReadOnly(i *uiaIface, out *int32) uintptr {
	v, ok := i.el.alive()
	if !ok || v == nil {
		return uiaElementGone
	}
	*out = 0
	if v.field != nil && v.field.config.Readonly || v.field == nil && !v.props.HasAction(driver.ActionIncrement) {
		*out = 1
	}
	return sOK
}

func uiaExpand(i *uiaIface) uintptr   { return i.el.expandAction(driver.ActionExpand) }
func uiaCollapse(i *uiaIface) uintptr { return i.el.expandAction(driver.ActionCollapse) }

func (e *uiaElement) expandAction(name string) uintptr {
	v, ok := e.alive()
	if !ok || v == nil {
		return uiaElementGone
	}
	if v.props.HasAction(name) && v.events.Has(driver.EventAccessibilityAction) {
		e.w.engine.Dispatch(driver.Event{Kind: driver.EventAccessibilityAction, ID: v.id, Text: name})
		return sOK
	}
	return e.activate()
}

func uiaExpandState(i *uiaIface, out *int32) uintptr {
	v, ok := i.el.alive()
	if !ok || v == nil {
		return uiaElementGone
	}
	*out = 0
	if i.el.spec(v).expanded {
		*out = 1
	}
	return sOK
}

func uiaSelect(i *uiaIface) uintptr { return i.el.activate() }

func uiaIsSelected(i *uiaIface, out *int32) uintptr {
	v, ok := i.el.alive()
	if !ok || v == nil {
		return uiaElementGone
	}
	*out = 0
	if i.el.spec(v).selected {
		*out = 1
	}
	return sOK
}

func uiaSelectionContainer(i *uiaIface, out *unsafe.Pointer) uintptr {
	*out = nil
	if p := i.el.parent(); p != nil {
		p.refs.Add(1)
		*out = unsafe.Pointer(&p.simple)
	}
	return sOK
}

func uiaFieldValue(i *uiaIface, out *uintptr) uintptr {
	*out = 0
	v, ok := i.el.alive()
	if !ok || v == nil || v.field == nil {
		return uiaElementGone
	}
	if v.field.masked() {
		return sOK // assistive technology must not read it
	}
	b, _, _ := sysAllocString.Call(uintptr(unsafe.Pointer(wide(v.field.model.text))))
	*out = b
	return sOK
}

func uiaSetFieldValue(i *uiaIface, text *uint16) uintptr {
	v, ok := i.el.alive()
	if !ok || v == nil || v.field == nil {
		return uiaElementGone
	}
	in := v.field
	in.model.selectAll()
	in.insert(syscall.UTF16ToString(unsafe.Slice(text, wcslen(text))))
	return sOK
}

func wcslen(p *uint16) int {
	n := 0
	for ; *(*uint16)(unsafe.Add(unsafe.Pointer(p), 2*n)) != 0; n++ {
	}
	return n
}

func uiaNotImplemented(i *uiaIface) uintptr { return 0x80004001 }

// Events.

// listening reports whether a UI Automation client listens.
func uiaListening() bool {
	if uiaClientsAreListening.Find() != nil {
		return false
	}
	r, _, _ := uiaClientsAreListening.Call()
	return r != 0
}

// accessChanged follows a change of what v exposes: a changed label or
// text inside a live region raises LiveRegionChanged.
func (w *window) accessChanged(v *view, text bool) {
	if !uiaListening() || w.uia == nil {
		return
	}
	if text {
		for p := v; p != nil; p = p.parent {
			if p.props.LiveRegion != driver.LiveRegionNone || p.props.Role == driver.RoleAlert {
				e := w.element(p.id, 0)
				uiaRaiseAutomationEvent.Call(uintptr(unsafe.Pointer(&e.simple)), eventLiveRegionChange)
				return
			}
		}
	}
}

// structureChanged tells clients the tree changed under the root.
func (w *window) structureChanged() {
	if !uiaListening() || w.uia == nil {
		return
	}
	root := w.element(0, 0)
	// The runtime ID of the root is the host's: none of its own.
	id := []int32{uiaAppendRuntimeID, 0}
	uiaRaiseStructureChangedEvent.Call(uintptr(unsafe.Pointer(&root.simple)), structureInvalidated,
		uintptr(unsafe.Pointer(&id[0])), uintptr(len(id)))
}

// focusChanged tells clients field v took the keyboard focus.
func (w *window) uiaFocusChanged(v *view) {
	if v == nil || !uiaListening() {
		return
	}
	e := w.element(v.id, 0)
	uiaRaiseAutomationEvent.Call(uintptr(unsafe.Pointer(&e.simple)), eventFocusChanged)
}

// uiaAnnounce has assistive technology read text, as a notification.
func (w *window) uiaAnnounce(text string) {
	if text == "" || uiaRaiseNotificationEvent.Find() != nil || !uiaListening() {
		return
	}
	root := w.element(0, 0)
	msg, _, _ := sysAllocString.Call(uintptr(unsafe.Pointer(wide(text))))
	activity, _, _ := sysAllocString.Call(uintptr(unsafe.Pointer(wide("nsgo.announce"))))
	const kindOther, processingAll = 4, 2
	uiaRaiseNotificationEvent.Call(uintptr(unsafe.Pointer(&root.simple)), kindOther, processingAll, msg, activity)
	sysFreeString.Call(msg)
	sysFreeString.Call(activity)
}

// canvasNodes returns a canvas's named parts.
func (v *view) canvasNodes() []driver.CanvasNode {
	if v.drawing == nil {
		return nil
	}
	return v.drawing.Nodes
}

// forgetElements disconnects the providers of a deleted view.
func (w *window) forgetElements(v *view) {
	if w.uia == nil {
		return
	}
	for part := 0; part <= len(v.canvasNodes()); part++ {
		k := uiaKey(v.id, part)
		if e := w.uia[k]; e != nil {
			if uiaDisconnectProvider.Find() == nil {
				uiaDisconnectProvider.Call(uintptr(unsafe.Pointer(&e.simple)))
			}
			delete(w.uia, k)
		}
	}
}
