//go:build js && wasm && nsgotest

package web

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"sync"
	"syscall/js"
	"time"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/testhost"
)

// The test host: nsgo test -target web runs each test in a page of a
// headless browser it drives over the DevTools protocol. The page asks it
// for trusted input through the nsgoTest binding, which a script of the
// tool answers with nsgoTestReply.

func init() { testhost.Register(host{}) }

type host struct{}

func (host) Main(run func() int) {
	go func() { os.Exit(run()) }()
	select {}
}

func (host) Open(app func(cx *core.Scope) core.Node, o testhost.Options) (testhost.Surface, error) {
	if js.Global().Get("nsgoTest").Type() != js.TypeFunction {
		return nil, errors.New("the page has no nsgoTest binding: run the tests with nsgo test -target web")
	}
	t := &testSurface{}
	if o.Size.Width > 0 {
		if _, err := t.ask(map[string]any{"op": "viewport", "width": o.Size.Width, "height": o.Size.Height}); err != nil {
			return nil, err
		}
	}
	test := &testOpen{options: o}
	t.s, t.test = open(app, Options{}, test), test
	return t, nil
}

type testSurface struct {
	s    *surface
	test *testOpen
}

var (
	replyOnce sync.Once
	replyMu   sync.Mutex
	replies   = map[int]chan reply{}
	asked     int
)

type reply struct {
	Error string          `json:"error"`
	Data  json.RawMessage `json:"data"`
}

// ask has the tool do op and waits for its answer.
func (t *testSurface) ask(op map[string]any) (json.RawMessage, error) {
	replyOnce.Do(func() {
		js.Global().Set("nsgoTestReply", js.FuncOf(func(_ js.Value, args []js.Value) any {
			var r reply
			json.Unmarshal([]byte(args[1].String()), &r)
			replyMu.Lock()
			ch := replies[args[0].Int()]
			delete(replies, args[0].Int())
			replyMu.Unlock()
			if ch != nil {
				ch <- r
			}
			return nil
		}))
	})
	replyMu.Lock()
	asked++
	id := asked
	ch := make(chan reply, 1)
	replies[id] = ch
	replyMu.Unlock()
	op["id"] = id
	data, _ := json.Marshal(op)
	js.Global().Call("nsgoTest", string(data))
	select {
	case r := <-ch:
		if r.Error != "" {
			return nil, errors.New(r.Error)
		}
		return r.Data, nil
	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("the tool did not answer %v", op["op"])
	}
}

// page is p, in the surface's points, in the page's CSS pixels.
func (t *testSurface) page(p driver.Point) map[string]any {
	r := t.s.host.Call("getBoundingClientRect")
	return map[string]any{"x": r.Get("left").Float() + float64(p.X), "y": r.Get("top").Float() + float64(p.Y)}
}

func with(m map[string]any, kv ...any) map[string]any {
	for i := 0; i+1 < len(kv); i += 2 {
		m[kv[i].(string)] = kv[i+1]
	}
	return m
}

func (t *testSurface) do(op map[string]any) error {
	_, err := t.ask(op)
	return err
}

func (t *testSurface) Press(at driver.Point, hold time.Duration) error {
	return t.do(with(t.page(at), "op", "press", "hold", hold.Milliseconds()))
}

func (t *testSurface) RightClick(at driver.Point) error {
	return t.do(with(t.page(at), "op", "rightClick"))
}

func (t *testSurface) Drag(path []driver.Point) error {
	var pts []map[string]any
	for _, p := range path {
		pts = append(pts, t.page(p))
	}
	return t.do(map[string]any{"op": "drag", "path": pts})
}

func (t *testSurface) Scroll(at driver.Point, delta driver.Point) error {
	return t.do(with(t.page(at), "op", "scroll", "dx", delta.X, "dy", delta.Y))
}

func (t *testSurface) Hover(at driver.Point) error {
	return t.do(with(t.page(at), "op", "hover"))
}

func (t *testSurface) Type(text string) error {
	return t.do(map[string]any{"op": "type", "text": text})
}

func (t *testSurface) Key(key string) error { return t.do(map[string]any{"op": "key", "key": key}) }

// Back is Escape, which the page takes as the back key.
func (t *testSurface) Back() error { return t.Key("escape") }

func (t *testSurface) Answer(id uint32, index int) error {
	for _, a := range t.s.alerts {
		if a.id != id {
			continue
		}
		if index < 0 {
			return t.Key("escape")
		}
		for at, i := range a.order {
			if i == index && at < len(a.buttons) {
				r := a.buttons[at].Call("getBoundingClientRect")
				x, y := r.Get("left").Float()+r.Get("width").Float()/2, r.Get("top").Float()+r.Get("height").Float()/2
				return t.do(map[string]any{"op": "press", "x": x, "y": y, "hold": 0})
			}
		}
		return fmt.Errorf("the alert has no button %d", index)
	}
	return fmt.Errorf("no alert %d shows", id)
}

func (t *testSurface) Screenshot() ([]byte, error) {
	data, err := t.ask(map[string]any{"op": "screenshot"})
	if err != nil {
		return nil, err
	}
	var b64 string
	if err := json.Unmarshal(data, &b64); err != nil {
		return nil, err
	}
	return base64.StdEncoding.DecodeString(b64)
}

// Animating counts the page's own finite animations too: transitions
// and a screen sliding in.
func (t *testSurface) Sync() { t.s.engine.Sync() }

func (t *testSurface) Animating() bool {
	if t.s.animator.Ending() {
		return true
	}
	running := js.Global().Get("document").Call("getAnimations")
	for i := range running.Length() {
		a := running.Index(i)
		if a.Get("playState").String() != "running" {
			continue
		}
		if effect := a.Get("effect"); effect.Truthy() {
			if n := effect.Call("getComputedTiming").Get("iterations"); n.Type() == js.TypeNumber && !math.IsInf(n.Float(), 0) {
				return true
			}
		}
	}
	return false
}

func (t *testSurface) Resize(size driver.Size) error {
	return t.do(map[string]any{"op": "viewport", "width": size.Width, "height": size.Height})
}

func (t *testSurface) SetEnvironment(env driver.Environment) error {
	t.test.pin.Set(&env)
	t.s.engine.Repin()
	return nil
}

// ChooseMenu implements testhost.Surface: no native menu bar shows here.
func (t *testSurface) ChooseMenu(uint32) error { return testhost.ErrNoMenu }

// ChoosePopup implements testhost.Surface: the core shows no native
// context menus here.
func (t *testSurface) ChoosePopup(uint32) error { return testhost.ErrNoMenu }

// Several implements testhost.Host: it shows one app at a time.
func (host) Several() bool { return false }
