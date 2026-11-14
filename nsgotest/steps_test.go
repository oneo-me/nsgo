package nsgotest

import (
	"fmt"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
)

// failing is a testing.TB whose fatal failures end only the goroutine
// running the test body, so a test can check that a step fails.
type failing struct {
	testing.TB
	mu       sync.Mutex
	failure  string
	cleanups []func()
}

func (f *failing) Helper() {}
func (f *failing) Fatalf(format string, args ...any) {
	f.mu.Lock()
	f.failure = fmt.Sprintf(format, args...)
	f.mu.Unlock()
	runtime.Goexit()
}
func (f *failing) Fatal(args ...any)                 { f.Fatalf("%s", fmt.Sprint(args...)) }
func (f *failing) Errorf(format string, args ...any) { f.Fatalf(format, args...) }
func (f *failing) Cleanup(fn func())                 { f.cleanups = append(f.cleanups, fn) }

// fails runs body and returns how it failed the test, or "".
func fails(t *testing.T, body func(tb testing.TB)) string {
	f := &failing{TB: t}
	done := make(chan struct{})
	go func() {
		defer close(done)
		body(f)
	}()
	<-done
	for i := len(f.cleanups) - 1; i >= 0; i-- {
		f.cleanups[i]()
	}
	return f.failure
}

func TestSelectorsMatchBareTextsAndRegularExpressions(t *testing.T) {
	app := Start(t, func(cx *core.Scope) core.Node {
		return core.View{Children: []core.Node{
			core.View{TestID: "card", Label: "Card", Style: core.Style{Height: core.Pt(40)}, Children: []core.Node{
				core.Text{Content: "Save draft"},
				core.Text{Content: " Save "},
			}},
			core.Text{Content: "Buy milk", Role: core.RoleHeader},
			core.Input{Placeholder: "Search", TestID: "search"},
		}}
	})
	for selector, want := range map[string]int{
		"Save":                    1, // a whole text, spaces around it aside
		"card":                    1, // a test ID first
		"Card":                    1, // then a label
		"text:Save":               2, // text: looks for a part
		"text:/^buy/i":            1,
		"label:/^c/i":             1,
		"role:header /milk$/":     1,
		"text:Search":             1, // an empty field's placeholder
		"id:card >> text:/draft/": 1,
		"text:Save >> nth=1":      1,
		"text:Save >> nth=2":      0,
	} {
		if got := len(app.Query(selector)); got != want {
			t.Errorf("%q matches %d views, want %d", selector, got, want)
		}
	}
	id := app.Find("id:search").ID
	app.Find(fmt.Sprintf("view:%d", id))
	if msg := fails(t, func(tb testing.TB) {
		Start(tb, func(cx *core.Scope) core.Node { return core.View{} }).Find("text:/a/x")
	}); !strings.Contains(msg, "flag") {
		t.Errorf("a bad flag fails with %q", msg)
	}
}

// Views hidden, clipped away, outside the window or without an area are
// not seen; a scroll view's content is, and so is a modal over all.
func TestQueriesSeeWhatAUserSees(t *testing.T) {
	sized := func(id string, h core.Points) core.View {
		return core.View{TestID: id, Style: core.Style{Height: core.Pt(h), Width: core.Pt(50)}}
	}
	app := Start(t, func(cx *core.Scope) core.Node {
		return core.View{Children: []core.Node{
			sized("plain", 10),
			sized("flat", 0),
			core.View{Style: core.Style{Display: core.DisplayNone}, Children: []core.Node{sized("none", 10)}},
			core.View{Style: core.Style{Height: core.Pt(10), OverflowX: core.Hidden, OverflowY: core.Hidden}, Children: []core.Node{
				sized("inside", 10), sized("clipped", 10)}},
			core.Scroll{Style: core.Style{Height: core.Pt(20)}, Children: []core.Node{
				sized("row-1", 15), sized("row-2", 15), sized("row-3", 15)}},
			core.View{Style: core.Style{Position: core.Absolute, Top: core.Pt(900)}, Children: []core.Node{sized("below", 10)}},
		}}
	})
	for id, want := range map[string]bool{
		"plain": true, "flat": false, "none": false, "inside": true, "clipped": false,
		"row-3": true, "below": false,
	} {
		if got := app.Exists("id:" + id); got != want {
			t.Errorf("%s seen = %v, want %v", id, got, want)
		}
	}
}

