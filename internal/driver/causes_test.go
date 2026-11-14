package driver

import (
	"strconv"
	"strings"
	"testing"
	"time"
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
	WriteTrace(&b, "commit 4", &MountBatch{Causes: &Causes{App: 1500 * time.Microsecond, Layout: 250 * time.Microsecond,
		Ran: []Ran{{Kind: "binding", Site: "app/row.go:9", Runs: 40, Took: 2 * time.Millisecond}, {Kind: "memo", Site: "app/main.go:5", Runs: 1, Took: 10 * time.Microsecond}}}}, 0)
	for _, want := range []string{"   app 1.50 ms before, layout 0.25 ms\n", "   longest binding app/row.go:9 ×40 2.00 ms, memo app/main.go:5 ×1 0.01 ms\n"} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("trace = %q, want %q", b.String(), want)
		}
	}
	b.Reset()
	WriteTrace(&b, "commit 3", &MountBatch{}, 0)
	if strings.Contains(b.String(), "after") || strings.Contains(b.String(), "ran") {
		t.Errorf("a batch without causes names some: %q", b.String())
	}
}

func TestApplyReportsThePlatformsTimeOnlyWhenAsked(t *testing.T) {
	b := &MountBatch{}
	applied := 0
	apply := func(*MountBatch) { applied++; time.Sleep(time.Millisecond) }
	Apply(b, apply)
	if applied != 1 {
		t.Fatalf("applied %d times", applied)
	}
	var heard *MountBatch
	var took time.Duration
	Applied = func(b *MountBatch, d time.Duration) { heard, took = b, d }
	defer func() { Applied = nil }()
	Apply(b, apply)
	if applied != 2 || heard != b || took < time.Millisecond {
		t.Fatalf("applied %d times, heard %p after %v", applied, heard, took)
	}
}
