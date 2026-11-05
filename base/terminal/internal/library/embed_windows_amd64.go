//go:build windows && amd64

package library

import _ "embed"

// name is the library's file name on the platform.
const name = "ghostty-vt.dll"

//go:embed lib/windows-amd64/ghostty-vt.dll
var embedded []byte
