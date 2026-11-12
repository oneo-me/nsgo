//go:build js && wasm

package main

import (
	"slices"
	"syscall/js"
)

// statsHosts are the site's hosts, where the web gallery counts its use,
// as the site's Umami script's data-domains: a local server reports
// nothing.
var statsHosts = []string{"nsgo.dev", "oneo-me.github.io", "nsgo.oneo.me"}

// statsPage returns the page's host and the referrer it was opened from,
// and whether the host is the site's.
func statsPage() (host, referrer string, ok bool) {
	host = js.Global().Get("location").Get("hostname").String()
	referrer = js.Global().Get("document").Get("referrer").String()
	return host, referrer, slices.Contains(statsHosts, host)
}
