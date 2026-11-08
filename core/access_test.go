package core

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/oneo-me/nsgo/internal/driver"
)

// hostsAccess allows network and web view requests to hosts, the host
// handlers named, and nothing else.
type hostsAccess struct {
	hosts    []string
	handlers []string
}

func (a hostsAccess) Allows(req AccessRequest) bool {
	switch req.Permission {
	case AccessNetwork, AccessWebView:
		host := strings.TrimPrefix(strings.TrimPrefix(req.Resource, "https://"), "http://")
		host, _, _ = strings.Cut(host, "/")
		return req.Method != AccessInput && driver.AllowsHost(a.hosts, host)
	case AccessHost:
		return slices.Contains(a.handlers, req.Resource)
	}
	return false
}

func (a hostsAccess) WebHosts() []string { return a.hosts }

func TestAccessHoldsWhatItsOwnerRenders(t *testing.T) {
	var failed []string
	onError := func(err error) { failed = append(failed, err.Error()) }
	h := newHost(t, Size{Width: 200, Height: 200}, func(cx *Scope) Node {
		held := Func(func(cx *Scope) Node {
			cx.ProvideAccess(hostsAccess{hosts: []string{"ok.com", "*.cdn.com"}})
			return View{Children: []Node{
				Image{Source: Src("https://ok.com/a.png"), TestID: "ok"},
				Image{Source: Src("https://evil.io/a.png"), OnError: onError, TestID: "evil"},
				Image{Source: Src("https://ok.com@evil.io/a.png"), TestID: "userinfo"},
				Image{Source: Src("file:///etc/passwd"), TestID: "file"},
				Image{Source: ImageSource{URI: "https://evil.io/b.png", Data: []byte{1}}, TestID: "bytes"},
				Image{Source: Src("data:image/png;base64,AA=="), TestID: "data"},
				Video{Source: "https://img.cdn.com/v.mp4", TestID: "video"},
				WebView{Source: WebSrc("https://evil.io/"), OnError: onError, TestID: "web"},
			}}
		})
		return View{Children: []Node{held, Image{Source: Src("https://evil.io/host.png"), TestID: "host"}}}
	})
	b := h.commit()
	uris := map[string]string{}
	var web *driver.WebConfig
	for _, it := range b.Items {
		switch it.Kind {
		case driver.ItemImage:
			uris[h.tree.nodes[it.ID].props.TestID] = it.Image.Source.URI
		case driver.ItemVideo:
			uris["video"] = it.Video.Source
		case driver.ItemWeb:
			web = it.Web
		}
	}
	want := map[string]string{"ok": "https://ok.com/a.png", "evil": "", "userinfo": "", "file": "",
		"bytes": "https://evil.io/b.png", "data": "data:image/png;base64,AA==", "video": "https://img.cdn.com/v.mp4",
		"host": "https://evil.io/host.png"}
	for id, uri := range want {
		if uris[id] != uri {
			t.Errorf("%s loads %q, want %q", id, uris[id], uri)
		}
	}
	if web == nil || web.Source.Src != "" || !web.Restricted || web.Hosts != "ok.com *.cdn.com" {
		t.Fatalf("the web view is sent %+v", web)
	}
	h.drain()
	h.commit()
	h.drain()
	if len(failed) != 2 || !strings.Contains(failed[0], "evil.io") {
		t.Fatalf("OnError heard %q, once for each refusal", failed)
	}
}

func TestAccessRefusesServices(t *testing.T) {
	var cx *Scope
	h := newHost(t, Size{Width: 100, Height: 100}, func(c *Scope) Node {
		cx = c.Child()
		cx.Run(func() { cx.ProvideAccess(hostsAccess{}) })
		return View{}
	})
	h.commit()
	var opened []bool
	cx.OpenURL("https://example.com", func(ok bool) { opened = append(opened, ok) })
	cx.SetStatusBar(StatusBarSetting{Hidden: ToggleOn})
	err := cx.CheckAccess(AccessRequest{Permission: AccessClipboard})
	var coded *Error
	if !errors.As(err, &coded) || coded.Code != CodeAccessDenied || coded.Permission != AccessClipboard {
		t.Fatalf("CheckAccess = %v", err)
	}
	b := h.commit()
	for _, it := range b.Items {
		if it.Kind == driver.ItemOpenURL {
			t.Fatal("a refused URL reached the platform")
		}
	}
	if !slices.Equal(opened, []bool{false}) || h.tree.status.base.Hidden != ToggleInherit {
		t.Fatalf("opened %v, status bar %+v", opened, h.tree.status.base)
	}
	if h.root.CheckAccess(AccessRequest{Permission: AccessClipboard}) != nil {
		t.Fatal("the app's own code was refused")
	}
}

