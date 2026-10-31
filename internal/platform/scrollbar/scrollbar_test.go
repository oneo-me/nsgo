package scrollbar

import (
	"testing"
	"time"

	"github.com/oneo-me/nsgo/internal/driver"
)

func TestInsetsScrollWithTheContent(t *testing.T) {
	c := &driver.ScrollConfig{Behavior: driver.ScrollBehavior{ContentInset: driver.Edges{Top: 20, Bottom: 30}}}
	frame, content := driver.Size{Width: 100, Height: 200}, driver.Size{Width: 100, Height: 800}
	if got := Clamp(c, frame, content, driver.Point{Y: -50}); got.Y != -20 {
		t.Fatalf("clamped the start to %v, want -20", got.Y)
	}
	if got := Clamp(c, frame, content, driver.Point{Y: 900}); got.Y != 630 {
		t.Fatalf("clamped the end to %v, want 630", got.Y)
	}
	// The thumb spans the content and its insets: 850 points.
	b := Bar{Frame: frame, Content: content, Offset: driver.Point{Y: -20}, Inset: c.Behavior.ContentInset}
	if thumb, _ := b.Thumb(); thumb.Origin.Y != 0 {
		t.Fatalf("thumb at the inset's start at %v", thumb.Origin.Y)
	}
	if got := b.Press(driver.Point{X: 95, Y: 1}).Offset(driver.Point{X: 95, Y: 1000}); got.Y != 630 {
		t.Fatalf("dragging to the end scrolls to %v, want 630", got.Y)
	}
}

func TestWheelsSettleOnSnapPoints(t *testing.T) {
	c := &driver.ScrollConfig{Behavior: driver.ScrollBehavior{Paging: true}}
	frame, content := driver.Size{Width: 100, Height: 200}, driver.Size{Width: 100, Height: 1000}
	var w Wheel
	now := time.Now()
	w.Turn(now, 48)
	w.Turn(now.Add(50*time.Millisecond), 48)
	if _, ok := w.Settled(now.Add(100 * time.Millisecond)); ok {
		t.Fatal("settled while turning")
	}
	v, ok := w.Settled(now.Add(250 * time.Millisecond))
	if !ok || v <= 0 || w.Turning() {
		t.Fatalf("settled %v %v", v, ok)
	}
	// A wheel turned down moves on to the next page.
	if to, ok := Settle(c, frame, content, driver.Point{Y: 96}, v); !ok || to.Y != 200 {
		t.Fatalf("settled at %v %v, want 200", to, ok)
	}
	// A released thumb settles at the nearest.
	if to, _ := Settle(c, frame, content, driver.Point{Y: 260}, 0); to.Y != 200 {
		t.Fatalf("released at %v, want 200", to)
	}
	if _, ok := Settle(&driver.ScrollConfig{}, frame, content, driver.Point{Y: 96}, v); ok {
		t.Fatal("a free view snapped")
	}
}

func TestThumbFollowsTheOffset(t *testing.T) {
	b := Bar{Frame: driver.Size{Width: 100, Height: 200}, Content: driver.Size{Width: 100, Height: 800}}
	thumb, ok := b.Thumb()
	want := driver.Bounds{Origin: driver.Point{X: 91}, Size: driver.Size{Width: Thickness, Height: 50}}
	if !ok || thumb != want {
		t.Fatalf("thumb = %v %v, want %v", thumb, ok, want)
	}
	b.Offset.Y = 600
	if thumb, _ := b.Thumb(); thumb.Origin.Y != 150 {
		t.Fatalf("thumb at the end starts at %v, want 150", thumb.Origin.Y)
	}
	if _, ok := (Bar{Frame: b.Frame, Content: b.Frame}).Thumb(); ok {
		t.Fatal("content that fits has a thumb")
	}
}

