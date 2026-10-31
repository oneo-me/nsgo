//go:build ios

package ios

import (
	"math"
	"slices"
	"structs"

	"github.com/ebitengine/purego/objc"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/platform/apple"
)

// Screens, sheets and the status bar, after NSH's navigation.rs. The
// stack stays the core's: a screen with a system header gets a
// UINavigationBar of its own over its top, attached to the status bar
// above it, holding a back item for the screen below, so UIKit draws the
// back button and its title. The bar's back button and a swipe from the
// leading edge report EventBack with the screen's ID, and the screens
// slide as UIKit's push and pop do, the bar with its screen. Sheets are
// presented view controllers holding the modal's view; they report their
// size, and their swipes as close requests.

// navBar is a screen's system header.
type navBar struct {
	bar, item, back objc.ID // UINavigationBar and its items
	config          driver.ScreenConfig
	edge            objc.ID // the swipe back's UIScreenEdgePanGestureRecognizer
	slide           *slide  // the swipe back's, while it lasts
	itemsSet        bool
	// collapsed: the large title went into the bar as the content
	// scrolled under it.
	collapsed bool
	// fit is the bar's height as UIKit sizes it, reported the one last
	// told to the engine.
	fit, reported driver.Points
	// system is the UIKit bar button of the header's RightSystemItem,
	// systemKind, made once per kind: a new one at every change would
	// flash.
	system     objc.ID
	systemKind driver.HeaderSystemItem
}

// sheet is a modal presented as a UIKit sheet.
type sheet struct {
	controller objc.ID
	gone       bool // the user swiped it away
	size       driver.Size
	insets     driver.Edges
}

// affineTransform is CGAffineTransform.
type affineTransform struct {
	_                structs.HostLayout
	A, B, C, D, X, Y float64
}

var identity = affineTransform{A: 1, D: 1}

var (
	classNavDelegate, classSheet objc.Class
	navDelegate                  objc.ID // one delegate for every bar
	bars                         = map[objc.ID]*view{}
	sheets                       = map[objc.ID]*view{}
	selEdgeSwipe                 = sel("nsgoEdgeSwipe:")
	selHeaderItem                = sel("nsgoHeaderItem:")
)

// UIInterfaceOrientationMask of each core orientation.
var orientationMasks = map[driver.Orientation]uint{
	driver.OrientationPortrait: 1 << 1, driver.OrientationPortraitUpsideDown: 1 << 2,
	driver.OrientationLandscapeLeft: 1 << 4, driver.OrientationLandscapeRight: 1 << 3,
}

func registerNav() {
	classNavDelegate = register("NsgoNavDelegate", objc.GetClass("NSObject"),
		[]*objc.Protocol{objc.GetProtocol("UINavigationBarDelegate")}, []objc.MethodDef{
			// The bar's background reaches up under the status bar where
			// the screen does.
			{Cmd: sel("positionForBar:"), Fn: func(_ objc.ID, _ objc.SEL, bar objc.ID) int {
				const top, topAttached = 2, 3
				if v := bars[bar]; v != nil && v.surface.barTop(v) > 0 {
					return topAttached
				}
				return top
			}},
			{Cmd: sel("navigationBar:shouldPopItem:"), Fn: func(_ objc.ID, _ objc.SEL, bar, _ objc.ID) bool {
				if v := bars[bar]; v != nil {
					v.surface.engine.Dispatch(driver.Event{Kind: driver.EventBack, ID: v.id})
				}
				// The core pops the screen, and the bar goes with it.
				return false
			}},
		})
	navDelegate = objc.ID(classNavDelegate).Send(selNew)
	classSheet = register("NsgoSheetController", objc.GetClass("UIViewController"),
		[]*objc.Protocol{objc.GetProtocol("UIAdaptivePresentationControllerDelegate")}, []objc.MethodDef{
			{Cmd: sel("viewDidLayoutSubviews"), Fn: func(self objc.ID, cmd objc.SEL) {
				apple.Super(self, cmd)
				if v := sheets[self]; v != nil {
					v.surface.sheetLaidOut(v)
				}
			}},
			{Cmd: sel("supportedInterfaceOrientations"), Fn: func(self objc.ID, cmd objc.SEL) uint {
				if v := sheets[self]; v != nil {
					if m := v.surface.orientations(); m != 0 {
						return m
					}
				}
				return apple.SendSuper[uint](self, cmd)
			}},
			{Cmd: sel("preferredStatusBarStyle"), Fn: func(self objc.ID, _ objc.SEL) int {
				if v := sheets[self]; v != nil {
					return v.surface.statusBar
				}
				return 0
			}},
			{Cmd: sel("prefersStatusBarHidden"), Fn: func(self objc.ID, _ objc.SEL) bool {
				v := sheets[self]
				return v != nil && v.surface.hideBar
			}},
			{Cmd: sel("preferredStatusBarUpdateAnimation"), Fn: func(self objc.ID, _ objc.SEL) int {
				if v := sheets[self]; v != nil {
					return v.surface.barAnimation()
				}
				return 0
			}},
			{Cmd: sel("presentationControllerDidDismiss:"), Fn: func(self objc.ID, _ objc.SEL, _ objc.ID) {
				if v := sheets[self]; v != nil && v.sheet != nil {
					v.sheet.gone = true
					s := v.surface
					s.engine.Dispatch(driver.Event{Kind: driver.EventRequestClose, ID: v.id, Text: "swipe", Finished: true})
					s.updateOrientations()
				}
			}},
			{Cmd: sel("presentationControllerDidAttemptToDismiss:"), Fn: func(self objc.ID, _ objc.SEL, _ objc.ID) {
				if v := sheets[self]; v != nil {
					v.surface.engine.Dispatch(driver.Event{Kind: driver.EventRequestClose, ID: v.id, Text: "swipe"})
				}
			}},
		})
}

