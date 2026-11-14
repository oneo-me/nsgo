package core_test

import (
	"strconv"
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

func TestDisplaysFollowEnvironment(t *testing.T) {
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		e := cx.Env()
		return core.Text{TestID: "count", ContentFunc: func() string { return strconv.Itoa(len(e.Displays())) }}
	}, nsgotest.Environment(core.Environment{Displays: []core.DisplayInfo{{ID: "main", Main: true}}}))
	if app.Find("id:count").Text != "1" {
		t.Fatalf("initial displays: %q", app.Find("id:count").Text)
	}
	app.SetEnvironment(core.Environment{Displays: []core.DisplayInfo{{ID: "main", Main: true}, {ID: "second"}}})
	if app.Find("id:count").Text != "2" {
		t.Fatalf("updated displays: %q", app.Find("id:count").Text)
	}
}
