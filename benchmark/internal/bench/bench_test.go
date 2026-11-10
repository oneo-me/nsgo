package bench

import (
	"fmt"
	"testing"
)

// TestMarket pins the market after 1,000 ticks, which the Rust programs'
// crate (../cases) makes the same.
func TestMarket(t *testing.T) {
	m := NewQuotes()
	for range 1000 {
		m.Tick()
	}
	q := m.Rows[7]
	if got := fmt.Sprintf("%s %.6f %d", q.Symbol, q.Price, q.Volume); got != want {
		t.Errorf("row 7 = %s, want %s", got, want)
	}
}

const want = "FHTW 256.835223 264295"
