package core

import (
	"time"

	"github.com/oneo-me/nsgo/internal/driver"
)

// Video plays a video in the platform's player, filling the view: the
// picture fits it by ResizeMode, and Controls shows playback controls.
// It plays unless Paused; Ref.Seek jumps to a time.
type Video struct {
	// Source is a URL, a file path or an "asset:" path in the app's
	// resources.
	Source     string
	SourceFunc func() string
	Paused     bool
	PausedFunc func() bool
	Muted      bool
	MutedFunc  func() bool
	// Volume runs from 0 to 1; 0 plays at full volume, and Muted
	// silences the video.
	Volume     float32
	VolumeFunc func() float32
	// Rate is the playback speed; 0 plays at 1.
	Rate     float32
	RateFunc func() float32
	// Repeat starts the video over when it ends.
	Repeat         bool
	ResizeMode     ResizeMode
	ResizeModeFunc func() ResizeMode
	Controls       bool
	// ProgressInterval is the time between OnProgress calls while the
	// video plays: 250 milliseconds when 0, and at least 16.
	ProgressInterval time.Duration
	// AudioSession is what the sound does to other apps' audio while it
	// plays: by default the app's settings decide, and on iOS the
	// device's silent mode silences it; a video of speech, as an
	// audiobook's, plays in silent mode with AudioSessionPlayback or
	// AudioSessionMixWithOthers. Changing it on Android opens the source
	// again.
	AudioSession AudioSession
	// Class is Tailwind CSS classes, as View's.
	Class     string
	Style     Style
	StyleFunc func() Style

	// OnLoadStart is called when a source starts opening, and OnLoad once
	// its duration and size are known.
	OnLoadStart func()
	OnLoad      func(VideoInfo)
	// OnProgress receives the position and duration while the video
	// plays, and once more when it pauses or ends.
	OnProgress func(position, duration time.Duration)
	// OnEnd is called each time the video reaches its end, before a
	// repeating one starts over.
	OnEnd func()
	// OnBuffer receives whether playback waits for data.
	OnBuffer func(buffering bool)
	OnError  func(err error)
	OnPress  func(PressEvent)
	OnLayout func(Bounds)
	Ref      *Ref
	Label    string
	TestID   string
}

// Render implements Node.
func (v Video) Render(*Scope) Node { return v }

// VideoInfo is what a video reports when it loads. Duration is 0 for a
// stream without one, such as a live broadcast.
type VideoInfo struct {
	Duration time.Duration
	// Size is the picture's natural size in points.
	Size Size
}

type videoNode struct {
	config, sent driver.VideoConfig
	loadStart    func()
	load         func(VideoInfo)
	progress     func(position, duration time.Duration)
	end          func()
	buffer       func(bool)
	loadError    func(error)
	// refused is the source Access refused last.
	refused string
}

func (t *Tree) mountVideo(cx *Scope, parent *node, v Video) {
	n := t.newNode(driver.KindVideo, parent, cx)
	t.setClasses(n, v.Class)
	t.bindStyle(cx, n, v.Style, v.StyleFunc)
	t.setRef(n, v.Ref)
	n.h = handlers{press: v.OnPress, layout: v.OnLayout}
	n.props = driver.ViewProps{Label: v.Label, TestID: v.TestID}
	vn := &videoNode{
		config: driver.VideoConfig{Source: v.Source, Paused: v.Paused, Muted: v.Muted, Volume: videoVolume(v.Volume),
			Rate: videoRate(v.Rate), Repeat: v.Repeat, ResizeMode: v.ResizeMode, Controls: v.Controls,
			ProgressInterval: videoInterval(v.ProgressInterval), AudioSession: v.AudioSession},
		loadStart: v.OnLoadStart, load: v.OnLoad, progress: v.OnProgress, end: v.OnEnd,
		buffer: v.OnBuffer, loadError: v.OnError,
	}
	n.video = vn
	bind := func(set func(*driver.VideoConfig)) {
		cx.renderEffect(func() {
			c := vn.config
			set(&c)
			if c != vn.config {
				vn.config = c
				t.touch(n)
			}
		})
	}
	if f := v.SourceFunc; f != nil {
		bind(func(c *driver.VideoConfig) { c.Source = f() })
	}
	if f := v.PausedFunc; f != nil {
		bind(func(c *driver.VideoConfig) { c.Paused = f() })
	}
	if f := v.MutedFunc; f != nil {
		bind(func(c *driver.VideoConfig) { c.Muted = f() })
	}
	if f := v.VolumeFunc; f != nil {
		bind(func(c *driver.VideoConfig) { c.Volume = videoVolume(f()) })
	}
	if f := v.ResizeModeFunc; f != nil {
		bind(func(c *driver.VideoConfig) { c.ResizeMode = f() })
	}
	if f := v.RateFunc; f != nil {
		bind(func(c *driver.VideoConfig) { c.Rate = videoRate(f()) })
	}
}

func videoVolume(v float32) float32 {
	if v <= 0 || v > 1 || v != v {
		return 1
	}
	return v
}

func videoRate(r float32) float32 {
	if r <= 0 || r != r {
		return 1
	}
	return r
}

func videoInterval(d time.Duration) time.Duration {
	if d == 0 {
		return 250 * time.Millisecond
	}
	return max(d, 16*time.Millisecond)
}

// dispatchVideo handles a video's events.
func (t *Tree) dispatchVideo(n *node, ev driver.Event) bool {
	vn := n.video
	call := func(fn func()) {
		t.within(n.owner, fn)
	}
	switch ev.Kind {
	case driver.EventLoadStart:
		if vn.loadStart != nil {
			call(vn.loadStart)
		}
	case driver.EventLoad:
		if vn.load != nil {
			call(func() { vn.load(VideoInfo{Duration: ev.Duration, Size: ev.Size}) })
		}
	case driver.EventProgress:
		if vn.progress != nil {
			call(func() { vn.progress(ev.Time, ev.Duration) })
		}
	case driver.EventEnd:
		if vn.end != nil {
			call(vn.end)
		}
	case driver.EventBuffer:
		if vn.buffer != nil {
			call(func() { vn.buffer(ev.Buffering) })
		}
	case driver.EventError:
		if vn.loadError != nil {
			call(func() { vn.loadError(&MediaError{Message: ev.Text}) })
		}
	default:
		return false
	}
	return true
}

// events is what a video reports.
func (vn *videoNode) events() driver.EventMask {
	var m driver.EventMask
	bit := func(k driver.EventKind, on bool) {
		if on {
			m |= 1 << k
		}
	}
	bit(driver.EventLoadStart, vn.loadStart != nil)
	bit(driver.EventLoad, vn.load != nil)
	bit(driver.EventProgress, vn.progress != nil)
	bit(driver.EventEnd, vn.end != nil)
	bit(driver.EventBuffer, vn.buffer != nil)
	bit(driver.EventError, vn.loadError != nil)
	return m
}

// MediaError is a video that failed to open or play.
type MediaError struct {
	Message string
}

func (e *MediaError) Error() string { return e.Message }

// Seek jumps the video r names to position.
func (r *Ref) Seek(position time.Duration) {
	if r.n == nil || r.n.deleted || r.n.video == nil {
		return
	}
	r.t.commands = append(r.t.commands, driver.MountItem{Kind: driver.ItemVideoSeek, ID: r.n.id, Time: max(position, 0)})
	r.t.changed = true
}
