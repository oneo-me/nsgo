package tsx

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/dop251/goja"
	"golang.org/x/net/idna"

	"github.com/oneo-me/nsgo/core"
)

// The web APIs (NSH's web/): web.js defines the classes on the natives
// here, fetch on net/http among them.

//go:embed web.js
var webSource string

var webProgram = goja.MustCompile("web.js", webSource, true)

//go:embed intl.js
var intlSource string

var intlProgram = goja.MustCompile("intl.js", intlSource, true)

// A Fetcher answers an app's http and https requests in place of the
// network: a test's stand-in network.
type Fetcher func(req *http.Request) (*http.Response, error)

var fetcherKey = core.NewKey[Fetcher]("tsx.fetcher", nil)

// WithFetcher returns app, its fetches answered by f.
func WithFetcher(app func(cx *core.Scope) core.Node, f Fetcher) func(cx *core.Scope) core.Node {
	return func(cx *core.Scope) core.Node {
		fetcherKey.Provide(cx, f)
		return app(cx)
	}
}

// roundTrip makes a Fetcher an http.RoundTripper.
type roundTrip Fetcher

func (f roundTrip) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

// installWeb defines the web APIs' globals.
func (r *runtime) installWeb() {
	v, err := r.vm.RunProgram(webProgram)
	if err != nil {
		panic(err)
	}
	install, _ := goja.AssertFunction(v)
	natives := r.webNatives()
	out, err := install(goja.Undefined(), r.vm.GlobalObject(), natives)
	if err != nil {
		panic(err)
	}
	r.web = out.ToObject(r.vm)
	v, err = r.vm.RunProgram(intlProgram)
	if err != nil {
		panic(err)
	}
	install, _ = goja.AssertFunction(v)
	if _, err := install(goja.Undefined(), r.vm.GlobalObject(), r.intlNatives()); err != nil {
		panic(err)
	}
}

// newFile makes a File of data.
func (r *runtime) newFile(data []byte, name, mime string) goja.Value {
	return r.call(r.web.Get("newFile"), r.vm.ToValue(r.vm.NewArrayBuffer(data)), r.vm.ToValue(name), r.vm.ToValue(mime))
}

// pastedFile is a pasted file as a File.
func (r *runtime) pastedFile(f core.PastedFile) goja.Value {
	return r.newFile(f.Data, f.Name, f.MIME)
}

