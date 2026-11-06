// Package terminal is a terminal for NSGo apps: a View that runs the
// user's shell, or any program, in a pseudo-terminal, with Ghostty's
// terminal emulator, libghostty-vt, loaded at run time with no cgo.
//
// It runs programs on macOS, Linux and Windows, and remote sessions
// (Options.Conn) there and on iOS; elsewhere New returns an error.
// It does what terminals do: colors (16, 256 and 24-bit) and styles,
// wide characters and grapheme clusters, a scrollback that reflows, the
// alternate screen, the keyboard as programs ask (legacy sequences,
// modifyOtherKeys or the Kitty keyboard protocol), bracketed paste, the
// title (OSC 0 and 2) and the working directory (OSC 7).
package terminal

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/oneo-me/nsgo/base/terminal/internal/library"
	"github.com/oneo-me/nsgo/base/terminal/internal/pty"
	"github.com/oneo-me/nsgo/base/terminal/internal/vt"
	"github.com/oneo-me/nsgo/base/terminal/locales"
)

// Options configure a terminal.
type Options struct {
	// Command is the program and its arguments: the user's login shell
	// when empty ($SHELL, else /bin/zsh on macOS and /bin/sh elsewhere,
	// and PowerShell on Windows).
	Command []string
	// Dir is the program's working directory: the home directory when
	// empty.
	Dir string
	// Env adds to the app's environment, which has TERM=xterm-256color,
	// COLORTERM=truecolor, and LANG=en_US.UTF-8 when no locale is set.
	Env []string
	// Conn connects the terminal to what reads and writes, as an SSH
	// session, instead of a program. A Conn with a method Resize(cols,
	// rows int) error is told the screen's size.
	Conn io.ReadWriteCloser
	// ResizeByConn leaves the size of the screen to whoever reads Conn: a
	// view asks Conn's ResizePixels(cols, rows, cellWidth, cellHeight int)
	// error, or else its Resize, for the size it fits, and the screen keeps
	// its size until Resize sets it, as a terminal showing a server's
	// (see Mirror) does for the frames that come.
	ResizeByConn bool
	// Scrollback is about how many bytes of output to keep above the
	// screen: 10 MB when zero, none when negative.
	Scrollback int
	// Cursor is the cursor programs reset to: a block when zero.
	Cursor CursorStyle
	// OnTitle, OnExit and OnBell run on a goroutine of the terminal when
	// the program sets the title, ends or rings the bell.
	OnTitle func(title string)
	OnExit  func(code int)
	OnBell  func()
	// OnSearch runs on the engine as the status of the search changes.
	OnSearch func(s SearchStatus)
}

// CursorStyle is the shape of the cursor.
type CursorStyle int

const (
	CursorBlock CursorStyle = iota
	CursorBar
	CursorUnderline
)

// Terminal is a terminal emulator and the program it runs. Its methods are
// safe from any goroutine.
type Terminal struct {
	opts Options
	in   inputQueue

	// mu guards the emulator, which the reader changes and views read.
	mu     sync.Mutex
	term   *vt.Terminal
	rs     *vt.RenderState
	size   gridSize
	events events
	closed bool
	// held is a program's synchronized update (mode 2026), during which
	// views draw the render state as it was when it started.
	held      bool
	heldSince time.Time
	theme     *Theme
	// mirrors are the terminal's Mirrors, each passing on what programs
	// copy; asked is the size a view last asked Conn for.
	mirrors []*Mirror
	asked   gridSize
	find    search
	// lang is the language of the app showing the terminal, which says
	// the terminal's own lines.
	lang string

	// src is the program's pseudo-terminal, or Conn: nil until the
	// program starts, from launch.
	src io.ReadWriteCloser
	// srcClosed tells that src was closed, which happens once.
	srcClosed bool
	proc      *pty.PTY
	launch    *pty.Options
	timer     *time.Timer

	draw     atomic.Pointer[func()] // asks the view for a frame
	done     chan struct{}
	doneOnce sync.Once
	code     atomic.Int64
}

