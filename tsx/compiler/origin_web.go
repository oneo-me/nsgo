//go:build nsgodev && js

package compiler

import "syscall/js"

// pageOrigin is the origin of the page the app runs in, which the dev
// server serves.
func pageOrigin() string { return js.Global().Get("location").Get("origin").String() }
