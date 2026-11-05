//go:build linux && !android && arm64

package library

import _ "embed"

// name is the library's file name on the platform.
const name = "libghostty-vt.so"

//go:embed lib/linux-arm64/libghostty-vt.so
var embedded []byte
