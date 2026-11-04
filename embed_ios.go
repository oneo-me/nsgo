//go:build ios && nsgoembed

package nsgo

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/internal/platform/ios"
)

// An embedded iOS build is the framework `nsgo build -embed` makes for a
// host app, which runs UIKit itself. The function the tool adds to the
// app's main package, NSGoStart, calls StartEmbedded; the app's main then
// runs as usual, and its Run hands the app to every NSGoView the host
// makes instead of running UIKit.

var (
	embedOnce sync.Once
	embedded  = make(chan struct{})
)

func run(root App, app func(*core.Scope), o options) {
	ios.Embed(rootOf(root, app), ios.Options{ID: id, Components: o.components, Fonts: o.fonts, Appearance: o.appearance})
	close(embedded)
	select {}
}

// StartEmbedded runs main, the app's own, on a goroutine of its own until
// it calls Run, then gets NSGoView ready, once. Only the code `nsgo build
// -embed` adds to the app calls it, on the host's main thread.
func StartEmbedded(main func()) {
	embedOnce.Do(func() {
		go main()
		select {
		case <-embedded:
		case <-time.After(10 * time.Second):
			panic("nsgo: the app's main has not called nsgo.Run")
		}
		resources := ""
		if exe, err := os.Executable(); err == nil {
			// The host copies the app's resources into its bundle under
			// the name the tool gave them.
			resources = filepath.Join(filepath.Dir(exe), name+"Resources")
		}
		ios.StartEmbedded(resources)
	})
}
