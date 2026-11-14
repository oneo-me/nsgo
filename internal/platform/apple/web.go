//go:build darwin

package apple

import (
	"log"
	"maps"
	neturl "net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// Web is a web view's WKWebView, filling the view that holds it. WebKit
// loads on the first web view, so apps without one do not pay for it.
// The WKWebView is made on the first Configure.
type Web struct {
	// Dispatch reports the web view's events.
	Dispatch func(driver.Event)

	holder  objc.ID
	web     objc.ID
	handler objc.ID
	config  driver.WebConfig
	fileDir string
	made    bool
}

var (
	classWebHandler objc.Class
	webHandlers     = map[objc.ID]*Web{}
	schemeHandlers  = map[objc.ID]*Web{}
)

// messageHandler is the name pages post to, as window.webkit.messageHandlers.nsgo.
const messageHandler = "nsgo"

var bridge = driver.WebBridge("function (d) { window.webkit.messageHandlers." + messageHandler + ".postMessage(d); }")

// noScroll stops the page scrolling where WebKit has no scroll view to
// turn off (macOS).
const noScroll = "document.documentElement.style.setProperty('overflow', 'hidden', 'important');" +
	"document.body && document.body.style.setProperty('overflow', 'hidden', 'important');"

func loadWebKit() {
	if classWebHandler != 0 {
		return
	}
	if _, err := purego.Dlopen("/System/Library/Frameworks/WebKit.framework/WebKit", purego.RTLD_NOW|purego.RTLD_GLOBAL); err != nil {
		panic(err)
	}
	nav := func(name string, loading bool, kinds ...driver.EventKind) objc.MethodDef {
		return objc.MethodDef{Cmd: sel(name), Fn: func(self objc.ID, _ objc.SEL, web, _ objc.ID) {
			if w := webHandlers[self]; w != nil {
				w.navigated(loading, kinds...)
			}
		}}
	}
	failed := func(self objc.ID, _ objc.SEL, _, _, err objc.ID) {
		if w := webHandlers[self]; w != nil {
			info := err.Send(sel("userInfo"))
			// NSURLErrorFailingURLStringErrorKey, else NSURLErrorFailingURLErrorKey.
			url := GoString(info.Send(sel("objectForKey:"), NSString("NSErrorFailingURLStringKey")))
			if url == "" {
				url = GoString(info.Send(sel("objectForKey:"), NSString("NSErrorFailingURLKey")).Send(sel("absoluteString")))
			}
			w.Dispatch(driver.Event{Kind: driver.EventError, Text: GoString(err.Send(sel("localizedDescription"))),
				Navigation: &driver.WebNavigation{URL: url}})
		}
	}
	var protocols []*objc.Protocol
	for _, name := range []string{"WKNavigationDelegate", "WKScriptMessageHandler", "WKDownloadDelegate", "WKURLSchemeHandler"} {
		if p := objc.GetProtocol(name); p != nil {
			protocols = append(protocols, p)
		}
	}
	classWebHandler = RegisterClass("NsgoWebHandler", Class("NSObject"), protocols,
		[]objc.MethodDef{
			nav("webView:didStartProvisionalNavigation:", true, driver.EventLoadStart, driver.EventNavigationStateChange),
			nav("webView:didFinishNavigation:", false, driver.EventLoad, driver.EventNavigationStateChange),
			{Cmd: sel("webView:didFailNavigation:withError:"), Fn: failed},
			{Cmd: sel("webView:didFailProvisionalNavigation:withError:"), Fn: failed},
			{Cmd: sel("userContentController:didReceiveScriptMessage:"), Fn: func(self objc.ID, _ objc.SEL, _, message objc.ID) {
				if w := webHandlers[self]; w != nil {
					body := message.Send(sel("body"))
					w.Dispatch(driver.Event{Kind: driver.EventMessage, Text: GoString(body.Send(sel("description")))})
				}
			}},
			{Cmd: sel("webView:startURLSchemeTask:"), Fn: func(self objc.ID, _ objc.SEL, _, task objc.ID) {
				w := schemeHandlers[self]
				if w == nil || task == 0 {
					return
				}
				u := task.Send(sel("request")).Send(sel("URL"))
				raw := GoString(u.Send(sel("absoluteString")))
				name := strings.ToLower(GoString(u.Send(sel("scheme"))))
				handler := w.config.SchemeHandlers[name]
				if handler == nil {
					task.Send(sel("didFinish"))
					return
				}
				body := handler(raw)
				mimeType := driver.WebMIME(GoString(u.Send(sel("path"))), body)
				response := objc.ID(Class("NSURLResponse")).Send(sel("alloc")).Send(
					sel("initWithURL:MIMEType:expectedContentLength:textEncodingName:"), u,
					NSString(mimeType), int64(len(body)), objc.ID(0))
				task.Send(sel("didReceiveResponse:"), response)
				response.Send(sel("release"))
				var ptr unsafe.Pointer
				if len(body) > 0 {
					ptr = unsafe.Pointer(&body[0])
				}
				data := objc.ID(Class("NSData")).Send(sel("dataWithBytes:length:"), ptr, uintptr(len(body)))
				task.Send(sel("didReceiveData:"), data)
				task.Send(sel("didFinish"))
			}},
			{Cmd: sel("webView:stopURLSchemeTask:"), Fn: func(objc.ID, objc.SEL, objc.ID, objc.ID) {}},
			// Downloads (WKDownload, macOS 11.3 and iOS 14.5), only for a
			// view that saves them.
			{Cmd: sel("webView:decidePolicyForNavigationAction:decisionHandler:"), Fn: func(self objc.ID, _ objc.SEL, _, action, decide objc.ID) {
				const cancel, allow, download = 0, 1, 2 // WKNavigationActionPolicy
				policy := uintptr(allow)
				// A restricted view's page goes only where its config
				// allows at the top level; a new window's has no frame.
				if w := webHandlers[self]; w != nil {
					frame := action.Send(sel("targetFrame"))
					if frame == 0 || objc.Send[bool](frame, sel("isMainFrame")) {
						url := GoString(action.Send(sel("request")).Send(sel("URL")).Send(sel("absoluteString")))
						allowed := true
						if strings.EqualFold(strings.SplitN(url, ":", 2)[0], "file") {
							allowed = w.fileDir != "" && fileURLInDir(url, w.fileDir)
						}
						if allowed && w.config.Restricted {
							allowed = w.config.Navigable(url)
						}
						if !allowed {
							CallBlock(decide, cancel)
							w.Dispatch(driver.Event{Kind: driver.EventError, Text: driver.RefusedNavigation(url),
								Navigation: &driver.WebNavigation{URL: url}})
							return
						}
					}
				}
				if w := webHandlers[self]; w != nil && w.config.DownloadDir != "" &&
					objc.Send[bool](action, sel("respondsToSelector:"), sel("shouldPerformDownload")) && objc.Send[bool](action, sel("shouldPerformDownload")) {
					policy = download
				}
				CallBlock(decide, policy)
			}},
			{Cmd: sel("webView:decidePolicyForNavigationResponse:decisionHandler:"), Fn: func(self objc.ID, _ objc.SEL, _, response, decide objc.ID) {
				const allow, download = 1, 2 // WKNavigationResponsePolicy
				policy := uintptr(allow)
				if w := webHandlers[self]; w != nil && w.config.DownloadDir != "" && objc.Send[bool](response, sel("isForMainFrame")) &&
					(!objc.Send[bool](response, sel("canShowMIMEType")) || attachment(response.Send(sel("response")))) {
					policy = download
				}
				CallBlock(decide, policy)
			}},
			{Cmd: sel("webView:navigationAction:didBecomeDownload:"), Fn: becameDownload},
			{Cmd: sel("webView:navigationResponse:didBecomeDownload:"), Fn: becameDownload},
			{Cmd: sel("download:decideDestinationUsingResponse:suggestedFilename:completionHandler:"),
				Fn: func(self objc.ID, _ objc.SEL, d, _, name, done objc.ID) {
					w, dl := webHandlers[self], downloads[d]
					if w == nil || dl == nil {
						CallBlock(done, 0)
						return
					}
					dl.Filename = filepath.Base(GoString(name))
					if dl.Filename == "." || dl.Filename == string(filepath.Separator) {
						dl.Filename = "download"
					}
					dir := filepath.Join(w.config.DownloadDir, dl.ID)
					if err := os.MkdirAll(dir, 0o755); err != nil {
						dl.State, dl.Message = driver.WebDownloadFailed, err.Error()
						w.download(d, dl)
						CallBlock(done, 0)
						return
					}
					dl.Path = filepath.Join(dir, dl.Filename)
					w.download(d, &driver.WebDownload{State: driver.WebDownloadStarted, ID: dl.ID, URL: dl.URL, Filename: dl.Filename, Path: dl.Path})
					CallBlock(done, uintptr(objc.ID(Class("NSURL")).Send(sel("fileURLWithPath:"), NSString(dl.Path))))
				}},
			{Cmd: sel("downloadDidFinish:"), Fn: func(self objc.ID, _ objc.SEL, d objc.ID) {
				if w, dl := webHandlers[self], downloads[d]; w != nil && dl != nil {
					dl.State = driver.WebDownloadFinished
					if info, err := os.Stat(dl.Path); err == nil {
						dl.Bytes = info.Size()
					}
					w.download(d, dl)
				}
			}},
			{Cmd: sel("download:didFailWithError:resumeData:"), Fn: func(self objc.ID, _ objc.SEL, d, err, _ objc.ID) {
				if w, dl := webHandlers[self], downloads[d]; w != nil && dl != nil {
					dl.State, dl.Message = driver.WebDownloadFailed, GoString(err.Send(sel("localizedDescription")))
					w.download(d, dl)
				}
			}},
		})
}

// downloads are the downloads running, by their WKDownload, each
// retained until it ends.
var (
	downloads   = map[objc.ID]*driver.WebDownload{}
	downloadSeq int
)

// becameDownload follows a navigation that became download d, which the
// web view's handler then delegates.
func becameDownload(self objc.ID, _ objc.SEL, _, _, d objc.ID) {
	w := webHandlers[self]
	if w == nil {
		return
	}
	downloadSeq++
	dl := &driver.WebDownload{ID: strconv.FormatInt(time.Now().UnixMilli(), 36) + "-" + strconv.Itoa(downloadSeq)}
	if request := d.Send(sel("originalRequest")); request != 0 {
		dl.URL = GoString(request.Send(sel("URL")).Send(sel("absoluteString")))
	}
	downloads[d.Send(sel("retain"))] = dl
	d.Send(sel("setDelegate:"), w.handler)
}

// attachment reports whether an HTTP response asks to be saved
// (Content-Disposition: attachment).
func attachment(response objc.ID) bool {
	if response == 0 || !objc.Send[bool](response, sel("isKindOfClass:"), Class("NSHTTPURLResponse")) {
		return false
	}
	v := GoString(response.Send(sel("valueForHTTPHeaderField:"), NSString("Content-Disposition")))
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(v)), "attachment")
}