func (r *runtime) webNatives() *goja.Object {
	n := r.vm.NewObject()
	set := func(name string, f func(call goja.FunctionCall) goja.Value) { _ = n.Set(name, r.function(f)) }
	set("now", func(goja.FunctionCall) goja.Value {
		return r.vm.ToValue(r.millis(r.cx.Now()))
	})
	set("utf8Encode", func(call goja.FunctionCall) goja.Value {
		return r.vm.ToValue(r.vm.NewArrayBuffer([]byte(arg(call, 0).String())))
	})
	set("utf8Decode", func(call goja.FunctionCall) goja.Value {
		b := r.bytes(arg(call, 0))
		if arg(call, 1).ToBoolean() && !utf8.Valid(b) {
			return goja.Null()
		}
		return r.vm.ToValue(strings.ToValidUTF8(string(b), "�"))
	})
	set("atob", func(call goja.FunctionCall) goja.Value {
		s := strings.Map(func(c rune) rune {
			if c == ' ' || c == '\t' || c == '\n' || c == '\f' || c == '\r' {
				return -1
			}
			return c
		}, arg(call, 0).String())
		if len(s)%4 != 0 {
			s += strings.Repeat("=", 4-len(s)%4)
		}
		b, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			return goja.Null()
		}
		runes := make([]rune, len(b))
		for i, c := range b {
			runes[i] = rune(c)
		}
		return r.vm.ToValue(string(runes))
	})
	set("btoa", func(call goja.FunctionCall) goja.Value {
		s := arg(call, 0).String()
		b := make([]byte, 0, len(s))
		for _, c := range s {
			if c > 0xff {
				return goja.Null()
			}
			b = append(b, byte(c))
		}
		return r.vm.ToValue(base64.StdEncoding.EncodeToString(b))
	})
	set("formEncode", func(call goja.FunctionCall) goja.Value {
		return r.vm.ToValue(formEncode(arg(call, 0).String()))
	})
	set("formDecode", func(call goja.FunctionCall) goja.Value {
		s := arg(call, 0).String()
		if d, err := url.QueryUnescape(s); err == nil {
			return r.vm.ToValue(d)
		}
		return r.vm.ToValue(strings.ReplaceAll(s, "+", " "))
	})
	set("parseURL", func(call goja.FunctionCall) goja.Value {
		var base string
		if defined(arg(call, 1)) {
			base = arg(call, 1).String()
		}
		u, ok := parseURL(arg(call, 0).String(), base)
		if !ok {
			return goja.Null()
		}
		return r.urlParts(u)
	})
	// A setter's URL is normalized again: a host lowercased, a default
	// port dropped, a path encoded.
	set("setURL", func(call goja.FunctionCall) goja.Value {
		u, ok := parseURL(arg(call, 0).String(), "")
		if !ok {
			return goja.Null()
		}
		v := arg(call, 2).String()
		switch arg(call, 1).String() {
		case "protocol":
			u.Scheme = strings.TrimSuffix(v, ":")
		case "username":
			_, hasPass := u.User.Password()
			pass, _ := u.User.Password()
			if hasPass {
				u.User = url.UserPassword(v, pass)
			} else {
				u.User = url.User(v)
			}
		case "password":
			u.User = url.UserPassword(u.User.Username(), v)
		case "host":
			u.Host = v
		case "hostname":
			if p := u.Port(); p != "" {
				u.Host = v + ":" + p
			} else {
				u.Host = v
			}
		case "port":
			if v == "" {
				u.Host = u.Hostname()
			} else {
				u.Host = u.Hostname() + ":" + v
			}
		case "pathname":
			if !strings.HasPrefix(v, "/") {
				v = "/" + v
			}
			u.Path, u.RawPath = v, ""
		case "search":
			u.RawQuery = strings.TrimPrefix(v, "?")
			u.ForceQuery = false
		case "hash":
			u.Fragment, u.RawFragment = strings.TrimPrefix(v, "#"), ""
		}
		u, ok = normalizeURL(u)
		if !ok {
			return goja.Null()
		}
		return r.urlParts(u)
	})
	set("baseURL", func(goja.FunctionCall) goja.Value { return r.vm.ToValue(r.base) })
	set("blobURL", func(call goja.FunctionCall) goja.Value {
		r.blobSeq++
		url := fmt.Sprintf("blob:nsgo/%d", r.blobSeq)
		if r.blobs == nil {
			r.blobs = map[string]blobEntry{}
		}
		r.blobs[url] = blobEntry{bytes.Clone(r.bytes(arg(call, 0))), arg(call, 1).String()}
		return r.vm.ToValue(url)
	})
	set("revokeBlobURL", func(call goja.FunctionCall) goja.Value {
		delete(r.blobs, arg(call, 0).String())
		return goja.Undefined()
	})
	set("blobOf", func(call goja.FunctionCall) goja.Value {
		b, ok := r.blobs[arg(call, 0).String()]
		if !ok {
			return goja.Null()
		}
		o := r.vm.NewObject()
		_ = o.Set("bytes", r.vm.NewArrayBuffer(bytes.Clone(b.bytes)))
		_ = o.Set("type", b.typ)
		return o
	})
	set("fetch", r.fetch)
	return n
}

// blobEntry is what a blob: URL names: a blob's bytes and type.
type blobEntry struct {
	bytes []byte
	typ   string
}

// bytes reads an ArrayBuffer or a view of one.
func (r *runtime) bytes(v goja.Value) []byte {
	switch x := v.Export().(type) {
	case goja.ArrayBuffer:
		return x.Bytes()
	case []byte:
		return x
	}
	if o, ok := v.(*goja.Object); ok {
		if buf, ok := o.Get("buffer").Export().(goja.ArrayBuffer); ok {
			off, n := int(o.Get("byteOffset").ToInteger()), int(o.Get("byteLength").ToInteger())
			return buf.Bytes()[off : off+n]
		}
	}
	return nil
}

