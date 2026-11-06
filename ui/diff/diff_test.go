package diff

import (
	"fmt"
	"strings"
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

const oldGreet = `package main

import "fmt"

// greet returns a greeting for name.
func greet(name string) string {
	return "Hello, " + name
}
` + filler + `
func main() {
	fmt.Println(greet("MyGo"))
}
`

const newGreet = `package main

import "fmt"

// greet returns a greeting for each name.
func greet(names ...string) string {
	return "Hello, " + strings.Join(names, " and ")
}
` + filler + `
func main() {
	fmt.Println(greet("MyGo", "Onevra"))
}
`

const filler = "\n// a\n// b\n// c\n// d\n// e\n// f\n// g\n// h\n// i\n// j\n// k\n// l\n"

func TestViewNumbersUnifiedLinesOnceAndTintsTheirGutters(t *testing.T) {
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return View{TestID: "diff", Unified: true, OldValue: func() string { return "one\ntwo\n" }, NewValue: func() string { return "one\nthree\n" },
			Style: core.Style{FlexGrow: 1}}
	}, nsgotest.Size(600, 400))
	app.Settle()
	// Unchanged, deleted and added: a number each.
	if got := len(app.Query("id:diff >> text:2")); got != 2 {
		t.Fatalf("%d numbers 2, want the deleted and added lines' one each\n%s", got, app.Dump())
	}
	if len(app.Query("id:diff >> text:1")) != 1 {
		t.Fatalf("an unchanged line takes one number\n%s", app.Dump())
	}
	number, text := app.Query("id:diff >> text:2")[1].Parent, app.Find("id:diff >> text:three").Parent
	if number.Style.Background == 0 || number.Style.Background != text.Style.Background {
		t.Fatalf("gutter %#x, line %#x\n%s", number.Style.Background, text.Style.Background, app.Dump())
	}
}

func TestViewMarksWordsUnifiedAsBoxesOfTheLinesHeight(t *testing.T) {
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return View{TestID: "diff", Unified: true, OldValue: func() string { return oldGreet }, NewValue: func() string { return newGreet },
			Style: core.Style{FlexGrow: 1}}
	}, nsgotest.Size(800, 600))
	app.Settle()
	line := app.Find("id:diff >> text:/for each name/")
	box := line.Parent.Children[0]
	if box.Style.Background == 0 || box.Style.CornerRadius != 3 || box.Frame.Size.Height != 18 {
		t.Fatalf("word box %+v\n%s", box, app.Dump())
	}
}

func TestViewExpandsCollapsedLinesFromItsGraySeparator(t *testing.T) {
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return View{TestID: "diff", OldValue: func() string { return oldGreet }, NewValue: func() string { return newGreet },
			Style: core.Style{FlexGrow: 1}}
	}, nsgotest.Size(800, 600))
	app.Settle()
	sep := app.Find("id:diff >> text:10 unchanged lines")
	if frame := sep.Parent.Parent; frame.Frame.Size.Height != 24 || frame.Style.CornerRadius != 6 || frame.Style.Background == 0 {
		t.Fatalf("separator %+v", frame)
	}
	app.Press("id:diff >> text:10 unchanged lines")
	app.Settle()
	if app.Exists("id:diff >> text:10 unchanged lines") {
		t.Fatal("the lines stayed collapsed")
	}
}

func TestViewExpandsCollapsedLinesFromAButtonAcrossTheSeparator(t *testing.T) {
	var lines strings.Builder
	for i := range 80 {
		fmt.Fprintf(&lines, "line %d\n", i)
	}
	oldText := lines.String()
	newText := strings.Replace(strings.Replace(oldText, "line 2\n", "two\n", 1), "line 70\n", "seventy\n", 1)
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return View{TestID: "diff", OldValue: func() string { return oldText }, NewValue: func() string { return newText },
			Style: core.Style{FlexGrow: 1}}
	}, nsgotest.Size(800, 600))
	app.Settle()
	button := app.Query("id:diff >> role:button Expand all")[0]
	if button.Frame.Size.Width != 792 || button.Frame.Size.Height != 24 {
		t.Fatalf("button %+v", button.Frame)
	}
	app.Press("id:diff >> role:button Expand all >> nth=0")
	app.Settle()
	if app.Exists("id:diff >> text:61 unchanged lines") {
		t.Fatal(app.Dump())
	}
}
