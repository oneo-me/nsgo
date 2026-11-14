//go:build nsgodev

package nsgo

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/devtools"
	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/engine"
)

// devDevtools is "1" in nsgo dev -devtools's builds, set with -ldflags -X.
var devDevtools = ""

// devCommand is what the DevTools page asks the app: an op on a surface, with what it needs.
type devCommand struct {
	Op      string `json:"op"`
	Surface int    `json:"surface"`
	ID      uint32 `json:"id"`
	On      bool   `json:"on"`
	Signal  uint64 `json:"signal"`
	Value   string `json:"value"`
	N       int    `json:"n"`
}

// devMessage is what the app tells the DevTools page.
type devMessage struct {
	Type    string                 `json:"type"`
	Surface int                    `json:"surface"`
	ID      uint32                 `json:"id,omitempty"`
	Error   string                 `json:"error,omitempty"`
	Data    any                    `json:"data,omitempty"`
	Commit  *devCommit             `json:"commit,omitempty"`
	Step    *devtools.Step         `json:"step,omitempty"`
	Hot     []devtools.Computation `json:"hot,omitempty"`
}

// maxUnapplied bounds the batches waiting for the platform that DevTools remember: a platform that applies none, as Android's Java runtime reports none, keeps no more.
const maxUnapplied = 1000

// devCommit is a commit as the profiler shows it.
type devCommit struct {
	N      int            `json:"n"`
	Items  int            `json:"items"`
	Millis float64        `json:"ms"`
	Causes *driver.Causes `json:"causes,omitempty"`
}

// startDevTools has core record what DevTools show, sends the page each commit, and follows the page's commands from the server's events. It runs before any graph starts.
func startDevTools(server string) {
	devtools.Enabled = true
	driver.TraceCauses = true
	out := &devOut{server: server, wake: make(chan struct{}, 1)}
	go out.run()
	commits := 0
	var mu sync.Mutex
	// sent are the batches the platform has not applied yet, by commit number, so its time joins the commit's row.
	sent := map[*driver.MountBatch]int{}
	prev := engine.Trace
	engine.Trace = func(b *driver.MountBatch, took time.Duration) {
		if prev != nil {
			prev(b, took)
		}
		mu.Lock()
		commits++
		n := commits
		if len(sent) < maxUnapplied {
			sent[b] = n
		}
		mu.Unlock()
		out.send(devMessage{Type: "commit", Commit: &devCommit{N: n, Items: len(b.Items), Millis: float64(took.Microseconds()) / 1000, Causes: b.Causes}})
	}
	driver.Applied = func(b *driver.MountBatch, took time.Duration) {
		mu.Lock()
		n, ok := sent[b]
		delete(sent, b)
		mu.Unlock()
		if ok {
			out.send(devMessage{Type: "applied", ID: uint32(n), Data: float64(took.Microseconds()) / 1000})
		}
	}
	go listenDevTools(server, out)
}

// listenDevTools follows the server's events for DevTools' commands, reconnecting while it runs.
func listenDevTools(server string, out *devOut) {
	for {
		resp, err := http.Get(server + "/_nsgo/events")
		if err == nil {
			sc := bufio.NewScanner(resp.Body)
			sc.Buffer(make([]byte, 1<<16), 16<<20)
			for sc.Scan() {
				if raw, ok := strings.CutPrefix(sc.Text(), "data: devtools "); ok {
					var c devCommand
					if json.Unmarshal([]byte(raw), &c) == nil {
						runDevCommand(c, out)
					}
				}
			}
			resp.Body.Close()
		}
		time.Sleep(time.Second)
	}
}

// runDevCommand runs c on its surface's graph and sends the page what it answers.
func runDevCommand(c devCommand, out *devOut) {
	if c.Op == "surfaces" {
		var ids []int
		for _, s := range devtools.Surfaces() {
			ids = append(ids, s.ID())
		}
		out.send(devMessage{Type: "surfaces", Data: ids})
		return
	}
	var s devtools.Surface
	for _, x := range devtools.Surfaces() {
		if x.ID() == c.Surface {
			s = x
		}
	}
	if s == nil {
		out.send(devMessage{Type: "error", Surface: c.Surface, Error: "no surface " + strconv.Itoa(c.Surface)})
		return
	}
	reply := func(m devMessage) {
		m.Surface = c.Surface
		out.send(m)
	}
	s.Post(func() {
		switch c.Op {
		case "elements":
			reply(devMessage{Type: "elements", Data: s.Elements()})
		case "owners":
			reply(devMessage{Type: "owners", Data: s.Owners()})
		case "inspect":
			s.Inspect(c.ID)
		case "highlight":
			s.Highlight(c.On)
		case "pick":
			s.Pick(c.On, func(id uint32) { reply(devMessage{Type: "hover", ID: id}) }, func(id uint32) { reply(devMessage{Type: "picked", ID: id}) })
		case "record":
			s.Record(c.On, func(step devtools.Step) { reply(devMessage{Type: "step", Step: &step}) })
		case "set":
			m := devMessage{Type: "set"}
			if err := s.Set(c.Signal, c.Value); err != nil {
				m.Error = err.Error()
			}
			reply(m)
		case "hot":
			reply(devMessage{Type: "hot", Hot: s.Hot(max(c.N, 1))})
		}
	})
}

// devOut sends the page messages: one goroutine posts what gathered while the last post was on its way, so the graph never waits for the network.
type devOut struct {
	server  string
	mu      sync.Mutex
	pending bytes.Buffer
	wake    chan struct{}
}

func (o *devOut) send(m devMessage) {
	data, err := json.Marshal(m)
	if err != nil {
		return
	}
	o.mu.Lock()
	o.pending.Write(data)
	o.pending.WriteByte('\n')
	o.mu.Unlock()
	select {
	case o.wake <- struct{}{}:
	default:
	}
}

func (o *devOut) run() {
	for range o.wake {
		// Commits come every frame: a quarter of a second's go in one post.
		time.Sleep(250 * time.Millisecond)
		o.mu.Lock()
		body := bytes.Clone(o.pending.Bytes())
		o.pending.Reset()
		o.mu.Unlock()
		if len(body) == 0 {
			continue
		}
		if resp, err := http.Post(o.server+"/_nsgo/devtools/out", "application/x-ndjson", bytes.NewReader(body)); err == nil {
			resp.Body.Close()
		}
	}
}

// withDevOverlay mounts DevTools' overlay over app, outlining commits from the start under -highlight.
func withDevOverlay(app App, highlight bool) App {
	return func(cx *core.Scope) core.Node {
		cx.Post(func() {
			for _, s := range devtools.Surfaces() {
				s.Highlight(highlight)
			}
		})
		return core.Group{core.Func(app), core.UpdateHighlights{}}
	}
}
