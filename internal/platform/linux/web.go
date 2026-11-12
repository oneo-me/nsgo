//go:build linux && !android

package linux

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/ebitengine/purego"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/engine"
)

// WebKitGTK 6.0, the GTK 4 one, loads on the first web view, so apps
// without one neither pay for it nor need it installed.
var (
	webkitOnce sync.Once
	webkitErr  error

	webkitWebViewGetType                 func() uintptr
	webkitWebViewGetBackgroundColor      func(uintptr, *[4]float32)
	webkitNetworkSessionNew              func(*byte, *byte) uintptr
	webkitNetworkSessionGetCookieManager func(uintptr) uintptr
	webkitCookieManagerSetPersistent     func(uintptr, *byte, int32)
	webkitWebViewGetUserContentManager   func(uintptr) uintptr
	webkitWebViewGetSettings             func(uintptr) uintptr
	webkitWebViewSetBackgroundColor      func(uintptr, *[4]float32)
	webkitWebViewLoadURI                 func(uintptr, *byte)
	webkitWebViewLoadHTML                func(uintptr, *byte, *byte)
	webkitWebViewReload                  func(uintptr)
	webkitWebViewGoBack                  func(uintptr)
	webkitWebViewGoForward               func(uintptr)
	webkitWebViewStopLoading             func(uintptr)
	webkitWebViewEvaluateJavascript      func(uintptr, *byte, int, *byte, *byte, uintptr, uintptr, uintptr)
	webkitWebViewExecuteEditingCommand   func(uintptr, *byte)
	webkitWebViewGetURI                  func(uintptr) uintptr
	webkitWebViewGetTitle                func(uintptr) uintptr
	webkitWebViewCanGoBack               func(uintptr) int32
	webkitWebViewCanGoForward            func(uintptr) int32
	webkitSettingsSetEnableJSMarkup      func(uintptr, int32)
	webkitSettingsSetUserAgent           func(uintptr, *byte)
	webkitSettingsSetBackForwardGestures func(uintptr, int32)
	webkitContentRegisterHandler         func(uintptr, *byte, *byte) int32
	webkitContentUnregisterHandler       func(uintptr, *byte, *byte)
	webkitContentAddScript               func(uintptr, uintptr)
	webkitContentRemoveAllScripts        func(uintptr)
	webkitContentAddStyleSheet           func(uintptr, uintptr)
	webkitContentRemoveAllStyleSheets    func(uintptr)
	webkitUserScriptNew                  func(*byte, int32, int32, uintptr, uintptr) uintptr
	webkitUserScriptUnref                func(uintptr)
	webkitUserStyleSheetNew              func(*byte, int32, int32, uintptr, uintptr) uintptr
	webkitUserStyleSheetUnref            func(uintptr)
	webkitNetworkErrorQuark              func() uint32
	webkitPolicyErrorQuark               func() uint32
	webkitNavigationDecisionGetAction    func(uintptr) uintptr
	webkitNavigationActionGetRequest     func(uintptr) uintptr
	webkitURIRequestGetURI               func(uintptr) uintptr
	webkitPolicyDecisionIgnore           func(uintptr)
	jscValueToString                     func(uintptr) uintptr
)

// openWebKit loads WebKitGTK and JavaScriptCore, reporting what is
// missing.
func openWebKit() error {
	webkitOnce.Do(func() {
		webkit, err := purego.Dlopen("libwebkitgtk-6.0.so.4", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			webkitErr = fmt.Errorf("web views need WebKitGTK 6.0 (libwebkitgtk-6.0.so.4): %w", err)
			return
		}
		jsc, err := purego.Dlopen("libjavascriptcoregtk-6.0.so.1", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			webkitErr = fmt.Errorf("web views need JavaScriptCore 6.0 (libjavascriptcoregtk-6.0.so.1): %w", err)
			return
		}
		defer func() {
			if r := recover(); r != nil {
				webkitErr = fmt.Errorf("web views need WebKitGTK 6.0: %v", r)
			}
		}()
		bindWebKit(webkit, jsc)
	})
	return webkitErr
}

