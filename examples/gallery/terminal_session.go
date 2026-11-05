package main

import (
	"fmt"
	"io"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// demoSession stands in for a remote shell, as an SSH session would be
// the terminal's Conn on a phone, which runs no local programs: it reads
// what the terminal sends, edits a line as a shell's line editor does
// and answers a few commands. Its output is the gallery's content, in
// English.
type demoSession struct {
	out *io.PipeReader
	w   *io.PipeWriter

	mu         sync.Mutex
	line       []rune
	esc        []byte // an escape sequence being read
	pasting    bool
	cols, rows int
	closed     bool
}

// demoFiles are the session's files.
var demoFiles = map[string]string{
	"README.md":   "# Demo session\n\nA stand-in for a remote shell, written in Go.\nType help for its commands.\n",
	"notes.txt":   "Long-press a word to select it and open the menu.\nCtrl and Alt on the key bar apply to the next key.\n",
	"colors.sh":   "printf '\\e[31mred \\e[32mgreen \\e[34mblue\\e[0m\\n'\n",
	"hello.go":    "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(\"hello\") }\n",
	"wide-字符.txt": "宽字符 take two cells: 你好，世界\n",
}

func newDemoSession() *demoSession {
	r, w := io.Pipe()
	s := &demoSession{out: r, w: w, cols: 80, rows: 24}
	go func() {
		s.print("\x1b[1mNSGo demo session\x1b[0m on %s/%s — type \x1b[36mhelp\x1b[0m\r\n", runtime.GOOS, runtime.GOARCH)
		s.prompt()
	}()
	return s
}

func (s *demoSession) Read(p []byte) (int, error) { return s.out.Read(p) }

// Resize hears the terminal's size, as an SSH session's window change.
func (s *demoSession) Resize(cols, rows int) error {
	s.mu.Lock()
	s.cols, s.rows = cols, rows
	s.mu.Unlock()
	return nil
}

func (s *demoSession) Close() error {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	return s.w.Close()
}

func (s *demoSession) print(format string, args ...any) {
	fmt.Fprintf(s.w, format, args...)
}

func (s *demoSession) prompt() { s.print("\x1b[32mdemo@nsgo\x1b[0m:\x1b[34m~\x1b[0m$ ") }

// Write takes what the terminal sends: keys and pastes.
func (s *demoSession) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return 0, io.ErrClosedPipe
	}
	n := len(p)
	for len(p) > 0 {
		b := p[0]
		if len(s.esc) > 0 || b == 0x1b {
			s.esc = append(s.esc, b)
			p = p[1:]
			s.escape()
			continue
		}
		r, size := utf8.DecodeRune(p)
		p = p[size:]
		switch {
		case r == '\r' || r == '\n':
			if s.pasting {
				s.line = append(s.line, ' ')
				s.print(" ")
				continue
			}
			s.print("\r\n")
			line := string(s.line)
			s.line = s.line[:0]
			s.run(strings.TrimSpace(line))
		case r == 0x7f || r == 0x08:
			if len(s.line) > 0 {
				s.line = s.line[:len(s.line)-1]
				s.print("\b \b")
			}
		case r == 0x03: // Control+C
			s.line = s.line[:0]
			s.print("^C\r\n")
			s.prompt()
		case r == 0x0c: // Control+L
			s.print("\x1b[H\x1b[2J")
			s.prompt()
			s.print("%s", string(s.line))
		case r == 0x15: // Control+U
			s.print("\r\x1b[K")
			s.line = s.line[:0]
			s.prompt()
		case r == 0x04 && len(s.line) == 0: // Control+D
			s.print("logout\r\n")
			go s.Close()
			return n, nil
		case r == '\t':
			s.complete()
		case r >= ' ':
			s.line = append(s.line, r)
			s.print("%c", r)
		}
	}
	return n, nil
}

// escape reads an escape sequence: bracketed pastes' marks, and keys,
// which move nothing in this line editor but Escape alone.
func (s *demoSession) escape() {
	seq := string(s.esc)
	if len(seq) < 2 || seq[1] == '[' && len(seq) < 3 {
		return
	}
	if seq[1] == '[' || seq[1] == 'O' {
		last := seq[len(seq)-1]
		if last < 0x40 || last > 0x7e {
			return
		}
	}
	s.esc = s.esc[:0]
	switch seq {
	case "\x1b[200~":
		s.pasting = true
	case "\x1b[201~":
		s.pasting = false
	}
}

// complete completes a file name, as Tab does in a shell.
func (s *demoSession) complete() {
	line := string(s.line)
	i := strings.LastIndexByte(line, ' ') + 1
	word := line[i:]
	var match []string
	for name := range demoFiles {
		if strings.HasPrefix(name, word) {
			match = append(match, name)
		}
	}
	if len(match) == 1 {
		rest := match[0][len(word):]
		s.line = append(s.line, []rune(rest)...)
		s.print("%s", rest)
	}
}

func (s *demoSession) run(line string) {
	name, arg, _ := strings.Cut(line, " ")
	arg = strings.TrimSpace(arg)
	switch name {
	case "":
	case "help":
		s.print("Commands: help, ls, cat FILE, echo TEXT, title TEXT, colors, size, date, top, clear, exit\r\n")
	case "ls":
		names := make([]string, 0, len(demoFiles))
		for n := range demoFiles {
			names = append(names, n)
		}
		sort.Strings(names)
		s.print("%s\r\n", strings.Join(names, "  "))
	case "cat":
		text, ok := demoFiles[arg]
		if !ok {
			s.print("cat: %s: No such file\r\n", arg)
			break
		}
		s.print("%s", strings.ReplaceAll(text, "\n", "\r\n"))
	case "echo":
		s.print("%s\r\n", arg)
	case "title":
		s.print("\x1b]2;%s\x07", arg)
	case "colors":
		for i := range 16 {
			s.print("\x1b[48;5;%dm  ", i)
			if i == 7 {
				s.print("\x1b[0m\r\n")
			}
		}
		s.print("\x1b[0m\r\n")
		for i := range 24 {
			v := 232 + i
			s.print("\x1b[48;5;%dm ", v)
		}
		s.print("\x1b[0m\r\n\x1b[1mbold\x1b[0m \x1b[3mitalic\x1b[0m \x1b[4munderline\x1b[0m \x1b[9mstrike\x1b[0m \x1b[7minverse\x1b[0m\r\n")
	case "size":
		s.print("%d columns × %d rows\r\n", s.cols, s.rows)
	case "date":
		s.print("%s\r\n", time.Now().Format(time.RFC1123))
	case "top":
		// A full-screen program's alternate screen, left after a moment.
		s.print("\x1b[?1049h\x1b[H\x1b[2J\x1b[7m%-*s\x1b[0m\r\n", s.cols, " demo top — the alternate screen")
		for i := 1; i < s.rows-1; i++ {
			s.print(" %3d  worker-%02d  %5.1f%%\r\n", 1000+i, i, float64(i*7%100))
		}
		go func() {
			time.Sleep(2 * time.Second)
			s.mu.Lock()
			defer s.mu.Unlock()
			if !s.closed {
				s.print("\x1b[?1049l")
				s.prompt()
			}
		}()
		return
	case "clear":
		s.print("\x1b[H\x1b[2J")
	case "exit", "logout":
		s.print("logout\r\n")
		go s.Close()
		return
	default:
		s.print("%s: command not found\r\n", name)
	}
	s.prompt()
}
