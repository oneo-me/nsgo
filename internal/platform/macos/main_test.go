//go:build darwin && !ios

package macos

import (
	"os"
	"testing"
)

// mainCalls carries what tests run on the main thread, where init keeps
// TestMain and AppKit wants its views.
var mainCalls = make(chan func())

func TestMain(m *testing.M) {
	done := make(chan int)
	go func() { done <- m.Run() }()
	for {
		select {
		case fn := <-mainCalls:
			fn()
		case code := <-done:
			os.Exit(code)
		}
	}
}

// onMain runs fn on the main thread and waits for it.
func onMain(fn func()) {
	done := make(chan struct{})
	mainCalls <- func() {
		defer close(done)
		fn()
	}
	<-done
}
