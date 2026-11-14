//go:build linux && !android

package linux

import (
	"bytes"
	"fmt"
	"image/png"
	"os"
	"strconv"
	"strings"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// Linux tray icons use the freedesktop StatusNotifierItem protocol. The
// watcher owns the visual presentation; the app owns one D-Bus item per tray
// ID and receives Activate, SecondaryActivate and ContextMenu calls.

const (
	trayServicePrefix = "org.freedesktop.StatusNotifierItem-"
	trayItemInterface = "org.freedesktop.StatusNotifierItem"
	trayMenuInterface = "com.canonical.dbusmenu"
	trayProperties    = "org.freedesktop.DBus.Properties"
	trayWatcher       = "org.kde.StatusNotifierWatcher"
	trayWatcherAlt    = "org.freedesktop.StatusNotifierWatcher"
	trayWatcherPath   = "/StatusNotifierWatcher"
	trayWatcherIface  = "org.kde.StatusNotifierWatcher"
)

const trayIntrospection = `
<node>
  <interface name="org.freedesktop.StatusNotifierItem">
    <method name="Activate"><arg name="x" type="i" direction="in"/><arg name="y" type="i" direction="in"/></method>
    <method name="SecondaryActivate"><arg name="x" type="i" direction="in"/><arg name="y" type="i" direction="in"/></method>
    <method name="ContextMenu"><arg name="x" type="i" direction="in"/><arg name="y" type="i" direction="in"/></method>
    <method name="Scroll"><arg name="delta" type="i" direction="in"/><arg name="orientation" type="s" direction="in"/></method>
    <signal name="NewTitle"/><signal name="NewIcon"/><signal name="NewAttentionIcon"/><signal name="NewOverlayIcon"/><signal name="NewToolTip"/><signal name="NewStatus"><arg name="status" type="s"/></signal>
    <property name="Category" type="s" access="read"/>
    <property name="Id" type="s" access="read"/>
    <property name="Title" type="s" access="read"/>
    <property name="Status" type="s" access="read"/>
    <property name="WindowId" type="i" access="read"/>
    <property name="IconName" type="s" access="read"/>
    <property name="IconPixmap" type="a(iiay)" access="read"/>
    <property name="ToolTip" type="(sa(iiay)ss)" access="read"/>
    <property name="Menu" type="o" access="read"/>
  </interface>
  <interface name="com.canonical.dbusmenu">
    <method name="GetLayout"><arg name="parentId" type="i" direction="in"/><arg name="recursionDepth" type="i" direction="in"/><arg name="propertyNames" type="as" direction="in"/><arg name="revision" type="u" direction="out"/><arg name="layout" type="(ia{sv}av)" direction="out"/></method>
    <method name="GetGroupProperties"><arg name="ids" type="ai" direction="in"/><arg name="propertyNames" type="as" direction="in"/><arg name="properties" type="a(ia{sv})" direction="out"/></method>
    <method name="GetProperty"><arg name="id" type="i" direction="in"/><arg name="name" type="s" direction="in"/><arg name="value" type="v" direction="out"/></method>
    <method name="Event"><arg name="id" type="i" direction="in"/><arg name="eventId" type="s" direction="in"/><arg name="data" type="v" direction="in"/><arg name="timestamp" type="u" direction="in"/></method>
    <method name="AboutToShow"><arg name="id" type="i" direction="in"/><arg name="needUpdate" type="b" direction="out"/></method>
    <method name="AboutToShowGroup"><arg name="ids" type="ai" direction="in"/><arg name="updatesNeeded" type="ai" direction="out"/><arg name="idErrors" type="ai" direction="out"/></method>
    <signal name="ItemsPropertiesUpdated"><arg name="updatedProps" type="a(ia{sv})"/><arg name="removedProps" type="a(ias)"/></signal>
    <signal name="LayoutUpdated"><arg name="revision" type="u"/><arg name="parentId" type="i"/></signal>
    <property name="Version" type="u" access="read"/>
    <property name="TextDirection" type="s" access="read"/>
    <property name="Status" type="s" access="read"/>
    <property name="IconThemePath" type="s" access="read"/>
  </interface>
  <interface name="org.freedesktop.DBus.Properties">
    <method name="Get"><arg name="interface" type="s" direction="in"/><arg name="property" type="s" direction="in"/><arg name="value" type="v" direction="out"/></method>
    <method name="GetAll"><arg name="interface" type="s" direction="in"/><arg name="properties" type="a{sv}" direction="out"/></method>
    <signal name="PropertiesChanged"><arg name="interface" type="s"/><arg name="changed_properties" type="a{sv}"/><arg name="invalidated_properties" type="as"/></signal>
  </interface>
</node>`

type gDBusInterfaceVTable struct {
	MethodCall  uintptr
	GetProperty uintptr
	SetProperty uintptr
	Padding     [8]uintptr
}

type linuxTray struct {
	cfg      driver.TrayConfig
	bus      uintptr
	name     string
	owner    uint32
	itemID   uint32
	propsID  uint32
	menuID   uint32
	menuPath string
	menu     []driver.MenuEntry
	revision uint32
	popupID  uint32
}

var (
	gDBusNodeInfoNewForXML           func(xml *byte, err *uintptr) uintptr
	gDBusNodeInfoLookupInterface     func(info uintptr, name *byte) uintptr
	gDBusConnectionRegisterObject    func(conn uintptr, path *byte, info uintptr, vtable *gDBusInterfaceVTable, data, destroy uintptr, err *uintptr) uint32
	gDBusConnectionUnregisterObject  func(conn uintptr, registration uint32) int32
	gBusOwnNameOnConnection          func(conn uintptr, name *byte, flags uint32, acquired, lost, data, destroy uintptr) uint32
	gBusUnownName                    func(owner uint32)
	gDBusMethodInvocationReturnValue func(invocation, value uintptr)
	gVariantNewFixedArray            func(elementType uintptr, data unsafe.Pointer, n, elementSize uintptr) uintptr
	gVariantNewObjectPath            func(*byte) uintptr
	trayBind                         = sync.OnceFunc(func() {
		bindInputSources()
		bindNotifications()
		purego.RegisterLibFunc(&gDBusNodeInfoNewForXML, gioLib, "g_dbus_node_info_new_for_xml")
		purego.RegisterLibFunc(&gDBusNodeInfoLookupInterface, gioLib, "g_dbus_node_info_lookup_interface")
		purego.RegisterLibFunc(&gDBusConnectionRegisterObject, gioLib, "g_dbus_connection_register_object")
		purego.RegisterLibFunc(&gDBusConnectionUnregisterObject, gioLib, "g_dbus_connection_unregister_object")
		purego.RegisterLibFunc(&gBusOwnNameOnConnection, gioLib, "g_bus_own_name_on_connection")
		purego.RegisterLibFunc(&gBusUnownName, gioLib, "g_bus_unown_name")
		purego.RegisterLibFunc(&gDBusMethodInvocationReturnValue, gioLib, "g_dbus_method_invocation_return_value")
		purego.RegisterLibFunc(&gVariantNewFixedArray, glibLib, "g_variant_new_fixed_array")
		purego.RegisterLibFunc(&gVariantNewObjectPath, glibLib, "g_variant_new_object_path")
	})
	trayNodeInfo      uintptr
	trayVTable        = gDBusInterfaceVTable{}
	cbTrayMethod      uintptr
	cbTrayGetProperty uintptr
	cbTraySetProperty uintptr
	cbTrayAcquired    uintptr
)

func trayObjectPath(id uint32) string {
	_ = id
	return "/StatusNotifierItem"
}

func trayMenuPath(id uint32) string {
	_ = id
	return "/Menu"
}

func init() {
	cbTrayMethod = newCallback(trayMethodCall)
	cbTrayGetProperty = newCallback(trayGetProperty)
	cbTraySetProperty = newCallback(traySetProperty)
	cbTrayAcquired = newCallback(trayNameAcquired)
	trayVTable = gDBusInterfaceVTable{MethodCall: cbTrayMethod, GetProperty: cbTrayGetProperty, SetProperty: cbTraySetProperty}
}

func (w *window) applyTray(cfg driver.TrayConfig) {
	trayBind()
	tr := w.trays[cfg.ID]
	if cfg.Removed {
		if tr != nil {
			w.removeTray(tr)
		}
		return
	}
	if tr != nil {
		tr.cfg = cfg
		return
	}
	bus := sessionBus()
	if bus == 0 {
		return
	}
	if trayNodeInfo == 0 {
		var err uintptr
		trayNodeInfo = gDBusNodeInfoNewForXML(cstr(trayIntrospection), &err)
		if err != 0 {
			gErrorFree(err)
			trayNodeInfo = 0
		}
	}
	if trayNodeInfo == 0 {
		return
	}
	itemInfo := gDBusNodeInfoLookupInterface(trayNodeInfo, cstr(trayItemInterface))
	propsInfo := gDBusNodeInfoLookupInterface(trayNodeInfo, cstr(trayProperties))
	menuInfo := gDBusNodeInfoLookupInterface(trayNodeInfo, cstr(trayMenuInterface))
	if itemInfo == 0 || propsInfo == 0 || menuInfo == 0 {
		return
	}
	tr = &linuxTray{cfg: cfg, bus: bus, name: fmt.Sprintf("%s%d-%d", trayServicePrefix, os.Getpid(), cfg.ID), menuPath: trayMenuPath(cfg.ID)}
	tr.owner = gBusOwnNameOnConnection(bus, cstr(tr.name), 0, cbTrayAcquired, 0, w.handle, 0)
	var err uintptr
	path := trayObjectPath(cfg.ID)
	tr.itemID = gDBusConnectionRegisterObject(bus, cstr(path), itemInfo, &trayVTable, w.handle, 0, &err)
	if err != 0 {
		gErrorFree(err)
	}
	if tr.itemID == 0 {
		if tr.owner != 0 {
			gBusUnownName(tr.owner)
		}
		return
	}
	tr.propsID = gDBusConnectionRegisterObject(bus, cstr(path), propsInfo, &trayVTable, w.handle, 0, &err)
	if err != 0 {
		gErrorFree(err)
	}
	tr.menuID = gDBusConnectionRegisterObject(bus, cstr(tr.menuPath), menuInfo, &trayVTable, w.handle, 0, &err)
	if err != 0 {
		gErrorFree(err)
	}
	w.trays[cfg.ID] = tr
}

func (w *window) removeTray(tr *linuxTray) {
	if tr.propsID != 0 {
		gDBusConnectionUnregisterObject(tr.bus, tr.propsID)
	}
	if tr.itemID != 0 {
		gDBusConnectionUnregisterObject(tr.bus, tr.itemID)
	}
	if tr.menuID != 0 {
		gDBusConnectionUnregisterObject(tr.bus, tr.menuID)
	}
	if tr.owner != 0 {
		gBusUnownName(tr.owner)
	}
	delete(w.trays, tr.cfg.ID)
}

func (w *window) removeTrays() {
	for _, tr := range w.trays {
		w.removeTray(tr)
	}
}

func trayWindow(data uintptr) *window { return windowOf(data) }

func trayNameAcquired(bus, name, data uintptr) {
	w := trayWindow(data)
	if w == nil || w.closing {
		return
	}
	service := goString(name)
	idx := strings.LastIndexByte(service, '-')
	if idx < 0 {
		return
	}
	id, err := strconv.ParseUint(service[idx+1:], 10, 32)
	if err != nil || w.trays[uint32(id)] == nil {
		return
	}
	// The protocol registers the service name; hosts use its conventional
	// /StatusNotifierItem object path. A missing watcher is normal on GNOME
	// without its AppIndicator extension.
	arg := gVariantNewString(cstr(service))
	if reply := dbusCall(bus, trayWatcher, trayWatcherPath, trayWatcherIface, "RegisterStatusNotifierItem", arg); reply != 0 {
		gVariantUnref(reply)
	} else {
		dbusCall(bus, trayWatcherAlt, trayWatcherPath, trayWatcherAlt, "RegisterStatusNotifierItem", gVariantNewString(cstr(service)))
	}
}

func trayMethodCall(conn, sender, path, iface, method, params, invocation, data uintptr) {
	w := trayWindow(data)
	if w == nil || w.engine == nil || w.closing {
		returnTray(invocation)
		return
	}
	id := trayID(w, path)
	if _, ok := w.trays[id]; !ok {
		returnTray(invocation)
		return
	}
	interfaceName := methodString(iface)
	if interfaceName == trayProperties {
		if strings.HasSuffix(goString(path), "/Menu") {
			gDBusMethodInvocationReturnValue(invocation, trayMenuPropertiesMethod(w, id, methodString(method), params))
		} else {
			gDBusMethodInvocationReturnValue(invocation, trayPropertiesMethod(w, id, methodString(method), params))
		}
		return
	}
	if interfaceName == trayMenuInterface {
		gDBusMethodInvocationReturnValue(invocation, trayMenuMethod(w, id, methodString(method), params))
		return
	}
	switch methodString(method) {
	case "Activate":
		w.engine.Dispatch(driver.Event{Kind: driver.EventTray, Count: id, Index: 0})
	case "SecondaryActivate", "ContextMenu":
		w.engine.Dispatch(driver.Event{Kind: driver.EventTray, Count: id, Index: 1})
	}
	returnTray(invocation)
}

func trayGetProperty(conn, sender, path, iface, property uintptr, err, data uintptr) uintptr {
	w := trayWindow(data)
	if w == nil {
		return 0
	}
	if strings.HasSuffix(goString(path), "/Menu") {
		return menuProperty(goString(property))
	}
	return trayProperty(w, trayID(w, path), goString(property))
}

func traySetProperty(conn, sender, path, iface, property, value, err, data uintptr) uintptr {
	return 0
}

func returnTray(invocation uintptr) {
	if invocation != 0 {
		gDBusMethodInvocationReturnValue(invocation, gVariantNewTuple(nil, 0))
	}
}

func methodString(p uintptr) string {
	if p == 0 {
		return ""
	}
	return goString(p)
}

func trayID(w *window, path uintptr) uint32 {
	p := goString(path)
	if p != "/StatusNotifierItem" && p != "/Menu" {
		return 0
	}
	for id := range w.trays {
		return id
	}
	return 0
}

func trayProperty(w *window, id uint32, property string) uintptr {
	tr := w.trays[id]
	if tr == nil {
		return 0
	}
	cfg := tr.cfg
	switch property {
	case "Category":
		return gVariantNewString(cstr("ApplicationStatus"))
	case "Id":
		return gVariantNewString(cstr(w.opts.ID))
	case "Title":
		return gVariantNewString(cstr(cfg.Title))
	case "Status":
		return gVariantNewString(cstr("Active"))
	case "WindowId":
		return gVariantNewInt32(0)
	case "IconName":
		return gVariantNewString(cstr(w.opts.ID))
	case "IconPixmap":
		return trayPixmap(cfg.Icon)
	case "ToolTip":
		children := []uintptr{gVariantNewString(cstr(w.opts.ID)), trayPixmap(cfg.Icon), gVariantNewString(cstr(cfg.Tooltip)), gVariantNewString(cstr(""))}
		return gVariantNewTuple(&children[0], uintptr(len(children)))
	case "Menu":
		return gVariantNewObjectPath(cstr(tr.menuPath))
	}
	return 0
}

func trayMenuMethod(w *window, id uint32, method string, params uintptr) uintptr {
	tr := w.trays[id]
	if tr == nil {
		return gVariantNewTuple(nil, 0)
	}
	switch method {
	case "GetLayout":
		// The method's output is (u(ia{sv}av)).
		children := []uintptr{gVariantNewUint32(tr.revision), menuNode(0, nil, tr.menu)}
		return gVariantNewTuple(&children[0], uintptr(len(children)))
	case "GetGroupProperties":
		return menuGroupProperties(tr.menu)
	case "GetProperty":
		name := variantString(params, 1)
		value := menuProperty(name)
		v := gVariantNewVariant(value)
		return gVariantNewTuple(&v, 1)
	case "Event":
		if variantString(params, 1) == "clicked" {
			child := gVariantGetChildValue(params, 0)
			itemID := uint32(gVariantGetInt32(child))
			gVariantUnref(child)
			if itemID != 0 {
				w.engine.Dispatch(driver.Event{Kind: driver.EventMenu, Count: itemID})
			}
			if tr.popupID != 0 {
				w.engine.Dispatch(driver.Event{Kind: driver.EventMenuClosed, Count: tr.popupID})
				tr.popupID = 0
			}
		}
		return gVariantNewTuple(nil, 0)
	case "AboutToShow":
		v := gVariantNewBoolean(1)
		return gVariantNewTuple(&v, 1)
	case "AboutToShowGroup":
		return menuEmptyIntArrays()
	}
	return gVariantNewTuple(nil, 0)
}

func trayMenuPropertiesMethod(w *window, id uint32, method string, params uintptr) uintptr {
	switch method {
	case "Get":
		v := gVariantNewVariant(menuProperty(variantString(params, 1)))
		return gVariantNewTuple(&v, 1)
	case "GetAll":
		entries := make([]uintptr, 0, 4)
		for _, name := range []string{"Version", "TextDirection", "Status", "IconThemePath"} {
			entries = append(entries, menuDictEntry(name, menuProperty(name)))
		}
		value := menuDict(entries)
		return gVariantNewTuple(&value, 1)
	}
	return gVariantNewTuple(nil, 0)
}

func menuProperty(name string) uintptr {
	switch name {
	case "Version":
		return gVariantNewUint32(3)
	case "TextDirection":
		return gVariantNewString(cstr("ltr"))
	case "Status":
		return gVariantNewString(cstr("normal"))
	case "IconThemePath":
		return gVariantNewString(cstr(""))
	default:
		return gVariantNewString(cstr(""))
	}
}

func menuNode(id int32, entry *driver.MenuEntry, children []driver.MenuEntry) uintptr {
	properties := menuEntryProperties(entry)
	childValues := make([]uintptr, len(children))
	for i := range children {
		childValues[i] = gVariantNewVariant(menuNode(int32(children[i].ID), &children[i], children[i].Items))
	}
	childArray := menuArray("v", childValues)
	node := []uintptr{gVariantNewInt32(id), properties, childArray}
	return gVariantNewTuple(&node[0], uintptr(len(node)))
}

func menuEntryProperties(entry *driver.MenuEntry) uintptr {
	props := []uintptr{}
	if entry != nil {
		props = append(props, menuDictEntry("label", gVariantNewString(cstr(entry.Label))))
		props = append(props, menuDictEntry("enabled", gVariantNewBoolean(menuBool(!entry.Disabled))))
		props = append(props, menuDictEntry("visible", gVariantNewBoolean(1)))
		if entry.Separator {
			props = append(props, menuDictEntry("type", gVariantNewString(cstr("separator"))))
		}
		if len(entry.Items) != 0 {
			props = append(props, menuDictEntry("children-display", gVariantNewString(cstr("submenu"))))
		}
		if entry.Checkable {
			props = append(props, menuDictEntry("toggle-type", gVariantNewString(cstr("checkmark"))))
			state := int32(0)
			if entry.Checked {
				state = 1
			}
			props = append(props, menuDictEntry("toggle-state", gVariantNewInt32(state)))
		}
	}
	return menuDict(props)
}

func menuGroupProperties(entries []driver.MenuEntry) uintptr {
	values := make([]uintptr, len(entries))
	for i := range entries {
		children := []uintptr{gVariantNewInt32(int32(entries[i].ID)), menuEntryProperties(&entries[i])}
		values[i] = gVariantNewTuple(&children[0], uintptr(len(children)))
	}
	return menuArray("(ia{sv})", values)
}

func menuEmptyIntArrays() uintptr {
	empty := menuArray("i", nil)
	values := []uintptr{empty, menuArray("i", nil)}
	return gVariantNewTuple(&values[0], uintptr(len(values)))
}

func menuDictEntry(key string, value uintptr) uintptr {
	return gVariantNewDictEntry(gVariantNewString(cstr(key)), gVariantNewVariant(value))
}

func menuDict(entries []uintptr) uintptr {
	return menuArray("{sv}", entries)
}

func menuArray(signature string, values []uintptr) uintptr {
	t := gVariantTypeNew(cstr(signature))
	var ptr *uintptr
	if len(values) != 0 {
		ptr = &values[0]
	}
	v := gVariantNewArray(t, ptr, uintptr(len(values)))
	gVariantTypeFree(t)
	return v
}

func menuBool(v bool) int32 {
	if v {
		return 1
	}
	return 0
}

// trayPixmap converts the PNG in TrayConfig to the StatusNotifierItem's
// network-order ARGB32 pixmap. An empty array lets a host use IconName.
func trayPixmap(data []byte) uintptr {
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil || img.Bounds().Empty() {
		t := gVariantTypeNew(cstr("(iiay)"))
		v := gVariantNewArray(t, nil, 0)
		gVariantTypeFree(t)
		return v
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w > 64 || h > 64 {
		w, h = min(w, 64), min(h, 64)
	}
	pix := make([]byte, w*h*4)
	for y := range h {
		sy := b.Min.Y + y*b.Dy()/h
		for x := range w {
			sx := b.Min.X + x*b.Dx()/w
			r, g, bl, a := img.At(sx, sy).RGBA()
			i := (y*w + x) * 4
			pix[i], pix[i+1], pix[i+2], pix[i+3] = byte(a/0x100), byte(r/0x100), byte(g/0x100), byte(bl/0x100)
		}
	}
	byteType := gVariantTypeNew(cstr("y"))
	bytesValue := gVariantNewFixedArray(byteType, unsafe.Pointer(&pix[0]), uintptr(len(pix)), 1)
	gVariantTypeFree(byteType)
	children := []uintptr{gVariantNewInt32(int32(w)), gVariantNewInt32(int32(h)), bytesValue}
	item := gVariantNewTuple(&children[0], uintptr(len(children)))
	tupleType := gVariantTypeNew(cstr("(iiay)"))
	result := gVariantNewArray(tupleType, &item, 1)
	gVariantTypeFree(tupleType)
	return result
}

// Properties.Get and GetAll are handled by the same vtable as the item
// methods because GDBus registers each interface against one object path.
func trayPropertiesMethod(w *window, id uint32, method string, params uintptr) uintptr {
	switch method {
	case "Get":
		name := variantString(params, 1)
		value := trayProperty(w, id, name)
		if value == 0 {
			return gVariantNewTuple(nil, 0)
		}
		v := gVariantNewVariant(value)
		return gVariantNewTuple(&v, 1)
	case "GetAll":
		entries := make([]uintptr, 0, 9)
		for _, name := range []string{"Category", "Id", "Title", "Status", "WindowId", "IconName", "IconPixmap", "ToolTip", "Menu"} {
			key := gVariantNewString(cstr(name))
			entries = append(entries, gVariantNewDictEntry(key, gVariantNewVariant(trayProperty(w, id, name))))
		}
		t := gVariantTypeNew(cstr("{sv}"))
		props := gVariantNewArray(t, &entries[0], uintptr(len(entries)))
		gVariantTypeFree(t)
		return gVariantNewTuple(&props, 1)
	}
	return gVariantNewTuple(nil, 0)
}
