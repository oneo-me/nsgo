//go:build android

// Package android is NSGo's Android backend. The app is a plain Go
// executable that the prebuilt Java runtime in java/ starts as a child
// process; nsgo packages it as libnsgo.so in an APK. The two talk over
// the process's pipes:
// one message per commit, per event and per step of text. Java keeps the
// text the engine prepares, measures it and lays it out with StaticLayout,
// and mounts one Android view per node; measuring, and laying out text
// with attachments, are the only round trips.
// Go fetches images and Java decodes them, animated GIF and WebP
// included; Go rasterizes SVG with imagedecode first, as Android cannot.
// Java hosts web views, videos and native components' views, plays canvas
// drawings, runs style transitions, a port of core.Animator and layout
// animations on Choreographer frames, shows input accessory bars above
// the keyboard and reports the environment, back, keys, presses outside
// overlays and answers to alerts and clipboard reads.
//
// Go builds Android shared libraries only with cgo and the NDK; a second
// process keeps every build an executable, so arm64 needs only Go. An
// x86_64 emulator needs cgo and the NDK's clang, as Go links android/amd64
// no other way.
package android

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/engine"
	"github.com/oneo-me/nsgo/platform/internal/imageload"
)

// Message types, kept in sync with Protocol.java.
const (
	// Go to Java.
	msgBatch   = 1 // a MountBatch, applied on the UI thread
	msgPrepare = 2 // keeps text and its style under a new id
	msgMeasure = 3 // asks a prepared text's size at a width
	msgLayout  = 4 // lays a prepared text out under a new layout id
	msgRelease = 5 // frees a prepared text
	msgImage   = 6 // bytes of an image view's source or placeholder

	// Java to Go.
	msgStart       = 1
	msgResize      = 2
	msgEvent       = 3
	msgMeasured    = 4
	msgEnvironment = 5 // the environment, before msgStart and on change
	msgDNS         = 6 // the default network's DNS servers
	msgLaidOut     = 7 // where a msgLayout put a text's attachments
)

// The parts of an image view's request msgImage carries.
const (
	imagePlaceholder = 0
	imageSource      = 1
)

type surface struct {
	mu       sync.Mutex // guards out and buf
	out      *os.File
	buf      []byte
	ids      atomic.Uint32 // the last prepared text or layout id
	measured chan core.Size
	laidOut  chan []attachmentBox
	engine   *engine.Engine
	started  chan struct{}    // closed once engine is set
	env      core.Environment // the last reported, on the reader goroutine

	ratio atomic.Uint32 // the pixel ratio's float32 bits, for rasterizing SVG

	// On the engine goroutine.
	images   map[core.NodeID]*imageView
	requests uint32 // the last image request number

	// The native components; cmu guards them, as the reader goroutine
	// hands them their views' events.
	cmu        sync.Mutex
	components core.NativeComponents
	natives    map[core.NodeID]*nativeView
	unknown    map[string]bool // component names logged as unregistered
}

// Options configures Run.
type Options struct {
	// Components are the native components Native elements name.
	Components core.NativeComponents
}

