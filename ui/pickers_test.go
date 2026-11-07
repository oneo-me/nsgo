package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/base/locales"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"
	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/nsgotest"
)

func fruitOptions() []Option[string] {
	return []Option[string]{{Value: "apple", Label: "Apple"}, {Value: "pear", Label: "Pear"}, {Value: "plum", Label: "Plum"}}
}

// A searchable select filters its rows, shows Empty while none matches and
// clears through OnClear.
func TestSelectSearchesAndClears(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		var fruit *core.Signal[string]
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			fruit = core.NewSignal(cx, "pear")
			return Select[string]{TestID: "fruit", Label: "Fruit", Options: fruitOptions(), Value: fruit.Get, OnChange: fruit.Set,
				Searchable: true, SearchPlaceholder: "Find", Empty: "Nothing", OnClear: func() { fruit.Set("") }}
		})
		app.Press("id:fruit")
		app.Type("role:textbox", "pl")
		if app.Exists("role:option Pear") || !app.Exists("role:option Plum") {
			t.Fatal("the search did not filter the rows")
		}
		app.Type("role:textbox", "zz")
		if !app.Exists("text:Nothing") {
			t.Fatal("no Empty while none matches")
		}
		app.Key("Escape")
		app.Settle()
		app.Advance(time.Second)
		app.Press("label:Clear")
		if fruit.Get() != "" {
			t.Fatalf("value = %q after clearing", fruit.Get())
		}
	})
}

// A combobox of several values toggles them, selecting their rows, and
// stays open.
func TestComboboxToggles(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		var got []string
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			value := core.NewSignalFunc(cx, []string{"apple"}, func(a, b []string) bool { return len(a) == len(b) })
			return Combobox[string]{Options: fruitOptions(), Multiple: true, Label: "Fruit", Value: value.Get,
				OnChange: func(v []string) { got = v; value.Set(v) }}
		})
		app.Press("role:button")
		app.Press("role:option Plum")
		if len(got) != 2 || got[1] != "plum" || !app.Exists("role:option Plum") {
			t.Fatalf("got %v, open %v", got, app.Exists("role:option Plum"))
		}
		if !app.Find("role:option Plum").Props.Selected {
			t.Fatal("a chosen option's row is not selected")
		}
	})
}

func TestCommandRuns(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		ran := ""
		app, _ := start(t, e, func(*core.Scope) core.Node {
			return Command{Label: "Commands", Placeholder: "Find", Empty: "Nothing", Style: core.Style{Height: core.Pt(200)},
				Items: func() []base.CommandItem {
					return []base.CommandItem{{ID: "a", Label: "Alpha", Shortcut: "mod+a"}, {ID: "b", Label: "Beta", Keywords: []string{"second"}}}
				}, OnSelect: func(id string) { ran = id }}
		})
		app.Type("role:textbox", "second")
		app.Press("role:option Beta")
		if ran != "b" {
			t.Fatalf("ran %q", ran)
		}
		app.Type("role:textbox", "zz")
		if !app.Exists("text:Nothing") {
			t.Fatal("no Empty while none matches")
		}
	})
}

func TestCommandShowsWhatAnItemSays(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		app, _ := start(t, e, func(*core.Scope) core.Node {
			return Command{Style: core.Style{Height: core.Pt(200)}, Items: func() []base.CommandItem {
				return []base.CommandItem{{ID: "a", Label: "Alpha", Description: "First letter", Icon: lucide.Settings(), Trailing: Badge{Text: "App"}, Group: "Letters"}}
			}}
		})
		for _, want := range []string{"role:option Alpha", "text:First letter", "text:App", "text:Letters"} {
			app.Find(want)
		}
		if !app.Exists("label:Search commands") {
			t.Fatal("no words of its own")
		}
		app.Type("role:textbox", "zz")
		if !app.Exists("text:No matching commands") {
			t.Fatal("no words of its own for no matches")
		}
	})
}

