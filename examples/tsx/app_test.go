package main

import (
	"slices"
	"testing"
	"time"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/nsgotest"
)

// TestMain hands the main thread to the platform under nsgo test -target.
func TestMain(m *testing.M) { nsgotest.Main(m) }

// has checks that texts show, as they show: uppercase classes apply.
func has(t *testing.T, app *nsgotest.App, texts ...string) {
	t.Helper()
	got := app.Texts()
	for _, want := range texts {
		if !slices.Contains(got, want) {
			t.Errorf("no %q among %q", want, got)
		}
	}
}

func TestTasks(t *testing.T) {
	app := nsgotest.Start(t, Root)
	has(t, app, "Today", "2 left · 1 done", "Sketch the settings screen", "3", "2", "1")

	app.Type("id:draft", "Buy milk")
	app.Press("id:urgent")
	app.Press("text:Add")
	has(t, app, "Buy milk", "3 left · 1 done")
	if v := app.Find("id:draft"); v.Text != "" {
		t.Errorf("the field still shows %q", v.Text)
	}

	// Done tasks are struck through.
	app.Press("id:check-1")
	has(t, app, "2 left · 2 done")
	if s := app.Find("id:title-1").TextStyle; s.Decoration != core.TextDecorationLineThrough {
		t.Errorf("the done task's decoration is %v", s.Decoration)
	}

	app.Press("text:Done")
	if app.Exists("text:Buy milk") || !app.Exists("text:Sketch the settings screen") {
		t.Errorf("the done filter shows %q", app.Texts())
	}
	app.Press("text:Clear completed")
	has(t, app, "Nothing here.")
	app.Press("text:All")
	has(t, app, "Buy milk", "Review the Tailwind classes", "2 left · 0 done")
}

func TestFocusTimer(t *testing.T) {
	app := nsgotest.Start(t, Root)
	has(t, app, "25:00", "READY", "A 25 minute session · 0 sessions · 0 min")
	app.Press("text:Start")
	app.Advance(3 * time.Second)
	has(t, app, "24:57", "RUNNING", "Pause")
	app.Press("text:Pause")
	app.Advance(2 * time.Second)
	has(t, app, "24:57", "PAUSED")
	app.Press("text:Reset")
	has(t, app, "25:00", "READY", "A 25 minute session · 1 session · 0 min")
}

func TestExport(t *testing.T) {
	app := nsgotest.Start(t, Root)
	app.Press("text:Export")
	app.WaitFor("text:Exported 3 tasks", 5*time.Second)
}

func TestAppearanceAndColumns(t *testing.T) {
	app := nsgotest.Start(t, Root, nsgotest.Size(960, 720))
	white := core.RGB(0xffffff)
	if bg := app.Find("text:Light").Parent.Style.Background; bg != white {
		t.Errorf("the switch is %#x in light", bg)
	}
	app.Press("id:appearance")
	has(t, app, "Dark")
	if bg := app.Find("text:Dark").Parent.Style.Background; bg == white {
		t.Error("the switch stays white in dark")
	}
	// From md: on, the timer sits beside the tasks.
	tasks, focus := app.Find("text:Tasks"), app.Find("text:Focus")
	if tasks.Frame.Origin.Y != focus.Frame.Origin.Y {
		t.Errorf("the cards' titles are at %v and %v", tasks.Frame.Origin.Y, focus.Frame.Origin.Y)
	}
}