// download reports dl, letting download d go once it ended.
func (w *Web) download(d objc.ID, dl *driver.WebDownload) {
	report := *dl
	w.Dispatch(driver.Event{Kind: driver.EventDownload, Data: &report})
	if dl.State != driver.WebDownloadStarted {
		delete(downloads, d)
		d.Send(sel("release"))
	}
}

// NewWeb returns the web view of holder, empty until configured.
func NewWeb(holder objc.ID) *Web {
	return &Web{holder: holder}
}

// View returns the WKWebView, 0 before the first Configure.
func (w *Web) View() objc.ID { return w.web }

// Configure applies c, loading the source again when it or the scripts
// changed.
func (w *Web) Configure(c *driver.WebConfig) {
	old, first := w.config, !w.made
	w.config = *c
	if first {
		w.make()
	}
	scripts := first || c.InjectedJavaScript != old.InjectedJavaScript ||
		c.InjectedJavaScriptBeforeContent != old.InjectedJavaScriptBeforeContent
	// NoScroll's script (macOS) applies from the next page on.
	if scripts || c.NoScroll != old.NoScroll {
		w.addScripts()
	}
	if c.UserAgent == "" {
		w.web.Send(sel("setCustomUserAgent:"), objc.ID(0))
	} else {
		w.web.Send(sel("setCustomUserAgent:"), NSString(c.UserAgent))
	}
	w.web.Send(sel("setAllowsBackForwardNavigationGestures:"), c.BackForwardGestures)
	w.web.Send(sel("configuration")).Send(sel("defaultWebpagePreferences")).
		Send(sel("setAllowsContentJavaScript:"), !c.NoJavaScript)
	if runtime.GOOS == "ios" {
		w.web.Send(sel("scrollView")).Send(sel("setScrollEnabled:"), !c.NoScroll)
	}
	if first || scripts || c.Source != old.Source || !maps.Equal(c.Headers, old.Headers) {
		w.load()
	}
}

