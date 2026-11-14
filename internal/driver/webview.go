package driver

import (
	"encoding/json"
	"errors"
	"net/url"
	"strings"
)

// WebSource is what a web view shows: the page at Src, or HTML, whose
// relative links resolve against BaseURL, instead when it is set.
type WebSource struct {
	Src     string
	HTML    string
	BaseURL string
}

// WebNavigation is a web view's page and history.
type WebNavigation struct {
	URL          string
	Title        string
	Loading      bool
	CanGoBack    bool
	CanGoForward bool
}

// WebDownload is a download of a web view: its State, and where it is
// saved. Bytes is what it wrote once finished, and Message why it failed.
type WebDownload struct {
	State    WebDownloadState
	ID       string
	URL      string
	Filename string
	Path     string
	Bytes    int64
	Message  string
}

// WebDownloadState is how far a WebDownload got.
type WebDownloadState uint8

const (
	WebDownloadStarted WebDownloadState = iota
	WebDownloadFinished
	WebDownloadFailed
)

// WebConfig is everything a web view loads and allows (ItemWeb).
// Platforms load the source again when it or the scripts change.
type WebConfig struct {
	Source                          WebSource
	InjectedJavaScript              string
	InjectedJavaScriptBeforeContent string
	UserAgent                       string
	// Headers are sent with the initial URL request where the platform
	// supports custom request headers. HTML sources do not make one.
	Headers map[string]string
	// Incognito keeps website data in memory for this view. Platforms read
	// this when the native view is first made.
	Incognito           bool
	NoScroll            bool
	NoJavaScript        bool
	BackForwardGestures bool
	// DownloadDir is where the view saves what its pages download, a
	// directory of its own for each (<id>/<file name>); "" saves nothing.
	DownloadDir string
	// Restricted has the platform refuse the top-level navigations
	// Navigable refuses, as they start, reporting EventError.
	Restricted bool
	// Hosts are the hosts Navigable allows, space-separated patterns:
	// "*", "*.example.com" or a host.
	Hosts string
}

