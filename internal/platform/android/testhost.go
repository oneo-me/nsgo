//go:build android && nsgotest

package android

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/testhost"
)

// The test host: nsgo test -target android starts the app's activity for
// each test, the test's arguments and environment in its intent. The
// runtime's test agent (TestAgent.java) acts on the UI thread with
// MotionEvents and KeyEvents, asked over the pipe. The process prints to
// standard error, which the runtime logs, and says how it exits there.

func init() { testhost.Register(&host{}) }

type host struct {
	s    *surface
	test *testOpen
}

func (h *host) Main(run func() int) {
	h.s = connect(Options{})
	h.test = &testOpen{size: make(chan driver.Size, 1)}
	// The device's log keeps earlier runs: the tool reads from the start
	// of this one, which its token marks.
	token := os.Getenv("NSGO_TEST_RUN")
	fmt.Fprintf(os.Stderr, "nsgo-test-start %s\n", token)
	go func() {
		code := run()
		fmt.Fprintf(os.Stderr, "nsgo-test-exit %s %d\n", token, code)
		os.Exit(code)
	}()
	h.s.serve(nil, Options{}, h.test)
	os.Exit(1)
}

func (h *host) Open(app func(cx *core.Scope) core.Node, o testhost.Options) (testhost.Surface, error) {
	h.test.options = o
	var size driver.Size
	select {
	case size = <-h.test.size:
	case <-time.After(30 * time.Second):
		return nil, errors.New("the runtime did not start")
	}
	t := &testSurface{s: h.s, test: h.test, replies: map[uint32]chan agentReply{}}
	h.s.testReply = t.reply
	h.s.startEngine(app, size, 0, h.test)
	return t, nil
}

type testSurface struct {
	s    *surface
	test *testOpen

	mu        sync.Mutex
	next      uint32
	replies   map[uint32]chan agentReply
	signature string
}

type agentReply struct{ err, data string }

// reply takes the agent's answer, on the pipe's reader.
func (t *testSurface) reply(r *reader) {
	id, err, data := r.u32(), r.str(), r.str()
	t.mu.Lock()
	ch := t.replies[id]
	delete(t.replies, id)
	t.mu.Unlock()
	if ch != nil {
		ch <- agentReply{err, data}
	}
}

// ask has the agent take op and returns its data.
func (t *testSurface) ask(op map[string]any) (string, error) {
	t.mu.Lock()
	t.next++
	id := t.next
	ch := make(chan agentReply, 1)
	t.replies[id] = ch
	t.mu.Unlock()
	data, _ := json.Marshal(op)
	t.s.send(msgTest, func(w *writer) {
		w.u32(id)
		w.str(string(data))
	})
	select {
	case r := <-ch:
		if r.err != "" {
			return "", errors.New(r.err)
		}
		return r.data, nil
	case <-time.After(30 * time.Second):
		return "", fmt.Errorf("the test agent did not answer %v", op["op"])
	}
}

func (t *testSurface) do(op map[string]any) error {
	_, err := t.ask(op)
	return err
}

func (t *testSurface) Press(at driver.Point, hold time.Duration) error {
	return t.do(map[string]any{"op": "press", "x": at.X, "y": at.Y, "hold": hold.Milliseconds()})
}

func (t *testSurface) RightClick(driver.Point) error {
	return errors.New("a phone has no secondary click: use LongPress")
}

func (t *testSurface) Drag(path []driver.Point) error {
	return t.do(map[string]any{"op": "drag", "path": path})
}

func (t *testSurface) Scroll(at driver.Point, delta driver.Point) error {
	return t.do(map[string]any{"op": "scroll", "x": at.X, "y": at.Y, "dx": delta.X, "dy": delta.Y})
}

func (t *testSurface) Hover(driver.Point) error { return nil }

func (t *testSurface) Type(text string) error {
	return t.do(map[string]any{"op": "type", "text": text})
}

func (t *testSurface) Key(key string) error { return t.do(map[string]any{"op": "key", "key": key}) }

func (t *testSurface) Back() error { return t.do(map[string]any{"op": "back"}) }

func (t *testSurface) Answer(id uint32, index int) error {
	return t.do(map[string]any{"op": "answer", "id": id, "index": index})
}

func (t *testSurface) Screenshot() ([]byte, error) {
	data, err := t.ask(map[string]any{"op": "screenshot"})
	if err != nil {
		return nil, err
	}
	return base64.StdEncoding.DecodeString(data)
}

// Animating compares where every view is, how far it scrolled and how
// opaque it is with the last look: anything that moved animates.
func (t *testSurface) Sync() { t.s.engine.Sync() }

func (t *testSurface) Animating() bool {
	sig, err := t.ask(map[string]any{"op": "signature"})
	if err != nil {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	moved := sig != t.signature
	t.signature = sig
	return moved
}

// Resize does nothing: a phone's screen is the size it is.
func (t *testSurface) Resize(driver.Size) error { return nil }

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
func (*host) Several() bool { return false }
