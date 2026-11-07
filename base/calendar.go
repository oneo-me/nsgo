package base

import (
	"cmp"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/oneo-me/nsgo/core"
)

const (
	yearsPerPage = 20
	monthsPerRow = 3
	yearsPerRow  = 5
	// calendarWeeks is the most weeks a month shows, calendarMinWeeks the
	// fewest; calendarLit how many signals the keyboard's stop is spread
	// over, more than the days shown.
	calendarWeeks    = 6
	calendarMinWeeks = 4
	calendarLit      = 96
)

// DateValue is a calendar's or a date picker's value: one date, or with
// Range a start and an end. A zero time is unset. A calendar holds days at
// midnight UTC, and a date picker's also hold their time of day.
type DateValue struct {
	Range      bool
	Start, End time.Time
}

// NewDate is the day y-m-d at t, in UTC as date values are.
func NewDate(y int, m time.Month, d int, t TimeOfDay) time.Time {
	return time.Date(y, m, d, t.Hour, t.Minute, t.Second, 0, time.UTC)
}

// IsSet reports whether it holds a date, a range's start.
func (d DateValue) IsSet() bool { return !d.Start.IsZero() }

// Complete reports whether it is a date, or a range with both ends.
func (d DateValue) Complete() bool { return d.IsSet() && (!d.Range || !d.End.IsZero()) }

// Format is the value as format says, strftime's, a range's ends joined by
// " - "; empty until it is complete.
func (d DateValue) Format(format string) string {
	switch {
	case !d.Complete():
		return ""
	case d.Range:
		return strftime(d.Start, format) + " - " + strftime(d.End, format)
	}
	return strftime(d.Start, format)
}

// shaped is d as a single date or a range.
func (d DateValue) shaped(isRange bool) DateValue {
	if !isRange {
		d.End = time.Time{}
	}
	d.Range = isRange
	return d
}

// picked is d after day was pressed: the date, or a range's end from its
// start, else a new range's start.
func (d DateValue) picked(day time.Time) DateValue {
	switch {
	case !d.Range:
		d.Start = day
	case !d.Start.IsZero() && d.End.IsZero() && !day.Before(d.Start):
		d.End = day
	default:
		d.Start, d.End = day, time.Time{}
	}
	return d
}

// dateOnly is d's days, at midnight.
func (d DateValue) dateOnly() DateValue {
	return DateValue{Range: d.Range, Start: midnight(d.Start), End: midnight(d.End)}
}