// Run serves app to the Java runtime over standard input and output until
// the runtime closes them. Call it from main.
func Run(app engine.App, opts Options) {
	// Keep the pipe to the runtime to ourselves and send the app's own
	// prints to standard error, which the runtime logs.
	fd, err := syscall.Dup(1)
	if err != nil {
		panic(err)
	}
	if err := syscall.Dup3(2, 1, 0); err != nil {
		panic(err)
	}
	s := &surface{
		out:      os.NewFile(uintptr(fd), "nsgo-runtime"),
		measured: make(chan core.Size, 1),
		laidOut:  make(chan []attachmentBox, 1),
		started:  make(chan struct{}),
		env:      core.Environment{OS: core.OSAndroid},
		images:   map[core.NodeID]*imageView{},

		components: opts.Components,
		natives:    map[core.NodeID]*nativeView{},
		unknown:    map[string]bool{},
	}
	useDNS()
	in := bufio.NewReader(os.Stdin)
	for {
		kind, payload, err := readMessage(in)
		if err != nil {
			return // the runtime is gone
		}
		r := reader{payload}
		switch kind {
		case msgStart:
			size := core.Size{Width: r.points(), Height: r.points()}
			eopts := engine.Options{Size: size, Environment: s.env}
			// The runtime passes the app's own files and cache directories.
			eopts.SetDirs(os.Getenv("NSGO_DATA_DIR"), os.Getenv("NSGO_CACHE_DIR"))
			s.engine = engine.Start(s, app, eopts)
			close(s.started)
		case msgResize:
			s.engine.Resize(core.Size{Width: r.points(), Height: r.points()})
		case msgEvent:
			ev := core.Event{
				Kind:  core.EventKind(r.byte()),
				ID:    core.NodeID(r.u32()),
				Count: r.u32(),
				Point: core.Point{X: r.points(), Y: r.points()},
				Text:  r.str(),
				Size:  core.Size{Width: r.points(), Height: r.points()},
				Index: int(int32(r.u32())),
				// A gesture's; zero for other events.
				Phase:    core.GesturePhase(r.byte()),
				Velocity: core.Point{X: r.points(), Y: r.points()},
				Value:    r.float(),
			}
			ev.Navigation = readNavigation(&r)
			// A video's; zero for other events.
			ev.Time = time.Duration(r.u64()) * time.Millisecond
			ev.Duration = time.Duration(r.u64()) * time.Millisecond
			ev.Buffering = r.byte() != 0
			// A native view's callback's arguments, as JSON, and whether
			// an animation finished.
			data := r.str()
			ev.Finished = r.byte() != 0
			if ev.Kind == core.EventNative {
				s.nativeEvent(ev.ID, ev.Text, data)
				break
			}
			s.engine.Dispatch(ev)
		case msgMeasured:
			s.measured <- core.Size{Width: r.points(), Height: r.points()}
		case msgLaidOut:
			boxes := make([]attachmentBox, r.u32())
			for i := range boxes {
				// NaN for one the line limit or the ellipsis cut off.
				x, y := r.points(), r.points()
				boxes[i] = attachmentBox{core.Point{X: x, Y: y}, !math.IsNaN(float64(x))}
			}
			s.laidOut <- boxes
		case msgEnvironment:
			env := readEnvironment(&r)
			if env == s.env {
				break
			}
			s.env = env
			s.ratio.Store(math.Float32bits(env.PixelRatio))
			if s.engine != nil {
				s.engine.SetEnvironment(env)
			}
		case msgDNS:
			servers := make([]string, r.u32())
			for i := range servers {
				servers[i] = r.str()
			}
			dnsServers.Store(&servers)
		}
	}
}

// readNavigation reads the web view's navigation NsgoView.event writes
// last, nil for events without one.
func readNavigation(r *reader) *core.WebNavigation {
	flags := r.byte()
	if flags&1 == 0 {
		return nil
	}
	return &core.WebNavigation{
		Loading:      flags&2 != 0,
		CanGoBack:    flags&4 != 0,
		CanGoForward: flags&8 != 0,
		URL:          r.str(),
		Title:        r.str(),
	}
}

// readEnvironment reads the environment NsgoView.environment writes.
func readEnvironment(r *reader) core.Environment {
	env := core.Environment{
		OS:             core.OSAndroid,
		SafeArea:       core.Edges{Top: r.points(), Right: r.points(), Bottom: r.points(), Left: r.points()},
		KeyboardHeight: r.points(),
		FontScale:      r.float(),
		PixelRatio:     r.float(),
	}
	flags := r.byte()
	env.Dark = flags&1 != 0
	env.ReduceMotion = flags&2 != 0
	env.ScreenReader = flags&4 != 0
	env.RTL = flags&8 != 0
	env.Idiom = core.Idiom(r.byte())
	env.Locale = r.str()
	return env
}

// dnsServers are the DNS servers the runtime last reported.
var dnsServers atomic.Pointer[[]string]

// useDNS sends the app's DNS queries to the servers the runtime reports.
// Android has no resolv.conf, so Go without cgo would ask localhost.
func useDNS() {
	net.DefaultResolver = &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			servers := dnsServers.Load()
			if servers == nil || len(*servers) == 0 {
				return nil, errors.New("android: no DNS servers")
			}
			// Go tries its default servers in turn, 127.0.0.1 first:
			// map that one to our first and the rest to our second.
			i := 0
			if address != "127.0.0.1:53" {
				i = 1
			}
			var d net.Dialer
			return d.DialContext(ctx, network, net.JoinHostPort((*servers)[i%len(*servers)], "53"))
		},
	}
}

