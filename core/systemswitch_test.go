package core_test

import (
	"testing"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/nsgotest"
)

func TestSystemSwitchFollowsTheApp(t *testing.T) {
	for _, os := range []driver.OS{driver.OSMacOS, driver.OSIOS} {
		app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
			on := core.NewSignal(cx, false)
			return core.SystemSwitch{Checked: on.Get, OnChange: on.Set, Size: "small", Label: "Wi-Fi", TestID: "switch"}
		}, nsgotest.Environment(driver.Environment{OS: os}))
		v := app.FindMounted("id:switch")
		if v.Component != driver.SwitchComponent || v.NativeProps["checked"] != false || v.NativeProps["size"] != "small" {
			t.Fatalf("%v: component %q, props %v", os, v.Component, v.NativeProps)
		}
		app.Emit("id:switch", "change", true)
		if got := app.FindMounted("id:switch").NativeProps["checked"]; got != true {
			t.Fatalf("%v: checked = %v after the user turned it on", os, got)
		}
	}
}

func TestSystemSwitchShowsNothingElsewhere(t *testing.T) {
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return core.SystemSwitch{TestID: "switch"}
	}, nsgotest.Environment(driver.Environment{OS: driver.OSAndroid}))
	if v := app.FindMounted("id:switch"); v.Component != "" {
		t.Fatalf("Android mounted the component %q", v.Component)
	}
}
