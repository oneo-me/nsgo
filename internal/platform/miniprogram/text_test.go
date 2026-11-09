package miniprogram

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/oneo-me/nsgo/internal/driver"
)

// approx measures every character 0.5 of the font size wide, CJK ones
// the whole size, from the size in the canvas font.
type approx struct{ calls int }

func (a *approx) Width(font, text string) float64 {
	a.calls++
	size := 14.0
	for _, f := range strings.Fields(font) {
		if v, ok := strings.CutSuffix(f, "px"); ok {
			size = 0
			for _, c := range v {
				size = size*10 + float64(c-'0')
			}
		}
	}
	w := 0.0
	for _, r := range text {
		if cjk(r) {
			w += size
		} else {
			w += size / 2
		}
	}
	return w
}

func plain(s string) *driver.AttributedText {
	return &driver.AttributedText{Text: s, Style: driver.TextStyle{Font: driver.Font{Size: 10, Weight: 400}, Color: driver.RGB(0),
		Align: driver.TextAlignStart}}
}

func runs(l driver.TextLayout) []string {
	var out []string
	for _, r := range l.(*TextLayout).runs {
		out = append(out, r.text)
	}
	return out
}

func TestBreaksAfterSpacesAndAroundCJK(t *testing.T) {
	got := func(s string) []string {
		var out []string
		for _, b := range breaks(s) {
			out = append(out, s[b[0]:b[1]])
		}
		return out
	}
	if g := got("hello  world"); strings.Join(g, "|") != "hello  |world" {
		t.Errorf("breaks = %q", g)
	}
	if g := got("你好abc"); strings.Join(g, "|") != "你|好|abc" {
		t.Errorf("breaks = %q", g)
	}
}

func TestLinesWrapGreedilyAndMeasureTheirWidest(t *testing.T) {
	e := NewTextEngine(&approx{})
	p := e.Prepare(plain("hello world"))
	// 5 points a character: "hello" is 25, "hello world" 55.
	if s := p.Measure(driver.Auto); s.Width != 55 || s.Height != 12 {
		t.Fatalf("one line = %+v", s)
	}
	if s := p.Measure(40); s.Width != 25 || s.Height != 24 {
		t.Fatalf("wrapped = %+v", s)
	}
	l := p.Layout(driver.Size{Width: 40, Height: 24})
	if g := runs(l); strings.Join(g, "|") != "hello|world" {
		t.Fatalf("runs = %q", g)
	}
	lines := l.(*TextLayout).TextLines()
	if len(lines) != 2 || lines[1].Bounds.Origin.Y != 12 || lines[1].Start != 6 {
		t.Fatalf("lines = %+v", lines)
	}
}

func TestLongWordsAndCJKBreakBetweenCharacters(t *testing.T) {
	e := NewTextEngine(&approx{})
	l := e.Prepare(plain("你好世界")).Layout(driver.Size{Width: 25, Height: 100})
	if g := runs(l); strings.Join(g, "|") != "你好|世界" {
		t.Fatalf("CJK runs = %q", g)
	}
	l = e.Prepare(plain("abcdefgh")).Layout(driver.Size{Width: 20, Height: 100})
	if g := runs(l); strings.Join(g, "|") != "abcd|efgh" {
		t.Fatalf("long word runs = %q", g)
	}
}

func TestLineLimitEllipsizes(t *testing.T) {
	e := NewTextEngine(&approx{})
	at := plain("one two three four")
	at.MaxLines, at.Ellipsize = 1, driver.EllipsizeTail
	p := e.Prepare(at)
	if s := p.Measure(40); s.Height != 12 {
		t.Fatalf("limited = %+v", s)
	}
	g := strings.Join(runs(p.Layout(driver.Size{Width: 40, Height: 12})), "")
	if !strings.HasSuffix(g, "…") || utf8.RuneCountInString(g)*5 > 40 {
		t.Fatalf("ellipsized = %q", g)
	}
	at.Ellipsize = driver.EllipsizeMiddle
	g = strings.Join(runs(e.Prepare(at).Layout(driver.Size{Width: 40, Height: 12})), "")
	if !strings.Contains(g, "…") || strings.HasSuffix(g, "…") || strings.HasPrefix(g, "…") {
		t.Fatalf("middle = %q", g)
	}
}

func TestSpansKeepTheirStylesOnOneBaseline(t *testing.T) {
	e := NewTextEngine(&approx{})
	at := plain("Hi, ￼Yo")
	big := at.Style
	big.Font.Size, big.Color = 20, driver.RGB(0xff0000)
	at.Spans = []driver.TextSpan{{Start: 7, End: 9, Style: big}}
	at.Attachments = []driver.TextAttachment{{Offset: 4, Size: driver.Size{Width: 10, Height: 10}}}
	l := e.Prepare(at).Layout(driver.Size{Width: 500, Height: 100}).(*TextLayout)
	if len(l.runs) != 2 || l.runs[0].text != "Hi, " || l.runs[1].text != "Yo" || l.runs[1].style != 1 {
		t.Fatalf("runs = %+v", l.runs)
	}
	if !strings.Contains(l.styles[1], "font-size:20px") || !strings.Contains(l.styles[1], "rgba(255,0,0,1)") {
		t.Fatalf("span style = %q", l.styles[1])
	}
	// Each baseline is 0.3 em below the middle of its run's line box.
	b0 := l.runs[0].y + l.runs[0].height/2 + 3
	b1 := l.runs[1].y + l.runs[1].height/2 + 6
	if d := b0 - b1; d > 0.01 || d < -0.01 {
		t.Fatalf("baselines %v and %v", b0, b1)
	}
	box, ok := l.Attachment(0)
	if !ok || box.Origin.X != 20 || box.Origin.Y+10 != b0 {
		t.Fatalf("attachment at %+v (baseline %v)", box, b0)
	}
}

func TestWidthsAreMeasuredOnce(t *testing.T) {
	m := &approx{}
	e := NewTextEngine(m)
	e.Prepare(plain("same words same words"))
	n := m.calls
	e.Prepare(plain("same words same words"))
	if m.calls != n {
		t.Fatalf("measured %d times again", m.calls-n)
	}
}

func TestAlignment(t *testing.T) {
	e := NewTextEngine(&approx{})
	at := plain("ab")
	at.Style.Align = driver.TextAlignCenter
	l := e.Prepare(at).Layout(driver.Size{Width: 30, Height: 12}).(*TextLayout)
	if l.runs[0].x != 10 {
		t.Fatalf("centered at %v", l.runs[0].x)
	}
	at.Style.Align, at.Style.RTL = driver.TextAlignStart, true
	l = e.Prepare(at).Layout(driver.Size{Width: 30, Height: 12}).(*TextLayout)
	if l.runs[0].x != 20 {
		t.Fatalf("right to left at %v", l.runs[0].x)
	}
}

func TestCaretsHitTheCharacters(t *testing.T) {
	e := NewTextEngine(&approx{})
	l := e.Prepare(plain("ab cd")).Layout(driver.Size{Width: 15, Height: 24}).(*TextLayout)
	if c := l.HitTest(driver.Point{X: 6, Y: 18}); c.Offset != 4 {
		t.Fatalf("hit = %+v", c)
	}
}