// Prepare implements core.TextEngine: Java keeps the text and its paint
// under a new id, so measuring and laying it out send only the id.
func (s *surface) Prepare(t *core.AttributedText) core.PreparedText {
	p := preparedText{s: s, id: s.ids.Add(1)}
	for _, a := range t.Attachments {
		p.attachments = append(p.attachments, a.Size)
	}
	s.send(msgPrepare, func(w *writer) {
		w.u32(p.id)
		w.str(t.Text)
		writeStyle(w, &t.Style)
		w.u32(uint32(t.MaxLines))
		w.u32(uint32(len(t.Spans)))
		for _, sp := range t.Spans {
			w.u32(uint32(sp.Start)) // UTF-8 offsets; Java converts them
			w.u32(uint32(sp.End))
			writeStyle(w, &sp.Style)
			w.u32(uint32(sp.Background.Color)) // 0: none
			w.points(sp.Background.Radius)
		}
		// Java sizes each placeholder and places it in its line as
		// core.TextAttachment.Ascent and Extent say.
		w.u32(uint32(len(t.Attachments)))
		for _, a := range t.Attachments {
			w.u32(uint32(a.Offset))
			w.points(a.Size.Width)
			w.points(a.Size.Height)
			w.byte(byte(a.Align))
		}
	})
	return p
}

// preparedText is text Java keeps under id until Release, and the sizes
// of its attachments.
type preparedText struct {
	s           *surface
	id          uint32
	attachments []core.Size
}

// Measure asks the runtime's StaticLayout and waits for the answer. Only
// the engine goroutine measures, so one request is in flight at a time.
func (p preparedText) Measure(maxWidth core.Points) core.Size {
	p.s.send(msgMeasure, func(w *writer) {
		w.u32(p.id)
		w.points(maxWidth) // Auto, a NaN, does not wrap
	})
	return <-p.s.measured
}

// Layout has Java build the final StaticLayout under a new id for an
// ItemText to name: Java reads it before the batch. Only a text with
// attachments waits for an answer, where they went.
func (p preparedText) Layout(size core.Size) core.TextLayout {
	l := &textLayout{id: p.s.ids.Add(1)}
	p.s.send(msgLayout, func(w *writer) {
		w.u32(p.id)
		w.u32(l.id)
		w.points(size.Width) // the height is the one measured
	})
	if len(p.attachments) > 0 {
		l.attachments = <-p.s.laidOut
		l.sizes = p.attachments
	}
	return l
}

func (p preparedText) Release() {
	p.s.send(msgRelease, func(w *writer) { w.u32(p.id) })
}

// textLayout names a StaticLayout Java built, and where it put the
// text's attachments. The runtime drops the layout itself when a later
// ItemText replaces it or its view is deleted.
type textLayout struct {
	id          uint32
	attachments []attachmentBox
	sizes       []core.Size
}

// attachmentBox is the top left of an attachment in the content box.
type attachmentBox struct {
	origin core.Point
	shown  bool
}

func (*textLayout) Release() {}

// Attachment implements core.AttachmentLayout.
func (l *textLayout) Attachment(i int) (core.Bounds, bool) {
	if i < 0 || i >= len(l.attachments) || i >= len(l.sizes) || !l.attachments[i].shown {
		return core.Bounds{}, false
	}
	return core.Bounds{Origin: l.attachments[i].origin, Size: l.sizes[i]}, true
}

// Mount implements engine.Platform: one message per commit, then the
// loads of the image sources it changed.
// Animate implements engine.Platform. This backend has no display link yet:
// apps drive their own frames with a timer, which costs the wakeups a
// platform-driven frame would not. See the macOS backend for the shape.
func (s *surface) Animate(on bool) {}

func (s *surface) Mount(b *core.MountBatch) {
	requests, loads := s.trackImages(b)
	natives := s.trackNatives(b)
	s.send(msgBatch, func(w *writer) { encode(w, b, requests, natives) })
	for _, l := range loads {
		go s.load(l)
	}
}

