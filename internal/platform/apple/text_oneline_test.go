//go:build darwin

package apple

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"

	"github.com/oneo-me/nsgo/internal/driver"
)

// TestOneLineMatchesTheFramesetter lays texts on one line out both ways,
// as a CTLine and through a framesetter: their sizes, pixels, lines and
// carets are the same.
func TestOneLineMatchesTheFramesetter(t *testing.T) {
	defer func() { oneLineFast = true }()
	texts := []string{"1810.HK", "Xiaomi 921", "+34.45", "-9.27%", "6.87M", "abc ", " lead", "Ünïcødé ﬁ", "שלום world", "日本語のテキスト", "😀 emoji", ""}
	for _, text := range texts {
		for _, align := range []driver.TextAlign{driver.TextAlignStart, driver.TextAlignCenter, driver.TextAlignEnd, driver.TextAlignJustify} {
			for _, rtl := range []bool{false, true} {
				for _, st := range []driver.TextStyle{
					{Font: driver.Font{Size: 13, Weight: 400}},
					{Font: driver.Font{Size: 17.5, Weight: 600, Italic: true}},
					{Font: driver.Font{Family: "Menlo", Size: 12, Weight: 400}, LetterSpacing: 0.5},
					{Font: driver.Font{Size: 15, Weight: 400, Variant: driver.FontTabularNums}},
				} {
					for _, lines := range []int{0, 1} {
						st.Color, st.Align, st.RTL = driver.RGB(0), align, rtl
						at := driver.AttributedText{Text: text, Style: st, MaxLines: lines}
						name := fmt.Sprintf("%q/%v/rtl=%v/%v/%d", text, align, rtl, st.Font, lines)
						compareOneLine(t, name, &at)
					}
				}
			}
		}
	}
}

func compareOneLine(t *testing.T, name string, at *driver.AttributedText) {
	t.Helper()
	type result struct {
		auto, wide, narrow driver.Size
		pixels, cut        []byte
		lines              []driver.TextLine
		carets             []driver.Bounds
	}
	run := func(fast bool) result {
		oneLineFast = fast
		p := TextEngine{}.Prepare(at)
		defer p.Release()
		var r result
		r.auto = p.Measure(driver.Auto)
		r.wide = p.Measure(r.auto.Width + 40)
		// Too narrow: the framesetter wraps or cuts it either way.
		half := driver.Points(int(r.auto.Width/2) + 1)
		r.narrow = p.Measure(half)
		if r.narrow.Width > 0 && r.narrow.Height > 0 {
			img, _ := renderWith(t, p, driver.Size{Width: half, Height: r.narrow.Height + 2})
			r.cut = img.Pix
		}
		size := driver.Size{Width: r.auto.Width + 30, Height: r.auto.Height + 4}
		img, l := renderWith(t, p, size)
		r.pixels = img.Pix
		r.lines = l.TextLines()
		for i := 0; i <= len(at.Text); i++ {
			if rect, ok := l.CaretRect(driver.Caret{Offset: i}); ok {
				r.carets = append(r.carets, rect)
			}
		}
		return r
	}
	slow, fast := run(false), run(true)
	if slow.narrow != fast.narrow || !bytes.Equal(slow.cut, fast.cut) {
		t.Errorf("%s: a narrow box differs from the framesetter's", name)
	}
	if slow.auto != fast.auto || slow.wide != fast.wide {
		t.Errorf("%s: sizes %v %v, want the framesetter's %v %v", name, fast.auto, fast.wide, slow.auto, slow.wide)
	}
	if !bytes.Equal(slow.pixels, fast.pixels) {
		t.Errorf("%s: pixels differ from the framesetter's", name)
	}
	if !reflect.DeepEqual(slow.lines, fast.lines) {
		t.Errorf("%s: lines %+v, want %+v", name, fast.lines, slow.lines)
	}
	if !reflect.DeepEqual(slow.carets, fast.carets) {
		t.Errorf("%s: carets %v, want %v", name, fast.carets, slow.carets)
	}
}
