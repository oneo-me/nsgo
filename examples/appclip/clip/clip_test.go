package main

import (
	"testing"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/nsgotest"
)

// TestMain hands the main thread to the platform under nsgo test -target.
func TestMain(m *testing.M) { nsgotest.Main(m) }

func TestOrdersAtTheLinksTable(t *testing.T) {
	app := nsgotest.Start(t, Root, nsgotest.Size(390, 640), nsgotest.Environment(core.Environment{
		OS: core.OSIOS, Idiom: core.IdiomPhone, Locale: "en-US", AppClip: true, OpenedURL: "https://cafe.example.com/table?n=12"}))
	if got := app.Find("id:table").Text; got != "Table 12" {
		t.Errorf("table = %q, want Table 12", got)
	}
	app.Press("id:add")
	if got := app.Find("id:cups").Text; got != "1 × Flat white" {
		t.Errorf("cups = %q", got)
	}
}

func TestWithoutALink(t *testing.T) {
	app := nsgotest.Start(t, Root, nsgotest.Size(390, 640))
	if got := app.Find("id:table").Text; got != "Scan a table's code" {
		t.Errorf("table = %q", got)
	}
}