// midnight is t's day; a zero time stays one.
func midnight(t time.Time) time.Time {
	if t.IsZero() {
		return t
	}
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// ISODate is t as an ISO date, "2026-10-28".
func ISODate(t time.Time) string { return t.Format(time.DateOnly) }

// ParseISODate reads an ISO date.
func ParseISODate(text string) (time.Time, bool) {
	t, err := time.Parse(time.DateOnly, strings.TrimSpace(text))
	return t, err == nil
}

// strftime formats t by format's C conversions: %Y %y %m %d %e %H %I %M %S
// %p %a %A %b %B %j and %%.
func strftime(t time.Time, format string) string {
	var b strings.Builder
	for i := 0; i < len(format); i++ {
		if format[i] != '%' || i+1 == len(format) {
			b.WriteByte(format[i])
			continue
		}
		i++
		switch format[i] {
		case 'Y':
			fmt.Fprintf(&b, "%d", t.Year())
		case 'y':
			fmt.Fprintf(&b, "%02d", t.Year()%100)
		case 'm':
			fmt.Fprintf(&b, "%02d", int(t.Month()))
		case 'd':
			fmt.Fprintf(&b, "%02d", t.Day())
		case 'e':
			fmt.Fprintf(&b, "%2d", t.Day())
		case 'H':
			fmt.Fprintf(&b, "%02d", t.Hour())
		case 'I':
			fmt.Fprintf(&b, "%02d", (t.Hour()+11)%12+1)
		case 'M':
			fmt.Fprintf(&b, "%02d", t.Minute())
		case 'S':
			fmt.Fprintf(&b, "%02d", t.Second())
		case 'p':
			b.WriteString(t.Format("PM"))
		case 'a':
			b.WriteString(t.Format("Mon"))
		case 'A':
			b.WriteString(t.Format("Monday"))
		case 'b':
			b.WriteString(t.Format("Jan"))
		case 'B':
			b.WriteString(t.Format("January"))
		case 'j':
			fmt.Fprintf(&b, "%03d", t.YearDay())
		case '%':
			b.WriteByte('%')
		default:
			b.WriteByte('%')
			b.WriteByte(format[i])
		}
	}
	return b.String()
}

// DaysOfWeek is a calendar's DisabledMatcher of those days, Sunday 0.
func DaysOfWeek(days ...int) func(time.Time) bool {
	return func(t time.Time) bool {
		for _, d := range days {
			if int(t.Weekday()) == d {
				return true
			}
		}
		return false
	}
}

// DatesOutside is a DisabledMatcher of the days before before and after
// after, either zero for no limit.
func DatesOutside(before, after time.Time) func(time.Time) bool {
	return func(t time.Time) bool {
		return !before.IsZero() && t.Before(before) || !after.IsZero() && t.After(after)
	}
}

// DatesBetween is a DisabledMatcher of the days from from to to, either
// zero for no limit.
func DatesBetween(from, to time.Time) func(time.Time) bool {
	return func(t time.Time) bool {
		return (from.IsZero() || !t.Before(from)) && (to.IsZero() || !t.After(to))
	}
}

// CalendarItemKind is what an item of a calendar is.
type CalendarItemKind uint8

const (
	CalendarDay CalendarItemKind = iota
	CalendarPrevious
	CalendarMonthToggle
	CalendarYearToggle
	CalendarNext
	CalendarWeekday
	CalendarMonth
	CalendarYear
)

// CalendarEntry is what RenderItem draws: what an item is, never its
// state, which is the item's Selected and Disabled styles and its
// range-start, range-end, range-middle, today and outside data styles.
type CalendarEntry struct {
	Kind CalendarItemKind
	// Value is a day's number, a month's, a year or a weekday, Sunday 0.
	Value int
	// Date is a day's, as an ISO date.
	Date           string
	Year, Month    int
	Today, Outside bool
}

// CalendarConfig is what a Calendar and a DatePicker's calendar take.
type CalendarConfig struct {
	// NumberOfMonths shown side by side, one when 0.
	NumberOfMonths int
	// FirstDayOfWeek starts the weeks, Sunday 0.
	FirstDayOfWeek int
	// DisabledMatcher says which days take no press, following what it
	// reads.
	DisabledMatcher func(day time.Time) bool
	// YearRange is the years to pick from, the end excluded, 50 years
	// each way from today when zero.
	YearRange [2]int
	// PreviousLabel and NextLabel name the buttons that turn the months.
	PreviousLabel, NextLabel string

	HeaderClass string
	HeaderStyle core.Style
	BodyClass   string
	BodyStyle   core.Style
	MonthClass  string
	MonthStyle  core.Style
	RowClass    string
	RowStyle    core.Style
	ItemClass   string
	ItemStyle   core.Style
	// ItemStyleOf is the style of a kind of item, following what it
	// reads.
	ItemStyleOf func(CalendarItemKind) core.Style
	RenderItem  func(CalendarEntry) core.Node
}

type calendarMode uint8

const (
	calendarDays calendarMode = iota
	calendarMonths
	calendarYears
)

// calendarGrid is a calendar's state: the shown page, the view and where
// the keyboard is. Its days are one Tab stop, the cursor's.
type calendarGrid struct {
	cx       *core.Scope
	config   CalendarConfig
	shown    *core.Signal[DateValue]
	view     *core.Signal[calendarMode]
	page     *core.Signal[time.Time]
	yearPage *core.Signal[int]
	today    time.Time
	// cursor is the day the keyboard is on, focusTo where it goes next.
	cursor, focusTo *core.Signal[time.Time]
	lit             [calendarLit]*core.Signal[time.Time]
	daysFocused     *core.Signal[bool]
	// focusedCell is the cell with the focus, -1 none, and moving says
	// the keyboard goes from one to another.
	focusedCell int
	moving      bool
}

func monthOf(t time.Time) time.Time {
	y, m, _ := t.Date()
	return time.Date(y, m, 1, 0, 0, 0, 0, time.UTC)
}

// addMonths is t n months on, at the end of a month too short for its day.
func addMonths(t time.Time, n int) time.Time {
	first := monthOf(t).AddDate(0, n, 0)
	last := first.AddDate(0, 1, -1).Day()
	return first.AddDate(0, 0, min(t.Day(), last)-1)
}

func newCalendarGrid(cx *core.Scope, config CalendarConfig, value func() DateValue) *calendarGrid {
	y, m, d := cx.WallTime().Date()
	today := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	start := core.Untracked(cx, value)
	page := today
	if start.IsSet() {
		page = start.Start
	}
	g := &calendarGrid{cx: cx, config: config, shown: core.NewSignal(cx, start), view: core.NewSignal(cx, calendarDays),
		page: core.NewSignal(cx, monthOf(page)), yearPage: core.NewSignal(cx, 0), today: today,
		cursor: core.NewSignal(cx, time.Time{}), focusTo: core.NewSignal(cx, time.Time{}), daysFocused: core.NewSignal(cx, false),
		focusedCell: -1}
	for i := range g.lit {
		g.lit[i] = core.NewSignal(cx, time.Time{})
	}
	first := true
	cx.Effect(func() {
		date := value()
		if first {
			first = false
			return
		}
		cx.Untrack(func() { g.show(date) })
	})
	// The stop's two days wake, however many are shown.
	var was time.Time
	cx.Effect(func() {
		stop := g.stop()
		if stop == was {
			return
		}
		old := was
		was = stop
		cx.Untrack(func() {
			cx.Batch(func() {
				if !old.IsZero() {
					g.litOf(old).Set(time.Time{})
				}
				g.litOf(stop).Set(stop)
			})
		})
	})
	return g
}

func (g *calendarGrid) litOf(day time.Time) *core.Signal[time.Time] {
	return g.lit[int(day.Unix()/86400%calendarLit+calendarLit)%calendarLit]
}

func (g *calendarGrid) show(date DateValue) {
	g.shown.Set(date)
	if date.IsSet() {
		g.page.Set(monthOf(date.Start))
	}
}

func (g *calendarGrid) months() int { return max(g.config.NumberOfMonths, 1) }

func (g *calendarGrid) years() [2]int {
	if r := g.config.YearRange; r != [2]int{} {
		return r
	}
	return [2]int{g.today.Year() - 50, g.today.Year() + 50}
}

func (g *calendarGrid) disabled(day time.Time) bool {
	return g.config.DisabledMatcher != nil && g.config.DisabledMatcher(day)
}

func (g *calendarGrid) pick(day time.Time, changed func(DateValue)) {
	if core.Untracked(g.cx, func() bool { return g.disabled(day) }) {
		return
	}
	next := g.shown.GetUntracked().picked(day)
	g.show(next)
	if next.Complete() {
		changed(next)
	}
}

func (g *calendarGrid) shows(day time.Time) bool {
	first := g.page.GetUntracked()
	return !day.Before(first) && day.Before(first.AddDate(0, g.months(), 0))
}

// stop is the day Tab reaches: the cursor's, else the selected one shown,
// else today, else the page's first.
func (g *calendarGrid) stop() time.Time {
	if day := g.cursor.Get(); !day.IsZero() {
		return day
	}
	page := g.page.Get()
	last := page.AddDate(0, g.months(), 0)
	shown := func(d time.Time) bool { return !d.IsZero() && !d.Before(page) && d.Before(last) }
	switch start := g.shown.Get().Start; {
	case shown(start):
		return start
	case shown(g.today):
		return g.today
	}
	return page
}

func (g *calendarGrid) focusDays() { g.focusTo.Set(core.Untracked(g.cx, g.stop)) }

func (g *calendarGrid) moveTo(day time.Time) {
	g.cx.Batch(func() {
		if !g.shows(day) {
			month := monthOf(day)
			if day.Before(g.page.GetUntracked()) {
				g.page.Set(month)
			} else {
				g.page.Set(month.AddDate(0, 1-g.months(), 0))
			}
		}
		g.cursor.Set(day)
		g.focusTo.Set(day)
	})
}

// keys binds the days' keys while one has the focus: the arrows, Home and
// End along the week, Page keys by a month, with Shift by a year, turning
// the months past the shown ones.
func (g *calendarGrid) keys(changed func(DateValue)) core.Node {
	back := func(d time.Time) int { return (int(d.Weekday()) + 7 - g.config.FirstDayOfWeek%7) % 7 }
	step := func(by func(time.Time) time.Time) func() {
		return func() { g.moveTo(by(core.Untracked(g.cx, g.stop))) }
	}
	days := func(n int) func() { return step(func(d time.Time) time.Time { return d.AddDate(0, 0, n) }) }
	months := func(n int) func() { return step(func(d time.Time) time.Time { return addMonths(d, n) }) }
	pick := func() { g.pick(core.Untracked(g.cx, g.stop), changed) }
	return core.Group{
		bindWhile(g.daysFocused.Get, core.KeyNotEditing, true,
			controlKey{"left", true, days(-1)}, controlKey{"right", true, days(1)}),
		bindWhile(g.daysFocused.Get, core.KeyNotEditing, false,
			controlKey{"up", true, days(-7)}, controlKey{"down", true, days(7)},
			controlKey{"home", true, step(func(d time.Time) time.Time { return d.AddDate(0, 0, -back(d)) })},
			controlKey{"end", true, step(func(d time.Time) time.Time { return d.AddDate(0, 0, 6-back(d)) })},
			controlKey{"pageup", true, months(-1)}, controlKey{"pagedown", true, months(1)},
			controlKey{"shift+pageup", true, months(-12)}, controlKey{"shift+pagedown", true, months(12)},
			controlKey{"enter", false, pick}, controlKey{"space", false, pick}),
	}
}

// grow shares a row equally.
var calendarGrow = core.Style{FlexGrow: 1, FlexShrink: 1, FlexBasis: core.Pt(0)}

var calendarRow = core.Style{Direction: core.Row}

// nodes are the header, the body and the keys.
func (g *calendarGrid) nodes(changed func(DateValue)) []core.Node {
	cfg := g.config
	// Leaving the days forgets where the keyboard was.
	g.cx.Effect(func() {
		if !g.daysFocused.Get() {
			g.cx.Untrack(func() { g.cursor.Set(time.Time{}) })
		}
	})
	header := []core.Node{g.nav(CalendarPrevious)}
	if g.months() == 1 {
		header = append(header, g.toggle(CalendarMonthToggle), g.toggle(CalendarYearToggle))
	} else {
		for m := range g.months() {
			header = append(header, g.title(m))
		}
	}
	header = append(header, g.nav(CalendarNext))
	mode := core.NewMemo(g.cx, g.view.Get)
	body := core.Dynamic(func(*core.Scope) core.Node {
		switch mode.Get() {
		case calendarMonths:
			items := make([]core.Node, 12)
			for m := range items {
				items[m] = g.pickerItem(CalendarMonth, m+1)
			}
			return g.rows(items, monthsPerRow)
		case calendarYears:
			return g.yearView()
		}
		months := make([]core.Node, g.months())
		for m := range months {
			months[m] = core.Func(func(cx *core.Scope) core.Node { return g.month(cx, m, changed) })
		}
		return core.View{Class: cfg.BodyClass, Style: calendarRow.Merge(cfg.BodyStyle), Children: months}
	})
	return []core.Node{
		core.View{Class: cfg.HeaderClass, Children: header,
			Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifySpaceBetween}.Merge(cfg.HeaderStyle)},
		body, g.keys(changed),
	}
}

