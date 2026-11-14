//go:build linux && !android

package linux

import (
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/ebitengine/purego"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/engine"
)

// nativeApp is the app a native test drives: a button and a field whose
// state the window's title shows, so the test can wait for the engine.
func nativeApp(title string) engine.App {
	return func(cx *core.Scope) core.Node {
		count := core.NewSignal(cx, 0)
		field := core.NewSignal(cx, "")
		submitted := core.NewSignal(cx, 0)
		keyed := core.NewSignal(cx, 0)
		cx.OnKey("mod+k", func() { keyed.Update(func(n int) int { return n + 1 }) })
		cx.Effect(func() {
			cx.SetTitle(fmt.Sprintf("%s|%d|%s|%d|%d", title, count.Get(), field.Get(), submitted.Get(), keyed.Get()))
		})
		return core.View{
			Style: core.Style{Width: driver.Pt(360), Height: driver.Pt(240), Padding: driver.All(10),
				Direction: core.Row, Gap: 10, Background: driver.RGB(0xffffff)},
			Children: []core.Node{
				core.View{OnPress: func(core.PressEvent) { count.Update(func(n int) int { return n + 1 }) },
					Style: core.Style{Width: driver.Pt(100), Height: driver.Pt(80), Background: driver.RGB(0x123456)}},
				core.Input{Value: field.Get, OnChange: field.Set, AutoFocus: true,
					OnSubmit: func(string) { submitted.Update(func(n int) int { return n + 1 }) },
					Style:    core.Style{Width: driver.Pt(200), Height: driver.Pt(30)}},
			},
		}
	}
}

// startTestWindow opens the window on a GTK thread of its own and returns
// it once its engine runs. The test skips where no display can open one.
func startTestWindow(t *testing.T, app engine.App, opts Options) *window {
	t.Helper()
	type result struct {
		w   *window
		err error
	}
	ready := make(chan result, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runtime.LockOSThread()
		w, err := start(app, opts)
		ready <- result{w, err}
		if err != nil {
			return
		}
		w.run()
		w.stop()
		// Close the window for good: its close request only stops the
		// loop, and a tiling window manager shares the screen between the
		// windows left open.
		gtkWindowDestroy(w.win)
		for gMainContextIteration(0, 0) != 0 {
		}
	}()
	var r result
	select {
	case r = <-ready:
	case <-time.After(10 * time.Second):
		t.Fatal("the window did not start")
	}
	if r.err != nil {
		<-done
		if strings.Contains(r.err.Error(), "no display") || strings.Contains(r.err.Error(), "GTK 4") {
			t.Skip(r.err)
		}
		t.Fatal(r.err)
	}
	t.Cleanup(func() {
		w := r.w
		if !w.closing {
			w.onLoop(t, func() { gtkWindowClose(w.win) })
		}
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Error("the window did not close")
		}
	})
	return r.w
}

// gMainContextIteration runs what the default main context has ready.
var gMainContextIteration = func() func(uintptr, int32) int32 {
	var fn func(uintptr, int32) int32
	if openLibs() == nil {
		purego.RegisterLibFunc(&fn, glibLib, "g_main_context_iteration")
	}
	return fn
}()

// onLoop runs fn on the window's GTK thread and waits for it.
func (w *window) onLoop(t *testing.T, fn func()) {
	t.Helper()
	done := make(chan struct{})
	w.post(func() {
		defer close(done)
		fn()
	})
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the GTK thread did not run the step")
	}
}

// title returns the window's GTK title.
func (w *window) titleOf(t *testing.T) string {
	t.Helper()
	var title string
	w.onLoop(t, func() { title = goString(gtkWindowGetTitle(w.win)) })
	return title
}