// gridSize is the size of the screen in cells and of a cell in pixels.
type gridSize struct{ cols, rows, cellW, cellH int }

// events are what a program did while the reader wrote its output, for
// the callbacks, which run once the lock is released.
type events struct {
	title bool
	bells int
}

var load struct {
	once sync.Once
	err  error
}

// Load loads libghostty-vt, once, from LibraryPath. New calls it; call it
// earlier to report a missing library before showing a terminal.
func Load() error {
	load.once.Do(func() {
		if !vt.Supported {
			load.err = fmt.Errorf("terminal: not supported on %s/%s", runtime.GOOS, runtime.GOARCH)
			return
		}
		var path string
		if path, load.err = LibraryPath(); load.err == nil {
			load.err = vt.Load(path)
		}
	})
	return load.err
}

// LibraryPath returns the libghostty-vt the terminal loads, the first of:
// the file $NSGO_GHOSTTY_VT names; the one beside the executable, or in
// its macOS bundle's Frameworks or Resources; the build the package
// embeds for the platform, written once into the user's cache
// (<cache>/nsgo/libghostty-vt/<sha256>/).
func LibraryPath() (string, error) { return library.Find() }

// New makes a terminal running opts.Command, or connected to opts.Conn.
// The program starts once a view shows the terminal, at the size of the
// view, or half a second later at 80×24 when none does.
func New(opts Options) (*Terminal, error) {
	if err := Load(); err != nil {
		return nil, err
	}
	t := &Terminal{opts: opts, done: make(chan struct{}), size: gridSize{80, 24, 8, 16}}
	var err error
	t.term, err = vt.NewTerminal(80, 24, vt.Effects{
		WritePTY:     func(p []byte) { t.in.push(append([]byte(nil), p...)) },
		Bell:         func() { t.events.bells++ },
		TitleChanged: func() { t.events.title = true },
		RenderHold:   t.renderHold,
		Clipboard: func(text string, primary bool) bool {
			if primary {
				return false
			}
			for _, m := range t.mirrors {
				m.clipOut = queueClip(m.clipOut, text)
			}
			return len(t.mirrors) > 0
		},
		Size: func() (int, int, int, int) { return t.size.cols, t.size.rows, t.size.cellW, t.size.cellH },
		Dark: func() bool { return t.theme == nil || t.theme.dark() },
	})
	if err != nil {
		return nil, err
	}
	t.term.SetTerminfoName("xterm-256color")
	// Grapheme clusters take the cells of their width, as in Ghostty.
	t.term.SetModeDefault(vt.ModeGraphemes, true)
	switch {
	case opts.Scrollback < 0:
		t.term.SetScrollback(0)
	case opts.Scrollback > 0:
		t.term.SetScrollback(opts.Scrollback)
	default:
		t.term.SetScrollback(10 << 20)
	}
	t.term.SetDefaultCursor(vtCursor(opts.Cursor), false)
	if t.rs, err = vt.NewRenderState(); err != nil {
		t.term.Free()
		return nil, err
	}
	t.useTheme(DarkTheme())
	if opts.Conn != nil {
		t.src = opts.Conn
		t.run()
		return t, nil
	}
	if runtime.GOOS == "ios" || runtime.GOOS == "android" {
		t.rs.Free()
		t.term.Free()
		return nil, fmt.Errorf("terminal: %s runs no local programs; give Options.Conn a remote session", runtime.GOOS)
	}
	path, args, err := command(opts.Command)
	if err != nil {
		t.rs.Free()
		t.term.Free()
		return nil, err
	}
	dir := opts.Dir
	if dir == "" {
		dir, _ = os.UserHomeDir()
	}
	t.launch = &pty.Options{Path: path, Args: args, Dir: dir, Env: environment(opts.Env)}
	if runtime.GOOS == "windows" {
		// ConPTY keeps a screen of its own, which a resize pulling rows
		// back from the scrollback would disagree with.
		t.term.SetResizePullBack(false)
	}
	t.mu.Lock()
	t.timer = time.AfterFunc(500*time.Millisecond, func() {
		t.mu.Lock()
		o := t.launch
		t.launch = nil
		t.mu.Unlock()
		t.start(o)
	})
	t.mu.Unlock()
	return t, nil
}

