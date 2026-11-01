package core

import "github.com/oneo-me/nsgo/internal/driver"

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
	OnLayout  func(Bounds)
	Ref       *Ref
	Label     string
	TestID    string
}

// Render implements Node.
func (w WebView) Render(*Scope) Node { return w }

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
			NoScroll: w.NoScroll, NoJavaScript: w.NoJavaScript, BackForwardGestures: w.BackForwardGestures},
		loadStart: w.OnLoadStart, load: w.OnLoad, navigation: w.OnNavigationStateChange,
		loadError: w.OnError, message: w.OnMessage,
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
		call(wn.loadStart)
	case driver.EventLoad:
		call(wn.load)
	case driver.EventNavigationStateChange:
		call(wn.navigation)
	case driver.EventError:
		if wn.loadError != nil {
			t.within(n.owner, func() { wn.loadError(&WebError{URL: nav.URL, Message: ev.Text}) })
		}
	case driver.EventMessage:
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
	bit(driver.EventLoadStart, wn.loadStart != nil)
	bit(driver.EventLoad, wn.load != nil)
	bit(driver.EventNavigationStateChange, wn.navigation != nil)
	bit(driver.EventError, wn.loadError != nil)
	bit(driver.EventMessage, wn.message != nil)
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
