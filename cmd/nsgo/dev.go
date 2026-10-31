package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// session is how dev builds and shows one target. start runs after each
// successful build and returns the running app.
type session struct {
	build func(ctx context.Context, p *project) error
	start func(ctx context.Context, p *project) (app, error)
}

// app is one started dev app. stop ends it, and wait returns once it has
// exited, reporting a failure. The web target only serves, so its wait is
// nil.
type app struct {
	stop func()
	wait func(context.Context) error
}

// devOptions are nsgo dev's flags that do not name a value in the project.
// The rest of the flags have replaced nsgo.yaml's values already, and apply
// replaces them again after each reload.
type devOptions struct {
	apply   func(*project)
	port    int    // the dev server's, which the web target is served from
	open    bool   // whether to open the web target in the browser
	device  string // the iPhone, simulator or Android device to run on
	team    string // the Apple developer team the iOS target signs with
	release bool   // build as nsgo build does, for measuring
	trace   bool   // print the running app's commits
	server  *devServer
}

// dev builds and runs the app, with a dev server its builds connect to:
// a failed build shows over the app, and Command or Control+R in the app,
// or r and Return in the terminal, builds and starts it again. Without
// nsgo.yaml's hot reload it exits once the app does; with it, it rebuilds
// and restarts the app whenever a Go file it compiles from, go.mod or
// nsgo.yaml changes, and keeps the running app when a build fails.
func dev(ctx context.Context, p *project, opts devOptions) error {
	p.prepareDev(opts)
	srv, err := startDevServer(opts.port)
	if err != nil {
		return err
	}
	defer srv.close()
	opts.server = srv
	s, err := sessionFor(p, opts)
	if err != nil {
		return err
	}
	go readReloadKey(ctx, srv)

	stop := func() {}
	defer func() { stop() }()
	header(p.Name, p.Version, "dev "+p.Dev.Target)
	var exited chan error
	first, hinted := true, false
	for {
		dirs, err := sourceDirs(ctx, p.Dir)
		if err != nil {
			return err
		}
		if p.Web.Host != "" {
			dirs = append(dirs, p.resolve(p.Web.Host))
		}
		fingerprint := scan(dirs)
		progress := begin("building %s", p.Dev.Target)
		err = s.build(ctx, p)
		switch {
		case ctx.Err() != nil:
			progress.fail()
			return nil
		case err != nil && first && !p.hotReload():
			progress.fail()
			return err
		case err != nil:
			progress.fail()
			srv.send("error", strings.TrimSpace(err.Error()))
			report(err)
			detail("waiting for changes (r and Return builds again)")
		default:
			progress.done(fmt.Sprintf("built %s", p.Dev.Target), "")
			stop()
			stop, exited = func() {}, nil
			if next, err := s.start(ctx, p); err != nil {
				fail("run failed: %v", err)
			} else {
				stop = next.stop
				if next.wait != nil {
					exited = make(chan error, 1)
					go func(wait func(context.Context) error, done chan error) { done <- wait(ctx) }(next.wait, exited)
				}
				if p.Dev.Target != "web" {
					event("running")
				}
				if !hinted {
					if p.hotReload() {
						detail("watching for changes (r and Return builds again)")
					} else {
						detail("r and Return builds again")
					}
					hinted = true
				}
			}
			srv.send("ok", "")
			srv.send("reload", "")
		}
		first = false
		if reason := waitForChange(ctx, p, srv, dirs, fingerprint, exited); reason != nil {
			return *reason
		}
		// The flags and the rest of nsgo.yaml may have changed too.
		if next, err := load(p.Dir); err == nil {
			opts.apply(next)
			next.prepareDev(opts)
			*p = *next
		} else {
			warn("%v", err)
		}
	}
}

// prepareDev sets what dev's builds carry: the monitor dev.fps asks for,
// the dev client, -trace's commits and the dev icons; -release strips
// them as nsgo build does, keeping the monitor for measuring.
func (p *project) prepareDev(opts devOptions) {
	p.Perf, p.Release, p.Dev.Client, p.Dev.Trace = p.Dev.FPS, opts.release, true, opts.trace
	p.useDevIcons()
}

