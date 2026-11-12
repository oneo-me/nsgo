//go:build windows

package windows

// Web views, as NSH's native/web.rs: WebView2 composition controllers in
// the window's composition.
//
// Each web view is a WebView2 composition controller ("visual hosting")
// drawing into a portal of the window's composition (compose.go): the
// frame records where the view goes as it draws the tree, the portal is
// clipped, transformed and faded like the view, and what is drawn after it
// goes on a layer above it, so modals, dialogs and later siblings cover
// the page as they would any view. The window gets the input: it finds the
// view under the pointer as for any view, and what falls on the page
// (nothing drawn after it is there) is forwarded as it comes: mouse
// buttons, moves and wheels through SendMouseInput, touch and pen contacts
// through SendPointerInput. The page's cursor comes back through
// CursorChanged. A press on the page gives it the keyboard focus
// (MoveFocus: WebView2 then takes keys and the IME itself, and the focused
// field blurs, as NSH's text_input::blur_all); a press elsewhere, Tab out of the page (MoveFocusRequested)
// and hiding the page give it back to the window. The page is rasterized
// at the window's scale; its bounds are where its view is in window
// pixels, which WebView2 places pop-ups and the IME's candidates from.
//
// WebView2 is asynchronous: the environment (one per thread and profile
// folder, %LOCALAPPDATA%\<app id>\WebView2) and then the controller are
// made on the message loop, and the configuration and commands that come
// before the controller wait for it. Without the runtime a web view draws
// a placeholder, as NSH's does.
//
// The bridge defines window.nsgo.postMessage(data); PostMessage
// dispatches a message event on the page's window. Events follow NSH's:
// EventLoadStart and EventNavigationStateChange when a navigation starts,
// EventLoad and EventNavigationStateChange when it completes, EventError
// with the page's URL when it fails.
//
//   - HTML with a base URL navigates to the base URL and answers that
//     document's request with the HTML (WebResourceRequested): the page's
//     URL and origin are the base URL's, and relative URLs resolve against
//     it. HTML without one loads with NavigateToString.
//   - NoScroll: the wheel goes to the scroll views around the web view,
//     and a style keeps the page's viewport from scrolling while scripts
//     still may.
//   - BackForwardGestures: WebView2's swipe navigation.
//
// The source loads again only when it or the injected scripts change,
// once the document scripts asked for are added.

