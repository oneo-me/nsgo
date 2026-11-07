package main

import (
	"fmt"
	"math"
	"math/rand/v2"
	"strconv"
	"time"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/base/datatable"
	"github.com/oneo-me/nsgo/core"

	"github.com/oneo-me/nsgo/examples/gallery/locales"
	"github.com/oneo-me/nsgo/ui"
)

// base's DataTable over a million stocks of sixty columns the app does
// not hold, with live quotes as gpui-component's data table story: a tick
// every 33 ms moves a third of the quotes in view. The table reports the
// rows in view (OnVisibleRowsChange), the app formats those and a reach
// around them, and Text reads them, so a scroll or a tick costs what is on
// screen, whatever the number of rows. Rises and falls take their colors
// from the columns' PositiveStyle and NegativeStyle. Sorting is the app's:
// the table reports the next sort, the app orders what it serves and sets
// Sort.

const (
	stockRows = 1_000_000
	stockTick = 33 * time.Millisecond
	// stockPage is the rows formatted and held; a scroll serves another
	// page when the rows in view come within stockMargin of its edge.
	stockPage   = 120
	stockMargin = 10
	indicators  = 39
)

var counters = [][3]string{
	{"AAPL", "Apple", "US"}, {"MSFT", "Microsoft", "US"}, {"NVDA", "NVIDIA", "US"}, {"TSLA", "Tesla", "US"},
	{"AMZN", "Amazon", "US"}, {"GOOG", "Alphabet", "US"}, {"META", "Meta", "US"}, {"0700.HK", "Tencent", "HK"},
	{"9988.HK", "Alibaba", "HK"}, {"3690.HK", "Meituan", "HK"}, {"1810.HK", "Xiaomi", "HK"},
	{"600519.SH", "Moutai", "CN"}, {"300750.SZ", "CATL", "CN"}, {"D05.SI", "DBS", "SG"},
}

// stockQuote is a stock's numbers.
type stockQuote struct {
	price, change, percent, volume, turnover, cap, ttm, bid, bidVolume, ask, askVolume,
	open, prevClose, high, low, shares, ratio, ranking float64
}

// compactQuote appends v in thousands, millions, billions or trillions.
func compactQuote(b []byte, v float64) []byte {
	a := math.Abs(v)
	for _, u := range [...]struct {
		at   float64
		unit byte
	}{{1e12, 'T'}, {1e9, 'B'}, {1e6, 'M'}, {1e3, 'K'}} {
		if a >= u.at {
			return append(strconv.AppendFloat(b, v/u.at, 'f', 2, 64), u.unit)
		}
	}
	return strconv.AppendFloat(b, v, 'f', 2, 64)
}

// signedQuote appends v with its sign and unit.
func signedQuote(b []byte, v float64, unit string) []byte {
	if v > 0 {
		b = append(b, '+')
	}
	return append(strconv.AppendFloat(b, v, 'f', 2, 64), unit...)
}

// stockHash is a number in [0, 1) fixed by id and k: a stock's quote
// before it ticks.
func stockHash(id, k int) float64 {
	x := uint32(id+1)*0x9e3779b1 ^ uint32(k+1)*0x85ebca6b
	x = (x ^ x>>16) * 0x45d9f3b
	x = (x ^ x>>16) * 0x45d9f3b
	return float64(x^x>>16) / 4294967296
}

// requote sets q from r, random numbers in [0, 1) by field.
func (q *stockQuote) requote(r func(k int) float64) {
	q.percent = (r(0) - 0.5) * 0.2
	q.price = 5 + r(1)*994
	q.change = q.price * q.percent
	q.prevClose = q.price - q.change
	q.volume = 1e4 + r(2)*5e7
	q.turnover = q.volume * q.price
	q.cap = q.price * q.shares
	q.ttm = 5 + r(3)*75
	q.bid = q.price * (1 - r(4)*0.01)
	q.bidVolume = 100 + r(5)*5e4
	q.ask = q.price * (1 + r(6)*0.01)
	q.askVolume = 100 + r(7)*5e4
	q.ratio = q.bidVolume / q.askVolume
	q.high = q.price * (1 + r(8)*0.08)
	q.low = q.price * (1 - r(9)*0.08)
	q.ranking = 1 + r(10)*100
}