// navMethods are the views' methods for a screen's swipe back.
func navMethods() []objc.MethodDef {
	return []objc.MethodDef{{Cmd: selEdgeSwipe, Fn: edgeSwiped}, {Cmd: selHeaderItem, Fn: headerItemPressed}}
}

// headerHeights measures a navigation bar on this device, as NSH does:
// its height, and with a large title the 52-point row large titles add.
func headerHeights() [2]driver.Points {
	bar := objc.ID(objc.GetClass("UINavigationBar")).Send(selNew)
	h := objc.Send[apple.CGSize](bar, sel("sizeThatFits:"), apple.CGSize{Width: 390}).Height
	bar.Send(selRelease)
	if h <= 0 {
		h = 44
	}
	return [2]driver.Points{driver.Points(h), driver.Points(h + 52)}
}

// configureScreen applies screen v's header.
func (s *surface) configureScreen(v *view, c *driver.ScreenConfig) {
	b := v.bar
	if b == nil {
		b = &navBar{}
		b.bar = objc.ID(objc.GetClass("UINavigationBar")).Send(selNew)
		b.bar.Send(selSetDeleg, navDelegate)
		const flexibleWidth = 2
		b.bar.Send(sel("setAutoresizingMask:"), flexibleWidth)
		if objc.Send[bool](b.bar, sel("respondsToSelector:"), sel("setPreferredBehavioralStyle:")) {
			const pad = 2
			b.bar.Send(sel("setPreferredBehavioralStyle:"), pad)
		}
		b.item = objc.ID(objc.GetClass("UINavigationItem")).Send(selNew)
		b.back = objc.ID(objc.GetClass("UINavigationItem")).Send(selNew)
		v.native.Send(selAddSubview, b.bar)
		bars[b.bar] = v
		v.bar = b
	}
	old := b.config
	b.config = *c
	b.bar.Send(selSetHidden, c.HeaderHidden)
	b.item.Send(sel("setTitle:"), apple.NSString(c.Title))
	b.back.Send(sel("setTitle:"), apple.NSString(c.BackTitle))
	b.item.Send(sel("setHidesBackButton:"), c.BackHidden)
	b.item.Send(sel("setLeftItemsSupplementBackButton:"), true)
	if !c.LargeTitle {
		b.collapsed = false
	}
	s.showLargeTitle(b)
	b.item.Send(sel("setLeftBarButtonItem:"), s.barItem(c.Left))
	b.item.Send(sel("setRightBarButtonItem:"), s.rightItem(v, c))
	b.item.Send(sel("setTitleView:"), s.headerWrap(c.TitleView))
	if !b.itemsSet || old.CanGoBack != c.CanGoBack {
		b.itemsSet = true
		items := []objc.ID{b.item}
		if c.CanGoBack {
			items = []objc.ID{b.back, b.item}
		}
		array := objc.ID(objc.GetClass("NSMutableArray")).Send(selNew)
		for _, it := range items {
			array.Send(sel("addObject:"), it)
		}
		b.bar.Send(sel("setItems:animated:"), array, false)
		array.Send(selRelease)
	}
	s.styleBar(b, c)
	s.layoutBar(v)
	v.native.Send(sel("bringSubviewToFront:"), b.bar)
	s.setSwipeBack(v)
}