func (w *Web) make() {
	loadWebKit()
	config := objc.ID(Class("WKWebViewConfiguration")).Send(sel("new"))
	if w.config.Incognito {
		store := objc.ID(Class("WKWebsiteDataStore")).Send(sel("nonPersistentDataStore"))
		config.Send(sel("setWebsiteDataStore:"), store)
	}
	w.handler = objc.ID(classWebHandler).Send(sel("new"))
	webHandlers[w.handler] = w
	config.Send(sel("userContentController")).Send(sel("addScriptMessageHandler:name:"), w.handler, NSString(messageHandler))
	for name := range w.config.SchemeHandlers {
		// WebKit raises NSInvalidArgumentException, which ends the app, for a scheme it handles itself: core refuses those, and this keeps a later WebKit's own from ending it.
		if objc.Send[bool](objc.ID(Class("WKWebView")), sel("handlesURLScheme:"), NSString(name)) {
			log.Printf("nsgo: WebKit handles the %q scheme itself: the web view does not serve it", name)
			continue
		}
		schemeHandlers[w.handler] = w
		config.Send(sel("setURLSchemeHandler:forURLScheme:"), w.handler, NSString(name))
	}
	bounds := objc.Send[CGRect](w.holder, sel("bounds"))
	w.web = objc.ID(Class("WKWebView")).Send(sel("alloc")).Send(sel("initWithFrame:configuration:"), bounds, config)
	config.Send(sel("release"))
	w.web.Send(sel("setNavigationDelegate:"), w.handler)
	const flexibleSize = 2 | 16 // width and height, in AppKit and UIKit alike
	w.web.Send(sel("setAutoresizingMask:"), uint(flexibleSize))
	// WebKit paints white otherwise; on macOS a page without a
	// background shows the view's own, as its style expects. iOS keeps
	// WebKit's, as NSH does.
	if runtime.GOOS != "ios" {
		no := objc.ID(Class("NSNumber")).Send(sel("numberWithBool:"), false)
		w.web.Send(sel("setValue:forKey:"), no, NSString("drawsBackground"))
	}
	w.holder.Send(sel("addSubview:"), w.web)
	w.made = true
}

