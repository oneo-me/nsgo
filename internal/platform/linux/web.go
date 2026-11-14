//go:build linux && !android

package linux

import (
	"fmt"
	"log"
	"maps"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

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
	webkitWebViewGetContext              func(uintptr) uintptr
	webkitWebContextRegisterURIScheme    func(uintptr, *byte, uintptr, uintptr, uintptr)
	webkitURISchemeRequestGetURI         func(uintptr) uintptr
	webkitURISchemeRequestGetWebView     func(uintptr) uintptr
	webkitURISchemeRequestFinish         func(uintptr, uintptr, int64, *byte)
	webkitURISchemeRequestFinishError    func(uintptr, uintptr)
	gErrorNewLiteral                     func(uint32, int32, *byte) uintptr
	gMemoryInputStreamNewFromBytes       func(uintptr) uintptr
	webkitWebViewGetBackgroundColor      func(uintptr, *[4]float32)
	webkitNetworkSessionNew              func(*byte, *byte) uintptr
	webkitNetworkSessionGetCookieManager func(uintptr) uintptr
	webkitDownloadGetWebView             func(uintptr) uintptr
	webkitDownloadGetRequest             func(uintptr) uintptr
	webkitDownloadSetDestination         func(uintptr, *byte)
	webkitDownloadGetDestination         func(uintptr) uintptr
	webkitDownloadGetReceivedDataLength  func(uintptr) uint64
	webkitDownloadCancel                 func(uintptr)
	webkitCookieManagerSetPersistent     func(uintptr, *byte, int32)
	webkitWebViewGetUserContentManager   func(uintptr) uintptr
	webkitWebViewGetSettings             func(uintptr) uintptr
	webkitWebViewSetBackgroundColor      func(uintptr, *[4]float32)
	webkitWebViewLoadURI                 func(uintptr, *byte)
	webkitWebViewLoadRequest             func(uintptr, uintptr)
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
	webkitURIRequestNew                  func(*byte) uintptr
	webkitURIRequestGetHTTPHeaders       func(uintptr) uintptr
	webkitURIRequestGetURI               func(uintptr) uintptr
	webkitPolicyDecisionIgnore           func(uintptr)
	webkitMessageHeadersAppend           func(uintptr, *byte, *byte)
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
		soup, err := purego.Dlopen("libsoup-3.0.so.0", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			webkitErr = fmt.Errorf("web views need libsoup 3 (libsoup-3.0.so.0): %w", err)
			return
		}
		defer func() {
			if r := recover(); r != nil {
				webkitErr = fmt.Errorf("web views need WebKitGTK 6.0: %v", r)
			}
		}()
		bindWebKit(webkit, jsc, soup)
	})
	return webkitErr
}

