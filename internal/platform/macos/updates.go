//go:build darwin && !ios

package macos

import (
	"log"

	"github.com/ebitengine/purego/objc"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/platform/apple"
)

// sparkle is the app's SPUStandardUpdaterController, which startSparkle
// makes when the bundle carries Sparkle.framework and its Info.plist names
// an appcast (SUFeedURL), as nsgo build makes them from nsgo.yaml's
// updates. Sparkle takes the public key (SUPublicEDKey) and its schedule
// (SUEnableAutomaticChecks) from Info.plist too, checks on its own and
// shows its windows. Installing, it quits the app as Quit does and starts
// the new version once this one has exited.
var sparkle objc.ID

// startSparkle starts the app's updater as the app launches.
func startSparkle() {
	main := objc.ID(apple.Class("NSBundle")).Send(sel("mainBundle"))
	if main.Send(sel("objectForInfoDictionaryKey:"), apple.NSString("SUFeedURL")) == 0 {
		return
	}
	path := apple.GoString(main.Send(sel("privateFrameworksPath"))) + "/Sparkle.framework"
	framework := objc.ID(apple.Class("NSBundle")).Send(sel("bundleWithPath:"), apple.NSString(path))
	controller := objc.Class(0)
	if framework != 0 && objc.Send[bool](framework, sel("load")) {
		controller = apple.Class("SPUStandardUpdaterController")
	}
	if controller == 0 {
		log.Printf("nsgo: Info.plist names an appcast but %s does not load: the app checks for no updates", path)
		return
	}
	sparkle = objc.ID(controller).Send(selAlloc).Send(sel("initWithStartingUpdater:updaterDelegate:userDriverDelegate:"),
		true, objc.ID(0), objc.ID(0))
}

// checkForUpdates has Sparkle check now and show what it finds, or in the
// background only a newer version.
func (s *surface) checkForUpdates(background bool, id uint32) {
	ev := driver.Event{Kind: driver.EventUpdate, Count: id, Finished: sparkle != 0}
	switch {
	case sparkle == 0:
		ev.Data = &driver.Update{Err: "updates: this build has no Sparkle: nsgo.yaml configures no updates"}
	case background:
		sparkle.Send(sel("updater")).Send(sel("checkForUpdatesInBackground"))
	default:
		sparkle.Send(sel("checkForUpdates:"), objc.ID(0))
	}
	s.engine.Dispatch(ev)
}
