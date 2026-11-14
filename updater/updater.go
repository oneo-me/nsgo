// Package updater keeps an app up to date the way its platform does. On
// macOS and Windows, Sparkle and WinSparkle, which nsgo build bundles when
// nsgo.yaml configures updates, read the app's appcast on their own
// schedule, download a newer release, verify its EdDSA signature,
// install it and restart the app. Linux and Android read the same appcast,
// verify and install its package after the user confirms, and iOS asks the
// App Store, once a day and whenever the user asks.
package updater

import (
	"context"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/updater/locales"
)

// Check checks for a newer version now, as a Check for Updates… item
// does, and shows what it finds: on macOS and Windows in Sparkle's and
// WinSparkle's windows, on Linux and Android an alert that installs the
// signed package after confirmation, elsewhere an alert that offers to open
// the newer version's page, says the app is up to date, or says the check
// failed.
func Check(cx *core.Scope) { check(cx, false, nil) }

// CheckInBackground checks without showing anything unless it finds a
// newer version, as the platforms' scheduled checks do.
func CheckInBackground(cx *core.Scope) { check(cx, true, nil) }

// CheckDaily's schedule: interval is how long it leaves between its
// checks, a day, as Sparkle and WinSparkle leave between theirs, and
// retry how long after a check that failed.
const (
	interval = 24 * time.Hour
	retry    = time.Hour
)

// CheckDaily keeps the time of its last check, in seconds since 1970, in
// a partition of the app's storage, which the app's own Storage neither
// lists nor clears.
const (
	schedulePartition = "nsgo.updater"
	checkedKey        = "checked"
)

// CheckDaily checks in the background, as CheckInBackground does, unless
// it has within the last day, so a newer version is offered once a day
// rather than at every start. Sparkle and WinSparkle keep such a schedule
// of their own; on Linux, Android and iOS nsgo.Run calls CheckDaily as
// the app starts and as the user comes back to it when nsgo.yaml's
// updates.automatic is on. A check that failed, as one offline does, is
// tried again an hour later.
func CheckDaily(cx *core.Scope) {
	now := cx.WallTime()
	var kept core.Storage
	due := true
	// The partition is a scope's of its own, so the app's storage stays
	// whole, and is read untracked.
	part := cx.Child()
	part.Run(func() {
		part.ProvideStorage(schedulePartition, 0)
		kept = part.Storage()
		if text, ok := kept.GetItem(checkedKey); ok {
			if sec, err := strconv.ParseInt(text, 10, 64); err == nil {
				// A time ahead of now is a clock set back since: check.
				since := now.Sub(time.Unix(sec, 0))
				due = since < 0 || since >= interval
			}
		}
	})
	part.Dispose()
	if !due {
		return
	}
	keep := func(checked time.Time) {
		if err := kept.SetItem(checkedKey, strconv.FormatInt(checked.Unix(), 10)); err != nil {
			log.Printf("updater: %v", err)
		}
	}
	// Kept before the answer, so a call meanwhile does not check too.
	keep(now)
	check(cx, true, func(c core.UpdateCheck) {
		if c.Err != nil {
			// Due again after retry rather than a day.
			keep(now.Add(retry - interval))
		}
	})
}

// check asks the platform's updater and shows what it answers, which
// answered, if any, hears first.
func check(cx *core.Scope, background bool, answered func(core.UpdateCheck)) {
	cx.CheckForUpdates(background, func(c core.UpdateCheck) {
		if answered != nil {
			answered(c)
		}
		if c.Shown {
			return
		}
		msgs := locales.For(cx)
		ok := []core.AlertAction{{Label: msgs.Ok(), Role: core.AlertDefault}}
		switch {
		case c.Err != nil:
			log.Printf("updater: %v", c.Err)
			if !background {
				cx.ShowAlert(core.AlertConfig{Title: msgs.UpdateError(), Message: msgs.UpdateErrorMessage(),
					Actions: ok, Cancelable: true}, func(int) {})
			}
		case c.Version == "":
			if !background {
				cx.ShowAlert(core.AlertConfig{Title: msgs.UpToDate(), Message: msgs.UpToDateMessage(c.App, c.Current),
					Actions: ok, Cancelable: true}, func(int) {})
			}
		default:
			cx.ShowAlert(core.AlertConfig{
				Title:   msgs.UpdateAvailable(c.App),
				Message: msgs.UpdateAvailableMessage(c.App, c.Version, c.Current),
				Actions: []core.AlertAction{{Label: msgs.Update(), Role: core.AlertDefault},
					{Label: msgs.Later(), Role: core.AlertCancel}},
				Cancelable: true,
			}, func(index int) {
				if index == 0 {
					if c.Download != "" && (cx.Env().OS() == core.OSLinux || cx.Env().OS() == core.OSAndroid) {
						downloadAndInstall(cx, c)
					} else {
						cx.OpenURL(c.URL, nil)
					}
				}
			})
		}
	})
}

// downloadAndInstall keeps network and package-manager work off the app's
// graph and main threads. The dialog's Update action is the user's explicit
// approval; package managers still ask for their own elevation or install
// permission.
func downloadAndInstall(cx *core.Scope, c core.UpdateCheck) {
	dir := cx.CacheDir()
	if dir == "" {
		dir = os.TempDir()
	}
	go func() {
		path, err := Download(context.Background(), c, dir, nil)
		cx.Post(func() {
			if err != nil {
				log.Printf("updater: %v", err)
				return
			}
			Install(cx, path, func(err error) {
				if err != nil {
					log.Printf("updater: install: %v", err)
					return
				}
				if cx.Env().OS() == core.OSLinux {
					cx.Restart()
				}
			})
		})
	}()
}