func bindWebKit(webkit, jsc, soup uintptr) {
	reg := func(fn any, name string) { purego.RegisterLibFunc(fn, webkit, name) }
	reg(&webkitWebViewGetType, "webkit_web_view_get_type")
	reg(&webkitWebViewGetContext, "webkit_web_view_get_context")
	reg(&webkitWebContextRegisterURIScheme, "webkit_web_context_register_uri_scheme")
	reg(&webkitURISchemeRequestGetURI, "webkit_uri_scheme_request_get_uri")
	reg(&webkitURISchemeRequestGetWebView, "webkit_uri_scheme_request_get_web_view")
	reg(&webkitURISchemeRequestFinish, "webkit_uri_scheme_request_finish")
	reg(&webkitURISchemeRequestFinishError, "webkit_uri_scheme_request_finish_error")
	purego.RegisterLibFunc(&gErrorNewLiteral, glibLib, "g_error_new_literal")
	purego.RegisterLibFunc(&gMemoryInputStreamNewFromBytes, gioLib, "g_memory_input_stream_new_from_bytes")
	reg(&webkitWebViewGetBackgroundColor, "webkit_web_view_get_background_color")
	reg(&webkitNetworkSessionNew, "webkit_network_session_new")
	reg(&webkitNetworkSessionGetCookieManager, "webkit_network_session_get_cookie_manager")
	reg(&webkitDownloadGetWebView, "webkit_download_get_web_view")
	reg(&webkitDownloadGetRequest, "webkit_download_get_request")
	reg(&webkitDownloadSetDestination, "webkit_download_set_destination")
	reg(&webkitDownloadGetDestination, "webkit_download_get_destination")
	reg(&webkitDownloadGetReceivedDataLength, "webkit_download_get_received_data_length")
	reg(&webkitDownloadCancel, "webkit_download_cancel")
	reg(&webkitCookieManagerSetPersistent, "webkit_cookie_manager_set_persistent_storage")
	reg(&webkitWebViewGetUserContentManager, "webkit_web_view_get_user_content_manager")
	reg(&webkitWebViewGetSettings, "webkit_web_view_get_settings")
	reg(&webkitWebViewSetBackgroundColor, "webkit_web_view_set_background_color")
	reg(&webkitWebViewLoadURI, "webkit_web_view_load_uri")
	reg(&webkitWebViewLoadRequest, "webkit_web_view_load_request")
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
	reg(&webkitURIRequestNew, "webkit_uri_request_new")
	reg(&webkitURIRequestGetHTTPHeaders, "webkit_uri_request_get_http_headers")
	reg(&webkitURIRequestGetURI, "webkit_uri_request_get_uri")
	reg(&webkitPolicyDecisionIgnore, "webkit_policy_decision_ignore")
	purego.RegisterLibFunc(&webkitMessageHeadersAppend, soup, "soup_message_headers_append")
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
	webkitNetworkUnknown   = 301 // WEBKIT_NETWORK_ERROR_UNKNOWN_PROTOCOL
	webkitNetworkFailed    = 399 // WEBKIT_NETWORK_ERROR_FAILED
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
	session    uintptr // the ephemeral network session an Incognito page owns, released with it; 0 for the app's
	content    uintptr // its WebKitUserContentManager
	config     driver.WebConfig
	made       bool
	failed     bool         // the navigation failed; WebKit still finishes it
	pending    bool         // a load waits for the page to be mapped
	background [4]float32   // WebKit's own, for views without an opaque one
	pointer    driver.Point // where the pointer is in the page
}

// webPage is the window and view a WebKitWebView belongs to.
type webPage struct {
	window *window
	view   *view
}

// webPages are the live pages by WebKitWebView, whatever their window and network session, so that what WebKit hands over with only a page (scheme requests, downloads) finds its view.
var webPages = map[uintptr]webPage{}

// webPageOf returns the live view showing page, or nils.
func webPageOf(page uintptr) (*window, *view) {
	p, ok := webPages[page]
	if !ok || p.window.closing || p.view.deleted || p.view.web == nil || p.view.web.widget != page {
		return nil, nil
	}
	return p.window, p.view
}

// webSchemes are the schemes registered on each WebKitWebContext. A context keeps a scheme for its life, so each is registered once, and its requests find their view by page (webSchemeCallback).
var webSchemes = map[uintptr]map[string]bool{}

// webDownload is a download a view saves.
type webDownload struct {
	window *window
	view   *view
	item   driver.WebDownload
}

var (
	webDownloads      = map[uintptr]*webDownload{}
	webDownloadSerial uint64
)

// report reports dl as it is now. The engine reads the event after the backend has gone on, so it gets a copy rather than the item later signals change.
func (dl *webDownload) report() {
	item := dl.item
	dl.window.report(dl.view, driver.Event{Kind: driver.EventDownload, ID: dl.view.id, Data: &item})
}

// webWarned is the warning, once, that web views show a placeholder.
var webWarned sync.Once