// imageView is what Go keeps of an image view: the sources of its
// current request and how to stop loading them.
type imageView struct {
	source, placeholder core.ImageSource
	events              core.EventMask
	request             uint32
	cancel              context.CancelFunc
	size                core.Size // the view's, which an SVG is drawn for
}

func (v *imageView) stop() {
	if v.cancel != nil {
		v.cancel()
	}
}

// imageLoad fetches the sources of one request of an image view.
type imageLoad struct {
	ctx                 context.Context
	id                  core.NodeID
	request             uint32
	source, placeholder core.ImageSource
	events              core.EventMask
	size                core.Size
}

// trackImages follows the image views of b. A view whose sources change
// gets a new request number, which its ItemImage carries and its bytes
// name, so Java drops the bytes of requests it has moved past. It returns
// the request of each ItemImage in b, in order, and the loads to start.
func (s *surface) trackImages(b *core.MountBatch) ([]uint32, []imageLoad) {
	var requests []uint32
	var loads []imageLoad
	for i := range b.Items {
		it := &b.Items[i]
		v := s.images[it.ID]
		switch it.Kind {
		case core.ItemCreate, core.ItemDelete:
			if v != nil {
				v.stop()
				delete(s.images, it.ID)
			}
			if it.Kind == core.ItemCreate && it.View == core.KindImage {
				s.images[it.ID] = &imageView{}
			}
		case core.ItemEvents:
			if v != nil {
				v.events = it.Events
			}
		case core.ItemFrame:
			if v != nil {
				v.size = it.Frame.Size
			}
		case core.ItemImage:
			if v == nil {
				v = &imageView{}
				s.images[it.ID] = v
			}
			c := it.Image
			if v.request == 0 || !sameSource(v.source, c.Source) || !sameSource(v.placeholder, c.Placeholder) {
				v.stop()
				s.requests++
				ctx, cancel := context.WithCancel(context.Background())
				v.source, v.placeholder, v.request, v.cancel = c.Source, c.Placeholder, s.requests, cancel
				loads = append(loads, imageLoad{ctx, it.ID, v.request, c.Source, c.Placeholder, v.events, v.size})
			}
			requests = append(requests, v.request)
		}
	}
	// A frame may follow its view's ItemImage.
	for i := range loads {
		if v := s.images[loads[i].id]; v != nil {
			loads[i].size = v.size
		}
	}
	return requests, loads
}

func sameSource(a, b core.ImageSource) bool {
	return a.URI == b.URI && len(a.Data) == len(b.Data) && (len(a.Data) == 0 || &a.Data[0] == &b.Data[0])
}

func isEmpty(src core.ImageSource) bool { return src.URI == "" && len(src.Data) == 0 }

// load fetches l's placeholder and source side by side and hands their
// bytes to Java, which decodes them and reports EventLoad, or EventError
// for bytes that are no image.
func (s *surface) load(l imageLoad) {
	if !isEmpty(l.placeholder) {
		go func() {
			if data, err := imageload.Load(l.ctx, l.placeholder, nil); err == nil && l.ctx.Err() == nil {
				if img, err := s.prepareImage(data, l.placeholder.URI, l.size); err == nil {
					s.sendImage(l, imagePlaceholder, img)
				}
			}
		}()
	}
	if isEmpty(l.source) {
		return
	}
	s.report(l, core.Event{Kind: core.EventLoadStart})
	var last time.Time
	data, err := imageload.Load(l.ctx, l.source, func(loaded, total int64) {
		if now := time.Now(); l.ctx.Err() == nil && now.Sub(last) >= 100*time.Millisecond {
			last = now
			s.report(l, core.Event{Kind: core.EventProgress, Loaded: loaded, Total: total})
		}
	})
	switch {
	case l.ctx.Err() != nil:
		// The view is gone or shows other sources.
	case err != nil:
		s.report(l, core.Event{Kind: core.EventError, Text: err.Error()})
		s.report(l, core.Event{Kind: core.EventLoadEnd})
	default:
		img, err := s.prepareImage(data, l.source.URI, l.size)
		if err != nil {
			s.report(l, core.Event{Kind: core.EventError, Text: err.Error()})
			s.report(l, core.Event{Kind: core.EventLoadEnd})
			return
		}
		s.sendImage(l, imageSource, img)
	}
}

