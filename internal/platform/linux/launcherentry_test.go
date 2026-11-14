//go:build linux && !android

package linux

import "testing"

func TestLauncherEntryNamesTheDesktopEntry(t *testing.T) {
	if got := launcherEntryURI("dev.nsgo.gallery"); got != "application://dev.nsgo.gallery.desktop" {
		t.Errorf("uri %q", got)
	}
	// Object path elements take only letters, digits and underscores.
	if got := launcherEntryPath("dev.nsgo-app/x"); got != "/com/canonical/unity/launcherentry/dev_nsgo_app_x" {
		t.Errorf("path %q", got)
	}
}
