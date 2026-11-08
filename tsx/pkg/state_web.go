//go:build js

package pkg

import (
	"fmt"
	"io/fs"
	"syscall/js"
)

// stateKey is the localStorage key of dir's state.
func stateKey(dir string) string { return "nsgo.trust:" + dir }

func readState(dir string) ([]byte, error) {
	v := js.Global().Get("localStorage").Call("getItem", stateKey(dir))
	if v.IsNull() {
		return nil, fs.ErrNotExist
	}
	return []byte(v.String()), nil
}

// writeState sets the key, which the browser writes whole.
func writeState(dir string, data []byte) (err error) {
	defer func() {
		// A full or refused storage throws.
		if r := recover(); r != nil {
			err = fmt.Errorf("localStorage: %v", r)
		}
	}()
	js.Global().Get("localStorage").Call("setItem", stateKey(dir), string(data))
	return nil
}
