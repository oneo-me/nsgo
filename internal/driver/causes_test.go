package driver

import (
	"strconv"
	"strings"
	"testing"
)

func TestTraceNamesWhatACommitFollowedFrom(t *testing.T) {
	var ws []Write
	for i := range 10 {
		ws = append(ws, Write{Site: "app/main.go:" + strconv.Itoa(10+i), Count: 10 - i})
	}
	var b strings.Builder
	WriteTrace(&b, "commit 1", &MountBatch{Causes: &Causes{Writes: ws, Renders: 3, Effects: 1, Memos: 2}}, 0)
	lines := strings.Split(b.String(), "\n")
	if want := "   after app/main.go:10 ×10, app/main.go:11 ×9, app/main.go:12 ×8, app/main.go:13 ×7, app/main.go:14 ×6, app/main.go:15 ×5, app/main.go:16 ×4, app/main.go:17 ×3 and 2 more"; lines[1] != want {
		t.Errorf("writes = %q, want %q", lines[1], want)
	}
	if want := "   ran 3 bindings, 1 effects, 2 memos"; lines[2] != want {
		t.Errorf("runs = %q, want %q", lines[2], want)
	}

	b.Reset()
	WriteTrace(&b, "commit 2", &MountBatch{Causes: &Causes{Writes: []Write{{Site: "app/main.go:3", Count: 1}}}}, 0)
	if !strings.Contains(b.String(), "   after app/main.go:3\n") {
		t.Errorf("a single write = %q", b.String())
	}
	b.Reset()
	WriteTrace(&b, "commit 3", &MountBatch{}, 0)
	if strings.Contains(b.String(), "after") || strings.Contains(b.String(), "ran") {
		t.Errorf("a batch without causes names some: %q", b.String())
	}
}
