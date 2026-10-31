package imagedecode

import (
	"context"
	"runtime"
)

// slots bounds how many images decode at once to the processors, as
// NSH's decode on a dispatch queue the system sizes to them. A decode in
// a native library holds its thread, so Go starts another for each one
// waiting, and each holds its source and its pixels: a fast scroll down a
// grid of pictures ran 25 at once.
var slots = make(chan struct{}, runtime.NumCPU())

// Wait waits for a slot to decode an image in, every backend's decoder
// or this package's, and returns the function that gives it back; it
// returns ctx's error instead if ctx ends first.
func Wait(ctx context.Context) (done func(), err error) {
	select {
	case slots <- struct{}{}:
		return func() { <-slots }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
