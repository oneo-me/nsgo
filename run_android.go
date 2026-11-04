//go:build android

package nsgo

import (
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/internal/platform/android"
)

func run(root App, app func(*core.Scope), o options) {
	android.Run(rootOf(root, app), android.Options{Components: o.components, Fonts: o.fonts, Appearance: o.appearance})
}
