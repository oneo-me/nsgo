package core

import (
	"context"
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
	// Download is the signed package for Version, when this target can
	// install it itself. Signature and SHA256 authenticate its bytes; Key is
	// the public Ed25519 key from the app's update configuration.
	Download, Signature, SHA256, Key string
	Size                             int64
	// Err says why the updater could not check.
	Err error
}

// UpdateDownloadProgress reports bytes received by DownloadUpdate.
type UpdateDownloadProgress struct{ Loaded, Total int64 }

// DownloadUpdate stages an update through the platform's HTTP client.
func DownloadUpdate(ctx context.Context, update UpdateCheck, dir string, progress func(UpdateDownloadProgress)) (string, error) {
	if driver.UpdateDownload == nil {
		return "", errors.New("core: update download is unavailable")
	}
	d := driver.Update{App: update.App, Current: update.Current, Version: update.Version, URL: update.URL,
		Download: update.Download, Signature: update.Signature, SHA256: update.SHA256, Key: update.Key, Size: update.Size}
	path, err := driver.UpdateDownload(ctx, d, dir, func(p driver.UpdateDownloadProgress) {
		if progress != nil {
			progress(UpdateDownloadProgress(p))
		}
	})
	return path, err
}

// InstallUpdate hands a verified package to the platform's installer.
func InstallUpdate(cx *Scope, path string, done func(error)) {
	if cx.Env().OS() == OSAndroid {
		cx.InstallPackage(path, func(started bool, err error) {
			if err == nil && !started {
				err = errors.New("core: Android installer did not start")
			}
			if done != nil {
				done(err)
			}
		})
		return
	}
	if driver.UpdateInstall == nil {
		if done != nil {
			done(errors.New("core: update installer is unavailable"))
		}
		return
	}
	driver.UpdateInstall(path, func(err error) {
		cx.Post(func() {
			if done != nil {
				done(err)
			}
		})
	})
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
			c.Download, c.Signature, c.SHA256, c.Key, c.Size = u.Download, u.Signature, u.SHA256, u.Key, u.Size
			if u.Err != "" {
				c.Err = errors.New(u.Err)
			}
		}
		done(c)
	})
}
