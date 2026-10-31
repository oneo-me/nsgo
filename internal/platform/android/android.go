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
	"encoding/base64"
	"encoding/binary"
	"errors"
	"io"
	"log"
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

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/engine"
	"github.com/oneo-me/nsgo/internal/platform/fontfile"
	"github.com/oneo-me/nsgo/internal/platform/imagedecode"
	"github.com/oneo-me/nsgo/internal/platform/imageload"
	"github.com/oneo-me/nsgo/internal/testhost"
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
	msgLines   = 7 // asks a text layout's lines
	// msgCanvasImage is the bytes of an image canvases draw, by its key.
	msgCanvasImage = 8
	// msgFonts lists the app's font files, with each face's family,
	// weight and slant, before anything else.
	msgFonts = 9
	// msgTest is a test host's step for the runtime's test agent.
	msgTest = 10
	// msgHostCall is the app's call of an embedding host's handler, a URL
	// for its handler or a failure (host.go).
	msgHostCall = 11
	// msgCarets asks where a text layout's characters are, for hit
	// testing.
	msgCarets = 12
	// msgWantFrame asks for msgFrame at the next Choreographer frame.
	msgWantFrame = 13

	// Java to Go.
	msgStart       = 1
	msgResize      = 2
	msgEvent       = 3
	msgMeasured    = 4
	msgEnvironment = 5  // the environment, before msgStart and on change
	msgDNS         = 6  // the default network's DNS servers
	msgLaidOut     = 7  // where a msgLayout put a text's attachments
	msgTextLines   = 8  // a text layout's lines, answering msgLines
	msgTestReply   = 9  // the test agent's answer to a msgTest
	msgHost        = 10 // the embedding host's part of the channel (host.go)
	msgCaretLines  = 11 // a text layout's lines and clusters, answering msgCarets
	msgFrame       = 12 // a Choreographer frame began, answering msgWantFrame
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
	measured chan driver.Size
	laidOut  chan []attachmentBox
	lines    chan []driver.TextLine
	carets   chan driver.CaretLayout
	engine   *engine.Engine
	started  chan struct{}      // closed once engine is set
	env      driver.Environment // the last reported, on the reader goroutine

	ratio atomic.Uint32 // the pixel ratio's float32 bits, for rasterizing SVG

	// On the engine goroutine.
	images   map[driver.NodeID]*imageView
	requests uint32 // the last image request number
	// canvasImages are the images canvases and shaders asked for, by their
	// key; programs the shader programs translated, numbered from
	// nextProgram.
	canvasImages map[string]bool
	programs     map[*driver.ShaderProgram]*glProgram
	nextProgram  uint32

	// The native components; cmu guards them, as the reader goroutine
	// hands them their views' events.
	cmu        sync.Mutex
	components driver.NativeComponents
	natives    map[driver.NodeID]*nativeView

	// testReply takes the runtime's answers to a test host's steps.
	testReply func(r *reader)
	// host is the channel to the app embedding the view.
	host *embedHost
}

// Options configures Run.
type Options struct {
	// Components are the native components Native elements name.
	Components driver.NativeComponents
	// Fonts are font files under the app's resources, registered before
	// the first view.
	Fonts []string
	// Appearance is the appearance the app starts in.
	Appearance driver.Appearance
}

// Run serves app to the Java runtime over standard input and output until
// the runtime closes them. Call it from main.
func Run(app engine.App, opts Options) {
	s := connect(opts)
	s.serve(app, opts, nil)
}

// connect keeps the pipe to the runtime to the surface, and sends the
// app's own prints to standard error, which the runtime logs.
func connect(opts Options) *surface {
	fd, err := syscall.Dup(1)
	if err != nil {
		panic(err)
	}
	if err := syscall.Dup3(2, 1, 0); err != nil {
		panic(err)
	}
	s := &surface{
		out:      os.NewFile(uintptr(fd), "nsgo-runtime"),
		measured: make(chan driver.Size, 1),
		laidOut:  make(chan []attachmentBox, 1),
		lines:    make(chan []driver.TextLine, 1),
		carets:   make(chan driver.CaretLayout, 1),
		started:  make(chan struct{}),
		env:      driver.Environment{OS: driver.OSAndroid},
		images:   map[driver.NodeID]*imageView{},

		components: opts.Components,
		natives:    map[driver.NodeID]*nativeView{},
		host:       newEmbedHost(),
	}
	useDNS()
	s.sendFonts(opts.Fonts)
	return s
}