// waitForChange returns nil once dev should build again: a reload asked
// for, or with hot reload a watched file changed. It returns the error
// dev ends with when the run is over: ctx done, or the app exited when it
// does not hot reload.
func waitForChange(ctx context.Context, p *project, srv *devServer, dirs []string, fingerprint string, exited chan error) *error {
	tick := time.NewTicker(300 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			var none error
			return &none
		case <-srv.reloads:
			return nil
		case err := <-exited:
			if !p.hotReload() {
				return &err
			}
			exited = nil
		case <-tick.C:
			if p.hotReload() && scan(dirs) != fingerprint {
				// Let the editor finish writing.
				time.Sleep(100 * time.Millisecond)
				return nil
			}
		}
	}
}

// readReloadKey asks for a new build when the terminal sends r and
// Return.
func readReloadKey(ctx context.Context, srv *devServer) {
	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() && ctx.Err() == nil {
		if strings.TrimSpace(sc.Text()) == "r" {
			srv.reload()
		}
	}
}

// devServer is what dev's builds connect to: it streams build results to
// the apps and pages it started, takes their reload requests and the
// commits -trace prints, and serves the web target. It listens on every interface, so a phone on the LAN
// reaches it.
type devServer struct {
	port    int
	srv     *http.Server
	mux     *http.ServeMux
	reloads chan struct{}

	mu      sync.Mutex
	clients map[chan string]bool
	last    string // the last error event, for apps that connect later
	files   string
}

// startDevServer listens on port, or the next free one after it; 0 takes
// any.
func startDevServer(port int) (*devServer, error) {
	var ln net.Listener
	var err error
	for p := port; p <= port+100; p++ {
		if ln, err = net.Listen("tcp", fmt.Sprintf(":%d", p)); err == nil || port == 0 {
			break
		}
	}
	if err != nil {
		return nil, fmt.Errorf("no free port from %d: %w", port, err)
	}
	s := &devServer{port: ln.Addr().(*net.TCPAddr).Port, mux: http.NewServeMux(), reloads: make(chan struct{}, 1),
		clients: map[chan string]bool{}}
	s.mux.HandleFunc("/_nsgo/events", s.events)
	s.mux.HandleFunc("/_nsgo/reload", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST to reload", http.StatusMethodNotAllowed)
			return
		}
		s.reload()
	})
	// A traced app's commits, printed as the app's output is.
	s.mux.HandleFunc("/_nsgo/trace", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST a trace", http.StatusMethodNotAllowed)
			return
		}
		io.Copy(&appConsole, r.Body)
	})
	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		files := s.files
		s.mu.Unlock()
		if files == "" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		http.FileServer(http.Dir(files)).ServeHTTP(w, r)
	})
	s.srv = &http.Server{Handler: s.mux}
	go s.srv.Serve(ln)
	return s, nil
}

func (s *devServer) close() { s.srv.Close() }

func (s *devServer) reload() {
	select {
	case s.reloads <- struct{}{}:
	default:
	}
}

// serve serves dir at the server's root, for the web target.
func (s *devServer) serve(dir string) {
	s.mu.Lock()
	s.files = dir
	s.mu.Unlock()
}

// url is the server's address on this machine.
func (s *devServer) url() string { return fmt.Sprintf("http://127.0.0.1:%d", s.port) }

// lanURLs are the server's addresses on the machine's networks.
func (s *devServer) lanURLs() []string {
	var out []string
	addrs, _ := net.InterfaceAddrs()
	for _, a := range addrs {
		if ip, ok := a.(*net.IPNet); ok && !ip.IP.IsLoopback() && ip.IP.To4() != nil && ip.IP.IsPrivate() {
			out = append(out, fmt.Sprintf("http://%s:%d", ip.IP, s.port))
		}
	}
	return out
}

