//go:build nsgodev && !(js && wasm)

package nsgo

// pageOrigin is where a web page was served from: no page runs the app.
func pageOrigin() string { return "" }
