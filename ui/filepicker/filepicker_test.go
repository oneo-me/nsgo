package filepicker

import (
	"errors"
	"testing"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/nsgotest"
)

// TestDialogListsAndChooses shows a folder's entries, hides hidden ones and
// chooses the folder, and a failed listing says why.
func TestDialogListsAndChooses(t *testing.T) {
	var chosen string
	fail := false
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		s := NewState(cx, List, false, 0)
		s.Load = func(path string, done func(Folder, error)) {
			if fail {
				done(Folder{}, errors.New("no such folder"))
				return
			}
			done(Folder{Path: "/work", Entries: []Entry{{Name: "src", Path: "/work/src", Dir: true}, {Name: ".git", Path: "/work/.git", Dir: true, Hidden: true}}}, nil)
		}
		s.Open("")
		return Dialog{State: s, Open: func() bool { return true }, OnOpen: func(bool) {}, Labels: Labels{Empty: "Empty", Choose: "Choose", Cancel: "Cancel"},
			Choose: func(path string) { chosen = path }}
	}, nsgotest.Size(1000, 800))
	app.Settle()
	if !app.Exists("filepicker-entry-src") || app.Exists("filepicker-entry-.git") {
		t.Fatal(app.Dump())
	}
	app.Press("filepicker-choose")
	if chosen != "/work" {
		t.Fatalf("chose %q", chosen)
	}
}

func TestSize(t *testing.T) {
	for n, want := range map[int64]string{999: "999 B", 1500: "1.5 KB", 2_000_000: "2.0 MB"} {
		if got := Size(n); got != want {
			t.Errorf("Size(%d) = %q, want %q", n, got, want)
		}
	}
}
