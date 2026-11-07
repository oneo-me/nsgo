package main

import (
	"fmt"
	"github.com/oneo-me/nsgo/icons/lucide"
	"strconv"

	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"

	"github.com/oneo-me/nsgo/examples/gallery/locales"
	"github.com/oneo-me/nsgo/ui"
)

// Storage keys of the demo.
const (
	launchesKey = "launches"
	noteKey     = "note"
)

// storageDemo is Scope.Storage: a count of this demo's openings, a note
// saved as it is typed and every key the app keeps (the gallery's own last
// demo among them), all still there after a restart. Reads are synchronous
// and subscribe to changes.
func storageDemo(cx *core.Scope) core.Node {
	tr := locales.For(cx)
	store := cx.Storage()
	saved := core.NewSignal(cx, "")
	launches := core.Untracked(cx, func() int {
		v, _ := store.GetItem(launchesKey)
		n, _ := strconv.Atoi(v)
		return n + 1
	})
	store.SetItem(launchesKey, strconv.Itoa(launches))
	shown := core.NewSignal(cx, launches)
	note := func() string { v, _ := store.GetItem(noteKey); return v }

	write := func(text string) {
		var err error
		if text != "" {
			err = store.SetItem(noteKey, text)
		} else {
			store.RemoveItem(noteKey)
		}
		if err != nil {
			saved.Set("failed")
		} else {
			saved.Set("saved")
		}
	}

	clearAll := func() {
		base.ShowAlertDialog(cx, base.AlertDialog{
			Title:       tr.StorageClearTheStorage(),
			Description: tr.StorageRemovesEveryKeyTheGallery(),
			Actions: []core.AlertAction{
				{Label: tr.StorageCancel(), Role: core.AlertCancel},
				{Label: tr.StorageClear(), Role: core.AlertDestructive},
			},
			CloseOnEscape: core.ToggleOn, CloseOnBackdropPress: core.ToggleOn,
		}, func(label string) {
			if label != tr.StorageClear() {
				return
			}
			store.Clear()
			shown.Set(0)
			saved.Set("")
		})
	}

	launchesCard := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		return Card{Title: tr.StorageLaunches(), Description: tr.StorageCountedEachTimeThisDemo(), Actions: []core.Node{ui.Badge{Text: tr.StorageKept(), Variant: ui.BadgeUp}}, Children: []core.Node{
			Figure{Value: func() string { return strconv.Itoa(shown.Get()) }, Note: func() string {
				if shown.Get() == 1 {
					return "opening"
				}
				return "openings"
			}},
			core.Text{ContentFunc: func() string { return fmt.Sprintf("SetItem(%q, %q)", launchesKey, strconv.Itoa(shown.Get())) },
				Style: core.Style{FontSize: 12, FontFamily: ui.Mono}, StyleFunc: ui.Colored(colors.Muted)},
		}}
	}

	noteCard := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		return Card{Title: tr.StorageNote(), Description: tr.StorageSavedAsYouTypeBack(), Children: []core.Node{
			ui.Field{Input: base.Input{Multiline: true, Value: note, OnChange: write, Placeholder: tr.StorageWriteSomethingThenComeBack(),
				Label: tr.StorageNote(), TestID: "note", Style: core.Style{Height: core.Pt(120), Padding: core.Edges{Top: 12, Right: 14, Bottom: 12, Left: 14}}}},
			core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 6, MinHeight: core.Pt(16)}, Children: []core.Node{
				core.Show{When: func() bool { return saved.Get() == "saved" }, Then: func(*core.Scope) core.Node { return ui.Icon(lucide.Check, 14, colors.Up) }},
				core.Text{ContentFunc: func() string {
					return map[string]string{"": "", "saved": tr.StorageSaved(), "failed": tr.StorageNotSaved()}[saved.Get()]
				}, StyleFunc: func() core.Style {
					c := colors.Faint()
					switch saved.Get() {
					case "failed":
						c = colors.Danger()
					case "saved":
						c = colors.Up()
					}
					return core.Style{FlexGrow: 1, FontSize: 12, Color: c}
				}},
				core.Text{ContentFunc: func() string { return tr.StorageChars(len([]rune(note()))) }, Style: core.Style{FontSize: 12}, StyleFunc: ui.Colored(colors.Faint)},
			}},
		}}
	}

	type entry struct {
		key    string
		length int
	}
	entries := func() []entry {
		var out []entry
		for _, k := range store.Keys() {
			v, _ := store.GetItem(k)
			out = append(out, entry{k, len(v)})
		}
		return out
	}
	keysCard := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		total := func() int {
			n := 0
			for _, e := range entries() {
				n += len(e.key) + e.length
			}
			return n
		}
		return Card{Title: tr.StorageStoredKeys(), Description: tr.StorageKeysAndEachValueS(), Children: []core.Node{
			core.Show{When: func() bool { return len(store.Keys()) > 0 }, Else: func(*core.Scope) core.Node { return Caption{Label: tr.StorageNothingStored()} },
				Then: func(*core.Scope) core.Node {
					return core.View{Children: []core.Node{core.Each(entries, func(e entry) string { return e.key },
						func(cx *core.Scope, e func() entry, i func() int) core.Node {
							colors := ui.ColorsOf(cx)
							return core.Dynamic(func(*core.Scope) core.Node {
								return Row{First: i() == 0, Children: []core.Node{
									core.Text{ContentFunc: func() string { return e().key }, MaxLines: 1, Style: core.Style{FlexGrow: 1, FlexShrink: 1, FontSize: 13, FontFamily: ui.Mono}, StyleFunc: ui.Colored(colors.Text)},
									core.Text{ContentFunc: func() string { return tr.StorageChars(e().length) }, Style: core.Style{FontSize: 13}, StyleFunc: ui.Colored(colors.Muted)},
								}}
							})
						})}}
				}},
			core.Text{ContentFunc: func() string { return tr.StorageKeysChars(len(store.Keys()), total()) },
				Style: core.Style{FontSize: 13}, StyleFunc: ui.Colored(colors.Faint)},
			ui.Button{Text: tr.StorageClearAll(), Variant: ui.ButtonSecondary, Disabled: func() bool { return len(store.Keys()) == 0 }, OnPress: clearAll},
			ui.Text(tr.StorageTheDataStaysOnThis(), 12, ui.Regular, colors.Faint),
		}}
	}

	dirsCard := func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		dir := func(d string) string {
			if d == "" {
				return tr.StorageNoneOnThisTarget()
			}
			return d
		}
		return Card{Title: tr.StorageAppDirectories(), Description: tr.StorageDatadirAndCachedirNamedBy(), Children: []core.Node{
			Caption{Label: tr.StorageData()}, Code{Text: dir(cx.DataDir())},
			Caption{Label: tr.StorageCache()}, Code{Text: dir(cx.CacheDir())},
		}}
	}

	return demoPage(cx, pageProps{}, launchesCard, noteCard, keysCard, dirsCard)
}
