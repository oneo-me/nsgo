// Package compiler compiles TSX apps into the scripts the tsx package
// runs, as NSH's bundler does (nsh-bundler): esbuild strips TypeScript,
// lowers syntax goja lacks and bundles the app's modules into one script,
// and jsx.go compiles the JSX into calls of the renderer, as NSH's
// compiler does. Imports of NSGo's modules (nsgo, nsgo/*, nsgo:rt) stay,
// as require calls the runtime answers. No Node is needed.
package compiler
