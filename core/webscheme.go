package core

import (
	"fmt"
	"strings"
)

// ServeScheme registers an app-owned URL scheme for web views created under
// cx. A web view serves the schemes registered before its native view is made;
// the handler runs on the platform's UI thread and returns the response body.
// The response's content type is inferred from the URL and bytes by the native
// backend.
func (cx *Scope) ServeScheme(scheme string, handler func(url string) []byte) error {
	if cx == nil || cx.rt == nil {
		return fmt.Errorf("core: no scope")
	}
	name, err := validWebScheme(scheme)
	if err != nil {
		return err
	}
	if handler == nil {
		return fmt.Errorf("core: web scheme %q has no handler", name)
	}
	if cx.rt.webSchemes == nil {
		cx.rt.webSchemes = make(map[string]func(string) []byte)
	}
	cx.rt.webSchemes[name] = handler
	return nil
}

func validWebScheme(raw string) (string, error) {
	name := strings.ToLower(strings.TrimSuffix(raw, ":"))
	if name == "" {
		return "", fmt.Errorf("core: web scheme is empty")
	}
	if name == "http" || name == "https" || name == "file" || name == "data" || name == "blob" || name == "about" {
		return "", fmt.Errorf("core: web scheme %q is reserved", name)
	}
	for i, r := range name {
		if (r >= 'a' && r <= 'z') || (i > 0 && ((r >= '0' && r <= '9') || r == '+' || r == '-' || r == '.')) {
			continue
		}
		return "", fmt.Errorf("core: invalid web scheme %q", raw)
	}
	return name, nil
}
