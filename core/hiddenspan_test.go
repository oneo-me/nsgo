package core_test

import (
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

// A hidden span keeps its place, draws nothing, takes no press, and is
// left out of what screen readers and copies read; showing it again
// lays nothing out.
func TestHiddenSpansKeepTheirPlace(t *testing.T) {
	var shown *core.Signal[bool]
	pressed := 0
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		shown = core.NewSignal(cx, false)
		return core.Text{TestID: "reply", Spans: []core.Span{
			{Content: "Hello "},
			{Content: "world", OnPress: func(core.PressEvent) { pressed++ }, StyleFunc: func() core.Style {
				if shown.Get() {
					return core.Style{}
				}
				return core.Style{Visibility: core.VisibilityHidden, Background: core.RGB(0xff0000)}
			}},
		}}
	}, nsgotest.Size(300, 100))
	v := app.Find("id:reply")
	frame := v.Frame
	sp := v.Spans[1]
	if !sp.Hidden || sp.Link || sp.Background.Color != 0 || sp.Style.Color&0xff != 0 {
		t.Fatalf("hidden span %+v", sp)
	}
	if v.Props.Label != "Hello " {
		t.Errorf("screen readers read %q", v.Props.Label)
	}
	if app.Exists("text:Hello world") {
		t.Error("a text selector saw the hidden span")
	}
	if got := driver.CopiedText(v.Text, v.Spans, 0, len(v.Text)); got != "Hello " {
		t.Errorf("a copy takes %q", got)
	}
	shown.Set(true)
	app.Settle()
	v = app.Find("id:reply")
	if v.Frame != frame || v.Spans[1].Hidden || v.Props.Label != "" {
		t.Errorf("shown: frame %v (was %v), span %+v, label %q", v.Frame, frame, v.Spans[1], v.Props.Label)
	}
	app.Find("text:Hello world")
}
