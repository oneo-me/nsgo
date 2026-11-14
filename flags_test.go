package nsgo

import (
	"slices"
	"testing"

	"github.com/nsgo-dev/nsgo/core"
)

func TestLaunchFlags(t *testing.T) {
	var f launchFlags
	got := f.parse([]string{"app", "--nsgo-fps", "-v", "-nsgo-lang=zh-Hans", "--nsgo-appearance=dark", "--nsgo-nope", "file.txt", "--", "--nsgo-fps=false"})
	if want := []string{"app", "-v", "file.txt", "--", "--nsgo-fps=false"}; !slices.Equal(got, want) {
		t.Errorf("args = %q, want %q", got, want)
	}
	if !f.fps || f.lang != "zh-Hans" || !f.appearanceSet || f.appearance != core.AppearanceDark {
		t.Errorf("flags = %+v", f)
	}

	f = launchFlags{}
	f.parse([]string{"app", "--nsgo-fps=false", "--nsgo-lang", "--nsgo-appearance=blue"})
	if f.fps || f.lang != "" || f.appearanceSet {
		t.Errorf("malformed flags = %+v", f)
	}

	o := options{appearance: core.AppearanceLight}
	(&launchFlags{fps: true, appearanceSet: true}).apply(&o)
	if !o.monitor || o.appearance != core.AppearanceSystem {
		t.Errorf("options = %+v", o)
	}
	if got := f.parse(nil); got != nil {
		t.Errorf("no args = %q", got)
	}
}

// Windows' installer and COM start the app with flags of their own, which the app's flag parsing never sees, COM's -Embedding included.
func TestToastLaunchFlags(t *testing.T) {
	var f launchFlags
	got := f.parse([]string{`C:\App\App.exe`, `--nsgo-register-toast=C:\Start Menu\Bob's Notes.lnk`, "--nsgo-unregister-toast"})
	if want := []string{`C:\App\App.exe`}; !slices.Equal(got, want) {
		t.Errorf("args = %q, want %q", got, want)
	}
	if f.registerToast != `C:\Start Menu\Bob's Notes.lnk` || !f.unregisterToast || f.toastActivation {
		t.Errorf("flags = %+v", f)
	}

	f = launchFlags{}
	got = f.parse([]string{"app", "--nsgo-toast-activation", "-Embedding", "-v"})
	if want := []string{"app", "-v"}; !slices.Equal(got, want) || !f.toastActivation {
		t.Errorf("args = %q, want %q, flags = %+v", got, want, f)
	}
	f = launchFlags{}
	if got, want := f.parse([]string{"app", "--nsgo-toast-activation", "/Embedding"}), []string{"app"}; !slices.Equal(got, want) {
		t.Errorf("args = %q, want %q", got, want)
	}
	// Without the activation flag -Embedding is the app's.
	f = launchFlags{}
	if got, want := f.parse([]string{"app", "-Embedding", "--nsgo-register-toast"}), []string{"app", "-Embedding"}; !slices.Equal(got, want) || f.registerToast != "" {
		t.Errorf("args = %q, want %q, flags = %+v", got, want, f)
	}
}