func bindWebKit(webkit, jsc uintptr) {
	reg := func(fn any, name string) { purego.RegisterLibFunc(fn, webkit, name) }
	reg(&webkitWebViewGetType, "webkit_web_view_get_type")
	reg(&webkitWebViewGetBackgroundColor, "webkit_web_view_get_background_color")
	reg(&webkitNetworkSessionNew, "webkit_network_session_new")
	reg(&webkitNetworkSessionGetCookieManager, "webkit_network_session_get_cookie_manager")
	reg(&webkitCookieManagerSetPersistent, "webkit_cookie_manager_set_persistent_storage")
	reg(&webkitWebViewGetUserContentManager, "webkit_web_view_get_user_content_manager")
	reg(&webkitWebViewGetSettings, "webkit_web_view_get_settings")
	reg(&webkitWebViewSetBackgroundColor, "webkit_web_view_set_background_color")
	reg(&webkitWebViewLoadURI, "webkit_web_view_load_uri")
	reg(&webkitWebViewLoadHTML, "webkit_web_view_load_html")
	reg(&webkitWebViewReload, "webkit_web_view_reload")
	reg(&webkitWebViewGoBack, "webkit_web_view_go_back")
	reg(&webkitWebViewGoForward, "webkit_web_view_go_forward")
	reg(&webkitWebViewStopLoading, "webkit_web_view_stop_loading")
	reg(&webkitWebViewEvaluateJavascript, "webkit_web_view_evaluate_javascript")
	reg(&webkitWebViewExecuteEditingCommand, "webkit_web_view_execute_editing_command")
	reg(&webkitWebViewGetURI, "webkit_web_view_get_uri")
	reg(&webkitWebViewGetTitle, "webkit_web_view_get_title")
	reg(&webkitWebViewCanGoBack, "webkit_web_view_can_go_back")
	reg(&webkitWebViewCanGoForward, "webkit_web_view_can_go_forward")
	reg(&webkitSettingsSetEnableJSMarkup, "webkit_settings_set_enable_javascript_markup")
	reg(&webkitSettingsSetUserAgent, "webkit_settings_set_user_agent")
	reg(&webkitSettingsSetBackForwardGestures, "webkit_settings_set_enable_back_forward_navigation_gestures")
	reg(&webkitContentRegisterHandler, "webkit_user_content_manager_register_script_message_handler")
	reg(&webkitContentUnregisterHandler, "webkit_user_content_manager_unregister_script_message_handler")
	reg(&webkitContentAddScript, "webkit_user_content_manager_add_script")
	reg(&webkitContentRemoveAllScripts, "webkit_user_content_manager_remove_all_scripts")
	reg(&webkitContentAddStyleSheet, "webkit_user_content_manager_add_style_sheet")
	reg(&webkitContentRemoveAllStyleSheets, "webkit_user_content_manager_remove_all_style_sheets")
	reg(&webkitUserScriptNew, "webkit_user_script_new")
	reg(&webkitUserScriptUnref, "webkit_user_script_unref")
	reg(&webkitUserStyleSheetNew, "webkit_user_style_sheet_new")
	reg(&webkitUserStyleSheetUnref, "webkit_user_style_sheet_unref")
	reg(&webkitNetworkErrorQuark, "webkit_network_error_quark")
	reg(&webkitPolicyErrorQuark, "webkit_policy_error_quark")
	reg(&webkitNavigationDecisionGetAction, "webkit_navigation_policy_decision_get_navigation_action")
	reg(&webkitNavigationActionGetRequest, "webkit_navigation_action_get_request")
	reg(&webkitURIRequestGetURI, "webkit_uri_request_get_uri")
	reg(&webkitPolicyDecisionIgnore, "webkit_policy_decision_ignore")
	purego.RegisterLibFunc(&jscValueToString, jsc, "jsc_value_to_string")
}

