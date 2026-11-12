package main

import (
	"testing"

	"github.com/nsgo-dev/nsgo/nsgotest"
)

// TestMain hands the main thread to the platform under nsgo test -target.
func TestMain(m *testing.M) { nsgotest.Main(m) }

func TestGreets(t *testing.T) {
	app := nsgotest.Start(t, Root, nsgotest.Size(420, 320))
	if got := app.Find("id:greeting").Text; got != "Hello, world!" {
		t.Errorf("greeting = %q", got)
	}
	app.Type("id:name", "Ada")
	if got := app.Find("id:greeting").Text; got != "Hello, Ada!" {
		t.Errorf("greeting after typing = %q", got)
	}
	app.Press("id:say")
	app.Press("id:say")
	if got := app.Find("id:said").Text; got != "Said 2 times" {
		t.Errorf("said = %q", got)
	}
}
