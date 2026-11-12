package ui

import (
	"math"
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

// TestSwitchSlidesAcrossUnrelatedChanges finishes the knob's slide when
// what Checked reads changes during it without changing its value, as an
// app's revision does.
func TestSwitchSlidesAcrossUnrelatedChanges(t *testing.T) {
	headless(t)
	var app *nsgotest.App
	var tick *core.Signal[int]
	app, _ = start(t, desktopEnv, func(cx *core.Scope) core.Node {
		on := core.NewSignal(cx, false)
		tick = core.NewSignal(cx, 0)
		return Switch{Label: "Keep awake", Checked: func() bool {
			tick.Get()
			return on.Get()
		}, OnChange: func(v bool) {
			on.Set(v)
			app.Post(func() { tick.Set(1) })
		}}
	})
	app.Press("role:switch Keep awake")
	const across = 15.2
	if x := app.Find("role:switch Keep awake").Children[0].Children[0].Animated.TranslateX; math.Abs(float64(x)-across) > 0.01 {
		t.Fatalf("knob at %v after its slide, want %v", x, across)
	}
}

func TestSystemSwitchIsThePlatformsOnApple(t *testing.T) {
	headless(t)
	for _, tc := range []struct {
		os     core.OS
		native bool
	}{{core.OSMacOS, true}, {core.OSIOS, true}, {core.OSAndroid, false}, {core.OSWindows, false}} {
		app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
			on := core.NewSignal(cx, true)
			return Switch{Checked: on.Get, OnChange: on.Set, Label: "Wi-Fi", Variant: SwitchSystem}
		}, nsgotest.Environment(core.Environment{OS: tc.os}))
		v := app.FindMounted("label:Wi-Fi")
		if native := v.Component != ""; native != tc.native {
			t.Fatalf("%v: component %q, want native %v", tc.os, v.Component, tc.native)
		}
	}
}
