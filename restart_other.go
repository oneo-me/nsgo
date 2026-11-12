//go:build !(darwin && !ios) && !(linux && !android) && !windows

package nsgo

import "errors"

// restart does nothing where the app does not start itself: phones and
// the web never return from Run.
func restart([]string) error { return errors.New("restarting is for desktops") }

func awaitRestarted() {}