func TestStepsFailOnManyDisabledOrMissingViews(t *testing.T) {
	root := func(cx *core.Scope) core.Node {
		return core.View{Children: []core.Node{
			core.View{OnPress: func(core.PressEvent) {}, Style: core.Style{Height: core.Pt(20)}, Children: []core.Node{core.Text{Content: "Go"}}},
			core.View{OnPress: func(core.PressEvent) {}, Style: core.Style{Height: core.Pt(20)}, Children: []core.Node{core.Text{Content: "Go"}}},
			core.View{OnPress: func(core.PressEvent) {}, Disabled: func() bool { return true }, Style: core.Style{Height: core.Pt(20)},
				Children: []core.Node{core.Text{Content: "Off"}}},
			core.Input{TestID: "ro", Readonly: true},
		}}
	}
	for selector, want := range map[string]string{
		"text:Go":  "matches 2",
		"text:Off": "disabled",
		"text:No":  "no pressable view",
	} {
		if msg := fails(t, func(tb testing.TB) { Start(tb, root).Press(selector) }); !strings.Contains(msg, want) {
			t.Errorf("pressing %q failed with %q, want %q", selector, msg, want)
		}
	}
	if msg := fails(t, func(tb testing.TB) { Start(tb, root).Type("id:ro", "x") }); !strings.Contains(msg, "disabled") {
		t.Errorf("typing in a read-only field failed with %q", msg)
	}
	Start(t, root).Press("text:Go >> nth=1")
}

