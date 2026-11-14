package core

import "github.com/nsgo-dev/nsgo/internal/driver"

// BiometricKind says which enrolled biometric the system can use.
type BiometricKind uint8

const (
	BiometricNone BiometricKind = iota
	BiometricFace
	BiometricFingerprint
)

// BiometricResult is the result of AuthenticateBiometric.
type BiometricResult uint8

const (
	BiometricSuccess BiometricResult = iota + 1
	BiometricFailure
	BiometricCancelled
	BiometricUnavailable
)

// BiometricAvailability answers whether an enrolled biometric is available,
// and which kind it is. It does not show a system prompt.
func (cx *Scope) BiometricAvailability(done func(BiometricKind)) {
	if done == nil {
		return
	}
	if t := cx.tree(); t != nil && !cx.denied(AccessRequest{Permission: AccessBiometrics}) {
		t.request(cx, driver.MountItem{Kind: driver.ItemBiometricAvailability}, func(ev driver.Event) {
			done(BiometricKind(ev.Index))
		})
		return
	}
	done(BiometricNone)
}

// AuthenticateBiometric asks the user to authenticate with reason. It never
// falls back to a passcode; the app chooses that separately.
func (cx *Scope) AuthenticateBiometric(reason string, done func(BiometricResult)) {
	answer := func(result BiometricResult) {
		if done != nil {
			done(result)
		}
	}
	if t := cx.tree(); t != nil && !cx.denied(AccessRequest{Permission: AccessBiometrics}) {
		t.request(cx, driver.MountItem{Kind: driver.ItemBiometricAuthenticate, Value: reason}, func(ev driver.Event) {
			answer(BiometricResult(ev.Index))
		})
		return
	}
	answer(BiometricUnavailable)
}
