//go:build windows

package windows

// Videos, as NSH's native/video.rs: Media Foundation's media engine in
// frame server mode, through COM over syscall.
//
// The engine decodes on the window's Direct3D device, shared through a
// DXGI device manager; Direct3D's multithread protection is on, since
// Media Foundation's threads use the device too. While a video plays, a
// frame client of the window (onFrame) asks the engine for a new frame at
// every display frame (OnVideoStreamTick) and copies it into a texture
// Direct2D draws as a bitmap (TransferVideoFrame): the picture is
// ordinary view content, clipped, transformed and covered like any other.
//
// The engine reports to a notify object on its own threads; each
// notification is posted to the main thread and finds its video again by
// node ID, with the serial of the source it was opened for, so a late
// notification of a replaced source is dropped.
//
// Events follow core's Video: EventLoadStart when a source opens,
// EventLoad with its duration and size once its metadata is known,
// EventProgress every ProgressInterval while playing, EventBuffer,
// EventEnd (not for a repeating video: the engine loops without ending),
// EventError. Controls draws a control bar of the
// window's own (play and pause, a scrubber, the time) that drives the
// engine directly, as the platforms' controls do.

import (
	"fmt"
	"math"
	"net/url"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/imageload"
)

var (
	mfplat                    = syscall.NewLazyDLL("mfplat.dll")
	mfStartup                 = mfplat.NewProc("MFStartup")
	mfCreateDXGIDeviceManager = mfplat.NewProc("MFCreateDXGIDeviceManager")
	mfCreateAttributes        = mfplat.NewProc("MFCreateAttributes")
	coCreateInstance          = ole32.NewProc("CoCreateInstance")
	oleaut32                  = syscall.NewLazyDLL("oleaut32.dll")
	sysAllocString            = oleaut32.NewProc("SysAllocString")
	sysFreeString             = oleaut32.NewProc("SysFreeString")
)

var (
	clsidMediaEngineClassFactory = parseGUID("b44392da-499b-446b-a4cb-005fead0e6d5")
	iidMediaEngineClassFactory   = parseGUID("4d645ace-26aa-4688-9be1-df3516990b93")
	iidMediaEngineNotify         = parseGUID("fee7c112-e776-42b5-9bbf-0048524e2bd5")
	mfMediaEngineDXGIManager     = parseGUID("065702da-1094-486d-8617-ee7cc4ee4648")
	mfMediaEngineCallback        = parseGUID("c60381b8-83a4-41f8-a3d0-de05076849a9")
	mfMediaEngineOutputFormat    = parseGUID("5066893c-8cf9-42bc-8b8a-472212e52726")
)

// Vtable indexes, from mfmediaengine.h, mfobjects.h and d3d11.h.
const (
	// IMFMediaEngine
	meSetSource              = 6
	meGetCurrentTime         = 16
	meSetCurrentTime         = 17
	meGetDuration            = 19
	meIsPaused               = 20
	meSetDefaultPlaybackRate = 22
	meSetPlaybackRate        = 24
	meIsEnded                = 27
	meSetLoop                = 31
	mePlay                   = 32
	mePause                  = 33
	meSetMuted               = 35
	meSetVolume              = 37
	meGetNativeVideoSize     = 40
	meShutdown               = 42
	meTransferVideoFrame     = 43
	meOnVideoStreamTick      = 44

	// IMFMediaEngineClassFactory
	factoryCreateInstance = 3

	// IMFDXGIDeviceManager
	managerResetDevice = 7

	// IMFAttributes
	attributesSetUINT32  = 21
	attributesSetUnknown = 27

	// ID3D11Device
	d3dCreateTexture2D = 5
)

// MF_MEDIA_ENGINE_EVENT
const (
	mediaError          = 5
	mediaPause          = 9
	mediaLoadedMetadata = 10
	mediaWaiting        = 12
	mediaPlaying        = 13
	mediaCanPlay        = 14
	mediaSeeked         = 17
	mediaEnded          = 19
	mediaBufferStarted  = 1005
	mediaBufferEnded    = 1006
	mediaFirstFrame     = 1009
)

