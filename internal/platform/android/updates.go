//go:build android

package android

import (
	"context"
	"os"
	"slices"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/updates"
)

// updateConfig is how the app Run runs updates. A check of an APK the
// user installed reads the app's appcast through the runtime's HTTP, and
// package updater hands a verified APK to Android's package installer; one
// of an app a store installed checks nothing, as the store updates it.
var updateConfig updates.Config

// InstalledBy names the store that installed the app, which updates it,
// from the installing package the runtime passes in NSGO_INSTALLER
// (updates.AndroidStore).
func InstalledBy() string { return updates.AndroidStore(os.Getenv("NSGO_INSTALLER")) }

// withoutUpdateChecks answers b's checks for updates, which Go carries
// out, and returns b without them, as the runtime knows no such item.
func (s *surface) withoutUpdateChecks(b *driver.MountBatch) *driver.MountBatch {
	check := func(it driver.MountItem) bool { return it.Kind == driver.ItemCheckForUpdates }
	if !slices.ContainsFunc(b.Items, check) {
		return b
	}
	c := updateConfig
	for _, it := range b.Items {
		if check(it) {
			updates.Answer(it.Count, func(ctx context.Context) driver.Update { return updates.Find(ctx, c) }, s.engine.Dispatch)
		}
	}
	out := *b
	out.Items = slices.DeleteFunc(slices.Clone(b.Items), check)
	return &out
}