// report dispatches ev about l's view if the view reports its kind.
func (s *surface) report(l imageLoad, ev core.Event) {
	if l.events.Has(ev.Kind) {
		<-s.started
		ev.ID = l.id
		s.engine.Dispatch(ev)
	}
}

func (s *surface) sendImage(l imageLoad, part byte, img decodedImage) {
	s.send(msgImage, func(w *writer) {
		w.u32(uint32(l.id))
		w.u32(l.request)
		w.byte(part)
		w.float(img.scale)
		w.points(img.size.Width) // 0: the bitmap's own pixels
		w.points(img.size.Height)
		w.u32(uint32(len(img.data)))
		w.buf = append(w.buf, img.data...)
	})
}

// scaleSuffix matches the "@2x" core names an asset variant with.
var scaleSuffix = regexp.MustCompile(`@(\d+(?:\.\d+)?)x(?:\.\w+)?$`)

// imageScale is the pixels per point a file name declares, as "@2x".
func imageScale(uri string) float32 {
	if strings.HasPrefix(uri, "data:") {
		return 1
	}
	m := scaleSuffix.FindStringSubmatch(uri)
	if m == nil {
		return 1
	}
	f, err := strconv.ParseFloat(m[1], 32)
	if err != nil || f <= 0 {
		return 1
	}
	return float32(f)
}

// send writes one message whose payload body writes.
func (s *surface) send(kind byte, body func(w *writer)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w := writer{append(s.buf[:0], kind, 0, 0, 0, 0)}
	body(&w)
	binary.BigEndian.PutUint32(w.buf[1:], uint32(len(w.buf)-5))
	s.out.Write(w.buf)
	if cap(w.buf) <= 1<<20 { // let an image's buffer go
		s.buf = w.buf
	}
}

func readMessage(r *bufio.Reader) (byte, []byte, error) {
	var head [5]byte
	if _, err := io.ReadFull(r, head[:]); err != nil {
		return 0, nil, err
	}
	payload := make([]byte, binary.BigEndian.Uint32(head[1:]))
	if _, err := io.ReadFull(r, payload); err != nil {
		return 0, nil, err
	}
	return head[0], payload, nil
}

// webInterface is the JavaScript interface WebNode adds for pages to post
// through, as WebNode.INTERFACE: window.nsgo is there from a page's first
// script, as NSH's window.nsh is.
const webInterface = "nsgo"

// webBridge wraps the interface once the script before content runs, so
// pages post any value as a string, as core.WebBridge's do; the Java
// bridge turns objects into "undefined".
const webBridge = "(function (n) { if (n) window." + webInterface +
	" = { postMessage: function (data) { n.postMessage(String(data)); } }; })(window." + webInterface + ");"

