//go:build !(js && wasm)

package main

// statsPage returns the site's host, which a native gallery reports as,
// and no referrer.
func statsPage() (host, referrer string, ok bool) {
	return "nsgo.dev", "", true
}
