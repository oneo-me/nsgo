// Package bench is what the Go programs of the benchmark share: the lines
// they print for the runner and the cases' data, which the Rust programs'
// crate (../cases) holds the same.
package bench

import (
	"fmt"
	"strconv"
	"time"
)

// The cases' sizes.
const (
	QuoteRows = 40
	QuoteRowH = 18
	QuoteGap  = 1
	ListRows  = 10000
	ListRowH  = 40
)

var (
	ready  bool
	frames int
	since  time.Time
)

// Frame counts a frame that did the case's work. The first prints
// "bench ready", and then each second "bench frames N", the frames of
// that second.
func Frame() {
	now := time.Now()
	if !ready {
		ready = true
		since = now
		fmt.Println("bench ready")
		return
	}
	frames++
	if d := now.Sub(since); d >= time.Second {
		fmt.Println("bench frames " + strconv.FormatFloat(float64(frames)/d.Seconds(), 'f', 1, 64))
		frames = 0
		since = now
	}
}

// Quote is a row of the quotes case.
type Quote struct {
	Symbol string
	Open   float64
	Price  float64
	Volume int
}

// Change is the price's change since the open.
func (q Quote) Change() float64 { return q.Price - q.Open }

// Percent is Change as a percentage of the open.
func (q Quote) Percent() float64 { return (q.Price - q.Open) / q.Open * 100 }

// Quotes is the quotes case's market: its rows and its random numbers,
// mulberry32 from seed 42, as every program's.
type Quotes struct {
	Rows []Quote
	seed uint32
}

// NewQuotes makes the case's rows.
func NewQuotes() *Quotes {
	q := &Quotes{seed: 42}
	q.Rows = make([]Quote, QuoteRows)
	for i := range q.Rows {
		var b [4]byte
		for j := range b {
			b[j] = byte('A' + int(q.next()*26))
		}
		px := 20 + q.next()*480
		q.Rows[i] = Quote{Symbol: string(b[:]), Open: px, Price: px, Volume: int(q.next() * 1000000)}
	}
	return q
}

func (q *Quotes) next() float64 {
	q.seed += 0x6d2b79f5
	t := q.seed
	t = (t ^ (t >> 15)) * (t | 1)
	t ^= t + (t^(t>>7))*(t|61)
	return float64(t^(t>>14)) / 4294967296.0
}

// Tick changes a quarter of the rows, drawn with replacement, and returns
// the rows it changed.
func (q *Quotes) Tick() []int {
	touched := make([]int, 0, QuoteRows/4)
	for range QuoteRows / 4 {
		i := int(q.next() * QuoteRows)
		q.Rows[i].Price *= 1 + (q.next()-0.5)*0.004
		q.Rows[i].Volume += int(q.next() * 500)
		touched = append(touched, i)
	}
	return touched
}
