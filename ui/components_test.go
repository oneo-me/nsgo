package ui

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/nsgotest"
)

// TestMain hands the main thread to the platform under nsgo test -target.
func TestMain(m *testing.M) { nsgotest.Main(m) }

var (
	desktopEnv = core.Environment{OS: core.OSMacOS, Idiom: core.IdiomDesktop, Locale: "en-US"}
	phoneEnv   = core.Environment{OS: core.OSIOS, Idiom: core.IdiomPhone, Locale: "en-US"}
)

// env is one of the four presentations every component is tested in.
type env struct {
	name string
	core.Environment
}

func envs() []env {
	darkDesktop, darkPhone := desktopEnv, phoneEnv
	darkDesktop.Dark, darkPhone.Dark = true, true
	return []env{{"desktop", desktopEnv}, {"desktop-dark", darkDesktop}, {"phone", phoneEnv}, {"phone-dark", darkPhone}}
}

// size is a window for e: a desktop's or a phone's.
func size(e core.Environment) nsgotest.Option {
	if e.Idiom != core.IdiomDesktop {
		return nsgotest.Size(390, 844)
	}
	return nsgotest.Size(1000, 760)
}

// start runs content under ui's theme for e, as an app sets it,
// with opts, and returns the colors it shows.
func start(t *testing.T, e core.Environment, content func(cx *core.Scope) core.Node, opts ...nsgotest.Option) (*nsgotest.App, Palette) {
	t.Helper()
	var shown Palette
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		cx.SetTheme(DefaultTheme())
		colors := ColorsOf(cx)
		shown = core.Untracked(cx, colors.Now)
		return core.View{Style: core.Style{FlexGrow: 1, Padding: core.All(16), Gap: 12}, StyleFunc: Filled(colors.Background),
			Children: []core.Node{core.Func(content)}}
	}, append([]nsgotest.Option{size(e), nsgotest.Environment(e)}, opts...)...)
	return app, shown
}

// each runs fn in every presentation.
func each(t *testing.T, fn func(t *testing.T, e core.Environment)) {
	for _, e := range envs() {
		t.Run(e.name, func(t *testing.T) { fn(t, e.Environment) })
	}
}

// headless skips a test of the headless model on a target.
func headless(t *testing.T) {
	if target := nsgotest.Target(); target != "" {
		t.Skipf("checks the headless model, not %s", target)
	}
}

// views are the views from v down, in order.
func views(v *nsgotest.View) []*nsgotest.View {
	out := []*nsgotest.View{v}
	for _, c := range v.Children {
		out = append(out, views(c)...)
	}
	return out
}

// looks are the styles each view from v down shows, in order, by view.
func looks(v *nsgotest.View) (ids []uint64, out []string) {
	for _, v := range views(v) {
		// JSON follows the pointers in the styles.
		look, err := json.Marshal(struct{ Style, Text any }{v.Style, v.TextStyle})
		if err != nil {
			panic(err)
		}
		ids = append(ids, uint64(v.ID))
		out = append(out, fmt.Sprintf("%d %q %s", v.Kind, v.Props.TestID, look))
	}
	return ids, out
}

// rootOf is the root view above v.
func rootOf(v *nsgotest.View) *nsgotest.View {
	for v.Parent != nil {
		v = v.Parent
	}
	return v
}