func TestProvideHostGivesASubtreeItsOwnChannel(t *testing.T) {
	app, plugin := NewHost(), NewHost()
	app.Handle("secret", func(c *HostCall) { c.Resolve("the app's") })
	plugin.Handle("greet", func(c *HostCall) { c.Resolve("hi " + c.Data.(string)) })
	plugin.Handle("other", func(c *HostCall) { c.Resolve("refused") })
	plugin.SetParams(map[string]any{"who": "plugin"})
	var inner, bare *Scope
	h := newEmbedded(t, app, func(cx *Scope) Node {
		inner, bare = cx.Child(), cx.Child()
		inner.Run(func() {
			inner.ProvideHost(plugin)
			inner.ProvideAccess(hostsAccess{handlers: []string{"greet"}})
		})
		bare.Run(func() { bare.ProvideHost(nil) })
		return View{}
	})
	var got []string
	answer := func(v any, err error) {
		if err != nil {
			got = append(got, "error "+ErrorCode(err))
		} else {
			got = append(got, v.(string))
		}
	}
	inner.CallHost("greet", "Ada", answer)
	inner.CallHost("other", nil, answer)
	inner.CallHost("secret", nil, answer)
	bare.CallHost("secret", nil, answer)
	h.root.CallHost("secret", nil, answer)
	h.drain()
	want := []string{"hi Ada", "error " + CodeAccessDenied, "error " + CodeAccessDenied, "error " + CodeUnavailable, "the app's"}
	if !slices.Equal(got, want) {
		t.Fatalf("answers %q, want %q", got, want)
	}
	if inner.HostParams()["who"] != "plugin" || bare.HasHost() || len(h.root.HostParams()) != 0 {
		t.Fatal("the parameters are not the subtree's host's")
	}
	var heard []any
	inner.Run(func() { inner.OnHostEvent("ping", func(d any) { heard = append(heard, d) }) })
	plugin.Send("ping", 1)
	app.Send("ping", 2)
	h.drain()
	if !slices.Equal(heard, []any{float64(1)}) {
		t.Fatalf("the subtree heard %v", heard)
	}
	inner.Dispose()
	if plugin.app != nil {
		t.Fatal("the host stayed attached to a disposed subtree")
	}
}

func TestProvideStoragePartitions(t *testing.T) {
	b := &recorder{kept: map[string]string{"app": "1"}}
	g := newGraph(NewStore(b))
	app := g.root.Storage()
	one, two := g.root.Child(), g.root.Child()
	one.ProvideStorage("one", 10)
	two.ProvideStorage("two", 0)
	s1, s2 := one.Storage(), two.Storage()
	if err := s1.SetItem("k", "v1"); err != nil {
		t.Fatal(err)
	}
	s2.SetItem("k", "v2")
	if v, _ := s1.GetItem("k"); v != "v1" {
		t.Fatalf("one reads %q", v)
	}
	if _, ok := s1.GetItem("app"); ok {
		t.Fatal("a partition read the app's entry")
	}
	if k := app.Keys(); !slices.Equal(k, []string{"app"}) {
		t.Fatalf("the app lists %v", k)
	}
	if k := s2.Keys(); !slices.Equal(k, []string{"k"}) {
		t.Fatalf("two lists %v", k)
	}
	var full *StorageFullError
	if err := s1.SetItem("big", "0123456789"); !errors.As(err, &full) || full.Quota != 10 {
		t.Fatalf("over its quota: %v", err)
	}
	app.Clear()
	if v, _ := s2.GetItem("k"); v != "v2" {
		t.Fatal("the app's Clear cleared a partition")
	}
	s1.Clear()
	if _, ok := s1.GetItem("k"); ok || len(s2.Keys()) != 1 {
		t.Fatal("a partition's Clear did not clear only its own")
	}
	for _, c := range b.changes {
		if c.Op == driver.StorageClear {
			t.Fatal("the backend was cleared whole")
		}
	}
}

func TestWebConfigNavigable(t *testing.T) {
	c := driver.WebConfig{Restricted: true, Hosts: "ok.com *.cdn.com"}
	for url, want := range map[string]bool{
		"https://ok.com/a": true, "http://OK.com": true, "https://a.cdn.com/x": true, "https://cdn.com/": false,
		"https://evil.io": false, "https://ok.com@evil.io/": false, "about:blank": true, "data:text/html,x": true,
		"file:///etc/hosts": false, "mailto:a@ok.com": false,
	} {
		if got := c.Navigable(url); got != want {
			t.Errorf("Navigable(%q) = %v", url, got)
		}
	}
	if got := driver.IntersectHosts([]string{"*.example.com", "a.com"}, []string{"api.example.com", "*"}); !slices.Equal(got, []string{"api.example.com", "*.example.com", "a.com"}) {
		t.Fatalf("IntersectHosts = %v", got)
	}
}