// encode writes a batch in the format NsgoView.mount reads: each item is
// its kind, its ID and the fields of that kind; requests numbers its
// ItemImage items in order, and natives says what its ItemNative items
// send. Kinds it does not know, ItemBackUnhandled and the desktop-only
// ItemWindow and ItemMenu, which Java ignores, have no fields. The batch's
// layout animation comes before its items.
func encode(w *writer, b *core.MountBatch, requests []uint32, natives []nativeItem) {
	w.u32(uint32(b.Root))
	writeLayoutAnimation(w, b.LayoutAnimation)
	w.u32(uint32(len(b.Items)))
	for i := range b.Items {
		it := &b.Items[i]
		w.byte(byte(it.Kind))
		w.u32(uint32(it.ID))
		switch it.Kind {
		case core.ItemCreate:
			w.byte(byte(it.View))
		case core.ItemInsert:
			w.u32(uint32(it.Parent))
			w.u32(uint32(it.Before))
		case core.ItemFrame:
			w.bounds(it.Frame)
		case core.ItemViewStyle:
			v := it.ViewStyle
			w.u32(uint32(v.Background))
			w.points(v.CornerRadius)
			w.points(v.BorderWidth)
			w.u32(uint32(v.BorderColor))
			// Each side's border, all 0 unless they differ.
			b, c := v.Borders, v.BorderColors
			w.points(b.Top)
			w.points(b.Right)
			w.points(b.Bottom)
			w.points(b.Left)
			w.u32(uint32(c.Top))
			w.u32(uint32(c.Right))
			w.u32(uint32(c.Bottom))
			w.u32(uint32(c.Left))
			w.float(v.Opacity)
			// Android draws shadows from elevation, which has no offset.
			w.u32(uint32(v.ShadowColor))
			w.points(v.ShadowRadius)
			w.u32(uint32(v.TintColor))
			// The scales as they are, 0 unset, for Java to combine with
			// animated ones as core.Transform.Scales does.
			tr := v.Transform
			w.points(tr.TranslateX)
			w.points(tr.TranslateY)
			w.floats(tr.Scale, tr.ScaleX, tr.ScaleY)
			w.float(tr.Rotate)
			tn := v.Transition
			w.u32(uint32(max(tn.Duration.Milliseconds(), 0)))
			w.u32(uint32(max(tn.Delay.Milliseconds(), 0)))
			w.float(tn.Easing.X1)
			w.float(tn.Easing.Y1)
			w.float(tn.Easing.X2)
			w.float(tn.Easing.Y2)
			w.byte(byte(tn.Properties))
			w.flags(v.Clip, v.Hidden)
		case core.ItemText:
			t := it.Text
			var layout uint32
			if l, ok := t.Layout.(*textLayout); ok {
				layout = l.id
			}
			w.str(t.Text)
			w.u32(layout)
			w.points(t.Content.Origin.X)
			w.points(t.Content.Origin.Y)
		case core.ItemInput:
			in := it.Input
			writeStyle(w, &in.Style)
			w.str(in.Placeholder)
			w.byte(byte(in.KeyboardType))
			w.flags(in.Masked, in.Readonly, in.Disabled)
			w.bounds(in.Content)
			w.u32(uint32(in.Accessory))
		case core.ItemInputValue:
			w.str(it.Value)
			w.u32(it.Count)
		case core.ItemEvents:
			w.u64(uint64(it.Events))
		case core.ItemViewProps:
			p := it.ViewProps
			w.byte(byte(p.Role))
			w.flags(p.Disabled, p.Selected, p.Checked, p.Indeterminate)
			w.str(p.Label)
			w.str(p.TestID)
			// Java decides when a drag activates the pan.
			a := p.PanActivation
			w.points(a.ActiveOffsetX)
			w.points(a.ActiveOffsetY)
			w.points(a.FailOffsetX)
			w.points(a.FailOffsetY)
		case core.ItemScrollConfig:
			w.flags(it.Scroll.Horizontal, it.Scroll.HideIndicator)
		case core.ItemScrollContent:
			w.points(it.Size.Width)
			w.points(it.Size.Height)
		case core.ItemImage:
			c := it.Image
			w.u32(requests[0])
			requests = requests[1:]
			w.byte(byte(c.ResizeMode - core.Cover)) // Java's are 0-based
			w.points(c.BlurRadius)
			w.u32(uint32(max(c.FadeDuration.Milliseconds(), 0))) // 0: the platform's
		case core.ItemModal:
			w.byte(byte(it.Modal.Animation))
		case core.ItemAlert:
			a := it.Alert
			w.u32(a.ID)
			w.str(a.Title)
			w.str(a.Message)
			w.u32(uint32(len(a.Actions)))
			for _, act := range a.Actions {
				w.str(act.Label)
				w.byte(byte(act.Role))
			}
		case core.ItemScrollTo:
			w.points(it.Point.X)
			w.points(it.Point.Y)
			w.flags(it.Animated)
		case core.ItemCanvas:
			// Length first: Java keeps the ops as they are and plays them
			// on each draw.
			at := len(w.buf)
			w.u32(0)
			writeDrawing(w, it.Drawing)
			binary.BigEndian.PutUint32(w.buf[at:], uint32(len(w.buf)-at-4))
		case core.ItemSetClipboard, core.ItemAnnounce, core.ItemTitle:
			w.str(it.Value)
		case core.ItemReadClipboard:
			w.u32(it.Count)
		case core.ItemKeymap:
			w.u32(uint32(len(it.Keys)))
			for _, k := range it.Keys {
				w.str(k)
			}
		case core.ItemStatusBar:
			w.byte(byte(it.Count)) // a StatusBarStyle
			w.flags(it.Animated)   // hidden
		case core.ItemWeb:
			c := it.Web
			w.str(c.Source.Src)
			w.str(c.Source.HTML)
			w.str(c.Source.BaseURL)
			w.str(c.InjectedJavaScript)
			// The bridge comes first, so the app's script can post.
			w.str(webBridge + "\n" + c.InjectedJavaScriptBeforeContent)
			w.str(c.UserAgent)
			w.flags(c.NoScroll, c.NoJavaScript) // Android has no back and forward swipes
		case core.ItemVideo:
			c := it.Video
			w.str(c.Source)
			w.flags(c.Paused, c.Muted, c.Repeat, c.Controls)
			w.float(c.Volume)
			w.float(c.Rate)
			w.byte(byte(c.ResizeMode - core.Cover)) // Java's are 0-based
			w.u32(uint32(c.ProgressInterval.Milliseconds()))
		case core.ItemVideoSeek:
			w.u64(uint64(it.Time.Milliseconds()))
		case core.ItemNative:
			n := natives[0]
			natives = natives[1:]
			w.str(n.class)
			w.u32(uint32(len(n.props)))
			for _, p := range n.props {
				w.str(p.Name)
				w.str(jsonValue(p.Value))
			}
		case core.ItemNativeCommand:
			w.str(it.Value)
			w.str(jsonValue(it.Data))
		case core.ItemAnimation:
			writeAnimation(w, it.Animation)
		case core.ItemWebCommand:
			w.byte(byte(it.WebCommand))
			if it.WebCommand == core.WebPostMessage {
				w.str(core.WebMessageScript(it.Value))
			} else {
				w.str(it.Value)
			}
		}
	}
}

