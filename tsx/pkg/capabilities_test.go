package pkg

import (
	"errors"
	"slices"
	"testing"
)

func TestCapabilitiesTravelInPackages(t *testing.T) {
	k := newKeys(t)
	caps := Capabilities{Network: &Network{Hosts: []string{"api.example.com"}}, Host: []string{"search"}, Clipboard: true}
	data, err := Build(Contents{App: App{ID: "com.example.app", Build: 1}, Source: "mount(1)", Capabilities: caps}, k.release, k.cert)
	if err != nil {
		t.Fatal(err)
	}
	p, err := k.trust().Verify(data)
	if err != nil {
		t.Fatal(err)
	}
	if !p.Capabilities.Allows("network", "https://api.example.com/x", "GET") || !p.Capabilities.Clipboard ||
		p.Capabilities.Allows("network", "https://evil.io/", "GET") || len(p.Ignored) != 0 {
		t.Fatalf("read %+v, ignored %v", p.Capabilities, p.Ignored)
	}
	// Without a declaration nothing is granted.
	bare, err := k.trust().Verify(k.craft(t, map[string]any{}, []byte("mount(1)")))
	if err != nil {
		t.Fatal(err)
	}
	if bare.Capabilities.Allows("network", "https://api.example.com/", "GET") || bare.Capabilities.Clipboard {
		t.Fatal("a package without capabilities was granted some")
	}
	// A newer signer's: what this runtime does not know is not granted.
	newer, err := k.trust().Verify(k.craft(t, map[string]any{"capabilities": map[string]any{
		"clipboard": true,
		"camera":    true,
		"network":   map[string]any{"hosts": []string{"a.com"}, "limit": 3},
		"webview":   map[string]any{"hosts": []string{"b.com"}},
	}}, []byte("mount(1)")))
	if err != nil {
		t.Fatal(err)
	}
	if newer.Capabilities.Network != nil || !newer.Capabilities.Clipboard || newer.Capabilities.WebView == nil ||
		!slices.Equal(newer.Ignored, []string{"capability camera", "capability network"}) {
		t.Fatalf("read %+v, ignored %v", newer.Capabilities, newer.Ignored)
	}
}

func TestParseCapabilitiesStrictly(t *testing.T) {
	for _, bad := range []string{
		`{"camera": true}`,
		`{"network": {"hosts": ["a.com"], "limit": 3}}`,
		`{"network": {"http": [{"host": "a.com", "schemes": ["ftp"]}]}}`,
		`{"network": {"http": [{"host": "a.com", "paths": ["v1"]}]}}`,
		`{"network": {"hosts": ["https://a.com"]}}`,
		`{"host": ["a/../b"]}`,
		`{"clipboard": "yes"}`,
	} {
		if _, _, err := ParseCapabilities([]byte(bad), true); !errors.Is(err, ErrFormat) {
			t.Errorf("%s: %v", bad, err)
		}
	}
	c, _, err := ParseCapabilities([]byte(`{"host": true, "openUrl": true}`), true)
	if err != nil || !c.Allows("host", "anything.at:all", "") || !c.Allows("openUrl", "mailto:a@b.c", "") {
		t.Fatalf("true: %+v %v", c, err)
	}
	// Leniently a rule with a key unknown goes alone.
	c, _, err = ParseCapabilities([]byte(`{"network": {"hosts": ["a.com"], "http": [{"host": "b.com", "new": 1}, {"host": "c.com"}]}}`), false)
	if err != nil || len(c.Network.HTTP) != 1 || c.Network.HTTP[0].Host != "c.com" {
		t.Fatalf("lenient rules: %+v %v", c.Network, err)
	}
}

func TestCapabilitiesAllow(t *testing.T) {
	c := Capabilities{
		Network: &Network{Hosts: []string{"*.cdn.com"}, HTTP: []HTTPRule{
			{Host: "api.com", Methods: []string{"GET"}, Schemes: []string{"https"}, PathPrefixes: []string{"/v1"}},
			{Host: "ws.com", Port: 8443, Paths: []string{"/live"}},
		}},
		WebView: &WebView{Hosts: []string{"docs.com"}},
		OpenURL: &OpenURL{Schemes: []string{"https"}},
		Host:    []string{"player.*", "search"},
	}
	for _, tc := range []struct {
		permission, resource, method string
		want                         bool
	}{
		{"network", "https://img.cdn.com/a.png", "GET", true},
		{"network", "https://cdn.com/a.png", "GET", false},
		{"network", "https://api.com/v1/x?q=1", "get", true},
		{"network", "https://api.com/v1", "GET", true},
		{"network", "https://api.com/v10", "GET", false},
		{"network", "https://api.com/v1/x", "POST", false},
		{"network", "http://api.com/v1/x", "GET", false},
		{"network", "https://api.com@evil.io/v1/", "GET", false},
		{"network", "wss://ws.com:8443/live", "GET", true},
		{"network", "wss://ws.com/live", "GET", false},
		{"network", "file:///etc/hosts", "GET", false},
		{"webview", "https://docs.com/a", "", true},
		{"webview", "https://evil.io/", "", false},
		{"webview", "", "input", false},
		{"openUrl", "https://a.com", "", true},
		{"openUrl", "tel:123", "", false},
		{"host", "player.play", "", true},
		{"host", "search", "", true},
		{"host", "searchAll", "", false},
		{"host", "player..x", "", false},
		{"clipboard", "", "", false},
	} {
		if got := c.Allows(tc.permission, tc.resource, tc.method); got != tc.want {
			t.Errorf("Allows(%s, %s, %s) = %v", tc.permission, tc.resource, tc.method, got)
		}
	}
}
