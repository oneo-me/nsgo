package pkg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

// Capabilities are what a package's code may reach, NSH's capabilities
// under NSH's keys: nsgo.yaml declares them, the package carries them and
// tsx.View holds the code to them, with what the host grants. What is not
// declared is refused.
type Capabilities struct {
	// Network is fetch and the images and videos loaded from the network.
	Network *Network `json:"network,omitempty"`
	// WebView is the pages web views show and navigate to.
	WebView *WebView `json:"webview,omitempty"`
	// OpenURL is openUrl: every URL, or those of Schemes.
	OpenURL *OpenURL `json:"openUrl,omitempty"`
	// Clipboard is reading and writing the system clipboard, Picker the
	// photo and camera pickers, Speech the microphone and speech
	// recognizer, StatusBar setting the status bar.
	Clipboard bool `json:"clipboard,omitempty"`
	Picker    bool `json:"picker,omitempty"`
	Speech    bool `json:"speech,omitempty"`
	StatusBar bool `json:"statusBar,omitempty"`
	// Host names the host's handlers host.call and host.post may reach:
	// a name, a prefix ending in "*", or "*" for all.
	Host []string `json:"host,omitempty"`
}

// Network says where requests may go: to Hosts, any request, or as an
// HTTP rule allows.
type Network struct {
	// Hosts are "*", "*.example.com" (its subdomains) or a host.
	Hosts []string   `json:"hosts,omitempty"`
	HTTP  []HTTPRule `json:"http,omitempty"`
}

// HTTPRule allows requests to Host's pattern, narrowed by each field set:
// Methods, Schemes (http, https, ws, wss), Port (a URL without one has
// its scheme's), Paths (exact, without the query) and PathPrefixes
// (whole segments: /v1 covers /v1/x, not /v10).
type HTTPRule struct {
	Host         string   `json:"host"`
	Methods      []string `json:"methods,omitempty"`
	Schemes      []string `json:"schemes,omitempty"`
	Port         int      `json:"port,omitempty"`
	Paths        []string `json:"paths,omitempty"`
	PathPrefixes []string `json:"pathPrefixes,omitempty"`
}

// WebView says what web views show: pages of Hosts, and with Input the
// trusted input their refs send, which carries user activation.
type WebView struct {
	Hosts []string `json:"hosts,omitempty"`
	Input bool     `json:"input,omitempty"`
}

// OpenURL allows URLs of Schemes, every scheme when empty.
type OpenURL struct {
	Schemes []string `json:"schemes,omitempty"`
}

// ParseCapabilities reads capabilities as JSON: strictly, as the signing
// tool does, refusing what it does not know; else as a runtime reads a
// package a newer tool may have signed, where an unknown capability is
// not granted and an unknown key inside one drops it, since it may
// narrow it, so what this runtime does not know never widens access.
// ignored names what was dropped. "host": true and "openUrl": true read
// as every handler and every URL.
func ParseCapabilities(data []byte, strict bool) (c Capabilities, ignored []string, err error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return c, nil, fmt.Errorf("%w: capabilities: %v", ErrFormat, err)
	}
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		raw := fields[k]
		var unknown []string
		switch k {
		case "network":
			c.Network, unknown, err = parseNetwork(raw, strict)
		case "webview":
			c.WebView = &WebView{}
			unknown, err = decodeKnown(raw, c.WebView, "hosts", "input")
			if err == nil {
				err = checkHosts(c.WebView.Hosts)
			}
		case "openUrl":
			if string(bytes.TrimSpace(raw)) == "true" {
				c.OpenURL = &OpenURL{}
				break
			}
			c.OpenURL = &OpenURL{}
			unknown, err = decodeKnown(raw, c.OpenURL, "schemes")
		case "clipboard":
			err = json.Unmarshal(raw, &c.Clipboard)
		case "picker":
			err = json.Unmarshal(raw, &c.Picker)
		case "speech":
			err = json.Unmarshal(raw, &c.Speech)
		case "statusBar":
			err = json.Unmarshal(raw, &c.StatusBar)
		case "host":
			if string(bytes.TrimSpace(raw)) == "true" {
				c.Host = []string{"*"}
				break
			}
			if err = json.Unmarshal(raw, &c.Host); err == nil {
				for _, h := range c.Host {
					if !validHostGrant(h) {
						err = fmt.Errorf("%q is not a host handler's name or prefix", h)
					}
				}
			}
		default:
			if strict {
				return Capabilities{}, nil, fmt.Errorf("%w: capabilities: no capability %q", ErrFormat, k)
			}
			ignored = append(ignored, "capability "+k)
			continue
		}
		if err == nil && len(unknown) > 0 {
			if strict {
				return Capabilities{}, nil, fmt.Errorf("%w: capabilities: %s has no %s", ErrFormat, k, strings.Join(unknown, ", "))
			}
			ignored = append(ignored, "capability "+k)
			dropCapability(&c, k)
			continue
		}
		if err != nil {
			if strict {
				return Capabilities{}, nil, fmt.Errorf("%w: capabilities: %s: %v", ErrFormat, k, err)
			}
			ignored = append(ignored, "capability "+k)
			dropCapability(&c, k)
		}
	}
	return c, ignored, nil
}

