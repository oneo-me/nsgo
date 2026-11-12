package editor

import (
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

func TestBlockDragSelectsColumns(t *testing.T) {
	ref := &Ref{}
	nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return Editor{Ref: ref, DefaultValue: "abcdef\nab\nabcdef", Style: core.Style{Width: core.Pt(400), Height: core.Pt(300)}}
	})
	e := ref.e
	x := func(col int) core.Points { return core.Points(pad + float32(col)*e.charW) }
	e.pressDown(core.Point{X: x(1), Y: 10}, 1, core.Modifiers{Alt: true, Shift: true})
	e.dragMove(core.Point{X: x(4), Y: 50})
	e.dragEnd()
	got := e.st.sels
	if len(got) != 3 || got[0].span() != (span{pos{0, 1}, pos{0, 4}}) || got[1].span() != (span{pos{1, 1}, pos{1, 2}}) ||
		e.st.primary().head != (pos{2, 4}) {
		t.Fatalf("block: %+v", got)
	}
}