// queueClip adds what a program copied to what a Mirror has to pass on,
// keeping the latest few if nothing frames them.
func queueClip(q []string, text string) []string {
	if len(q) >= 16 {
		q = q[1:]
	}
	return append(q, text)
}

// run reads the output and writes the input of the program, or Conn.
func (t *Terminal) run() {
	go t.read()
	go t.in.run(t.src)
}

// start starts the program of o, which the caller took, in a
// pseudo-terminal of the screen's size.
func (t *Terminal) start(o *pty.Options) {
	if o == nil {
		return
	}
	t.mu.Lock()
	s := t.size
	t.mu.Unlock()
	o.Cols, o.Rows, o.Width, o.Height = s.cols, s.rows, s.cols*s.cellW, s.rows*s.cellH
	proc, err := pty.Start(*o)
	if err != nil {
		t.write([]byte("\x1b[31m" + err.Error() + "\x1b[0m"))
		t.code.Store(-1)
		t.finish()
		return
	}
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		proc.Close()
		t.finish()
		return
	}
	t.proc, t.src = proc, proc
	if t.size != s {
		proc.Resize(t.size.cols, t.size.rows, t.size.cols*t.size.cellW, t.size.rows*t.size.cellH)
	}
	t.mu.Unlock()
	go func() {
		code, _ := proc.Wait()
		t.code.Store(int64(code))
		// What the program wrote before exiting is still to be read; then
		// hang up whatever else has the terminal, as terminal apps do.
		select {
		case <-t.done:
		case <-time.After(100 * time.Millisecond):
			proc.Close()
		}
	}()
	t.run()
}

// command returns the program to run and its arguments, with args[0] its
// name: the user's shell as a login shell ("-zsh") when cmd is empty.
func command(cmd []string) (path string, args []string, err error) {
	if len(cmd) > 0 {
		path, err = exec.LookPath(cmd[0])
		if err != nil {
			return "", nil, err
		}
		return path, cmd, nil
	}
	if runtime.GOOS == "windows" {
		for _, name := range []string{"pwsh.exe", "powershell.exe", os.Getenv("COMSPEC"), "cmd.exe"} {
			if p, err := exec.LookPath(name); name != "" && err == nil {
				return p, []string{name}, nil
			}
		}
		return "", nil, errors.New("terminal: no shell found")
	}
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
		if runtime.GOOS == "darwin" {
			shell = "/bin/zsh"
		}
	}
	return shell, []string{"-" + filepath.Base(shell)}, nil
}

// environment returns the program's environment: this process's with the
// terminal's variables and extra.
func environment(extra []string) []string {
	env := os.Environ()
	set := func(kv string) {
		k, _, _ := strings.Cut(kv, "=")
		for i, e := range env {
			if strings.HasPrefix(e, k+"=") {
				env[i] = kv
				return
			}
		}
		env = append(env, kv)
	}
	set("TERM=xterm-256color")
	set("COLORTERM=truecolor")
	if runtime.GOOS != "windows" && os.Getenv("LANG") == "" && os.Getenv("LC_ALL") == "" && os.Getenv("LC_CTYPE") == "" {
		set("LANG=en_US.UTF-8")
	}
	for _, kv := range extra {
		set(kv)
	}
	return env
}

// read processes the program's output until it ends.
func (t *Terminal) read() {
	buf := make([]byte, 64<<10)
	for {
		n, err := t.src.Read(buf)
		if n > 0 {
			t.write(buf[:n])
		}
		if err != nil {
			break
		}
	}
	t.ended()
}