// openingQuote is stock id's quote as it opened: the same every time.
func openingQuote(id int) *stockQuote {
	q := &stockQuote{shares: 1e8 + stockHash(id, 20)*1e10, open: 5 + stockHash(id, 21)*994}
	q.requote(func(k int) float64 { return stockHash(id, k) })
	return q
}

// stockColumns are the table's columns, titled in tr's language.
func stockColumns(tr locales.Messages) []datatable.Column {
	right := func(key, title string) datatable.Column {
		return datatable.Column{Key: key, Title: title, Width: 110, Align: datatable.AlignEndColumn}
	}
	cols := []datatable.Column{
		{Key: "id", Title: tr.TableId(), Width: 90, Align: datatable.AlignEndColumn, Fixed: true, Sortable: true},
		{Key: "symbol", Title: tr.TableSymbol(), Width: 110, Fixed: true},
		{Key: "name", Title: tr.TableName(), Width: 150},
		{Key: "market", Title: tr.TableMarket(), Width: 80},
		right("price", tr.TablePrice()), right("change", tr.TableChg()), right("percent", tr.TableChg2()), right("volume", tr.TableVolume()),
		right("turnover", tr.TableTurnover()), right("cap", tr.TableMarketCap()), right("ttm", tr.TableTtm()), right("bid", tr.TableBid()),
		right("bidVolume", tr.TableBidVol()), right("ask", tr.TableAsk()), right("askVolume", tr.TableAskVol()), right("open", tr.TableOpen()),
		right("prevClose", tr.TablePrevClose()), right("high", tr.TableHigh()), right("low", tr.TableLow()), right("ratio", tr.TableBidAsk()),
		right("ranking", tr.Table5mRanking()),
	}
	for i := range indicators {
		cols = append(cols, right(fmt.Sprint("x", i+1), tr.TableIndicator(i+1)))
	}
	return cols
}

// toned are the columns whose rises and falls take colors.
func toned(key string) bool {
	return key == "change" || key == "percent" || len(key) > 1 && key[0] == 'x'
}

// quoteRow is the cells of stock id's row with quote q, in the columns'
// order.
func quoteRow(id int, q *stockQuote) []string {
	c := counters[id%len(counters)]
	// The cells are cut from one string: a page's rows are formatted on
	// every scroll and tick.
	const cells = 21 + indicators
	var ends [cells]int
	b := make([]byte, 0, 8*cells)
	n := 0
	cell := func(next []byte) {
		b = next
		ends[n] = len(b)
		n++
	}
	f := func(v float64) { cell(strconv.AppendFloat(b, v, 'f', 2, 64)) }
	cell(strconv.AppendInt(b, int64(id+1), 10))
	cell(append(b, c[0]...))
	cell(strconv.AppendInt(append(append(b, c[1]...), ' '), int64(id+1), 10))
	cell(append(b, c[2]...))
	f(q.price)
	cell(signedQuote(b, q.change, ""))
	cell(signedQuote(b, q.percent*100, "%"))
	cell(compactQuote(b, q.volume))
	cell(compactQuote(b, q.turnover))
	cell(compactQuote(b, q.cap))
	f(q.ttm)
	f(q.bid)
	cell(compactQuote(b, q.bidVolume))
	f(q.ask)
	cell(compactQuote(b, q.askVolume))
	f(q.open)
	f(q.prevClose)
	f(q.high)
	f(q.low)
	f(q.ratio)
	cell(strconv.AppendFloat(b, q.ranking, 'f', 1, 64))
	// Indicators move with the change, each by a share of its own.
	for i := range indicators {
		sign := -1.0
		if i%2 == 1 {
			sign = 1
		}
		cell(signedQuote(b, q.percent*100*sign*(0.25+float64(i%7)*0.2), "%"))
	}
	all := string(b)
	out := make([]string, cells)
	at := 0
	for i, end := range ends {
		out[i], at = all[at:end], end
	}
	return out
}

// stockPageRows is the rows formatted and held, from start.
type stockPageRows struct {
	start int
	rows  [][]string
}

// tableStyles is the gallery's table look, base having none, as its
// StylesFunc.
func tableStyles(cx *core.Scope) func() datatable.Styles {
	return ui.Themed(cx, func() datatable.Styles { return tableParts(cx) })
}

