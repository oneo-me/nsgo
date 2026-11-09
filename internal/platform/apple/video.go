//go:build darwin

package apple

import (
	"math"
	"path/filepath"
	"runtime"
	"strings"
	"structs"
	"time"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/platform/imageload"
)

// Video is a video's AVPlayer, filling the view that holds it: in an
// AVPlayerLayer, or with the platform's controls in an AVPlayerView
// (macOS) or an AVPlayerViewController (iOS). AVFoundation and AVKit load
// on the first video, so apps without one do not pay for them.
type Video struct {
	// Dispatch reports the video's events.
	Dispatch func(driver.Event)
	// Parent is the view controller an iOS player's controls join when
	// the view is not in one yet.
	Parent objc.ID

	holder     objc.ID
	player     objc.ID
	shown      objc.ID // macOS: the layer view or the AVPlayerView
	layer      objc.ID // the AVPlayerLayer, without controls
	controller objc.ID // iOS: the AVPlayerViewController, with them
	timeToken  objc.ID
	endToken   objc.ID
	status     *videoStatus
	config     driver.VideoConfig
	made       bool
}

// videoStatus is what a player reported so far.
type videoStatus struct {
	loaded, failed, buffering bool
}

// cmTime is CoreMedia's CMTime.
type cmTime struct {
	_         structs.HostLayout
	Value     int64
	Timescale int32
	Flags     uint32
	Epoch     int64
}

func cmTimeOf(d time.Duration) cmTime {
	return cmTime{Value: int64(math.Round(d.Seconds() * 600)), Timescale: 600, Flags: 1}
}

// duration is t, or 0 when it is invalid or indefinite (live streams).
func (t cmTime) duration() time.Duration {
	const valid, indefinite = 1, 0x10
	if t.Flags&valid == 0 || t.Flags&indefinite != 0 || t.Timescale == 0 {
		return 0
	}
	return time.Duration(float64(t.Value) / float64(t.Timescale) * float64(time.Second))
}

var (
	avFoundation, avKit         bool
	avFoundationTried, kitTried bool
	classVideoLayerView         objc.Class
)