// write processes output, then runs the callbacks of what it did.
func (t *Terminal) write(p []byte) {
	t.mu.Lock()
	var ev events
	var title string
	if t.term != nil {
		t.term.Write(p)
		ev, t.events = t.events, events{}
		if ev.title {
			title = t.term.Title()
		}
	}
	t.mu.Unlock()
	if ev.title && t.opts.OnTitle != nil {
		t.opts.OnTitle(title)
	}
	for range ev.bells {
		if t.opts.OnBell != nil {
			t.opts.OnBell()
		}
	}
	t.redraw()
}

// ended marks the program ended, once its output did.
func (t *Terminal) ended() {
	t.closeSrc()
	if t.proc != nil {
		code, _ := t.proc.Wait()
		t.code.Store(int64(code))
	}
	t.mu.Lock()
	if t.term != nil && !t.closed {
		m := locales.In(t.lang)
		msg := m.ProcessExited()
		if code := t.code.Load(); code != 0 {
			msg = m.ProcessExitedWithCode(int(code))
		}
		if t.opts.Conn != nil {
			msg = m.Disconnected()
		}
		if x, _ := t.term.Cursor(); x > 0 {
			msg = "\r\n" + msg
		}
		t.term.Write([]byte("\x1b[0;2m" + msg + "\x1b[0m"))
	}
	t.mu.Unlock()
	t.finish()
}

// finish closes Done and calls OnExit, once.
func (t *Terminal) finish() {
	t.doneOnce.Do(func() {
		close(t.done)
		t.in.close()
		if t.opts.OnExit != nil {
			t.opts.OnExit(int(t.code.Load()))
		}
		t.redraw()
	})
}

// Done is closed once the program exited (or Conn ended).
func (t *Terminal) Done() <-chan struct{} { return t.done }

// ExitCode returns the exit code of the program once it exited: -1 for a
// program that could not start.
func (t *Terminal) ExitCode() int { return int(t.code.Load()) }

// Title returns the title the program set (OSC 0, OSC 2).
func (t *Terminal) Title() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.term == nil {
		return ""
	}
	return t.term.Title()
}

// Dir returns the working directory the shell reported (OSC 7), as a path
// when it is a file URL.
func (t *Terminal) Dir() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.term == nil {
		return ""
	}
	return fileURLPath(t.term.Pwd())
}

// Size returns the size of the screen in cells.
func (t *Terminal) Size() (cols, rows int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.size.cols, t.size.rows
}

// Text returns the text of the screen and its scrollback, with the lines a
// program soft-wrapped joined.
func (t *Terminal) Text() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.term == nil {
		return ""
	}
	return t.term.Text()
}

// Resize sets the size of the screen in cells, for a terminal no view
// shows.
func (t *Terminal) Resize(cols, rows int) {
	t.mu.Lock()
	t.resize(gridSize{max(cols, 1), max(rows, 1), t.size.cellW, t.size.cellH})
	t.mu.Unlock()
	t.redraw()
}

// ResizePixels is Resize with the size of a cell in pixels, for a terminal
// no view shows that stands for one that does, as a server's for the
// window showing its session: programs asking the size in pixels get it. A
// zero cell size keeps the one before.
func (t *Terminal) ResizePixels(cols, rows, cellWidth, cellHeight int) {
	t.mu.Lock()
	if cellWidth <= 0 || cellHeight <= 0 {
		cellWidth, cellHeight = t.size.cellW, t.size.cellH
	}
	t.resize(gridSize{max(cols, 1), max(rows, 1), cellWidth, cellHeight})
	t.mu.Unlock()
	t.redraw()
}

// ClearScreen clears the scrollback and the screen above the cursor's line,
// which moves to the top, so the prompt the shell shows stays. A
// full-screen program's screen, which this would break, is left alone; it
// reports whether it cleared.
func (t *Terminal) ClearScreen() bool {
	t.mu.Lock()
	if t.term == nil || t.term.AltScreen() {
		t.mu.Unlock()
		return false
	}
	t.term.SetSelection(nil)
	t.term.ScrollToBottom()
	x, y := t.term.Cursor()
	seq := "\x1b[3J"
	if y > 0 {
		// Delete the rows above the cursor's from the top, then put the
		// cursor back on its line, now the first.
		seq += "\x1b[H\x1b[" + strconv.Itoa(y) + "M\x1b[1;" + strconv.Itoa(x+1) + "H"
	}
	// Under the lock, so that no output comes between the cursor read and
	// the rows deleted.
	t.term.Write([]byte(seq))
	t.mu.Unlock()
	t.redraw()
	return true
}