// special are the schemes WHATWG URLs give a host and a path of "/".
var special = map[string]string{"http": "80", "https": "443", "ws": "80", "wss": "443", "ftp": "21", "file": ""}

// parseURL parses s against base as WHATWG URLs resolve them: absolute
// URLs need a scheme.
func parseURL(s, base string) (*url.URL, bool) {
	s = strings.TrimSpace(s)
	if base != "" {
		b, ok := parseURL(base, "")
		if !ok {
			return nil, false
		}
		ref, err := url.Parse(s)
		if err != nil {
			return nil, false
		}
		return normalizeURL(b.ResolveReference(ref))
	}
	u, err := url.Parse(s)
	if err != nil || u.Scheme == "" {
		return nil, false
	}
	return normalizeURL(u)
}

// normalizeURL gives u the form WHATWG URLs serialize: its scheme
// lowercase, a special scheme's host as the host parser leaves it (IDNA,
// IPv4 and IPv6 forms), no default port, its path without dot segments,
// and its query's spaces and quotes encoded. It reports false for a host
// that does not parse.
func normalizeURL(u *url.URL) (*url.URL, bool) {
	u.Scheme = strings.ToLower(u.Scheme)
	port, isSpecial := special[u.Scheme]
	if isSpecial && (u.Host != "" || u.Scheme != "file") {
		host, ok := whatwgHost(u.Hostname(), strings.HasPrefix(u.Host, "["))
		if !ok {
			return nil, false
		}
		if p := u.Port(); p != "" && p != port {
			host += ":" + p
		}
		u.Host = host
	}
	if isSpecial && u.Path == "" && u.Opaque == "" {
		u.Path = "/"
	}
	if u.Opaque == "" && u.Path != "" {
		u.Path, u.RawPath = removeDotSegments(u.Path), ""
	}
	u.RawQuery = encodeQuery(u.RawQuery, isSpecial)
	return u, true
}

// whatwgHost parses a special URL's host: an IPv6 address in brackets,
// in its shortest form; an IPv4 address in any of the forms WHATWG
// reads (0x7f.1); else a domain, through IDNA's lookup mapping.
func whatwgHost(h string, bracketed bool) (string, bool) {
	if bracketed {
		a, err := netip.ParseAddr(h)
		if err != nil || !a.Is6() {
			return "", false
		}
		return "[" + a.String() + "]", true
	}
	if d, err := url.PathUnescape(h); err == nil {
		h = d
	}
	ascii, err := idna.Lookup.ToASCII(h)
	if err != nil {
		return "", false
	}
	if ip, ok, isIP := whatwgIPv4(ascii); isIP {
		return ip, ok
	}
	return ascii, ascii != ""
}

// whatwgIPv4 reads host as WHATWG's IPv4 parser does when its last label
// is a number: up to four parts, each decimal, octal (0 first) or hex
// (0x first), the last filling the bytes the others leave. isIP is false
// for a host that is a domain.
func whatwgIPv4(host string) (ip string, ok, isIP bool) {
	parts := strings.Split(strings.TrimSuffix(host, "."), ".")
	number := func(s string) (uint64, bool) {
		base := 10
		switch {
		case strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X"):
			s, base = s[2:], 16
		case len(s) > 1 && s[0] == '0':
			s, base = s[1:], 8
		}
		if s == "" {
			return 0, true
		}
		n, err := strconv.ParseUint(s, base, 64)
		return n, err == nil
	}
	if _, isNum := number(parts[len(parts)-1]); !isNum {
		return "", false, false
	}
	if len(parts) > 4 {
		return "", false, true
	}
	var nums []uint64
	for _, p := range parts {
		n, ok := number(p)
		if !ok || p == "" {
			return "", false, true
		}
		nums = append(nums, n)
	}
	last := nums[len(nums)-1]
	if last >= 1<<(8*(5-len(nums))) {
		return "", false, true
	}
	addr := last
	for i, n := range nums[:len(nums)-1] {
		if n > 255 {
			return "", false, true
		}
		addr += n << (8 * (3 - i))
	}
	return fmt.Sprintf("%d.%d.%d.%d", addr>>24&255, addr>>16&255, addr>>8&255, addr&255), true, true
}