func (g *calendarGrid) rows(items []core.Node, perRow int) core.Node {
	cfg := g.config
	var rows []core.Node
	for len(items) > 0 {
		n := min(perRow, len(items))
		row := append([]core.Node(nil), items[:n]...)
		items = items[n:]
		// A short last row keeps the columns of the others.
		for len(row) < perRow {
			row = append(row, core.View{Style: calendarGrow})
		}
		rows = append(rows, core.View{Class: cfg.RowClass, Style: calendarRow.Merge(cfg.RowStyle), Children: row})
	}
	return core.View{Class: cfg.BodyClass, Style: cfg.BodyStyle, Children: rows}
}

func (g *calendarGrid) yearView() core.Node {
	cfg := g.config
	holds := func(at int) bool { return g.yearPage.Get()+at < g.years()[1] }
	row := func(r int) core.Node {
		cells := make([]core.Node, yearsPerRow)
		for i := range cells {
			at := r*yearsPerRow + i
			cells[i] = core.Show{When: func() bool { return holds(at) },
				Then: func(*core.Scope) core.Node { return g.pickerItem(CalendarYear, at) },
				Else: func(*core.Scope) core.Node { return core.View{Style: calendarGrow} }}
		}
		return core.View{Class: cfg.RowClass, Style: calendarRow.Merge(cfg.RowStyle), Children: cells}
	}
	rows := make([]core.Node, yearsPerPage/yearsPerRow)
	for r := range rows {
		rows[r] = core.Show{When: func() bool { return holds(r * yearsPerRow) }, Then: func(*core.Scope) core.Node { return row(r) }}
	}
	return core.View{Class: cfg.BodyClass, Style: cfg.BodyStyle, Children: rows}
}

