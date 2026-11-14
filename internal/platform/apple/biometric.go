//go:build darwin

package apple

import (
	"log"
	"sync"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

const localAuthenticationPath = "/System/Library/Frameworks/LocalAuthentication.framework/LocalAuthentication"

var loadLocalAuthentication = sync.OnceValue(func() bool {
	if _, err := purego.Dlopen(localAuthenticationPath, purego.RTLD_NOW|purego.RTLD_GLOBAL); err != nil {
		return false
	}
	return Class("LAContext") != 0
})

// HandleBiometric carries out a biometric request on the main thread and
// dispatches its answer to the engine.
func HandleBiometric(it *driver.MountItem, dispatch func(driver.Event)) {
	id := it.Count
	if !loadLocalAuthentication() {
		dispatch(driver.Event{Kind: driver.EventBiometric, Count: id, Index: driver.BiometricUnavailable})
		return
	}
	ctx := objc.ID(Class("LAContext")).Send(sel("new"))
	if ctx == 0 {
		dispatch(driver.Event{Kind: driver.EventBiometric, Count: id, Index: driver.BiometricUnavailable})
		return
	}
	if it.Kind == driver.ItemBiometricAvailability {
		defer ctx.Send(selRelease)
		if !objc.Send[bool](ctx, sel("canEvaluatePolicy:error:"), 1, objc.ID(0)) {
			dispatch(driver.Event{Kind: driver.EventBiometric, Count: id, Index: driver.BiometricNone})
			return
		}
		kind := driver.BiometricNone
		switch objc.Send[int](ctx, sel("biometryType")) {
		case 1: // LABiometryTypeTouchID
			kind = driver.BiometricFingerprint
		case 2: // LABiometryTypeFaceID
			kind = driver.BiometricFace
		}
		dispatch(driver.Event{Kind: driver.EventBiometric, Count: id, Index: kind})
		return
	}

	if !DeclaresUsage("NSFaceIDUsageDescription") && objc.Send[int](ctx, sel("biometryType")) == 2 {
		log.Printf("nsgo: biometric authentication needs NSFaceIDUsageDescription in the app's Info.plist (nsgo.yaml's apple.info)")
		ctx.Send(selRelease)
		dispatch(driver.Event{Kind: driver.EventBiometric, Count: id, Index: driver.BiometricFailure})
		return
	}
	reason := NSString(it.Value)
	block := objc.NewBlock(func(_ objc.Block, success bool, err objc.ID) {
		OnMain(func() {
			result := driver.BiometricFailure
			if success {
				result = driver.BiometricSuccess
			} else if err != 0 {
				// LAErrorUserCancel, AppCancel and SystemCancel.
				code := objc.Send[int](err, sel("code"))
				if code == 2 || code == 3 || code == 4 {
					result = driver.BiometricCancelled
				} else if code == 6 || code == 7 || code == 8 {
					result = driver.BiometricUnavailable
				}
			}
			ctx.Send(selRelease)
			dispatch(driver.Event{Kind: driver.EventBiometric, Count: id, Index: result})
		})
	})
	ctx.Send(sel("evaluatePolicy:localizedReason:reply:"), 1, reason, block)
	block.Release()
}
