package diff

import (
	"testing"

	"github.com/oneo-me/nsgo/base/diff"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/nsgotest"
)

// TestViewShowsAChangeInUIsLook mounts a diff in ui's colors.
func TestViewShowsAChangeInUIsLook(t *testing.T) {
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return View{TestID: "diff", Unified: true, OldValue: func() string { return "one\n" }, NewValue: func() string { return "two\n" },
			Style: core.Style{FlexGrow: 1}}
	}, nsgotest.Size(600, 400))
	app.Settle()
	if !app.Exists("diff") {
		t.Fatal(app.Dump())
	}
}

func TestMergeKeepsUIsPartsUnderTheApps(t *testing.T) {
	red := core.Color(0xff0000ff)
	got := merge(diff.Styles{LineAdded: core.Style{Background: red, FontSize: 12}}, diff.Styles{LineAdded: core.Style{FontSize: 14}})
	if got.LineAdded.Background != red || got.LineAdded.FontSize != 14 {
		t.Fatalf("%+v", got.LineAdded)
	}
}
