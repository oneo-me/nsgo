package core

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"maps"
	"path/filepath"
	"strings"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// WebView shows a web page in the platform's web view, which fills the
// view and takes its own pointer, wheel and keyboard input. Pages reach
// the app with window.nsgo.postMessage(data), which OnMessage receives;
// Ref.PostMessage dispatches a MessageEvent on the page's window.
type WebView struct {
	Source WebSource
	// The page loads again when SourceFunc's source changes.
	SourceFunc func() WebSource
	// InjectedJavaScript runs after each page loads, and
	// InjectedJavaScriptBeforeContent before each page's own scripts.
	InjectedJavaScript              string
	InjectedJavaScriptBeforeContent string
	// UserAgent replaces the platform's.
	UserAgent string
	// Headers are sent with the initial URL request where the platform
	// supports custom request headers. HTML sources do not make one.
	Headers map[string]string
	// Incognito keeps website data in memory for this view. It is read when
	// the native view is first made; changing it afterwards has no effect.
	Incognito bool
	// NoScroll keeps the user from scrolling the page; NoJavaScript keeps
	// pages from running scripts.
	NoScroll     bool
	NoJavaScript bool
	// BackForwardGestures lets swipes go back and forward where the
	// platform has them (iOS, macOS trackpads).
	BackForwardGestures bool
	// Class is Tailwind CSS classes, as View's.
	Class     string
	Style     Style
	StyleFunc func() Style

	// OnLoadStart, OnLoad and OnNavigationStateChange receive the page's
	// state as it starts loading, finishes and navigates.
	OnLoadStart             func(WebNavigation)
	OnLoad                  func(WebNavigation)
	OnNavigationStateChange func(WebNavigation)
	// OnError receives a *WebError.
	OnError func(err error)
	// OnMessage receives what the page posted, as a string.
	OnMessage func(data string)
	// OnDownload follows what the page downloads, which only a web view
	// with one saves, to downloads/<id>/ in the app's data directory:
	// files WebKit cannot show and links that ask for a download, on
	// Apple platforms, Linux, Windows and Android.
	OnDownload func(WebDownload)
	OnLayout   func(Bounds)
	Ref        *Ref
	Label      string
	TestID     string
}

// Render implements Node.
func (w WebView) Render(*Scope) Node { return w }

func (w WebView) mountIn(t *Tree, cx *Scope, parent *node) { t.mountWebView(cx, parent, w) }

// WebSrc returns the source loading the page at url.
func WebSrc(url string) WebSource { return WebSource{Src: url} }

// WebHTML returns the source showing html, with relative links resolved
// against baseURL ("" for none).
func WebHTML(html, baseURL string) WebSource { return WebSource{HTML: html, BaseURL: baseURL} }

// WebError is a page that failed to load, or a navigation the
// platform refused; URL is the page's, when known.
type WebError struct {
	URL     string
	Message string
}

func (e *WebError) Error() string { return e.Message }

type webNode struct {
	config                      driver.WebConfig
	sent                        driver.WebConfig
	loadStart, load, navigation func(WebNavigation)
	loadError                   func(error)
	message                     func(string)
	download                    func(WebDownload)
	// refused is the source Access refused last.
	refused string
	// evals are the Evaluates waiting for their pages' answers, by the
	// marker their answers start with.
	evals map[string]func(string, error)
}

func cloneWebHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	return maps.Clone(headers)
}

func (t *Tree) mountWebView(cx *Scope, parent *node, w WebView) {
	n := t.newNode(driver.KindWeb, parent, cx)
	t.setClasses(n, w.Class)
	t.bindStyle(cx, n, w.Style, w.StyleFunc)
	t.setRef(n, w.Ref)
	n.h = handlers{layout: w.OnLayout}
	n.props = driver.ViewProps{Label: w.Label, TestID: w.TestID}
	n.web = &webNode{
		config: driver.WebConfig{Source: w.Source, InjectedJavaScript: w.InjectedJavaScript,
			InjectedJavaScriptBeforeContent: w.InjectedJavaScriptBeforeContent, UserAgent: w.UserAgent,
			Headers: cloneWebHeaders(w.Headers), Incognito: w.Incognito,
			NoScroll: w.NoScroll, NoJavaScript: w.NoJavaScript, BackForwardGestures: w.BackForwardGestures},
		loadStart: w.OnLoadStart, load: w.OnLoad, navigation: w.OnNavigationStateChange,
		loadError: w.OnError, message: w.OnMessage, download: w.OnDownload,
	}
	if w.OnDownload != nil && cx.rt.dataDir != "" {
		n.web.config.DownloadDir = filepath.Join(cx.rt.dataDir, "downloads")
	}
	if w.SourceFunc != nil {
		cx.renderEffect(func() {
			if src := w.SourceFunc(); src != n.web.config.Source {
				n.web.config.Source = src
				t.touch(n)
			}
		})
	}
}