// calendarItem is an item of a calendar.
type calendarItem struct {
	entry    func() CalendarEntry
	selected func() bool
	disabled func() bool
	label    func() string
	// content is the text it shows without RenderItem, label when nil.
	content func() string
	// shown holds while it is in a week the month has.
	shown func() bool
	style core.Style
	press func()
}

func (g *calendarGrid) item(i calendarItem) core.View {
	cfg := g.config
	content := i.content
	if content == nil {
		content = i.label
	}
	var child core.Node = core.Text{ContentFunc: content}
	if cfg.RenderItem != nil {
		child = core.Dynamic(func(*core.Scope) core.Node {
			if i.shown != nil && !i.shown() {
				return nil
			}
			return cfg.RenderItem(i.entry())
		})
	}
	v := core.View{Role: core.RoleButton, Disabled: i.disabled, Selected: i.selected, LabelFunc: i.label,
		Class: cfg.ItemClass, Style: i.style.Merge(cfg.ItemStyle), Children: []core.Node{child}}
	if cfg.ItemStyleOf != nil {
		v.StyleFunc = func() core.Style { return cfg.ItemStyleOf(i.entry().Kind) }
	}
	if i.press != nil {
		v.OnPress = func(core.PressEvent) { i.press() }
	}
	return v
}