// tableParts are the table's styles, read where ui.ColorsOf's colors are.
func tableParts(cx *core.Scope) datatable.Styles {
	colors := ui.ColorsOf(cx).Now()
	selected := &core.Style{Color: colors.Text, Opacity: core.Alpha(1)}
	chevron := core.Style{Color: colors.Muted, Opacity: core.Alpha(0.6), Selected: selected}
	return datatable.Styles{
		Header:             core.Style{Background: ui.Alpha(colors.Inset, 0.5), Color: colors.Muted, Borders: core.Edges{Bottom: 1}, BorderColor: colors.Border},
		HeaderCell:         core.Style{Padding: core.Edges{Left: 8, Right: 4}},
		HeaderContent:      core.Style{Gap: 4},
		HeaderCellSelected: core.Style{Background: ui.Alpha(colors.Accent, 0.10)},
		HeaderCellDragging: core.Style{Opacity: core.Alpha(0.5)},
		GroupCell:          core.Style{Borders: core.Edges{Bottom: 1, Right: 1}, BorderColor: colors.Border, Padding: core.Symmetric(0, 8)},
		SortIndicator:      core.Style{CornerRadius: 4, Color: colors.Muted, Hover: &core.Style{Background: ui.Alpha(colors.Text, 0.10)}},
		SortAscending:      chevron,
		SortDescending:     chevron,
		ResizeHandle:       core.Style{Background: colors.Border},
		MoveIndicator:      core.Style{Background: colors.Accent},
		Row:                core.Style{Borders: core.Edges{Bottom: 1}, BorderColor: ui.Alpha(colors.Border, 0.6)},
		RowOdd:             core.Style{Background: ui.Alpha(colors.Inset, 0.35)},
		RowSelected:        core.Style{Background: ui.Alpha(colors.Accent, 0.12)},
		RowActive:          core.Style{Background: ui.Alpha(colors.Inset, 0.6)},
		Cell:               core.Style{Padding: core.Symmetric(0, 8)},
		CellSelected:       core.Style{BorderWidth: 1, BorderColor: colors.Accent},
		CellColumnSelected: core.Style{Background: ui.Alpha(colors.Accent, 0.08)},
		CellActive:         core.Style{BorderWidth: 1, BorderColor: ui.Alpha(colors.Accent, 0.5)},
		RowHeader:          core.Style{Color: colors.Muted},
		FixedEdge:          core.Style{Background: colors.Border},
	}
}

