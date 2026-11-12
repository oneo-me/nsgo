//go:build darwin

package apple

import (
	"strconv"
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// A cell of the quotes case: four digits and a sign change, 13pt.
func benchText(v float64) *driver.AttributedText {
	return &driver.AttributedText{
		Text:  strconv.FormatFloat(v, 'f', 2, 64),
		Style: driver.TextStyle{Font: driver.Font{Size: 13}, Color: driver.RGB(0x111111)},
	}
}

// BenchmarkPrepareSameText prepares the same text and style over and
// over, which is what a table of numbers does every frame: the row's
// value changes, its font does not.
func BenchmarkPrepareSameText(b *testing.B) {
	var e TextEngine
	t := benchText(123.45)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p := e.Prepare(t)
		p.Measure(driver.Points(90))
		p.Release()
	}
}

// BenchmarkPrepareManyTexts prepares 41 distinct texts, as one frame of
// the quotes case does (10 rows x 4 changing cells + the header).
func BenchmarkPrepareManyTexts(b *testing.B) {
	var e TextEngine
	texts := make([]*driver.AttributedText, 41)
	for i := range texts {
		texts[i] = benchText(100 + float64(i))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, t := range texts {
			p := e.Prepare(t)
			p.Measure(driver.Points(90))
			p.Release()
		}
	}
}
