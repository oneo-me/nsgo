package core

import "github.com/nsgo-dev/nsgo/internal/driver"

// Haptic is feedback the device plays under the user's finger: the
// core's part of mobile.Haptic, which apps use.
type Haptic uint8

// The haptics, as iOS's feedback generators name them: a selection
// changing, an impact of a weight, and a task's outcome.
const (
	// HapticSelection is a selection changing: a picker's value, a
	// segment, a slider's step.
	HapticSelection Haptic = iota + 1
	// HapticLight to HapticRigid are elements colliding, by their weight
	// and give.
	HapticLight
	HapticMedium
	HapticHeavy
	HapticSoft
	HapticRigid
	// HapticSuccess, HapticWarning and HapticError are a task ending so.
	HapticSuccess
	HapticWarning
	HapticError
)

// PlayHaptic is the core's part of mobile.PlayHaptic, which apps use: it
// plays h on iOS and Android, and does nothing elsewhere.
func (cx *Scope) PlayHaptic(h Haptic) {
	if h < HapticSelection || h > HapticError {
		return
	}
	if os := cx.Env().os.GetUntracked(); os != OSIOS && os != OSAndroid {
		return
	}
	if t := cx.tree(); t != nil {
		t.commands = append(t.commands, driver.MountItem{Kind: driver.ItemHaptic, Count: uint32(h)})
		t.changed = true
	}
}