// Send sends data to the program as if typed.
func (t *Terminal) Send(data []byte) { t.in.push(append([]byte(nil), data...)) }

// Paste pastes text, bracketed when the program asks (mode 2004), so that
// pasted lines do not run as commands.
func (t *Terminal) Paste(text string) {
	t.mu.Lock()
	if t.term == nil {
		t.mu.Unlock()
		return
	}
	p := vt.EncodePaste(text, t.term.Mode(vt.ModeBracketedPaste))
	t.term.ScrollToBottom()
	t.term.SetSelection(nil)
	t.mu.Unlock()
	if len(p) > 0 {
		t.in.push(p)
	}
	t.redraw()
}

// Feed shows data as if the program wrote it: text and escape sequences.
func (t *Terminal) Feed(data []byte) { t.write(data) }

// SetCursor changes the cursor programs reset to, as Options.Cursor, and
// shows it now.
func (t *Terminal) SetCursor(s CursorStyle) {
	t.mu.Lock()
	if t.term != nil {
		t.term.SetDefaultCursor(vtCursor(s), false)
		t.term.Write([]byte("\x1b[0 q"))
	}
	t.mu.Unlock()
	t.redraw()
}

// ClearScrollback clears the scrollback, leaving the screen as it is.
func (t *Terminal) ClearScrollback() { t.write([]byte("\x1b[3J")) }

// Close closes the terminal: the program is hung up (SIGHUP), or Conn
// closed, and the emulator freed.
func (t *Terminal) Close() error {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return nil
	}
	t.closed = true
	if t.timer != nil {
		t.timer.Stop()
	}
	t.launch = nil
	src := t.src
	t.mu.Unlock()
	var err error
	if src != nil {
		err = t.closeSrc()
	} else {
		t.finish()
	}
	// The reader stops once the source is closed; the emulator goes once
	// no view or reader can reach it.
	go func() {
		<-t.done
		t.mu.Lock()
		if t.term != nil {
			t.endSearch()
			t.rs.Free()
			t.term.Free()
			t.term, t.rs = nil, nil
		}
		t.mu.Unlock()
		t.redraw()
	}()
	return err
}

func (t *Terminal) closeSrc() error {
	t.mu.Lock()
	src := t.src
	if t.srcClosed {
		src = nil
	}
	t.srcClosed = src != nil || t.srcClosed
	t.mu.Unlock()
	if src == nil {
		return nil
	}
	return src.Close()
}

// useTheme gives the terminal the colors of a view; t.mu held or not yet
// shared.
func (t *Terminal) useTheme(th *Theme) {
	if th == t.theme {
		return
	}
	old := t.theme
	t.theme = th
	th.apply(t.term)
	// Programs that asked are told when the colors go from light to dark
	// or back (mode 2031).
	if old != nil && old.dark() != th.dark() && t.term.Mode(vt.ModeColorScheme) {
		report := "\x1b[?997;2n"
		if th.dark() {
			report = "\x1b[?997;1n"
		}
		t.in.push([]byte(report))
	}
}

// resize sets the size of the screen, from a view; t.mu is held. The
// program starts at the first size a view gives.
func (t *Terminal) resize(s gridSize) {
	if t.term == nil {
		return
	}
	if o := t.launch; o != nil {
		t.launch = nil
		t.timer.Stop()
		defer func() { go t.start(o) }()
	}
	if s == t.size {
		return
	}
	t.size = s
	t.term.Resize(s.cols, s.rows, s.cellW, s.cellH)
	switch src := t.src.(type) {
	case *pty.PTY:
		src.Resize(s.cols, s.rows, s.cols*s.cellW, s.rows*s.cellH)
	case interface{ Resize(cols, rows int) error }:
		if !t.opts.ResizeByConn { // else the size came from Conn's reader
			go src.Resize(s.cols, s.rows)
		}
	}
}

