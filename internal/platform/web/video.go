//go:build js && wasm

package web

import (
	"syscall/js"
	"time"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// video is a video view's <video>, as NSH's web backend has it: the
// browser's media element filling the view, reporting its own events.
type video struct {
	el       js.Value
	src      string // the URL it plays, "" for none
	interval time.Duration
	// progress reports EventProgress every interval while playing.
	progress js.Func
	timer    js.Value
	// buffering is what EventBuffer last said.
	buffering bool
	warned    bool // that the browser would not play before a gesture
}

// videoFit is the CSS object-fit of each resize mode.
var videoFit = map[driver.ResizeMode]string{driver.Cover: "cover", driver.Contain: "contain", driver.Stretch: "fill", driver.Center: "none",
	driver.ScaleDown: "scale-down"}

func (s *surface) createVideo(v *view) {
	vd := &video{el: s.element("video")}
	v.video = vd
	vd.el.Set("playsInline", true)
	setStyle(vd.el, map[string]string{"width": "100%", "height": "100%", "border-radius": "inherit"})
	v.inner = vd.el
	v.el.Call("appendChild", vd.el)
	vd.progress = js.FuncOf(func(js.Value, []js.Value) any {
		s.videoProgress(v)
		return nil
	})
	on := func(name string, fn func()) { v.listeners.on(vd.el, name, func(js.Value) { fn() }) }
	on("loadstart", func() {
		if vd.el.Call("hasAttribute", "src").Bool() {
			s.send(v, driver.Event{Kind: driver.EventLoadStart})
		}
	})
	on("loadedmetadata", func() {
		s.send(v, driver.Event{Kind: driver.EventLoad, Duration: vd.seconds("duration"), Size: driver.Size{
			Width:  driver.Points(vd.el.Get("videoWidth").Float()),
			Height: driver.Points(vd.el.Get("videoHeight").Float()),
		}})
	})
	on("playing", func() {
		s.videoBuffer(v, false)
		vd.stopProgress()
		if v.events.Has(driver.EventProgress) {
			vd.timer = js.Global().Call("setInterval", vd.progress, vd.interval.Milliseconds())
		}
	})
	on("pause", func() {
		vd.stopProgress()
		s.videoProgress(v)
	})
	on("waiting", func() { s.videoBuffer(v, true) })
	on("canplay", func() { s.videoBuffer(v, false) })
	on("ended", func() {
		vd.stopProgress()
		s.videoProgress(v)
		s.send(v, driver.Event{Kind: driver.EventEnd})
	})
	on("error", func() {
		vd.stopProgress()
		e := vd.el.Get("error")
		if e.Truthy() && vd.el.Call("hasAttribute", "src").Bool() {
			msg := e.Get("message").String()
			if msg == "" {
				msg = "media error " + e.Get("code").String()
			}
			s.send(v, driver.Event{Kind: driver.EventError, Text: msg})
		}
	})
}

// seconds reads one of the element's times in seconds; NaN and the
// infinity of a live stream are 0.
func (vd *video) seconds(name string) time.Duration {
	f := vd.el.Get(name).Float()
	if f != f || f > 1e12 {
		return 0
	}
	return time.Duration(f * float64(time.Second))
}

func (s *surface) videoProgress(v *view) {
	vd := v.video
	s.send(v, driver.Event{Kind: driver.EventProgress, Time: vd.seconds("currentTime"), Duration: vd.seconds("duration")})
}

// videoBuffer reports whether the video waits for data when that changed.
func (s *surface) videoBuffer(v *view, on bool) {
	if v.video.buffering == on {
		return
	}
	v.video.buffering = on
	s.send(v, driver.Event{Kind: driver.EventBuffer, Buffering: on})
}

func (vd *video) stopProgress() {
	if vd.timer.Truthy() {
		js.Global().Call("clearInterval", vd.timer)
		vd.timer = js.Undefined()
	}
}

func (s *surface) setVideo(v *view, c *driver.VideoConfig) {
	vd := v.video
	url := ""
	if c.Source != "" {
		url, _ = imageURL(driver.ImageSource{URI: c.Source})
	}
	if url != vd.src {
		vd.src = url
		if url != "" {
			vd.el.Set("src", url)
		} else {
			vd.el.Call("removeAttribute", "src")
		}
	}
	vd.interval = c.ProgressInterval
	el := vd.el
	el.Set("muted", c.Muted)
	el.Set("volume", min(max(c.Volume, 0), 1))
	el.Set("playbackRate", c.Rate)
	el.Set("loop", c.Repeat)
	el.Set("controls", c.Controls)
	fit, ok := videoFit[c.ResizeMode]
	if !ok {
		fit = "contain"
	}
	setStyle(el, map[string]string{"object-fit": fit})
	if c.Paused {
		el.Call("pause")
	} else if url != "" && el.Get("paused").Bool() {
		// Settled either way, with the error when it fails.
		var settled js.Func
		settled = js.FuncOf(func(_ js.Value, args []js.Value) any {
			settled.Release()
			// Browsers only start sound on their own after a user gesture.
			if err := args[0]; err.Truthy() && err.Get("name").String() == "NotAllowedError" && !vd.warned {
				vd.warned = true
				js.Global().Get("console").Call("warn",
					"nsgo: the browser did not let the video play before a user gesture (muted videos can)")
			}
			return nil
		})
		el.Call("play").Call("then", settled, settled)
	}
}

// seek moves the video to t.
func (vd *video) seek(t time.Duration) {
	vd.el.Set("currentTime", t.Seconds())
}

// release stops the video: a removed media element keeps playing until
// it is collected.
func (vd *video) release() {
	vd.stopProgress()
	vd.progress.Release()
	vd.el.Call("pause")
	vd.el.Call("removeAttribute", "src")
	vd.el.Call("load")
}