// styleBar sets the bar's appearance from the header's colors, UIKit's
// own where they are unset, as NSH's header_appearance.
func (s *surface) styleBar(b *navBar, c *driver.ScreenConfig) {
	tint := objc.ID(0)
	if c.TintColor != 0 {
		tint = uiColor(c.TintColor)
	}
	b.bar.Send(sel("setTintColor:"), tint)
	a := objc.ID(objc.GetClass("UINavigationBarAppearance")).Send(selNew)
	switch {
	case c.HeaderTransparent:
		a.Send(sel("configureWithTransparentBackground"))
	case c.Background != 0:
		a.Send(sel("configureWithOpaqueBackground"))
		a.Send(sel("setBackgroundColor:"), uiColor(c.Background))
	default:
		a.Send(sel("configureWithDefaultBackground"))
	}
	if c.TitleColor != 0 {
		key := apple.Global(uikitPath, "NSForegroundColorAttributeName")
		attrs := objc.ID(objc.GetClass("NSDictionary")).Send(sel("dictionaryWithObject:forKey:"), uiColor(c.TitleColor), key)
		a.Send(sel("setTitleTextAttributes:"), attrs)
		a.Send(sel("setLargeTitleTextAttributes:"), attrs)
	}
	if c.TintColor != 0 {
		// The back button's title takes the tint only through the
		// appearance's button styles.
		key := apple.Global(uikitPath, "NSForegroundColorAttributeName")
		attrs := objc.ID(objc.GetClass("NSDictionary")).Send(sel("dictionaryWithObject:forKey:"), tint, key)
		for _, part := range []string{"buttonAppearance", "backButtonAppearance"} {
			a.Send(sel(part)).Send(sel("normal")).Send(sel("setTitleTextAttributes:"), attrs)
		}
	}
	b.bar.Send(sel("setStandardAppearance:"), a)
	b.bar.Send(sel("setScrollEdgeAppearance:"), a)
	b.bar.Send(sel("setCompactAppearance:"), a)
	a.Send(selRelease)
}

// showLargeTitle sets whether bar b shows its large title.
func (s *surface) showLargeTitle(b *navBar) {
	const always, never = 1, 2
	large := b.config.LargeTitle && !b.collapsed
	mode := never
	if large {
		mode = always
	}
	b.item.Send(sel("setLargeTitleDisplayMode:"), mode)
	b.bar.Send(sel("setPrefersLargeTitles:"), large)
}

// layoutBar places screen v's bar at the bottom of its header area: the
// part above it is the status bar, which the bar's background reaches
// under. It tells the engine the bar's height as UIKit sizes it, with
// its items and title view.
func (s *surface) layoutBar(v *view) {
	b := v.bar
	w := objc.Send[apple.CGRect](v.native, selBounds).Size.Width
	if !b.config.LargeTitle {
		if fit := objc.Send[apple.CGSize](b.bar, sel("sizeThatFits:"), apple.CGSize{Width: w}); fit.Height > 0 {
			b.fit = driver.Points(fit.Height)
		}
	}
	top := s.barTop(v)
	b.bar.Send(selSetFrame, apple.Rect(0, float64(top), w, float64(s.barHeight(v))))
	if bar := s.expandedBar(v); !b.config.HeaderHidden && bar != b.reported {
		b.reported = bar
		s.engine.Dispatch(driver.Event{Kind: driver.EventScreenHeader, ID: v.id, Value: float32(bar)})
	}
}

// expandedBar is screen v's bar's height with its large title shown.
func (s *surface) expandedBar(v *view) driver.Points {
	switch {
	case v.bar.config.LargeTitle:
		return s.headers[1]
	case v.bar.fit > 0:
		return v.bar.fit
	}
	return s.headers[0]
}

// barHeight is screen v's bar's height now, barTop how much of the
// status bar is above it.
func (s *surface) barHeight(v *view) driver.Points {
	if v.bar.collapsed {
		return s.headers[0]
	}
	return s.expandedBar(v)
}

func (s *surface) barTop(v *view) driver.Points {
	return max(v.bar.config.HeaderHeight-s.expandedBar(v), 0)
}

// followLargeTitle collapses the large title of the screen around scroll
// view sv into its bar once the content scrolled half the title's row
// under the header, and brings it back nearer the top, as UIKit's
// navigation controller does.
func (s *surface) followLargeTitle(sv *view, offset driver.Points) {
	if sv.sc == nil || sv.sc.config.HeaderInset <= 0 {
		return
	}
	var screen *view
	for n := sv.native.Send(selSuperview); n != 0; n = n.Send(selSuperview) {
		if v := views[n]; v != nil && v.bar != nil {
			screen = v
			break
		}
	}
	if screen == nil || !screen.bar.config.LargeTitle {
		return
	}
	scrolled := offset + sv.sc.config.Behavior.ContentInset.Top
	collapsed := scrolled > (s.headers[1]-s.headers[0])/2
	b := screen.bar
	if collapsed == b.collapsed {
		return
	}
	b.collapsed = collapsed
	animations := objc.NewBlock(func(objc.Block) {
		s.showLargeTitle(b)
		s.layoutBar(screen)
		b.bar.Send(sel("layoutIfNeeded"))
	})
	objc.ID(objc.GetClass("UIView")).Send(sel("animateWithDuration:animations:"), 0.25, animations)
	animations.Release()
}

