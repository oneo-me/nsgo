package base

import (
	"math"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/oneo-me/nsgo/core"
)

// AvatarStatus is where an Avatar's image is.
type AvatarStatus uint8

const (
	// AvatarIdle has no image.
	AvatarIdle AvatarStatus = iota
	AvatarLoading
	AvatarLoaded
	AvatarError
)

// Avatar is a person's picture, or a fallback while it loads, when it
// fails and when there is none. Its parts are the fallback and the image
// over it, both as large as the avatar, then its children, what shows over
// the picture. The image is transparent until it has loaded, then the
// fallback goes, so loading lays nothing out again. It is one image for
// assistive technology, labelled with the person's name, and sizes,
// shapes and colors are the app's.
type Avatar struct {
	// Src is the picture; an empty URI is none.
	Src     core.ImageSource
	SrcFunc func() core.ImageSource
	// Name is what assistive technology says, unless Label does, and the
	// fallback's initials when Fallback is nil.
	Name string
	// Label is what assistive technology says, over Name.
	Label string
	// Decorative hides it from assistive technology, a picture beside the
	// name it shows, as HTML's alt="".
	Decorative bool
	// FallbackDelay is how long the fallback waits while the image loads,
	// so one that loads within it shows without the fallback flashing
	// first; math.Inf(1) is no fallback while it loads. In milliseconds,
	// as TSX's fallbackDelay.
	FallbackDelay float64
	// OnStatusChange hears the image's status change.
	OnStatusChange func(status AvatarStatus)
	Fallback       core.Node
	Class          string
	Style          core.Style
	StyleFunc      func() core.Style
	ImageClass     string
	ImageStyle     core.Style
	// ImageResizeMode is how the image fills the avatar.
	ImageResizeMode core.ResizeMode
	FallbackClass   string
	FallbackStyle   core.Style
	Children        []core.Node
	TestID          string
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
}

// Render implements core.Node.
func (a Avatar) Render(cx *core.Scope) core.Node {
	src := func() core.ImageSource {
		if a.SrcFunc != nil {
			return a.SrcFunc()
		}
		return a.Src
	}
	source := core.NewMemo(cx, func() string { return src().URI })
	statusOf := func(uri string) AvatarStatus {
		if uri == "" {
			return AvatarIdle
		}
		return AvatarLoading
	}
	status := core.NewSignal(cx, statusOf(source.GetUntracked()))
	reported := AvatarIdle
	set := func(next AvatarStatus) {
		status.Set(next)
		if reported != next {
			reported = next
			if a.OnStatusChange != nil {
				a.OnStatusChange(next)
			}
		}
	}
	cx.Effect(func() {
		next := statusOf(source.Get())
		cx.Untrack(func() { set(next) })
	})
	delay, forever := fallbackDelay(a.FallbackDelay)
	waits := func(s AvatarStatus) bool { return s == AvatarLoading && (forever || delay > 0) }
	waited := core.NewSignal(cx, !waits(status.GetUntracked()))
	var stop func() bool
	cx.Effect(func() {
		waiting := waits(status.Get())
		cx.Untrack(func() {
			if stop != nil {
				stop()
				stop = nil
			}
			waited.Set(!waiting)
			if waiting && !forever {
				stop = cx.AfterFunc(delay, func() { waited.Set(true) })
			}
		})
	})
	loaded := func() bool { return status.Get() == AvatarLoaded }
	label := a.Label
	if label == "" {
		label = a.Name
	}
	hidden := func() bool { return true }
	full := core.Style{Width: core.Pct(100), Height: core.Pct(100)}
	fallback := core.Show{When: func() bool { return !loaded() && waited.Get() }, Then: func(*core.Scope) core.Node {
		content := a.Fallback
		if content == nil {
			content = core.Text{Content: initials(a.Name)}
		}
		return core.View{Class: a.FallbackClass, Style: full.Merge(a.FallbackStyle), AccessibilityElementsHidden: hidden, Children: []core.Node{content}}
	}}
	inset := core.Style{Position: core.Absolute, Left: core.Pt(0), Top: core.Pt(0), Right: core.Pt(0), Bottom: core.Pt(0)}
	image := core.Dynamic(func(*core.Scope) core.Node {
		// An image of its own for each source, so a late event of the old
		// one is not heard as the new one's.
		uri := source.Get()
		if uri == "" {
			return nil
		}
		return core.Image{Class: a.ImageClass, Source: core.Untracked(cx, src), ResizeMode: a.ImageResizeMode, Style: inset.Merge(a.ImageStyle),
			StyleFunc: func() core.Style {
				if loaded() {
					return core.Style{Opacity: core.Alpha(1)}
				}
				return core.Style{Opacity: core.Alpha(0)}
			},
			ElementsHidden: true, OnLoadStart: func() { set(AvatarLoading) },
			OnLoad: func(core.Size) { set(AvatarLoaded) }, OnError: func(error) { set(AvatarError) }}
	})
	important := core.ImportantAuto
	if a.Decorative || label == "" {
		important = core.ImportantNoHideDescendants
	}
	return core.View{Class: a.Class, Style: a.Style, StyleFunc: a.StyleFunc, Role: core.RoleImage, Label: label, TestID: a.TestID, OnLayout: a.OnLayout, Tooltip: a.Tooltip,
		Accessible: !a.Decorative && label != "", ImportantForAccessibility: important,
		Children: append([]core.Node{fallback, image}, a.Children...)}
}