func TestCommandDialogCancelsAfterClearing(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		var open *core.Signal[bool]
		ran := ""
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			open = core.NewSignal(cx, false)
			return CommandDialog{Open: open.Get, OnOpenChange: func(o bool, _ base.Reason) { open.Set(o) }, Command: Command{OnSelect: func(id string) { ran = id; open.Set(false) },
				Items: func() []base.CommandItem {
					return []base.CommandItem{{ID: "a", Label: "Alpha"}, {ID: "b", Label: "Beta"}}
				}}}
		})
		open.Set(true)
		app.Settle()
		if !app.Find("role:textbox").Focused {
			t.Fatal("the search field has no keyboard")
		}
		app.Type("role:textbox", "be")
		app.Key("escape")
		if app.Find("role:textbox").Text != "" || !open.GetUntracked() {
			t.Fatal("Escape did not clear the query first")
		}
		app.Key("escape")
		if open.GetUntracked() {
			t.Fatal("Escape with no query did not close")
		}
		open.Set(true)
		app.Settle()
		app.Type("role:textbox", "be")
		app.Key("enter")
		if ran != "b" || open.GetUntracked() {
			t.Fatalf("ran %q, open %v", ran, open.GetUntracked())
		}
	})
}

func TestCommandDialogFitsTheFieldUntilASearch(t *testing.T) {
	var open *core.Signal[bool]
	app, _ := start(t, envs()[0].Environment, func(cx *core.Scope) core.Node {
		open = core.NewSignal(cx, true)
		return CommandDialog{Open: open.Get, TestID: "palette", Command: Command{ExpandOnSearch: true, Items: func() []base.CommandItem {
			return []base.CommandItem{{ID: "a", Label: "Alpha"}}
		}}}
	})
	app.Settle()
	field := app.Find("id:palette").Frame.Size.Height
	app.Type("role:textbox", "a")
	if tall := app.Find("id:palette").Frame.Size.Height; field > 100 || tall < 400 {
		t.Fatalf("%v before a search, %v after", field, tall)
	}
}

func TestCalendarPicksADay(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		var got base.DateValue
		app, _ := start(t, e, func(*core.Scope) core.Node {
			return Calendar{DefaultValue: base.DateValue{Start: base.NewDate(2026, 10, 9, base.TimeOfDay{})}, OnChange: func(v base.DateValue) { got = v }}
		})
		if !app.Exists("label:2026-10-09") || !app.Exists("text:October") && !app.Exists("text:Oct") {
			t.Fatal("no October")
		}
		app.Press("label:2026-10-15")
		if d := base.ISODate(got.Start); d != "2026-10-15" {
			t.Fatalf("picked %q", d)
		}
	})
}

// February 2026 takes four weeks, May six, and the months and years replace
// them all: nothing moves.
func TestCalendarKeepsItsSize(t *testing.T) {
	headless(t)
	february := base.DateValue{Start: base.NewDate(2026, 2, 9, base.TimeOfDay{})}
	each(t, func(t *testing.T, e core.Environment) {
		var next string
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			next = "label:" + locales.CalendarNext(cx)
			return Calendar{DefaultValue: february, TestID: "calendar"}
		})
		was := app.Find("id:calendar").Frame
		if was.Size.Width != 224 || was.Size.Height != 232 {
			t.Fatalf("a calendar is %v", was.Size)
		}
		if !app.Exists("label:2026-03-14") {
			t.Fatal("February does not show six weeks")
		}
		// Each toggle opens its view, and the first of its label closes it.
		for _, p := range []string{next, next, next, "label:5", "label:5 >> nth=0", "label:2026", "label:2026 >> nth=0"} {
			app.Press(p)
			if now := app.Find("id:calendar").Frame; now != was {
				t.Fatalf("%v became %v after %s", was, now, p)
			}
		}
	})
}

// A popup has no frame to find: its next button stays under the pointer.
func TestDatePickerKeepsItsSize(t *testing.T) {
	headless(t)
	each(t, func(t *testing.T, e core.Environment) {
		var next string
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			next = "label:" + locales.CalendarNext(cx)
			return DatePicker{DefaultValue: base.DateValue{Start: base.NewDate(2026, 2, 9, base.TimeOfDay{})}, TestID: "picker", Label: "Date"}
		})
		app.Press("id:picker")
		was := app.Find(next).Frame
		for _, p := range []string{next, next, next, "label:5"} {
			app.Press(p)
			if now := app.Find(next).Frame; now != was {
				t.Fatalf("%v became %v after %s", was, now, p)
			}
		}
	})
}