// rightItem returns the bar button at the end of screen v's header:
// UIKit's own for c's RightSystemItem, else c's Right view.
func (s *surface) rightItem(v *view, c *driver.ScreenConfig) objc.ID {
	b := v.bar
	code, ok := systemItems[c.RightSystemItem]
	if !ok || c.Right == 0 {
		if b.system != 0 {
			b.system.Send(selRelease)
			b.system, b.systemKind = 0, 0
		}
		return s.barItem(c.Right)
	}
	if b.systemKind != c.RightSystemItem {
		if b.system != 0 {
			b.system.Send(selRelease)
		}
		// The screen's view is the target, which UIKit holds weakly; it
		// outlives the button.
		b.system = objc.ID(objc.GetClass("UIBarButtonItem")).Send(selAlloc).
			Send(sel("initWithBarButtonSystemItem:target:action:"), code, v.native, selHeaderItem)
		b.systemKind = c.RightSystemItem
	}
	return b.system
}

// systemItems are UIKit's UIBarButtonSystemItem of each header item.
var systemItems = map[driver.HeaderSystemItem]int{
	driver.HeaderItemDone: 0, driver.HeaderItemCancel: 1, driver.HeaderItemEdit: 2, driver.HeaderItemSave: 3,
	driver.HeaderItemAdd: 4, driver.HeaderItemCompose: 7, driver.HeaderItemShare: 9, driver.HeaderItemSearch: 12,
	driver.HeaderItemTrash: 16, driver.HeaderItemClose: 24,
}

// headerItemPressed presses the first view of the header's Right that
// takes presses, as the system bar button stands for it.
func headerItemPressed(self objc.ID, _ objc.SEL, _ objc.ID) {
	v := views[self]
	if v == nil || v.bar == nil {
		return
	}
	s, id := v.surface, v.bar.config.Right
	if int(id) >= len(s.views) || s.views[id] == nil {
		return
	}
	if p := firstPressable(s.views[id].native); p != nil {
		p.report(driver.Event{Kind: driver.EventPress, Index: -1})
	}
}

// firstPressable returns the first view under native, depth first, that
// takes presses.
func firstPressable(native objc.ID) *view {
	subs := native.Send(sel("subviews"))
	for i, n := 0, objc.Send[int](subs, sel("count")); i < n; i++ {
		sub := subs.Send(sel("objectAtIndex:"), i)
		if v := views[sub]; v != nil && v.events.Has(driver.EventPress) {
			return v
		}
		if v := firstPressable(sub); v != nil {
			return v
		}
	}
	return nil
}

// barItem returns a bar button item showing header view id, or nil.
func (s *surface) barItem(id driver.NodeID) objc.ID {
	wrap := s.headerWrap(id)
	if wrap == 0 {
		return 0
	}
	item := objc.ID(objc.GetClass("UIBarButtonItem")).Send(selAlloc).Send(sel("initWithCustomView:"), wrap)
	item.Send(sel("autorelease"))
	return item
}

// headerWrap returns the view the bar holds for header view id: a plain
// view at its size, which UIKit may move while the core keeps the header
// view at its origin.
func (s *surface) headerWrap(id driver.NodeID) objc.ID {
	if int(id) >= len(s.views) || s.views[id] == nil {
		return 0
	}
	v := s.views[id]
	if v.headerWrap == 0 {
		v.headerWrap = objc.ID(objc.GetClass("UIView")).Send(selNew)
		v.headerWrap.Send(selAddSubview, v.native)
	}
	s.sizeHeaderWrap(v)
	return v.headerWrap
}

func (s *surface) sizeHeaderWrap(v *view) {
	w := v.headerWrap
	f := objc.Send[apple.CGRect](w, sel("frame"))
	size := v.frame.Size
	w.Send(selSetFrame, apple.Rect(f.Origin.X, f.Origin.Y, float64(size.Width), float64(size.Height)))
	w.Send(sel("invalidateIntrinsicContentSize"))
	if up := w.Send(selSuperview); up != 0 {
		up.Send(selSetNeedsLayout)
	}
}

