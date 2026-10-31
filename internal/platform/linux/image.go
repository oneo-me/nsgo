//go:build linux && !android

package linux

import (
	"context"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/platform/imagedecode"
	"github.com/oneo-me/nsgo/internal/platform/imageload"
)

// progressInterval spaces an image's progress reports.
const progressInterval = 100 * time.Millisecond

// picture is a decoded image: premultiplied BGRA rows, top to bottom, of
// the frame it shows.
type picture struct {
	width, height int
	pix           []byte
	// An animated GIF or WebP's frames, which take turns in pix, and how
	// many times it plays, 0 for ever; nil for a still image.
	frames []imagedecode.Frame
	plays  int
	// A vector picture is an SVG drawn oversampled; size is its intrinsic
	// size in points.
	vector bool
	size   driver.Size
}

// imageSlot is an image view's source or placeholder and what the window
// keeps for it.
type imageSlot struct {
	src     driver.ImageSource
	load    *imageLoad
	pic     *picture
	surface uintptr // pic as a Cairo surface, tinted with tint
	tint    driver.Color
	// An animation's frame showing, since when, and how many times it
	// played through.
	frame   int
	frameAt time.Time
	played  int
	// A source that loaded fades in over fade from fadeAt, over the
	// placeholder.
	fadeAt time.Time
	fade   time.Duration
}

type imageLoad struct{ cancel context.CancelFunc }

// alpha returns the opacity s's fade-in shows it at by now: 1 once over.
func (s *imageSlot) alpha(now time.Time) float64 {
	if s.fade <= 0 {
		return 1
	}
	return min(float64(now.Sub(s.fadeAt))/float64(s.fade), 1)
}

func (s *imageSlot) empty() bool { return s.src.URI == "" && len(s.src.Data) == 0 }

// reset stops the slot's load and frees its picture, for src.
func (s *imageSlot) reset(src driver.ImageSource) {
	if s.load != nil {
		s.load.cancel()
		s.load = nil
	}
	if s.surface != 0 {
		cairoSurfaceDestroy(s.surface)
		s.surface = 0
	}
	s.pic = nil
	s.src = src
	s.frame, s.played, s.frameAt = 0, 0, time.Time{}
	s.fade = 0
}

func sameSource(a, b driver.ImageSource) bool {
	return a.URI == b.URI && a.Size == b.Size && len(a.Data) == len(b.Data) &&
		(len(a.Data) == 0 || &a.Data[0] == &b.Data[0])
}

// setImage applies an image view's configuration and reports whether a
// source changed, to load once the batch is applied.
func (w *window) setImage(v *view, c *driver.ImageConfig) bool {
	v.image = *c
	changed := false
	if !sameSource(v.source.src, c.Source) {
		v.source.reset(c.Source)
		changed = true
	}
	if !sameSource(v.placeholder.src, c.Placeholder) {
		v.placeholder.reset(c.Placeholder)
		changed = true
	}
	return changed
}

