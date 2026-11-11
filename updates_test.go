package nsgo

import (
	"slices"
	"testing"
	"time"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/nsgotest"
)

// automaticUpdates turns nsgo.yaml's updates.automatic on for a test and
// returns the app function Run would run for a view saying hello.
func automaticUpdates(t *testing.T) func(*core.Scope) {
	t.Helper()
	was := updateAutomatic
	updateAutomatic = "1"
	t.Cleanup(func() { updateAutomatic = was })
	root, app := withUpdateChecks(func(cx *core.Scope) core.Node { return core.Text{Content: "hello"} }, nil)
	if root != nil || app == nil {
		t.Fatal("automatic updates run no app function")
	}
	return app
}

func TestAutomaticUpdatesCheckAsTheAppStarts(t *testing.T) {
	view := func(cx *core.Scope) core.Node { return core.Text{Content: "hello"} }
	if root, app := withUpdateChecks(view, nil); root == nil || app != nil {
		t.Fatal("an app with no automatic updates changed what it runs")
	}
	a := nsgotest.Start(t, core.AppRoot(automaticUpdates(t)))
	a.Find("text:hello")
	if !slices.Equal(a.UpdateChecks, []bool{true}) {
		t.Errorf("checks %v, want one in the background as the app starts", a.UpdateChecks)
	}
	if len(a.Alerts()) != 0 {
		t.Errorf("finding no newer version showed %+v", a.Alerts())
	}
}

// A phone's app stays in memory for days, so coming back to it checks
// too, on the same daily schedule.
func TestAutomaticUpdatesCheckAsTheUserComesBack(t *testing.T) {
	a := nsgotest.Start(t, core.AppRoot(automaticUpdates(t)))
	leaveAndComeBack := func() {
		a.SetEnvironment(core.Environment{Lifecycle: core.LifecycleBackground})
		a.SetEnvironment(core.Environment{})
	}
	leaveAndComeBack()
	if !slices.Equal(a.UpdateChecks, []bool{true}) {
		t.Fatalf("checks %v, want none more within the day", a.UpdateChecks)
	}
	a.Clock.SetWallTime(a.Clock.WallTime().Add(25 * time.Hour))
	leaveAndComeBack()
	if !slices.Equal(a.UpdateChecks, []bool{true, true}) {
		t.Errorf("checks %v, want one more a day later", a.UpdateChecks)
	}
}

// An app started out of sight, as iOS starts one in the background,
// checks once the user sees it.
func TestAutomaticUpdatesWaitForTheUser(t *testing.T) {
	a := nsgotest.Start(t, core.AppRoot(automaticUpdates(t)),
		nsgotest.Environment(core.Environment{Lifecycle: core.LifecycleBackground}))
	if len(a.UpdateChecks) != 0 {
		t.Fatalf("checks %v in the background, want none", a.UpdateChecks)
	}
	a.SetEnvironment(core.Environment{})
	if !slices.Equal(a.UpdateChecks, []bool{true}) {
		t.Errorf("checks %v, want one as the app becomes active", a.UpdateChecks)
	}
}
