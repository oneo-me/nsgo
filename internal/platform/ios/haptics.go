//go:build ios

package ios

import (
	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
)

// haptic plays h (driver.ItemHaptic) through UIKit's feedback generators,
// which play nothing on a device without a Taptic Engine or with system
// haptics off; tvOS has none of them.
func haptic(h driver.Haptic) {
	// UIImpactFeedbackStyle's and UINotificationFeedbackType's values.
	impacts := map[driver.Haptic]int{driver.HapticLight: 0, driver.HapticMedium: 1, driver.HapticHeavy: 2,
		driver.HapticSoft: 3, driver.HapticRigid: 4}
	outcomes := map[driver.Haptic]int{driver.HapticSuccess: 0, driver.HapticWarning: 1, driver.HapticError: 2}
	var gen objc.ID
	switch {
	case h == driver.HapticSelection:
		if c := apple.Class("UISelectionFeedbackGenerator"); c != 0 {
			gen = objc.ID(c).Send(selNew)
			gen.Send(sel("selectionChanged"))
		}
	case h >= driver.HapticLight && h <= driver.HapticRigid:
		// initWithStyle: is deprecated from iOS 17.5 for one made for a
		// view, which iOS 15 lacks.
		if c := apple.Class("UIImpactFeedbackGenerator"); c != 0 {
			gen = objc.ID(c).Send(selAlloc).Send(sel("initWithStyle:"), impacts[h])
			gen.Send(sel("impactOccurred"))
		}
	default:
		if c := apple.Class("UINotificationFeedbackGenerator"); c != 0 {
			gen = objc.ID(c).Send(selNew)
			gen.Send(sel("notificationOccurred:"), outcomes[h])
		}
	}
	if gen != 0 {
		gen.Send(selRelease)
	}
}
