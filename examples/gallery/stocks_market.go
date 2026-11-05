package main

import (
	"fmt"
	"github.com/oneo-me/nsgo/examples/gallery/locales"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/oneo-me/nsgo/core"

	"github.com/oneo-me/nsgo/ui"
)

// A simulated market: fictional companies whose prices walk every 50 ms.
// The clock runs 20× real time (a 50 ms tick is one market second), so a
// one-minute candle closes every three seconds.

const (
	// tickEvery is the time between ticks; perSecond ticks make a real
	// second.
	tickEvery = 50 * time.Millisecond
	perSecond = 20
	// marketOpen and marketClose are 09:30 and 16:00, marketStart the
	// market time the demo starts at, in seconds after midnight.
	marketOpen  = 9.5 * 3600
	marketClose = 16 * 3600
	marketStart = 10*3600 + 12*60
	// session is the minutes of a session.
	session = (marketClose - marketOpen) / 60
	// bookLevels is the levels on each side of the order book.
	bookLevels = 10
	// tapeLength is the trades kept for the tape.
	tapeLength = 40
	// blockSize is the size a trade is a block at, highlighted on the tape.
	blockSize = 8000
	// indexBase is the composite's previous close.
	indexBase = 4120.55
	// issues is the composite's issues, the listing and the rest, for
	// breadth.
	issues = 160
)

type sector string

var sectors = []struct {
	id    sector
	label func(locales.Messages) string
}{{"tech", locales.Messages.StocksTechnology}, {"health", locales.Messages.StocksHealth}, {"finance", locales.Messages.StocksFinance},
	{"consumer", locales.Messages.StocksConsumer}, {"industry", locales.Messages.StocksIndustrials}, {"energy", locales.Messages.StocksEnergy}}

// flowBuckets are the order size buckets of the trade flow, in shares,
// inclusive upper bounds.
var flowBuckets = []struct {
	label func(locales.Messages) string
	note  string
	max   float64
}{{locales.Messages.StocksSmall, "≤ 500", 500}, {locales.Messages.StocksMedium, "≤ 2K", 2000}, {locales.Messages.StocksLarge, "≤ 8K", blockSize - 1},
	{locales.Messages.StocksBlock2, "8K +", math.Inf(1)}}

type quote struct {
	last, open, high, low, prevClose, volume, turnover float64
	// dir is the direction of the last change: 1 up, -1 down.
	dir int
	// changedAt is the tick of the last change, for the price flash.
	changedAt int
}

type candle struct {
	// t is the start in market seconds after midnight, or for days the
	// days before today.
	t          float64
	o, h, l, c float64
	v          float64
	// a is the day's average price at the candle's close.
	a float64
}

type level struct{ price, size float64 }

type orderBook struct{ bids, asks []level }

type trade struct {
	id       int
	t, price float64
	size     float64
	// side is 1 bought at the ask, -1 sold at the bid.
	side int
}

// flow is the volume bought and sold by order size, of the focused stock.
type flow struct{ buy, sell [4]float64 }

type breadth struct {
	up, down, flat int
	// changes is every issue's change in percent, highest first.
	changes []float64
}

type stock struct {
	code, name string
	sector     sector
	// tick is the price step.
	tick  float64
	quote *core.Signal[quote]
	// minutes are today's one-minute candles, changed in place and set on
	// every change.
	minutes *core.Signal[[]candle]
	// days are the days before today, oldest first.
	days []candle
	// sigma is the per-trade volatility, relative to the price; anchor
	// where the price drifts back to.
	sigma, anchor float64
}

