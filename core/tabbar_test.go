package core_test

import (
	"testing"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/nsgotest"
)

func TestTabBarTakesTheAccent(t *testing.T) {
	theme := core.DefaultTheme()
	theme.Light.Accent = core.RGB(0xbd481b)
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		cx.SetTheme(theme)
		return core.TabBar{Items: []core.TabBarItem{{Title: "Home", Icon: "house"}}, Style: core.Style{Height: core.Pt(49)}, TestID: "bar"}
	}, nsgotest.Environment(driver.Environment{OS: driver.OSIOS}))
	if got := app.Find("id:bar").NativeProps["tint"]; got != float64(core.RGB(0xbd481b)) {
		t.Fatalf("tint = %v, want the theme's accent", got)
	}
}