func (w *Web) addScripts() {
	content := w.web.Send(sel("configuration")).Send(sel("userContentController"))
	content.Send(sel("removeAllUserScripts"))
	add := func(source string, at int) {
		if source == "" {
			return
		}
		script := objc.ID(Class("WKUserScript")).Send(sel("alloc")).Send(
			sel("initWithSource:injectionTime:forMainFrameOnly:"), NSString(source), at, true)
		content.Send(sel("addUserScript:"), script)
		script.Send(sel("release"))
	}
	const atStart, atEnd = 0, 1 // WKUserScriptInjectionTime
	add(bridge, atStart)
	add(w.config.InjectedJavaScriptBeforeContent, atStart)
	if w.config.NoScroll && runtime.GOOS != "ios" {
		add(noScroll, atEnd)
	}
	add(w.config.InjectedJavaScript, atEnd)
}

func (w *Web) load() {
	src := w.config.Source
	w.fileDir = ""
	switch {
	case src.HTML != "":
		base := objc.ID(0)
		if src.BaseURL != "" {
			base = nsURL(src.BaseURL)
		}
		w.web.Send(sel("loadHTMLString:baseURL:"), NSString(src.HTML), base)
	case src.Src != "":
		// A URL NSURL cannot parse loads nothing.
		if url := nsURL(src.Src); url != 0 {
			if path, ok := fileURLPath(src.Src); ok {
				w.fileDir = filepath.Dir(path)
				access := url.Send(sel("URLByDeletingLastPathComponent"))
				w.web.Send(sel("loadFileURL:allowingReadAccessToURL:"), url, access)
				return
			}
			request := objc.ID(Class("NSMutableURLRequest")).Send(sel("requestWithURL:"), url)
			for name, value := range w.config.Headers {
				request.Send(sel("setValue:forHTTPHeaderField:"), NSString(value), NSString(name))
			}
			w.web.Send(sel("loadRequest:"), request)
		}
	}
}