// WebKit's enums.
const (
	webkitTopFrame         = 1 // WEBKIT_USER_CONTENT_INJECT_TOP_FRAME
	webkitAtStart          = 0 // WEBKIT_USER_SCRIPT_INJECT_AT_DOCUMENT_START
	webkitAtEnd            = 1 // WEBKIT_USER_SCRIPT_INJECT_AT_DOCUMENT_END
	webkitUserStyle        = 0 // WEBKIT_USER_STYLE_LEVEL_USER
	webkitLoadStarted      = 0
	webkitLoadFinished     = 3
	webkitNetworkCancelled = 302 // WEBKIT_NETWORK_ERROR_CANCELLED
	webkitPolicyInterrupt  = 102 // WEBKIT_POLICY_ERROR_FRAME_LOAD_INTERRUPTED_BY_POLICY_CHANGE
	webkitNewWindowAction  = 1   // WEBKIT_POLICY_DECISION_TYPE_NEW_WINDOW_ACTION
	webkitCookieSQLite     = 1   // WEBKIT_COOKIE_PERSISTENT_STORAGE_SQLITE
	gtkOverflowHidden      = 1
)

// webHandler is the name pages post to, as
// window.webkit.messageHandlers.nsgo.
const webHandler = "nsgo"

var webBridge = driver.WebBridge("function (d) { window.webkit.messageHandlers." + webHandler + ".postMessage(d); }")

// webNoScroll keeps the top document from scrolling.
const webNoScroll = "html, body { overflow: hidden !important; }"

// web is a web view's WebKitWebView, hosted in the view's widget as NSH's
// Linux backend hosts it. Without WebKitGTK it is a placeholder label and
// widget is 0.
type web struct {
	hosted
	widget     uintptr // WebKitWebView
	content    uintptr // its WebKitUserContentManager
	config     driver.WebConfig
	made       bool
	failed     bool         // the navigation failed; WebKit still finishes it
	pending    bool         // a load waits for the page to be mapped
	background [4]float32   // WebKit's own, for views without an opaque one
	pointer    driver.Point // where the pointer is in the page
}

// webWarned is the warning, once, that web views show a placeholder.
var webWarned sync.Once

// configureWeb applies a web view's configuration, making its page on
// the first one and loading the source again when it or the scripts
// changed; the rest applies to the page as it is.
func (w *window) configureWeb(v *view, c *driver.WebConfig) {
	if v.web == nil {
		v.web = &web{}
		if err := openWebKit(); err != nil {
			webWarned.Do(func() { log.Printf("nsgo: WebView is not available: %v; it shows a placeholder", err) })
			w.host(v, &v.web.hosted, webPlaceholder(), false)
			return
		}
		w.makeWeb(v)
	}
	wv := v.web
	if wv.widget == 0 {
		return
	}
	old, first := wv.config, !wv.made
	wv.config, wv.made = *c, true
	scripts := first || c.InjectedJavaScript != old.InjectedJavaScript ||
		c.InjectedJavaScriptBeforeContent != old.InjectedJavaScriptBeforeContent
	if scripts {
		webkitContentRemoveAllScripts(wv.content)
		addScript(wv.content, webBridge, webkitAtStart)
		addScript(wv.content, c.InjectedJavaScriptBeforeContent, webkitAtStart)
		addScript(wv.content, c.InjectedJavaScript, webkitAtEnd)
	}
	if first || c.NoScroll != old.NoScroll {
		// Wheels over a page that does not scroll go to the scroll views
		// around it (webScrollCallback).
		webkitContentRemoveAllStyleSheets(wv.content)
		if c.NoScroll {
			sheet := webkitUserStyleSheetNew(cstr(webNoScroll), webkitTopFrame, webkitUserStyle, 0, 0)
			webkitContentAddStyleSheet(wv.content, sheet)
			webkitUserStyleSheetUnref(sheet)
		}
	}
	settings := webkitWebViewGetSettings(wv.widget)
	// Markup's scripts only: injected ones still run, as WebKit's
	// allowsContentJavaScript has it on Apple's platforms.
	webkitSettingsSetEnableJSMarkup(settings, boolInt(!c.NoJavaScript))
	if c.UserAgent == "" {
		webkitSettingsSetUserAgent(settings, nil)
	} else {
		webkitSettingsSetUserAgent(settings, cstr(c.UserAgent))
	}
	webkitSettingsSetBackForwardGestures(settings, boolInt(c.BackForwardGestures))
	w.webBackground(v)
	// Scripts apply to pages loaded from now on: load again.
	if first || scripts || c.Source != old.Source {
		w.loadWeb(v)
	}
}

