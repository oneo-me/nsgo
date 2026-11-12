//go:build ios

package ios

import (
	"context"

	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
	"github.com/nsgo-dev/nsgo/internal/platform/updates"
)

// updateConfig is how the app Run runs updates. The App Store updates iOS
// apps: a check asks it for the app's version in the user's country, and
// package updater offers its page, which the App Store opens.
var updateConfig updates.Config

// InstalledBy is "TestFlight" for a build TestFlight installed, or a
// development build on a device, whose receipt is the App Store's
// sandbox's: the store lists none of their versions, so they check
// nothing. It is "" for the App Store's own, and in the simulator, whose
// builds ask the App Store as its own would.
func InstalledBy() string {
	url := objc.ID(apple.Class("NSBundle")).Send(sel("mainBundle")).Send(sel("appStoreReceiptURL"))
	if url != 0 && apple.GoString(url.Send(sel("lastPathComponent"))) == "sandboxReceipt" {
		return "TestFlight"
	}
	return ""
}

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