func fileURLPath(raw string) (string, bool) {
	u, err := neturl.Parse(raw)
	if err != nil || !strings.EqualFold(u.Scheme, "file") || (u.Host != "" && !strings.EqualFold(u.Host, "localhost")) || u.Path == "" {
		return "", false
	}
	p, err := neturl.PathUnescape(u.EscapedPath())
	if err != nil || p == "" {
		return "", false
	}
	return filepath.Clean(p), true
}

func fileURLInDir(raw, dir string) bool {
	path, ok := fileURLPath(raw)
	if !ok {
		return false
	}
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func nsURL(s string) objc.ID {
	return objc.ID(Class("NSURL")).Send(sel("URLWithString:"), NSString(s))
}

// navigated reports the page's state as kinds.
func (w *Web) navigated(loading bool, kinds ...driver.EventKind) {
	nav := driver.WebNavigation{
		URL:          GoString(w.web.Send(sel("URL")).Send(sel("absoluteString"))),
		Title:        GoString(w.web.Send(sel("title"))),
		Loading:      loading,
		CanGoBack:    objc.Send[bool](w.web, sel("canGoBack")),
		CanGoForward: objc.Send[bool](w.web, sel("canGoForward")),
	}
	for _, k := range kinds {
		n := nav
		w.Dispatch(driver.Event{Kind: k, Navigation: &n})
	}
}

// Command does c, with value for messages and scripts.
func (w *Web) Command(c driver.WebCommand, value string) {
	if w.web == 0 {
		return
	}
	// stopLoading may call the delegate back at once; the view outlives
	// the command.
	web := w.web.Send(sel("retain"))
	defer web.Send(sel("release"))
	switch c {
	case driver.WebReload:
		web.Send(sel("reload"))
	case driver.WebGoBack:
		web.Send(sel("goBack"))
	case driver.WebGoForward:
		web.Send(sel("goForward"))
	case driver.WebStopLoading:
		web.Send(sel("stopLoading"))
	case driver.WebPostMessage:
		w.evaluate(driver.WebMessageScript(value))
	case driver.WebInjectJavaScript:
		w.evaluate(value)
	}
}

func (w *Web) evaluate(script string) {
	w.web.Send(sel("evaluateJavaScript:completionHandler:"), NSString(script), objc.ID(0))
}

// Close stops the page and lets the WKWebView go.
func (w *Web) Close() {
	if w.web != 0 {
		w.close()
	}
}

func (w *Web) close() {
	w.web.Send(sel("stopLoading"))
	w.web.Send(sel("setNavigationDelegate:"), objc.ID(0))
	// The content controller holds the handler, which would keep it alive.
	w.web.Send(sel("configuration")).Send(sel("userContentController")).
		Send(sel("removeScriptMessageHandlerForName:"), NSString(messageHandler))
	w.web.Send(sel("removeFromSuperview"))
	w.web.Send(sel("release"))
	delete(webHandlers, w.handler)
	delete(schemeHandlers, w.handler)
	w.handler.Send(sel("release"))
	w.web, w.handler = 0, 0
}
