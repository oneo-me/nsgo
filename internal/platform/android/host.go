//go:build android

package android

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"sync"

	"github.com/nsgo-dev/nsgo/internal/engine"
)

// Embedding: an Android app of its own shows NSGo apps in NsgoViews from
// the library `nsgo build -embed` makes, each starting a process of its
// app. The host's settings that are environment, its locale, text size
// and appearance, reach the app in the environment Java reports. The
// channel crosses the pipe as JSON: the parameters in NSGO_HOST_PARAMS as
// the process starts, then msgHost from Java and msgHostCall to it.

// The parts of a msgHost.
const (
	hostEvent      = 1 // an event the host sends: its name and data
	hostHandle     = 2 // the host handles a name, or no longer
	hostReply      = 3 // the host's answer to a call
	hostTokens     = 4 // the tokens, light and dark
	hostURLHandler = 5 // the host sees the URLs the app opens, or no longer
	hostURLAnswer  = 6 // whether it took one
)

// The parts of a msgHostCall.
const (
	callCall    = 1 // a call or post of the app's
	callOpenURL = 2 // a URL the app opens, for the host's handler
	callError   = 3 // a failure the app did not catch
)

// embedHost is the surface's end of the channel.
type embedHost struct {
	ch       *engine.Host
	settings *engine.HostSettings
	mu       sync.Mutex
	next     uint32
	calls    map[uint32]*engine.HostCall
	urls     map[uint32]func(bool)
}

// newEmbedHost makes the channel, with the parameters the host gave the
// process.
func newEmbedHost() *embedHost {
	h := &embedHost{ch: engine.NewHost(), settings: &engine.HostSettings{}, calls: map[uint32]*engine.HostCall{},
		urls: map[uint32]func(bool){}}
	if p := os.Getenv("NSGO_HOST_PARAMS"); p != "" {
		var params map[string]any
		if err := json.Unmarshal([]byte(p), &params); err != nil {
			log.Printf("nsgo: the host's parameters: %v", err)
		}
		h.ch.SetParams(params)
	}
	return h
}

// options are what the engine takes of the host.
func (s *surface) hostOptions() engine.HostOptions {
	return engine.HostOptions{Channel: s.host.ch, Settings: s.host.settings, OnError: func(err error) {
		s.send(msgHostCall, func(w *writer) {
			w.byte(callError)
			w.str(err.Error())
		})
	}}
}

// hostMessage handles a msgHost.
func (s *surface) hostMessage(r *reader) {
	h := s.host
	switch r.byte() {
	case hostEvent:
		name, data := r.str(), r.str()
		var v any
		json.Unmarshal([]byte(data), &v)
		h.ch.Send(name, v)
	case hostHandle:
		name, on := r.str(), r.byte() != 0
		if !on {
			h.ch.RemoveHandler(name)
			return
		}
		h.ch.Handle(name, func(c *engine.HostCall) {
			h.mu.Lock()
			h.next++
			id := h.next
			if !c.IsPost() {
				h.calls[id] = c
			}
			h.mu.Unlock()
			s.send(msgHostCall, func(w *writer) {
				w.byte(callCall)
				w.u32(id)
				w.str(c.Name)
				w.str(jsonValue(c.Data))
				w.flags(c.IsPost())
			})
		})
	case hostReply:
		id, ok, value := r.u32(), r.byte() != 0, r.str()
		h.mu.Lock()
		c := h.calls[id]
		delete(h.calls, id)
		h.mu.Unlock()
		if c == nil {
			return
		}
		if !ok {
			c.Reject(errors.New(value))
			return
		}
		var v any
		json.Unmarshal([]byte(value), &v)
		c.Resolve(v)
	case hostTokens:
		var t engine.Tokens
		json.Unmarshal([]byte(r.str()), &t.Values)
		json.Unmarshal([]byte(r.str()), &t.Dark)
		h.settings.SetTokens(t)
	case hostURLHandler:
		if r.byte() == 0 {
			h.ch.HandleOpenURL(nil)
			return
		}
		h.ch.HandleOpenURL(func(url string, answer func(bool)) {
			h.mu.Lock()
			h.next++
			id := h.next
			h.urls[id] = answer
			h.mu.Unlock()
			s.send(msgHostCall, func(w *writer) {
				w.byte(callOpenURL)
				w.u32(id)
				w.str(url)
			})
		})
	case hostURLAnswer:
		id, handled := r.u32(), r.byte() != 0
		h.mu.Lock()
		answer := h.urls[id]
		delete(h.urls, id)
		h.mu.Unlock()
		if answer != nil {
			answer(handled)
		}
	}
}
