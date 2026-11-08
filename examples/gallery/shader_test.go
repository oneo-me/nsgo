package main

import (
	"os"
	"testing"
	"time"

	"github.com/oneo-me/nsgo/nsgotest"
)

// TestShaderEffects draws the Shader page's WGSL demos: the wave's card
// still takes presses, the dissolve runs between its panels and a page
// opens over its stack through a circle and closes through it.
func TestShaderEffects(t *testing.T) {
	app := startDesktop(t, nsgotest.Size(1200, 900))
	if os.Getenv("NSGO_GALLERY_OPEN") != "Shader" {
		open(app, "Shader")
	}
	app.WaitFor("id:wave-effect", 10*time.Second)
	if fx := app.Find("id:wave-effect").Shader; fx == nil || fx.Role != nsgotest.ShaderEffect || !fx.WGSL {
		t.Fatalf("the wave is %+v", fx)
	}
	app.Screenshot("shader-effect-before")
	app.Press("id:wave-button")
	app.WaitFor("text:Start pressed 1 time", time.Second)
	app.Screenshot("shader-effect")
	app.Press("id:dissolve-next")
	app.Find("text:Second panel")
	if tr := app.Find("id:dissolve"); tr.ShaderStarts != 1 {
		t.Errorf("the dissolve started %d times", tr.ShaderStarts)
	}
	app.Screenshot("shader-transition")
	app.Press("id:page-home")
	app.WaitFor("id:page-page", time.Second)
	app.Screenshot("shader-page")
	app.Press("id:page-page")
	app.WaitFor("id:page-home", time.Second)
}

// TestShaderEffectsPhone does the same on a phone, where the page slides
// in its own way otherwise.
func TestShaderEffectsPhone(t *testing.T) {
	var app *nsgotest.App
	if target := nsgotest.Target(); target == "ios_simulator" || target == "android" {
		// The phone's own environment.
		app = nsgotest.Start(t, Root)
	} else {
		app = startPhone(t)
	}
	openPhone(app, "Shader")
	app.Press("id:wave-button")
	app.WaitFor("text:Start pressed 1 time", time.Second)
	app.Press("id:dissolve-next")
	app.Find("text:Second panel")
	app.Press("id:page-home")
	app.WaitFor("id:page-page", time.Second)
	app.Press("id:page-page")
	app.WaitFor("id:page-home", time.Second)
}
