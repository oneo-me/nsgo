package updater_test

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
	"github.com/nsgo-dev/nsgo/updater"
)

// start runs an app whose Check, Quietly and Daily buttons check for
// updates, now, in the background and on the daily schedule, with the
// platform's updater answering found.
func start(t *testing.T, found core.UpdateCheck) *nsgotest.App {
	t.Helper()
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		button := func(label string, check func(*core.Scope)) core.Node {
			return base.Button{OnPress: func(core.PressEvent) { check(cx) },
				Style: core.Style{Width: core.Pt(80), Height: core.Pt(30)}, Children: []core.Node{core.Text{Content: label}}}
		}
		return core.View{Children: []core.Node{button("Check", updater.Check), button("Quietly", updater.CheckInBackground),
			button("Daily", updater.CheckDaily)}}
	}, nsgotest.Size(300, 120))
	app.Update = func(bool) core.UpdateCheck { return found }
	return app
}

var newer = core.UpdateCheck{App: "Hello", Current: "1.0", Version: "1.1", URL: "https://example.com/hello/1.1"}

func TestUpdateOpensTheNewerVersion(t *testing.T) {
	app := start(t, newer)
	app.Press("text:Check")
	a := app.Alerts()
	if len(a) != 1 || a[0].Title != "A new version of Hello is available" || a[0].Message != "Hello 1.1 is available. You have 1.0." {
		t.Fatalf("alerts %+v", a)
	}
	if labels := []string{a[0].Actions[0].Label, a[0].Actions[1].Label}; !slices.Equal(labels, []string{"Update", "Later"}) {
		t.Errorf("buttons %v", labels)
	}
	app.Answer(0)
	if !slices.Equal(app.OpenedURLs, []string{newer.URL}) {
		t.Errorf("opened %v, want the newer version's page", app.OpenedURLs)
	}
	app.Press("text:Check")
	app.Answer(1)
	if len(app.OpenedURLs) != 1 {
		t.Errorf("Later opened %v", app.OpenedURLs[1:])
	}
	if !slices.Equal(app.UpdateChecks, []bool{false, false}) {
		t.Errorf("checks %v", app.UpdateChecks)
	}
}

func TestBackgroundChecksShowOnlyANewerVersion(t *testing.T) {
	for _, c := range []core.UpdateCheck{{App: "Hello", Current: "1.0"}, {Err: errors.New("offline")}} {
		app := start(t, c)
		app.Press("text:Quietly")
		if a := app.Alerts(); len(a) != 0 {
			t.Errorf("a background check finding %+v showed %+v", c, a)
		}
		if !slices.Equal(app.UpdateChecks, []bool{true}) {
			t.Errorf("checks %v, want one in the background", app.UpdateChecks)
		}
	}
	app := start(t, newer)
	app.Press("text:Quietly")
	if a := app.Alerts(); len(a) != 1 || a[0].Title != "A new version of Hello is available" {
		t.Errorf("alerts %+v, want the newer version offered", a)
	}
}

func TestCheckSaysWhatItFound(t *testing.T) {
	for _, c := range []struct {
		found          core.UpdateCheck
		title, message string
	}{
		{core.UpdateCheck{App: "Hello", Current: "1.0"}, "You’re up to date", "Hello 1.0 is the newest version available."},
		{core.UpdateCheck{Err: errors.New("offline")}, "Can’t check for updates", "Something went wrong while checking for updates. Try again later."},
	} {
		app := start(t, c.found)
		app.Press("text:Check")
		if a := app.Alerts(); len(a) != 1 || a[0].Title != c.title || a[0].Message != c.message || len(a[0].Actions) != 1 {
			t.Errorf("finding %+v showed %+v", c.found, a)
		}
	}
}

// Sparkle's and WinSparkle's windows show what they find instead.
func TestPlatformWindowsShowWhatTheyFind(t *testing.T) {
	app := start(t, core.UpdateCheck{Shown: true})
	app.Press("text:Check")
	if a := app.Alerts(); len(a) != 0 {
		t.Errorf("alerts %+v over the platform's own windows", a)
	}
}

// later moves the wall clock on by d, as the time until a later start.
func later(app *nsgotest.App, d time.Duration) {
	app.Clock.SetWallTime(app.Clock.WallTime().Add(d))
}

func TestDailyChecksWaitADay(t *testing.T) {
	app := start(t, newer)
	app.Press("text:Daily")
	if a := app.Alerts(); len(a) != 1 || a[0].Title != "A new version of Hello is available" {
		t.Fatalf("alerts %+v, want the newer version offered", a)
	}
	app.Answer(1)
	// Later lasts the day: it neither checks nor offers again.
	later(app, 12*time.Hour)
	app.Press("text:Daily")
	if len(app.UpdateChecks) != 1 || len(app.Alerts()) != 0 {
		t.Fatalf("checks %v and alerts %+v within the day, want none more", app.UpdateChecks, app.Alerts())
	}
	later(app, 13*time.Hour)
	app.Press("text:Daily")
	if !slices.Equal(app.UpdateChecks, []bool{true, true}) || len(app.Alerts()) != 1 {
		t.Fatalf("checks %v and alerts %+v a day later, want the newer version offered again", app.UpdateChecks, app.Alerts())
	}
	app.Answer(1)
	// A check the user asks for is not held back.
	app.Press("text:Check")
	if !slices.Equal(app.UpdateChecks, []bool{true, true, false}) {
		t.Errorf("checks %v, want Check to check whatever the schedule", app.UpdateChecks)
	}
	// The schedule is kept apart from the app's own entries.
	if keys := app.Storage().Keys(); len(keys) != 0 {
		t.Errorf("the app's storage lists %v", keys)
	}
}

func TestDailyChecksTryAgainAfterAFailure(t *testing.T) {
	app := start(t, core.UpdateCheck{Err: errors.New("offline")})
	app.Press("text:Daily")
	// Not at once: coming back to the app would ask a server that is down
	// every time.
	app.Press("text:Daily")
	if !slices.Equal(app.UpdateChecks, []bool{true}) || len(app.Alerts()) != 0 {
		t.Fatalf("checks %v and alerts %+v, want one failed check, quietly", app.UpdateChecks, app.Alerts())
	}
	later(app, time.Hour)
	app.Press("text:Daily")
	if len(app.UpdateChecks) != 2 || len(app.Alerts()) != 0 {
		t.Fatalf("checks %v and alerts %+v, want a failed check tried again an hour later, quietly", app.UpdateChecks, app.Alerts())
	}
	// An answer counts, a newer version or none.
	app.Update = func(bool) core.UpdateCheck { return core.UpdateCheck{App: "Hello", Current: "1.0"} }
	later(app, time.Hour)
	app.Press("text:Daily")
	later(app, 23*time.Hour)
	app.Press("text:Daily")
	if len(app.UpdateChecks) != 3 {
		t.Errorf("checks %v, want none within a day of one answered", app.UpdateChecks)
	}
}

// A clock set back since the last check does not hold the next one off.
func TestDailyChecksAfterTheClockGoesBack(t *testing.T) {
	app := start(t, core.UpdateCheck{App: "Hello", Current: "1.0"})
	app.Press("text:Daily")
	later(app, -48*time.Hour)
	app.Press("text:Daily")
	if len(app.UpdateChecks) != 2 {
		t.Errorf("checks %v, want one more with the clock two days back", app.UpdateChecks)
	}
}