// dataTableDemo is a million stocks of sixty columns whose quotes in view
// tick every 33 ms, the ID and Symbol columns fixed at the left, the ID
// column sortable, rows, cells or columns selected, columns resized and
// moved.
func dataTableDemo(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	mode := core.NewSignal(cx, "row")
	sort := core.NewSignal(cx, datatable.Sort{})
	page := core.NewSignalFunc(cx, stockPageRows{}, func(a, b stockPageRows) bool { return false })
	selection := core.NewSignal(cx, datatable.NoTableSelection)
	visible := core.NewSignal(cx, [2]int{})
	stripe, live, ticks := core.NewSignal(cx, true), core.NewSignal(cx, true), core.NewSignal(cx, 0)
	table := &datatable.Ref{}
	around := [2]int{0, 40}
	// The quotes that ticked, by stock: those of the page held. The others
	// show how they opened.
	quotes := map[int]*stockQuote{}
	descending := func() bool { s := sort.GetUntracked(); return s.Key == "id" && s.Descending }
	stockAt := func(at int) int {
		if descending() {
			return stockRows - 1 - at
		}
		return at
	}
	quoteOf := func(id int) *stockQuote {
		if q, ok := quotes[id]; ok {
			return q
		}
		return openingQuote(id)
	}
	// serve formats the page around the rows in view: the rows the page
	// held keep their cells, the others are formatted (all when fresh, the
	// order changed).
	serve := func(fresh bool) {
		span := around[1] - around[0]
		start := max(0, min(stockRows-stockPage, around[0]-(stockPage-span)/2))
		held := page.GetUntracked()
		rows := make([][]string, 0, stockPage)
		for at := start; at < min(stockRows, start+stockPage); at++ {
			if i := at - held.start; !fresh && i >= 0 && i < len(held.rows) {
				rows = append(rows, held.rows[i])
				continue
			}
			id := stockAt(at)
			rows = append(rows, quoteRow(id, quoteOf(id)))
		}
		page.Set(stockPageRows{start, rows})
		// Quotes off the page go back to how they opened, so the app holds
		// what is near the view only.
		for id := range quotes {
			at := id
			if descending() {
				at = stockRows - 1 - id
			}
			if at < start || at >= start+stockPage {
				delete(quotes, id)
			}
		}
	}
	follow := func() {
		held := page.GetUntracked()
		end := held.start + len(held.rows)
		nearStart := held.start > 0 && around[0] < held.start+stockMargin
		nearEnd := end < stockRows && around[1] > end-stockMargin
		if nearStart || nearEnd || around[0] < held.start || around[1] > end {
			serve(false)
		}
	}
	// A third of the rows held move, as in gpui-component's story, and
	// only those are formatted again.
	stop := every(cx, stockTick, func() {
		if !live.GetUntracked() {
			return
		}
		held := page.GetUntracked()
		rows := append([][]string(nil), held.rows...)
		for i := range rows {
			at := held.start + i
			if at%(3+rand.IntN(7)) != 0 {
				continue
			}
			id := stockAt(at)
			q, ok := quotes[id]
			if !ok {
				q = openingQuote(id)
				quotes[id] = q
			}
			q.requote(func(int) float64 { return rand.Float64() })
			rows[i] = quoteRow(id, q)
		}
		page.Set(stockPageRows{held.start, rows})
		ticks.Update(func(n int) int { return n + 1 })
	})
	cx.Cleanup(func() { stop() })
	serve(false)

	// The columns color their rises and falls in the palette shown, which
	// the table follows without resetting their widths or order.
	columnsNow := func() []datatable.Column {
		up, down := colors.Up(), colors.Down()
		titled := stockColumns(tr)
		columns := make([]datatable.Column, len(titled))
		for i, c := range titled {
			c.FitPadding = 16
			if toned(c.Key) {
				c.PositiveStyle, c.NegativeStyle = core.Style{Color: up}, core.Style{Color: down}
			}
			columns[i] = c
		}
		return columns
	}
	columns := core.Untracked(cx, columnsNow)
	describe := func() string {
		s := selection.Get()
		switch {
		case s.Row >= 0 && s.Column >= 0:
			return tr.TableCellRow(itoa(s.Row+1), columns[s.Column].Title)
		case s.Row >= 0:
			return tr.TableRow(itoa(s.Row + 1))
		case s.Column >= 0:
			return tr.TableColumn() + columns[s.Column].Title
		}
		return tr.TableNothingSelected()
	}
	status := func(fn func() string, c func() core.Color) core.Node {
		return core.Text{ContentFunc: fn, Style: core.Style{FontFamily: ui.Mono, FontSize: 12}, StyleFunc: ui.Colored(c)}
	}
	return stagePage(cx, core.View{Style: core.Style{FlexGrow: 1, FlexShrink: 1, MinHeight: core.Pt(0)}, Children: []core.Node{
		datatable.DataTable{
			Ref: table, TestID: "table", Label: tr.TableStocks(),
			Columns:  columnsNow,
			RowCount: func() int { return stockRows },
			Text: func(row, col int) string {
				p := page.Get()
				if i := row - p.start; i >= 0 && i < len(p.rows) && col < len(p.rows[i]) {
					return p.rows[i][col]
				}
				return ""
			},
			RowHeight: 28, FontSize: 13, NoRowHeader: true,
			Stripe:     stripe.Get,
			CellSelect: func() bool { return mode.Get() == "cell" },
			Sort:       sort.Get,
			StylesFunc: tableStyles(cx),
			StyleFunc: func() core.Style {
				return core.Style{FlexGrow: 1, FlexShrink: 1, MinHeight: core.Pt(0), CornerRadius: 8, BorderWidth: 1,
					BorderColor: colors.Border(), Background: colors.Background(), Color: colors.Text()}
			},
			OnSortChange: func(next datatable.Sort) {
				sort.Set(next)
				clear(quotes)
				serve(true)
			},
			OnSelectionChange: selection.Set,
			OnVisibleRowsChange: func(start, end int) {
				visible.Set([2]int{start, end})
				around = [2]int{start, end}
				follow()
			},
		},
	}},
		Section{Title: tr.TerminalOptions(), Note: "datatable.DataTable", Children: []core.Node{
			choiceKnob("CellSelect", mode, ui.Option[string]{Value: "row", Label: tr.TableRows()}, ui.Option[string]{Value: "cell", Label: tr.TableCells()}),
			SwitchRow{Label: tr.TableStripes(), Code: "Stripe", Checked: stripe.Get, OnChange: stripe.Set},
			SwitchRow{Label: tr.TableLive(), Checked: live.Get, OnChange: live.Set},
		}},
		Section{Title: tr.KitActions(), Note: "datatable.Ref", Children: []core.Node{
			ui.Wrap{Children: []core.Node{
				ui.Button{Text: tr.TableRow500000(), Label: tr.TableGoToRow500000(), Variant: ui.ButtonSecondary, Small: true, OnPress: func() { table.ScrollToRow(499_999, false) }},
				ui.Button{Text: tr.TableIndicator30(), Label: tr.TableGoToIndicator30(), Variant: ui.ButtonSecondary, Small: true, OnPress: func() { table.ScrollToColumn("x30", false) }},
			}},
		}},
		Section{Title: tr.TerminalStatus(), Note: "OnVisibleRowsChange", Children: []core.Node{
			status(describe, colors.Text),
			status(func() string {
				v := visible.Get()
				return tr.TableRowsOf(itoa(v[0]+1), itoa(v[1]), visitors(stockRows))
			}, colors.Muted),
			status(func() string { return tr.TableColumnsTick(len(columns), ticks.Get()) }, colors.Muted),
		}})
}

