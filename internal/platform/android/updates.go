//go:build android

package android

import (
	"context"
	"slices"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/platform/updates"
)

// updateConfig is how the app Run runs updates. The Play Store installs
// only what it lists, and an app may not install its own APK, so a check
// reads the app's appcast through the runtime's HTTP, and package updater
// offers the newer release's page.
var updateConfig updates.Config

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
