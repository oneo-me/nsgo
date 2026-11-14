package main

import (
	"bufio"
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
)

// devtoolsPage is the DevTools page nsgo dev -devtools serves: plain HTML and JavaScript, so it opens at once and builds with nothing.
//
//go:embed devtools
var devtoolsPage embed.FS

// serveDevTools adds DevTools to the dev server: the page at /_nsgo/devtools/, the app's messages, which it posts to /_nsgo/devtools/out and pages follow at /_nsgo/devtools/stream, and the pages' commands, which /_nsgo/devtools/command passes to the app as devtools events.
func (s *devServer) serveDevTools() {
	page, _ := fs.Sub(devtoolsPage, "devtools")
	s.mux.Handle("/_nsgo/devtools/", http.StripPrefix("/_nsgo/devtools/", http.FileServer(http.FS(page))))
	s.mux.HandleFunc("/_nsgo/devtools/out", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST the app's messages", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", "*")
		sc := bufio.NewScanner(r.Body)
		sc.Buffer(make([]byte, 1<<16), 64<<20)
		for sc.Scan() {
			if line := bytes.TrimSpace(sc.Bytes()); len(line) > 0 {
				s.toPages(string(line))
			}
		}
	})
	s.mux.HandleFunc("/_nsgo/devtools/stream", s.pageStream)
	// The app's directory, for the page's links to TSX files, which source maps name relative to it.
	s.mux.HandleFunc("/_nsgo/devtools/project", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"dir": s.dir})
	})
	s.mux.HandleFunc("/_nsgo/devtools/command", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST a command", http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		var compact bytes.Buffer
		if err == nil {
			err = json.Compact(&compact, body)
		}
		if err != nil {
			http.Error(w, "not a JSON command", http.StatusBadRequest)
			return
		}
		s.send("devtools", compact.String())
	})
}

// toPages sends every DevTools page msg, dropping it for a page that has not taken the last ones: the page asks again for what it lacks.
func (s *devServer) toPages(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for ch := range s.pages {
		select {
		case ch <- msg:
		default:
		}
	}
}

// pageStream follows the app's messages for a DevTools page.
func (s *devServer) pageStream(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	ch := make(chan string, 256)
	s.mu.Lock()
	s.pages[ch] = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.pages, ch)
		s.mu.Unlock()
	}()
	flusher, _ := w.(http.Flusher)
	// The page asks the app what it shows once it hears it is there.
	fmt.Fprint(w, "data: {\"type\":\"connected\"}\n\n")
	if flusher != nil {
		flusher.Flush()
	}
	for {
		select {
		case <-r.Context().Done():
			return
		case msg := <-ch:
			if _, err := fmt.Fprintf(w, "data: %s\n\n", msg); err != nil {
				return
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
	}
}