// setSwipeBack adds or removes screen v's swipe back.
func (s *surface) setSwipeBack(v *view) {
	c := &v.bar.config
	want := c.CanGoBack && !c.NoPopOnSwipe && !c.NoPopOnBack
	b := v.bar
	switch {
	case want && b.edge == 0:
		b.edge = objc.ID(objc.GetClass("UIScreenEdgePanGestureRecognizer")).Send(selAlloc).
			Send(sel("initWithTarget:action:"), v.native, selEdgeSwipe)
		v.native.Send(sel("addGestureRecognizer:"), b.edge)
		b.edge.Send(selRelease)
	case !want && b.edge != 0:
		v.native.Send(sel("removeGestureRecognizer:"), b.edge)
		b.edge = 0
	}
	if b.edge != 0 {
		const left, right = 2, 8
		edge := left
		if s.env.RTL {
			edge = right
		}
		b.edge.Send(sel("setEdges:"), edge)
	}
}

// screenBelow returns the screen under v in its stack.
func (s *surface) screenBelow(v *view) *view {
	up := v.native.Send(selSuperview)
	if up == 0 {
		return nil
	}
	var below *view
	for _, sub := range apple.Subviews(up) {
		if sub == v.native {
			return below
		}
		if o := views[sub]; o != nil && o.bar != nil {
			below = o
		}
	}
	return nil
}

// A stack's screens slide as UINavigationController's do: a pushed
// screen comes in from the trailing edge over the one it covers, which
// moves 30% of the way the other way and dims; a pop is the reverse. The
// core only shows and hides screens, laying out none it hides, so the
// first item of a batch that changes a stack's screens snapshots the
// screen it shows, and once the batch is applied a new top screen slides
// in over that snapshot or the snapshot slides off it. A swipe back is a
// slide of the screens themselves, which the core lays out for it.

const (
	// slideSeconds is how long a push or a pop takes, on the keyboard's
	// curve, as UIKit's.
	slideSeconds = 0.35
	// parallax is how far, of the width, the screen below moves; dimmed
	// how dark it gets.
	parallax = 0.3
	dimmed   = 0.1
	// slideKey names a slide's animations.
	slideKey = "nsgoSlide"
)

// slide is a stack's top screen moving over the one below it. A nil
// screen is snap, the screen the stack showed, as it last drew.
type slide struct {
	stack, top, below *view
	snap, dim         objc.ID // dim is a black view between them
	// swipe follows the finger; popped says it let the top screen go.
	swipe, popped bool
	ended         bool
}

// stackChange is a stack whose screens a batch changes: the screen it
// showed, a snapshot of it where it was, and the screens under it.
type stackChange struct {
	stack, was *view
	snap       objc.ID
	frame      apple.CGRect
	under      []*view
}

// stackOf returns the stack screen v is in, or nil.
func (s *surface) stackOf(v *view) *view { return views[v.native.Send(selSuperview)] }

// touchStack notes that the batch changes the screens of stack, keeping
// how the screen it shows looks before the batch hides or deletes it.
func (s *surface) touchStack(stack *view) {
	if stack == nil || slices.ContainsFunc(s.changes, func(c stackChange) bool { return c.stack == stack }) {
		return
	}
	c := stackChange{stack: stack, was: s.tops[stack]}
	sl := s.slides[stack]
	if was := c.was; was != nil && (sl == nil || !sl.swipe) && s.views[was.id] == was && was.native.Send(selSuperview) == stack.native {
		if c.snap = was.native.Send(sel("snapshotViewAfterScreenUpdates:"), false); c.snap != 0 {
			c.snap.Send(sel("retain"))
			c.frame = objc.Send[apple.CGRect](was.native, sel("frame"))
		}
		for _, sub := range apple.Subviews(stack.native) {
			if sub == was.native {
				break
			}
			if o := views[sub]; o != nil && o.bar != nil {
				c.under = append(c.under, o)
			}
		}
	}
	s.changes = append(s.changes, c)
}

// topScreen returns the last screen of stack the core shows.
func (s *surface) topScreen(stack *view) *view {
	subs := apple.Subviews(stack.native)
	for i := len(subs) - 1; i >= 0; i-- {
		if v := views[subs[i]]; v != nil && v.bar != nil && !v.style.Hidden {
			return v
		}
	}
	return nil
}

// slideStacks slides the screens of each stack whose top screen the batch
// changed.
func (s *surface) slideStacks() {
	for _, c := range s.changes {
		if s.views[c.stack.id] != c.stack {
			s.dropSnapshot(c.snap)
			continue
		}
		now := s.topScreen(c.stack)
		if now == c.was {
			s.dropSnapshot(c.snap)
			continue
		}
		if sl := s.slides[c.stack]; sl != nil {
			s.endSlide(sl)
		}
		s.tops[c.stack] = now
		if now == nil {
			delete(s.tops, c.stack)
			s.dropSnapshot(c.snap)
			continue
		}
		s.slideScreens(c, now)
	}
	clear(s.changes)
	s.changes = s.changes[:0]
}

// dropSnapshot releases a snapshot no slide shows.
func (s *surface) dropSnapshot(snap objc.ID) {
	if snap != 0 {
		snap.Send(selRelease)
	}
}

