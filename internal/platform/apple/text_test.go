//go:build darwin

package apple

import (
	"image"
	"image/png"
	"os"
	"strings"
	"testing"
	"unsafe"

	"github.com/ebitengine/purego"

	"github.com/oneo-me/nsgo/internal/driver"
)

// render draws text laid out at size into a white RGBA bitmap, as a
// flipped view's drawing does.
func render(t *testing.T, text *driver.AttributedText, size driver.Size) (*image.RGBA, *Layout) {
	t.Helper()
	cg := load("/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics")
	var (
		colorSpace func() uintptr
		bitmap     func(data unsafe.Pointer, w, h, bits, row, space uintptr, info uint32) uintptr
	)
	purego.RegisterLibFunc(&colorSpace, cg, "CGColorSpaceCreateDeviceRGB")
	purego.RegisterLibFunc(&bitmap, cg, "CGBitmapContextCreate")
	w, h := int(size.Width), int(size.Height)
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = 0xff
	}
	space := colorSpace()
	defer Release(space)
	const premultipliedLast = 1
	ctx := bitmap(unsafe.Pointer(&img.Pix[0]), uintptr(w), uintptr(h), 8, uintptr(img.Stride), space, premultipliedLast)
	defer Release(ctx)
	p := TextEngine{}.Prepare(text)
	defer p.Release()
	return draw(t, ctx, img, p, size)
}

// renderWith is render of a text prepared already.
func renderWith(t *testing.T, p driver.PreparedText, size driver.Size) (*image.RGBA, *Layout) {
	t.Helper()
	cg := load("/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics")
	var (
		colorSpace func() uintptr
		bitmap     func(data unsafe.Pointer, w, h, bits, row, space uintptr, info uint32) uintptr
	)
	purego.RegisterLibFunc(&colorSpace, cg, "CGColorSpaceCreateDeviceRGB")
	purego.RegisterLibFunc(&bitmap, cg, "CGBitmapContextCreate")
	w, h := int(size.Width), int(size.Height)
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = 0xff
	}
	space := colorSpace()
	defer Release(space)
	const premultipliedLast = 1
	ctx := bitmap(unsafe.Pointer(&img.Pix[0]), uintptr(w), uintptr(h), 8, uintptr(img.Stride), space, premultipliedLast)
	defer Release(ctx)
	return draw(t, ctx, img, p, size)
}

func draw(t *testing.T, ctx uintptr, img *image.RGBA, p driver.PreparedText, size driver.Size) (*image.RGBA, *Layout) {
	t.Helper()
	w, h := int(size.Width), int(size.Height)
	l := p.Layout(size).(*Layout)
	t.Cleanup(l.Release)
	// The bitmap's rows go top down in memory but its space is y up:
	// flip it as a flipped view's is.
	cgContextTranslateCTM(ctx, 0, float64(h))
	cgContextScaleCTM(ctx, 1, -1)
	l.DrawBackgrounds(ctx, Rect(0, 0, 0, 0))
	l.Draw(ctx, float64(h), Rect(0, 0, float64(w), float64(h)), true)
	if dir := os.Getenv("NSGO_TEXT_PNG"); dir != "" {
		f, err := os.Create(dir + "/" + strings.ReplaceAll(t.Name(), "/", "-") + ".png")
		if err == nil {
			png.Encode(f, img)
			f.Close()
		}
	}
	return img, l
}

// inked counts the pixels in r darker than white, and those that are red.
func inked(img *image.RGBA, r image.Rectangle) (dark, red int) {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			c := img.RGBAAt(x, y)
			if c.R < 200 || c.G < 200 || c.B < 200 {
				dark++
			}
			if c.R > 180 && c.G < 120 && c.B < 120 {
				red++
			}
		}
	}
	return dark, red
}

func TestTextEllipsizeModes(t *testing.T) {
	text := "Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor"
	for name, mode := range map[string]driver.EllipsizeMode{"tail": driver.EllipsizeTail, "head": driver.EllipsizeHead,
		"middle": driver.EllipsizeMiddle, "clip": driver.EllipsizeClip} {
		t.Run(name, func(t *testing.T) {
			at := driver.AttributedText{Text: text, Style: driver.TextStyle{Font: driver.Font{Size: 14, Weight: 400}, Color: driver.RGB(0)},
				MaxLines: 2, Ellipsize: mode}
			_, l := render(t, &at, driver.Size{Width: 200, Height: 40})
			lines := l.TextLines()
			if len(lines) != 2 {
				t.Fatalf("%d lines, want 2", len(lines))
			}
			if w := lines[1].Bounds.Size.Width; w > 201 {
				t.Errorf("last line %v wide, want within 200", w)
			}
			if lines[1].End != len(text) {
				t.Errorf("last line ends at %d, want the text's end", lines[1].End)
			}
			if lines[0].CapHeight <= 0 || lines[0].XHeight <= 0 || lines[0].Ascender <= 0 {
				t.Errorf("metrics %+v", lines[0])
			}
		})
	}
}