// listing is the fictional companies: code, name, sector, previous close
// and volatility.
var listing = []struct {
	code, name string
	sector     sector
	prevClose  float64
	sigma      float64
}{
	{"ORBX", "Orbex Freight", "industry", 48.2, 0.0009},
	{"KLPW", "Kelpwise Foods", "consumer", 21.35, 0.0007},
	{"VLTG", "Voltgrid Energy", "energy", 112.6, 0.0011},
	{"LUMO", "Lumora Optics", "tech", 236.4, 0.0014},
	{"BRXH", "Brixhill Homes", "consumer", 67.9, 0.0008},
	{"QBFG", "Qubit Forge", "tech", 18.42, 0.0022},
	{"FRNH", "Fernhollow Pharma", "health", 91.05, 0.0012},
	{"HLRB", "Halo Robotics", "tech", 154.8, 0.0016},
	{"MNTP", "Mintpath Payments", "finance", 73.15, 0.0013},
	{"CRGM", "Cragmoor Mining", "energy", 33.6, 0.0015},
	{"TDWS", "Tidewell Shipping", "industry", 26.9, 0.001},
	{"NVSM", "Novasem Devices", "tech", 412.3, 0.0018},
	{"PLMR", "Palmara Resorts", "consumer", 58.75, 0.0009},
	{"STRK", "Starkline Rail", "industry", 84.4, 0.0007},
	{"GLDW", "Goldwing Air", "industry", 15.62, 0.0017},
	{"BKSY", "Bookshy Media", "consumer", 39.1, 0.0012},
	{"CLDP", "Cloudpine Software", "tech", 288.0, 0.0015},
	{"RVNB", "Ravenbrook Bank", "finance", 46.85, 0.0006},
	{"SPRT", "Sprout Grocers", "consumer", 12.04, 0.0011},
	{"ZNTH", "Zenith Biolabs", "health", 64.3, 0.0025},
	{"HRBR", "Harborline Water", "energy", 29.75, 0.0005},
	{"FLKS", "Flakestone Snacks", "consumer", 8.66, 0.0014},
}

// mulberry is a seeded generator (mulberry32), so every run starts the
// same day.
func mulberry(seed uint32) func() float64 {
	return func() float64 {
		seed += 0x6d2b79f5
		t := (seed ^ (seed >> 15)) * (1 | seed)
		t = (t + (t^(t>>7))*(61|t)) ^ t
		return float64(t^(t>>14)) / 4294967296
	}
}

// gauss is a standard normal sample (Box–Muller).
func gauss(rand func() float64) float64 {
	u := rand()
	if u == 0 {
		u = 1e-9
	}
	return math.Sqrt(-2*math.Log(u)) * math.Cos(2*math.Pi*rand())
}

func roundTo(p, tick float64) float64 { return math.Max(tick, math.Round(p/tick)*tick) }

// lotSize is lots of 100 shares, mostly small with a long tail.
func lotSize(rand func() float64) float64 {
	u := rand()
	if u == 0 {
		u = 1e-9
	}
	return 100 * math.Ceil(-math.Log(u)*6)
}

// makeStock builds a stock with 120 days of history and today's minutes
// up to the start time.
func makeStock(cx *core.Scope, index int) *stock {
	row := listing[index]
	rand := mulberry(uint32(index*7919 + 17))
	tick := 0.01
	if row.prevClose < 10 {
		tick = 0.005
	}
	days := make([]candle, 0, 120)
	c := row.prevClose
	for d := 120; d >= 1; d-- {
		o := c * (1 - gauss(rand)*row.sigma*6)
		h := math.Max(o, c) * (1 + rand()*row.sigma*14)
		l := math.Min(o, c) * (1 - rand()*row.sigma*14)
		days = append([]candle{{t: float64(d), o: roundTo(o, tick), h: roundTo(h, tick), l: roundTo(l, tick), c: roundTo(c, tick), v: 2e6 + rand()*6e6, a: c}}, days...)
		c = o
	}
	// Today, minute by minute up to the start.
	var minutes []candle
	p := roundTo(row.prevClose*(1+gauss(rand)*row.sigma*4), tick)
	open := p
	high, low, volume, turnover := p, p, 0.0, 0.0
	for t := float64(marketOpen); t < marketStart; t += 60 {
		o := p
		h, l, v := o, o, 0.0
		for s := 0; s < 60; s += 4 {
			p = roundTo(p+gauss(rand)*row.sigma*2*p-0.002*(p-row.prevClose), tick)
			h, l = math.Max(h, p), math.Min(l, p)
			size := lotSize(rand)
			v += size
			volume += size
			turnover += size * p
		}
		minutes = append(minutes, candle{t: t, o: o, h: h, l: l, c: p, v: v, a: turnover / volume})
		high, low = math.Max(high, h), math.Min(low, l)
	}
	never := func(a, b []candle) bool { return false }
	return &stock{code: row.code, name: row.name, sector: row.sector, tick: tick, sigma: row.sigma, anchor: row.prevClose, days: days,
		quote:   core.NewSignal(cx, quote{last: p, open: open, high: high, low: low, prevClose: row.prevClose, volume: volume, turnover: turnover, changedAt: -1}),
		minutes: core.NewSignalFunc(cx, minutes, never)}
}