func TestPressesAndLongPressesSendWhatPlatformsSend(t *testing.T) {
	var log []string
	var at time.Duration
	var app *App
	note := func(s string) func(core.PressEvent) { return func(core.PressEvent) { log = append(log, s) } }
	app = Start(t, func(cx *core.Scope) core.Node {
		return core.View{Children: []core.Node{
			core.View{TestID: "b", Style: core.Style{Height: core.Pt(40)}, OnPress: note("press"), OnPressDown: note("down"),
				OnPressUp: note("up"), OnLongPress: func(core.PressEvent) {
					log = append(log, "long")
					at = app.Clock.Now().Sub(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
				}, DelayLongPress: 300 * time.Millisecond},
		}}
	})
	app.Press("id:b")
	if !slices.Equal(log, []string{"down", "up", "press"}) {
		t.Fatalf("a press sent %v", log)
	}
	log = nil
	app.LongPress("id:b")
	if !slices.Equal(log, []string{"down", "long", "up"}) || at != 300*time.Millisecond {
		t.Fatalf("a long press sent %v at %v", log, at)
	}
}

func TestDragAtStartsWhereItSays(t *testing.T) {
	var start core.Point
	app := Start(t, func(cx *core.Scope) core.Node {
		return core.View{Style: core.Style{Padding: core.All(20)}, Children: []core.Node{
			core.View{TestID: "pad", Style: core.Style{Height: core.Pt(100)}, OnTouch: func(e core.TouchEvent) {
				if e.Phase == core.TouchStart {
					start = e.Touches[0].Position
				}
			}},
		}}
	})
	app.DragAt("id:pad", core.Point{X: 10, Y: 30}, core.Point{X: 40})
	if start != (core.Point{X: 10, Y: 30}) {
		t.Fatalf("the finger went down at %v", start)
	}
}

func TestLongPressSelectsText(t *testing.T) {
	app := Start(t, func(cx *core.Scope) core.Node {
		return core.View{Children: []core.Node{
			core.Text{Content: "Select me", Selectable: core.SelectableOn},
			core.View{Selectable: true, Children: []core.Node{core.Text{Content: "In an area"}}},
		}}
	})
	app.LongPress("text:Select me")
	app.LongPress("text:In an area")
}

func TestTypingIsKeyByKey(t *testing.T) {
	var keys, changes []string
	var submitted string
	app := Start(t, func(cx *core.Scope) core.Node {
		return core.Input{TestID: "f", MaxLength: 4, DefaultValue: "a",
			OnKeyPress: func(k string) { keys = append(keys, k) },
			OnChange:   func(s string) { changes = append(changes, s) },
			OnSubmit:   func(s string) { submitted = s }}
	})
	app.Type("id:f", "bcde\n")
	if !slices.Equal(keys, []string{"b", "c", "d", "e", "Enter"}) || !slices.Equal(changes, []string{"ab", "abc", "abcd"}) {
		t.Fatalf("keys %q, changes %q", keys, changes)
	}
	if submitted != "abcd" {
		t.Fatalf("submitted %q", submitted)
	}
	keys, changes = nil, nil
	app.Clear("id:f")
	if !slices.Equal(keys, []string{"Backspace"}) || !slices.Equal(changes, []string{""}) {
		t.Fatalf("clearing sent keys %q, changes %q", keys, changes)
	}
}

func TestDragsGoToWhatAFingerMoves(t *testing.T) {
	var log []string
	app := Start(t, func(cx *core.Scope) core.Node {
		rows := make([]core.Node, 20)
		for i := range rows {
			rows[i] = core.View{Style: core.Style{Height: core.Pt(50)}, OnPress: func(core.PressEvent) { log = append(log, "press") },
				OnPressUp: func(core.PressEvent) { log = append(log, "up") }, TestID: fmt.Sprint("row-", i)}
		}
		return core.View{Children: []core.Node{
			core.View{TestID: "card", Style: core.Style{Height: core.Pt(60)},
				OnPan: func(e core.PanEvent) { log = append(log, fmt.Sprintf("pan %v %v", e.Phase, e.Translation.X)) }},
			core.Scroll{TestID: "list", Style: core.Style{Height: core.Pt(300)}, Children: rows},
		}}
	})
	app.Drag("id:card", core.Point{X: 40})
	if len(log) < 2 || log[0] != fmt.Sprintf("pan %v 10", core.GestureBegan) || log[len(log)-1] != fmt.Sprintf("pan %v 40", core.GestureEnded) {
		t.Fatalf("the pan heard %v", log)
	}
	log = nil
	app.Drag("id:row-1", core.Point{Y: -120})
	if got := app.Find("id:list").Offset.Y; got < 100 {
		t.Fatalf("the list scrolled to %v", got)
	}
	if !slices.Equal(log, []string{"up"}) {
		t.Fatalf("the row under the finger heard %v, want its press cancelled", log)
	}
	log = nil
	app.Drag("id:row-4", core.Point{Y: 3})
	if !slices.Equal(log, []string{"up", "press"}) {
		t.Fatalf("a finger that stayed pressed: %v", log)
	}
	before := app.Clock.Now()
	app.Scroll("id:row-5", 100)
	if got := app.Find("id:list").Offset.Y; got < 200 {
		t.Fatalf("scrolled to %v", got)
	}
	if took := app.Clock.Now().Sub(before); took < 10*driver.FrameInterval {
		t.Fatalf("100 points took %v of the clock, want a frame each 10", took)
	}
}

func TestHoverMovesBetweenViews(t *testing.T) {
	var log []string
	hover := func(id string) core.View {
		return core.View{TestID: id, Style: core.Style{Height: core.Pt(20)},
			OnMouseEnter: func(core.MouseEvent) { log = append(log, "in "+id) }, OnMouseLeave: func(core.MouseEvent) { log = append(log, "out "+id) }}
	}
	app := Start(t, func(cx *core.Scope) core.Node {
		return core.View{Children: []core.Node{hover("a"), hover("b")}}
	})
	app.Hover("id:a")
	app.Hover("id:b")
	app.HoverOut()
	if want := []string{"in a", "out a", "in b", "out b"}; !slices.Equal(log, want) {
		t.Fatalf("hovering heard %v, want %v", log, want)
	}
}

func TestAlertsGetDefaultAnswersAndFastForward(t *testing.T) {
	var answered []string
	var fired int
	app := Start(t, func(cx *core.Scope) core.Node {
		cx.AfterFunc(time.Minute, func() { fired++ })
		cx.AfterFunc(2*time.Minute, func() { fired++ })
		return core.View{TestID: "b", Style: core.Style{Height: core.Pt(20)}, OnPress: func(core.PressEvent) {
			cx.ShowAlert(core.AlertConfig{Title: "Sure?", Actions: []core.AlertAction{{Label: "Yes"}, {Label: "No", Role: core.AlertCancel}}},
				func(i int) { answered = append(answered, fmt.Sprint(i)) })
		}}
	})
	app.Press("id:b")
	app.Press("id:b")
	if !slices.Equal(answered, []string{"1"}) {
		t.Fatalf("answered %v, want the first alert cancelled as the next step starts", answered)
	}
	app.OnAlert = func(driver.AlertConfig) int { return 0 }
	app.Press("id:b")
	if len(answered) != 3 || answered[2] != "0" {
		t.Fatalf("answered %v, want OnAlert's answers", answered)
	}
	app.FastForward(5 * time.Minute)
	if fired != 2 {
		t.Fatalf("fast forward fired %d timers", fired)
	}
}

func TestSpanStylesAreRead(t *testing.T) {
	app := Start(t, func(cx *core.Scope) core.Node {
		return core.Text{Style: core.Style{FontSize: 12}, Spans: []core.Span{
			{Content: "plain "}, {Content: "bold", Style: core.Style{FontWeight: core.FontWeightBold, FontSize: 20}},
		}}
	})
	if s := app.SpanStyle("text:bold"); s.Font.Weight != core.FontWeightBold || s.Font.Size != 20 {
		t.Fatalf("bold's style %+v", s)
	}
	if s := app.SpanStyle("text:plain"); s.Font.Size != 12 {
		t.Fatalf("plain's style %+v", s)
	}
}

func TestTabMovesTheFocusBetweenPressablesAndFields(t *testing.T) {
	var log []string
	button := func(name string, noFocus bool) core.Node {
		return core.View{TestID: name, NoFocus: noFocus, Style: core.Style{Height: core.Pt(20)},
			OnPress: func(p core.PressEvent) { log = append(log, fmt.Sprintf("press %s %d", name, p.Clicks)) },
			OnFocus: func() { log = append(log, "focus "+name) },
			OnBlur:  func() { log = append(log, "blur "+name) }}
	}
	a := Start(t, func(cx *core.Scope) core.Node {
		return core.View{Children: []core.Node{
			button("a", false),
			button("skipped", true),
			core.Input{TestID: "field", Style: core.Style{Height: core.Pt(20)}},
			button("b", false),
		}}
	}, Environment(driver.Environment{OS: driver.OSMacOS}))
	a.Key("Tab")
	a.Key("Tab")
	if f := a.Find("field"); !f.Focused {
		t.Fatalf("the second Tab should focus the field\n%s", a.Dump())
	}
	a.Key("Tab")
	a.Key("Space")
	a.Key("shift+Tab")
	a.Key("shift+Tab")
	a.Press("a")
	a.Press("a")
	want := []string{"focus a", "blur a", "focus b", "press b 1", "blur b", "focus a", "blur a", "press a 1", "press a 2"}
	if !slices.Equal(log, want) {
		t.Fatalf("log = %q\nwant  %q", log, want)
	}
}

func TestFocusTargetsAnyKeyboardStop(t *testing.T) {
	pressed := 0
	root := func(cx *core.Scope) core.Node {
		return core.View{Children: []core.Node{
			core.View{TestID: "save", Style: core.Style{Height: core.Pt(20)}, OnPress: func(core.PressEvent) { pressed++ }},
			core.Input{TestID: "name", Style: core.Style{Height: core.Pt(20)}},
			core.View{TestID: "plain", Style: core.Style{Height: core.Pt(20)}},
		}}
	}
	a := Start(t, root, Environment(driver.Environment{OS: driver.OSMacOS}))
	a.Focus("id:save")
	if !a.Find("id:save").Focused {
		t.Fatalf("Focus should focus a pressable view\n%s", a.Dump())
	}
	a.Key("Return")
	if pressed != 1 {
		t.Fatalf("Return pressed the focused button %d times, want once", pressed)
	}
	a.Focus("id:name")
	if !a.Find("id:name").Focused {
		t.Fatalf("Focus should keep focusing text fields\n%s", a.Dump())
	}
	a.Type("id:name", "Ada")
	if got := a.Find("id:name").Text; got != "Ada" {
		t.Fatalf("typed %q, want Ada", got)
	}
	if msg := fails(t, func(tb testing.TB) { Start(tb, root).Focus("id:plain") }); !strings.Contains(msg, "id:plain") || !strings.Contains(msg, "focusable") {
		t.Fatalf("non-focusable Focus failed with %q", msg)
	}
}

func TestAnAppsRefreshIndicatorFollowsThePull(t *testing.T) {
	var refreshing *core.Signal[bool]
	var pull *core.AnimatedValue
	refreshes := 0
	a := Start(t, func(cx *core.Scope) core.Node {
		refreshing = core.NewSignal(cx, false)
		pull = core.NewAnimatedValue(cx, 0)
		return core.View{Style: core.Style{Height: core.Pt(600)}, Children: []core.Node{
			core.Scroll{TestID: "page", Style: core.Style{FlexGrow: 1},
				Behavior:   core.ScrollBehavior{RefreshIndicator: 72},
				Refreshing: refreshing.Get, PullValue: pull,
				OnRefresh: func() { refreshes++; refreshing.Set(true) },
				Children:  []core.Node{core.View{Style: core.Style{Height: core.Pt(2000)}}}},
			core.View{TestID: "indicator", Style: core.Style{Position: core.Absolute, Width: core.Pt(390), Height: core.Pt(72),
				Animated: &core.AnimatedStyle{TranslateY: pull}}},
		}}
	})
	at := func() driver.Points { return driver.Points(a.Find("indicator").Animated.TranslateY) }
	a.Drag("page", driver.Point{Y: 60})
	if refreshes != 0 {
		t.Fatal("a pull short of the indicator's height refreshed")
	}
	if got := at(); got != 0 {
		t.Fatalf("the pull is %v after it sprang back, want 0", got)
	}
	a.Drag("page", driver.Point{Y: 100})
	if refreshes != 1 || at() != 72 {
		t.Fatalf("refreshes %d, pull %v: want one refresh keeping the indicator's 72", refreshes, at())
	}
	refreshing.Set(false)
	a.Settle()
	if at() != 0 {
		t.Fatalf("the pull is %v once refreshed, want 0", at())
	}
}
