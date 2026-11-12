package main

import (
	"testing"

	"github.com/nsgo-dev/nsgo/nsgotest"
)

// TestMain hands the main thread to the platform under nsgo test -target.
func TestMain(m *testing.M) { nsgotest.Main(m) }

func TestShows(t *testing.T) {
	app := nsgotest.Start(t, Root, nsgotest.Size(390, 640))
	if got := app.Find("id:title").Text; got != "Cafe" {
		t.Errorf("title = %q", got)
	}
}
