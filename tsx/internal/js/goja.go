//go:build !js

// Package js is the JavaScript engine package tsx runs apps on: goja on
// every target but the web, where the page's own engine runs them
// (browser.go). Its API is goja's, so on goja it is goja's types under
// other names, and the browser's implements the part tsx uses.
package js

import "github.com/dop251/goja"

type (
	Value              = goja.Value
	Object             = goja.Object
	Runtime            = goja.Runtime
	FunctionCall       = goja.FunctionCall
	ConstructorCall    = goja.ConstructorCall
	Exception          = goja.Exception
	Promise            = goja.Promise
	PromiseState       = goja.PromiseState
	ArrayBuffer        = goja.ArrayBuffer
	Symbol             = goja.Symbol
	Callable           = goja.Callable
	Program            = goja.Program
	InterruptedError   = goja.InterruptedError
	StackOverflowError = goja.StackOverflowError
)

const (
	PromiseStatePending   = goja.PromiseStatePending
	PromiseStateFulfilled = goja.PromiseStateFulfilled
	PromiseStateRejected  = goja.PromiseStateRejected

	FLAG_TRUE  = goja.FLAG_TRUE
	FLAG_FALSE = goja.FLAG_FALSE
)

// Browser reports whether apps run on the page's engine.
const Browser = false

var (
	New                  = goja.New
	Undefined            = goja.Undefined
	Null                 = goja.Null
	IsUndefined          = goja.IsUndefined
	IsNull               = goja.IsNull
	AssertFunction       = goja.AssertFunction
	AssertConstructor    = goja.AssertConstructor
	NewSymbol            = goja.NewSymbol
	MustCompile          = goja.MustCompile
	UncapFieldNameMapper = goja.UncapFieldNameMapper
)

// Caller is the line of JS code r runs now, through the source maps the code carries, as file and line; ok is false while r runs none.
func Caller(r *Runtime) (file string, line int, ok bool) {
	for _, f := range r.CaptureCallStack(16, nil) {
		if p := f.Position(); p.Filename != "" && p.Line > 0 {
			return p.Filename, p.Line, true
		}
	}
	return "", 0, false
}
