package main

import (
	"testing"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/nsgotest"
)

// TestMain hands the main thread to the platform under nsgo test -target.
func TestMain(m *testing.M) { nsgotest.Main(m) }

func TestGreetsTheLinksName(t *testing.T) {
	app := nsgotest.Start(t, Root, nsgotest.Size(390, 640), nsgotest.Environment(core.Environment{
		OS: core.OSIOS, Idiom: core.IdiomPhone, Locale: "en-US", AppClip: true, OpenedURL: "https://hello.example.com/?name=Ada"}))
	if got := app.Find("id:greeting").Text; got != "Hello, Ada!" {
		t.Errorf("greeting = %q", got)
	}
	app.Press("id:say")
	if got := app.Find("id:said").Text; got != "Said 1 time" {
		t.Errorf("said = %q", got)
	}
}

func TestWithoutALink(t *testing.T) {
	app := nsgotest.Start(t, Root, nsgotest.Size(390, 640))
	if got := app.Find("id:greeting").Text; got != "Hello, world!" {
		t.Errorf("greeting = %q", got)
	}
}