// webBackground shows the view's background behind the page while it
// loads and behind pages without one of their own, when it is opaque,
// as on Windows; else WebKit's own.
func (w *window) webBackground(v *view) {
	wv := v.web
	if wv == nil || wv.widget == 0 {
		return
	}
	bg := wv.background
	if c := v.style.Background; c&0xff == 0xff {
		bg = [4]float32{float32(c>>24) / 255, float32(c>>16&0xff) / 255, float32(c>>8&0xff) / 255, 1}
	}
	webkitWebViewSetBackgroundColor(wv.widget, &bg)
}

// webPlaceholder is what a web view shows without WebKitGTK, as NSH's
// does: gray, which reads on light and dark apps alike.
func webPlaceholder() uintptr {
	label := gtkLabelNew(nil)
	gtkLabelSetMarkup(label, cstr(`<span foreground="#8c8c8c">WebView is not available on Linux</span>`))
	gtkLabelSetWrap(label, 1)
	// Small boxes clip the text rather than overflow.
	gtkLabelSetEllipsize(label, pangoEllipsizeEnd)
	gtkLabelSetLines(label, 3)
	gtkLabelSetJustify(label, gtkJustifyCenter)
	gtkWidgetSetCanTarget(label, 0)
	return label
}

func addScript(content uintptr, source string, at int32) {
	if source == "" {
		return
	}
	script := webkitUserScriptNew(cstr(source), webkitTopFrame, at, 0, 0)
	webkitContentAddScript(content, script)
	webkitUserScriptUnref(script)
}

// makeWeb makes v's page in the app's network session and hosts it.
func (w *window) makeWeb(v *view) {
	wv := v.web
	wv.widget = gObjectNew(webkitWebViewGetType(), cstr("network-session"), w.webSession(), uintptr(0))
	wv.content = webkitWebViewGetUserContentManager(wv.widget)
	webkitContentRegisterHandler(wv.content, cstr(webHandler), nil)
	webkitWebViewGetBackgroundColor(wv.widget, &wv.background)
	w.host(v, &wv.hosted, wv.widget, true)
	w.connect(v, wv.widget, "load-changed", cbWebLoadChanged)
	w.connect(v, wv.widget, "load-failed", cbWebLoadFailed)
	w.connect(v, wv.widget, "decide-policy", cbWebPolicy)
	w.connect(v, wv.widget, "map", cbWebMap)
	w.connect(v, wv.content, "script-message-received::"+webHandler, cbWebMessage)

	// History keys and mouse buttons, before WebKit sees them: it keeps
	// arrow keys, used or not, so Alt+Left would never reach the window.
	keys := gtkEventControllerKeyNew()
	gtkEventControllerSetPropagationPhase(keys, propagationPhaseCapture)
	w.connect(v, keys, "key-pressed", cbWebKey)
	gtkWidgetAddController(wv.widget, keys)
	buttons := gtkGestureClickNew()
	// Any button: only the back (8) and forward (9) ones are taken.
	gtkGestureSingleSetButton(buttons, 0)
	gtkEventControllerSetPropagationPhase(buttons, propagationPhaseCapture)
	w.connect(v, buttons, "pressed", cbWebButton)
	gtkWidgetAddController(wv.widget, buttons)

	motion := gtkEventControllerMotionNew()
	w.connect(v, motion, "motion", cbWebMotion)
	gtkWidgetAddController(wv.widget, motion)
	scroll := gtkEventControllerScrollNew(3) // both axes
	gtkEventControllerSetPropagationPhase(scroll, propagationPhaseCapture)
	w.connect(v, scroll, "scroll", cbWebScroll)
	gtkWidgetAddController(wv.widget, scroll)
}

