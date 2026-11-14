package core

import (
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

func TestServeScheme(t *testing.T) {
	h := newHost(t, Size{Width: 100, Height: 100}, func(cx *Scope) Node {
		if err := cx.ServeScheme("App:", func(string) []byte { return []byte("ok") }); err != nil {
			t.Fatal(err)
		}
		return WebView{Source: WebSrc("app://index.html"), TestID: "web"}
	})
	b := h.commit()
	var got *driver.WebConfig
	for _, item := range b.Items {
		if item.Kind == driver.ItemWeb {
			got = item.Web
		}
	}
	if got == nil || got.SchemeHandlers["app"] == nil || string(got.SchemeHandlers["app"]("app://index.html")) != "ok" {
		t.Fatalf("scheme handlers %+v", got)
	}
	for _, scheme := range []string{"", "http", "1app", "bad scheme", "WS", "wss:", "javascript", "ftp", "applewebdata", "webkit-fake-url", "blob", "about"} {
		if err := h.tree.root.ServeScheme(scheme, func(string) []byte { return nil }); err == nil {
			t.Errorf("ServeScheme(%q) accepted", scheme)
		}
	}
}

func TestServeSchemeStaysInItsSubtree(t *testing.T) {
	answer := func(body string) func(string) []byte { return func(string) []byte { return []byte(body) } }
	h := newHost(t, Size{Width: 100, Height: 100}, func(cx *Scope) Node {
		if err := cx.ServeScheme("app", answer("outer")); err != nil {
			t.Fatal(err)
		}
		return View{Children: []Node{
			Func(func(cx *Scope) Node {
				if err := cx.ServeScheme("app", answer("inner")); err != nil {
					t.Fatal(err)
				}
				if err := cx.ServeScheme("own", answer("own")); err != nil {
					t.Fatal(err)
				}
				return WebView{Source: WebSrc("own://index.html"), TestID: "served"}
			}),
			Func(func(cx *Scope) Node {
				cx.ProvideAccess(hostsAccess{})
				return WebView{Source: WebSrc("own://index.html"), TestID: "sibling"}
			}),
		}}
	})
	configs := map[driver.NodeID]*driver.WebConfig{}
	for _, item := range h.commit().Items {
		if item.Kind == driver.ItemWeb {
			configs[item.ID] = item.Web
		}
	}
	served, sibling := configs[h.find("served").id], configs[h.find("sibling").id]
	if served == nil || sibling == nil {
		t.Fatalf("web configs %v", configs)
	}
	if got := string(served.SchemeHandlers["app"]("app://x")); got != "inner" || served.SchemeHandlers["own"] == nil {
		t.Errorf("the subtree's schemes: app answers %q, own %v", got, served.SchemeHandlers["own"] != nil)
	}
	if got := string(sibling.SchemeHandlers["app"]("app://x")); got != "outer" || sibling.SchemeHandlers["own"] != nil {
		t.Errorf("a sibling's schemes: app answers %q, own %v", got, sibling.SchemeHandlers["own"] != nil)
	}
	// A restricted sibling may not load a scheme only another subtree serves.
	if sibling.Source.Src != "" || sibling.Navigable("own://index.html") {
		t.Errorf("a sibling loads another subtree's scheme: %+v", sibling.Source)
	}
}

func TestServeSchemeStaysInItsWindow(t *testing.T) {
	w := newWindows(t, func(cx *Scope) {
		cx.SetMainView(func(cx *Scope) Node {
			if err := cx.ServeScheme("app", func(string) []byte { return []byte("ok") }); err != nil {
				t.Fatal(err)
			}
			return WebView{Source: WebSrc("app://index.html")}
		})
		cx.OpenWindow(WindowOptions{Title: "Other"}, func(cx *Scope) Node {
			return WebView{Source: WebSrc("app://index.html")}
		})
	})
	main := w.take(1, driver.ItemWeb)
	other := w.take(2, driver.ItemWeb)
	if len(main) != 1 || len(other) != 1 {
		t.Fatalf("web items %d and %d", len(main), len(other))
	}
	if main[0].Web.SchemeHandlers["app"] == nil {
		t.Error("the main window's web view does not serve its scheme")
	}
	if schemes := other[0].Web.SchemeHandlers; schemes != nil {
		t.Errorf("another window's web view serves %v", schemes)
	}
}
