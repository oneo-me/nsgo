package core

import (
	"errors"
	"slices"
	"testing"

	"github.com/oneo-me/nsgo/internal/driver"
)

func TestSpeechSessions(t *testing.T) {
	var cx *Scope
	h := newHost(t, Size{Width: 100, Height: 100}, func(c *Scope) Node { cx = c; return View{} })
	h.tree.SetEnvironment(Environment{Speech: true})
	h.commit()
	var log []string
	var ended error
	opts := SpeechOptions{Lang: "en-US", Interim: true,
		OnStart:  func() { log = append(log, "start") },
		OnResult: func(text string, final bool) { log = append(log, "result "+text) },
		OnEnd: func(text string, err error) {
			log = append(log, "end "+text)
			ended = err
		}}
	s, err := cx.StartSpeech(opts)
	if err != nil {
		t.Fatal(err)
	}
	it := command(h.commit(), driver.ItemSpeech)
	if it == nil || it.Speech.Command != driver.SpeechStart || it.Speech.Lang != "en-US" || !it.Speech.Interim {
		t.Fatalf("start = %+v", it)
	}
	report := func(id uint32, e driver.SpeechEvent) {
		h.tree.Dispatch(driver.Event{Kind: driver.EventSpeech, Count: id, Data: &e})
	}
	report(it.Count, driver.SpeechEvent{Phase: driver.SpeechStarted})
	report(it.Count, driver.SpeechEvent{Phase: driver.SpeechResult, Text: "hello"})
	s.Stop()
	if stop := command(h.commit(), driver.ItemSpeech); stop == nil || stop.Speech.Command != driver.SpeechStop || stop.Count != it.Count {
		t.Fatalf("stop = %+v", stop)
	}
	report(it.Count, driver.SpeechEvent{Phase: driver.SpeechResult, Text: "hello world", Final: true})
	report(it.Count, driver.SpeechEvent{Phase: driver.SpeechEnded, Error: driver.SpeechNoSpeech})
	report(it.Count, driver.SpeechEvent{Phase: driver.SpeechResult, Text: "late"}) // after the end
	if want := []string{"start", "result hello", "result hello world", "end hello world"}; !slices.Equal(log, want) {
		t.Fatalf("log %q, want %q", log, want)
	}
	if !errors.Is(ended, SpeechNoSpeech) || !s.Ended() || s.Text() != "hello world" {
		t.Fatalf("ended with %v, %v %q", ended, s.Ended(), s.Text())
	}

	// A new session aborts the one running, which ends at once.
	log = nil
	first, _ := cx.StartSpeech(opts)
	firstID := command(h.commit(), driver.ItemSpeech).Count
	second, _ := cx.StartSpeech(SpeechOptions{})
	b := h.commit()
	var cmds []driver.SpeechCommand
	for _, it := range b.Items {
		if it.Kind == driver.ItemSpeech {
			cmds = append(cmds, it.Speech.Command)
		}
	}
	if !first.Ended() || second.Ended() || !slices.Equal(cmds, []driver.SpeechCommand{driver.SpeechAbort, driver.SpeechStart}) ||
		!slices.Equal(log, []string{"end "}) {
		t.Fatalf("first ended %v, second %v, commands %v, log %q", first.Ended(), second.Ended(), cmds, log)
	}
	report(firstID, driver.SpeechEvent{Phase: driver.SpeechResult, Text: "stale"})
	if len(log) != 1 {
		t.Fatalf("an aborted session heard %q", log)
	}

	if _, err := cx.StartSpeech(SpeechOptions{Lang: "not a tag"}); err == nil {
		t.Error("a Lang that is no tag starts")
	}
}

func TestSpeechUnavailable(t *testing.T) {
	var cx *Scope
	h := newHost(t, Size{Width: 100, Height: 100}, func(c *Scope) Node { cx = c; return View{} })
	h.commit()
	var ended error
	s, _ := cx.StartSpeech(SpeechOptions{OnEnd: func(_ string, err error) { ended = err }})
	h.drain()
	if !errors.Is(ended, SpeechUnavailable) || !s.Ended() || command(h.commit(), driver.ItemSpeech) != nil {
		t.Fatalf("ended with %v", ended)
	}
}