// webSessions are the network sessions by app ID: one network process,
// and one set of cookies and storage, serves every web view of an app.
var webSessions = map[string]uintptr{}

// webSession returns the network session of w's app, which keeps
// website data in $XDG_DATA_HOME/<app id>/webkit and its cache in
// $XDG_CACHE_HOME/<app id>/webkit, with cookies in cookies.sqlite there.
func (w *window) webSession() uintptr {
	if s := webSessions[w.appID]; s != 0 {
		return s
	}
	data, cache := webFolders(w.appID)
	if err := os.MkdirAll(data, 0o755); err != nil {
		log.Printf("nsgo: web view: %v", err)
	}
	s := webkitNetworkSessionNew(cstr(data), cstr(cache))
	// Cookies outlive the app, as other website data does.
	if cookies := webkitNetworkSessionGetCookieManager(s); cookies != 0 {
		webkitCookieManagerSetPersistent(cookies, cstr(filepath.Join(data, "cookies.sqlite")), webkitCookieSQLite)
	}
	webSessions[w.appID] = s
	return s
}

// webFolders are where web views of app id keep their data and cache.
func webFolders(id string) (data, cache string) {
	data, cache = engine.AppDirs(id)
	return filepath.Join(data, "webkit"), filepath.Join(cache, "webkit")
}

// loadWeb loads v's source once its page is mapped. With WebKit's
// DMA-BUF renderer a page loaded before it is mapped can stay blank for
// good (NSH's Linux backend found so), so WebKit's MiniBrowser loads
// after mapping too.
func (w *window) loadWeb(v *view) {
	wv := v.web
	if gtkWidgetGetMapped(wv.widget) == 0 {
		wv.pending = true
		return
	}
	wv.pending = false
	src := wv.config.Source
	switch {
	case src.HTML != "":
		var base *byte
		if src.BaseURL != "" {
			base = cstr(src.BaseURL)
		}
		webkitWebViewLoadHTML(wv.widget, cstr(src.HTML), base)
	case src.Src != "":
		webkitWebViewLoadURI(wv.widget, cstr(src.Src))
	}
}

// webCommand has v's page do c, with value for messages and scripts.
func (w *window) webCommand(v *view, c driver.WebCommand, value string) {
	if v.web == nil || v.web.widget == 0 {
		return // the placeholder has no page
	}
	page := v.web.widget
	switch c {
	case driver.WebReload:
		webkitWebViewReload(page)
	case driver.WebGoBack:
		webkitWebViewGoBack(page)
	case driver.WebGoForward:
		webkitWebViewGoForward(page)
	case driver.WebStopLoading:
		webkitWebViewStopLoading(page)
	case driver.WebPostMessage:
		evaluate(page, driver.WebMessageScript(value))
	case driver.WebInjectJavaScript:
		evaluate(page, value)
	}
}

func evaluate(page uintptr, script string) {
	webkitWebViewEvaluateJavascript(page, cstr(script), -1, nil, nil, 0, 0, 0)
}

// closeWeb stops v's page and lets it go.
func (w *window) closeWeb(v *view) {
	wv := v.web
	v.web = nil
	if wv.widget != 0 {
		webkitWebViewStopLoading(wv.widget)
		// Before unhosting, which lets the page and its content manager go.
		webkitContentUnregisterHandler(wv.content, cstr(webHandler), nil)
	}
	w.unhost(v, &wv.hosted)
}