func TestContainsTheStripAlongTheEdge(t *testing.T) {
	b := Bar{Horizontal: true, Frame: driver.Size{Width: 200, Height: 100}, Content: driver.Size{Width: 400, Height: 100}}
	if !b.Contains(driver.Point{X: 150, Y: 95}) || b.Contains(driver.Point{X: 150, Y: 50}) || b.Contains(driver.Point{X: 250, Y: 95}) {
		t.Fatal("the bar's strip is not its bottom edge")
	}
}

func TestDragMovesTheContentByTheThumb(t *testing.T) {
	b := Bar{Frame: driver.Size{Width: 100, Height: 200}, Content: driver.Size{Width: 100, Height: 800}}
	d := b.Press(driver.Point{X: 95, Y: 10}) // on the thumb, 0 to 50
	// The thumb travels 150 for 600 of content.
	if got := d.Offset(driver.Point{X: 95, Y: 40}); got.Y != 120 {
		t.Fatalf("dragging 30 scrolls to %v, want 120", got.Y)
	}
	if got := d.Offset(driver.Point{X: 95, Y: 1000}); got.Y != 600 {
		t.Fatalf("dragging past the end scrolls to %v, want 600", got.Y)
	}
	// Off the thumb, it centers under the pointer.
	d = b.Press(driver.Point{X: 95, Y: 100})
	if got := d.Offset(driver.Point{X: 95, Y: 100}); got.Y != 300 {
		t.Fatalf("pressing the middle scrolls to %v, want 300", got.Y)
	}
}

func TestIndicatorIsMacOSs(t *testing.T) {
	b := Bar{Frame: driver.Size{Width: 100, Height: 300}, Content: driver.Size{Width: 100, Height: 600}}
	rect, travel, ok := b.Indicator()
	// The track is 296 long, the thumb 148, and it starts a margin in.
	want := driver.Bounds{Origin: driver.Point{X: 91, Y: 2}, Size: driver.Size{Width: Thickness, Height: 148}}
	if !ok || rect != want || travel != 148 {
		t.Fatalf("indicator = %v %v %v, want %v and 148", rect, travel, ok, want)
	}
	b.Offset.Y = 300
	if rect, _, _ := b.Indicator(); rect.Origin.Y != 150 {
		t.Fatalf("indicator at the end starts at %v, want 150", rect.Origin.Y)
	}
	// Pulled past the start, it shrinks; long content stops at the minimum.
	b.Offset.Y = -40
	if rect, _, _ := b.Indicator(); rect.Size.Height >= 148 {
		t.Fatalf("pulled indicator is %v long, want shorter than 148", rect.Size.Height)
	}
	long := Bar{Frame: b.Frame, Content: driver.Size{Width: 100, Height: 1e6}}
	if rect, _, _ := long.Indicator(); rect.Size.Height != indicatorMin {
		t.Fatalf("indicator on long content is %v long, want %v", rect.Size.Height, indicatorMin)
	}
	if _, _, ok := (Bar{Frame: b.Frame, Content: b.Frame}).Indicator(); ok {
		t.Fatal("content that fits has an indicator")
	}
}

func TestIndicatorDragMovesTheContentByTheThumb(t *testing.T) {
	b := Bar{Frame: driver.Size{Width: 100, Height: 300}, Content: driver.Size{Width: 100, Height: 600}}
	d := b.PressIndicator(driver.Point{X: 95, Y: 10}) // on the thumb, 2 to 150
	// The thumb travels 148 for 300 of content.
	if got := d.Offset(driver.Point{X: 95, Y: 84}); got.Y != 150 {
		t.Fatalf("dragging 74 scrolls to %v, want 150", got.Y)
	}
	if got := d.Offset(driver.Point{X: 95, Y: 1000}); got.Y != 300 {
		t.Fatalf("dragging past the end scrolls to %v, want 300", got.Y)
	}
	// Off the thumb, it centers under the pointer, margin aside.
	d = b.PressIndicator(driver.Point{X: 95, Y: 150})
	if got := d.Offset(driver.Point{X: 95, Y: 150}); got.Y != 150 {
		t.Fatalf("pressing the middle scrolls to %v, want 150", got.Y)
	}
}