func TestDatePickerShowsWhatItPicked(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		app, _ := start(t, e, func(*core.Scope) core.Node {
			return DatePicker{DefaultValue: base.DateValue{Start: base.NewDate(2026, 10, 9, base.TimeOfDay{})}, Placeholder: "Pick", TestID: "date", Label: "Date"}
		})
		if !app.Exists("text:2026-10-09") {
			t.Fatal("the field does not show its date")
		}
		app.Press("id:date")
		app.Press("label:2026-10-15")
		if !app.Exists("text:2026-10-15") {
			t.Fatal("the field does not show the date picked")
		}
	})
}

func TestTimeFieldEdits(t *testing.T) {
	var got base.TimeOfDay
	each(t, func(t *testing.T, e core.Environment) {
		app, _ := start(t, e, func(*core.Scope) core.Node {
			return TimeField{Label: "Time", DefaultValue: base.TimeOfDay{Hour: 9, Minute: 30}, OnChange: func(v base.TimeOfDay) { got = v }}
		})
		app.Type("label:Time", "2")
		if got.Hour != 2 {
			t.Fatalf("got %+v", got)
		}
	})
}

// TestColorPickerFields edits the color in every notation, keeping the
// alpha through six digits and taking it from eight.
func TestColorPickerFields(t *testing.T) {
	var value core.Color
	ended := 0
	app, _ := start(t, envs()[0].Environment, func(*core.Scope) core.Node {
		return ColorPicker{DefaultValue: 0xff000080, OnChange: func(c core.Color, _ string) { value = c }, OnChangeEnd: func(core.Color, string) { ended++ }}
	})
	retype := func(label, text string) {
		t.Helper()
		app.Clear("label:" + label)
		app.Type("label:"+label, text)
	}
	if got := app.Find("label:Hex color").Text; got != "FF0000" {
		t.Fatalf("hex reads %q", got)
	}
	if got := app.Find("label:Red").Text; got != "255" {
		t.Fatalf("red reads %q", got)
	}
	retype("Hex color", "112233")
	if value != core.SetColor(0x11223380) {
		t.Fatalf("six digits gave %x, as they were typed with the alpha kept", value)
	}
	retype("Hex color", "44556640")
	if value != core.SetColor(0x44556640) {
		t.Fatalf("eight digits gave %x", value)
	}
	app.Focus("label:Green")
	before := ended
	retype("Green", "200")
	if value != core.SetColor(0x44c86640) {
		t.Fatalf("green gave %x as it was typed", value)
	}
	if ended != before {
		t.Fatal("typing ended the change")
	}
	app.Focus("label:Opacity percent")
	if got := app.Find("label:Hex color").Text; got != "44C866" {
		t.Fatalf("hex follows as %q", got)
	}
	if ended != before+1 {
		t.Fatalf("leaving a field ended the change %d times", ended-before)
	}
	retype("Opacity percent", "100%")
	if value != core.SetColor(0x44c866ff) {
		t.Fatalf("opacity gave %x", value)
	}
	app.Press("label:Color format")
	app.Press("text:HSBA")
	app.Settle()
	if app.Exists("label:Red") {
		t.Fatal("the format did not change the fields")
	}
	if got := app.Find("label:Hue degrees").Text; got != "135.5" {
		t.Fatalf("hue reads %q", got)
	}
	if got := app.Find("label:Saturation percent").Text; !strings.HasSuffix(got, "%") {
		t.Fatalf("saturation reads %q", got)
	}
	retype("Brightness percent", "0")
	app.Focus("label:Saturation percent")
	retype("Brightness percent", "78.4%")
	app.Focus("label:Saturation percent")
	if got := app.Find("label:Hue degrees").Text; got != "135.5" {
		t.Fatalf("black lost the hue: %q", got)
	}
	app.Press("label:Color format")
	app.Press("text:HSLA")
	app.Settle()
	app.Find("label:Lightness percent")
}