const (
	mfVersion          = 0x00020070 // MF_VERSION
	mfStartupFull      = 0
	clsctxInprocServer = 1
	d3dBindShader      = 0x8
	d3dBindTarget      = 0x20

	// videoBar is the control bar's height in points, videoButton the
	// play and pause button's width at its left, and videoTime the room
	// the time takes at its right.
	videoBar    = 40
	videoButton = 40
	videoTime   = 96
	// videoFrameWait is how long a paused video waits for the frame it
	// asked for (a seek, the first frame) before its frame client stops.
	videoFrameWait = 3 * time.Second
)

// texture2DDesc is D3D11_TEXTURE2D_DESC.
type texture2DDesc struct {
	width, height, mipLevels, arraySize, format uint32
	sampleCount, sampleQuality                  uint32
	usage, bindFlags, cpuAccess, misc           uint32
}

// video is a video view's media engine and the picture it shows.
type video struct {
	w      *window
	v      *view
	engine *comObject // IMFMediaEngine
	// serial names the source opened: notifications for another are
	// stale.
	serial uint64
	// What the engine reported so far, and whether playing was asked for,
	// applied again once the metadata is known.
	loaded, failed, buffering, playing bool

	config     driver.VideoConfig
	configured bool
	frame      *videoFrame
	natural    [2]uint32 // the picture's size in pixels
	pumping    bool
	wantFrame  time.Time // a paused video waits for a frame until then
	progressAt time.Time
	scrubbing  bool
}

// videoFrame is a frame in a texture shared with Direct2D.
type videoFrame struct {
	texture, bitmap *comObject
	width, height   uint32
	generation      uint64
	filled          bool
}

func (f *videoFrame) release() {
	if f != nil {
		releaseAll(&f.bitmap, &f.texture)
	}
}

// videoSerials numbers the sources opened, across views: node IDs are
// reused.
var videoSerials uint64

func (vd *video) report(e driver.Event) {
	if vd.v.events.Has(e.Kind) {
		e.ID = vd.v.id
		vd.w.engine.Dispatch(e)
	}
}

// configure applies c, opening its source when it changed.
func (vd *video) configure(c *driver.VideoConfig) {
	reopen := !vd.configured || c.Source != vd.config.Source
	vd.config, vd.configured = *c, true
	if reopen {
		vd.open()
	}
	vd.playing = !c.Paused
	if e := vd.engine; e != nil {
		e.call(meSetMuted, boolArg(c.Muted))
		e.callDouble(meSetVolume, float64(min(max(c.Volume, 0), 1)))
		e.call(meSetLoop, boolArg(c.Repeat))
		e.callDouble(meSetDefaultPlaybackRate, float64(c.Rate))
		e.callDouble(meSetPlaybackRate, float64(c.Rate))
		if c.Paused {
			e.call(mePause)
		} else if !vd.failed {
			e.call(mePlay)
		}
		vd.setBuffering(!c.Paused && !vd.loaded && !vd.failed)
		if !c.Paused {
			vd.pump()
		}
	}
	vd.w.redrawAll()
}

// open opens the source, or nothing.
func (vd *video) open() {
	vd.stop()
	source := vd.config.Source
	if source == "" {
		return
	}
	location, ok := videoURL(source, imageload.ResourceRoot)
	if !ok {
		vd.failed = true
		vd.report(driver.Event{Kind: driver.EventError, Text: source + ": cannot resolve the video's location"})
		return
	}
	engine, err := newMediaEngine(vd.w, vd.v.id, vd.serial)
	if err != nil {
		vd.failed = true
		vd.report(driver.Event{Kind: driver.EventError, Text: "cannot play videos: " + err.Error()})
		return
	}
	vd.report(driver.Event{Kind: driver.EventLoadStart})
	bstr, _, _ := sysAllocString.Call(uintptr(unsafe.Pointer(wide(location))))
	hr := engine.call(meSetSource, bstr)
	sysFreeString.Call(bstr)
	if failed(hr) {
		vd.failed = true
		vd.report(driver.Event{Kind: driver.EventError, Text: source + ": " + hresultError("cannot open it", hr).Error()})
	}
	vd.engine = engine
	vd.wantFrame = time.Now().Add(videoFrameWait)
}