// slideScreens slides stack c from the screen it showed to now: a pop
// when now was under it, else a push.
func (s *surface) slideScreens(c stackChange, now *view) {
	if c.snap == 0 {
		return
	}
	sl := &slide{stack: c.stack, top: now, snap: c.snap}
	from, to := 1.0, 0.0
	pop := slices.Contains(c.under, now)
	if pop {
		sl.top, sl.below = nil, now
		from, to = 0, 1
	}
	if s.slideWidth(sl) == 0 {
		s.dropSnapshot(c.snap)
		return
	}
	c.snap.Send(selSetFrame, c.frame)
	c.snap.Send(sel("setUserInteractionEnabled:"), false)
	if pop {
		c.stack.native.Send(selAddSubview, c.snap)
	} else {
		c.stack.native.Send(selInsertBelow, c.snap, now.native)
	}
	s.startSlide(sl)
	s.animateSlide(sl, from, to, slideSeconds)
}

// slideViews returns the views of the screens of sl still there, the top
// one first.
func (s *surface) slideViews(sl *slide) (top, below objc.ID) {
	native := func(v *view) objc.ID {
		switch {
		case v == nil:
			return sl.snap
		case s.views[v.id] == v:
			return v.native
		}
		return 0
	}
	return native(sl.top), native(sl.below)
}

// slideWidth is how far the top screen of sl moves going off, negative
// right to left.
func (s *surface) slideWidth(sl *slide) float64 {
	w := objc.Send[apple.CGRect](sl.stack.native, selBounds).Size.Width
	if s.env.RTL {
		return -w
	}
	return w
}

// startSlide puts the dimming view under the top screen of sl.
func (s *surface) startSlide(sl *slide) {
	s.slides[sl.stack] = sl
	sl.dim = objc.ID(objc.GetClass("UIView")).Send(selNew)
	sl.dim.Send(sel("setBackgroundColor:"), objc.ID(objc.GetClass("UIColor")).Send(sel("blackColor")))
	sl.dim.Send(sel("setUserInteractionEnabled:"), false)
	sl.dim.Send(sel("setAlpha:"), 0.0)
	sl.dim.Send(selSetFrame, objc.Send[apple.CGRect](sl.stack.native, selBounds))
	top, _ := s.slideViews(sl)
	sl.stack.native.Send(selInsertBelow, sl.dim, top)
}

// placeSlide puts the top screen of sl a fraction p of the way off, as a
// finger moves it.
func (s *surface) placeSlide(sl *slide, p float64) {
	w := s.slideWidth(sl)
	top, below := s.slideViews(sl)
	if top == 0 || below == 0 {
		return
	}
	top.Send(sel("setTransform:"), affineTransform{A: 1, D: 1, X: p * w})
	below.Send(sel("setTransform:"), affineTransform{A: 1, D: 1, X: -(1 - p) * w * parallax})
	sl.dim.Send(sel("setAlpha:"), (1-p)*dimmed)
}

// animateSlide moves the top screen of sl from a fraction from of the
// way off to to, then ends the slide. Core Animation moves the screens
// over where the batches put them, so they keep changing as they slide.
func (s *surface) animateSlide(sl *slide, from, to, seconds float64) {
	sl.stack.native.Send(sel("setUserInteractionEnabled:"), false)
	sl.dim.Send(sel("setAlpha:"), (1-to)*dimmed)
	w := s.slideWidth(sl)
	top, below := s.slideViews(sl)
	tx := objc.ID(objc.GetClass("CATransaction"))
	tx.Send(sel("begin"))
	done := objc.NewBlock(func(objc.Block) { s.endSlide(sl) })
	tx.Send(sel("setCompletionBlock:"), done)
	slideLayer(top, "transform.translation.x", from*w, to*w, seconds, true)
	slideLayer(below, "transform.translation.x", -(1-from)*w*parallax, -(1-to)*w*parallax, seconds, true)
	slideLayer(sl.dim, "opacity", (1-from)*dimmed, (1-to)*dimmed, seconds, false)
	tx.Send(sel("commit"))
	done.Release()
}

