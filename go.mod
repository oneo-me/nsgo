module github.com/oneo-me/nsgo

go 1.26.0

require github.com/ebitengine/purego v0.11.2

require (
	github.com/alecthomas/chroma/v2 v2.27.0
	github.com/andybalholm/brotli v1.2.6
	github.com/dop251/goja v0.0.0-20261027234916-5f913d1693d9
	github.com/evanw/esbuild v0.28.2
	github.com/go-sourcemap/sourcemap v2.1.3+incompatible
	github.com/yuin/goldmark v1.8.6
	go.yaml.in/yaml/v3 v3.0.6
	golang.org/x/image v0.45.0
	golang.org/x/net v0.58.0
	golang.org/x/text v0.43.0
	rsc.io/qr v0.2.0
)

require (
	github.com/dlclark/regexp2/v2 v2.11.0 // indirect
	github.com/google/pprof v0.0.0-20230207041349-798e818bf904 // indirect
	golang.org/x/sys v0.47.0 // indirect
)

tool github.com/oneo-me/nsgo/cmd/nsgo