func loadAVFoundation() bool {
	if !avFoundationTried {
		avFoundationTried = true
		_, err := purego.Dlopen("/System/Library/Frameworks/AVFoundation.framework/AVFoundation", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		avFoundation = err == nil
		if avFoundation && runtime.GOOS != "ios" {
			// A layer-hosting view showing the player layer, which lets
			// the pointer through to the node's view.
			classVideoLayerView = RegisterClass("NsgoVideoLayerView", Class("NSView"), nil, []objc.MethodDef{
				{Cmd: sel("isFlipped"), Fn: func(objc.ID, objc.SEL) bool { return true }},
				{Cmd: sel("hitTest:"), Fn: func(objc.ID, objc.SEL, CGPoint) objc.ID { return 0 }},
			})
		}
	}
	return avFoundation
}

func loadAVKit() bool {
	if !kitTried {
		kitTried = true
		_, err := purego.Dlopen("/System/Library/Frameworks/AVKit.framework/AVKit", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		avKit = err == nil
	}
	return avKit
}

// playerClass is AVKit's class name when controls are asked for and
// AVKit loads, else 0.
func playerClass(controls bool, name string) objc.Class {
	if !controls || !loadAVKit() {
		return 0
	}
	return Class(name)
}

// NewVideo returns the video of holder, empty until configured.
func NewVideo(holder objc.ID) *Video {
	return &Video{holder: holder, status: &videoStatus{}}
}

// View returns the AppKit view showing the player (macOS), 0 before one.
func (v *Video) View() objc.ID { return v.shown }

// Configure applies c, opening the source again when it changed.
func (v *Video) Configure(c *driver.VideoConfig) {
	if !loadAVFoundation() {
		return
	}
	old, first := v.config, !v.made
	v.config, v.made = *c, true
	reopened := first || c.Source != old.Source
	if reopened {
		v.open()
	}
	if reopened || c.Controls != old.Controls {
		v.show()
	}
	if reopened || c.ProgressInterval != old.ProgressInterval {
		v.observe()
	}
	if v.player != 0 {
		v.player.Send(sel("setMuted:"), c.Muted)
		v.player.Send(sel("setVolume:"), c.Volume)
		if c.Paused {
			v.player.Send(sel("pause"))
		} else {
			v.player.Send(sel("setRate:"), c.Rate)
		}
	}
	// The audio session follows what plays, not what is mounted.
	if v.player != 0 && !c.Paused {
		audioPlaying(v, c.AudioSession)
	} else {
		audioPlaying(v, driver.AudioSessionDefault)
	}
	gravity := NSString("AVLayerVideoGravityResizeAspect")
	switch c.ResizeMode {
	case driver.Cover:
		gravity = NSString("AVLayerVideoGravityResizeAspectFill")
	case driver.Stretch:
		gravity = NSString("AVLayerVideoGravityResize")
	}
	switch {
	case v.layer != 0:
		v.layer.Send(sel("setVideoGravity:"), gravity)
	case v.controller != 0:
		v.controller.Send(sel("setVideoGravity:"), gravity)
	case v.shown != 0:
		v.shown.Send(sel("setVideoGravity:"), gravity)
	}
}

// locate finds source: "asset:" paths under root, a URL when it has a
// scheme, else a file path. ok is false when there is no root.
func locate(source, root string) (url, path string, ok bool) {
	if p, isAsset := strings.CutPrefix(source, "asset:"); isAsset {
		if root == "" {
			return "", "", false
		}
		return "", filepath.Join(root, filepath.FromSlash(strings.TrimLeft(p, "/"))), true
	}
	if strings.Contains(source, "://") {
		return source, "", true
	}
	return "", source, true
}

// videoURL is the NSURL of source, 0 when it has none.
func videoURL(source string) objc.ID {
	url, path, ok := locate(source, imageload.ResourceRoot)
	nsURL := objc.ID(Class("NSURL"))
	switch {
	case !ok:
		return 0
	case url != "":
		return nsURL.Send(sel("URLWithString:"), NSString(url))
	}
	return nsURL.Send(sel("fileURLWithPath:"), NSString(path))
}

// open starts playing the source, or nothing.
func (v *Video) open() {
	v.close()
	st := &videoStatus{}
	v.status = st
	source := v.config.Source
	if source == "" {
		return
	}
	url := videoURL(source)
	if url == 0 {
		st.failed = true
		v.Dispatch(driver.Event{Kind: driver.EventError, Text: source + ": cannot resolve the video's location"})
		return
	}
	v.player = objc.ID(Class("AVPlayer")).Send(sel("playerWithURL:"), url).Send(sel("retain"))
	v.Dispatch(driver.Event{Kind: driver.EventLoadStart})
	// The end of the item: report it, and loop if asked.
	ended := objc.NewBlock(func(objc.Block, objc.ID) {
		if v.status != st || v.player == 0 {
			return
		}
		v.Dispatch(driver.Event{Kind: driver.EventEnd})
		if !v.config.Repeat {
			// It stopped: the other apps' audio may play again.
			audioPlaying(v, driver.AudioSessionDefault)
		}
		if v.config.Repeat {
			v.player.Send(sel("seekToTime:"), cmTimeOf(0))
			v.player.Send(sel("play"))
		}
	})
	center := objc.ID(Class("NSNotificationCenter")).Send(sel("defaultCenter"))
	queue := objc.ID(Class("NSOperationQueue")).Send(selMainQueue)
	v.endToken = center.Send(selAddObserver, NSString("AVPlayerItemDidPlayToEndTimeNotification"),
		v.player.Send(sel("currentItem")), queue, ended).Send(sel("retain"))
	ended.Release()
	v.poll(st, 0)
}

// close stops the player and stops observing it. A macOS view keeps the
// last frame; an iOS one lets the player's layer or controller go.
func (v *Video) close() {
	audioPlaying(v, driver.AudioSessionDefault)
	if v.player != 0 && v.timeToken != 0 {
		v.player.Send(sel("removeTimeObserver:"), v.timeToken)
	}
	if v.endToken != 0 {
		objc.ID(Class("NSNotificationCenter")).Send(sel("defaultCenter")).Send(sel("removeObserver:"), v.endToken)
	}
	for _, id := range []objc.ID{v.timeToken, v.endToken} {
		if id != 0 {
			id.Send(sel("release"))
		}
	}
	v.timeToken, v.endToken = 0, 0
	if v.player != 0 {
		v.player.Send(sel("pause"))
		v.player.Send(sel("release"))
		v.player = 0
	}
	if runtime.GOOS == "ios" {
		v.detach()
	}
}

// show shows the player in a layer, or with the platform's controls.
func (v *Video) show() {
	if runtime.GOOS == "ios" {
		v.attach()
		return
	}
	if v.shown != 0 {
		v.shown.Send(sel("removeFromSuperview"))
		v.shown.Send(sel("release"))
		v.shown = 0
	}
	if v.layer != 0 {
		v.layer.Send(sel("release"))
		v.layer = 0
	}
	if v.player == 0 {
		return
	}
	var shown objc.ID
	if c := playerClass(v.config.Controls, "AVPlayerView"); c != 0 {
		shown = objc.ID(c).Send(sel("alloc")).Send(sel("initWithFrame:"), CGRect{})
		shown.Send(sel("setPlayer:"), v.player)
		const inline = 1 // AVPlayerViewControlsStyleInline
		shown.Send(sel("setControlsStyle:"), inline)
	} else if c := Class("AVPlayerLayer"); c != 0 {
		v.layer = objc.ID(c).Send(sel("playerLayerWithPlayer:"), v.player).Send(sel("retain"))
		shown = objc.ID(classVideoLayerView).Send(sel("alloc")).Send(sel("initWithFrame:"), CGRect{})
		// Layer-hosting: the player layer is the view's own, so it
		// follows the view's size.
		shown.Send(sel("setLayer:"), v.layer)
		shown.Send(sel("setWantsLayer:"), true)
	} else {
		return
	}
	WithoutAnimation(func() { fill(v.holder, shown) })
	v.shown = shown
}

// attach shows an iOS player in a layer, or in an
// AVPlayerViewController with its controls.
func (v *Video) attach() {
	v.detach()
	if v.player == 0 {
		return
	}
	bounds := objc.Send[CGRect](v.holder, sel("bounds"))
	if c := playerClass(v.config.Controls, "AVPlayerViewController"); c != 0 {
		ctrl := objc.ID(c).Send(sel("new"))
		ctrl.Send(sel("setPlayer:"), v.player)
		ctrl.Send(sel("setShowsPlaybackControls:"), true)
		if view := ctrl.Send(sel("view")); view != 0 {
			view.Send(sel("setFrame:"), bounds)
			fill(v.holder, view)
		}
		if parent := v.parentController(); parent != 0 {
			parent.Send(sel("addChildViewController:"), ctrl)
			ctrl.Send(sel("didMoveToParentViewController:"), parent)
		}
		v.controller = ctrl
	} else if c := Class("AVPlayerLayer"); c != 0 {
		v.layer = objc.ID(c).Send(sel("playerLayerWithPlayer:"), v.player).Send(sel("retain"))
		WithoutAnimation(func() {
			v.layer.Send(sel("setFrame:"), bounds)
			v.holder.Send(sel("layer")).Send(sel("addSublayer:"), v.layer)
		})
	}
}

// parentController is the nearest view controller of the holder, else
// Parent: the view joins the window after its video is configured.
func (v *Video) parentController() objc.ID {
	vc := Class("UIViewController")
	for r := v.holder.Send(sel("nextResponder")); r != 0; r = r.Send(sel("nextResponder")) {
		if objc.Send[bool](r, sel("isKindOfClass:"), vc) {
			return r
		}
	}
	return v.Parent
}

// detach undoes attach.
func (v *Video) detach() {
	if v.layer != 0 {
		v.layer.Send(sel("removeFromSuperlayer"))
		v.layer.Send(sel("release"))
		v.layer = 0
	}
	if v.controller != 0 {
		v.controller.Send(sel("willMoveToParentViewController:"), objc.ID(0))
		v.controller.Send(sel("view")).Send(sel("removeFromSuperview"))
		v.controller.Send(sel("removeFromParentViewController"))
		v.controller.Send(sel("release"))
		v.controller = 0
	}
}

// fill adds view to holder at its size, resizing with it.
func fill(holder, view objc.ID) {
	view.Send(sel("setFrame:"), objc.Send[CGRect](holder, sel("bounds")))
	const flexibleSize = 2 | 16 // width and height, in AppKit and UIKit alike
	view.Send(sel("setAutoresizingMask:"), uint(flexibleSize))
	holder.Send(sel("addSubview:"), view)
}

// Layout keeps an iOS player layer at the view's size.
func (v *Video) Layout() {
	if runtime.GOOS == "ios" && v.layer != 0 {
		WithoutAnimation(func() { v.layer.Send(sel("setFrame:"), objc.Send[CGRect](v.holder, sel("bounds"))) })
	}
}

// observe reports the progress every ProgressInterval while time passes,
// and the load, buffering and failure as they happen.
func (v *Video) observe() {
	if v.player == 0 {
		return
	}
	if v.timeToken != 0 {
		v.player.Send(sel("removeTimeObserver:"), v.timeToken)
		v.timeToken.Send(sel("release"))
		v.timeToken = 0
	}
	st := v.status
	// The block's CMTime argument is left out: the player's current time
	// is the same, and purego callbacks take no structs.
	tick := objc.NewBlock(func(objc.Block) {
		if v.status != st || v.player == 0 {
			return
		}
		v.report()
		at := max(objc.Send[cmTime](v.player, sel("currentTime")).duration(), 0)
		v.Dispatch(driver.Event{Kind: driver.EventProgress, Time: at, Duration: v.itemDuration()})
	})
	// A nil queue is the main queue.
	v.timeToken = v.player.Send(sel("addPeriodicTimeObserverForInterval:queue:usingBlock:"),
		cmTimeOf(v.config.ProgressInterval), objc.ID(0), tick).Send(sel("retain"))
	tick.Release()
}

// poll reports the player's state every 100ms until it loaded or failed,
// for at most 30 seconds: a paused player reports no time to the periodic
// observer.
func (v *Video) poll(st *videoStatus, attempt int) {
	if v.status != st || v.player == 0 {
		return
	}
	v.report()
	if st.loaded || st.failed || attempt >= 300 {
		return
	}
	time.AfterFunc(100*time.Millisecond, func() { OnMain(func() { v.poll(st, attempt+1) }) })
}

func (v *Video) itemDuration() time.Duration {
	item := v.player.Send(sel("currentItem"))
	if item == 0 {
		return 0
	}
	return objc.Send[cmTime](item, sel("duration")).duration()
}

// report reports a finished load and a failure, each once, and buffering
// changes.
func (v *Video) report() {
	item := v.player.Send(sel("currentItem"))
	if item == 0 {
		return
	}
	st := v.status
	const ready, failed = 1, 2 // AVPlayerItemStatus
	switch state := objc.Send[int](item, sel("status")); {
	case state == ready && !st.loaded:
		st.loaded = true
		size := objc.Send[CGSize](item, sel("presentationSize"))
		v.Dispatch(driver.Event{Kind: driver.EventLoad, Duration: v.itemDuration(),
			Size: driver.Size{Width: driver.Points(size.Width), Height: driver.Points(size.Height)}})
	case state == failed && !st.failed:
		st.failed = true
		message := "the video failed to load"
		if err := item.Send(sel("error")); err != 0 {
			message = GoString(err.Send(sel("localizedDescription")))
		}
		v.Dispatch(driver.Event{Kind: driver.EventError, Text: message})
	}
	const waitingToPlay = 1 // AVPlayerTimeControlStatusWaitingToPlayAtSpecifiedRate
	waiting := objc.Send[int](v.player, sel("timeControlStatus")) == waitingToPlay && !st.failed
	if st.buffering != waiting {
		st.buffering = waiting
		v.Dispatch(driver.Event{Kind: driver.EventBuffer, Buffering: waiting})
	}
}

// Seek jumps to position exactly.
func (v *Video) Seek(position time.Duration) {
	if v.player != 0 {
		zero := cmTimeOf(0)
		v.player.Send(sel("seekToTime:toleranceBefore:toleranceAfter:"), cmTimeOf(max(position, 0)), zero, zero)
	}
}

// Close stops the player. The view keeps the last frame while it still
// shows.
func (v *Video) Close() {
	v.close()
	v.status = nil
	for _, id := range []objc.ID{v.shown, v.layer} {
		if id != 0 {
			id.Send(sel("release"))
		}
	}
	v.shown, v.layer = 0, 0
}