// videoURL returns the URL Media Foundation opens source at: "asset:"
// paths under root, a URL when it has a scheme, else a file path. ok is
// false for an asset without a root.
func videoURL(source, root string) (location string, ok bool) {
	path := source
	if p, isAsset := strings.CutPrefix(source, "asset:"); isAsset {
		if root == "" {
			return "", false
		}
		path = filepath.Join(root, filepath.FromSlash(strings.TrimLeft(p, "/")))
	} else if strings.Contains(source, "://") {
		return source, true
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	return (&url.URL{Scheme: "file", Path: "/" + filepath.ToSlash(path)}).String(), true
}

// stop shuts the engine down and forgets the source; the last frame
// stays.
func (vd *video) stop() {
	videoSerials++
	vd.serial = videoSerials
	if e := vd.engine; e != nil {
		e.call(mePause)
		e.call(meShutdown)
		e.release()
		vd.engine = nil
	}
	vd.loaded, vd.failed, vd.buffering = false, false, false
	vd.natural = [2]uint32{}
	vd.wantFrame, vd.progressAt = time.Time{}, time.Time{}
}

// close lets the video go with its view.
func (vd *video) close() {
	vd.stop()
	vd.frame.release()
	vd.frame = nil
	if vd.w.barVideo == vd {
		vd.w.barVideo = nil
	}
}

// seek jumps to position.
func (vd *video) seek(position time.Duration) {
	vd.seekTo(max(position.Seconds(), 0))
}

// seekTo jumps to t seconds and waits for its frame: in frame server mode
// a seek completes as frames are asked for.
func (vd *video) seekTo(t float64) {
	if vd.engine == nil {
		return
	}
	vd.engine.callDouble(meSetCurrentTime, t)
	vd.wantFrame = time.Now().Add(videoFrameWait)
	vd.pump()
}

func (vd *video) setBuffering(buffering bool) {
	if vd.buffering != buffering {
		vd.buffering = buffering
		vd.report(driver.Event{Kind: driver.EventBuffer, Buffering: buffering})
	}
}

// notified handles a notification of the engine, on the main thread.
func (vd *video) notified(serial uint64, event uint32, param1 uintptr, param2 uint32) {
	e := vd.engine
	if serial != vd.serial || e == nil {
		return
	}
	switch event {
	case mediaLoadedMetadata:
		if vd.loaded || vd.failed {
			return
		}
		vd.loaded = true
		e.call(meGetNativeVideoSize, uintptr(unsafe.Pointer(&vd.natural[0])), uintptr(unsafe.Pointer(&vd.natural[1])))
		duration := finite(e.getDouble(meGetDuration))
		// Playing asked for before the source was ready.
		if vd.playing && uint32(e.call(meIsPaused)) != 0 {
			e.call(mePlay)
		}
		vd.setBuffering(false)
		vd.report(driver.Event{Kind: driver.EventLoad, Duration: seconds(duration),
			Size: driver.Size{Width: driver.Points(vd.natural[0]), Height: driver.Points(vd.natural[1])}})
		vd.wantFrame = time.Now().Add(videoFrameWait)
		vd.pump()
	case mediaPlaying:
		if vd.loaded {
			vd.setBuffering(false)
		}
		vd.pump()
	case mediaCanPlay, mediaBufferEnded:
		if vd.loaded {
			vd.setBuffering(false)
		}
	case mediaWaiting, mediaBufferStarted:
		if vd.playing && !vd.failed {
			vd.setBuffering(true)
		}
	case mediaFirstFrame, mediaSeeked:
		vd.wantFrame = time.Now().Add(videoFrameWait)
		vd.pump()
	case mediaPause:
		vd.w.redrawAll()
	case mediaEnded:
		if !vd.config.Repeat {
			vd.report(driver.Event{Kind: driver.EventEnd})
		}
		vd.w.redrawAll()
	case mediaError:
		if vd.failed {
			return
		}
		vd.failed = true
		vd.setBuffering(false)
		vd.report(driver.Event{Kind: driver.EventError, Text: mediaErrorText(uint16(param1), int32(param2))})
	}
}

// progress reports the position and duration, once the source loaded.
func (vd *video) progress() {
	if vd.loaded && vd.engine != nil && vd.v.events.Has(driver.EventProgress) {
		t, d := vd.times()
		vd.report(driver.Event{Kind: driver.EventProgress, Time: seconds(t), Duration: seconds(d)})
	}
}

// pump runs the frame client, once, while the video plays or waits for a
// frame.
func (vd *video) pump() {
	if vd.pumping {
		return
	}
	vd.pumping = true
	vd.w.onFrame(func(now time.Time) bool {
		keep := !vd.v.deleted && vd.tick(now)
		if !keep {
			vd.pumping = false
		}
		return keep
	})
}

// tick is one display frame while pumping: it copies a new frame and
// reports progress, and reports whether there is more to do.
func (vd *video) tick(now time.Time) bool {
	e := vd.engine
	if e == nil {
		return false
	}
	var pts int64
	// S_FALSE, no new frame, is a success code too.
	if fresh := uint32(e.call(meOnVideoStreamTick, uintptr(unsafe.Pointer(&pts)))) == 0; fresh && vd.transfer() {
		vd.wantFrame = time.Time{}
		vd.w.redrawAll()
	}
	playing := uint32(e.call(meIsPaused)) == 0 && uint32(e.call(meIsEnded)) == 0
	if playing && vd.loaded && now.Sub(vd.progressAt) >= vd.config.ProgressInterval {
		vd.progressAt = now
		vd.progress()
		// The control bar's time and scrubber move with it.
		if vd.config.Controls {
			vd.w.redrawAll()
		}
	}
	return playing || !vd.wantFrame.IsZero() && now.Before(vd.wantFrame)
}

// transfer copies the engine's current frame into the texture, made again
// for a new size or devices.
func (vd *video) transfer() bool {
	var w, h uint32
	vd.engine.call(meGetNativeVideoSize, uintptr(unsafe.Pointer(&w)), uintptr(unsafe.Pointer(&h)))
	if w == 0 || h == 0 {
		return false
	}
	vd.natural = [2]uint32{w, h}
	g, err := devices()
	if err != nil {
		return false
	}
	if _, err := deviceManager(g); err != nil {
		return false
	}
	if f := vd.frame; f == nil || f.width != w || f.height != h || f.generation != gfxGeneration {
		vd.frame.release()
		if vd.frame, err = makeVideoFrame(g, w, h); err != nil {
			return false
		}
	}
	r := rect{0, 0, int32(w), int32(h)}
	black := uint32(0xff000000) // MFARGB: blue, green, red, alpha
	if failed(vd.engine.call(meTransferVideoFrame, vd.frame.texture.ptr(), 0, uintptr(unsafe.Pointer(&r)),
		uintptr(unsafe.Pointer(&black)))) {
		return false
	}
	vd.frame.filled = true
	return true
}

// times returns the position and duration in seconds, 0 when unknown.
func (vd *video) times() (t, d float64) {
	if vd.engine == nil {
		return 0, 0
	}
	return finite(vd.engine.getDouble(meGetCurrentTime)), finite(vd.engine.getDouble(meGetDuration))
}

// playingNow reports whether the engine plays now.
func (vd *video) playingNow() bool {
	e := vd.engine
	return e != nil && uint32(e.call(meIsPaused)) == 0 && uint32(e.call(meIsEnded)) == 0
}

// barAt reports whether x, y in the view's points of a width by height
// view are on the control bar, which alone takes the pointer.
func (vd *video) barAt(x, y, width, height float32) bool {
	return vd.config.Controls && x >= 0 && x < width && y >= height-videoBar && y < height
}

// press handles a press on the bar at x: the button plays or pauses, the
// rest scrubs.
func (vd *video) press(x, width float32) {
	if e := vd.engine; e != nil && x < videoButton {
		if vd.playingNow() {
			e.call(mePause)
		} else {
			if uint32(e.call(meIsEnded)) != 0 {
				vd.seekTo(0)
			}
			e.call(mePlay)
		}
	} else {
		vd.scrubbing = true
		vd.scrub(x, width)
	}
	vd.w.redrawAll()
}

// drag follows a drag on the bar to x.
func (vd *video) drag(x, width float32) {
	if vd.scrubbing {
		vd.scrub(x, width)
		vd.w.redrawAll()
	}
}

func (vd *video) released() {
	vd.scrubbing = false
	vd.w.redrawAll()
}

// scrub seeks to where x falls on the scrubber of a bar width wide.
func (vd *video) scrub(x, width float32) {
	start, end := scrubber(width)
	_, d := vd.times()
	if vd.engine != nil && d > 0 && end > start {
		f := float64(min(max((x-start)/(end-start), 0), 1))
		vd.seekTo(f * d)
	}
}

// scrubber returns the scrubber's ends on a control bar width wide.
func scrubber(width float32) (start, end float32) { return videoButton + 8, width - videoTime - 4 }

// videoFit returns where a picture of natural size goes in a box of size,
// as x, y, width and height.
func videoFit(mode driver.ResizeMode, nw, nh, w, h float32) (x, y, fw, fh float32) {
	b := mode.Fit(driver.Size{Width: driver.Points(nw), Height: driver.Points(nh)},
		driver.Size{Width: driver.Points(w), Height: driver.Points(h)}, driver.ObjectPosition{})
	return float32(b.Origin.X), float32(b.Origin.Y), float32(b.Size.Width), float32(b.Size.Height)
}

// drawVideo paints vd's last frame in a view of width by height points,
// fitted by its resize mode, and its control bar.
func (p *painter) drawVideo(vd *video, width, height float32) {
	if f := vd.frame; f != nil && f.filled && f.generation == gfxGeneration {
		x, y, fw, fh := videoFit(vd.config.ResizeMode, float32(f.width), float32(f.height), width, height)
		d := rectF{x, y, x + fw, y + fh}
		p.dc.callF(rtDrawBitmap, 0b100, f.bitmap.ptr(), uintptr(unsafe.Pointer(&d)), f32(1), d2dInterpolationLinear, 0)
	}
	if vd.config.Controls {
		p.drawVideoBar(vd, width, height)
	}
}

// drawVideoBar paints the control bar along the bottom of a view of
// width by height points.
func (p *painter) drawVideoBar(vd *video, w, h float32) {
	if w < videoButton+24 || h < videoBar {
		return
	}
	top := h - videoBar
	p.fillRect(rectF{0, top, w, h}, driver.Color(0x0000008c))
	white := driver.RGB(0xffffff)
	cx, cy := float32(videoButton/2), top+videoBar/2
	if vd.playingNow() {
		for _, dx := range []float32{-5, 2} {
			p.fillRounded(rectF{cx + dx, cy - 7, cx + dx + 3.5, cy + 7}, 1, white)
		}
	} else if tri := p.polygon([]pointF{{cx - 5, cy - 8}, {cx + 8, cy}, {cx - 5, cy + 8}}); tri != nil {
		p.fillGeometry(tri, white)
		tri.release()
	}
	t, d := vd.times()
	if start, end := scrubber(w); end > start {
		fraction := float32(0)
		if d > 0 {
			fraction = float32(min(max(t/d, 0), 1))
		}
		knob := start + (end-start)*fraction
		p.fillRounded(rectF{start, cy - 2, end, cy + 2}, 2, driver.Color(0xffffff5a))
		p.fillRounded(rectF{start, cy - 2, max(knob, start), cy + 2}, 2, white)
		r := float32(6)
		if vd.scrubbing {
			r = 7
		}
		e := ellipse{pointF{knob, cy}, r, r}
		p.dc.call(rtFillEllipse, uintptr(unsafe.Pointer(&e)), p.solid(white).ptr())
	}
	if format := videoTimeFormat(); format != nil {
		text := utf16.Encode([]rune(clock(t) + " / " + clock(d)))
		r := rectF{w - videoTime, top, w - 10, h}
		p.dc.call(rtDrawText, uintptr(unsafe.Pointer(&text[0])), uintptr(len(text)), format.ptr(),
			uintptr(unsafe.Pointer(&r)), p.solid(white).ptr(), 0, 0)
	}
}

// videoTimeFormat is the control bar's time format, made once.
func videoTimeFormat() *comObject {
	if timeFormat == nil {
		f := textFormat(driver.Font{Size: 12, Weight: driver.FontWeightNormal})
		if f == nil {
			return nil
		}
		f.call(formatSetTextAlignment, dwAlignTrailing)
		f.call(formatSetParagraphAlignment, dwParagraphCenter)
		timeFormat = f
	}
	return timeFormat
}

var timeFormat *comObject

// clock formats a time for the control bar: m:ss, h:mm:ss from an hour
// on.
func clock(seconds float64) string {
	total := 0
	if !math.IsInf(seconds, 0) && !math.IsNaN(seconds) && seconds > 0 {
		total = int(seconds)
	}
	h, m, s := total/3600, total/60%60, total%60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

// finite is x, or 0 when it is not a finite number: live streams have no
// duration.
func finite(x float64) float64 {
	if math.IsInf(x, 0) || math.IsNaN(x) {
		return 0
	}
	return x
}

func seconds(s float64) time.Duration { return time.Duration(s * float64(time.Second)) }

// mediaErrorText is what a media engine error means, for apps.
func mediaErrorText(code uint16, hr int32) string {
	what := "The video cannot be played."
	switch code {
	case 1:
		what = "Loading was aborted."
	case 2:
		what = "A network error stopped the download."
	case 3:
		what = "The video cannot be decoded."
	case 4:
		what = "The source cannot be played (unsupported format, or not found)."
	case 5:
		what = "The video is encrypted."
	}
	if hr == 0 {
		return what
	}
	return fmt.Sprintf("%s (0x%08X)", what, uint32(hr))
}

// mfStarted reports whether Media Foundation started, which it does once
// for the process and never shuts down.
var mfStarted = func() func() error {
	var err error
	done := false
	return func() error {
		if !done {
			done = true
			if e := mfStartup.Find(); e != nil {
				err = e
			} else if hr, _, _ := mfStartup.Call(mfVersion, mfStartupFull); failed(hr) {
				err = hresultError("Media Foundation could not start", hr)
			}
		}
		return err
	}
}()

// The device manager media engines decode with: the main thread's
// Direct3D device, its reset token, and the device generation it holds.
var (
	mfManager           *comObject
	mfManagerToken      uint32
	mfManagerGeneration uint64
)

// deviceManager returns the DXGI device manager, holding the current
// device.
func deviceManager(g *gfx) (*comObject, error) {
	if mfManager == nil {
		if hr, _, _ := mfCreateDXGIDeviceManager.Call(uintptr(unsafe.Pointer(&mfManagerToken)),
			uintptr(unsafe.Pointer(&mfManager))); failed(hr) || mfManager == nil {
			mfManager = nil
			return nil, hresultError("no DXGI device manager", hr)
		}
	}
	if mfManagerGeneration != gfxGeneration {
		if hr := mfManager.call(managerResetDevice, g.d3d.ptr(), uintptr(mfManagerToken)); failed(hr) {
			return nil, hresultError("the device manager took no device", hr)
		}
		mfManagerGeneration = gfxGeneration
	}
	return mfManager, nil
}

// newMediaEngine makes a media engine reporting to w for video id, source
// serial.
func newMediaEngine(w *window, id driver.NodeID, serial uint64) (*comObject, error) {
	if err := mfStarted(); err != nil {
		return nil, err
	}
	if err := initCOM(); err != nil {
		return nil, err
	}
	g, err := devices()
	if err != nil {
		return nil, err
	}
	manager, err := deviceManager(g)
	if err != nil {
		return nil, err
	}
	// EventNotify comes on Media Foundation's threads.
	notify := newNotify(&iidMediaEngineNotify, func(event, param1, param2 uintptr) {
		w.post(func() {
			if v := w.view(id); v != nil && v.video != nil {
				v.video.notified(serial, uint32(event), param1, uint32(param2))
			}
		})
	})
	defer notify.release()
	var attributes *comObject
	if hr, _, _ := mfCreateAttributes.Call(uintptr(unsafe.Pointer(&attributes)), 3); failed(hr) || attributes == nil {
		return nil, hresultError("no media attributes", hr)
	}
	defer attributes.release()
	attributes.call(attributesSetUnknown, uintptr(unsafe.Pointer(&mfMediaEngineDXGIManager)), manager.ptr())
	attributes.call(attributesSetUnknown, uintptr(unsafe.Pointer(&mfMediaEngineCallback)), notify.ptr())
	attributes.call(attributesSetUINT32, uintptr(unsafe.Pointer(&mfMediaEngineOutputFormat)), dxgiFormatBGRA)
	var factory *comObject
	if hr, _, _ := coCreateInstance.Call(uintptr(unsafe.Pointer(&clsidMediaEngineClassFactory)), 0, clsctxInprocServer,
		uintptr(unsafe.Pointer(&iidMediaEngineClassFactory)), uintptr(unsafe.Pointer(&factory))); failed(hr) || factory == nil {
		return nil, hresultError("no media engine factory", hr)
	}
	defer factory.release()
	var engine *comObject
	if hr := factory.call(factoryCreateInstance, 0, attributes.ptr(), uintptr(unsafe.Pointer(&engine))); failed(hr) || engine == nil {
		return nil, hresultError("no media engine", hr)
	}
	return engine, nil
}

// makeVideoFrame makes a BGRA texture of width by height pixels and the
// Direct2D bitmap on it.
func makeVideoFrame(g *gfx, width, height uint32) (*videoFrame, error) {
	desc := texture2DDesc{width: width, height: height, mipLevels: 1, arraySize: 1, format: dxgiFormatBGRA,
		sampleCount: 1, bindFlags: d3dBindTarget | d3dBindShader}
	f := &videoFrame{width: width, height: height, generation: gfxGeneration}
	if hr := g.d3d.call(d3dCreateTexture2D, uintptr(unsafe.Pointer(&desc)), 0, uintptr(unsafe.Pointer(&f.texture))); failed(hr) {
		return nil, hresultError("no frame texture", hr)
	}
	surface := f.texture.query(&iidDXGISurface)
	if surface == nil {
		f.release()
		return nil, hresultError("no frame surface", eNoInterface)
	}
	defer surface.release()
	props := bitmapProperties1{format: dxgiFormatBGRA, alphaMode: d2dAlphaIgnore, dpiX: 96, dpiY: 96}
	if hr := g.dc.call(dcCreateBitmapFromDxgiSurface, surface.ptr(), uintptr(unsafe.Pointer(&props)),
		uintptr(unsafe.Pointer(&f.bitmap))); failed(hr) {
		f.bitmap = nil
		f.release()
		return nil, hresultError("no frame bitmap", hr)
	}
	return f, nil
}