// send tells every client kind: "error" with a failed build's output,
// "ok" once a build succeeds, "reload" for web pages to load it.
func (s *devServer) send(kind, text string) {
	msg := kind
	if kind == "error" {
		msg += " " + strconv.Quote(text)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	switch kind {
	case "error":
		s.last = msg
	case "ok":
		s.last = ""
	}
	for ch := range s.clients {
		select {
		case ch <- msg:
		default:
		}
	}
}

func (s *devServer) events(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	ch := make(chan string, 8)
	s.mu.Lock()
	s.clients[ch] = true
	last := s.last
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.clients, ch)
		s.mu.Unlock()
	}()
	flusher, _ := w.(http.Flusher)
	write := func(msg string) bool {
		if _, err := fmt.Fprintf(w, "data: %s\n\n", msg); err != nil {
			return false
		}
		if flusher != nil {
			flusher.Flush()
		}
		return true
	}
	if !write("hello") || last != "" && !write(last) {
		return
	}
	for {
		select {
		case msg := <-ch:
			if !write(msg) {
				return
			}
		case <-r.Context().Done():
			return
		}
	}
}

// sessionFor picks how dev builds and shows nsgo.yaml's target.
func sessionFor(p *project, opts devOptions) (session, error) {
	switch p.Dev.Target {
	case "native":
		switch runtime.GOOS {
		case "darwin":
			return macSession(opts), nil
		case "windows":
			return windowsSession(opts), nil
		case "linux":
			return linuxSession(opts), nil
		}
		return session{}, fmt.Errorf("native development runs on macOS, Windows and Linux; use -target web")
	case "web":
		return webSession(opts), nil
	case "ios":
		return iosDeviceSession(opts), nil
	case "ios_simulator":
		return iosSession(opts), nil
	case "android":
		return androidSession(opts), nil
	default:
		return session{}, fmt.Errorf("unknown dev target %q; use native, web, ios, ios_simulator or android", p.Dev.Target)
	}
}

// sourceDirs lists the project's directory and those of every non-standard
// package it compiles, NSGo's own included, and the files they embed: a
// TSX app's sources.
func sourceDirs(ctx context.Context, dir string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "go", "list", "-deps", "-f",
		"{{if not .Standard}}{{.Dir}}{{range .EmbedFiles}} {{$.Dir}}/{{.}}{{end}}{{end}}", ".")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		// A project that does not compile still has its own files to watch.
		return []string{dir}, nil
	}
	dirs := []string{dir}
	for _, d := range strings.Fields(string(out)) {
		dirs = append(dirs, d)
	}
	return dirs, nil
}

// scan returns a fingerprint of the watched files' names, sizes and times:
// the Go files of each directory, and each file listed.
func scan(dirs []string) string {
	var b strings.Builder
	for _, dir := range dirs {
		if info, err := os.Stat(dir); err == nil && !info.IsDir() {
			fmt.Fprintf(&b, "%s %d %d\n", dir, info.Size(), info.ModTime().UnixNano())
			continue
		}
		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || !(strings.HasSuffix(name, ".go") || name == "go.mod" || name == "go.sum" || name == "nsgo.yaml") {
				continue
			}
			if info, err := e.Info(); err == nil {
				fmt.Fprintf(&b, "%s/%s %d %d\n", dir, name, info.Size(), info.ModTime().UnixNano())
			}
		}
	}
	return b.String()
}

// devEnv is the environment of a dev app this machine starts: where its
// dev server is.
func devEnv(opts devOptions) []string {
	return []string{"NSGO_DEV_SERVER=" + opts.server.url()}
}

// macSession runs the app's bundle binary in the foreground, its output
// in this terminal.
func macSession(opts devOptions) session {
	return session{
		build: func(ctx context.Context, p *project) error {
			dir := p.cache("dev", "macos")
			os.RemoveAll(dir)
			_, err := buildMacApp(ctx, p, dir, runtime.GOARCH)
			return err
		},
		start: func(ctx context.Context, p *project) (app, error) {
			bin := filepath.Join(p.cache("dev", "macos"), p.Name+".app", "Contents", "MacOS", p.Name)
			stop, wait, err := startProcessEnv(devEnv(opts), bin)
			if err != nil {
				return app{}, err
			}
			return app{stop: stop, wait: wait}, nil
		},
	}
}