func (g *calendarGrid) nav(kind CalendarItemKind) core.Node {
	previous := kind == CalendarPrevious
	disabled := core.NewMemo(g.cx, func() bool {
		switch g.view.Get() {
		case calendarMonths:
			return true
		case calendarYears:
			if previous {
				return g.yearPage.Get() <= g.years()[0]
			}
			return g.yearPage.Get()+yearsPerPage >= g.years()[1]
		}
		return false
	})
	label := g.config.NextLabel
	fallback := "›"
	if previous {
		label, fallback = g.config.PreviousLabel, "‹"
	}
	label = cmp.Or(label, fallback)
	return g.item(calendarItem{entry: func() CalendarEntry { return CalendarEntry{Kind: kind} }, disabled: disabled.Get,
		label: func() string { return label },
		press: func() {
			if disabled.GetUntracked() {
				return
			}
			forward := 1
			if previous {
				forward = -1
			}
			if g.view.GetUntracked() == calendarDays {
				g.page.Set(g.page.GetUntracked().AddDate(0, forward, 0))
			} else {
				g.yearPage.Set(g.yearPage.GetUntracked() + forward*yearsPerPage)
			}
		}})
}

func (g *calendarGrid) toggle(kind CalendarItemKind) core.Node {
	month := kind == CalendarMonthToggle
	opens := calendarYears
	if month {
		opens = calendarMonths
	}
	entry := core.NewMemo(g.cx, func() CalendarEntry {
		page := g.page.Get()
		e := CalendarEntry{Kind: kind, Value: page.Year(), Year: page.Year()}
		if month {
			e.Value, e.Month = int(page.Month()), int(page.Month())
		}
		return e
	})
	return g.item(calendarItem{entry: entry.Get, selected: func() bool { return g.view.Get() == opens },
		label: func() string { return strconv.Itoa(entry.Get().Value) },
		press: func() {
			next := opens
			if g.view.GetUntracked() == opens {
				next = calendarDays
			}
			if next == calendarYears {
				// The page of years with the shown one.
				years := core.Untracked(g.cx, g.years)
				year := min(max(g.page.GetUntracked().Year(), years[0]), years[1]-1)
				g.yearPage.Set(years[0] + (year-years[0])/yearsPerPage*yearsPerPage)
			}
			g.view.Set(next)
		}})
}