// load fetches and decodes s's source on a goroutine of its own and hands
// the picture to the window thread. Only the source, not the placeholder,
// reports its loading.
func (w *window) load(v *view, s *imageSlot, report bool) {
	if s.load != nil || s.pic != nil || s.empty() {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	ld := &imageLoad{cancel: cancel}
	s.load = ld
	if report {
		w.report(v, driver.Event{Kind: driver.EventLoadStart, ID: v.id})
	}
	var progress imageload.Progress
	if report && v.events.Has(driver.EventProgress) {
		var last time.Time
		progress = func(loaded, total int64) {
			if now := time.Now(); now.Sub(last) >= progressInterval || loaded == total {
				last = now
				w.post(func() {
					if s.load == ld {
						w.report(v, driver.Event{Kind: driver.EventProgress, ID: v.id, Loaded: loaded, Total: total})
					}
				})
			}
		}
	}
	src := s.src
	go func() {
		data, err := imageload.Load(ctx, src, progress)
		var pic *picture
		if err == nil {
			pic, err = decode(ctx, data)
		}
		if ctx.Err() != nil {
			return
		}
		w.post(func() {
			if s.load != ld {
				return
			}
			s.load = nil
			cancel()
			if err != nil {
				if report {
					w.report(v, driver.Event{Kind: driver.EventError, ID: v.id, Text: err.Error()})
					w.report(v, driver.Event{Kind: driver.EventLoadEnd, ID: v.id})
				}
				return
			}
			s.pic = pic
			if report {
				// The source's size: an SVG's in whole points, a bitmap's
				// pixels.
				size := driver.Size{Width: driver.Points(pic.width), Height: driver.Points(pic.height)}
				if pic.vector {
					size = driver.Size{Width: driver.Points(math.Round(float64(pic.size.Width))),
						Height: driver.Points(math.Round(float64(pic.size.Height)))}
				}
				w.report(v, driver.Event{Kind: driver.EventLoad, ID: v.id, Size: size})
				w.report(v, driver.Event{Kind: driver.EventLoadEnd, ID: v.id})
				if fade := v.image.FadeDuration; fade > 0 {
					s.fadeAt, s.fade = time.Now(), fade
					w.fading = append(w.fading, v)
					w.startClock()
				}
			}
			w.playImage(v)
			w.changed(v)
		})
	}()
}

// report dispatches ev when v still exists and its events hold ev's kind.
func (w *window) report(v *view, ev driver.Event) {
	if !v.deleted && v.events.Has(ev.Kind) {
		w.engine.Dispatch(ev)
	}
}

// decode decodes a PNG, JPEG, GIF, WebP or SVG into premultiplied BGRA,
// as Cairo's ARGB32 has it on little-endian machines. An SVG is drawn at
// 4 pixels a point, at most 2048 a side, as NSH's Linux backend draws
// it, so it stays sharp when scaled up. It decodes in one of
// imagedecode's slots and returns ctx's error if ctx ends first.
func decode(ctx context.Context, data []byte) (*picture, error) {
	done, err := imagedecode.Wait(ctx)
	if err != nil {
		return nil, err
	}
	defer done()
	img, err := imagedecode.Decode(data, imagedecode.Options{Layout: imagedecode.BGRA})
	if err != nil {
		return nil, err
	}
	pic := &picture{width: img.Width, height: img.Height, pix: img.Frames[0].Pix,
		vector: img.Format == imagedecode.SVG, size: img.Size}
	if len(img.Frames) > 1 {
		pic.frames, pic.plays = img.Frames, img.Plays
	}
	return pic, nil
}

// playImage plays v's animated pictures on the frame clock, unless v is
// tinted: a tinted animation shows its first frame, as NSH's does.
func (w *window) playImage(v *view) {
	if v.deleted || w.styleOf(v).TintColor&0xff != 0 {
		return
	}
	for _, s := range []*imageSlot{&v.source, &v.placeholder} {
		if s.pic != nil && s.pic.frames != nil && !s.finished() {
			if s.frameAt.IsZero() {
				s.frameAt = time.Now()
			}
			if !slices.Contains(w.playing, v) {
				w.playing = append(w.playing, v)
			}
			w.startClock()
		}
	}
}

// finished reports whether s's animation played as many times as it
// plays.
func (s *imageSlot) finished() bool {
	return s.pic.plays > 0 && s.played >= s.pic.plays
}

// stepImages shows the frames of the playing animations due by now, and
// reports whether any still plays.
func (w *window) stepImages(now time.Time) bool {
	live := w.playing[:0]
	for _, v := range w.playing {
		if v.deleted {
			continue
		}
		tinted := w.styleOf(v).TintColor&0xff != 0
		playing := false
		for _, s := range []*imageSlot{&v.source, &v.placeholder} {
			if s.pic == nil || s.pic.frames == nil || s.finished() {
				continue
			}
			if tinted {
				if s.frame != 0 {
					s.show(0)
					w.changed(v)
				}
				s.frameAt = time.Time{}
				continue
			}
			if s.step(now) {
				w.changed(v)
			}
			playing = playing || !s.finished()
		}
		if playing {
			live = append(live, v)
		}
	}
	clear(w.playing[len(live):])
	w.playing = live
	// Fading sources draw again each frame until they are opaque.
	fading := w.fading[:0]
	for _, v := range w.fading {
		if v.deleted || v.source.fade <= 0 {
			continue
		}
		w.changed(v)
		if v.source.alpha(now) >= 1 {
			v.source.fade = 0
			continue
		}
		fading = append(fading, v)
	}
	clear(w.fading[len(fading):])
	w.fading = fading
	return len(live) > 0 || len(fading) > 0
}

// step moves s's animation to the frame due at now, and reports whether
// it shows another one.
func (s *imageSlot) step(now time.Time) bool {
	frames := s.pic.frames
	at := s.frame
	// A frame shows for its delay from when the last one was due, so the
	// animation keeps its pace; one far behind starts again from now.
	for range len(frames) + 1 {
		due := s.frameAt.Add(frames[at].Delay)
		if now.Before(due) {
			break
		}
		if at == len(frames)-1 {
			s.played++
			if s.finished() {
				break // it stays on its last frame
			}
			at = 0
		} else {
			at++
		}
		s.frameAt = due
	}
	if now.Sub(s.frameAt) > time.Second {
		s.frameAt = now
	}
	if at == s.frame {
		return false
	}
	s.show(at)
	return true
}

// show makes frame i the one s shows.
func (s *imageSlot) show(i int) {
	s.frame = i
	s.pic.pix = s.pic.frames[i].Pix
	if s.surface != 0 {
		cairoSurfaceDestroy(s.surface)
		s.surface = 0
	}
}

// surfaceOf returns s's picture as a Cairo image surface with tint
// applied, made once per frame and tint; 0 until the picture arrives.
func (s *imageSlot) surfaceOf(tint driver.Color) uintptr {
	if s.pic == nil {
		return 0
	}
	if s.surface != 0 {
		if s.tint == tint {
			return s.surface
		}
		cairoSurfaceDestroy(s.surface)
		s.surface = 0
	}
	surf := cairoImageSurfaceCreate(0, int32(s.pic.width), int32(s.pic.height))
	if surf == 0 {
		return 0
	}
	data := cairoImageSurfaceGetData(surf)
	stride := int(cairoImageSurfaceGetStride(surf))
	rows := unsafe.Slice((*byte)(cPointer(data)), stride*s.pic.height)
	row := make([]byte, s.pic.width*4)
	for y := 0; y < s.pic.height; y++ {
		src := s.pic.pix[y*s.pic.width*4 : (y+1)*s.pic.width*4]
		if tint&0xff == 0 {
			copy(rows[y*stride:y*stride+len(src)], src)
			continue
		}
		tintInto(row, src, tint)
		copy(rows[y*stride:y*stride+len(row)], row)
	}
	cairoSurfaceMarkDirty(surf)
	s.surface, s.tint = surf, tint
	return surf
}

// tintInto copies premultiplied pixels from src to dst with their colors
// replaced by tint's, keeping their alpha.
func tintInto(dst, src []byte, tint driver.Color) {
	r, g, b := uint32(tint>>24&0xff), uint32(tint>>16&0xff), uint32(tint>>8&0xff)
	for i := 0; i+3 < len(src); i += 4 {
		a := uint32(src[i+3])
		dst[i], dst[i+1], dst[i+2], dst[i+3] = byte(b*a/255), byte(g*a/255), byte(r*a/255), byte(a)
	}
}

// snapshotImage draws image view v in a box of width by height: its
// placeholder until the source arrives or while the source fades in over
// it, then the source, each fitted by the view's resize mode and position
// and blurred by its blur radius, as NSH's Linux backend draws them.
func (w *window) snapshotImage(v *view, tint driver.Color, snap uintptr, width, height float64) {
	box := rect{w: width, h: height}
	now := time.Now()
	alpha := 1.0
	if v.source.pic != nil {
		alpha = v.source.alpha(now)
	}
	if v.source.pic == nil || alpha < 1 {
		w.snapshotPicture(v, &v.placeholder, tint, snap, box)
	}
	if v.source.pic == nil || alpha <= 0 {
		return
	}
	if alpha < 1 {
		gtkSnapshotPushOpacity(snap, alpha)
	}
	w.snapshotPicture(v, &v.source, tint, snap, box)
	if alpha < 1 {
		gtkSnapshotPop(snap)
	}
}

// snapshotPicture draws s's picture fitted in box. A blurred one is drawn
// enlarged under a clip to its own rectangle, so the blur's transparent
// fringe falls outside and its edges stay opaque.
func (w *window) snapshotPicture(v *view, s *imageSlot, tint driver.Color, snap uintptr, box rect) {
	surf := s.surfaceOf(tint)
	if surf == 0 {
		return
	}
	dst := w.fit(v, s, box)
	if dst.w <= 0 || dst.h <= 0 {
		return
	}
	blur := max(float64(v.image.BlurRadius), 0)
	if blur > 0 {
		x0, y0 := max(dst.x, box.x), max(dst.y, box.y)
		x1, y1 := min(dst.x+dst.w, box.x+box.w), min(dst.y+dst.h, box.y+box.h)
		if x1 <= x0 || y1 <= y0 {
			return
		}
		clip := grapheneRect{float32(x0), float32(y0), float32(x1 - x0), float32(y1 - y0)}
		gtkSnapshotPushClip(snap, &clip)
		gtkSnapshotPushBlur(snap, blur)
		grow := 2 * blur
		dst = rect{x: dst.x - grow, y: dst.y - grow, w: dst.w + 2*grow, h: dst.h + 2*grow}
	}
	bounds := grapheneRect{float32(dst.x), float32(dst.y), float32(dst.w), float32(dst.h)}
	cr := gtkSnapshotAppendCairo(snap, &bounds)
	paintPicture(cr, s, surf, dst)
	cairoDestroy(cr)
	if blur > 0 {
		gtkSnapshotPop(snap)
		gtkSnapshotPop(snap)
	}
}

// drawImage paints v's source, or its placeholder until the source
// arrives, into r as v's resize mode and position fit it, unblurred.
func (p *painter) drawImage(v *view, tint driver.Color, r rect) {
	s := &v.source
	if s.pic == nil {
		s = &v.placeholder
	}
	surf := s.surfaceOf(tint)
	if surf == 0 || r.w <= 0 || r.h <= 0 {
		return
	}
	cairoSave(p.cr)
	cairoRectangle(p.cr, r.x, r.y, r.w, r.h)
	cairoClip(p.cr)
	paintPicture(p.cr, s, surf, p.w.fit(v, s, r))
	cairoRestore(p.cr)
}

// paintPicture paints surf, s's picture, scaled into dst.
func paintPicture(cr uintptr, s *imageSlot, surf uintptr, dst rect) {
	if dst.w <= 0 || dst.h <= 0 {
		return
	}
	cairoSave(cr)
	cairoTranslate(cr, dst.x, dst.y)
	cairoScale(cr, dst.w/float64(s.pic.width), dst.h/float64(s.pic.height))
	cairoSetSourceSurface(cr, surf, 0, 0)
	cairoPaint(cr)
	cairoRestore(cr)
}

// fit returns where s's picture goes in r: core's fit of its natural
// size, by v's resize mode and position.
func (w *window) fit(v *view, s *imageSlot, r rect) rect {
	b := v.image.ResizeMode.Fit(natural(s), driver.Size{Width: driver.Points(r.w), Height: driver.Points(r.h)}, v.image.Position)
	return rect{x: r.x + float64(b.Origin.X), y: r.y + float64(b.Origin.Y),
		w: float64(b.Size.Width), h: float64(b.Size.Height)}
}

// natural returns the size in points s's picture shows at: the size its
// source declares, an SVG's intrinsic size, or its pixels over the
// density its URI's "@2x" declares.
func natural(s *imageSlot) driver.Size {
	if size := s.src.Size; size.Width > 0 && size.Height > 0 {
		return size
	}
	if s.pic.vector {
		return s.pic.size
	}
	d := density(s.src.URI)
	return driver.Size{Width: driver.Points(float64(s.pic.width) / d), Height: driver.Points(float64(s.pic.height) / d)}
}

// imageInfo answers an ItemImageInfo off the main thread.
func (w *window) imageInfo(it *driver.MountItem) {
	item := *it
	go func() { w.engine.Dispatch(imageload.Info(&item, imagedecode.Size)) }()
}

// density returns the N of a "name@Nx.png" URI, else 1.
func density(uri string) float64 {
	name := uri[strings.LastIndexAny(uri, `/\:`)+1:]
	at := strings.LastIndexByte(name, '@')
	if at < 0 {
		return 1
	}
	n, _, ok := strings.Cut(name[at+1:], "x")
	f, err := strconv.ParseFloat(n, 64)
	if !ok || err != nil || f <= 0 {
		return 1
	}
	return f
}
