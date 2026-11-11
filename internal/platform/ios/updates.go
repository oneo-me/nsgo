//go:build ios

package ios

import (
	"context"

	"github.com/ebitengine/purego/objc"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/platform/apple"
	"github.com/oneo-me/nsgo/internal/platform/updates"
)

// updateConfig is how the app Run runs updates. The App Store updates iOS
// apps: a check asks it for the app's version in the user's country, and
// package updater offers its page, which the App Store opens.
var updateConfig updates.Config

// checkForUpdates asks the App Store off the main thread and answers with
// what it found. The bundle's identifier and version are the ones the
// store lists, whatever signing gave them.
func (s *surface) checkForUpdates(id uint32) {
	c := updateConfig
	bundle := objc.ID(apple.Class("NSBundle")).Send(sel("mainBundle"))
	if v := bundle.Send(sel("bundleIdentifier")); v != 0 {
		c.ID = apple.GoString(v)
	}
	if v := bundle.Send(sel("objectForInfoDictionaryKey:"), apple.NSString("CFBundleShortVersionString")); v != 0 {
		c.Version = apple.GoString(v)
	}
	country := ""
	if v := objc.ID(apple.Class("NSLocale")).Send(sel("currentLocale")).Send(sel("countryCode")); v != 0 {
		country = apple.GoString(v)
	}
	updates.Answer(id, func(ctx context.Context) driver.Update { return updates.FindInAppStore(ctx, c, country) }, s.engine.Dispatch)
}
