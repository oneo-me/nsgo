package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nsgo-dev/nsgo/tsx/pkg"
)

// capabilities reads nsgo.yaml's capabilities strictly, nil when it has
// none. YAML's keys are snake_case (open_url, status_bar, path_prefixes),
// the package's camelCase.
func (p *project) capabilities() (*pkg.Capabilities, error) {
	if p.Capabilities.IsZero() {
		return nil, nil
	}
	var v any
	if err := p.Capabilities.Decode(&v); err != nil {
		return nil, fmt.Errorf("nsgo.yaml: capabilities: %w", err)
	}
	data, err := json.Marshal(camelKeys(v))
	if err != nil {
		return nil, fmt.Errorf("nsgo.yaml: capabilities: %w", err)
	}
	c, _, err := pkg.ParseCapabilities(data, true)
	if err != nil {
		return nil, fmt.Errorf("nsgo.yaml: %w", err)
	}
	return &c, nil
}

// camelKeys is v with its maps' snake_case keys in camelCase.
func camelKeys(v any) any {
	switch v := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for k, x := range v {
			parts := strings.Split(k, "_")
			for i := 1; i < len(parts); i++ {
				if parts[i] != "" {
					parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
				}
			}
			out[strings.Join(parts, "")] = camelKeys(x)
		}
		return out
	case []any:
		for i, x := range v {
			v[i] = camelKeys(x)
		}
	}
	return v
}

// describeCapabilities lists what c grants, a line each.
func describeCapabilities(c pkg.Capabilities) []string {
	var out []string
	if n := c.Network; n != nil {
		for _, h := range n.Hosts {
			out = append(out, "network: "+h)
		}
		for _, r := range n.HTTP {
			line := "network: " + r.Host
			if len(r.Methods) > 0 {
				line += " " + strings.Join(r.Methods, ",")
			}
			if len(r.PathPrefixes)+len(r.Paths) > 0 {
				line += " " + strings.Join(append(append([]string(nil), r.Paths...), r.PathPrefixes...), ",")
			}
			out = append(out, line)
		}
	}
	if w := c.WebView; w != nil {
		line := "webview: " + strings.Join(w.Hosts, ", ")
		if w.Input {
			line += ", with trusted input"
		}
		out = append(out, line)
	}
	if o := c.OpenURL; o != nil {
		schemes := "any URL"
		if len(o.Schemes) > 0 {
			schemes = strings.Join(o.Schemes, ", ")
		}
		out = append(out, "open_url: "+schemes)
	}
	for _, b := range []struct {
		on   bool
		name string
	}{{c.Clipboard, "clipboard"}, {c.Picker, "picker"}, {c.Speech, "speech"}, {c.StatusBar, "status_bar"}} {
		if b.on {
			out = append(out, b.name)
		}
	}
	if len(c.Host) > 0 {
		out = append(out, "host: "+strings.Join(c.Host, ", "))
	}
	return out
}
