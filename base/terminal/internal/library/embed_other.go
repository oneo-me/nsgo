//go:build !((darwin && !ios && (arm64 || amd64)) || (linux && !android && (arm64 || amd64)) || (windows && (arm64 || amd64)))

package library

// name is the library's file name beside the executable; embedded is
// empty where no build is embedded, iOS's being in its app.
const name = ""

var embedded []byte