// TestColorPickerScrubs moves a channel by its letter, a change ending as
// the drag does.
func TestColorPickerScrubs(t *testing.T) {
	var value core.Color
	ended := 0
	app, _ := start(t, envs()[0].Environment, func(*core.Scope) core.Node {
		return ColorPicker{DefaultValue: 0x640000ff, OnChange: func(c core.Color, _ string) { value = c }, OnChangeEnd: func(core.Color, string) { ended++ }}
	})
	app.Pan(`text:"R"`, core.Point{X: 4}, core.Point{X: 10})
	if value != core.SetColor(0x6e0000ff) {
		t.Fatalf("a scrub of ten points gave %x", value)
	}
	if ended != 1 {
		t.Fatalf("a scrub ended the change %d times", ended)
	}
}

// TestColorPickerCopies writes the color in the menu's formats, and leaves
// alpha out with NoAlpha.
func TestColorPickerCopies(t *testing.T) {
	app, _ := start(t, envs()[0].Environment, func(*core.Scope) core.Node {
		return ColorPicker{DefaultValue: 0xff000080}
	})
	for item, want := range map[string]string{"RGBA": "rgb(255 0 0 / 0.5)", "HSBA": "hsb(0 100% 100% / 0.5)", "HSLA": "hsl(0 100% 50% / 0.5)", "HEX": "#ff000080"} {
		app.Press("label:Copy color")
		app.Press("role:menu >> text:" + item)
		app.Settle()
		if got := app.Clipboard; got != want {
			t.Fatalf("%s copied %q, want %q", item, got, want)
		}
	}
	plain, _ := start(t, envs()[0].Environment, func(*core.Scope) core.Node {
		return ColorPicker{DefaultValue: 0xff0000ff, NoAlpha: true}
	})
	if plain.Exists("label:Opacity percent") || plain.Exists("label:Opacity") || plain.Find("label:Color format") == nil {
		t.Fatal("NoAlpha kept the opacity")
	}
	plain.Find("text:RGB")
}

// TestColorPickerEyeDropper shows the eyedropper where there is a sampler,
// and takes its color.
func TestColorPickerEyeDropper(t *testing.T) {
	app, _ := start(t, envs()[0].Environment, func(*core.Scope) core.Node { return ColorPicker{DefaultValue: 0xff0000ff} })
	if app.Exists("label:Pick a color from the screen") {
		t.Fatal("an eyedropper shows with no sampler")
	}
	env := desktopEnv
	env.ScreenColor = true
	var value core.Color
	sampled, _ := start(t, env, func(*core.Scope) core.Node {
		return ColorPicker{DefaultValue: 0xff0000ff, OnChangeEnd: func(c core.Color, _ string) { value = c }}
	})
	sampled.PickScreenColor = func() (driver.Color, bool) { return core.SetColor(0x00ff00ff), true }
	sampled.Press("label:Pick a color from the screen")
	if value != core.SetColor(0x00ff00ff) {
		t.Fatalf("the eyedropper gave %x", value)
	}
}

// On a desktop a select opens below its field, and with an option chosen
// lays that row over the field.
func TestSelectLaysItsChoiceOverTheField(t *testing.T) {
	desktop := core.Environment{OS: core.OSMacOS, Idiom: core.IdiomDesktop, Locale: "en-US"}
	var fruit *core.Signal[string]
	app, _ := start(t, envs()[0].Environment, func(cx *core.Scope) core.Node {
		fruit = core.NewSignal(cx, "")
		return core.View{Style: core.Style{Padding: core.Edges{Top: 200}}, Children: []core.Node{
			Select[string]{TestID: "fruit", Label: "Fruit", Options: fruitOptions(), Value: fruit.Get, OnChange: fruit.Set},
		}}
	}, nsgotest.Environment(desktop))
	popupTop := func() driver.Points { return app.Find("role:listbox").Parent.Frame.Origin.Y }
	app.Press("id:fruit")
	below := popupTop()
	app.Key("Escape")
	app.Settle()
	fruit.Set("plum")
	app.Press("id:fruit")
	app.Settle()
	over := popupTop()
	if over >= below {
		t.Fatalf("the popup starts at %v with a choice and %v without", over, below)
	}
}