// tableDemo is base's Table.
func tableDemo(cx *core.Scope) core.Node { return demoPage(cx, pageProps{}, tableCard) }

// tableCard is base's Table around a few rows the app writes out, as NSH's
// website shows it: the parts give screen readers a table, the styles its
// look.
func tableCard(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	colors := ui.ColorsOf(cx)
	invoices := [][4]string{{"INV001", tr.TablePaid(), tr.TableCard(), "$250.00"}, {"INV002", tr.TablePending(), tr.TableTransfer(), "$150.00"},
		{"INV003", tr.TableUnpaid(), tr.TableCard(), "$350.00"}, {"INV004", tr.TablePaid(), tr.TableCash(), "$450.00"}}
	widths := []core.Length{core.Pt(84), {}, core.Pt(84), core.Pt(84)}
	line := core.Style{Borders: core.Edges{Bottom: 1}}
	lineColor := func() core.Style { return core.Style{BorderColor: colors.Line()} }
	cell := func(i int, head, end bool, s string) core.Node {
		st := core.Style{Padding: core.Symmetric(10, 12)}
		if i == 1 {
			st.FlexGrow, st.FlexShrink = 1, 1
		} else {
			st.Width = widths[i]
		}
		align := core.TextAlignStart
		if end {
			align = core.TextAlignEnd
		}
		c, w := colors.Text, ui.Regular
		if head {
			c, w = colors.Muted, ui.Medium
		}
		content := []core.Node{core.Text{Content: s, Style: core.Style{FontSize: 13, FontWeight: w, TextAlign: align}, StyleFunc: ui.Colored(c)}}
		if head {
			return base.TableHead{Style: st, Children: content}
		}
		return base.TableCell{Style: st, Children: content}
	}
	var body []core.Node
	for _, inv := range invoices {
		body = append(body, base.TableRow{Style: line, StyleFunc: lineColor, Children: []core.Node{
			cell(0, false, false, inv[0]), cell(1, false, false, inv[1]), cell(2, false, false, inv[2]), cell(3, false, true, inv[3]),
		}})
	}
	var head []core.Node
	for i, title := range []string{tr.TableInvoice(), tr.TableStatus(), tr.TableMethod(), tr.TableAmount()} {
		head = append(head, cell(i, true, i == 3, title))
	}
	return Card{Title: tr.TableTable(), Description: tr.TableAFewRowsTheApp(), Children: []core.Node{
		base.Table{Label: tr.TableRecentInvoices(), Children: []core.Node{
			base.TableHeader{Children: []core.Node{base.TableRow{Style: line, StyleFunc: lineColor, Children: head}}},
			base.TableBody{Children: body},
			base.TableFooter{Children: []core.Node{base.TableRow{Children: []core.Node{
				cell(0, true, false, tr.TableTotal()), cell(1, false, false, ""), cell(2, false, false, ""), cell(3, false, true, "$1,200.00"),
			}}}},
			base.TableCaption{Style: core.Style{Padding: core.Symmetric(8, 12)}, Children: []core.Node{
				core.Text{Content: tr.TableAListOfYourRecent(), Style: core.Style{FontSize: 13}, StyleFunc: ui.Colored(colors.Muted)},
			}},
		}},
	}}
}
