package core

import (
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

func TestWebViewConfiguresLoadsAndReports(t *testing.T) {
	var ref Ref
	var src *Signal[WebSource]
	var loaded []WebNavigation
	var messages []string
	h := newHost(t, Size{Width: 300, Height: 400}, func(cx *Scope) Node {
		src = NewSignal(cx, WebSrc("https://example.com"))
		return WebView{
			SourceFunc:         src.Get,
			InjectedJavaScript: "1",
			Headers:            map[string]string{"X-Test": "one"},
			Ref:                &ref,
			TestID:             "web",
			Style:              Style{Width: Pt(300), Height: Pt(200)},
			OnLoad:             func(n WebNavigation) { loaded = append(loaded, n) },
			OnMessage:          func(d string) { messages = append(messages, d) },
		}
	})
	b := h.commit()
	var cfg *driver.WebConfig
	var mask driver.EventMask
	for _, it := range b.Items {
		switch it.Kind {
		case driver.ItemWeb:
			cfg = it.Web
		case driver.ItemEvents:
			if it.ID == h.find("web").id {
				mask = it.Events
			}
		}
	}
	if n := h.find("web"); n.kind != driver.KindWeb {
		t.Fatalf("kind %v, want KindWeb", n.kind)
	}
	if cfg == nil || cfg.Source.Src != "https://example.com" || cfg.InjectedJavaScript != "1" || cfg.Headers["X-Test"] != "one" {
		t.Fatalf("config %+v", cfg)
	}
	if !mask.Has(driver.EventLoad) || !mask.Has(driver.EventMessage) || mask.Has(driver.EventLoadStart) {
		t.Fatalf("mask %b", mask)
	}
	if k := kinds(h.commit()); k[driver.ItemWeb] != 0 {
		t.Fatalf("resent an unchanged config: %v", k)
	}

	src.Set(WebHTML("<p>hi</p>", ""))
	b = h.commit()
	if k := kinds(b); k[driver.ItemWeb] != 1 {
		t.Fatalf("source change sent %v", k)
	}

	ref.PostMessage("ping")
	ref.GoBack()
	b = h.commit()
	var cmds []driver.WebCommand
	for _, it := range b.Items {
		if it.Kind == driver.ItemWebCommand {
			cmds = append(cmds, it.WebCommand)
			if it.WebCommand == driver.WebPostMessage && it.Value != "ping" {
				t.Fatalf("message %q", it.Value)
			}
		}
	}
	if len(cmds) != 2 || cmds[0] != driver.WebPostMessage || cmds[1] != driver.WebGoBack {
		t.Fatalf("commands %v", cmds)
	}

	id := h.find("web").id
	h.tree.Dispatch(driver.Event{Kind: driver.EventLoad, ID: id, Navigation: &WebNavigation{URL: "about:srcdoc", CanGoBack: true}})
	h.tree.Dispatch(driver.Event{Kind: driver.EventMessage, ID: id, Text: "pong"})
	if len(loaded) != 1 || !loaded[0].CanGoBack || len(messages) != 1 || messages[0] != "pong" {
		t.Fatalf("loaded %v, messages %v", loaded, messages)
	}
}

func TestJSString(t *testing.T) {
	for in, want := range map[string]string{
		`a"b\c` + "\nd": `"a\"b\\c\nd"`,
		"\x01":          `"\u0001"`,
		"\u2028é":       `"\u2028é"`,
	} {
		if got := driver.JSString(in); got != want {
			t.Errorf("JSString(%q) = %s, want %s", in, got, want)
		}
	}
}
