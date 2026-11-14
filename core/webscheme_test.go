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
	for _, scheme := range []string{"", "http", "1app", "bad scheme"} {
		if err := h.tree.root.ServeScheme(scheme, func(string) []byte { return nil }); err == nil {
			t.Errorf("ServeScheme(%q) accepted", scheme)
		}
	}
}