// serve takes the runtime's messages until it closes the pipe. A test
// host's app is the test's: the runtime's size goes to test, and the
// test starts the engine.
func (s *surface) serve(app engine.App, opts Options, test *testOpen) {
	in := bufio.NewReader(os.Stdin)
	for {
		kind, payload, err := readMessage(in)
		if err != nil {
			// The runtime is gone, and the app with it.
			if s.engine != nil {
				s.engine.Close()
			}
			s.closeNatives()
			return
		}
		r := reader{payload}
		switch kind {
		case msgStart:
			size := driver.Size{Width: r.points(), Height: r.points()}
			if test != nil {
				test.size <- size
				<-s.started
				break
			}
			s.startEngine(app, size, opts.Appearance, nil)
		case msgFrame:
			s.engine.Frame()
		case msgResize:
			s.engine.Resize(driver.Size{Width: r.points(), Height: r.points()})
		case msgEvent:
			ev := driver.Event{
				Kind:  driver.EventKind(r.byte()),
				ID:    driver.NodeID(r.u32()),
				Count: r.u32(),
				Point: driver.Point{X: r.points(), Y: r.points()},
				Text:  r.str(),
				Size:  driver.Size{Width: r.points(), Height: r.points()},
				Index: int(int32(r.u32())),
				// A gesture's; zero for other events.
				Phase:    driver.GesturePhase(r.byte()),
				Velocity: driver.Point{X: r.points(), Y: r.points()},
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
			// A pointer's position in its root view and modifier keys.
			ev.Page = driver.Point{X: r.points(), Y: r.points()}
			mods := r.byte()
			ev.Modifiers = driver.Modifiers{Control: mods&1 != 0, Alt: mods&2 != 0, Shift: mods&4 != 0, Platform: mods&8 != 0}
			if ev.Kind == driver.EventNative {
				s.nativeEvent(ev.ID, ev.Text, data)
				break
			}
			if ev.Kind == driver.EventTouch {
				ev.Touches = readTouches(data)
			}
			switch ev.Kind {
			case driver.EventPaste:
				ev.Data, ev.Text = readPaste(ev.Text, data), ""
			case driver.EventClipboard:
				readClipboard(&ev, data)
			case driver.EventPick:
				ev.Data = readPicked(data)
			}
			if ev.Kind == driver.EventPan {
				// A pan's start and end carry its bound values as the size.
				ev.Values, ev.Size = driver.Point{X: ev.Size.Width, Y: ev.Size.Height}, driver.Size{}
			}
			s.engine.Dispatch(ev)
		case msgMeasured:
			s.measured <- driver.Size{Width: r.points(), Height: r.points()}
		case msgLaidOut:
			boxes := make([]attachmentBox, r.u32())
			for i := range boxes {
				// NaN for one the line limit or the ellipsis cut off.
				x, y := r.points(), r.points()
				boxes[i] = attachmentBox{driver.Point{X: x, Y: y}, !math.IsNaN(float64(x))}
			}
			s.laidOut <- boxes
		case msgTextLines:
			lines := make([]driver.TextLine, r.u32())
			for i := range lines {
				l := &lines[i]
				l.Start, l.End = int(r.u32()), int(r.u32()) // UTF-8 offsets
				l.Bounds = driver.Bounds{Origin: driver.Point{X: r.points(), Y: r.points()},
					Size: driver.Size{Width: r.points(), Height: r.points()}}
				l.Ascender, l.Descender, l.CapHeight, l.XHeight = r.points(), r.points(), r.points(), r.points()
			}
			s.lines <- lines
		case msgCaretLines:
			s.carets <- readCarets(&r)
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
		case msgTestReply:
			if s.testReply != nil {
				s.testReply(&r)
			}
		case msgHost:
			s.hostMessage(&r)
		case msgDNS:
			servers := make([]string, r.u32())
			for i := range servers {
				servers[i] = r.str()
			}
			dnsServers.Store(&servers)
		}
	}
}

// startEngine starts app's engine at size; test, when set, is what a test
// host opens it with.
func (s *surface) startEngine(app engine.App, size driver.Size, appearance driver.Appearance, test *testOpen) {
	eopts := engine.Options{Size: size, Environment: s.env, Appearance: appearance, Host: s.hostOptions()}
	// The runtime passes the app's own files and cache directories.
	eopts.SetDirs(os.Getenv("NSGO_DATA_DIR"), os.Getenv("NSGO_CACHE_DIR"))
	if test != nil {
		eopts.UseTest(test.options, &test.pin)
		// A phone's screen is what it is.
		eopts.Size = size
	}
	imageload.UseDiskCache(eopts.CacheDir)
	s.engine = engine.Start(s, app, eopts)
	close(s.started)
}

// testOpen is what a test host opens the app with: the test's options,
// the environment it pins and the runtime's size, once it starts.
type testOpen struct {
	options testhost.Options
	pin     engine.TestPin
	size    chan driver.Size
}

// readNavigation reads the web view's navigation NsgoView.event writes
// last, nil for events without one.
func readNavigation(r *reader) *driver.WebNavigation {
	flags := r.byte()
	if flags&1 == 0 {
		return nil
	}
	return &driver.WebNavigation{
		Loading:      flags&2 != 0,
		CanGoBack:    flags&4 != 0,
		CanGoForward: flags&8 != 0,
		URL:          r.str(),
		Title:        r.str(),
	}
}

// readClipboard fills in what an EventClipboard carries in its data:
// "image", the PNG's size and the PNG in base64, or "uri" lines of a
// file's file: URL.
func readClipboard(ev *driver.Event, data string) {
	for _, line := range strings.Split(data, "\n") {
		kind, rest, _ := strings.Cut(line, "\t")
		switch kind {
		case "uri":
			ev.URIs = append(ev.URIs, rest)
		case "image":
			f := strings.SplitN(rest, "\t", 3)
			if len(f) != 3 {
				continue
			}
			w, _ := strconv.Atoi(f[0])
			h, _ := strconv.Atoi(f[1])
			if b, err := base64.StdEncoding.DecodeString(f[2]); err == nil && len(b) <= driver.MaxClipboardImage {
				ev.Data = &driver.ClipboardImage{PNG: b, Size: driver.Size{Width: driver.Points(w), Height: driver.Points(h)}}
			}
		}
	}
}

// readPicked reads the files an EventPick carries in its data, one per
// line: the name, the MIME type, the width, the height, the time it was
// last changed in milliseconds and the bytes in base64.
func readPicked(data string) []driver.PickedFile {
	var files []driver.PickedFile
	for _, line := range strings.Split(data, "\n") {
		f := strings.Split(line, "\t")
		if len(f) != 6 {
			continue
		}
		b, err := base64.StdEncoding.DecodeString(f[5])
		if err != nil {
			continue
		}
		w, _ := strconv.Atoi(f[2])
		h, _ := strconv.Atoi(f[3])
		file := driver.PickedFile{Name: f[0], MIME: f[1], Data: b, Size: driver.Size{Width: driver.Points(w), Height: driver.Points(h)}}
		if ms, _ := strconv.ParseInt(f[4], 10, 64); ms > 0 {
			file.Modified = time.UnixMilli(ms)
		}
		files = append(files, file)
	}
	return files
}

// readEnvironment reads the environment NsgoView.environment writes.
func readEnvironment(r *reader) driver.Environment {
	env := driver.Environment{
		OS:             driver.OSAndroid,
		SafeArea:       driver.Edges{Top: r.points(), Right: r.points(), Bottom: r.points(), Left: r.points()},
		KeyboardHeight: r.points(),
		FontScale:      r.float(),
		PixelRatio:     r.float(),
	}
	flags := r.byte()
	env.Dark = flags&1 != 0
	env.ReduceMotion = flags&2 != 0
	env.ScreenReader = flags&4 != 0
	env.RTL = flags&8 != 0
	env.Idiom = driver.Idiom(r.byte())
	env.Locale = r.str()
	// The toolbar's heights, without and with a large title.
	env.HeaderHeight, env.LargeHeaderHeight = r.points(), r.points()
	env.OSVersion.Major = int(r.u32()) // the API level
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
func (s *surface) Prepare(t *driver.AttributedText) driver.PreparedText {
	p := preparedText{s: s, id: s.ids.Add(1)}
	if (t.Ellipsize == driver.EllipsizeHead || t.Ellipsize == driver.EllipsizeMiddle) && t.MaxLines > 1 {
		// StaticLayout ellipsizes a head or a middle on one line only.
		c := *t
		p.cut = &c
	}
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
		// Where the line limit cuts, the shadow, then the decorations'
		// styles and colors of the text and each span.
		w.byte(byte(t.Ellipsize))
		w.u32(uint32(t.Shadow.Color)) // 0: none
		w.points(t.Shadow.Offset.X)
		w.points(t.Shadow.Offset.Y)
		w.points(t.Shadow.Radius)
		w.byte(byte(t.Style.DecorationStyle))
		w.u32(uint32(t.Style.DecorationColor))
		for _, sp := range t.Spans {
			w.byte(byte(sp.Style.DecorationStyle))
			w.u32(uint32(sp.Style.DecorationColor))
		}
	})
	return p
}

