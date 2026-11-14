package core_test

import (
	"slices"
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

func TestBiometricsAvailabilityAndAuthentication(t *testing.T) {
	var scope *core.Scope
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		scope = cx
		return core.View{}
	})
	app.BiometricAvailability = core.BiometricFace
	app.BiometricAnswer = core.BiometricSuccess
	var kinds []core.BiometricKind
	var results []core.BiometricResult
	scope.BiometricAvailability(func(kind core.BiometricKind) { kinds = append(kinds, kind) })
	scope.AuthenticateBiometric("Confirm transfer", func(result core.BiometricResult) { results = append(results, result) })
	app.Settle()
	if !slices.Equal(kinds, []core.BiometricKind{core.BiometricFace}) || !slices.Equal(results, []core.BiometricResult{core.BiometricSuccess}) {
		t.Fatalf("biometric answers: kinds %v, results %v", kinds, results)
	}
	if !slices.Equal(app.AuthenticationReasons, []string{"Confirm transfer"}) {
		t.Fatalf("reasons %v", app.AuthenticationReasons)
	}
}

func TestBiometricsHeldToAccess(t *testing.T) {
	var scope *core.Scope
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		scope = cx
		cx.ProvideAccess(denyAll{})
		return core.View{}
	})
	var kind core.BiometricKind
	var result core.BiometricResult
	scope.BiometricAvailability(func(v core.BiometricKind) { kind = v })
	scope.AuthenticateBiometric("Confirm", func(v core.BiometricResult) { result = v })
	app.Settle()
	if kind != core.BiometricNone || result != core.BiometricUnavailable {
		t.Fatalf("refused answers: %v, %v", kind, result)
	}
}

func TestBiometricsRefuseAnEmptyReason(t *testing.T) {
	var scope *core.Scope
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		scope = cx
		return core.View{}
	})
	app.BiometricAvailability = core.BiometricFace
	app.BiometricAnswer = core.BiometricSuccess
	var result core.BiometricResult
	scope.AuthenticateBiometric("", func(v core.BiometricResult) { result = v })
	app.Settle()
	if result != core.BiometricFailure || len(app.AuthenticationReasons) != 0 {
		t.Fatalf("empty reason: result %v, reasons %q", result, app.AuthenticationReasons)
	}
}