func (g *calendarGrid) title(offset int) core.Node {
	entry := core.NewMemo(g.cx, func() CalendarEntry {
		page := g.page.Get().AddDate(0, offset, 0)
		return CalendarEntry{Kind: CalendarMonthToggle, Value: int(page.Month()), Year: page.Year(), Month: int(page.Month())}
	})
	return g.item(calendarItem{entry: entry.Get, disabled: func() bool { return true },
		label: func() string { e := entry.Get(); return fmt.Sprintf("%d %d", e.Month, e.Year) }})
}

func (g *calendarGrid) pickerItem(kind CalendarItemKind, at int) core.Node {
	isMonth := kind == CalendarMonth
	value := func() int {
		if isMonth {
			return at
		}
		return g.yearPage.Get() + at
	}
	entry := func() CalendarEntry {
		v := value()
		if isMonth {
			return CalendarEntry{Kind: kind, Value: v, Year: g.page.Get().Year(), Month: v}
		}
		return CalendarEntry{Kind: kind, Value: v, Year: v}
	}
	return g.item(calendarItem{entry: entry, style: calendarGrow,
		selected: func() bool {
			if isMonth {
				return int(g.page.Get().Month()) == value()
			}
			return g.page.Get().Year() == value()
		},
		label: func() string { return strconv.Itoa(value()) },
		press: func() {
			v, p := core.Untracked(g.cx, value), g.page.GetUntracked()
			if isMonth {
				g.page.Set(time.Date(p.Year(), time.Month(v), 1, 0, 0, 0, 0, time.UTC))
			} else {
				g.page.Set(time.Date(v, p.Month(), 1, 0, 0, 0, 0, time.UTC))
			}
			g.view.Set(calendarDays)
		}})
}

