//go:build darwin

package apple

import (
	"encoding/json"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

// The values an embedding host's Swift or Objective-C code exchanges with
// the app cross as JSON does: NSDictionary, NSArray, NSString, NSNumber
// and NSNull on one side, Go's decoded JSON on the other.

// JSONObject returns v as Foundation's objects, autoreleased, or 0 for
// nil or a value JSON cannot encode.
func JSONObject(v any) objc.ID {
	if v == nil {
		return 0
	}
	b, err := json.Marshal(v)
	if err != nil {
		return 0
	}
	const fragmentsAllowed = 4
	return objc.ID(Class("NSJSONSerialization")).Send(sel("JSONObjectWithData:options:error:"),
		NSData(b), uint(fragmentsAllowed), uintptr(0))
}

// GoJSON returns Foundation's object o as Go's decoded JSON: maps,
// slices, strings, float64, bool or nil.
func GoJSON(o objc.ID) any {
	if o == 0 || o == objc.ID(Class("NSNull")).Send(sel("null")) {
		return nil
	}
	const fragmentsAllowed = 4
	data := objc.ID(Class("NSJSONSerialization")).Send(sel("dataWithJSONObject:options:error:"),
		o, uint(fragmentsAllowed), uintptr(0))
	var v any
	if json.Unmarshal(Bytes(data), &v) != nil {
		return nil
	}
	return v
}

// CallBlock calls a block another language made, such as a Swift closure
// Objective-C passed NSGo, with up to six object, pointer or integer
// arguments, and returns its result's register. purego's Block.Invoke
// calls only the blocks it made.
func CallBlock(block objc.ID, args ...uintptr) uintptr {
	if block == 0 {
		return 0
	}
	// A block is an object whose self is the literal: its isa, flags,
	// reserved, then its function.
	lit := objc.Send[*blockLiteral](block, sel("self"))
	r, _, _ := purego.SyscallN(lit.invoke, append([]uintptr{uintptr(block)}, args...)...)
	return r
}

type blockLiteral struct {
	isa             uintptr
	flags, reserved int32
	invoke          uintptr
}