// restrictWeb holds c to the Access of n's owner: a source it refuses
// shows nothing, the platform refuses navigations to hosts it does not
// allow, and nothing is downloaded.
func (t *Tree) restrictWeb(n *node, c *driver.WebConfig) {
	a := accessOf(n.owner)
	if a == nil {
		return
	}
	wn := n.web
	report := func(err error) {
		if wn.loadError != nil {
			wn.loadError(&WebError{URL: c.Source.Src, Message: err.Error()})
		}
	}
	t.refuseSource(n, AccessWebView, &c.Source.Src, &wn.refused, report)
	if c.Source.BaseURL != "" && sourceRefusal(n.owner, AccessWebView, c.Source.BaseURL) != nil {
		c.Source.BaseURL = ""
	}
	c.Restricted, c.Hosts, c.DownloadDir = true, strings.Join(a.WebHosts(), " "), ""
}

// How far a WebDownload got.
const (
	WebDownloadStarted  = driver.WebDownloadStarted
	WebDownloadFinished = driver.WebDownloadFinished
	WebDownloadFailed   = driver.WebDownloadFailed
)

// dispatchWeb handles a web view's events.
func (t *Tree) dispatchWeb(n *node, ev driver.Event) {
	wn := n.web
	var nav WebNavigation
	if ev.Navigation != nil {
		nav = *ev.Navigation
	}
	call := func(fn func(WebNavigation)) {
		if fn != nil {
			t.within(n.owner, func() { fn(nav) })
		}
	}
	switch ev.Kind {
	case driver.EventLoadStart:
		// The page that would answer is going.
		t.endEvals(n, errWebPageLeft)
		call(wn.loadStart)
	case driver.EventLoad:
		call(wn.load)
	case driver.EventNavigationStateChange:
		call(wn.navigation)
	case driver.EventError:
		if wn.loadError != nil {
			t.within(n.owner, func() { wn.loadError(&WebError{URL: nav.URL, Message: ev.Text}) })
		}
	case driver.EventDownload:
		if d, ok := ev.Data.(*WebDownload); ok && wn.download != nil {
			t.within(n.owner, func() { wn.download(*d) })
		}
	case driver.EventMessage:
		if t.answerEval(n, ev.Text) {
			return
		}
		if wn.message != nil {
			t.within(n.owner, func() { wn.message(ev.Text) })
		}
	}
}

// webEvents is what a web view reports.
func (wn *webNode) events() driver.EventMask {
	var m driver.EventMask
	bit := func(k driver.EventKind, on bool) {
		if on {
			m |= 1 << k
		}
	}
	bit(driver.EventLoadStart, wn.loadStart != nil || len(wn.evals) > 0)
	bit(driver.EventLoad, wn.load != nil)
	bit(driver.EventNavigationStateChange, wn.navigation != nil)
	bit(driver.EventError, wn.loadError != nil)
	bit(driver.EventMessage, wn.message != nil || len(wn.evals) > 0)
	bit(driver.EventDownload, wn.download != nil)
	return m
}

// webCommand sends c to the web view r names.
func (r *Ref) webCommand(c driver.WebCommand, value string) {
	if r.n == nil || r.n.deleted || r.n.web == nil {
		return
	}
	r.t.commands = append(r.t.commands, driver.MountItem{Kind: driver.ItemWebCommand, ID: r.n.id, WebCommand: c, Value: value})
	r.t.changed = true
}

// Reload loads the web view's page again.
func (r *Ref) Reload() { r.webCommand(driver.WebReload, "") }

// GoBack goes back in the web view's history.
func (r *Ref) GoBack() { r.webCommand(driver.WebGoBack, "") }

// GoForward goes forward in the web view's history.
func (r *Ref) GoForward() { r.webCommand(driver.WebGoForward, "") }

// StopLoading stops the web view's page loading.
func (r *Ref) StopLoading() { r.webCommand(driver.WebStopLoading, "") }

// PostMessage dispatches a MessageEvent carrying data on the web view's
// page.
func (r *Ref) PostMessage(data string) { r.webCommand(driver.WebPostMessage, data) }