// market is the simulated market's state, as signals.
type market struct {
	cx     *core.Scope
	stocks []*stock
	// clock is market time in seconds after midnight; ticks the ticks so
	// far; pulse counts real seconds while running, the cadence of slow
	// summaries.
	clock, ticks, pulse *core.Signal[int]
	running             *core.Signal[bool]
	// composite is the composite now, index one value per minute of
	// today, changed in place.
	composite *core.Signal[float64]
	index     *core.Signal[[]float64]
	// breadth is advancers and decliners across the issues, every second.
	breadth *core.Signal[breadth]
	// book, trades and flow are the focused stock's, empty without one.
	book   *core.Signal[orderBook]
	trades *core.Signal[[]trade]
	flow   *core.Signal[flow]
	// stats is engine time per tick, averaged over a second, and quotes
	// changed per second.
	stats *core.Signal[[2]float64]

	rand           func() float64
	others         []float64
	focused        *stock
	depth, targets map[int]float64
	tradeID        int
	stop           func()
	spent          time.Duration
	updates        int
	lastMean       float64
	changes        []float64
}

func newMarket(cx *core.Scope) *market {
	m := &market{cx: cx, rand: mulberry(uint32(time.Now().UnixNano())), depth: map[int]float64{}, targets: map[int]float64{}, changes: make([]float64, issues)}
	for i := range listing {
		m.stocks = append(m.stocks, makeStock(cx, i))
	}
	m.clock, m.ticks, m.pulse = core.NewSignal(cx, marketStart), core.NewSignal(cx, 0), core.NewSignal(cx, 0)
	m.running = core.NewSignal(cx, false)
	m.book = core.NewSignalFunc(cx, orderBook{}, func(a, b orderBook) bool { return false })
	m.trades = core.NewSignalFunc(cx, []trade(nil), func(a, b []trade) bool { return false })
	m.flow = core.NewSignalFunc(cx, flow{}, func(a, b flow) bool { return false })
	m.stats = core.NewSignal(cx, [2]float64{})
	// The composite: the listing's mean change, per minute so far.
	var history []float64
	n := len(m.stocks[0].minutes.GetUntracked())
	for i := range n {
		sum := 0.0
		for _, s := range m.stocks {
			q := s.quote.GetUntracked()
			sum += (s.minutes.GetUntracked()[i].c - q.prevClose) / q.prevClose
		}
		history = append(history, indexBase*(1+sum/float64(len(m.stocks))))
	}
	mean := m.meanChange()
	m.lastMean = mean
	m.composite = core.NewSignal(cx, indexBase*(1+mean))
	m.index = core.NewSignalFunc(cx, history, func(a, b []float64) bool { return false })
	// The issues outside the listing follow the market with their own
	// noise.
	for range issues - len(m.stocks) {
		m.others = append(m.others, mean*100*(0.4+m.rand())+gauss(m.rand)*0.7)
	}
	m.breadth = core.NewSignalFunc(cx, m.measureBreadth(), func(a, b breadth) bool { return false })
	cx.Cleanup(m.halt)
	return m
}

func (m *market) meanChange() float64 {
	sum := 0.0
	for _, s := range m.stocks {
		q := s.quote.GetUntracked()
		sum += (q.last - q.prevClose) / q.prevClose
	}
	return sum / float64(len(m.stocks))
}

func (m *market) measureBreadth() breadth {
	for i, s := range m.stocks {
		q := s.quote.GetUntracked()
		m.changes[i] = (q.last - q.prevClose) / q.prevClose * 100
	}
	copy(m.changes[len(m.stocks):], m.others)
	out := slices.Clone(m.changes)
	slices.SortFunc(out, func(a, b float64) int {
		switch {
		case a > b:
			return -1
		case a < b:
			return 1
		}
		return 0
	})
	b := breadth{changes: out}
	for _, c := range out {
		switch {
		case c > 0.02:
			b.up++
		case c < -0.02:
			b.down++
		}
	}
	b.flat = issues - b.up - b.down
	return b
}

// restingSize is the resting size at a price step of the focused stock;
// each step drifts back to a target, so the book changes smoothly.
func (m *market) restingSize(step int) float64 {
	size, ok := m.depth[step]
	if !ok {
		size = lotSize(m.rand) * (3 + math.Floor(m.rand()*6))
		m.depth[step], m.targets[step] = size, size
	}
	return size
}

// drift moves the resting sizes around at a little toward their targets;
// now and then a target changes, as an order arrives or leaves.
func (m *market) drift(at int) {
	for step := at - bookLevels - 1; step <= at+bookLevels+1; step++ {
		size := m.restingSize(step)
		target := m.targets[step]
		if m.rand() < 0.004 {
			target = lotSize(m.rand) * (3 + math.Floor(m.rand()*6))
			m.targets[step] = target
		}
		m.depth[step] = math.Max(100, size+(target-size)*0.06+gauss(m.rand)*target*0.012)
	}
}