// focusedWeb returns the web view whose page has keyboard focus, or nil.
func (w *window) focusedWeb() *view {
	if len(w.natives) == 0 {
		return nil
	}
	for f := gtkWindowGetFocus(w.win); f != 0; f = gtkWidgetGetParent(f) {
		if v := w.natives[f]; v != nil && v.web != nil && v.web.widget == f {
			return v
		}
	}
	return nil
}

// webEdit runs an editing menu role in the focused page, if any.
func (w *window) webEdit(role driver.MenuRole) bool {
	v := w.focusedWeb()
	command := map[driver.MenuRole]string{
		driver.MenuUndo: "Undo", driver.MenuRedo: "Redo", driver.MenuCut: "Cut",
		driver.MenuCopy: "Copy", driver.MenuPaste: "Paste", driver.MenuSelectAll: "SelectAll",
	}[role]
	if v == nil || command == "" {
		return false
	}
	webkitWebViewExecuteEditingCommand(v.web.widget, cstr(command))
	return true
}

// webOf returns the live web view whose page p belongs to.
func webOf(data, p uintptr) (*window, *view) {
	w, v := nativeOf(data, p)
	if v == nil || v.web == nil || v.web.widget == 0 {
		return nil, nil
	}
	return w, v
}

// navigated reports the page's state as kind and as a navigation state
// change.
func (w *window) navigated(v *view, loading bool, kind driver.EventKind) {
	page := v.web.widget
	nav := driver.WebNavigation{
		URL:          goString(webkitWebViewGetURI(page)),
		Title:        goString(webkitWebViewGetTitle(page)),
		Loading:      loading,
		CanGoBack:    webkitWebViewCanGoBack(page) != 0,
		CanGoForward: webkitWebViewCanGoForward(page) != 0,
	}
	for _, k := range []driver.EventKind{kind, driver.EventNavigationStateChange} {
		n := nav
		w.report(v, driver.Event{Kind: k, ID: v.id, Navigation: &n})
	}
}

func webLoadChangedCallback(page uintptr, event uint32, data uintptr) {
	w, v := webOf(data, page)
	if v == nil {
		return
	}
	switch event {
	case webkitLoadStarted:
		v.web.failed = false
		w.navigated(v, true, driver.EventLoadStart)
	case webkitLoadFinished:
		// A failed navigation finishes too; it was reported as an error.
		if !v.web.failed {
			w.navigated(v, false, driver.EventLoad)
		}
	}
}

// gError is GLib's GError.
type gError struct {
	domain  uint32
	code    int32
	message uintptr
}

func webLoadFailedCallback(page uintptr, event uint32, uri, gerr uintptr, data uintptr) int32 {
	w, v := webOf(data, page)
	if v == nil {
		return 0
	}
	v.web.failed = true
	if gerr != 0 {
		e := (*gError)(cPointer(gerr))
		// Stopped or replaced by another navigation: not errors.
		cancelled := e.domain == webkitNetworkErrorQuark() && e.code == webkitNetworkCancelled ||
			e.domain == webkitPolicyErrorQuark() && e.code == webkitPolicyInterrupt
		if !cancelled {
			w.report(v, driver.Event{Kind: driver.EventError, ID: v.id, Text: goString(e.message),
				Navigation: &driver.WebNavigation{URL: goString(uri)}})
		}
	}
	return 1 // the app shows failures itself, without WebKit's error page
}

// webPolicyCallback refuses the navigations a restricted view's config
// does not allow: the top level's (decide-policy's navigation actions)
// and new windows'.
func webPolicyCallback(page, decision uintptr, kind uint32, data uintptr) int32 {
	w, v := webOf(data, page)
	if v == nil || !v.web.config.Restricted || kind > webkitNewWindowAction {
		return 0
	}
	url := goString(webkitURIRequestGetURI(webkitNavigationActionGetRequest(webkitNavigationDecisionGetAction(decision))))
	if v.web.config.Navigable(url) {
		return 0
	}
	webkitPolicyDecisionIgnore(decision)
	w.report(v, driver.Event{Kind: driver.EventError, ID: v.id, Text: driver.RefusedNavigation(url),
		Navigation: &driver.WebNavigation{URL: url}})
	return 1
}