// InjectJavaScript runs script in the web view's page.
func (r *Ref) InjectJavaScript(script string) { r.webCommand(driver.WebInjectJavaScript, script) }

// SendInput gives the web view's page input as the user's own, whose
// events are trusted and carry user activation, which no script the page
// runs can make: a click, typed text or a key. It takes the keyboard focus
// first; macOS has it, other platforms drop it.
func (r *Ref) SendInput(in WebInput) {
	if r.n == nil || r.n.deleted || r.n.web == nil {
		return
	}
	if r.t.rt.restricted && refusal(accessOf(r.n.owner), AccessRequest{Permission: AccessWebView, Method: AccessInput}) != nil {
		log.Printf("nsgo: a web view's trusted input needs the %q capability's input", AccessWebView)
		return
	}
	r.t.commands = append(r.t.commands, driver.MountItem{Kind: driver.ItemWebCommand, ID: r.n.id, WebCommand: driver.WebSendInput, Data: &in})
	r.t.changed = true
}

// WebClick is a primary click at x, y in CSS pixels from the page's
// viewport, as getBoundingClientRect reports them.
func WebClick(x, y Points) WebInput {
	return WebInput{Kind: driver.WebInputClick, Point: Point{X: x, Y: y}}
}

// WebText types text into what has the page's focus.
func WebText(text string) WebInput { return WebInput{Kind: driver.WebInputText, Text: text} }

// WebKey presses key, as KeyboardEvent.key names it: Enter, Tab, Escape
// or Backspace.
func WebKey(key string) WebInput { return WebInput{Kind: driver.WebInputKey, Key: key} }

// errWebPageLeft ends an Evaluate whose page left before it answered.
var errWebPageLeft = errors.New("nsgo: the web view's page left before the script answered")

// evalMarker starts a page's answer to an Evaluate, before its random
// key, so no message of the page's own is taken for one. It is plain text:
// a platform's bridge may read the message as a C string.
const evalMarker = "nsgo-evaluate:"

// Evaluate runs script in the web view's page, as an expression, and
// calls done on the engine's goroutine, while the web view lives, with
// its result as JSON, or the
// error it threw: a promise it returns is awaited, undefined is null, and
// what JSON cannot hold (a function, a cycle) is an error. The answer
// comes through the page's channel to the app, as OnMessage's, on every
// backend with web views. A page that leaves first ends it with an error,
// as does a view with NoJavaScript, and a view that goes never answers.
func (r *Ref) Evaluate(script string, done func(json string, err error)) {
	if r.n == nil || r.n.deleted || r.n.web == nil {
		done("", errors.New("nsgo: Evaluate on no web view"))
		return
	}
	n, wn := r.n, r.n.web
	if wn.config.NoJavaScript {
		done("", errors.New("nsgo: the web view runs no JavaScript"))
		return
	}
	var key [8]byte
	_, _ = rand.Read(key[:])
	marker := evalMarker + hex.EncodeToString(key[:]) + ":"
	if wn.evals == nil {
		wn.evals = map[string]func(string, error){}
		r.t.touch(n)
	}
	t := r.t
	wn.evals[marker] = func(json string, err error) {
		if !n.owner.disposed {
			t.within(n.owner, func() { done(json, err) })
		}
	}
	r.webCommand(driver.WebInjectJavaScript, driver.WebEvaluateScript(script, marker))
}

// answerEval hands a page's answer to the Evaluate it is for, and
// reports whether text was one.
func (t *Tree) answerEval(n *node, text string) bool {
	wn := n.web
	if len(wn.evals) == 0 || !strings.HasPrefix(text, evalMarker) {
		return false
	}
	i := strings.IndexByte(text[len(evalMarker):], ':')
	if i < 0 {
		return false
	}
	marker := text[:len(evalMarker)+i+1]
	done, ok := wn.evals[marker]
	if !ok {
		return false
	}
	delete(wn.evals, marker)
	if len(wn.evals) == 0 {
		wn.evals = nil
		t.touch(n)
	}
	done(driver.WebEvaluateAnswer(text[len(marker):]))
	return true
}

// endEvals ends the Evaluates n waits for with err.
func (t *Tree) endEvals(n *node, err error) {
	evals := n.web.evals
	if len(evals) == 0 {
		return
	}
	n.web.evals = nil
	t.touch(n)
	for _, done := range evals {
		done("", err)
	}
}
