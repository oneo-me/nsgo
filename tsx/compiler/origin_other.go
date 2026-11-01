//go:build nsgodev && !js

package compiler

// pageOrigin is the web page's origin, none elsewhere.
func pageOrigin() string { return "" }