// writeDrawing writes each op as its kind and the fields it uses, in the
// format CanvasNode plays.
func writeDrawing(w *writer, d *core.Drawing) {
	if d == nil {
		return
	}
	for i := range d.Ops {
		op := &d.Ops[i]
		w.byte(byte(op.Kind))
		switch op.Kind {
		case core.DrawTranslate, core.DrawScale, core.DrawMoveTo, core.DrawLineTo:
			w.floats(op.X, op.Y)
		case core.DrawRotate, core.DrawAlpha:
			w.float(op.A)
		case core.DrawQuadTo:
			w.floats(op.X1, op.Y1, op.X, op.Y)
		case core.DrawCubicTo:
			w.floats(op.X1, op.Y1, op.X2, op.Y2, op.X, op.Y)
		case core.DrawArc:
			w.floats(op.X, op.Y, op.R, op.A0, op.A1)
			w.flags(op.CCW)
		case core.DrawRect, core.DrawClear:
			w.floats(op.X, op.Y, op.W, op.H)
		case core.DrawRoundRect:
			w.floats(op.X, op.Y, op.W, op.H, op.R)
		case core.DrawEllipse:
			w.floats(op.X, op.Y, op.R, op.RY)
		case core.DrawFill, core.DrawClip:
			w.flags(op.EvenOdd)
		case core.DrawFillColor, core.DrawStrokeColor:
			w.u32(uint32(op.Color))
		case core.DrawFillGradient, core.DrawStrokeGradient:
			writeGradient(w, op.Gradient)
		case core.DrawLineWidth:
			w.float(op.W)
		case core.DrawLineCap:
			w.byte(byte(op.Cap))
		case core.DrawLineJoin:
			w.byte(byte(op.Join))
		case core.DrawFillText:
			w.str(op.Text)
			w.floats(op.X, op.Y)
			w.str(op.Font.Family)
			w.points(op.Font.Size)
			w.u16(uint16(op.Font.Weight))
			w.flags(op.Font.Italic)
		}
	}
}

// writeStyle writes a text style in the format Text.Style reads.
func writeStyle(w *writer, t *core.TextStyle) {
	w.str(t.Font.Family)
	w.points(t.Font.Size)
	w.u16(uint16(t.Font.Weight))
	w.u32(uint32(t.Color))
	w.points(t.LineHeight)
	w.points(t.LetterSpacing)
	w.byte(byte(t.Align))
	w.flags(t.Font.Italic,
		t.Decoration&core.TextDecorationUnderline != 0,
		t.Decoration&core.TextDecorationLineThrough != 0)
}
