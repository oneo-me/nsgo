//go:build linux && !android

package linux

import (
	"fmt"
	"sync/atomic"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// GTK's settings, which pick light or dark, are the process's, and an
// embedded view leaves them to its host. A view whose app or host forces
// an appearance draws the GTK controls it hosts in it through a CSS
// provider scoped to its widget's name: its fields' carets, selections
// and scroll bars and its menus' popovers, in the colors of GTK's
// default theme. Its own drawing follows the core's appearance anyway.

var surfaceNames atomic.Uint64

// schemeCSS is the scoped style of the controls in each appearance.
const schemeCSS = `
#%[1]s popover > contents, #%[1]s popover > arrow { background-color: %[2]s; color: %[3]s; }
#%[1]s popover modelbutton:hover { background-color: alpha(%[3]s, 0.08); }
#%[1]s text, #%[1]s entry { caret-color: %[3]s; }
#%[1]s text selection, #%[1]s entry selection { background-color: alpha(#3584e4, 0.5); color: %[3]s; }
#%[1]s scrollbar slider { background-color: alpha(%[3]s, 0.4); }
`

// scopeAppearance draws the controls of an embedded view in shown, the
// appearance its app or host forces; AppearanceSystem leaves them to the
// host's settings.
func (w *window) scopeAppearance(shown driver.Appearance) {
	if !w.embedded || shown == w.scoped {
		return
	}
	w.scoped = shown
	display := gdkDisplayGetDefault()
	if w.schemeProvider != 0 {
		gtkStyleContextRemoveDisplay(display, w.schemeProvider)
		gObjectUnref(w.schemeProvider)
		w.schemeProvider = 0
	}
	if shown == driver.AppearanceSystem {
		return
	}
	if w.surfaceName == "" {
		w.surfaceName = fmt.Sprintf("nsgo-surface-%d", surfaceNames.Add(1))
		gtkWidgetSetName(w.surface, cstr(w.surfaceName))
	}
	popover, fg := "#ffffff", "#000000"
	if shown == driver.AppearanceDark {
		popover, fg = "#383838", "#ffffff"
	}
	w.schemeProvider = gtkCssProviderNew()
	gtkCssProviderLoadFromString(w.schemeProvider, cstr(fmt.Sprintf(schemeCSS, w.surfaceName, popover, fg)))
	// Above the theme, below each field's own style.
	gtkStyleContextAddForDisplay(display, w.schemeProvider, 700)
}

// shownAppearance is the appearance a view forces: its app's, else its
// host's.
func (w *window) shownAppearance() driver.Appearance {
	if w.forced != driver.AppearanceSystem {
		return w.forced
	}
	return w.opts.Host.Settings.Environment.Appearance
}
