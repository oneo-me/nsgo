//go:build linux && !android

package linux

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/updates"
)

// updateConfig is how the app Run runs updates. Linux has no updater of
// its own: a check reads the app's appcast, and package updater hands a
// verified Debian or Arch package to the user's package manager.
var updateConfig updates.Config

func init() { driver.UpdateInstall = installUpdate }

func installUpdate(path string, done func(error)) {
	go func() {
		var cmd *exec.Cmd
		switch filepath.Ext(path) {
		case ".deb":
			cmd = exec.Command("pkexec", "dpkg", "--install", path)
		case ".gz":
			cmd = exec.Command("pkexec", "pacman", "--upgrade", "--noconfirm", path)
		default:
			done(fmt.Errorf("updater: unsupported Linux package %q", filepath.Base(path)))
			return
		}
		err := cmd.Run()
		done(err)
	}()
}

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