func dropCapability(c *Capabilities, key string) {
	switch key {
	case "network":
		c.Network = nil
	case "webview":
		c.WebView = nil
	case "openUrl":
		c.OpenURL = nil
	case "host":
		c.Host = nil
	}
}

// parseNetwork reads network: a rule with a key not known is dropped
// alone, leniently.
func parseNetwork(raw json.RawMessage, strict bool) (*Network, []string, error) {
	var fields struct {
		Hosts []string          `json:"hosts"`
		HTTP  []json.RawMessage `json:"http"`
	}
	unknown, err := decodeKnown(raw, &fields, "hosts", "http")
	if err != nil || len(unknown) > 0 {
		return nil, unknown, err
	}
	if err := checkHosts(fields.Hosts); err != nil {
		return nil, nil, err
	}
	n := &Network{Hosts: fields.Hosts}
	for _, r := range fields.HTTP {
		var rule HTTPRule
		unknown, err := decodeKnown(r, &rule, "host", "methods", "schemes", "port", "paths", "pathPrefixes")
		if err == nil {
			err = rule.check()
		}
		if err == nil && len(unknown) > 0 {
			err = fmt.Errorf("an http rule has no %s", strings.Join(unknown, ", "))
		}
		if err != nil {
			if strict {
				return nil, nil, err
			}
			continue
		}
		n.HTTP = append(n.HTTP, rule)
	}
	return n, nil, nil
}

// decodeKnown decodes raw into v, returning its keys not in known.
func decodeKnown(raw json.RawMessage, v any, known ...string) ([]string, error) {
	if err := json.Unmarshal(raw, v); err != nil {
		return nil, err
	}
	unknown, err := unknownFields(raw, known)
	if err != nil {
		return nil, err
	}
	return unknown, nil
}

func checkHosts(hosts []string) error {
	for _, h := range hosts {
		if !validHostPattern(h) {
			return fmt.Errorf("%q is not a host pattern: *, *.example.com or a host", h)
		}
	}
	return nil
}

// check refuses a rule that cannot match, rather than granting nothing.
func (r *HTTPRule) check() error {
	if !validHostPattern(r.Host) {
		return fmt.Errorf("an http rule's host %q is not a host pattern", r.Host)
	}
	for _, m := range r.Methods {
		if m == "" || strings.IndexFunc(m, func(c rune) bool { return !isTokenChar(c) }) >= 0 {
			return fmt.Errorf("%q is not an HTTP method", m)
		}
	}
	for _, s := range r.Schemes {
		if !slices.Contains([]string{"http", "https", "ws", "wss"}, strings.ToLower(s)) {
			return fmt.Errorf("scheme %q is none of http, https, ws and wss", s)
		}
	}
	if r.Port < 0 || r.Port > 65535 {
		return fmt.Errorf("port %d is not 1 to 65535", r.Port)
	}
	for _, p := range append(slices.Clone(r.Paths), r.PathPrefixes...) {
		if !strings.HasPrefix(p, "/") || strings.ContainsAny(p, "*?#") {
			return fmt.Errorf("path %q does not start with / or holds *, ? or #", p)
		}
	}
	return nil
}

func isTokenChar(c rune) bool {
	return c < 128 && (c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("!#$%&'*+-.^_`|~", c))
}

func validHostPattern(p string) bool {
	if p == "*" {
		return true
	}
	p = strings.TrimPrefix(p, "*.")
	return p != "" && !strings.ContainsAny(p, "*/:@ ?#")
}

// validHostName reports whether name may name a host handler: 1 to 128
// of A-Z, a-z, 0-9, _ . : -, without "..", as NSH's.
func validHostName(name string) bool {
	if name == "" || len(name) > 128 || strings.Contains(name, "..") {
		return false
	}
	for _, c := range name {
		if !(c < 128 && (c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("_.:-", c))) {
			return false
		}
	}
	return true
}

