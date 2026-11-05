//go:build darwin && !ios && arm64

package library

import _ "embed"

// name is the library's file name on the platform.
const name = "libghostty-vt.dylib"

//go:embed lib/darwin-arm64/libghostty-vt.dylib
var embedded []byte