// Navigable reports whether a restricted view's page may navigate to
// rawURL at its top level: a page of its own (about:, data:, blob:), or
// an http or https one of a host Hosts allows. Other schemes, file:
// among them, are refused.
func (c *WebConfig) Navigable(rawURL string) bool {
	if !c.Restricted {
		return true
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	switch strings.ToLower(u.Scheme) {
	case "about", "data", "blob":
		return true
	case "http", "https":
		return AllowsHost(strings.Fields(c.Hosts), u.Hostname())
	}
	return false
}

// RefusedNavigation is the message of EventError for a navigation
// Navigable refused.
func RefusedNavigation(url string) string {
	return "nsgo: the web view may not navigate to " + url + " (the webview capability's hosts)"
}

// AllowsHost reports whether one of patterns allows host: "*" any, "*.example.com"
// its subdomains, else the host itself. Hosts compare in lower case.
func AllowsHost(patterns []string, host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	for _, p := range patterns {
		if HostCovers(p, host) {
			return true
		}
	}
	return false
}

// HostCovers reports whether pattern covers other, a host or a pattern.
func HostCovers(pattern, other string) bool {
	pattern, other = strings.ToLower(pattern), strings.ToLower(other)
	if pattern == "*" {
		return true
	}
	if suffix, ok := strings.CutPrefix(pattern, "*."); ok {
		if inner, ok := strings.CutPrefix(other, "*."); ok && inner == suffix {
			return true
		}
		return strings.HasSuffix(other, "."+suffix)
	}
	return pattern == other
}

// IntersectHosts returns the patterns allowing what both a and b allow.
func IntersectHosts(a, b []string) []string {
	var out []string
	for _, x := range a {
		for _, y := range b {
			switch {
			case HostCovers(x, y):
				out = append(out, y)
			case HostCovers(y, x):
				out = append(out, x)
			}
		}
	}
	return out
}

// WebCommand is what ItemWebCommand asks a web view to do.
type WebCommand uint8

const (
	WebReload WebCommand = iota + 1
	WebGoBack
	WebGoForward
	WebStopLoading
	// WebPostMessage dispatches a MessageEvent carrying Value on the
	// page's window.
	WebPostMessage
	// WebInjectJavaScript runs Value in the page.
	WebInjectJavaScript
	// WebSendInput gives the page Data, a *WebInput, as the user's own
	// input.
	WebSendInput
)

// WebInput is input a web view's page takes as the user's hands give it:
// its events are trusted (isTrusted), which no script it runs can make.
type WebInput struct {
	Kind WebInputKind
	// Point is a click's, in CSS pixels from the viewport's top left, as
	// getBoundingClientRect reports.
	Point Point
	// Text is what WebInputText types.
	Text string
	// Key is what WebInputKey presses, as KeyboardEvent.key names it:
	// Enter, Tab, Escape or Backspace.
	Key string
}

// WebInputKind is what a WebInput gives.
type WebInputKind uint8

const (
	// WebInputClick is a primary click at Point.
	WebInputClick WebInputKind = iota
	// WebInputText types Text into what has the page's focus.
	WebInputText
	// WebInputKey presses and releases Key.
	WebInputKey
)

// WebBridge is the script a platform runs at the start of every page
// before the app's, given post, the JavaScript expression of a function
// that sends its string argument to the app.
func WebBridge(post string) string {
	return "window.nsgo = window.nsgo || { postMessage: function (data) { (" + post + ")(String(data)); } };"
}

// WebEvaluateScript is the script that runs script in a page as an
// expression and posts its answer through the bridge, marker first: its
// result as JSON, a promise's once settled, or what it threw (decoded by
// WebEvaluateAnswer).
func WebEvaluateScript(script, marker string) string {
	return `(function () {
  var bridge = window.nsgo, post = bridge && bridge.postMessage;
  if (!post) return;
  var send = function (answer) { post.call(bridge, ` + JSString(marker) + ` + JSON.stringify(answer)); };
  var fail = function (e) { send({ error: String(e && e.message !== undefined ? e.message : e) }); };
  try {
    Promise.resolve((0, eval)(` + JSString(script) + `)).then(function (v) {
      var json;
      try { json = JSON.stringify(v === undefined ? null : v); } catch (e) { fail(e); return; }
      if (json === undefined) { fail("the result is no JSON value"); return; }
      send({ json: json });
    }, fail);
  } catch (e) { fail(e); }
})();`
}

// WebEvaluateAnswer decodes what WebEvaluateScript posted after its
// marker: the result as JSON, or the error the script threw.
func WebEvaluateAnswer(answer string) (string, error) {
	var a struct {
		JSON  *string `json:"json"`
		Error *string `json:"error"`
	}
	if err := json.Unmarshal([]byte(answer), &a); err != nil {
		return "", err
	}
	switch {
	case a.Error != nil:
		return "", errors.New(*a.Error)
	case a.JSON == nil:
		return "", errors.New("nsgo: a web view's answer holds no result")
	}
	return *a.JSON, nil
}

// WebMessageScript is the script WebPostMessage runs in the page.
func WebMessageScript(data string) string {
	return "window.dispatchEvent(new MessageEvent('message', { data: " + JSString(data) + " }));"
}

// JSString quotes s as a JavaScript string literal.
func JSString(s string) string {
	b := make([]byte, 0, len(s)+2)
	b = append(b, '"')
	for _, r := range s {
		switch {
		case r == '"':
			b = append(b, '\\', '"')
		case r == '\\':
			b = append(b, '\\', '\\')
		case r == '\n':
			b = append(b, '\\', 'n')
		case r == '\r':
			b = append(b, '\\', 'r')
		case r < 0x20 || r == 0x2028 || r == 0x2029:
			const hex = "0123456789abcdef"
			b = append(b, '\\', 'u', hex[r>>12&15], hex[r>>8&15], hex[r>>4&15], hex[r&15])
		default:
			b = append(b, string(r)...)
		}
	}
	return string(append(b, '"'))
}
