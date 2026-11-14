//go:build darwin

package apple

import (
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// LocalAuthentication's error codes are negative: a cancel answers cancelled and a lockout unavailable, not a failure.
func TestBiometricResultReadsLAErrorCodes(t *testing.T) {
	for code, want := range map[int]int{
		-1:    driver.BiometricFailure,     // LAErrorAuthenticationFailed
		-2:    driver.BiometricCancelled,   // LAErrorUserCancel
		-3:    driver.BiometricCancelled,   // LAErrorUserFallback
		-4:    driver.BiometricCancelled,   // LAErrorSystemCancel
		-5:    driver.BiometricUnavailable, // LAErrorPasscodeNotSet
		-6:    driver.BiometricUnavailable, // LAErrorBiometryNotAvailable
		-7:    driver.BiometricUnavailable, // LAErrorBiometryNotEnrolled
		-8:    driver.BiometricUnavailable, // LAErrorBiometryLockout
		-9:    driver.BiometricCancelled,   // LAErrorAppCancel
		-10:   driver.BiometricFailure,     // LAErrorInvalidContext
		-1004: driver.BiometricUnavailable, // LAErrorNotInteractive
		2:     driver.BiometricFailure,
	} {
		if got := biometricResult(code); got != want {
			t.Errorf("biometricResult(%d) = %d, want %d", code, got, want)
		}
	}
}
