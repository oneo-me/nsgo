package watchos

import (
	"testing"

	"github.com/oneo-me/nsgo/internal/driver"
)

func TestBoxOpsDrawBackgroundUnderAndBorderOver(t *testing.T) {
	s := driver.DefaultViewStyle
	s.Background, s.BorderWidth, s.BorderColor, s.CornerRadius = 0x112233ff, 2, 0xffffffff, 8
	back, front := boxOps(&s, 100, 40)
	fills := func(ops []driver.DrawOp, kind driver.DrawKind) int {
		n := 0
		for _, op := range ops {
			if op.Kind == kind {
				n++
			}
		}
		return n
	}
	if fills(back, driver.DrawFill) != 1 || fills(back, driver.DrawStroke) != 0 {
		t.Fatalf("the background draws %d fills and %d strokes, want one fill", fills(back, driver.DrawFill), fills(back, driver.DrawStroke))
	}
	if fills(front, driver.DrawStroke) != 1 {
		t.Fatalf("the border draws %d strokes over the content, want one", fills(front, driver.DrawStroke))
	}
	if back, front := boxOps(&driver.DefaultViewStyle, 100, 40); len(back)+len(front) != 0 || paints(&driver.DefaultViewStyle) {
		t.Fatal("a box of the default style draws")
	}
}

func TestInkGrowsByShadowAndOutline(t *testing.T) {
	s := driver.DefaultViewStyle
	s.ShadowColor, s.ShadowRadius, s.ShadowOffset = 0x00000080, 4, driver.Point{Y: 2}
	ink := inkOf(&s, 50, 50)
	if ink.Origin.X != -8 || ink.Origin.Y != -6 || ink.Size.Width != 66 || ink.Size.Height != 66 {
		t.Fatalf("the shadow's ink is %v, want it 8 points round, moved down 2", ink)
	}
	s = driver.DefaultViewStyle
	s.Outline = driver.Outline{Width: 2, Offset: 1, Color: 0xff0000ff}
	if ink := inkOf(&s, 10, 10); ink.Origin.X != -3 || ink.Size.Width != 16 {
		t.Fatalf("the outline's ink is %v, want 3 points round", ink)
	}
}
