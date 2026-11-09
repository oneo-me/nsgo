//go:build ios

package ios

import (
	"os"

	"github.com/ebitengine/purego/objc"

	"github.com/oneo-me/nsgo/internal/platform/apple"
)

// openedURL is the web link that last opened the app (Environment's
// OpenedURL): an App Clip's invocation or a universal link, which UIKit
// hands the scene as an activity of browsing the web, as it connects or
// when the link opens the running app again.
var openedURL string

// takeActivity keeps the link of activity, when it browses the web, and
// reports whether it did.
func takeActivity(activity objc.ID) bool {
	if activity == 0 || apple.GoString(activity.Send(sel("activityType"))) != "NSUserActivityTypeBrowsingWeb" {
		return false
	}
	url := activity.Send(sel("webpageURL"))
	if url == 0 {
		return false
	}
	openedURL = apple.GoString(url.Send(sel("absoluteString")))
	return true
}

// takeConnectionActivities keeps the link a scene's connection options
// open it with. Without one, an App Clip takes _XCAppClipURL's, through
// which Xcode, and nsgo dev -clip, open it in the simulator, where no
// link can.
func takeConnectionActivities(options objc.ID) {
	if options != 0 {
		if activities := options.Send(sel("userActivities")); activities != 0 {
			apple.ForEach(activities, func(a objc.ID) { takeActivity(a) })
		}
	}
	if openedURL == "" && appClip() {
		openedURL = os.Getenv("_XCAppClipURL")
	}
}

// sceneContinueActivity is the scene delegate's
// scene:continueUserActivity:, as a link opens the running app.
func sceneContinueActivity(_ objc.ID, _ objc.SEL, _, activity objc.ID) {
	if takeActivity(activity) {
		runner.refresh()
	}
}

// isAppClip is whether the app runs as an App Clip, whose Info.plist
// holds NSAppClip; 0 until read, then 1 or 2.
var isAppClip uint8

func appClip() bool {
	if isAppClip == 0 {
		isAppClip = 2
		bundle := objc.ID(apple.Class("NSBundle")).Send(sel("mainBundle"))
		if bundle.Send(sel("objectForInfoDictionaryKey:"), apple.NSString("NSAppClip")) != 0 {
			isAppClip = 1
		}
	}
	return isAppClip == 1
}