// slideLayer animates key of view's layer from from to to, over its
// value when additive, and holds the end until the slide ends.
func slideLayer(view objc.ID, key string, from, to, seconds float64, additive bool) {
	if view == 0 {
		return
	}
	number := objc.ID(objc.GetClass("NSNumber"))
	c := driver.KeyboardCurve
	anim := objc.ID(objc.GetClass("CABasicAnimation")).Send(sel("animationWithKeyPath:"), apple.NSString(key))
	anim.Send(sel("setFromValue:"), number.Send(sel("numberWithDouble:"), from))
	anim.Send(sel("setToValue:"), number.Send(sel("numberWithDouble:"), to))
	anim.Send(sel("setAdditive:"), additive)
	anim.Send(sel("setDuration:"), seconds)
	anim.Send(sel("setTimingFunction:"), objc.ID(objc.GetClass("CAMediaTimingFunction")).Send(
		sel("functionWithControlPoints::::"), c.X1, c.Y1, c.X2, c.Y2))
	anim.Send(sel("setFillMode:"), apple.NSString("forwards"))
	anim.Send(sel("setRemovedOnCompletion:"), false)
	view.Send(selLayer).Send(sel("addAnimation:forKey:"), anim, apple.NSString(slideKey))
}

// endSlide puts the screens of sl back in place; a swipe tells the core
// where it let go.
func (s *surface) endSlide(sl *slide) {
	if sl.ended {
		return
	}
	sl.ended = true
	if s.slides[sl.stack] == sl {
		delete(s.slides, sl.stack)
	}
	sl.dim.Send(selRemoveFromSuperview)
	sl.dim.Send(selRelease)
	sl.stack.native.Send(sel("setUserInteractionEnabled:"), true)
	top, below := s.slideViews(sl)
	for _, n := range []objc.ID{top, below} {
		if n != 0 {
			n.Send(selLayer).Send(sel("removeAnimationForKey:"), apple.NSString(slideKey))
		}
	}
	if sl.snap != 0 {
		sl.snap.Send(selRemoveFromSuperview)
		sl.snap.Send(selRelease)
	}
	if !sl.swipe {
		return
	}
	for _, v := range []*view{sl.top, sl.below} {
		if s.views[v.id] == v {
			v.native.Send(sel("setTransform:"), identity)
			// Until the core hides it too.
			v.native.Send(selSetHidden, (v == sl.top) == sl.popped)
		}
	}
	if sl.popped {
		s.tops[sl.stack] = sl.below
		s.engine.Dispatch(driver.Event{Kind: driver.EventBack, ID: sl.top.id})
	} else {
		s.engine.Dispatch(driver.Event{Kind: driver.EventBackCancelled})
	}
}

// edgeSwiped follows a swipe back: the screen follows the finger over the
// one below, then either goes, reporting EventBack, or comes back.
func edgeSwiped(self objc.ID, _ objc.SEL, r objc.ID) {
	v := views[self]
	if v == nil || v.bar == nil {
		return
	}
	s, b := v.surface, v.bar
	state := objc.Send[int](r, selState)
	if state == stateBegan {
		b.slide = nil
		stack, below := s.stackOf(v), s.screenBelow(v)
		if stack == nil || below == nil || s.slides[stack] != nil {
			return
		}
		// The core lays the screen below out for the swipe to uncover.
		s.engine.Dispatch(driver.Event{Kind: driver.EventBackStarted})
		below.native.Send(selSetHidden, false)
		b.slide = &slide{stack: stack, top: v, below: below, swipe: true}
		s.startSlide(b.slide)
	}
	sl := b.slide
	if sl == nil || sl.ended {
		return
	}
	up := self.Send(selSuperview)
	w := objc.Send[apple.CGRect](self, selBounds).Size.Width
	sign := 1.0
	if s.env.RTL {
		sign = -1
	}
	p := min(max(objc.Send[apple.CGPoint](r, sel("translationInView:"), up).X*sign/max(w, 1), 0), 1)
	switch state {
	case stateBegan, stateChanged:
		s.placeSlide(sl, p)
	case stateEnded, stateCancelled, stateFailed:
		velocity := objc.Send[apple.CGPoint](r, sel("velocityInView:"), up).X * sign
		sl.popped = state == stateEnded && (p > 1.0/3 || velocity > 500)
		to := 0.0
		if sl.popped {
			to = 1
		}
		// From where the finger left it, on what is left of a slide's time.
		top, below := s.slideViews(sl)
		for _, n := range []objc.ID{top, below} {
			if n != 0 {
				n.Send(sel("setTransform:"), identity)
			}
		}
		s.animateSlide(sl, p, to, slideSeconds*max(math.Abs(to-p), 0.2))
	}
}

// isSheet reports whether modal v is presented as a UIKit sheet.
func (v *view) isSheet() bool { return v.modal.Presentation != driver.ModalFullScreen }