// firstShown is the first day month's weeks show.
func firstShown(month time.Time, firstDay int) time.Time {
	return month.AddDate(0, 0, -((int(month.Weekday()) + 7 - firstDay%7) % 7))
}

// weekCount is how many weeks month's show.
func weekCount(month time.Time, firstDay int) int {
	days := int(month.AddDate(0, 1, 0).Sub(firstShown(month, firstDay)) / (24 * time.Hour))
	return (days + 6) / 7
}

// month is the weekdays and the weeks of a month: 42 days made once, a turn
// of the months changing what they show.
func (g *calendarGrid) month(cx *core.Scope, offset int, changed func(DateValue)) core.Node {
	cfg := g.config
	first := cfg.FirstDayOfWeek
	page := core.NewMemo(cx, func() time.Time { return g.page.Get().AddDate(0, offset, 0) })
	start := core.NewMemo(cx, func() time.Time { return firstShown(page.Get(), first) })
	count := core.NewMemo(cx, func() int { return weekCount(page.Get(), first) })
	row := func(children []core.Node, style func() core.Style) core.Node {
		return core.View{Class: cfg.RowClass, Style: calendarRow.Merge(cfg.RowStyle), StyleFunc: style, Children: children}
	}
	weekdays := make([]core.Node, 7)
	for at := range weekdays {
		day := (first + at) % 7
		weekdays[at] = g.item(calendarItem{entry: func() CalendarEntry { return CalendarEntry{Kind: CalendarWeekday, Value: day} },
			disabled: func() bool { return true }, label: func() string { return strconv.Itoa(day) }, style: calendarGrow})
	}
	nodes := []core.Node{row(weekdays, nil)}
	for w := range calendarWeeks {
		var shown func() bool
		var style func() core.Style
		if w >= calendarMinWeeks {
			shown = func() bool { return w < count.Get() }
			style = func() core.Style {
				if !shown() {
					return core.Style{Display: core.DisplayNone}
				}
				return core.Style{}
			}
		}
		days := make([]core.Node, 7)
		for d := range days {
			at := w*7 + d
			days[d] = core.Func(func(cx *core.Scope) core.Node {
				return g.day(cx, dayCell{page: page, start: start, at: at, id: offset*42 + at, shown: shown}, changed)
			})
		}
		nodes = append(nodes, row(days, style))
	}
	return core.View{Class: cfg.MonthClass, Style: calendarGrow.Merge(cfg.MonthStyle), Children: nodes}
}

// dayMarks is what the value makes of a day.
type dayMarks struct{ selected, start, end, middle bool }

func marksOf(v DateValue, day time.Time) dayMarks {
	if !v.Range {
		return dayMarks{selected: v.Start == day}
	}
	if v.Start.IsZero() || v.End.IsZero() {
		// Half a range: its start is selected, and no range shows.
		return dayMarks{selected: v.Start == day}
	}
	inside := !day.Before(v.Start) && !day.After(v.End)
	start, end := day == v.Start, day == v.End
	return dayMarks{selected: inside, start: start, end: end, middle: inside && !start && !end}
}

type dayCell struct {
	page, start *core.Memo[time.Time]
	at, id      int
	shown       func() bool
}