// layBook builds the book's levels around the best bid and ask.
func (m *market) layBook(s *stock, bid, ask float64) {
	b := m.book.GetUntracked()
	if len(b.bids) != bookLevels {
		b = orderBook{bids: make([]level, bookLevels), asks: make([]level, bookLevels)}
	}
	bidStep, askStep := int(math.Round(bid/s.tick)), int(math.Round(ask/s.tick))
	for i := range bookLevels {
		b.bids[i] = level{float64(bidStep-i) * s.tick, math.Round(m.restingSize(bidStep-i)/100) * 100}
		b.asks[i] = level{float64(askStep+i) * s.tick, math.Round(m.restingSize(askStep+i)/100) * 100}
	}
	m.book.Set(b)
}

func bucketOf(size float64) int {
	k := 0
	for size > flowBuckets[k].max {
		k++
	}
	return k
}

func (m *market) step() {
	began := time.Now()
	n := m.ticks.GetUntracked() + 1
	now := m.clock.GetUntracked() + 1
	var tape []trade
	f := m.flow.GetUntracked()
	m.cx.Batch(func() {
		m.ticks.Set(n)
		m.clock.Set(now)
		mean := 0.0
		for _, s := range m.stocks {
			focused := s == m.focused
			q := s.quote.GetUntracked()
			if !focused && m.rand() > 0.3 {
				mean += (q.last - q.prevClose) / q.prevClose
				continue
			}
			last, high, low, volume, turnover := q.last, q.high, q.low, q.volume, q.turnover
			trades := 1
			if focused {
				trades = 1 + int(m.rand()*3)
			}
			side := 1
			for range trades {
				move := gauss(m.rand)*s.sigma*last - 0.003*(last-s.anchor)
				next := roundTo(last+move, s.tick)
				switch {
				case next > last:
					side = 1
				case next < last:
					side = -1
				case m.rand() < 0.5:
					side = 1
				default:
					side = -1
				}
				last = next
				size := lotSize(m.rand)
				// Now and then an institution trades the focused stock in
				// size.
				if focused && m.rand() < 0.035 {
					size *= 8 + math.Floor(m.rand()*16)
				}
				volume += size
				turnover += size * last
				high, low = math.Max(high, last), math.Min(low, last)
				if focused {
					m.tradeID++
					tape = append(tape, trade{id: m.tradeID, t: float64(now), price: last, size: size, side: side})
					if side > 0 {
						f.buy[bucketOf(size)] += size
					} else {
						f.sell[bucketOf(size)] += size
					}
					at := int(math.Round(last / s.tick))
					// A trade takes some of the level; it refills toward its
					// target.
					m.depth[at] = math.Max(100, m.restingSize(at)-size*0.3)
				}
			}
			// The anchor wanders too, so the day has a trend.
			s.anchor += gauss(m.rand) * s.sigma * 0.2 * s.anchor
			dir := q.dir
			switch {
			case last > q.last:
				dir = 1
			case last < q.last:
				dir = -1
			}
			changed := q.changedAt
			if last != q.last {
				changed = n
			}
			s.quote.Set(quote{last: last, open: q.open, high: high, low: low, prevClose: q.prevClose, volume: volume, turnover: turnover, dir: dir, changedAt: changed})
			mean += (last - q.prevClose) / q.prevClose
			m.updates++

			// Today's minute candle.
			minutes := s.minutes.GetUntracked()
			start := float64(now) - math.Mod(float64(now)-marketOpen, 60)
			traded, avg := volume-q.volume, turnover/volume
			if k := len(minutes) - 1; k >= 0 && minutes[k].t == start {
				c := &minutes[k]
				c.h, c.l, c.c, c.v, c.a = math.Max(c.h, last), math.Min(c.l, last), last, c.v+traded, avg
			} else {
				minutes = append(minutes, candle{t: start, o: q.last, h: math.Max(q.last, last), l: math.Min(q.last, last), c: last, v: traded, a: avg})
			}
			s.minutes.Set(minutes)

			if focused {
				// The resting orders drift; the book follows the last
				// trade.
				m.drift(int(math.Round(last / s.tick)))
				if side > 0 {
					m.layBook(s, last-s.tick, last)
				} else {
					m.layBook(s, last, last+s.tick)
				}
			}
		}
		mean /= float64(len(m.stocks))

		// The composite: a new point each market minute, the last one
		// moves.
		lvl := indexBase * (1 + mean)
		m.composite.Set(lvl)
		points := m.index.GetUntracked()
		minute := int((float64(now) - marketOpen) / 60)
		added := minute >= len(points)
		if added {
			points = append(points, lvl)
		} else {
			points[len(points)-1] = lvl
		}
		// The chart redraws five times a second, not on every tick.
		if added || n%4 == 0 {
			m.index.Set(points)
		}

		if len(tape) > 0 {
			slices.Reverse(tape)
			all := append(tape, m.trades.GetUntracked()...)
			m.trades.Set(all[:min(len(all), tapeLength)])
		}

		if n%perSecond == 0 {
			moved := (mean - m.lastMean) * 100
			m.lastMean = mean
			for i := range m.others {
				m.others[i] += moved*(0.5+float64(i%7)/7) + gauss(m.rand)*0.035 - 0.004*m.others[i]
			}
			m.breadth.Set(m.measureBreadth())
			if m.focused != nil {
				m.flow.Set(f)
			}
			m.pulse.Set(m.pulse.GetUntracked() + 1)
		}
	})
	m.spent += time.Since(began)
	if n%perSecond == 0 {
		m.stats.Set([2]float64{float64(m.spent.Microseconds()) / 1000 / perSecond, float64(m.updates)})
		m.spent, m.updates = 0, 0
	}
}

