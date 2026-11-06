package driver

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
	NoScroll                        bool
	NoJavaScript                    bool
	BackForwardGestures             bool
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
)

// WebBridge is the script a platform runs at the start of every page
// before the app's, given post, the JavaScript expression of a function
// that sends its string argument to the app.
func WebBridge(post string) string {
	return "window.nsgo = window.nsgo || { postMessage: function (data) { (" + post + ")(String(data)); } };"
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