// removeDotSegments resolves a path's . and .. segments, as RFC 3986's
// algorithm does: /a/b/../c is /a/c.
func removeDotSegments(p string) string {
	if !strings.Contains(p, ".") {
		return p
	}
	var out []string
	segs := strings.Split(p, "/")
	for i, s := range segs {
		last := i == len(segs)-1
		switch s {
		case ".":
			if last {
				out = append(out, "")
			}
		case "..":
			if len(out) > 1 {
				out = out[:len(out)-1]
			}
			if last {
				out = append(out, "")
			}
		default:
			out = append(out, s)
		}
	}
	return strings.Join(out, "/")
}

// encodeQuery percent-encodes what WHATWG's query percent-encode set
// holds: controls, space, quotes, <, > and, in a special URL, '; and
// what is not ASCII.
func encodeQuery(q string, special bool) string {
	var b strings.Builder
	for i := 0; i < len(q); i++ {
		c := q[i]
		if c <= 0x20 || c >= 0x7f || c == '"' || c == '<' || c == '>' || special && c == '\'' {
			fmt.Fprintf(&b, "%%%02X", c)
		} else {
			b.WriteByte(c)
		}
	}
	return b.String()
}

// formEncode is application/x-www-form-urlencoded's serializer: spaces
// as +, letters, digits and *-._ as they are, other bytes of the UTF-8
// percent-encoded.
func formEncode(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == ' ':
			b.WriteByte('+')
		case 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9' || strings.IndexByte("*-._", c) >= 0:
			b.WriteByte(c)
		default:
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

func (r *runtime) urlParts(u *url.URL) *goja.Object {
	o := r.vm.NewObject()
	href := u.String()
	_ = o.Set("href", href)
	_ = o.Set("protocol", u.Scheme+":")
	_ = o.Set("username", u.User.Username())
	pass, _ := u.User.Password()
	_ = o.Set("password", pass)
	_ = o.Set("host", u.Host)
	_ = o.Set("hostname", u.Hostname())
	_ = o.Set("port", u.Port())
	path := u.EscapedPath()
	if u.Opaque != "" {
		path = u.Opaque
	}
	_ = o.Set("pathname", path)
	search := ""
	if u.RawQuery != "" {
		search = "?" + u.RawQuery
	}
	_ = o.Set("search", search)
	hash := ""
	if u.Fragment != "" {
		hash = "#" + u.EscapedFragment()
	}
	_ = o.Set("hash", hash)
	origin := "null"
	if _, ok := special[u.Scheme]; ok && u.Scheme != "file" {
		origin = u.Scheme + "://" + u.Host
	}
	_ = o.Set("origin", origin)
	return o
}

// fetch is fetch's request: fetch(url, method, headers, body, redirect,
// done, fail). It runs on a goroutine of its own, answers on the engine
// goroutine, and returns a function that cancels it.
func (r *runtime) fetch(call goja.FunctionCall) goja.Value {
	target, method := arg(call, 0).String(), arg(call, 1).String()
	var headers [][2]string
	for _, h := range r.arrayItems(arg(call, 2)) {
		pair := r.arrayItems(h)
		if len(pair) == 2 {
			headers = append(headers, [2]string{pair[0].String(), pair[1].String()})
		}
	}
	var body []byte
	if b := arg(call, 3); defined(b) {
		body = bytes.Clone(r.bytes(b))
	}
	redirect, done, fail := arg(call, 4).String(), arg(call, 5), arg(call, 6)
	// A request belongs to the app, not to the owner that sent it: only
	// its AbortSignal, or the app's end, cancels it.
	var ctx context.Context
	var cancel context.CancelFunc
	r.cx.Run(func() { ctx, cancel = context.WithCancel(r.cx.Context()) })
	answer := func(fn func()) { r.cx.Post(func() { r.inJS(fn) }) }
	fetcher := fetcherKey.Use(r.cx)
	asset, shipped := r.assets[target]
	go func() {
		var res *fetched
		var err error
		if strings.HasPrefix(target, "asset:") {
			res, err = fetchAsset(target, asset, shipped)
		} else {
			res, err = doFetch(ctx, target, method, headers, body, redirect, fetcher)
		}
		if err != nil {
			if ctx.Err() == nil {
				answer(func() { r.call(fail, r.vm.ToValue(err.Error())) })
			}
			return
		}
		answer(func() {
			o := r.vm.NewObject()
			_ = o.Set("status", res.status)
			_ = o.Set("statusText", res.statusText)
			_ = o.Set("url", res.url)
			_ = o.Set("redirected", res.redirected)
			hs := make([]any, len(res.headers))
			for i, h := range res.headers {
				hs[i] = []any{h[0], h[1]}
			}
			_ = o.Set("headers", hs)
			_ = o.Set("body", r.vm.NewArrayBuffer(res.body))
			r.call(done, o)
		})
	}()
	return r.function(func(goja.FunctionCall) goja.Value {
		cancel()
		return goja.Undefined()
	})
}

type fetched struct {
	status          int
	statusText, url string
	redirected      bool
	headers         [][2]string
	body            []byte
}

var errRedirect = errors.New("redirect mode is set to error")

// doFetch fetches target: http(s), data: and file: URLs.
func doFetch(ctx context.Context, target, method string, headers [][2]string, body []byte, redirect string,
	fetcher Fetcher) (*fetched, error) {
	u, err := url.Parse(target)
	if err != nil {
		return nil, err
	}
	switch u.Scheme {
	case "data":
		return fetchData(target)
	case "file":
		b, err := os.ReadFile(u.Path)
		if err != nil {
			return nil, err
		}
		return &fetched{status: 200, statusText: "OK", url: target, body: b}, nil
	case "http", "https":
	default:
		return nil, errors.New("fetch: unsupported scheme " + u.Scheme)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if body == nil {
		req.Body = http.NoBody
	}
	for _, h := range headers {
		req.Header.Add(h[0], h[1])
	}
	client := &http.Client{}
	if fetcher != nil {
		client.Transport = roundTrip(fetcher)
	}
	redirected := false
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		switch redirect {
		case "error":
			return errRedirect
		case "manual":
			return http.ErrUseLastResponse
		}
		redirected = true
		return nil
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	f := &fetched{status: res.StatusCode, statusText: strings.TrimSpace(strings.TrimPrefix(res.Status, strconv.Itoa(res.StatusCode))),
		url: res.Request.URL.String(), redirected: redirected, body: b}
	for k, vs := range res.Header {
		for _, v := range vs {
			f.headers = append(f.headers, [2]string{strings.ToLower(k), v})
		}
	}
	return f, nil
}

// fetchAsset reads an asset: URL from what the bundle ships.
func fetchAsset(target, data string, shipped bool) (*fetched, error) {
	if !shipped {
		return nil, errors.New("fetch: no asset " + target)
	}
	res, err := fetchData(data)
	if err != nil {
		return nil, err
	}
	res.url = target
	res.headers = append(res.headers, [2]string{"content-length", strconv.Itoa(len(res.body))})
	return res, nil
}

// fetchData reads a data: URL.
func fetchData(target string) (*fetched, error) {
	meta, data, ok := strings.Cut(strings.TrimPrefix(target, "data:"), ",")
	if !ok {
		return nil, errors.New("fetch: invalid data: URL")
	}
	mime, isBase64 := strings.CutSuffix(meta, ";base64")
	var b []byte
	if isBase64 {
		var err error
		if b, err = base64.StdEncoding.DecodeString(data); err != nil {
			return nil, err
		}
	} else {
		s, err := url.PathUnescape(data)
		if err != nil {
			return nil, err
		}
		b = []byte(s)
	}
	if mime == "" {
		mime = "text/plain;charset=US-ASCII"
	}
	return &fetched{status: 200, statusText: "OK", url: target, headers: [][2]string{{"content-type", mime}}, body: b}, nil
}