func TestTextDecorationsAndShadow(t *testing.T) {
	red := driver.RGB(0xff0000)
	at := driver.AttributedText{Text: "Wavy", Style: driver.TextStyle{Font: driver.Font{Size: 20, Weight: 400}, Color: driver.RGB(0x000000),
		Decoration: driver.TextDecorationUnderline, DecorationStyle: driver.DecorationWavy, DecorationColor: red}}
	img, _ := render(t, &at, driver.Size{Width: 80, Height: 30})
	if _, r := inked(img, img.Rect); r < 10 {
		t.Errorf("a wavy red underline drew %d red pixels", r)
	}
	plain := driver.AttributedText{Text: "Shadow", Style: driver.TextStyle{Font: driver.Font{Size: 20, Weight: 400}, Color: driver.RGB(0xffffff)}}
	img, _ = render(t, &plain, driver.Size{Width: 90, Height: 30})
	if d, _ := inked(img, img.Rect); d != 0 {
		t.Fatalf("white text drew %d dark pixels", d)
	}
	plain.Shadow = driver.TextShadow{Color: driver.RGB(0x000000), Offset: driver.Point{X: 2, Y: 2}}
	img, _ = render(t, &plain, driver.Size{Width: 90, Height: 30})
	if d, _ := inked(img, img.Rect); d < 20 {
		t.Errorf("a black shadow drew %d dark pixels", d)
	}
}

func TestTextJustified(t *testing.T) {
	at := driver.AttributedText{Text: "aa bb cc dd ee ff gg hh ii jj kk ll mm nn oo pp", Style: driver.TextStyle{
		Font: driver.Font{Size: 14, Weight: 400}, Color: driver.RGB(0), Align: driver.TextAlignJustify}}
	_, l := render(t, &at, driver.Size{Width: 120, Height: 80})
	lines := l.TextLines()
	if len(lines) < 2 {
		t.Fatalf("%d lines, want several", len(lines))
	}
	if w := lines[0].Bounds.Size.Width; w < 118 {
		t.Errorf("first line %v wide, want stretched to 120", w)
	}
	if lines[1].Start != lines[0].End {
		t.Errorf("lines %+v do not follow each other", lines[:2])
	}
}

func TestLayoutsHitTestTheirCharacters(t *testing.T) {
	style := driver.TextStyle{Font: driver.Font{Size: 20}, Color: driver.RGB(0), Align: driver.TextAlignStart}
	text := &driver.AttributedText{Text: "abc def\nghi", Style: style}
	p := TextEngine{}.Prepare(text)
	defer p.Release()
	size := p.Measure(driver.Auto)
	l := p.Layout(size).(*Layout)
	defer l.Release()
	lines := l.TextLines()
	if len(lines) != 2 {
		t.Fatalf("%d lines, want 2", len(lines))
	}
	first := lines[0].Bounds
	// Past the first line's end: before its break, on that line.
	if c := l.HitTest(driver.Point{X: first.Size.Width + 50, Y: first.Origin.Y + 2}); c.Offset != 7 {
		t.Fatalf("past the first line: %+v, want 7", c)
	}
	if c := l.HitTest(driver.Point{X: -5, Y: first.Size.Height + 4}); c.Offset != 8 {
		t.Fatalf("the second line's start: %+v, want 8", c)
	}
	a, ok := l.CaretRect(driver.Caret{Offset: 1})
	b, _ := l.CaretRect(driver.Caret{Offset: 2})
	if !ok || a.Origin.X <= 0 || b.Origin.X <= a.Origin.X || a.Size.Height < 20 {
		t.Fatalf("carets %v and %v should step right, line high", a, b)
	}
	if c := l.HitTest(driver.Point{X: (a.Origin.X+b.Origin.X)/2 + 1, Y: 2}); c.Offset != 2 {
		t.Fatalf("right of the middle of b: %+v, want 2", c)
	}
	rects := l.RangeRects(5, 9, nil)
	if len(rects) != 2 || rects[1].Origin.Y <= rects[0].Origin.Y || rects[0].Origin.X+rects[0].Size.Width < first.Size.Width-0.5 {
		t.Fatalf("a range across the break covers %v, want the rest of the first line and the second's start", rects)
	}
	// Mixed directions: the Hebrew is drawn right to left after "ab".
	mixed := TextEngine{}.Prepare(&driver.AttributedText{Text: "ab אבג cd", Style: style})
	defer mixed.Release()
	ml := mixed.Layout(mixed.Measure(driver.Auto)).(*Layout)
	defer ml.Release()
	if r := ml.RangeRects(1, 5, nil); len(r) != 2 {
		t.Fatalf("a range from b into the Hebrew covers %v, want two boxes", r)
	}
	// A run's last character ends where the next run starts.
	bold := style
	bold.Font.Weight = 700
	styled := TextEngine{}.Prepare(&driver.AttributedText{Text: "NSGo runs apps", Style: style,
		Spans: []driver.TextSpan{{Start: 0, End: 4, Style: bold}}})
	defer styled.Release()
	sl := styled.Layout(styled.Measure(driver.Auto)).(*Layout)
	defer sl.Release()
	apps, _ := sl.CaretRect(driver.Caret{Offset: 10})
	if r := sl.RangeRects(10, 14, nil); len(r) != 1 || r[0].Origin.X < apps.Origin.X-0.5 {
		t.Fatalf("the range of apps covers %v, want one box from x %v", r, apps.Origin.X)
	}
	empty := TextEngine{}.Prepare(&driver.AttributedText{Style: style})
	defer empty.Release()
	el := empty.Layout(empty.Measure(driver.Auto)).(*Layout)
	if r, ok := el.CaretRect(driver.Caret{}); !ok || r.Size.Height <= 0 || el.HitTest(driver.Point{X: 5, Y: 5}).Offset != 0 {
		t.Fatalf("an empty text's caret %v %v", r, ok)
	}
	el.Release()
	unread := p.Layout(size).(*Layout)
	unread.Release()
	if c := unread.HitTest(driver.Point{X: 10, Y: 2}); c != (driver.Caret{}) {
		t.Fatalf("a layout released unread answered %+v", c)
	}
}
