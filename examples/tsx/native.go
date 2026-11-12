package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/nsgo-dev/nsgo/tsx"
)

// FocusLog keeps the focus sessions in Go; TSX makes one with new.
type FocusLog struct {
	seconds []int
}

// Record logs a session of seconds, ignoring empty ones.
func (l *FocusLog) Record(seconds int) {
	if seconds > 0 {
		l.seconds = append(l.seconds, seconds)
	}
}

// Summary says how many sessions and minutes the log holds.
func (l *FocusLog) Summary() string {
	total := 0
	for _, s := range l.seconds {
		total += s
	}
	if len(l.seconds) == 1 {
		return fmt.Sprintf("1 session · %d min", total/60)
	}
	return fmt.Sprintf("%d sessions · %d min", len(l.seconds), total/60)
}

// exportTasks writes the tasks as a Markdown checklist on a goroutine,
// calling progress after each, and resolves with it.
func exportTasks(ctx context.Context, titles []string, done []bool, progress func(n int)) (string, error) {
	var b strings.Builder
	for i, title := range titles {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		mark := " "
		if i < len(done) && done[i] {
			mark = "x"
		}
		fmt.Fprintf(&b, "- [%s] %s\n", mark, title)
		progress(i + 1)
	}
	return b.String(), nil
}

func init() {
	tsx.RegisterClass("app/native", "FocusLog", tsx.Class[FocusLog]{
		New:     func() *FocusLog { return &FocusLog{} },
		Getters: []string{"Summary"},
	})
	tsx.RegisterFunctions("app/native", map[string]any{"exportTasks": exportTasks})
}
