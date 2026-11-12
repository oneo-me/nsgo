package core

import (
	"errors"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// UpdateCheck is what the platform's updater answers CheckForUpdates
// with.
type UpdateCheck struct {
	// Shown says the platform's own updater showed what it found in its
	// windows, as Sparkle on macOS and WinSparkle on Windows do: they
	// offer the newer version, install it and restart the app. Nothing
	// else is set then.
	Shown bool
	// App and Current are the app's name and running version.
	App, Current string
	// Version is the newer version found, "" when Current is the newest,
	// and URL the page that gets it: the App Store's on iOS, the
	// release's elsewhere.
	Version, URL string
	// Err says why the updater could not check.
	Err error
}

// CheckForUpdates is the core's part of updater.Check and
// updater.CheckInBackground, which apps use: it asks the platform's
// updater for a newer version of the app, quietly when background, and
// done hears its answer. A platform with no updater, such as the web,
// never answers.
func (cx *Scope) CheckForUpdates(background bool, done func(UpdateCheck)) {
	t := cx.tree()
	if t == nil {
		return
	}
	t.request(cx, driver.MountItem{Kind: driver.ItemCheckForUpdates, On: background}, func(ev driver.Event) {
		c := UpdateCheck{Shown: ev.Finished}
		if u, ok := ev.Data.(*driver.Update); ok && u != nil {
			c.App, c.Current, c.Version, c.URL = u.App, u.Current, u.Version, u.URL
			if u.Err != "" {
				c.Err = errors.New(u.Err)
			}
		}
		done(c)
	})
}
