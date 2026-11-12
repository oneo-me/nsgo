package core

import (
	"errors"
	"slices"
	"strconv"
	"strings"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

type imageNode struct {
	config driver.ImageConfig
	sent   driver.ImageConfig // as resolved for the pixel ratio
	// refused and refusedPlaceholder are the URIs Access refused last.
	refused, refusedPlaceholder string
}

func (t *Tree) mountImage(cx *Scope, parent *node, v Image) {
	n := t.newNode(driver.KindImage, parent, cx)
	t.setClasses(n, v.Class)
	t.bindStyle(cx, n, v.Style, v.StyleFunc)
	t.setRef(n, v.Ref)
	n.h = handlers{press: v.OnPress, layout: v.OnLayout, loadStart: v.OnLoadStart, loadEnd: v.OnLoadEnd,
		progress: v.OnProgress, load: v.OnLoad, loadError: v.OnError}
	n.props = driver.ViewProps{Role: RoleImage, Label: v.Label, TestID: v.TestID, ElementsHidden: v.ElementsHidden}
	n.image = &imageNode{config: driver.ImageConfig{Source: v.Source, Placeholder: v.Placeholder,
		ResizeMode: v.ResizeMode, BlurRadius: v.BlurRadius, FadeDuration: v.FadeDuration}}
	if v.SourceFunc != nil {
		cx.renderEffect(func() {
			src := v.SourceFunc()
			if !src.Same(n.image.config.Source) {
				if src.Size != n.image.config.Source.Size {
					t.markLayout(n)
				}
				n.image.config.Source = src
				t.mark(n)
			}
		})
	}
}

// naturalSize is what an image measures without a style size.
func (im *imageNode) naturalSize() Size { return im.config.Source.Size }

// Positions along an axis.
var (
	PositionStart  = PositionAxis{Offset: Pct(0)}
	PositionCenter = PositionAxis{Offset: Pct(50)}
	PositionEnd    = PositionAxis{Offset: Pct(100)}
)

// ImageSize calls done with the size in pixels of the image src names,
// as Image's OnLoad reports it, or the error loading or reading it (NSH's
// Image.getSize). The platform loads it as an Image would, the variant of
// an "asset:" source with Scales included, and keeps its bytes for the
// views that show it next.
func (cx *Scope) ImageSize(src ImageSource, done func(pixels Size, err error)) {
	t := cx.tree()
	if t == nil {
		return
	}
	if len(src.Data) == 0 {
		if err := sourceRefusal(cx.current(), AccessNetwork, src.URI); err != nil {
			done(Size{}, err)
			return
		}
	}
	t.alertID++
	if t.imageInfos == nil {
		t.imageInfos = map[uint32]imageInfoWait{}
	}
	t.imageInfos[t.alertID] = imageInfoWait{cx.current(), done}
	t.commands = append(t.commands, driver.MountItem{Kind: driver.ItemImageInfo, Count: t.alertID,
		Image: &driver.ImageConfig{Source: t.pickScale(src)}})
	t.changed = true
}

// PrefetchImage loads the image src names ahead of showing it, keeping
// its bytes for the views that show it next (NSH's Image.prefetch), and
// calls done, when not nil, with whether it loaded.
func (cx *Scope) PrefetchImage(src ImageSource, done func(ok bool)) {
	cx.ImageSize(src, func(_ Size, err error) {
		if done != nil {
			done(err == nil)
		}
	})
}

type imageInfoWait struct {
	owner *Scope
	done  func(Size, error)
}

func (t *Tree) answerImageInfo(ev driver.Event) {
	w, ok := t.imageInfos[ev.Count]
	if !ok {
		return
	}
	delete(t.imageInfos, ev.Count)
	owner := w.owner
	if owner.disposed {
		owner = t.root
	}
	var err error
	if ev.Text != "" {
		err = errors.New(ev.Text)
	}
	if w.done != nil {
		t.within(owner, func() { w.done(ev.Size, err) })
	}
}

// ScalesOf returns the set of scales, leaving out any that is not one of
// ImageScaleValues.
func ScalesOf(scales ...float32) ImageScales {
	var set ImageScales
	for _, s := range scales {
		if i := slices.Index(driver.ImageScaleValues[:], s); i >= 0 {
			set |= 1 << i
		}
	}
	return set
}

// pickScale returns s as the platform loads it at the screen's pixel
// ratio: an "asset:" image with scales names the variant they pick
// (asset:/a.png at 2 is asset:/a@2x.png); without scales, for the 1x pick
// or with a query or fragment, which no file name has, the URI stays.
func (t *Tree) pickScale(s ImageSource) ImageSource {
	scales := s.Scales
	s.Scales = 0
	if scales == 0 || s.Variants == nil && (!strings.HasPrefix(s.URI, "asset:") || strings.ContainsAny(s.URI, "?#")) {
		return s
	}
	pick := scales.Pick(t.root.Env().ratio.GetUntracked())
	if pick == 1 {
		return s
	}
	if uri, ok := s.Variants[pick]; ok {
		s.URI, s.Variants = uri, nil
		return s
	}
	name := strings.LastIndexByte(s.URI, '/') + 1
	if name == 0 {
		name = len("asset:")
	}
	dot := strings.LastIndexByte(s.URI[name:], '.')
	if dot > 0 {
		dot += name
	} else {
		dot = len(s.URI)
	}
	s.URI = s.URI[:dot] + "@" + strconv.FormatFloat(float64(pick), 'g', -1, 32) + "x" + s.URI[dot:]
	return s
}

// resolveMode is m, or def for the zero value.
func resolveMode(m, def ResizeMode) ResizeMode {
	if m == 0 {
		return def
	}
	return m
}