func (g *calendarGrid) day(cx *core.Scope, c dayCell, changed func(DateValue)) core.Node {
	date := core.NewMemo(cx, func() time.Time { return c.start.Get().AddDate(0, 0, c.at) })
	// Not a day of the month around it, which that month shows as its own.
	own := core.NewMemo(cx, func() bool { return date.Get().Month() == c.page.Get().Month() })
	label := func() string { return ISODate(date.Get()) }
	disabled := core.NewMemo(cx, func() bool { return g.disabled(date.Get()) })
	isStop := core.NewMemo(cx, func() bool { d := date.Get(); return own.Get() && g.litOf(d).Get() == d })
	// A pick checks every day and changes those it changed only.
	marks := core.NewMemo(cx, func() dayMarks { return marksOf(g.shown.Get(), date.Get()) })
	entry := core.NewMemo(cx, func() CalendarEntry {
		d, p := date.Get(), c.page.Get()
		return CalendarEntry{Kind: CalendarDay, Value: d.Day(), Date: ISODate(d), Year: p.Year(), Month: int(p.Month()),
			Today: d == g.today, Outside: !own.Get()}
	})
	me := new(core.Ref)
	laidOut := core.NewSignal(cx, false)
	// The keyboard comes here once the day is laid out.
	cx.Effect(func() {
		if !laidOut.Get() || !isStop.Get() || g.focusTo.Get() != date.GetUntracked() {
			return
		}
		cx.Untrack(func() {
			g.focusTo.Set(time.Time{})
			held := g.focusedCell
			if held == c.id {
				// The months turned under the cell with the focus.
				return
			}
			if held >= 0 && !disabled.GetUntracked() {
				g.moving = true
			}
			me.Focus()
		})
	})
	v := g.item(calendarItem{entry: entry.Get, selected: func() bool { return marks.Get().selected }, disabled: disabled.Get,
		label: label, content: func() string { return strconv.Itoa(date.Get().Day()) }, shown: c.shown, style: calendarGrow,
		press: func() { g.pick(date.GetUntracked(), changed) }})
	v.Ref = me
	v.PartStates = func() core.PartStates {
		m, e := marks.Get(), entry.Get()
		var data []string
		for _, d := range []struct {
			on   bool
			name string
		}{{m.start, "range-start"}, {m.end, "range-end"}, {m.middle, "range-middle"}, {e.Today, "today"}, {e.Outside, "outside"}} {
			if d.on {
				data = append(data, d.name)
			}
		}
		return core.PartStates{Data: data}
	}
	v.OnLayout = func(core.Bounds) {
		if !laidOut.GetUntracked() {
			laidOut.Set(true)
		}
	}
	v.Focusable = func() bool { return isStop.Get() && !disabled.Get() }
	v.OnFocus = func() {
		g.moving = false
		g.focusedCell = c.id
		g.cx.Batch(func() {
			g.daysFocused.Set(true)
			g.cursor.Set(date.GetUntracked())
		})
	}
	v.OnBlur = func() {
		if g.focusedCell != c.id && g.focusedCell >= 0 {
			return // another day has the focus already
		}
		g.focusedCell = -1
		// The keyboard moves to another day: the keys stay bound.
		if g.moving {
			g.moving = false
			return
		}
		g.daysFocused.Set(false)
	}
	return v
}

// Calendar is months of days to pick a date or a range from, as React
// Aria's. Value makes the value controlled, with its own from DefaultValue
// otherwise; a range reports once both ends are picked. The toggles turn
// the header into months and years to jump to, and the days are one Tab
// stop whose arrows, Home, End and Page keys move the cursor, turning the
// months. It has no look: the app styles each part or draws the items.
type Calendar struct {
	Value        func() DateValue
	DefaultValue DateValue
	// Range picks a start and an end.
	Range bool
	CalendarConfig
	OnChange func(value DateValue)
	Class    string
	Style    core.Style
	TestID   string
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
}

// Render implements core.Node.
func (c Calendar) Render(cx *core.Scope) core.Node {
	own := core.NewSignal(cx, c.DefaultValue.shaped(c.Range))
	value := func() DateValue {
		if c.Value != nil {
			return c.Value().shaped(c.Range)
		}
		return own.Get()
	}
	g := newCalendarGrid(cx, c.CalendarConfig, value)
	changed := func(d DateValue) {
		if c.Value == nil {
			own.Set(d)
		}
		if c.OnChange != nil {
			c.OnChange(d)
		}
	}
	return core.View{Class: c.Class, Style: c.Style, TestID: c.TestID, OnLayout: c.OnLayout, Tooltip: c.Tooltip, Children: g.nodes(changed)}
}