// waitForTitle waits for the engine to set the window's title.
func waitForTitle(t *testing.T, w *window, want string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if got := w.titleOf(t); got == want {
			return
		} else if time.Now().After(deadline) {
			t.Fatalf("window title = %q, want %q", got, want)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestWindowRunsAnAppAndTakesNativeInput opens a real GTK window, clicks
// its view, types into its GtkEntry and reads the pixels it painted.
func TestWindowRunsAnAppAndTakesNativeInput(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	title := "nsgo-native-" + strings.NewReplacer("/", "-", " ", "-").Replace(t.Name())
	w := startTestWindow(t, nativeApp(title), Options{Title: title, Width: 360, Height: 240})
	waitForTitle(t, w, title+"|0||0|0")

	// The app's keymap takes a native key press before the view does.
	w.onLoop(t, func() { keyPressedCallback(0, 'k', 0, gdkControl, w.handle) })
	waitForTitle(t, w, title+"|0||0|1")

	// The button at (10, 10) to (110, 90) in the surface.
	w.onLoop(t, func() { w.press(driver.Point{X: 60, Y: 50}) })
	waitForTitle(t, w, title+"|1||0|1")

	// The native GtkEntry reports its changes to the engine.
	w.onLoop(t, func() {
		for entry, v := range w.entries {
			if v.kind == driver.KindInput && entry == v.entry {
				gtkEditableSetText(entry, cstr("hi"))
			}
		}
	})
	waitForTitle(t, w, title+"|1|hi|0|1")

	// Return submits the focused field.
	w.onLoop(t, func() {
		for entry, v := range w.entries {
			if v.kind == driver.KindInput && entry == v.entry {
				gSignalEmitByName(entry, cstr("activate"))
			}
		}
	})
	waitForTitle(t, w, title+"|1|hi|1|1")

	// What the views drew is in the surface, not only in the engine's
	// model.
	waitPixels(t, w, "button pixel #123456", func(p pixels) bool {
		pix := p.at(60, 50)
		return pix[2] == 0x12 && pix[1] == 0x34 && pix[0] == 0x56
	})
}

// pixels is the surface as GTK draws it, premultiplied ARGB32.
type pixels struct {
	data   []byte
	stride int
	width  int
	height int
}

// at returns the BGRA bytes of the pixel at x, y.
func (p pixels) at(x, y int) []byte {
	if x < 0 || y < 0 || x >= p.width || y >= p.height {
		return []byte{0, 0, 0, 0}
	}
	return p.data[y*p.stride+x*4 : y*p.stride+x*4+4]
}

// surfacePixels draws the surface, natives included, into an image.
func (w *window) surfacePixels(t *testing.T) pixels {
	t.Helper()
	var p pixels
	w.onLoop(t, func() {
		width, height := int(gtkWidgetGetWidth(w.surface)), int(gtkWidgetGetHeight(w.surface))
		if width <= 0 || height <= 0 {
			return
		}
		surface := cairoImageSurfaceCreate(0, int32(width), int32(height))
		cr := cairoCreate(surface)
		w.paint(cr)
		cairoSurfaceFlush(surface)
		stride := int(cairoImageSurfaceGetStride(surface))
		data := (*[1 << 30]byte)(cPointer(cairoImageSurfaceGetData(surface)))[: stride*height : stride*height]
		p = pixels{data: append([]byte(nil), data...), stride: stride, width: width, height: height}
		cairoDestroy(cr)
		cairoSurfaceDestroy(surface)
	})
	return p
}

// waitPixels waits until the drawn surface satisfies ok: GTK lays out and
// draws a batch on its next frame.
func waitPixels(t *testing.T, w *window, what string, ok func(pixels) bool) pixels {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		p := w.surfacePixels(t)
		if p.width > 0 && ok(p) {
			return p
		}
		if time.Now().After(deadline) {
			t.Fatalf("want %s", what)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// pixelAt returns the four bytes of the premultiplied ARGB32 pixel at x,
// y.
func pixelAt(data uintptr, stride, x, y int) []byte {
	p := cPointer(data)
	row := (*[1 << 30]byte)(p)[y*stride : (y+1)*stride]
	return row[x*4 : x*4+4]
}

// TestTextDrawsBackgroundsAndAttachments opens a window with a text whose
// span has a background and whose attachment is a pressable view, reads
// the pixels painted and presses the attachment.
func TestTextDrawsBackgroundsAndAttachments(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	title := "nsgo-native-" + t.Name()
	app := func(cx *core.Scope) core.Node {
		pressed := core.NewSignal(cx, 0)
		cx.Effect(func() { cx.SetTitle(fmt.Sprintf("%s|%d", title, pressed.Get())) })
		return core.View{Style: core.Style{Width: driver.Pt(360), Height: driver.Pt(240), Padding: driver.All(10),
			Background: driver.RGB(0xffffff), FontSize: 16, Color: driver.RGB(0x000000)},
			Children: []core.Node{core.Text{Spans: []core.Span{
				{Content: "text "},
				{Content: "x      x", Style: core.Style{Background: driver.RGB(0xff0000), CornerRadius: 4}},
				{Content: " and "},
				{Style: core.Style{VerticalAlign: driver.VerticalAlignMiddle},
					Attachment: core.View{TestID: "button", OnPress: func(core.PressEvent) { pressed.Update(func(n int) int { return n + 1 }) },
						Style: core.Style{Width: driver.Pt(40), Height: driver.Pt(30), Background: driver.RGB(0x0000ff)}}},
				{Content: " after"},
			}}},
		}
	}
	w := startTestWindow(t, app, Options{Title: title, Width: 360, Height: 240})
	waitForTitle(t, w, title+"|0")

	var button driver.Bounds
	var bg rect
	var textAt driver.Point
	w.onLoop(t, func() {
		for _, v := range w.views {
			if v == nil || v.kind != driver.KindText {
				continue
			}
			b, _ := w.surfaceBounds(v)
			textAt = driver.Point{X: b.Origin.X + v.content.Origin.X, Y: b.Origin.Y + v.content.Origin.Y}
			if l, ok := v.text.(*textLayout); ok && len(l.under) > 0 {
				bg = l.under[0]
			}
			for _, c := range v.children {
				button, _ = w.surfaceBounds(c)
			}
		}
	})
	if button.Size != (driver.Size{Width: 40, Height: 30}) {
		t.Fatalf("attachment frame = %v, want a 40x30 child of the text", button)
	}
	if bg.w == 0 || button.Origin.X < textAt.X+driver.Points(bg.x+bg.w) {
		t.Fatalf("background %+v, attachment %v: want the attachment after the span", bg, button)
	}

	waitPixels(t, w, "a red span background and a blue attachment", func(p pixels) bool {
		span := p.at(int(textAt.X)+int(bg.x+bg.w/2), int(textAt.Y)+int(bg.y+bg.h/2))
		attached := p.at(int(button.Origin.X+20), int(button.Origin.Y+15))
		return span[2] == 0xff && span[1] == 0 && span[0] == 0 && attached[0] == 0xff && attached[1] == 0 && attached[2] == 0
	})

	// The attachment takes its own presses.
	w.onLoop(t, func() { w.press(driver.Point{X: button.Origin.X + 20, Y: button.Origin.Y + 15}) })
	waitForTitle(t, w, title+"|1")
}

// TestWebViewTalksToThePage opens a window with a web view showing
// inline HTML, receives the message the page posts and posts one back,
// which the page answers. It skips where WebKitGTK 6.0 is missing.
func TestWebViewTalksToThePage(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	if err := openWebKit(); err != nil {
		t.Skip(err)
	}
	const page = `<!doctype html><html><body><script>
		addEventListener('message', (e) => window.nsgo.postMessage('echo:' + e.data));
		window.nsgo.postMessage('ready');
	</script></body></html>`
	title := "nsgo-native-" + t.Name()
	app := func(cx *core.Scope) core.Node {
		var ref core.Ref
		last := core.NewSignal(cx, "")
		loaded := core.NewSignal(cx, 0)
		cx.Effect(func() { cx.SetTitle(fmt.Sprintf("%s|%d|%s", title, loaded.Get(), last.Get())) })
		return core.View{Style: core.Style{Width: driver.Pt(360), Height: driver.Pt(240), Padding: driver.All(10)},
			Children: []core.Node{core.WebView{Source: core.WebHTML(page, ""), Ref: &ref,
				Style:   core.Style{Width: driver.Pt(300), Height: driver.Pt(200)},
				OnLoad:  func(driver.WebNavigation) { loaded.Update(func(n int) int { return n + 1 }) },
				OnError: func(err error) { last.Set("error:" + err.Error()) },
				OnMessage: func(data string) {
					last.Set(data)
					if data == "ready" {
						ref.PostMessage("ping")
					}
				}}}}
	}
	w := startTestWindow(t, app, Options{Title: title, Width: 360, Height: 240})
	waitForTitle(t, w, title+"|1|echo:ping")

	// The page fills the web view's frame, hosted in its widget.
	waitHosted(t, w, func(v *view) bool { return v.web != nil && v.web.widget != 0 },
		driver.Bounds{Origin: driver.Point{X: 10, Y: 10}, Size: driver.Size{Width: 300, Height: 200}})
}

// TestWebViewsOutliveEachOther opens an Incognito web view and one in the app's session, both serving an app-owned scheme, and one that serves none. Each page keeps its scheme and downloads as the others unmount and mount again, and the page without a handler gets an error rather than a load that never ends. It skips where WebKitGTK 6.0 is missing. WebKit takes the thread that first uses it for its main one, and each test runs GTK on a thread of its own, so the test runs in a process of its own.
func TestWebViewsOutliveEachOther(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	if os.Getenv("NSGO_TEST_WEBKIT_PROCESS") != t.Name() {
		cmd := exec.Command(os.Args[0], "-test.run=^"+t.Name()+"$", "-test.v")
		// The app's website data and downloads go to a directory of the test's.
		dir := t.TempDir()
		cmd.Env = append(os.Environ(), "NSGO_TEST_WEBKIT_PROCESS="+t.Name(),
			"XDG_DATA_HOME="+filepath.Join(dir, "data"), "XDG_CACHE_HOME="+filepath.Join(dir, "cache"))
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		if strings.Contains(string(out), "--- SKIP") {
			t.Skipf("%s", out)
		}
		return
	}
	if err := openWebKit(); err != nil {
		t.Skip(err)
	}
	// B's page downloads a file as it loads, which its view saves. WebKit's network process cannot fetch app-owned schemes, so the file is a data: URL.
	const page = `<!doctype html><html><body><script>
		window.nsgo.postMessage(location.host);
		if (location.host === 'b' && location.pathname === '/') {
			const a = document.createElement('a');
			a.href = 'data:text/plain,saved';
			a.download = 'saved.txt';
			document.body.append(a);
			a.click();
		}
	</script></body></html>`
	title := "nsgo-native-" + t.Name()
	var root *core.Scope
	var showA, showB *core.Signal[bool]
	app := func(cx *core.Scope) core.Node {
		root = cx
		showA, showB = core.NewSignal(cx, true), core.NewSignal(cx, true)
		a, b := core.NewSignal(cx, 0), core.NewSignal(cx, 0)
		failed := core.NewSignal(cx, false)
		saved := core.NewSignal(cx, 0)
		cx.Effect(func() {
			cx.SetTitle(fmt.Sprintf("%s|a%d|b%d|%t|d%d", title, a.Get(), b.Get(), failed.Get(), saved.Get()))
		})
		frame := core.Style{Width: driver.Pt(100), Height: driver.Pt(100)}
		served := func(name string, incognito bool, count *core.Signal[int]) core.Node {
			return core.Func(func(cx *core.Scope) core.Node {
				if err := cx.ServeScheme("app", func(string) []byte { return []byte(page) }); err != nil {
					t.Error(err)
				}
				web := core.WebView{Source: core.WebSrc("app://" + name + "/"), Incognito: incognito, Style: frame,
					OnMessage: func(data string) {
						if data == name {
							count.Update(func(n int) int { return n + 1 })
						}
					}}
				if name == "b" {
					web.OnDownload = func(d core.WebDownload) {
						if d.State != core.WebDownloadFinished {
							return
						}
						if body, err := os.ReadFile(d.Path); err != nil || string(body) != "saved" || d.Filename != "saved.txt" {
							t.Errorf("download %+v saved %q, %v", d, body, err)
						}
						saved.Update(func(n int) int { return n + 1 })
					}
				}
				return web
			})
		}
		return core.View{Style: core.Style{Width: driver.Pt(360), Height: driver.Pt(240), Direction: core.Row}, Children: []core.Node{
			core.Show{When: showA.Get, Then: func(*core.Scope) core.Node { return served("a", true, a) }},
			core.Show{When: showB.Get, Then: func(*core.Scope) core.Node { return served("b", false, b) }},
			core.WebView{Source: core.WebSrc("app://c/"), Style: frame, OnError: func(error) { failed.Set(true) }},
		}}
	}
	w := startTestWindow(t, app, Options{ID: "nsgo-test-webviews", Title: title, Width: 360, Height: 240})
	waitForTitle(t, w, title+"|a1|b1|true|d1")

	// The app's session outlives its view: B mounts again into it, and A still serves its scheme.
	root.Post(func() { showB.Set(false) })
	waitTitle(t, w, func(s string) bool { return webPagesOf(t, w) == 2 })
	root.Post(func() { showB.Set(true) })
	waitForTitle(t, w, title+"|a1|b2|true|d2")

	// An Incognito view's own session goes with it, and the view mounts again with a new one, while B keeps serving.
	root.Post(func() { showA.Set(false) })
	waitTitle(t, w, func(s string) bool { return webPagesOf(t, w) == 2 })
	root.Post(func() { showA.Set(true) })
	waitForTitle(t, w, title+"|a2|b2|true|d2")

}

// webPagesOf returns how many pages w shows.
func webPagesOf(t *testing.T, w *window) int {
	t.Helper()
	n := 0
	w.onLoop(t, func() {
		for _, v := range w.views {
			if v != nil && !v.deleted && v.web != nil && v.web.widget != 0 {
				n++
			}
		}
	})
	return n
}

// waitHosted waits until the native widget of the view that is matches
// is a child of the view's widget, mapped at want in the surface.
func waitHosted(t *testing.T, w *window, is func(*view) bool, want driver.Bounds) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		var got driver.Bounds
		var hosted, mapped bool
		w.onLoop(t, func() {
			for _, v := range w.views {
				if v == nil || !is(v) || v.hosted() == nil || v.hosted().child == 0 {
					continue
				}
				child := v.hosted().child
				hosted = gtkWidgetGetParent(child) == v.widget
				mapped = gtkWidgetGetMapped(child) != 0
				var r grapheneRect
				if gtkWidgetComputeBounds(child, w.surface, &r) != 0 {
					got = driver.Bounds{Origin: driver.Point{X: driver.Points(r.x), Y: driver.Points(r.y)},
						Size: driver.Size{Width: driver.Points(r.w), Height: driver.Points(r.h)}}
				}
			}
		})
		if hosted && mapped && got == want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("native hosted %t, mapped %t at %v, want %v", hosted, mapped, got, want)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// waitTitle waits until the window's title satisfies ok.
func waitTitle(t *testing.T, w *window, ok func(string) bool) string {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		got := w.titleOf(t)
		if ok(got) {
			return got
		} else if time.Now().After(deadline) {
			t.Fatalf("window title = %q", got)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// aiff is half a second of 8 kHz mono silence, which GStreamer plays
// with its base and bad plug-ins alone.
func aiff() []byte {
	const rate, frames = 8000, 4000
	be := binary.BigEndian
	comm := make([]byte, 18)
	be.PutUint16(comm[0:], 1)      // channels
	be.PutUint32(comm[2:], frames) // sample frames
	be.PutUint16(comm[6:], 16)     // bits
	// 8000 as an 80-bit extended float: exponent 16383+12, mantissa
	// 8000 << (63-12).
	be.PutUint16(comm[8:], 16383+12)
	be.PutUint64(comm[10:], rate<<(63-12))
	ssnd := make([]byte, 8+frames*2)
	chunk := func(id string, data []byte) []byte {
		b := append([]byte(id), 0, 0, 0, 0)
		be.PutUint32(b[4:], uint32(len(data)))
		return append(b, data...)
	}
	body := append([]byte("AIFF"), chunk("COMM", comm)...)
	body = append(body, chunk("SSND", ssnd)...)
	return chunk("FORM", body)
}

// TestVideoReportsLoadProgressAndEnd plays a short sound through a
// Video, as NSH's Linux backend does, and reads its events.
func TestVideoReportsLoadProgressAndEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	path := filepath.Join(t.TempDir(), "silence.aiff")
	if err := os.WriteFile(path, aiff(), 0o644); err != nil {
		t.Fatal(err)
	}
	title := "nsgo-native-" + t.Name()
	app := func(cx *core.Scope) core.Node {
		state := core.NewSignal(cx, "")
		progress, ended := core.NewSignal(cx, 0), core.NewSignal(cx, 0)
		cx.Effect(func() { cx.SetTitle(fmt.Sprintf("%s|%s|%d|%d", title, state.Get(), progress.Get(), ended.Get())) })
		return core.View{Style: core.Style{Width: driver.Pt(360), Height: driver.Pt(240)}, Children: []core.Node{
			core.Video{Source: path, ProgressInterval: 50 * time.Millisecond,
				Style:   core.Style{Width: driver.Pt(200), Height: driver.Pt(100)},
				OnLoad:  func(i core.VideoInfo) { state.Set(fmt.Sprintf("load %v", i.Duration.Round(100*time.Millisecond))) },
				OnError: func(err error) { state.Set("error " + err.Error()) },
				OnProgress: func(time.Duration, time.Duration) {
					progress.Update(func(n int) int { return n + 1 })
				},
				OnEnd: func() { ended.Update(func(n int) int { return n + 1 }) }},
		}}
	}
	w := startTestWindow(t, app, Options{Title: title, Width: 360, Height: 240})
	got := waitTitle(t, w, func(s string) bool { return strings.HasSuffix(s, "|1") || strings.Contains(s, "|error ") })
	if strings.Contains(got, "No installed GStreamer plug-in") {
		// GTK plays sound through autoaudiosink, of gst-plugins-good.
		t.Skip(got)
	}
	if !strings.HasPrefix(got, title+"|load 500ms|") {
		t.Fatalf("title = %q, want a load of 500ms and an end", got)
	}
	var p, e int
	fmt.Sscanf(got[strings.LastIndex(got, "ms|")+3:], "%d|%d", &p, &e)
	if p == 0 {
		t.Errorf("title = %q: no progress while playing", got)
	}
}

// TestVideoReportsWhatGStreamerCannotPlay opens bytes that are no media.
func TestVideoReportsWhatGStreamerCannotPlay(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	path := filepath.Join(t.TempDir(), "noise.mp4")
	if err := os.WriteFile(path, []byte(strings.Repeat("not a video ", 512)), 0o644); err != nil {
		t.Fatal(err)
	}
	title := "nsgo-native-" + t.Name()
	app := func(cx *core.Scope) core.Node {
		state := core.NewSignal(cx, "")
		cx.Effect(func() { cx.SetTitle(title + "|" + state.Get()) })
		return core.Video{Source: path, Style: core.Style{Width: driver.Pt(200), Height: driver.Pt(100)},
			OnLoadStart: func() { state.Set("start") },
			OnLoad:      func(core.VideoInfo) { state.Set("load") },
			OnError:     func(err error) { state.Set("error " + err.Error()) }}
	}
	w := startTestWindow(t, app, Options{Title: title, Width: 360, Height: 240})
	got := waitTitle(t, w, func(s string) bool { return strings.Contains(s, "|error ") || strings.HasSuffix(s, "|load") })
	if !strings.Contains(got, "|error ") || strings.Contains(got, "\n") {
		t.Fatalf("title = %q, want one line of error", got)
	}
}

// TestWebViewWithoutWebKitShowsAPlaceholder runs where WebKitGTK 6.0 is
// missing, as NSH's Linux backend built without it does.
func TestWebViewWithoutWebKitShowsAPlaceholder(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	if openWebKit() == nil {
		t.Skip("WebKitGTK 6.0 is installed")
	}
	title := "nsgo-native-" + t.Name()
	app := func(cx *core.Scope) core.Node {
		cx.SetTitle(title)
		return core.View{Style: core.Style{Padding: driver.All(10)}, Children: []core.Node{
			core.WebView{Source: core.WebSrc("https://example.com"), Style: core.Style{Width: driver.Pt(300), Height: driver.Pt(200)}},
		}}
	}
	w := startTestWindow(t, app, Options{Title: title, Width: 360, Height: 240})
	waitForTitle(t, w, title)
	waitHosted(t, w, func(v *view) bool { return v.web != nil && v.web.widget == 0 },
		driver.Bounds{Origin: driver.Point{X: 10, Y: 10}, Size: driver.Size{Width: 300, Height: 200}})
}

// brokenVideo writes bytes GStreamer cannot play: GtkVideo then shows its
// black frame and an error icon.
func brokenVideo(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "broken.mp4")
	if err := os.WriteFile(path, []byte(strings.Repeat("not a video ", 512)), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// dark reports a pixel drawn by GtkVideo's black frame.
func dark(p []byte) bool { return p[3] == 0xff && p[0] < 0x30 && p[1] < 0x30 && p[2] < 0x30 }

// The tests below keep what they read in the window's top left 130 × 130
// points: a tiling window manager may give a test window little room.

// TestLaterViewsCoverHostedNatives covers part of a video with a later
// sibling, as a dialog covers one: the sibling is drawn and picked over
// the video, which still shows beside it.
func TestLaterViewsCoverHostedNatives(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	path := brokenVideo(t)
	title := "nsgo-native-" + t.Name()
	app := func(cx *core.Scope) core.Node {
		cx.SetTitle(title)
		return core.View{Style: core.Style{Width: driver.Pt(360), Height: driver.Pt(240), Background: driver.RGB(0xffffff)},
			Children: []core.Node{
				core.Video{Source: path, Controls: true, Style: core.Style{Width: driver.Pt(140), Height: driver.Pt(120)}},
				core.View{TestID: "cover", Style: core.Style{Position: core.Absolute, Left: driver.Pt(70), Top: driver.Pt(0),
					Width: driver.Pt(70), Height: driver.Pt(120), Background: driver.RGB(0xff0000)}},
			}}
	}
	w := startTestWindow(t, app, Options{Title: title, Width: 360, Height: 240})
	waitForTitle(t, w, title)
	waitPixels(t, w, "the cover red over the video, the video dark beside it", func(p pixels) bool {
		cover, video := p.at(105, 60), p.at(20, 15)
		return cover[2] == 0xff && cover[1] == 0 && cover[0] == 0 && dark(video)
	})
	// GTK gives presses there to the cover too, not to the video.
	var picked, cover uintptr
	w.onLoop(t, func() {
		picked = gtkWidgetPick(w.surface, 105, 60, gtkPickDefault)
		for _, v := range w.views {
			if v != nil && v.props.TestID == "cover" {
				cover = v.widget
			}
		}
	})
	if picked == 0 || picked != cover {
		t.Fatalf("picked %#x, want the cover %#x", picked, cover)
	}
}

// TestRoundedParentsClipHostedNatives puts a video in a rounded parent
// that clips: the parent's corners cut the video's off.
func TestRoundedParentsClipHostedNatives(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	path := brokenVideo(t)
	title := "nsgo-native-" + t.Name()
	app := func(cx *core.Scope) core.Node {
		cx.SetTitle(title)
		return core.View{Style: core.Style{Width: driver.Pt(360), Height: driver.Pt(240), Padding: driver.All(10),
			Background: driver.RGB(0xffffff)},
			Children: []core.Node{core.View{
				Style: core.Style{Width: driver.Pt(120), Height: driver.Pt(120), CornerRadius: 40, Overflow: core.Hidden},
				Children: []core.Node{core.Video{Source: path, Controls: true,
					Style: core.Style{Width: driver.Pt(120), Height: driver.Pt(120)}}},
			}}}
	}
	w := startTestWindow(t, app, Options{Title: title, Width: 360, Height: 240})
	waitForTitle(t, w, title)
	waitPixels(t, w, "the video dark inside the corners and cut off at them", func(p pixels) bool {
		corner, inside := p.at(12, 12), p.at(70, 20)
		white := corner[0] == 0xff && corner[1] == 0xff && corner[2] == 0xff
		return white && dark(inside)
	})
}

// TestWheelScrollsContentAndClipsIt turns the wheel over a scroll view:
// its content moves inside it, clipped, and the move is reported.
func TestWheelScrollsContentAndClipsIt(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	title := "nsgo-native-" + t.Name()
	app := func(cx *core.Scope) core.Node {
		offset := core.NewSignal(cx, driver.Points(0))
		cx.Effect(func() { cx.SetTitle(fmt.Sprintf("%s|%v", title, offset.Get())) })
		return core.View{Style: core.Style{Width: driver.Pt(360), Height: driver.Pt(240), Padding: driver.All(5),
			Background: driver.RGB(0xffffff)},
			Children: []core.Node{core.Scroll{HideIndicator: true, OnScroll: func(e core.ScrollEvent) { offset.Set(e.Offset.Y) },
				Style: core.Style{Width: driver.Pt(40), Height: driver.Pt(40)},
				Children: []core.Node{
					core.View{Style: core.Style{Height: driver.Pt(40), Background: driver.RGB(0xff0000)}},
					core.View{Style: core.Style{Height: driver.Pt(100), Background: driver.RGB(0x0000ff)}},
				}}}}
	}
	w := startTestWindow(t, app, Options{Title: title, Width: 360, Height: 240})
	waitForTitle(t, w, title+"|0")
	waitPixels(t, w, "red content at the top", func(p pixels) bool { return p.at(20, 10)[2] == 0xff })
	w.onLoop(t, func() { w.wheel(driver.Point{X: 20, Y: 20}, 0, 60) })
	waitForTitle(t, w, title+"|60")
	waitPixels(t, w, "blue content scrolled up, white below the scroll view", func(p pixels) bool {
		in, below := p.at(20, 10), p.at(20, 52)
		return in[0] == 0xff && in[2] == 0 && below[0] == 0xff && below[1] == 0xff && below[2] == 0xff
	})
}

// TestTouchpadGlides lifts the touchpad's fingers at a velocity: the view
// the turns moved glides on, slows down and rests, and fingers landing on
// another scroll stop it where it is.
func TestTouchpadGlides(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	title := "nsgo-native-" + t.Name()
	app := func(cx *core.Scope) core.Node {
		offset, ended := core.NewSignal(cx, driver.Points(0)), core.NewSignal(cx, 0)
		cx.Effect(func() { cx.SetTitle(fmt.Sprintf("%s|%d|%v", title, ended.Get(), offset.Get())) })
		return core.Scroll{HideIndicator: true, OnScroll: func(e core.ScrollEvent) { offset.Set(e.Offset.Y) },
			OnMomentumEnd: func(core.ScrollEvent) { ended.Update(func(n int) int { return n + 1 }) },
			Style:         core.Style{Width: driver.Pt(100), Height: driver.Pt(100)},
			Children:      []core.Node{core.View{Style: core.Style{Height: driver.Pt(100000)}}}}
	}
	w := startTestWindow(t, app, Options{Title: title, Width: 100, Height: 100})
	waitForTitle(t, w, title+"|0|0")
	glide := func(stop bool) {
		w.onLoop(t, func() {
			w.wheel(driver.Point{X: 20, Y: 20}, 0, 10)
			w.wheelGlide(0, 2)
			if stop {
				w.stopWheelGlide()
			}
		})
	}
	glide(false)
	deadline := time.Now().Add(10 * time.Second)
	for !strings.HasPrefix(w.titleOf(t), title+"|1|") {
		if time.Now().After(deadline) {
			t.Fatalf("window title = %q, want the glide ended", w.titleOf(t))
		}
		time.Sleep(10 * time.Millisecond)
	}
	var at float64
	if _, err := fmt.Sscanf(strings.TrimPrefix(w.titleOf(t), title+"|1|"), "%g", &at); err != nil || at < 200 {
		t.Fatalf("glided to %v (%v), want well past the turn's 10", at, err)
	}
	glide(true)
	waitForTitle(t, w, fmt.Sprintf("%s|2|%v", title, at+10))
	// At the end of the content it stops, with no bounce past it.
	var end driver.Points
	w.onLoop(t, func() {
		v := w.wheeled
		end = v.size.Height - v.frame.Size.Height
		w.scrollTo(v, driver.Point{Y: end - 20})
		w.wheelGlide(0, 2)
	})
	waitForTitle(t, w, fmt.Sprintf("%s|3|%v", title, end))
}

// TestWheelSettlesOnPagesWithinInsets turns the wheel over a paging
// scroll view with a content inset: it settles on the next page once the
// wheel rests, reporting the momentum, and the inset widens its offsets.
func TestWheelSettlesOnPagesWithinInsets(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	title := "nsgo-native-" + t.Name()
	app := func(cx *core.Scope) core.Node {
		offset, rested := core.NewSignal(cx, driver.Points(0)), core.NewSignal(cx, 0)
		cx.Effect(func() { cx.SetTitle(fmt.Sprintf("%s|%v|%d", title, offset.Get(), rested.Get())) })
		return core.View{Style: core.Style{Width: driver.Pt(360), Height: driver.Pt(240), Background: driver.RGB(0xffffff)},
			Children: []core.Node{core.Scroll{HideIndicator: true, ContentOffset: driver.Point{Y: -10},
				Behavior:      driver.ScrollBehavior{Paging: true, ContentInset: driver.Edges{Top: 10}},
				OnScroll:      func(e core.ScrollEvent) { offset.Set(e.Offset.Y) },
				OnMomentumEnd: func(core.ScrollEvent) { rested.Update(func(n int) int { return n + 1 }) },
				Style:         core.Style{Width: driver.Pt(100), Height: driver.Pt(100)},
				Children:      []core.Node{core.View{Style: core.Style{Height: driver.Pt(400), Background: driver.RGB(0x0000ff)}}}}}}
	}
	w := startTestWindow(t, app, Options{Title: title, Width: 360, Height: 240})
	waitForTitle(t, w, title+"|-10|0")
	w.onLoop(t, func() { w.wheel(driver.Point{X: 20, Y: 20}, 0, 30) })
	waitForTitle(t, w, title+"|100|1")
}

// TestTouchAreasDecideWhatTakesAPress presses a menu outside its parent,
// a small target by its hit slop and a button under a veil that takes no
// pointer.
func TestTouchAreasDecideWhatTakesAPress(t *testing.T) {
	w := startTestWindow(t, touchAreasApp("areas"), Options{Title: "areas|start", Width: 320, Height: 200})
	waitForTitle(t, w, "areas|none")
	for _, c := range []struct {
		at   driver.Point
		want string
	}{{driver.Point{X: 60, Y: 60}, "menu"}, {driver.Point{X: 225, Y: 25}, "slop"}, {driver.Point{X: 200, Y: 140}, "under"}} {
		w.onLoop(t, func() { w.press(c.at) })
		waitForTitle(t, w, "areas|"+c.want)
	}
}

// TestLongPressWaitsForTheViewsDelay holds a press on a view whose long
// press takes 300 ms: it becomes a long press and its release presses
// nothing, while a quick press stays a press.
func TestLongPressWaitsForTheViewsDelay(t *testing.T) {
	app := func(cx *core.Scope) core.Node {
		last := core.NewSignal(cx, "none")
		cx.Effect(func() { cx.SetTitle("long|" + last.Get()) })
		return core.View{Style: core.Style{Width: driver.Pt(320), Height: driver.Pt(200)}, Children: []core.Node{
			core.View{DelayLongPress: 300 * time.Millisecond, Style: core.Style{Width: driver.Pt(100), Height: driver.Pt(100)},
				OnPress:     func(core.PressEvent) { last.Set("press") },
				OnLongPress: func(core.PressEvent) { last.Set("long") }},
		}}
	}
	w := startTestWindow(t, app, Options{Title: "long|start", Width: 320, Height: 200})
	waitForTitle(t, w, "long|none")
	at := driver.Point{X: 50, Y: 50}
	w.onLoop(t, func() { w.pointerDown(at, 1, 0, 0, 1) })
	waitForTitle(t, w, "long|long")
	w.onLoop(t, func() { w.pointerUp(at) })
	time.Sleep(100 * time.Millisecond)
	waitForTitle(t, w, "long|long")
	w.onLoop(t, func() { w.pointerDown(at, 1, 0, 0, 1); w.pointerUp(at) })
	waitForTitle(t, w, "long|press")
}

// TestTheResponderFollowsADrag drags on a view that takes the touches as
// they start: it is granted them, follows the move and is released.
func TestTheResponderFollowsADrag(t *testing.T) {
	w := startTestWindow(t, responderApp("responder"), Options{Title: "responder|start", Width: 320, Height: 200})
	waitForTitle(t, w, "responder|")
	w.onLoop(t, func() { w.pointerDown(driver.Point{X: 20, Y: 20}, 1, 0, 0, 1) })
	waitForTitle(t, w, "responder|grant 0,0")
	w.onLoop(t, func() { w.movePointer(driver.Point{X: 50, Y: 30}, gdkButton1) })
	waitForTitle(t, w, "responder|move 30,10")
	w.onLoop(t, func() { w.pointerUp(driver.Point{X: 50, Y: 30}) })
	waitForTitle(t, w, "responder|release 30,10")
}

// responderApp is a view that takes the touches as they start and logs
// its responder's events in the title.
func responderApp(title string) func(*core.Scope) core.Node {
	return func(cx *core.Scope) core.Node {
		log := core.NewSignal(cx, "")
		cx.Effect(func() { cx.SetTitle(title + "|" + log.Get()) })
		say := func(what string) func(core.ResponderEvent) {
			return func(e core.ResponderEvent) {
				log.Set(fmt.Sprintf("%s %.0f,%.0f", what, e.Gesture.Delta.X, e.Gesture.Delta.Y))
			}
		}
		return core.View{Style: core.Style{Width: driver.Pt(320), Height: driver.Pt(200), Background: driver.RGB(0xffffff)},
			Children: []core.Node{core.View{Style: core.Style{Width: driver.Pt(100), Height: driver.Pt(100)},
				Responder: &core.Responder{Claims: []core.TouchClaim{{}}, OnGrant: say("grant"), OnMove: say("move"),
					OnRelease: say("release")}}}}
	}
}

func touchAreasApp(title string) func(*core.Scope) core.Node {
	return func(cx *core.Scope) core.Node {
		last := core.NewSignal(cx, "none")
		cx.Effect(func() { cx.SetTitle(title + "|" + last.Get()) })
		press := func(name string) func(core.PressEvent) { return func(core.PressEvent) { last.Set(name) } }
		return core.View{Style: core.Style{Width: driver.Pt(320), Height: driver.Pt(200), Background: driver.RGB(0xffffff)},
			Children: []core.Node{
				// A menu 30 points below its field, outside it.
				core.View{Style: core.Style{Position: core.Absolute, Left: driver.Pt(10), Top: driver.Pt(10), Width: driver.Pt(100),
					Height: driver.Pt(30)}, Children: []core.Node{
					core.View{OnPress: press("menu"), Style: core.Style{Position: core.Absolute, Top: driver.Pt(30),
						Width: driver.Pt(100), Height: driver.Pt(40), Background: driver.RGB(0x123456)}},
				}},
				// A 10-point target with 20 points of slop.
				core.View{OnPress: press("slop"), HitSlop: driver.All(20), Style: core.Style{Position: core.Absolute,
					Left: driver.Pt(200), Top: driver.Pt(20), Width: driver.Pt(10), Height: driver.Pt(10), Background: driver.RGB(0x123456)}},
				// A veil over a button lets its presses through.
				core.View{OnPress: press("under"), Style: core.Style{Position: core.Absolute, Left: driver.Pt(150),
					Top: driver.Pt(120), Width: driver.Pt(100), Height: driver.Pt(40), Background: driver.RGB(0x123456)}},
				core.View{OnPress: press("veil"), PointerEvents: driver.PointerEventsNone, Style: core.Style{Position: core.Absolute,
					Left: driver.Pt(140), Top: driver.Pt(110), Width: driver.Pt(120), Height: driver.Pt(60)}},
			}}
	}
}

// TestPointerMovesReportPositions moves the pointer over a view that
// follows moves: OnMouseMove hears it in the view and the surface.
func TestPointerMovesReportPositions(t *testing.T) {
	app := func(cx *core.Scope) core.Node {
		hover := core.NewSignal(cx, core.MouseEvent{})
		cx.Effect(func() {
			h := hover.Get()
			cx.SetTitle(fmt.Sprintf("moves|%.0f,%.0f|%.0f,%.0f", h.Position.X, h.Position.Y,
				h.PagePosition.X, h.PagePosition.Y))
		})
		return core.View{Style: core.Style{Width: driver.Pt(320), Height: driver.Pt(200), Padding: driver.All(10)},
			Children: []core.Node{
				core.View{Style: core.Style{Width: driver.Pt(100), Height: driver.Pt(80)},
					OnMouseMove: hover.Set},
			}}
	}
	w := startTestWindow(t, app, Options{Title: "moves|start", Width: 320, Height: 200})
	waitForTitle(t, w, "moves|0,0|0,0")
	w.onLoop(t, func() { motionCallback(0, 40, 30, w.handle) })
	waitForTitle(t, w, "moves|30,20|40,30")
}

// TestHoverFollowsThePointer moves the pointer onto a view and its
// child, off them and out of the window, as GTK's motion controller
// reports it, and checks the hover state style shows while it is over.
func TestHoverFollowsThePointer(t *testing.T) {
	app := func(cx *core.Scope) core.Node {
		in, out := core.NewSignal(cx, 0), core.NewSignal(cx, 0)
		cx.Effect(func() { cx.SetTitle(fmt.Sprintf("hover|%d|%d", in.Get(), out.Get())) })
		return core.View{Style: core.Style{Width: driver.Pt(320), Height: driver.Pt(200), Padding: driver.All(10)},
			Children: []core.Node{
				core.View{TestID: "target", Style: core.Style{Width: driver.Pt(100), Height: driver.Pt(80), Padding: driver.All(20),
					Background: driver.RGB(0x123456), Hover: &core.Style{Background: driver.RGB(0x654321)}},
					OnMouseEnter: func(core.MouseEvent) { in.Update(func(n int) int { return n + 1 }) },
					OnMouseLeave: func(core.MouseEvent) { out.Update(func(n int) int { return n + 1 }) },
					Children:     []core.Node{core.View{Style: core.Style{Width: driver.Pt(20), Height: driver.Pt(20)}}}},
			}}
	}
	w := startTestWindow(t, app, Options{Title: "hover|0|0", Width: 320, Height: 200})
	waitForTitle(t, w, "hover|0|0")
	move := func(x, y float64) { w.onLoop(t, func() { motionCallback(0, x, y, w.handle) }) }
	background := func() (c driver.Color) {
		w.onLoop(t, func() {
			for _, v := range w.views {
				if v != nil && v.props.TestID == "target" {
					c = v.style.Background
				}
			}
		})
		return c
	}
	move(15, 15)
	waitForTitle(t, w, "hover|1|0")
	// Over its child, the view stays hovered.
	move(40, 40)
	time.Sleep(50 * time.Millisecond)
	waitForTitle(t, w, "hover|1|0")
	deadline := time.Now().Add(5 * time.Second)
	for background() != driver.RGB(0x654321) {
		if time.Now().After(deadline) {
			t.Fatalf("background = %x, want the hover style's", background())
		}
		time.Sleep(10 * time.Millisecond)
	}
	move(200, 150)
	waitForTitle(t, w, "hover|1|1")
	move(15, 15)
	waitForTitle(t, w, "hover|2|1")
	w.onLoop(t, func() { leaveCallback(0, w.handle) })
	waitForTitle(t, w, "hover|2|2")
}

// TestStickyListRowsStayPinned turns the wheel over a list whose first
// row sticks: the row stays at the top, moved natively from its frame.
func TestStickyListRowsStayPinned(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	title := "nsgo-native-" + t.Name()
	app := func(cx *core.Scope) core.Node {
		offset := core.NewSignal(cx, driver.Points(0))
		cx.Effect(func() { cx.SetTitle(fmt.Sprintf("%s|%v", title, offset.Get())) })
		return core.View{Style: core.Style{Width: driver.Pt(360), Height: driver.Pt(240), Padding: driver.All(5),
			Background: driver.RGB(0xffffff)},
			Children: []core.Node{core.List{HideIndicator: true, OnScroll: func(e core.ScrollEvent) { offset.Set(e.Offset.Y) },
				Count: func() int { return 20 }, StickyHeaderIndices: []int{0}, EstimatedSize: 20,
				Style: core.Style{Width: driver.Pt(40), Height: driver.Pt(100)},
				Row: func(cx *core.Scope, r core.ListRow) core.Node {
					return core.View{StyleFunc: func() core.Style {
						if r.Index() == 0 {
							return core.Style{Height: driver.Pt(20), Background: driver.RGB(0xff0000)}
						}
						return core.Style{Height: driver.Pt(20), Background: driver.RGB(0x0000ff)}
					}}
				}}}}
	}
	w := startTestWindow(t, app, Options{Title: title, Width: 360, Height: 240})
	waitForTitle(t, w, title+"|0")
	w.onLoop(t, func() { w.wheel(driver.Point{X: 20, Y: 20}, 0, 60) })
	waitForTitle(t, w, title+"|60")
	waitPixels(t, w, "the sticky row at the top over blue rows", func(p pixels) bool {
		top, below := p.at(20, 12), p.at(20, 40)
		return top[2] == 0xff && top[0] == 0 && below[0] == 0xff && below[2] == 0
	})
}
