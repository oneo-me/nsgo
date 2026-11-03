package tsx

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/dop251/goja"

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
		return r.vm.ToValue(url.QueryEscape(arg(call, 0).String()))
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
		return r.urlParts(u)
	})
	set("baseURL", func(goja.FunctionCall) goja.Value { return r.vm.ToValue(r.base) })
	set("fetch", r.fetch)
	return n
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
		u := b.ResolveReference(ref)
		return normalizeURL(u), true
	}
	u, err := url.Parse(s)
	if err != nil || u.Scheme == "" {
		return nil, false
	}
	return normalizeURL(u), true
}

func normalizeURL(u *url.URL) *url.URL {
	u.Scheme = strings.ToLower(u.Scheme)
	if port, ok := special[u.Scheme]; ok {
		u.Host = strings.ToLower(u.Host)
		if u.Port() == port && port != "" {
			u.Host = u.Hostname()
		}
		if u.Path == "" && u.Opaque == "" {
			u.Path = "/"
		}
	}
	return u
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
	go func() {
		res, err := doFetch(ctx, target, method, headers, body, redirect, fetcher)
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
