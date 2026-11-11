//go:build linux && !android

package linux

import (
	"context"
	"os"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/platform/updates"
)

// updateConfig is how the app Run runs updates. Linux has no updater of
// its own: a check reads the app's appcast, and package updater offers
// the newer release's page, as the user installs packages.
var updateConfig updates.Config

// InstalledBy names the sandbox whose store installed the app and updates
// it, Flatpak or Snap, as they mark their apps' environments; "" for any
// other install, a package manager's included, which nsgo.yaml's
// updates.channel tells apart.
func InstalledBy() string {
	if _, err := os.Stat("/.flatpak-info"); err == nil {
		return "Flatpak"
	}
	if os.Getenv("SNAP") != "" {
		return "Snap"
	}
	return ""
}

// checkForUpdates reads the appcast off the main thread and answers with
// what it found.
func (w *window) checkForUpdates(id uint32) {
	c := updateConfig
	updates.Answer(id, func(ctx context.Context) driver.Update { return updates.Find(ctx, c) }, w.engine.Dispatch)
}
