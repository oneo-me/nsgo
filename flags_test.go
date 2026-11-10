package nsgo

import (
	"slices"
	"testing"

	"github.com/oneo-me/nsgo/core"
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