// viewResize gives the screen the size the view fits, s, or with
// Options.ResizeByConn asks Conn for it, the screen keeping its size and
// taking the cells' new metrics only; t.mu is held.
func (t *Terminal) viewResize(s gridSize) {
	if !t.opts.ResizeByConn {
		t.resize(s)
		return
	}
	if t.term == nil {
		return
	}
	if s.cellW != t.size.cellW || s.cellH != t.size.cellH {
		t.size.cellW, t.size.cellH = s.cellW, s.cellH
		t.term.Resize(t.size.cols, t.size.rows, s.cellW, s.cellH)
	}
	if s == t.asked {
		return
	}
	t.asked = s
	switch src := t.src.(type) {
	case interface {
		ResizePixels(cols, rows, cellWidth, cellHeight int) error
	}:
		go src.ResizePixels(s.cols, s.rows, s.cellW, s.cellH)
	case interface{ Resize(cols, rows int) error }:
		go src.Resize(s.cols, s.rows)
	}
}

// renderHold starts or ends a program's synchronized update; t.mu is held.
func (t *Terminal) renderHold(held bool) {
	if held && t.rs != nil {
		t.rs.Update(t.term)
		t.heldSince = time.Now()
	}
	t.held = held
}

// redraw asks the view for a frame.
func (t *Terminal) redraw() {
	if f := t.draw.Load(); f != nil {
		(*f)()
	}
}

// fileURLPath returns the path of a file URL, as shells report their
// directory (file://host/path), or s when it is not one.
func fileURLPath(s string) string {
	rest, ok := strings.CutPrefix(s, "file://")
	if !ok {
		return s
	}
	if i := strings.IndexByte(rest, '/'); i >= 0 {
		rest = rest[i:]
	}
	return unescapePath(rest)
}

func unescapePath(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+2 < len(s) {
			hi, ok1 := unhex(s[i+1])
			lo, ok2 := unhex(s[i+2])
			if ok1 && ok2 {
				b.WriteByte(hi<<4 | lo)
				i += 2
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func unhex(c byte) (byte, bool) {
	switch {
	case '0' <= c && c <= '9':
		return c - '0', true
	case 'a' <= c && c <= 'f':
		return c - 'a' + 10, true
	case 'A' <= c && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}

// inputQueue writes what is typed to the program in order, from a
// goroutine of its own: a program that does not read its input must not
// block the reader, which answers its queries.
type inputQueue struct {
	mu     sync.Mutex
	cond   *sync.Cond
	queue  [][]byte
	closed bool
}

func (q *inputQueue) init() {
	if q.cond == nil {
		q.cond = sync.NewCond(&q.mu)
	}
}

func (q *inputQueue) push(p []byte) {
	q.mu.Lock()
	q.init()
	if !q.closed {
		q.queue = append(q.queue, p)
		q.cond.Signal()
	}
	q.mu.Unlock()
}

func (q *inputQueue) close() {
	q.mu.Lock()
	q.init()
	q.closed = true
	q.queue = nil
	q.cond.Broadcast()
	q.mu.Unlock()
}

func (q *inputQueue) run(w io.Writer) {
	for {
		q.mu.Lock()
		q.init()
		for len(q.queue) == 0 && !q.closed {
			q.cond.Wait()
		}
		if q.closed {
			q.mu.Unlock()
			return
		}
		p := q.queue[0]
		q.queue = q.queue[1:]
		q.mu.Unlock()
		if _, err := w.Write(p); err != nil {
			q.close()
			return
		}
	}
}

func vtCursor(s CursorStyle) vt.CursorStyle {
	switch s {
	case CursorBar:
		return vt.CursorBar
	case CursorUnderline:
		return vt.CursorUnderline
	}
	return vt.CursorBlock
}
