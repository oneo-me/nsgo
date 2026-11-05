//go:build linux && !android && amd64

package library

import _ "embed"

// name is the library's file name on the platform.
const name = "libghostty-vt.so"

//go:embed lib/linux-amd64/libghostty-vt.so
var embedded []byte