// windowsSession builds the app as a Windows executable and runs it in
// the foreground.
func windowsSession(opts devOptions) session {
	return session{
		build: func(ctx context.Context, p *project) error {
			dir := p.cache("dev", "windows")
			os.RemoveAll(dir)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
			return buildWindows(ctx, p, dir, runtime.GOARCH)
		},
		start: func(ctx context.Context, p *project) (app, error) {
			bin := filepath.Join(p.cache("dev", "windows"), p.Name+".exe")
			stop, wait, err := startProcessEnv(devEnv(opts), bin)
			if err != nil {
				return app{}, err
			}
			return app{stop: stop, wait: wait}, nil
		},
	}
}

// linuxSession builds the app as a Linux executable and runs it in the
// foreground.
func linuxSession(opts devOptions) session {
	return session{
		build: func(ctx context.Context, p *project) error {
			dir := p.cache("dev", "linux")
			os.RemoveAll(dir)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
			return buildLinux(ctx, p, dir, runtime.GOARCH)
		},
		start: func(ctx context.Context, p *project) (app, error) {
			bin := filepath.Join(p.cache("dev", "linux"), p.Name)
			stop, wait, err := startProcessEnv(devEnv(opts), bin)
			if err != nil {
				return app{}, err
			}
			return app{stop: stop, wait: wait}, nil
		},
	}
}

// webSession serves the build from the dev server, whose page shows a
// failed build and loads each new one.
func webSession(opts devOptions) session {
	var once sync.Once
	return session{
		build: func(ctx context.Context, p *project) error {
			return buildWeb(ctx, p, p.cache("dev", "web"), true)
		},
		start: func(ctx context.Context, p *project) (app, error) {
			once.Do(func() {
				opts.server.serve(p.cache("dev", "web"))
				url := fmt.Sprintf("http://localhost:%d/", opts.server.port)
				note("%s", link(url))
				for _, lan := range opts.server.lanURLs() {
					detail("%s", link(lan+"/"))
				}
				if opts.open {
					name, args := openCommand(url)
					exec.Command(name, args...).Start()
				}
			})
			return app{stop: func() {}}, nil
		},
	}
}

// startProcess runs a command with its output above the tool's lines. stop
// ends the process; wait returns its error once it exits, or nil when stop
// or ctx ended it.
func startProcess(name string, args ...string) (stop func(), wait func(context.Context) error, err error) {
	return startProcessEnv(nil, name, args...)
}

// startProcessEnv is startProcess with env added to this process's.
func startProcessEnv(env []string, name string, args ...string) (stop func(), wait func(context.Context) error, err error) {
	cmd := exec.Command(name, args...)
	cmd.Stdout, cmd.Stderr = &appConsole, &appConsole
	if env != nil {
		cmd.Env = append(os.Environ(), env...)
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}
	done := make(chan struct{})
	var waitErr error
	go func() {
		waitErr = cmd.Wait()
		close(done)
	}()
	var once sync.Once
	stopped := false
	stop = func() {
		once.Do(func() {
			select {
			case <-done:
				return
			default:
			}
			stopped = true
			if runtime.GOOS == "windows" {
				// Ask the app's windows to close, so it saves and exits;
				// a process without one is killed below.
				exec.Command("taskkill", "/PID", strconv.Itoa(cmd.Process.Pid)).Run()
			} else {
				cmd.Process.Signal(os.Interrupt)
			}
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				cmd.Process.Kill()
				<-done
			}
		})
	}
	wait = func(ctx context.Context) error {
		select {
		case <-done:
			if stopped {
				return nil
			}
			var exit *exec.ExitError
			if errors.As(waitErr, &exit) && exit.ExitCode() < 0 {
				return nil
			}
			return waitErr
		case <-ctx.Done():
			stop()
			return nil
		}
	}
	return stop, wait, nil
}
