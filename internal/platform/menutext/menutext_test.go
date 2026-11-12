package menutext

import (
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

func TestLabel(t *testing.T) {
	for _, c := range []struct {
		lang, label, app, want string
		role                   driver.MenuRole
	}{
		{lang: "zh-Hans", role: driver.MenuPaste, want: "粘贴"},
		{lang: "zh-Hans", role: driver.MenuPaste, label: "Paste", want: "粘贴"},
		{lang: "zh-Hans", role: driver.MenuQuit, app: "Neorio", want: "退出 Neorio"},
		{lang: "en", role: driver.MenuAbout, app: "Neorio", want: "About Neorio"},
		{lang: "en", role: driver.MenuAbout, want: "About"},
		{lang: "de", role: driver.MenuClose, want: "Close"},
		{lang: "zh-Hans", role: driver.MenuPaste, label: "Paste Text", want: "Paste Text"},
		{lang: "zh-Hans", label: "Paste", want: "Paste"},
	} {
		e := driver.MenuEntry{Label: c.label, Role: c.role}
		if got := Label(c.lang, &e, c.app); got != c.want {
			t.Errorf("Label(%q, %+v, %q) = %q, want %q", c.lang, e, c.app, got, c.want)
		}
	}
}
