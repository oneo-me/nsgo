package main

import (
	"fmt"
	"slices"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"

	"github.com/oneo-me/nsgo/examples/gallery/locales"
	"github.com/oneo-me/nsgo/ui"
)

// What shows things: each ui form under its base form, with their props to
// set.

// demoPeople are the avatars' names.
var demoPeople = []string{"Ada Lovelace", "Grace Hopper", "Alan Turing", "Katherine Johnson", "Linus Pauling", "Edsger Dijkstra"}

// badgeVariants are the badge's looks.
var badgeVariants = []ui.BadgeVariant{ui.BadgeNeutral, ui.BadgeAccent, ui.BadgeInfo, ui.BadgeWarn, ui.BadgeDanger, ui.BadgeUp, ui.BadgeDown, ui.BadgeOutline}

func badgeCard(*core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		return Playground{Title: "Badge", UIOnly: true, Stage: func() core.Node {
			var badges []core.Node
			for _, v := range badgeVariants {
				badges = append(badges, ui.Badge{Text: tr.LibNewVersion(), Variant: v})
			}
			return stageRow(badges...)
		}}
	}
}

func avatarCard(*core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		size := core.NewSignal(cx, 48.0)
		status := core.NewSignal(cx, ui.AvatarOnline)
		picture := core.NewSignal(cx, false)
		return Playground{Title: "Avatar", Base: baseAvatarForm(cx, picture), Stage: func() core.Node {
			a := ui.Avatar{Name: demoPeople[0], Size: core.Points(size.Get()), Status: status.Get()}
			if picture.Get() {
				a.Image = core.ImageSource{Data: logoPNG}
			}
			return a
		}, BaseKnobs: []core.Node{boolKnob("Src", picture)}, Knobs: []core.Node{
			numberKnob("Size", size, 24, 80, 4),
			choiceKnob("Status", status, options([]ui.AvatarStatus{ui.AvatarNoStatus, ui.AvatarOnline, ui.AvatarOffline}, "None", "Online", "Offline")...),
		}}
	}
}

func avatarGroupCard(*core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		limit, size := core.NewSignal(cx, 4.0), core.NewSignal(cx, 36.0)
		var members []ui.AvatarGroupMember
		for _, name := range demoPeople {
			members = append(members, ui.AvatarGroupMember{Name: name})
		}
		return Playground{Title: "AvatarGroup", Base: baseAvatarGroupForm(cx, limit), Stage: func() core.Node {
			return ui.AvatarGroup{Members: members, Max: int(limit.Get()), Size: core.Points(size.Get()), Label: tr.ControlsMembers(),
				MoreLabel: func(n int) string { return fmt.Sprintf("+%d", n) }}
		}, BaseKnobs: []core.Node{numberKnob("Limit", limit, 1, 6, 1)}, Knobs: []core.Node{numberKnob("Size", size, 24, 56, 4)}}
	}
}

// demoImage is the gallery's icon, as a picture to show.
func demoImage() core.ImageSource {
	return core.ImageSource{Data: logoPNG, Size: core.Size{Width: 512, Height: 512}}
}

func imagePlayground(*core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		mode := core.NewSignal(cx, core.Cover)
		ratio := core.NewSignal(cx, 1.5)
		return Playground{Title: "Image", UIOnly: true, Fill: true, Stage: func() core.Node {
			return ui.Image{Source: demoImage(), Label: tr.DemoImage(), ResizeMode: mode.Get(), AspectRatio: float32(ratio.Get())}
		}, Knobs: []core.Node{
			choiceKnob("ResizeMode", mode, options([]core.ResizeMode{core.Cover, core.Contain, core.Center}, "Cover", "Contain", "Center")...),
			numberKnob("AspectRatio", ratio, 1, 3, 0.25),
		}}
	}
}

// swatch is a slide or a side of a comparison: a tinted tile around text.
func swatch(cx *core.Scope, t Tone, text string) core.Node {
	return core.View{StyleFunc: func() core.Style {
		return core.Style{FlexGrow: 1, MinHeight: core.Pt(120), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, CornerRadius: ui.RadiusInset, Background: t.Tint(cx)}
	}, Children: []core.Node{ui.Text(text, 15, ui.Semibold, t.Shade(cx))}}
}

func imageCompareCard(*core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		position := core.NewSignal(cx, 50.0)
		vertical := core.NewSignal(cx, false)
		return Playground{Title: "ImageCompare", UIOnly: true, Fill: true, Stage: func() core.Node {
			return ui.ImageCompare{Before: swatch(cx, ToneInfo, tr.KitBefore()), After: swatch(cx, ToneAccent, tr.KitAfter()), Vertical: vertical.Get(),
				Position: func() float32 { return float32(position.Get()) }, OnChange: func(p float32) { position.Set(float64(p)) },
				BeforeLabel: tr.KitBefore(), AfterLabel: tr.KitAfter(), Label: tr.DemoImage(), AspectRatio: 2}
		}, Knobs: []core.Node{numberKnob("Position", position, 0, 100, 1), boolKnob("Vertical", vertical)}}
	}
}

func carouselCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		index := core.NewSignal(cx, 0)
		width, count := core.NewSignal(cx, 220.0), core.NewSignal(cx, 5.0)
		looping := core.NewSignal(cx, false)
		return Playground{Title: "Carousel", Fill: true, Base: baseCarouselForm(cx, last, count, looping), Stage: func() core.Node {
			return ui.Carousel{Count: int(count.Get()), SlideWidth: core.Points(width.Get()), Index: index.Get, Label: tr.DemoImage(),
				OnChange: func(i int) { index.Set(i); last.Set(fmt.Sprintf("Carousel: %d", i+1)) },
				Slide: func(cx *core.Scope, i int) core.Node {
					return swatch(cx, tileTones[i%len(tileTones)], itoa(i+1))
				},
				SlideLabel:    func(i, n int) string { return fmt.Sprintf("%d / %d", i+1, n) },
				PreviousLabel: tr.LibBack(), NextLabel: tr.LibNext()}
		}, BaseKnobs: []core.Node{numberKnob("Count", count, 2, 8, 1), boolKnob("Looping", looping)}, Knobs: []core.Node{numberKnob("SlideWidth", width, 120, 320, 20)}}
	}
}

func listCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		multiple := core.NewSignal(cx, false)
		return Playground{Title: "List", Fill: true, Base: baseListForm(cx, last), Stage: func() core.Node {
			many := multiple.Get()
			return core.Func(func(cx *core.Scope) core.Node {
				selection := ui.NewSelection(cx, many)
				return ui.List{Label: tr.ControlsMembers(), Selection: selection,
					Count: func() int { return len(demoPeople) }, Key: func(i int) any { return demoPeople[i] },
					RowLabel:   func(i int) string { return demoPeople[i] },
					OnActivate: func(i int) { last.Set("List: " + demoPeople[i]) },
					Row: func(cx *core.Scope, row core.ListRow) core.Node {
						return ui.HStack{Gap: 8, Children: []core.Node{ui.Avatar{Name: demoPeople[core.Untracked(cx, row.Index)], Size: 22},
							ui.Textf(func() string { return demoPeople[row.Index()] }, 13, ui.Regular, colors.Text)}}
					},
					Style: core.Style{Height: core.Pt(180)}}
			})
		}, Knobs: []core.Node{boolKnob("Multiple", multiple)}}
	}
}

// fileTree is the Tree card's project.
var fileTree = map[any][]any{"source": {"source/app", "source/lib"}, "source/app": {"main.go", "view.go"}, "source/lib": {"store.go"}, "docs": {"guide.md"}}

func treeCard(last *core.Signal[string]) cardFunc {
	return func(cx *core.Scope) core.Node {
		tr := locales.For(cx)
		colors := ui.ColorsOf(cx)
		open := core.NewSignalFunc(cx, []string{"source"}, slices.Equal)
		selection := ui.NewSelection(cx, false)
		return Playground{Title: "Tree", Fill: true, Base: baseTreeForm(cx, last), Stage: func() core.Node {
			return ui.Tree{Label: tr.LibFiles(), Selection: selection, ToggleLabel: tr.KitExpand(),
				Roots: func() []any { return []any{"source", "docs", "README.md"} }, Children: func(k any) []any { return fileTree[k] },
				Expanded: func(k any) bool { return slices.Contains(open.Get(), k.(string)) },
				OnExpand: func(k any, on bool) {
					next := slices.DeleteFunc(slices.Clone(open.GetUntracked()), func(s string) bool { return s == k })
					if on {
						next = append(next, k.(string))
					}
					open.Set(next)
				},
				OnActivate: func(k any) { last.Set(fmt.Sprintf("Tree: %v", k)) },
				RowLabel:   func(k any) string { return k.(string) },
				Row: func(cx *core.Scope, item ui.TreeItem) core.Node {
					return ui.HStack{Gap: 6, Children: []core.Node{
						ui.IconOf(func() *core.SVG {
							if item.Branch() {
								return lucide.Folder()
							}
							return lucide.File()
						}, 14, colors.Muted),
						ui.Textf(func() string { return fmt.Sprint(item.Key()) }, 13, ui.Regular, colors.Text)}}
				},
				Style: core.Style{Height: core.Pt(220)}}
		}}
	}
}