// fallbackDelay is FallbackDelay's milliseconds as a delay, and whether it
// never ends, past what a timer waits (2^31 - 1 ms, as the web's).
func fallbackDelay(ms float64) (time.Duration, bool) {
	switch {
	case math.IsNaN(ms) || ms <= 0:
		return 0, false
	case ms > math.MaxInt32:
		return 0, true
	}
	return time.Duration(ms * float64(time.Millisecond)), false
}

// initials are name's as gpui-component's avatar takes them: the first
// characters of its first two words, upper case; a single word gives its
// first two letters, or in a script without letter case its first
// character, which stands for a name there.
func initials(name string) string {
	words := strings.Fields(name)
	if len(words) == 0 {
		return ""
	}
	first := []rune(words[0])
	var picked []rune
	switch {
	case len(words) > 1:
		picked = []rune{first[0], []rune(words[1])[0]}
	case unicode.IsUpper(first[0]) || unicode.IsLower(first[0]):
		picked = first[:min(2, len(first))]
	default:
		picked = first[:1]
	}
	return strings.ToUpper(string(picked))
}

// AvatarGroup is a row of avatars that shows the first few and stands for
// the rest with Overflow, after them while more people are in it. Its
// overlap, rings and the overflow's shape are the app's, and assistive
// technology hears a group.
type AvatarGroup struct {
	// Limit is how many of its avatars show, 3 when 0 and none when
	// negative; the others make no views.
	Limit int
	// Total is how many people it stands for when the app passes only the
	// first; 0 is its children's number.
	Total int
	// Overflow stands for the people not shown; the app writes the count.
	Overflow core.Node
	// OverflowLabel is what assistive technology says of it; without it,
	// its content.
	OverflowLabel string
	OverflowClass string
	OverflowStyle core.Style
	Class         string
	Style         core.Style
	StyleFunc     func() core.Style
	Children      []core.Node
	Label         string
	TestID        string
	// OnLayout and Tooltip are its view's, as a core.View's.
	OnLayout func(core.Bounds)
	Tooltip  core.Node
}

// Render implements core.Node.
func (g AvatarGroup) Render(*core.Scope) core.Node {
	limit := g.Limit
	if limit == 0 {
		limit = 3
	}
	children := g.Children[:max(min(limit, len(g.Children)), 0)]
	total := g.Total
	if total <= 0 {
		total = len(g.Children)
	}
	children = slices.Clip(children)
	if total > len(children) && g.Overflow != nil {
		children = append(children, core.View{Accessible: g.OverflowLabel != "", Label: g.OverflowLabel,
			Class: g.OverflowClass, Style: g.OverflowStyle, Children: []core.Node{g.Overflow}})
	}
	return core.View{Class: g.Class, Style: g.Style, StyleFunc: g.StyleFunc, Role: core.RoleGroup, Label: g.Label, TestID: g.TestID, OnLayout: g.OnLayout, Tooltip: g.Tooltip,
		Children: children}
}