// configureWeb applies a web view's configuration, making its page on
// the first one and loading the source again when it or the scripts
// changed; the rest applies to the page as it is.
func (w *window) configureWeb(v *view, c *driver.WebConfig) {
	if v.web == nil {
		v.web = &web{config: *c}
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
	if first || scripts || c.Source != old.Source || !maps.Equal(c.Headers, old.Headers) {
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

// makeWeb makes v's page in the app's network session, or an ephemeral one of its own for Incognito, and hosts it.
func (w *window) makeWeb(v *view) {
	wv := v.web
	var session uintptr
	if wv.config.Incognito {
		// A session without data or cache directories is ephemeral. The page holds its own reference; wv.session is released when the view closes.
		wv.session = webkitNetworkSessionNew(nil, nil)
		gSignalConnect(wv.session, "download-started", cbWebDownloadStarted, 0)
		session = wv.session
	} else {
		session = w.webSession()
	}
	wv.widget = gObjectNew(webkitWebViewGetType(), cstr("network-session"), session, uintptr(0))
	wv.content = webkitWebViewGetUserContentManager(wv.widget)
	webPages[wv.widget] = webPage{window: w, view: v}
	registerWebSchemes(wv)
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

// registerWebSchemes registers the schemes wv's app serves on its page's WebKitWebContext, which every page of the process shares, so a scheme is registered once for every view that serves it.
func registerWebSchemes(wv *web) {
	if len(wv.config.SchemeHandlers) == 0 {
		return
	}
	context := webkitWebViewGetContext(wv.widget)
	if context == 0 {
		return
	}
	registered := webSchemes[context]
	if registered == nil {
		registered = map[string]bool{}
		webSchemes[context] = registered
	}
	for name := range wv.config.SchemeHandlers {
		if !registered[name] {
			webkitWebContextRegisterURIScheme(context, cstr(name), cbWebScheme, 0, 0)
			registered[name] = true
		}
	}
}

// webSessions are the network sessions by app ID: one network process, and one set of cookies and storage, serves every web view of an app that is not Incognito. A session lives as long as the process, so views mounted again find it.
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
	gSignalConnect(s, "download-started", cbWebDownloadStarted, 0)
	// Cookies outlive the app, as other website data does.
	if cookies := webkitNetworkSessionGetCookieManager(s); cookies != 0 {
		webkitCookieManagerSetPersistent(cookies, cstr(filepath.Join(data, "cookies.sqlite")), webkitCookieSQLite)
	}
	webSessions[w.appID] = s
	return s
}

// webSchemeCallback answers a request for an app-owned scheme from the handler of the view whose page made it. Every request is finished, with an error when no live view serves its scheme, so no page waits on one.
func webSchemeCallback(request, _ uintptr) {
	_, v := webPageOf(webkitURISchemeRequestGetWebView(request))
	raw := goString(webkitURISchemeRequestGetURI(request))
	u, err := url.Parse(raw)
	if err != nil {
		finishWebSchemeError(request, webkitNetworkFailed, err.Error())
		return
	}
	var handler func(string) []byte
	if v != nil {
		handler = v.web.config.SchemeHandlers[strings.ToLower(u.Scheme)]
	}
	if handler == nil {
		finishWebSchemeError(request, webkitNetworkUnknown, "nsgo: no handler serves "+u.Scheme+": URLs in this web view")
		return
	}
	body := handler(raw)
	bytes := gBytesNew(unsafe.Pointer(nil), 0)
	if len(body) > 0 {
		bytes = gBytesNew(unsafe.Pointer(&body[0]), uintptr(len(body)))
	}
	stream := gMemoryInputStreamNewFromBytes(bytes)
	gBytesUnref(bytes)
	contentType := driver.WebMIME(u.Path, body)
	webkitURISchemeRequestFinish(request, stream, int64(len(body)), cstr(contentType))
	gObjectUnref(stream)
}

// finishWebSchemeError ends request with a WebKit network error of code and message.
func finishWebSchemeError(request uintptr, code int32, message string) {
	err := gErrorNewLiteral(webkitNetworkErrorQuark(), code, cstr(message))
	webkitURISchemeRequestFinishError(request, err)
	gErrorFree(err)
}

// webDownloadStartedCallback takes a download a network session started for the view whose page asked for it, when the view saves downloads, and cancels it otherwise.
func webDownloadStartedCallback(_, download, _ uintptr) {
	w, v := webPageOf(webkitDownloadGetWebView(download))
	if v == nil || v.web.config.DownloadDir == "" {
		webkitDownloadCancel(download)
		return
	}
	webDownloadSerial++
	id := strconv.FormatInt(time.Now().UnixMilli(), 36) + "-" + strconv.FormatUint(webDownloadSerial, 36)
	urlText := ""
	if request := webkitDownloadGetRequest(download); request != 0 {
		urlText = goString(webkitURIRequestGetURI(request))
	}
	dl := &webDownload{window: w, view: v, item: driver.WebDownload{State: driver.WebDownloadStarted, ID: id, URL: urlText}}
	webDownloads[download] = dl
	// Downloads own these temporary signal connections; attaching them to the view's hosted-signal list would leave stale object pointers after WebKit releases a completed download.
	gSignalConnect(download, "decide-destination", cbWebDownloadDestination, 0)
	gSignalConnect(download, "received-data", cbWebDownloadReceived, 0)
	gSignalConnect(download, "finished", cbWebDownloadFinished, 0)
	gSignalConnect(download, "failed", cbWebDownloadFailed, 0)
}

func webDownloadDestinationCallback(download, suggested, _ uintptr) int32 {
	dl := webDownloads[download]
	if dl == nil {
		return 0
	}
	name := webDownloadFilename(goString(suggested))
	dir := filepath.Join(dl.view.web.config.DownloadDir, dl.item.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		dl.item.State, dl.item.Message = driver.WebDownloadFailed, err.Error()
		dl.report()
		delete(webDownloads, download)
		webkitDownloadCancel(download)
		return 1
	}
	dl.item.Filename = name
	dl.item.Path = filepath.Join(dir, name)
	webkitDownloadSetDestination(download, cstr(dl.item.Path))
	dl.report()
	return 1
}

func webDownloadFilename(suggested string) string {
	name := filepath.Base(suggested)
	if name == "." || name == ".." || name == string(filepath.Separator) || name == "" {
		return "download"
	}
	return name
}

func webDownloadReceivedCallback(download, _ uint64, _ uintptr) {
	// WebKit reports progress through received-data; the public event model
	// reports only started, finished and failed, as the Apple backend does.
}

func webDownloadFinishedCallback(download, _ uintptr) {
	dl := webDownloads[download]
	if dl == nil {
		return
	}
	dl.item.State = driver.WebDownloadFinished
	dl.item.Bytes = int64(webkitDownloadGetReceivedDataLength(download))
	if dest := webkitDownloadGetDestination(download); dest != 0 {
		dl.item.Path = goString(dest)
	}
	dl.report()
	delete(webDownloads, download)
}

func webDownloadFailedCallback(download, gerr, _ uintptr) {
	dl := webDownloads[download]
	if dl == nil {
		return
	}
	dl.item.State = driver.WebDownloadFailed
	if gerr != 0 {
		dl.item.Message = goString((*gError)(cPointer(gerr)).message)
	}
	dl.report()
	delete(webDownloads, download)
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
		if len(wv.config.Headers) == 0 {
			webkitWebViewLoadURI(wv.widget, cstr(src.Src))
			return
		}
		request := webkitURIRequestNew(cstr(src.Src))
		if request == 0 {
			webkitWebViewLoadURI(wv.widget, cstr(src.Src))
			return
		}
		headers := webkitURIRequestGetHTTPHeaders(request)
		if headers != 0 {
			for name, value := range wv.config.Headers {
				webkitMessageHeadersAppend(headers, cstr(name), cstr(value))
			}
		}
		webkitWebViewLoadRequest(wv.widget, request)
		gObjectUnref(request)
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
		delete(webPages, wv.widget)
		webkitWebViewStopLoading(wv.widget)
		// Before unhosting, which lets the page and its content manager go.
		webkitContentUnregisterHandler(wv.content, cstr(webHandler), nil)
	}
	w.unhost(v, &wv.hosted)
	if wv.session != 0 {
		// The Incognito page's own session, whose website data goes with its last reference.
		gObjectUnref(wv.session)
	}
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