// preparedText is text Java keeps under id until Release, and the sizes
// of its attachments.
type preparedText struct {
	s           *surface
	id          uint32
	attachments []driver.Size
	// cut is the text, kept when it ellipsizes at its head or middle on
	// several lines: Layout cuts its last line's rest.
	cut *driver.AttributedText
}

// Measure asks the runtime's StaticLayout and waits for the answer. Only
// the engine goroutine measures, so one request is in flight at a time.
func (p preparedText) Measure(maxWidth driver.Points) driver.Size {
	p.s.send(msgMeasure, func(w *writer) {
		w.u32(p.id)
		w.points(maxWidth) // Auto, a NaN, does not wrap
	})
	return <-p.s.measured
}

// Layout has Java build the final StaticLayout under a new id for an
// ItemText to name: Java reads it before the batch. Only a text with
// attachments waits for an answer, where they went.
func (p preparedText) Layout(size driver.Size) driver.TextLayout {
	if p.cut != nil {
		if l := p.layoutCut(size); l != nil {
			return l
		}
	}
	l := &textLayout{id: p.s.ids.Add(1), s: p.s}
	l.carets.Set(l.caretLines)
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

// layoutCut lays p out with its last line's rest cut at its head or
// middle, as driver.AttributedText.CutRest does, from where StaticLayout
// breaks its lines at size's width; nil when the lines fit.
func (p preparedText) layoutCut(size driver.Size) driver.TextLayout {
	whole := *p.cut
	whole.MaxLines = 0
	q := p.s.Prepare(&whole).(preparedText)
	ql := q.Layout(size).(*textLayout)
	lines := ql.TextLines()
	p.s.send(msgRelease, func(w *writer) { w.u32(ql.id) })
	q.Release()
	if len(lines) <= p.cut.MaxLines {
		return nil
	}
	fits := func(c *driver.AttributedText) bool {
		r := p.s.Prepare(c)
		defer r.Release()
		return r.Measure(driver.Auto).Width <= size.Width+0.01
	}
	cut := p.cut.CutRest(lines[p.cut.MaxLines-1].Start, fits)
	r := p.s.Prepare(&cut).(preparedText)
	defer r.Release()
	l := r.Layout(size).(*textLayout)
	// Attachments by the app's indices: those cut out are hidden.
	if n := len(p.cut.Attachments); n > 0 {
		boxes, sizes := make([]attachmentBox, n), make([]driver.Size, n)
		for k, o := range cut.Origin {
			if k < len(l.attachments) && o < n {
				boxes[o], sizes[o] = l.attachments[k], l.sizes[k]
			}
		}
		l.attachments, l.sizes = boxes, sizes
	}
	return l
}

// textLayout names a StaticLayout Java built, and where it put the
// text's attachments. The runtime drops the layout itself when a later
// ItemText replaces it or its view is deleted.
type textLayout struct {
	id          uint32
	s           *surface
	attachments []attachmentBox
	sizes       []driver.Size
	// carets hit-tests the lines, which Java reads when first asked.
	carets driver.LazyCarets
}

// HitTest implements driver.TextHitTester.
func (l *textLayout) HitTest(p driver.Point) driver.Caret { return l.carets.HitTest(p) }

// CaretRect implements driver.TextHitTester.
func (l *textLayout) CaretRect(c driver.Caret) (driver.Bounds, bool) { return l.carets.CaretRect(c) }

// RangeRects implements driver.TextHitTester.
func (l *textLayout) RangeRects(start, end int, out []driver.Bounds) []driver.Bounds {
	return l.carets.RangeRects(start, end, out)
}

// caretLines asks Java where the StaticLayout's characters are, which it
// keeps while a text view shows it, and waits for the answer.
func (l *textLayout) caretLines() driver.CaretLayout {
	l.s.send(msgCarets, func(w *writer) { w.u32(l.id) })
	return <-l.s.carets
}

// readCarets reads a msgCaretLines: each line's UTF-16 range, box and
// direction, then its clusters'.
func readCarets(r *reader) driver.CaretLayout {
	out := make(driver.CaretLayout, r.u32())
	for i := range out {
		l := &out[i]
		l.Start, l.End = int(r.u32()), int(r.u32())
		l.Box = driver.Bounds{Origin: driver.Point{X: r.points(), Y: r.points()},
			Size: driver.Size{Width: r.points(), Height: r.points()}}
		l.RTL = r.byte() != 0
		l.Clusters = make([]driver.CaretCluster, r.u32())
		for k := range l.Clusters {
			c := &l.Clusters[k]
			c.Start, c.End = int(r.u32()), int(r.u32())
			c.Left, c.Right = r.points(), r.points()
			c.RTL = r.byte() != 0
		}
	}
	return out
}

// TextLines implements core.LineLayout: Java answers with the lines of
// the StaticLayout it keeps until the batch that names it.
func (l *textLayout) TextLines() []driver.TextLine {
	l.s.send(msgLines, func(w *writer) { w.u32(l.id) })
	return <-l.s.lines
}

// attachmentBox is the top left of an attachment in the content box.
type attachmentBox struct {
	origin driver.Point
	shown  bool
}

func (*textLayout) Release() {}

// Attachment implements core.AttachmentLayout.
func (l *textLayout) Attachment(i int) (driver.Bounds, bool) {
	if i < 0 || i >= len(l.attachments) || i >= len(l.sizes) || !l.attachments[i].shown {
		return driver.Bounds{}, false
	}
	return driver.Bounds{Origin: l.attachments[i].origin, Size: l.sizes[i]}, true
}

// RequestFrame implements engine.Platform: Java answers with msgFrame
// when its next Choreographer frame begins, before it steps the native
// animations.
func (s *surface) RequestFrame() { s.send(msgWantFrame, func(*writer) {}) }

// Mount implements engine.Platform: one message per commit, then the
// loads of the image sources it changed.
func (s *surface) Mount(b *driver.MountBatch) {
	requests, loads := s.trackImages(b)
	natives := s.trackNatives(b)
	shaders := s.trackShaders(b)
	s.send(msgBatch, func(w *writer) { encode(w, b, requests, natives, shaders) })
	for _, l := range loads {
		go s.load(l)
	}
	// Go loads the images Java shows, so it answers ImageSize too.
	for i := range b.Items {
		if it := b.Items[i]; it.Kind == driver.ItemImageInfo {
			go s.engine.Dispatch(imageload.Info(&it, imagedecode.Size))
		}
	}
}

// imageView is what Go keeps of an image view: the sources of its
// current request and how to stop loading them.
type imageView struct {
	source, placeholder driver.ImageSource
	events              driver.EventMask
	request             uint32
	cancel              context.CancelFunc
	size                driver.Size // the view's, which an SVG is drawn for
}

func (v *imageView) stop() {
	if v.cancel != nil {
		v.cancel()
	}
}

// imageLoad fetches the sources of one request of an image view.
type imageLoad struct {
	ctx                 context.Context
	id                  driver.NodeID
	request             uint32
	source, placeholder driver.ImageSource
	events              driver.EventMask
	size                driver.Size
}

// trackImages follows the image views of b. A view whose sources change
// gets a new request number, which its ItemImage carries and its bytes
// name, so Java drops the bytes of requests it has moved past. It returns
// the request of each ItemImage in b, in order, and the loads to start.
func (s *surface) trackImages(b *driver.MountBatch) ([]uint32, []imageLoad) {
	var requests []uint32
	var loads []imageLoad
	for i := range b.Items {
		it := &b.Items[i]
		v := s.images[it.ID]
		switch it.Kind {
		case driver.ItemCreate, driver.ItemDelete:
			if v != nil {
				v.stop()
				delete(s.images, it.ID)
			}
			if it.Kind == driver.ItemCreate && it.View == driver.KindImage {
				s.images[it.ID] = &imageView{}
			}
		case driver.ItemEvents:
			if v != nil {
				v.events = it.Events
			}
		case driver.ItemFrame:
			if v != nil {
				v.size = it.Frame.Size
			}
		case driver.ItemCanvas:
			s.loadCanvasImages(it.Drawing)
		case driver.ItemImage:
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

func sameSource(a, b driver.ImageSource) bool {
	return a.URI == b.URI && len(a.Data) == len(b.Data) && (len(a.Data) == 0 || &a.Data[0] == &b.Data[0])
}

func isEmpty(src driver.ImageSource) bool { return src.URI == "" && len(src.Data) == 0 }

// load fetches l's placeholder and source side by side and hands their
// bytes to Java, which decodes them and reports EventLoad, or EventError
// for bytes that are no image.
func (s *surface) load(l imageLoad) {
	if !isEmpty(l.placeholder) {
		go func() {
			if data, err := imageload.Load(l.ctx, l.placeholder, nil); err == nil && l.ctx.Err() == nil {
				if img, err := s.prepareImage(l.ctx, data, l.placeholder.URI, l.size); err == nil {
					s.sendImage(l, imagePlaceholder, img)
				}
			}
		}()
	}
	if isEmpty(l.source) {
		return
	}
	s.report(l, driver.Event{Kind: driver.EventLoadStart})
	var last time.Time
	data, err := imageload.Load(l.ctx, l.source, func(loaded, total int64) {
		if now := time.Now(); l.ctx.Err() == nil && now.Sub(last) >= 100*time.Millisecond {
			last = now
			s.report(l, driver.Event{Kind: driver.EventProgress, Loaded: loaded, Total: total})
		}
	})
	switch {
	case l.ctx.Err() != nil:
		// The view is gone or shows other sources.
	case err != nil:
		s.report(l, driver.Event{Kind: driver.EventError, Text: err.Error()})
		s.report(l, driver.Event{Kind: driver.EventLoadEnd})
	default:
		img, err := s.prepareImage(l.ctx, data, l.source.URI, l.size)
		if err != nil {
			s.report(l, driver.Event{Kind: driver.EventError, Text: err.Error()})
			s.report(l, driver.Event{Kind: driver.EventLoadEnd})
			return
		}
		s.sendImage(l, imageSource, img)
	}
}

// report dispatches ev about l's view if the view reports its kind.
func (s *surface) report(l imageLoad, ev driver.Event) {
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
func encode(w *writer, b *driver.MountBatch, requests []uint32, natives []nativeItem, shaders []shaderItem) {
	w.u32(uint32(b.Root))
	writeLayoutAnimation(w, b.LayoutAnimation)
	w.u32(uint32(len(b.Items)))
	for i := range b.Items {
		it := &b.Items[i]
		w.byte(byte(it.Kind))
		w.u32(uint32(it.ID))
		switch it.Kind {
		case driver.ItemCreate:
			w.byte(byte(it.View))
		case driver.ItemInsert:
			w.u32(uint32(it.Parent))
			w.u32(uint32(it.Before))
		case driver.ItemFrame:
			w.bounds(it.Frame)
		case driver.ItemViewStyle:
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
			w.flags(v.Clip, v.Hidden, v.ContinuousCorners, v.BackfaceHidden)
			writeDecor(w, v)
		case driver.ItemText:
			t := it.Text
			var layout uint32
			if l, ok := t.Layout.(*textLayout); ok {
				layout = l.id
			}
			w.str(t.Text)
			w.u32(layout)
			w.points(t.Content.Origin.X)
			w.points(t.Content.Origin.Y)
		case driver.ItemInput:
			in := it.Input
			writeStyle(w, &in.Style)
			w.str(in.Placeholder)
			w.byte(byte(in.KeyboardType))
			w.flags(in.Masked, in.Readonly, in.Disabled)
			w.bounds(in.Content)
			w.u32(uint32(in.Accessory))
			writeEditing(w, in)
		case driver.ItemInputValue:
			w.str(it.Value)
			w.u32(it.Count)
		case driver.ItemSelection:
			w.u32(uint32(max(it.Selection.Start, 0)))
			w.u32(uint32(max(it.Selection.End, 0)))
		case driver.ItemScrollBy:
			w.points(it.Point.X)
			w.points(it.Point.Y)
		case driver.ItemStickyCells:
			w.u32(uint32(len(it.Sticky)))
			for _, c := range it.Sticky {
				w.u32(uint32(c.Cell))
				w.points(c.Start)
				w.points(c.End)
				w.u32(c.Node)
				w.points(c.Inset)
			}
		case driver.ItemEvents:
			w.u64(uint64(it.Events))
		case driver.ItemViewProps:
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
			// Accessibility beyond the role, label and states.
			w.str(p.Hint)
			w.flags(p.Expandable, p.Expanded, p.ElementsHidden, p.Value.IsRange())
			w.byte(byte(p.Accessible))
			w.byte(byte(p.LiveRegion))
			w.float(p.Value.Min)
			w.float(p.Value.Max)
			w.float(p.Value.Now)
			w.str(p.Value.Text)
			actions := p.ActionList()
			w.u32(uint32(len(actions)))
			for _, act := range actions {
				w.str(act.Name)
				w.str(act.Label)
			}
			// The runtime routes touches by them and times long presses.
			w.byte(byte(p.PointerEvents))
			w.points(p.HitSlop.Top)
			w.points(p.HitSlop.Right)
			w.points(p.HitSlop.Bottom)
			w.points(p.HitSlop.Left)
			w.u32(uint32(max(p.DelayLongPress.Milliseconds(), 0)))
			if r := p.Ripple; r != nil {
				w.flags(true, r.Borderless, r.Foreground)
				w.u32(uint32(r.Color))
				w.points(r.Radius)
			} else {
				w.byte(0)
			}
			// Selections run on the UI thread (Selection.java).
			w.flags(p.SelectionArea)
			w.byte(byte(p.Selectable))
			w.u32(uint32(p.SelectionColor))
		case driver.ItemScrollConfig:
			c := it.Scroll
			b := &c.Behavior
			w.flags(c.Horizontal, c.HideIndicator)
			w.flags(b.Paging, b.DisableIntervalMomentum, b.NoScroll, b.NoBounces, c.Refreshable, c.Refreshing,
				c.NoBouncesZoom, c.NoPinchZoom)
			w.flags(c.CenterContent)
			w.points(b.SnapToInterval)
			w.byte(byte(b.SnapToAlignment))
			w.u32(uint32(len(b.SnapToOffsets)))
			for _, o := range b.SnapToOffsets {
				w.points(o)
			}
			w.float(b.Deceleration())
			in := b.ContentInset
			w.points(in.Top)
			w.points(in.Right)
			w.points(in.Bottom)
			w.points(in.Left)
			w.byte(byte(b.KeyboardDismissMode))
			w.byte(byte(b.KeyboardShouldPersistTaps))
			w.float(c.MinZoomScale)
			w.float(c.MaxZoomScale)
			w.float(c.ZoomScale)
			w.points(b.RefreshIndicator)
		case driver.ItemScrollContent:
			w.points(it.Size.Width)
			w.points(it.Size.Height)
		case driver.ItemImage:
			c := it.Image
			w.u32(requests[0])
			requests = requests[1:]
			w.byte(byte(c.ResizeMode - driver.Cover)) // Java's are 0-based
			w.points(c.BlurRadius)
			w.u32(uint32(max(c.FadeDuration.Milliseconds(), 0))) // 0: the platform's
			// Each axis of the position as a fraction of the free space
			// plus points, which is what any position resolves to.
			for _, a := range [...]driver.PositionAxis{c.Position.X, c.Position.Y} {
				at := a.Resolve(0)
				w.float(float32(a.Resolve(1) - at))
				w.points(at)
			}
		case driver.ItemModal:
			w.byte(byte(it.Modal.Animation))
		case driver.ItemAlert:
			a := it.Alert
			w.u32(a.ID)
			w.str(a.Title)
			w.str(a.Message)
			w.u32(uint32(len(a.Actions)))
			for _, act := range a.Actions {
				w.str(act.Label)
				w.byte(byte(act.Role))
			}
			w.flags(a.Sheet)
			w.flags(a.Cancelable)
		case driver.ItemScrollTo:
			w.points(it.Point.X)
			w.points(it.Point.Y)
			w.flags(it.Animated)
		case driver.ItemZoomTo:
			z := it.Zoom
			w.float(z.Scale)
			var center driver.Point
			if z.Center != nil {
				center = *z.Center
			}
			w.flags(z.Center != nil)
			w.points(center.X)
			w.points(center.Y)
			w.flags(it.Animated)
		case driver.ItemCanvas:
			// The ops' length first: Java keeps them as they are and plays
			// them on each draw.
			writeDrawing(w, it.Drawing)
		case driver.ItemShader:
			if it.Shader != nil && len(shaders) > 0 {
				writeShader(w, it.Shader, shaders[0])
				shaders = shaders[1:]
			} else {
				w.u32(0) // no program
				w.flags(false)
				w.u32(0)
				w.u32(0)
				w.u32(0)
			}
		case driver.ItemSetClipboard, driver.ItemAnnounce, driver.ItemTitle, driver.ItemAccessibilityLabel:
			w.str(it.Value)
		case driver.ItemReadClipboard:
			w.u32(it.Count)
			w.byte(byte(it.Clipboard))
		case driver.ItemSetClipboardImage:
			data, _ := it.Data.([]byte)
			w.str(string(data))
		case driver.ItemOpenURL:
			w.u32(it.Count)
			w.str(it.Value)
		case driver.ItemPickImages:
			// The request, as Picker.java reads it: the limit, the longest
			// side (0: any), the JPEG quality (0: the default, keeping
			// JPEGs) and whether the stored format stays.
			r := it.Pick
			w.u32(it.Count)
			w.u32(uint32(r.Limit))
			w.u32(uint32(r.MaxDimension))
			w.float(r.Quality)
			w.flags(r.Format == driver.PickCurrent)
		case driver.ItemAppearance:
			w.byte(byte(it.Appearance))
		case driver.ItemKeymap:
			w.u32(uint32(len(it.Keys)))
			for _, k := range it.Keys {
				w.str(k.Key)
				w.flags(k.InText, k.Repeat)
			}
		case driver.ItemStatusBar:
			c := it.StatusBar
			w.byte(byte(c.Style))
			w.flags(c.Hidden, c.Animated)
		case driver.ItemOverflow:
			w.bounds(it.Frame)
		case driver.ItemScreen:
			c := it.Screen
			w.str(c.Title)
			w.flags(c.HeaderHidden, c.HeaderTransparent, c.LargeTitle, c.CanGoBack, c.BackHidden, c.NoPopOnSwipe, c.NoPopOnBack)
			w.str(c.BackTitle)
			w.u32(uint32(c.Background))
			w.u32(uint32(c.TitleColor))
			w.u32(uint32(c.TintColor))
			w.u32(uint32(c.Left))
			w.u32(uint32(c.Right))
			w.u32(uint32(c.TitleView))
			w.points(c.HeaderHeight)
		case driver.ItemWeb:
			c := it.Web
			w.str(c.Source.Src)
			w.str(c.Source.HTML)
			w.str(c.Source.BaseURL)
			w.str(c.InjectedJavaScript)
			// The bridge comes first, so the app's script can post.
			w.str(webBridge + "\n" + c.InjectedJavaScriptBeforeContent)
			w.str(c.UserAgent)
			w.flags(c.NoScroll, c.NoJavaScript) // Android has no back and forward swipes
		case driver.ItemVideo:
			c := it.Video
			w.str(c.Source)
			w.flags(c.Paused, c.Muted, c.Repeat, c.Controls)
			w.float(c.Volume)
			w.float(c.Rate)
			w.byte(byte(c.ResizeMode - driver.Cover)) // Java's are 0-based
			w.u32(uint32(c.ProgressInterval.Milliseconds()))
			w.byte(byte(c.AudioSession))
		case driver.ItemVideoSeek:
			w.u64(uint64(it.Time.Milliseconds()))
		case driver.ItemNative:
			n := natives[0]
			natives = natives[1:]
			w.str(n.class)
			w.u32(uint32(len(n.props)))
			for _, p := range n.props {
				w.str(p.Name)
				w.str(jsonValue(p.Value))
			}
			// The props animated nodes drive, when they changed: Java
			// hands the view their values on the frames they change.
			w.flags(it.Native.Bind)
			if it.Native.Bind {
				w.u32(uint32(len(it.Native.Bindings)))
				for _, b := range it.Native.Bindings {
					w.str(b.Name)
					w.u32(b.Node)
					w.flags(b.Color)
				}
			}
		case driver.ItemNativeCommand:
			w.str(it.Value)
			w.str(jsonValue(it.Data))
		case driver.ItemAnimation:
			writeAnimation(w, it.Animation)
		case driver.ItemWebCommand:
			w.byte(byte(it.WebCommand))
			if it.WebCommand == driver.WebPostMessage {
				w.str(driver.WebMessageScript(it.Value))
			} else {
				w.str(it.Value)
			}
		}
	}
}

// writeDrawing writes each op as its kind and the fields it uses, in the
// format CanvasNode plays, then where the arguments bound to animated
// nodes are in those bytes, which Java writes their values into, then
// the drawing's nodes. DrawTextAlign and DrawTextBaseline, which only
// the recorder reads, are left out.
func writeDrawing(w *writer, d *driver.Drawing) {
	type slot struct {
		at    int
		color bool
		node  uint32
	}
	var slots []slot
	at := len(w.buf)
	w.u32(0)
	start := len(w.buf)
	bound := map[[2]int]uint32{}
	if d != nil {
		for _, s := range d.Slots {
			bound[[2]int{s.Op, int(s.Arg)}] = s.Node
		}
	}
	var i int
	// arg writes an argument, noting where it is when a node drives it.
	arg := func(a driver.DrawArg, v float32) {
		if n, ok := bound[[2]int{i, int(a)}]; ok {
			slots = append(slots, slot{at: len(w.buf) - start, node: n})
		}
		w.float(v)
	}
	color := func(c driver.Color) {
		if n, ok := bound[[2]int{i, int(driver.ArgColor)}]; ok {
			slots = append(slots, slot{at: len(w.buf) - start, color: true, node: n})
		}
		w.u32(uint32(c))
	}
	var ops []driver.DrawOp
	if d != nil {
		ops = d.Ops
	}
	for i = range ops {
		op := &ops[i]
		if op.Kind == driver.DrawTextAlign || op.Kind == driver.DrawTextBaseline {
			continue
		}
		w.byte(byte(op.Kind))
		switch op.Kind {
		case driver.DrawTranslate, driver.DrawScale, driver.DrawMoveTo, driver.DrawLineTo:
			arg(driver.ArgX, op.X)
			arg(driver.ArgY, op.Y)
		case driver.DrawRotate, driver.DrawAlpha:
			arg(driver.ArgA, op.A)
		case driver.DrawQuadTo:
			arg(driver.ArgX1, op.X1)
			arg(driver.ArgY1, op.Y1)
			arg(driver.ArgX, op.X)
			arg(driver.ArgY, op.Y)
		case driver.DrawCubicTo:
			arg(driver.ArgX1, op.X1)
			arg(driver.ArgY1, op.Y1)
			arg(driver.ArgX2, op.X2)
			arg(driver.ArgY2, op.Y2)
			arg(driver.ArgX, op.X)
			arg(driver.ArgY, op.Y)
		case driver.DrawTransform, driver.DrawSetTransform:
			arg(driver.ArgX1, op.X1)
			arg(driver.ArgY1, op.Y1)
			arg(driver.ArgX2, op.X2)
			arg(driver.ArgY2, op.Y2)
			arg(driver.ArgX, op.X)
			arg(driver.ArgY, op.Y)
		case driver.DrawArc:
			arg(driver.ArgX, op.X)
			arg(driver.ArgY, op.Y)
			arg(driver.ArgR, op.R)
			arg(driver.ArgA0, op.A0)
			arg(driver.ArgA1, op.A1)
			w.flags(op.CCW)
		case driver.DrawRect, driver.DrawClear:
			arg(driver.ArgX, op.X)
			arg(driver.ArgY, op.Y)
			arg(driver.ArgW, op.W)
			arg(driver.ArgH, op.H)
		case driver.DrawRoundRect:
			arg(driver.ArgX, op.X)
			arg(driver.ArgY, op.Y)
			arg(driver.ArgW, op.W)
			arg(driver.ArgH, op.H)
			arg(driver.ArgR, op.R)
		case driver.DrawEllipse:
			arg(driver.ArgX, op.X)
			arg(driver.ArgY, op.Y)
			arg(driver.ArgR, op.R)
			arg(driver.ArgRY, op.RY)
		case driver.DrawFill, driver.DrawClip:
			w.flags(op.EvenOdd)
		case driver.DrawFillColor, driver.DrawStrokeColor:
			color(op.Color)
		case driver.DrawFillGradient, driver.DrawStrokeGradient:
			writeGradient(w, op.Gradient)
		case driver.DrawLineWidth, driver.DrawLineDashOffset, driver.DrawMiterLimit:
			arg(driver.ArgW, op.W)
		case driver.DrawLineCap:
			w.byte(byte(op.Cap))
		case driver.DrawLineJoin:
			w.byte(byte(op.Join))
		case driver.DrawComposite:
			w.byte(byte(op.Composite))
		case driver.DrawLineDash:
			var dash []float32
			if op.Dash != nil {
				dash = *op.Dash
			}
			w.u32(uint32(len(dash)))
			w.floats(dash...)
		case driver.DrawShadow:
			color(op.Color)
			arg(driver.ArgX, op.X)
			arg(driver.ArgY, op.Y)
			arg(driver.ArgR, op.R)
		case driver.DrawFillText, driver.DrawStrokeText:
			w.str(op.Text)
			// The point the recorder placed the text at.
			arg(driver.ArgX, op.X+op.X1)
			arg(driver.ArgY, op.Y+op.Y1)
			w.str(op.Font.Family)
			w.points(op.Font.Size)
			w.u16(uint16(op.Font.Weight))
			w.flags(op.Font.Italic)
		case driver.DrawImage:
			var key string
			var density float32 = 1
			if op.Image != nil {
				key, density = op.Image.Key(), op.Image.Density()
			}
			w.str(key)
			w.float(density)
			arg(driver.ArgX, op.X)
			arg(driver.ArgY, op.Y)
			arg(driver.ArgW, op.W)
			arg(driver.ArgH, op.H)
			arg(driver.ArgX1, op.X1)
			arg(driver.ArgY1, op.Y1)
			arg(driver.ArgX2, op.X2)
			arg(driver.ArgY2, op.Y2)
		}
	}
	binary.BigEndian.PutUint32(w.buf[at:], uint32(len(w.buf)-start))
	w.u32(uint32(len(slots)))
	for _, s := range slots {
		w.u32(uint32(s.at))
		w.flags(s.color)
		w.u32(s.node)
	}
	var nodes []driver.CanvasNode
	if d != nil {
		nodes = d.Nodes
	}
	w.u32(uint32(len(nodes)))
	for _, n := range nodes {
		w.bounds(n.Bounds)
		w.byte(byte(n.Role))
		w.str(n.Label)
		w.str(n.TestID)
		w.flags(n.Selected)
	}
}

// sendFonts tells Java the app's font files, paths in the APK's assets,
// with their faces' families, which Java loads as Typefaces before it
// prepares any text.
func (s *surface) sendFonts(paths []string) {
	type font struct {
		path string
		face fontfile.Face
	}
	var fonts []font
	for _, p := range paths {
		data, err := imageload.ReadResource(p)
		if err != nil {
			log.Printf("nsgo: font %s: %v", p, err)
			continue
		}
		faces, err := fontfile.Describe(data)
		if err != nil || len(faces) == 0 {
			log.Printf("nsgo: font %s: %v", p, err)
			continue
		}
		// Typeface reads a collection's first face.
		fonts = append(fonts, font{p, faces[0]})
	}
	if len(fonts) == 0 {
		return
	}
	s.send(msgFonts, func(w *writer) {
		w.u32(uint32(len(fonts)))
		for _, f := range fonts {
			w.str(f.path)
			w.str(f.face.Family)
			w.u16(uint16(f.face.Weight))
			w.flags(f.face.Italic)
		}
	})
}

// loadCanvasImages loads the images d draws that no canvas asked for
// yet and hands Java their bytes, which it decodes once for every
// canvas.
func (s *surface) loadCanvasImages(d *driver.Drawing) {
	if d == nil {
		return
	}
	var srcs []driver.ImageSource
	for i := range d.Ops {
		if op := &d.Ops[i]; op.Kind == driver.DrawImage && op.Image != nil {
			srcs = append(srcs, *op.Image)
		}
	}
	s.loadImages(srcs)
}

// loadImages loads the images canvases and shaders read that none asked
// for yet and hands Java their bytes, which it decodes once for all.
func (s *surface) loadImages(srcs []driver.ImageSource) {
	for _, src := range srcs {
		key := src.Key()
		if s.canvasImages[key] {
			continue
		}
		if s.canvasImages == nil {
			s.canvasImages = map[string]bool{}
		}
		s.canvasImages[key] = true
		go func() {
			data, err := imageload.Load(context.Background(), src, nil)
			var img decodedImage
			if err == nil {
				img, err = s.prepareImage(context.Background(), data, src.URI, driver.Size{})
			}
			if err != nil {
				log.Printf("nsgo: canvas: image %s: %v", key, err)
				return
			}
			s.send(msgCanvasImage, func(w *writer) {
				w.str(key)
				w.float(img.scale)
				w.u32(uint32(len(img.data)))
				w.buf = append(w.buf, img.data...)
			})
		}()
	}
}

// writeDecor writes what a view style draws past React Native's styles,
// after the rest, in the order NsgoView.style reads it.
func writeDecor(w *writer, v *driver.ViewStyle) {
	c := v.Corners
	w.points(c.TopLeft)
	w.points(c.TopRight)
	w.points(c.BottomRight)
	w.points(c.BottomLeft)
	w.byte(byte(v.BorderStyle))
	o := v.Outline
	w.points(o.Width)
	w.points(o.Offset)
	w.u32(uint32(o.Color))
	w.byte(byte(o.Style))
	tr := v.Transform
	w.floats(tr.RotateX, tr.RotateY)
	w.points(tr.Perspective)
	w.points(v.Origin.X)
	w.points(v.Origin.Y)
	w.points(v.ChildPerspective)
	w.byte(byte(v.BlendMode))
	e := v.Effects
	if e == nil {
		e = &driver.Effects{}
	}
	w.byte(byte(min(len(e.Shadows), 255)))
	for _, sh := range e.Shadows[:min(len(e.Shadows), 255)] {
		w.points(sh.Offset.X)
		w.points(sh.Offset.Y)
		w.points(sh.Blur)
		w.points(sh.Spread)
		w.u32(uint32(sh.Color))
		w.flags(sh.Inset)
	}
	w.byte(byte(min(len(e.Gradients), 255)))
	for _, g := range e.Gradients[:min(len(e.Gradients), 255)] {
		w.byte(byte(g.Kind))
		w.floats(g.X0, g.Y0, g.X1, g.Y1, g.CX, g.CY, g.RX, g.RY, g.Angle)
		w.u32(uint32(len(g.Stops)))
		for _, s := range g.Stops {
			w.float(s.Offset)
			w.u32(uint32(s.Color))
		}
	}
	// Filters: their color matrix, blur and widest drop shadow, the last
	// of equals, which Android draws from Android 12.
	m, color, blur, shadows := driver.SplitFilters(e.Filters)
	w.flags(color)
	if color {
		w.floats(m[:]...)
	}
	w.points(blur)
	w.byte(byte(min(len(shadows), 1)))
	if len(shadows) > 0 {
		sh := shadows[0]
		for _, s := range shadows[1:] {
			if s.Blur >= sh.Blur {
				sh = s
			}
		}
		w.points(sh.Offset.X)
		w.points(sh.Offset.Y)
		w.points(sh.Blur)
		w.u32(uint32(sh.Color))
	}
	// The keyframes' steps, which Java eases between.
	a := v.Animation
	d, frames := a.Cycle()
	w.u32(uint32(max(d.Milliseconds(), 0)))
	w.u32(uint32(max(a.Delay.Milliseconds(), 0)))
	w.u32(uint32(max(a.Iterations, 0)))
	w.byte(byte(len(frames)))
	for _, k := range frames {
		w.floats(k.Offset, k.Opacity, k.Scale, k.Rotate, k.TranslateY, k.Easing.X1, k.Easing.Y1, k.Easing.X2, k.Easing.Y2)
	}
}

// writeStyle writes a text style in the format Text.Style reads.
func writeStyle(w *writer, t *driver.TextStyle) {
	w.str(t.Font.Family)
	w.points(t.Font.Size)
	w.u16(uint16(t.Font.Weight))
	w.u32(uint32(t.Color))
	w.points(t.LineHeight)
	w.points(t.LetterSpacing)
	w.byte(byte(t.Align))
	w.flags(t.Font.Italic,
		t.Decoration&driver.TextDecorationUnderline != 0,
		t.Decoration&driver.TextDecorationLineThrough != 0, t.RTL)
}