func validHostGrant(g string) bool {
	return g == "*" || validHostName(strings.TrimSuffix(g, "*"))
}

// The capabilities' keys, which core.Access names as its permissions.
const (
	capNetwork   = "network"
	capWebView   = "webview"
	capOpenURL   = "openUrl"
	capClipboard = "clipboard"
	capPicker    = "picker"
	capSpeech    = "speech"
	capStatusBar = "statusBar"
	capHost      = "host"
)

// Allows reports whether c grants permission, a capability's key, for
// resource: a URL, or a host handler's name. method is a network
// request's HTTP method, or "input" for a web view's trusted input. It
// is core.Access's Allows.
func (c *Capabilities) Allows(permission, resource, method string) bool {
	switch permission {
	case capNetwork:
		return c.Network != nil && c.Network.allows(resource, method)
	case capWebView:
		if c.WebView == nil {
			return false
		}
		if method == "input" {
			return c.WebView.Input
		}
		u, ok := webURL(resource)
		return ok && hostAllowed(c.WebView.Hosts, u.Hostname())
	case capOpenURL:
		if c.OpenURL == nil {
			return false
		}
		u, err := url.Parse(resource)
		return err == nil && (len(c.OpenURL.Schemes) == 0 || slices.ContainsFunc(c.OpenURL.Schemes, func(s string) bool {
			return strings.EqualFold(s, u.Scheme)
		}))
	case capClipboard:
		return c.Clipboard
	case capPicker:
		return c.Picker
	case capSpeech:
		return c.Speech
	case capStatusBar:
		return c.StatusBar
	case capHost:
		if !validHostName(resource) {
			return false
		}
		for _, g := range c.Host {
			if g == "*" || g == resource || strings.HasSuffix(g, "*") && strings.HasPrefix(resource, strings.TrimSuffix(g, "*")) {
				return true
			}
		}
	}
	return false
}

// WebHosts are the hosts c's web views may navigate to, core.Access's.
func (c *Capabilities) WebHosts() []string {
	if c.WebView == nil {
		return nil
	}
	return c.WebView.Hosts
}

// webURL parses an http, https, ws or wss URL with a host.
func webURL(s string) (*url.URL, bool) {
	u, err := url.Parse(s)
	if err != nil || u.Hostname() == "" {
		return nil, false
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https", "ws", "wss":
		return u, true
	}
	return nil, false
}

func (n *Network) allows(resource, method string) bool {
	u, ok := webURL(resource)
	if !ok {
		return false
	}
	if hostAllowed(n.Hosts, u.Hostname()) {
		return true
	}
	for _, r := range n.HTTP {
		if r.allows(u, method) {
			return true
		}
	}
	return false
}

func (r *HTTPRule) allows(u *url.URL, method string) bool {
	if !hostAllowed([]string{r.Host}, u.Hostname()) {
		return false
	}
	if len(r.Methods) > 0 && !slices.ContainsFunc(r.Methods, func(m string) bool { return strings.EqualFold(m, method) }) {
		return false
	}
	scheme := strings.ToLower(u.Scheme)
	if len(r.Schemes) > 0 && !slices.ContainsFunc(r.Schemes, func(s string) bool { return strings.EqualFold(s, scheme) }) {
		return false
	}
	if r.Port != 0 {
		port := u.Port()
		if port == "" {
			port = map[string]string{"http": "80", "ws": "80", "https": "443", "wss": "443"}[scheme]
		}
		if port != strconv.Itoa(r.Port) {
			return false
		}
	}
	if len(r.Paths) == 0 && len(r.PathPrefixes) == 0 {
		return true
	}
	path := u.EscapedPath()
	if path == "" {
		path = "/"
	}
	if slices.Contains(r.Paths, path) {
		return true
	}
	for _, p := range r.PathPrefixes {
		if path == p || strings.HasPrefix(path, p) && (strings.HasSuffix(p, "/") || path[len(p)] == '/') {
			return true
		}
	}
	return false
}

// hostAllowed reports whether a pattern of patterns allows host: "*"
// any, "*.example.com" its subdomains, else the host itself.
func hostAllowed(patterns []string, host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	for _, p := range patterns {
		p = strings.ToLower(p)
		if p == "*" || p == host {
			return true
		}
		if suffix, ok := strings.CutPrefix(p, "*."); ok && strings.HasSuffix(host, "."+suffix) {
			return true
		}
	}
	return false
}