import (
	"os"
	"path/filepath"
	"strings"
	"unicode/utf16"
	"unsafe"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

var webBridge = driver.WebBridge("function (d) { window.chrome.webview.postMessage(d); }")

// webView is a web view's portal and, once made, its WebView2 objects.
type webView struct {
	w      *window
	v      *view
	portal *portal // nil once closed, or when the window cannot compose

	env        *comObject // ICoreWebView2Environment
	comp       *comObject // ICoreWebView2CompositionController
	controller *comObject // ICoreWebView2Controller
	web        *comObject // ICoreWebView2
	settings   *comObject // ICoreWebView2Settings
	executed   *comHandler

	ready, closed bool
	// failed is set when no web view can be made: a placeholder shows.
	failed bool

	config, applied        driver.WebConfig
	configured, hasApplied bool
	commands               []webCommand // waiting for the controller
	userAgent              string       // WebView2's own

	// The document scripts: the app's IDs and the one keeping the page
	// from scrolling, the generation of each kind, and how many are still
	// being added, which a navigation waits for.
	scripts               []string
	scrollScript          string
	scriptGen, scrollGen  int
	adding                int
	navigatePending       bool
	html, base            string // the HTML served at its base URL
	serving               bool
	at                    *placement // where the last frame put it
	bounds                rect       // the controller's, when shown
	shown                 bool
	raster                float32
	cursor                uintptr // the page's HCURSOR
	hovered, focused      bool
	buttons               uint8 // mouse buttons held on the page, by bit
	lastDown              uint32
	lastDownAt            point
	lastButton, downCount int
}

type webCommand struct {
	command driver.WebCommand
	value   string
}

// newWeb makes v's portal and starts making its controller.
func (w *window) newWeb(v *view) *webView {
	wv := &webView{w: w, v: v}
	w.webs[wv] = true
	p, err := w.newPortal()
	if err != nil {
		wv.unavailable()
		return wv
	}
	wv.portal = p
	withWebEnvironment(w, w.profileFolder(), wv.environmentReady)
	return wv
}

// profileFolder is where w's web views keep their profile.
func (w *window) profileFolder() string {
	base := w.cacheDir
	if base == "" {
		base = filepath.Join(os.TempDir(), "nsgo")
	}
	return filepath.Join(base, "WebView2")
}

// composed reports whether the page is composed: it takes the pointer
// where nothing covers it.
func (wv *webView) composed() bool { return !wv.failed && !wv.closed && wv.portal != nil }

// unavailable shows the placeholder: there is no web view.
func (wv *webView) unavailable() {
	wv.failed = true
	if wv.portal != nil {
		wv.portal.release()
		wv.portal = nil
	}
	wv.w.redrawAll()
}

func (wv *webView) report(e driver.Event) {
	if wv.v.events.Has(e.Kind) {
		e.ID = wv.v.id
		wv.w.engine.Dispatch(e)
	}
}

func (wv *webView) environmentReady(env *comObject, err error) {
	if wv.closed {
		return
	}
	if err != nil {
		wv.unavailable()
		return
	}
	env3 := env.query(&iidEnvironment3)
	if env3 == nil {
		// The runtime cannot host it in a visual.
		wv.unavailable()
		return
	}
	defer env3.release()
	env.addRef()
	wv.env = env
	var h *comHandler
	h = newHandler(&iidCompositionControllerCompleted, func(hr uintptr, result unsafe.Pointer) {
		defer h.release()
		c := (*comObject)(result)
		if wv.closed {
			// The view went away meanwhile.
			if c != nil && !failed(hr) {
				if ctl := c.query(&iidController); ctl != nil {
					ctl.call(controllerClose)
					ctl.release()
				}
			}
			return
		}
		if failed(hr) || c == nil {
			wv.unavailable()
			return
		}
		c.addRef()
		wv.controllerReady(c)
	})
	if hr := env3.call(envCreateCompositionController, wv.w.hwnd, h.ptr()); failed(hr) {
		h.release()
		wv.unavailable()
	}
}

// controllerReady sets the controller up and applies what waited for it.
func (wv *webView) controllerReady(comp *comObject) {
	wv.comp = comp
	wv.controller = comp.query(&iidController)
	if wv.controller == nil || wv.portal == nil {
		wv.unavailable()
		return
	}
	wv.controller.call(controllerGetCoreWebView2, uintptr(unsafe.Pointer(&wv.web)))
	if wv.web == nil {
		wv.unavailable()
		return
	}
	if failed(comp.call(compPutRootVisualTarget, wv.portal.inner.ptr())) {
		wv.controller.call(controllerClose)
		wv.unavailable()
		return
	}
	if c3 := wv.controller.query(&iidController3); c3 != nil {
		// The window's scale, not the monitor's: the window decides.
		c3.call(controllerPutShouldDetectMonitorScaleChanges, 0)
		c3.release()
	}
	wv.controller.call(controllerPutIsVisible, 0)
	wv.web.call(webGetSettings, uintptr(unsafe.Pointer(&wv.settings)))
	if wv.settings != nil {
		wv.settings.call(settingsPutIsStatusBarEnabled, 0)
		wv.userAgent = wv.settings.getString(settingsGetUserAgent)
	}
	wv.addScript(webBridge, func(string) bool { return true })
	wv.listen()
	wv.listenInput()
	wv.executed = newHandler(&iidExecuteScriptCompleted, nil)
	wv.ready = true
	wv.paintBackground()
	wv.shown = false
	wv.place()
	if wv.configured {
		wv.apply()
	}
	commands := wv.commands
	wv.commands = nil
	for _, c := range commands {
		wv.command(c.command, c.value)
	}
	// The root visual target shows once the composition commits: the
	// next frame does.
	wv.w.redrawAll()
}

// listen reports the page's navigations and messages.
func (wv *webView) listen() {
	web := wv.web
	web.subscribe(webAddNavigationStarting, newHandler(&iidNavigationStarting, func(_ uintptr, args unsafe.Pointer) {
		if wv.closed {
			return
		}
		a := (*comObject)(args)
		nav := wv.navigation(true)
		nav.URL = a.getString(navigationStartingGetUri)
		// A restricted view's page goes only where its config allows;
		// NavigationStarting is the top level's.
		if !wv.config.Navigable(nav.URL) {
			a.call(navigationStartingPutCancel, 1)
			wv.report(driver.Event{Kind: driver.EventError, Text: driver.RefusedNavigation(nav.URL), Navigation: &nav})
			return
		}
		wv.navigated(nav, driver.EventLoadStart, driver.EventNavigationStateChange)
	}))
	web.subscribe(webAddNavigationCompleted, newHandler(&iidNavigationCompleted, func(_ uintptr, args unsafe.Pointer) {
		if wv.closed {
			return
		}
		a := (*comObject)(args)
		var status int32
		a.call(navigationGetWebErrorStatus, uintptr(unsafe.Pointer(&status)))
		if a.getBool(navigationGetIsSuccess) {
			wv.navigated(wv.navigation(false), driver.EventLoad, driver.EventNavigationStateChange)
		} else if why := webError(status); why != "" {
			nav := wv.navigation(false)
			wv.report(driver.Event{Kind: driver.EventError, Text: why, Navigation: &nav})
		}
	}))
	web.subscribe(webAddWebMessageReceived, newHandler(&iidWebMessageReceived, func(_ uintptr, args unsafe.Pointer) {
		if wv.closed || !wv.v.events.Has(driver.EventMessage) {
			return
		}
		// A message that is not a string comes as JSON.
		a := (*comObject)(args)
		var p *uint16
		data := ""
		if failed(a.call(messageTryGetString, uintptr(unsafe.Pointer(&p)))) {
			data = a.getString(messageGetJSON)
		} else {
			data = takeString(p)
		}
		wv.report(driver.Event{Kind: driver.EventMessage, Text: data})
	}))
}

// listenInput follows the page's cursor and keyboard focus.
func (wv *webView) listenInput() {
	wv.comp.subscribe(compAddCursorChanged, newHandler(&iidCursorChanged, func(uintptr, unsafe.Pointer) {
		if wv.closed {
			return
		}
		var cursor uintptr
		if !failed(wv.comp.call(compGetCursor, uintptr(unsafe.Pointer(&cursor)))) {
			wv.cursor = cursor
			if wv.hovered && cursor != 0 {
				// The window asks for the cursor only when the mouse moves:
				// show the page's now.
				setCursor.Call(cursor)
			}
		}
	}))
	wv.controller.subscribe(controllerAddGotFocus, newHandler(&iidFocusChanged, func(uintptr, unsafe.Pointer) {
		wv.focused = true
		// The page has the keyboard: the window's fields give it up.
		if !wv.closed {
			wv.w.blurAll()
		}
	}))
	wv.controller.subscribe(controllerAddLostFocus, newHandler(&iidFocusChanged, func(uintptr, unsafe.Pointer) {
		wv.focused = false
	}))
	wv.controller.subscribe(controllerAddMoveFocusRequested, newHandler(&iidMoveFocusRequested, func(_ uintptr, args unsafe.Pointer) {
		if wv.closed {
			return
		}
		wv.focusWindow()
		(*comObject)(args).call(moveFocusPutHandled, 1)
	}))
}

// navigation returns the page's state.
func (wv *webView) navigation(loading bool) driver.WebNavigation {
	return driver.WebNavigation{
		URL:          wv.web.getString(webGetSource),
		Title:        wv.web.getString(webGetDocumentTitle),
		Loading:      loading,
		CanGoBack:    wv.web.getBool(webGetCanGoBack),
		CanGoForward: wv.web.getBool(webGetCanGoForward),
	}
}

// navigated reports nav as kinds.
func (wv *webView) navigated(nav driver.WebNavigation, kinds ...driver.EventKind) {
	for _, k := range kinds {
		n := nav
		wv.report(driver.Event{Kind: k, Navigation: &n})
	}
}

// webError is what a failed navigation's COREWEBVIEW2_WEB_ERROR_STATUS
// reports, "" for one that was replaced or cancelled.
func webError(status int32) string {
	switch status {
	case 1:
		return "The certificate's common name does not match the host."
	case 2:
		return "The certificate has expired."
	case 3:
		return "The client certificate contains errors."
	case 4:
		return "The certificate has been revoked."
	case 5:
		return "The certificate is invalid."
	case 6:
		return "The server is unreachable."
	case 7:
		return "The connection timed out."
	case 8:
		return "The server returned an invalid response."
	case 9:
		return "The connection was aborted."
	case 10:
		return "The connection was reset."
	case 11:
		return "The internet connection was lost."
	case 12:
		return "Cannot connect to the server."
	case 13:
		return "The host name could not be resolved."
	case 14:
		return ""
	case 15:
		return "A redirect failed."
	case 17:
		return "Authentication is required."
	case 18:
		return "Proxy authentication is required."
	}
	return "The page could not be loaded."
}

// configure takes c, applying it once the controller exists.
func (wv *webView) configure(c *driver.WebConfig) {
	wv.config, wv.configured = *c, true
	if wv.ready {
		wv.paintBackground()
		wv.apply()
	}
	wv.w.redrawAll()
}

// apply sets the configuration, loading the source again when it or the
// injected scripts changed.
func (wv *webView) apply() {
	c, old, first := wv.config, wv.applied, !wv.hasApplied
	wv.applied, wv.hasApplied = c, true
	if s := wv.settings; s != nil {
		s.call(settingsPutIsScriptEnabled, boolArg(!c.NoJavaScript))
		ua := c.UserAgent
		if ua == "" {
			ua = wv.userAgent
		}
		if s2 := s.query(&iidSettings2); s2 != nil {
			s2.call(settingsPutUserAgent, uintptr(unsafe.Pointer(wide(ua))))
			s2.release()
		}
		if s6 := s.query(&iidSettings6); s6 != nil {
			s6.call(settingsPutIsSwipeNavigationEnabled, boolArg(c.BackForwardGestures))
			s6.release()
		}
	}
	scripts := first || c.InjectedJavaScript != old.InjectedJavaScript ||
		c.InjectedJavaScriptBeforeContent != old.InjectedJavaScriptBeforeContent
	if scripts {
		for _, id := range wv.scripts {
			wv.web.call(webRemoveScriptOnDocumentCreated, uintptr(unsafe.Pointer(wide(id))))
		}
		wv.scripts = nil
		wv.scriptGen++
		gen := wv.scriptGen
		keep := func(id string) bool {
			if gen != wv.scriptGen {
				return false
			}
			wv.scripts = append(wv.scripts, id)
			return true
		}
		if js := c.InjectedJavaScriptBeforeContent; js != "" {
			wv.addScript(beforeContent(js), keep)
		}
		if js := c.InjectedJavaScript; js != "" {
			wv.addScript(afterContent(js), keep)
		}
	}
	if first && c.NoScroll || !first && c.NoScroll != old.NoScroll {
		wv.setScrolls(!c.NoScroll)
	}
	if scripts || c.Source != old.Source {
		wv.navigatePending = true
		wv.navigateIfReady()
	}
}

// addScript adds js to run as each document is created; keep receives its
// ID once it is added and reports whether it stays.
func (wv *webView) addScript(js string, keep func(id string) bool) {
	wv.adding++
	var h *comHandler
	h = newHandler(&iidAddScriptCompleted, func(hr uintptr, id unsafe.Pointer) {
		defer h.release()
		if wv.closed {
			return
		}
		wv.adding--
		if !failed(hr) && id != nil {
			if added := wideString((*uint16)(id)); !keep(added) {
				wv.web.call(webRemoveScriptOnDocumentCreated, uintptr(unsafe.Pointer(wide(added))))
			}
		}
		wv.navigateIfReady()
	})
	if failed(wv.web.call(webAddScriptOnDocumentCreated, uintptr(unsafe.Pointer(wide(js))), h.ptr())) {
		h.release()
		wv.adding--
	}
}

// navigateIfReady loads the source asked for once the document scripts
// are added: a script is only sure to run on navigations after it is.
func (wv *webView) navigateIfReady() {
	if wv.adding == 0 && wv.navigatePending {
		wv.navigatePending = false
		wv.navigate()
	}
}

// beforeContent is InjectedJavaScriptBeforeContent's script: in the main
// frame, before the page's own scripts (document scripts run in every
// frame).
func beforeContent(js string) string { return "if (window === window.top) {\n" + js + "\n}" }

// afterContent is InjectedJavaScript's script: in the main frame, once the
// document is parsed.
func afterContent(js string) string {
	return "if (window === window.top) { (function () { var run = function () {\n" + js + "\n}; " +
		"if (document.readyState === 'loading') { document.addEventListener('DOMContentLoaded', run, { once: true }); } " +
		"else { run(); } })(); }"
}

// scrollStyle is the script that lets the page's viewport scroll, or
// keeps it from scrolling with a style on its root element; scripts still
// scroll it.
func scrollStyle(enabled bool) string {
	on := "false"
	if enabled {
		on = "true"
	}
	return "(function () { var on = " + on + "; var id = 'nsgo-no-scroll'; " +
		"function apply() { var s = document.getElementById(id); " +
		"if (on) { if (s) s.remove(); return; } " +
		"if (!s && document.documentElement) { s = document.createElement('style'); s.id = id; " +
		"s.textContent = 'html { overflow: hidden !important; }'; document.documentElement.appendChild(s); } } " +
		"if (document.documentElement) apply(); else document.addEventListener('readystatechange', apply, { once: true }); })();"
}

// setScrolls lets the page's viewport scroll, or not: the style goes in
// the page now and in every page loaded later.
func (wv *webView) setScrolls(enabled bool) {
	if wv.scrollScript != "" {
		wv.web.call(webRemoveScriptOnDocumentCreated, uintptr(unsafe.Pointer(wide(wv.scrollScript))))
		wv.scrollScript = ""
	}
	wv.scrollGen++
	if !enabled {
		gen := wv.scrollGen
		wv.addScript(scrollStyle(false), func(id string) bool {
			if gen != wv.scrollGen {
				return false
			}
			wv.scrollScript = id
			return true
		})
	}
	wv.execute(scrollStyle(enabled))
}

func (wv *webView) navigate() {
	src := wv.config.Source
	wv.html, wv.base = "", ""
	var hr uintptr
	switch {
	case src.HTML != "" && src.BaseURL != "":
		wv.html, wv.base = src.HTML, src.BaseURL
		wv.serveHTML()
		hr = wv.web.call(webNavigate, uintptr(unsafe.Pointer(wide(src.BaseURL))))
	case src.HTML != "":
		hr = wv.web.call(webNavigateToString, uintptr(unsafe.Pointer(wide(src.HTML))))
	case src.Src != "":
		hr = wv.web.call(webNavigate, uintptr(unsafe.Pointer(wide(webURL(src.Src)))))
	default:
		hr = wv.web.call(webNavigate, uintptr(unsafe.Pointer(wide("about:blank"))))
	}
	if failed(hr) {
		url := src.BaseURL
		if src.HTML == "" {
			url = src.Src
		}
		wv.report(driver.Event{Kind: driver.EventError, Text: hresultError("the page could not be loaded", hr).Error(),
			Navigation: &driver.WebNavigation{URL: url}})
	}
}

// webURL is s as WebView2 navigates to it: an absolute path as a file
// URL.
func webURL(s string) string {
	if filepath.IsAbs(s) {
		return "file:///" + filepath.ToSlash(s)
	}
	return s
}

// serveHTML answers the request of the document at the base URL with the
// app's HTML.
func (wv *webView) serveHTML() {
	if wv.serving {
		return
	}
	wv.serving = true
	wv.web.call(webAddWebResourceRequestedFilter, uintptr(unsafe.Pointer(wide("*"))), webResourceContextDocument)
	wv.web.subscribe(webAddWebResourceRequested, newHandler(&iidWebResourceRequested, func(_ uintptr, args unsafe.Pointer) {
		if wv.closed || wv.html == "" || wv.env == nil {
			return
		}
		a := (*comObject)(args)
		var request *comObject
		if failed(a.call(resourceArgsGetRequest, uintptr(unsafe.Pointer(&request)))) || request == nil {
			return
		}
		uri := request.getString(requestGetUri)
		request.release()
		if !sameDocument(uri, wv.base) {
			return
		}
		body := []byte(wv.html)
		stream, _, _ := shCreateMemStream.Call(uintptr(unsafe.Pointer(&body[0])), uintptr(len(body)))
		if stream == 0 {
			return
		}
		s := *(**comObject)(unsafe.Pointer(&stream))
		defer s.release()
		var response *comObject
		if failed(wv.env.call(envCreateWebResourceResponse, s.ptr(), 200, uintptr(unsafe.Pointer(wide("OK"))),
			uintptr(unsafe.Pointer(wide("Content-Type: text/html; charset=utf-8"))), uintptr(unsafe.Pointer(&response)))) {
			return
		}
		a.call(resourceArgsPutResponse, response.ptr())
		response.release()
	}))
}

// sameDocument reports whether a and b name the same document, fragments
// aside; a URL with no path is its root's.
func sameDocument(a, b string) bool {
	normal := func(url string) string {
		url, _, _ = strings.Cut(url, "#")
		scheme, rest, ok := strings.Cut(url, "://")
		if !ok {
			return url
		}
		host, path := rest, "/"
		if i := strings.IndexByte(rest, '/'); i >= 0 {
			host, path = rest[:i], rest[i:]
		}
		return strings.ToLower(scheme) + "://" + strings.ToLower(host) + path
	}
	return normal(a) == normal(b)
}

// command does c, or keeps it until the controller exists.
func (wv *webView) command(c driver.WebCommand, value string) {
	if wv.failed || wv.closed {
		return
	}
	if !wv.ready {
		wv.commands = append(wv.commands, webCommand{c, value})
		return
	}
	switch c {
	case driver.WebReload:
		wv.web.call(webReload)
	case driver.WebGoBack:
		wv.web.call(webGoBack)
	case driver.WebGoForward:
		wv.web.call(webGoForward)
	case driver.WebStopLoading:
		wv.web.call(webStop)
	case driver.WebPostMessage:
		wv.execute(driver.WebMessageScript(value))
	case driver.WebInjectJavaScript:
		wv.execute(value)
	}
}

func (wv *webView) execute(script string) {
	wv.web.call(webExecuteScript, uintptr(unsafe.Pointer(wide(script))), wv.executed.ptr())
}

// restyle follows the view's new style.
func (wv *webView) restyle() {
	wv.paintBackground()
}

// paintBackground puts the view's background behind the page until it
// paints, when the view has an opaque one.
func (wv *webView) paintBackground() {
	bg := wv.v.style.Background
	if wv.controller == nil || bg&0xff != 0xff {
		return
	}
	if c2 := wv.controller.query(&iidController2); c2 != nil {
		// COREWEBVIEW2_COLOR is A, R, G, B bytes, passed by value.
		color := uintptr(0xff) | uintptr(bg>>24&0xff)<<8 | uintptr(bg>>16&0xff)<<16 | uintptr(bg>>8&0xff)<<24
		c2.call(controllerPutDefaultBackgroundColor, color)
		c2.release()
	}
}

// composeAt takes where the last frame put the page, nil when it did not
// draw it.
func (wv *webView) composeAt(at *placement) {
	if wv.closed {
		return
	}
	if at == nil {
		wv.at = nil
	} else {
		pl := *at
		wv.at = &pl
	}
	if wv.portal != nil {
		if at != nil {
			wv.portal.place(*at)
		} else {
			wv.portal.hidden()
		}
	}
	wv.place()
}

// place gives the controller the bounds, visibility and scale of where
// the last frame put the page.
func (wv *webView) place() {
	if wv.controller == nil {
		return
	}
	shown := wv.at != nil
	var b rect
	if shown {
		b = wv.at.bounds()
	}
	if shown != wv.shown || shown && b != wv.bounds {
		before := wv.shown
		wv.shown, wv.bounds = shown, b
		if shown {
			if scale := float32(wv.w.scale); wv.raster != scale {
				if c3 := wv.controller.query(&iidController3); c3 != nil {
					c3.callDouble(controllerPutRasterizationScale, float64(scale))
					c3.release()
				}
				wv.raster = scale
			}
			wv.controller.callRect(controllerPutBounds, b)
			if !before {
				wv.controller.call(controllerPutIsVisible, 1)
			} else {
				// Pop-ups (select menus, tooltips) follow the page.
				wv.controller.call(controllerNotifyParentPositionChanged)
			}
		} else {
			wv.controller.call(controllerPutIsVisible, 0)
		}
	}
	if !shown && wv.focused {
		// A hidden page keeps no keyboard focus.
		wv.focusWindow()
	}
}

// moved tells WebView2 the window moved, which it places pop-ups by.
func (wv *webView) moved() {
	if wv.controller != nil {
		wv.controller.call(controllerNotifyParentPositionChanged)
	}
}

// takeFocus gives the page the keyboard focus.
func (wv *webView) takeFocus() {
	if wv.controller != nil {
		const programmatic = 0
		wv.controller.call(controllerMoveFocus, programmatic)
	}
}

// focusWindow gives the keyboard back to the window.
func (wv *webView) focusWindow() {
	hwnd := wv.w.hwnd
	focus, _, _ := getFocus.Call()
	if child, _, _ := isChild.Call(hwnd, focus); focus != hwnd && (focus == 0 || child != 0) {
		setFocus.Call(hwnd)
	}
	wv.focused = false
}

// close lets the page go, with its visuals.
func (wv *webView) close() {
	if wv.closed {
		return
	}
	if wv.focused {
		wv.focusWindow()
	}
	wv.closed = true
	wv.commands = nil
	if wv.controller != nil {
		wv.controller.call(controllerClose)
	}
	releaseAll(&wv.settings, &wv.web, &wv.controller, &wv.comp, &wv.env)
	if wv.executed != nil {
		wv.executed.release()
		wv.executed = nil
	}
	if wv.portal != nil {
		wv.portal.release()
		wv.portal = nil
	}
	delete(wv.w.webs, wv)
	wv.w.forget(wv)
	wv.w.redrawAll()
}

// The mouse event kinds of SendMouseInput.
const (
	mouseMove                  = 512
	mouseLeftDown              = 513
	mouseLeftUp                = 514
	mouseLeftDoubleClick       = 515
	mouseRightDown             = 516
	mouseRightUp               = 517
	mouseRightDoubleClick      = 518
	mouseMiddleDown            = 519
	mouseMiddleUp              = 520
	mouseMiddleDoubleClick     = 521
	mouseWheel                 = 522
	mouseXDown                 = 523
	mouseXUp                   = 524
	mouseXDoubleClick          = 525
	mouseHorizontalWheel       = 526
	mouseLeave                 = 675
	webResourceContextDocument = 1
)

// buttonEvent returns the mouse event of button (0 left, 1 right, 2
// middle, 3 back, 4 forward) going down, with its click count, or up, and
// its mouse data.
func buttonEvent(button int, down bool, clicks int) (kind, data uintptr) {
	// Windows sends a double click as the second press of a pair.
	double := down && clicks >= 2 && clicks%2 == 0
	kinds := [4][3]uintptr{
		{mouseLeftDown, mouseLeftDoubleClick, mouseLeftUp},
		{mouseRightDown, mouseRightDoubleClick, mouseRightUp},
		{mouseMiddleDown, mouseMiddleDoubleClick, mouseMiddleUp},
		{mouseXDown, mouseXDoubleClick, mouseXUp},
	}[min(button, 3)]
	switch {
	case !down:
		kind = kinds[2]
	case double:
		kind = kinds[1]
	default:
		kind = kinds[0]
	}
	switch button {
	case 3:
		data = 1 // XBUTTON1
	case 4:
		data = 2 // XBUTTON2
	}
	return kind, data
}

// mouse forwards a button going down or up, or a move (button < 0), at
// x, y in the view's pixels.
func (wv *webView) mouse(button int, down bool, x, y int32, time uint32) {
	if button < 0 {
		wv.hovered = true
		wv.sendMouse(mouseMove, 0, x, y)
		return
	}
	bit := uint8(1) << button
	if !down {
		kind, data := buttonEvent(button, false, 1)
		wv.buttons &^= bit
		wv.sendMouse(kind, data, x, y)
		return
	}
	// Count clicks as Windows would for a window with CS_DBLCLKS.
	dt, _, _ := getDoubleClickTime.Call()
	cx, _, _ := getSystemMetrics.Call(smCXDoubleClk)
	cy, _, _ := getSystemMetrics.Call(smCYDoubleClk)
	if button == wv.lastButton && time-wv.lastDown <= uint32(dt) &&
		abs32(x-wv.lastDownAt.X) <= int32(cx)/2 && abs32(y-wv.lastDownAt.Y) <= int32(cy)/2 {
		wv.downCount++
	} else {
		wv.downCount = 1
	}
	wv.lastButton, wv.lastDown, wv.lastDownAt = button, time, point{x, y}
	kind, data := buttonEvent(button, true, wv.downCount)
	wv.buttons |= bit
	wv.sendMouse(kind, data, x, y)
	if !wv.focused {
		wv.takeFocus()
	}
}

// leave tells the page the mouse left it.
func (wv *webView) leave() {
	wv.hovered = false
	wv.buttons = 0
	wv.sendMouse(mouseLeave, 0, 0, 0)
}

// wheel forwards a wheel's delta (WM_MOUSEWHEEL's, or WM_MOUSEHWHEEL's
// when horizontal) at x, y in the view's pixels, and reports whether the
// page took it: one that does not scroll leaves it to the scroll views
// around it.
func (wv *webView) wheel(delta int16, horizontal bool, x, y int32) bool {
	if wv.applied.NoScroll {
		return false
	}
	kind := uintptr(mouseWheel)
	if horizontal {
		kind = mouseHorizontalWheel
	}
	wv.sendMouse(kind, uintptr(uint32(int32(delta))), x, y)
	return true
}

func (wv *webView) sendMouse(kind, data uintptr, x, y int32) {
	if wv.comp == nil {
		return
	}
	// MK_LBUTTON, MK_RBUTTON, MK_MBUTTON, MK_XBUTTON1 and MK_XBUTTON2 by
	// button; MK_SHIFT and MK_CONTROL.
	buttonKeys := [5]uintptr{0x1, 0x2, 0x10, 0x20, 0x40}
	var keys uintptr
	for b, k := range buttonKeys {
		if wv.buttons&(1<<b) != 0 {
			keys |= k
		}
	}
	if keyDown(vkShift) {
		keys |= 0x4
	}
	if keyDown(vkControl) {
		keys |= 0x8
	}
	args := append([]uintptr{kind, keys, data}, pointArgs(x, y)...)
	wv.comp.call(compSendMouseInput, args...)
}

// pointer forwards a touch or pen contact through SendPointerInput, with
// the system's data of the pointer moved to x, y in the view's pixels. It
// reports false when the system has no data for it.
func (wv *webView) pointer(message uint32, id uint32, touch bool, x, y int32) bool {
	if wv.comp == nil || wv.env == nil {
		return false
	}
	env3 := wv.env.query(&iidEnvironment3)
	if env3 == nil {
		return false
	}
	defer env3.release()
	var kind uintptr
	switch message {
	case wmPointerDown:
		kind = pointerEventDown
	case wmPointerUpdate:
		kind = pointerEventUpdate
	case wmPointerUp:
		kind = pointerEventUp
	default:
		kind = pointerEventLeave
	}
	var info pointerTouchInfo
	if r, _, _ := getPointerInfo.Call(uintptr(id), uintptr(unsafe.Pointer(&info.pointerInfo))); r == 0 {
		return false
	}
	var p *comObject
	if failed(env3.call(envCreatePointerInfo, uintptr(unsafe.Pointer(&p)))) || p == nil {
		return false
	}
	defer p.release()
	pi := &info.pointerInfo
	client := pi.pixelLocation
	screenToClient.Call(wv.w.hwnd, uintptr(unsafe.Pointer(&client)))
	// From where the contact is in the window to where it is on the page.
	dx, dy := x-client.X, y-client.Y
	var device, display rect
	getPointerDeviceRects.Call(pi.sourceDevice, uintptr(unsafe.Pointer(&device)), uintptr(unsafe.Pointer(&display)))
	// Himetric units per pixel.
	ratio := func(d, s int32) float32 {
		if s == 0 {
			return 1
		}
		return float32(d) / float32(s)
	}
	hx := ratio(device.Right-device.Left, display.Right-display.Left)
	hy := ratio(device.Bottom-device.Top, display.Bottom-display.Top)
	moved := func(q point) []uintptr { return pointArgs(q.X+dx, q.Y+dy) }
	himetric := func(q point) []uintptr {
		return pointArgs(q.X+int32(float32(dx)*hx), q.Y+int32(float32(dy)*hy))
	}
	set := func(method int, args ...uintptr) { p.call(method, args...) }
	set(pointerPutPointerKind, uintptr(pi.pointerType))
	set(pointerPutPointerId, uintptr(pi.pointerID))
	set(pointerPutFrameId, uintptr(pi.frameID))
	set(pointerPutPointerFlags, uintptr(pi.pointerFlags))
	p.callRect(pointerPutPointerDeviceRect, device)
	p.callRect(pointerPutDisplayRect, display)
	set(pointerPutPixelLocation, pointArgs(x, y)...)
	set(pointerPutHimetricLocation, himetric(pi.himetricLocation)...)
	set(pointerPutPixelLocationRaw, moved(pi.pixelLocationRaw)...)
	set(pointerPutHimetricLocationRaw, himetric(pi.himetricLocationRaw)...)
	set(pointerPutTime, uintptr(pi.time))
	set(pointerPutHistoryCount, uintptr(pi.historyCount))
	set(pointerPutInputData, uintptr(pi.inputData))
	set(pointerPutKeyStates, uintptr(pi.keyStates))
	set(pointerPutPerformanceCount, u64Args(pi.performanceCount)...)
	set(pointerPutButtonChangeKind, uintptr(pi.buttonChangeType))
	if touch {
		if r, _, _ := getPointerTouchInfo.Call(uintptr(id), uintptr(unsafe.Pointer(&info))); r != 0 {
			shift := func(r rect) rect { return rect{r.Left + dx, r.Top + dy, r.Right + dx, r.Bottom + dy} }
			set(pointerPutTouchFlags, uintptr(info.touchFlags))
			set(pointerPutTouchMask, uintptr(info.touchMask))
			p.callRect(pointerPutTouchContact, shift(info.contact))
			p.callRect(pointerPutTouchContactRaw, shift(info.contactRaw))
			set(pointerPutTouchOrientation, uintptr(info.orientation))
			set(pointerPutTouchPressure, uintptr(info.pressure))
		}
	} else {
		var pen pointerPenInfo
		if r, _, _ := getPointerPenInfo.Call(uintptr(id), uintptr(unsafe.Pointer(&pen))); r != 0 {
			set(pointerPutPenFlags, uintptr(pen.penFlags))
			set(pointerPutPenMask, uintptr(pen.penMask))
			set(pointerPutPenPressure, uintptr(pen.pressure))
			set(pointerPutPenRotation, uintptr(pen.rotation))
			set(pointerPutPenTiltX, uintptr(pen.tiltX))
			set(pointerPutPenTiltY, uintptr(pen.tiltY))
		}
	}
	if failed(wv.comp.call(compSendPointerInput, kind, p.ptr())) {
		return false
	}
	if message == wmPointerDown && !wv.focused {
		wv.takeFocus()
	}
	return true
}

func abs32(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}

// drawWebPlaceholder draws what a web view shows without the WebView2
// runtime, in a box of width by height points.
func (p *painter) drawWebPlaceholder(width, height float32) {
	format := placeholderFormat()
	if format == nil {
		return
	}
	text := utf16.Encode([]rune("WebView is not available on this system"))
	r := rectF{8, 0, 8 + max(width-16, 0), height}
	p.dc.call(rtDrawText, uintptr(unsafe.Pointer(&text[0])), uintptr(len(text)), format.ptr(),
		uintptr(unsafe.Pointer(&r)), p.solid(driver.RGB(0x8c8c8c)).ptr(), 0, 0)
}

// placeholderFormat is the placeholder's text format, made once.
func placeholderFormat() *comObject {
	if webPlaceholderFormat == nil {
		f := textFormat(driver.Font{Size: 14, Weight: driver.FontWeightNormal})
		if f == nil {
			return nil
		}
		f.call(formatSetTextAlignment, dwAlignCenter)
		f.call(formatSetParagraphAlignment, dwParagraphCenter)
		f.call(formatSetWordWrapping, dwWrap)
		webPlaceholderFormat = f
	}
	return webPlaceholderFormat
}

var webPlaceholderFormat *comObject