// presentSheet presents modal v as a sheet over the top view controller.
func (s *surface) presentSheet(v *view) {
	c := v.modal
	ctrl := objc.ID(classSheet).Send(selNew)
	container := ctrl.Send(sel("view"))
	container.Send(sel("setBackgroundColor:"), objc.ID(objc.GetClass("UIColor")).Send(sel("systemBackgroundColor")))
	container.Send(selAddSubview, v.native)
	const pageSheet, formSheet = 1, 2
	style := pageSheet
	if c.Presentation == driver.ModalFormSheet {
		style = formSheet
	}
	ctrl.Send(sel("setModalPresentationStyle:"), style)
	if c.Animation == driver.ModalFade {
		const crossDissolve = 2
		ctrl.Send(sel("setModalTransitionStyle:"), crossDissolve)
	}
	ctrl.Send(sel("setModalInPresentation:"), c.NoCloseOnSwipe)
	ctrl.Send(sel("setModalPresentationCapturesStatusBarAppearance:"), true)
	ctrl.Send(sel("presentationController")).Send(selSetDeleg, ctrl)
	v.sheet = &sheet{controller: ctrl}
	sheets[ctrl] = v
	s.topController().Send(sel("presentViewController:animated:completion:"), ctrl, c.Animation != driver.ModalNone, objc.ID(0))
}

// dismissSheet takes modal v's sheet away, unless the user did.
func (s *surface) dismissSheet(v *view) {
	sh := v.sheet
	if sh == nil {
		return
	}
	v.sheet = nil
	delete(sheets, sh.controller)
	if !sh.gone {
		// Through the presenter, which dismisses what this sheet presents
		// too.
		from := sh.controller.Send(sel("presentingViewController"))
		if from == 0 {
			from = sh.controller
		}
		from.Send(sel("dismissViewControllerAnimated:completion:"), v.modal.Animation != driver.ModalNone, objc.ID(0))
	}
	sh.controller.Send(selRelease)
}

// sheetLaidOut reports the size UIKit gave sheet v and what of it the
// system covers.
func (s *surface) sheetLaidOut(v *view) {
	sh := v.sheet
	container := sh.controller.Send(sel("view"))
	size := objc.Send[apple.CGRect](container, selBounds).Size
	in := objc.Send[edgeInsets](container, sel("safeAreaInsets"))
	got := driver.Size{Width: driver.Points(size.Width), Height: driver.Points(size.Height)}
	insets := driver.Edges{Top: driver.Points(in.Top), Right: driver.Points(in.Right), Bottom: driver.Points(in.Bottom), Left: driver.Points(in.Left)}
	if got == sh.size && insets == sh.insets {
		return
	}
	sh.size, sh.insets = got, insets
	s.engine.Dispatch(driver.Event{Kind: driver.EventModalSize, ID: v.id, Size: got, Insets: insets})
}

// orientations is the UIInterfaceOrientationMask of the last presented
// modal that limits them, or 0.
func (s *surface) orientations() uint {
	for i := len(s.oriented) - 1; i >= 0; i-- {
		v := s.oriented[i]
		if s.views[v.id] != v || (v.sheet == nil && !v.presented) || (v.sheet != nil && v.sheet.gone) {
			continue
		}
		var m uint
		for o, bit := range orientationMasks {
			if v.modal.Orientations&o != 0 {
				m |= bit
			}
		}
		return m
	}
	return 0
}

// updateOrientations has UIKit ask the orientations again.
func (s *surface) updateOrientations() {
	live := s.oriented[:0]
	for _, v := range s.oriented {
		if s.views[v.id] == v {
			live = append(live, v)
		}
	}
	s.oriented = live
	for _, c := range []objc.ID{s.viewController(), s.topController()} {
		if objc.Send[bool](c, sel("respondsToSelector:"), sel("setNeedsUpdateOfSupportedInterfaceOrientations")) {
			c.Send(sel("setNeedsUpdateOfSupportedInterfaceOrientations"))
		}
	}
}

// setStatusBar applies the status bar the core merged.
func (s *surface) setStatusBar(c *driver.StatusBarConfig) {
	const lightContent, darkContent = 1, 3
	s.statusBar = 0
	switch c.Style {
	case driver.StatusBarDark:
		s.statusBar = darkContent
	case driver.StatusBarLight:
		s.statusBar = lightContent
	}
	s.hideBar, s.barAnimated = c.Hidden, c.Animated
	update := func() {
		// An embedding host's controller asks the view (NSGoView's
		// preferredStatusBarStyle) if it forwards it.
		s.viewController().Send(sel("setNeedsStatusBarAppearanceUpdate"))
		for c, v := range sheets {
			if v.surface == s {
				c.Send(sel("setNeedsStatusBarAppearanceUpdate"))
			}
		}
	}
	if !c.Animated {
		update()
		return
	}
	block := objc.NewBlock(func(objc.Block) { update() })
	objc.ID(objc.GetClass("UIView")).Send(sel("animateWithDuration:animations:"), 0.25, block)
	block.Release()
}

// barAnimation is the UIStatusBarAnimation of the last change.
func (s *surface) barAnimation() int {
	if s.barAnimated {
		const fade = 1
		return fade
	}
	return 0
}