func webMessageCallback(content, value uintptr, data uintptr) {
	w, v := webOf(data, content)
	if v == nil {
		return
	}
	p := jscValueToString(value)
	text := goString(p)
	if p != 0 {
		gFree(p)
	}
	w.report(v, driver.Event{Kind: driver.EventMessage, ID: v.id, Text: text})
}

func webMapCallback(page uintptr, data uintptr) {
	w, v := webOf(data, page)
	if v != nil && v.web.pending {
		w.loadWeb(v)
	}
}

// The keys that step through a page's history, beside the arrows.
const (
	gdkKPLeft  = 0xff96
	gdkKPRight = 0xff98
	gdkBack    = 0x1008ff26
	gdkForward = 0x1008ff27

	gestureClaimed = 1 // GTK_EVENT_SEQUENCE_CLAIMED
)

// historyKey reports which way keyval with state steps through history:
// -1 back, 1 forward, 0 neither. Lock keys and held buttons do not count.
func historyKey(keyval, state uint32) int {
	mods := state & (gdkControl | gdkAlt | gdkShift | gdkSuper)
	switch {
	case keyval == gdkBack && mods == 0:
		return -1
	case keyval == gdkForward && mods == 0:
		return 1
	case (keyval == gdkLeft || keyval == gdkKPLeft) && mods == gdkAlt:
		return -1
	case (keyval == gdkRight || keyval == gdkKPRight) && mods == gdkAlt:
		return 1
	}
	return 0
}

// goHistory steps the page way through its history, or back in the app
// when the page has nowhere to go back to, and reports whether something
// took it.
func (w *window) goHistory(v *view, way int) bool {
	page := v.web.widget
	switch {
	case way < 0 && webkitWebViewCanGoBack(page) != 0:
		webkitWebViewGoBack(page)
	case way > 0 && webkitWebViewCanGoForward(page) != 0:
		webkitWebViewGoForward(page)
	case way < 0:
		w.back()
	default:
		return false
	}
	return true
}

func webKeyCallback(controller uintptr, keyval, keycode, state uint32, data uintptr) int32 {
	w, v := webOf(data, controller)
	if v == nil {
		return 0
	}
	if way := historyKey(keyval, state); way != 0 && w.goHistory(v, way) {
		return 1
	}
	return 0
}

func webButtonCallback(gesture uintptr, presses int32, x, y float64, data uintptr) {
	w, v := webOf(data, gesture)
	if v == nil {
		return
	}
	way := 0
	switch gtkGestureSingleGetCurrentButton(gesture) {
	case 8:
		way = -1
	case 9:
		way = 1
	}
	if way != 0 && w.goHistory(v, way) {
		gtkGestureSetState(gesture, gestureClaimed)
	}
}

func webMotionCallback(controller uintptr, x, y float64, data uintptr) {
	if _, v := webOf(data, controller); v != nil {
		v.web.pointer = driver.Point{X: driver.Points(x), Y: driver.Points(y)}
	}
}

// webScrollCallback leaves the wheels over a page that does not scroll
// to the scroll views around it.
func webScrollCallback(controller uintptr, dx, dy float64, data uintptr) int32 {
	w, v := webOf(data, controller)
	if v == nil || !v.web.config.NoScroll {
		return 0
	}
	at := w.frameAt(v)
	at.X += v.web.pointer.X
	at.Y += v.web.pointer.Y
	w.wheelEvent(gtkEventControllerGetCurrentEventState(controller), at, dx, dy, controllerPoints(controller))
	return 1
}