func (m *market) start() {
	if m.stop != nil {
		return
	}
	m.stop = every(m.cx, tickEvery, m.step)
	m.running.Set(true)
}

func (m *market) halt() {
	if m.stop != nil {
		m.stop()
		m.stop = nil
	}
	m.running.Set(false)
}

// focus keeps the book, the tape and the flow of s, and makes it trade
// on every tick.
func (m *market) focus(s *stock) {
	m.focused = s
	m.depth, m.targets = map[int]float64{}, map[int]float64{}
	m.trades.Set(nil)
	if s == nil {
		m.book.Set(orderBook{})
		return
	}
	q := s.quote.GetUntracked()
	m.layBook(s, q.last-s.tick, q.last)
	// The morning so far, split by order size with a slight buy lean.
	shares := [4]float64{0.34, 0.31, 0.21, 0.14}
	var f flow
	for i, sh := range shares {
		f.buy[i] = math.Round(q.volume * sh * (0.48 + m.rand()*0.1))
		f.sell[i] = math.Round(q.volume * sh * (0.44 + m.rand()*0.1))
	}
	m.flow.Set(f)
}

// marketTime is HH:MM:SS of a market time.
func marketTime(seconds float64, withSeconds bool) string {
	s := int(seconds)
	if withSeconds {
		return fmt.Sprintf("%02d:%02d:%02d", s/3600, s%3600/60, s%60)
	}
	return fmt.Sprintf("%02d:%02d", s/3600, s%3600/60)
}

// priceText is a price with the stock's decimals.
func priceText(v float64, s *stock) string {
	if s.tick < 0.01 {
		return fmt.Sprintf("%.3f", v)
	}
	return fmt.Sprintf("%.2f", v)
}

// grouped is 1,234,567.89.
func grouped(v float64, decimals int) string {
	s := fmt.Sprintf("%.*f", decimals, math.Abs(v))
	whole, fraction, _ := strings.Cut(s, ".")
	var b strings.Builder
	for i, r := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	out := b.String()
	if fraction != "" {
		out += "." + fraction
	}
	if v < 0 {
		out = "-" + out
	}
	return out
}

// compactNumber is 1.23M, 45.6K.
func compactNumber(n float64) string {
	a := math.Abs(n)
	switch {
	case a >= 1e9:
		return fmt.Sprintf("%.2fB", n/1e9)
	case a >= 1e6:
		return fmt.Sprintf("%.2fM", n/1e6)
	case a >= 1e4:
		return fmt.Sprintf("%.1fK", n/1e3)
	}
	return fmt.Sprintf("%.0f", n)
}

// signed is +1.23 or -1.23: an explicit sign.
func signed(v float64, decimals int) string {
	if v >= 0 {
		return fmt.Sprintf("+%.*f", decimals, v)
	}
	return fmt.Sprintf("%.*f", decimals, v)
}

// percentOf is the change against the previous close, in percent.
func percentOf(q quote) float64 { return (q.last - q.prevClose) / q.prevClose * 100 }

// toneOf is the rising, falling or flat color, read where ui.ColorsOf's
// colors are.
func toneOf(cx *core.Scope, v float64) core.Color {
	colors := ui.ColorsOf(cx)
	switch {
	case v > 0:
		return colors.Up()
	case v < 0:
		return colors.Down()
	}
	return colors.Muted()
}
