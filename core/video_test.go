package core

import (
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

func TestVideoConfiguresSeeksAndReports(t *testing.T) {
	var ref Ref
	var paused *Signal[bool]
	var info []VideoInfo
	var positions []time.Duration
	var ended, buffering int
	var failed error
	h := newHost(t, Size{Width: 300, Height: 400}, func(cx *Scope) Node {
		paused = NewSignal(cx, true)
		return Video{
			Source:     "asset:/clip.mp4",
			PausedFunc: paused.Get,
			Volume:     2,
			Repeat:     true,
			Ref:        &ref,
			TestID:     "video",
			Style:      Style{Width: Pt(300), Height: Pt(200)},
			OnLoad:     func(i VideoInfo) { info = append(info, i) },
			OnProgress: func(p, _ time.Duration) { positions = append(positions, p) },
			OnEnd:      func() { ended++ },
			OnBuffer:   func(bool) { buffering++ },
			OnError:    func(err error) { failed = err },
		}
	})
	b := h.commit()
	var cfg *driver.VideoConfig
	var mask driver.EventMask
	for _, it := range b.Items {
		switch it.Kind {
		case driver.ItemVideo:
			cfg = it.Video
		case driver.ItemEvents:
			if it.ID == h.find("video").id {
				mask = it.Events
			}
		}
	}
	if n := h.find("video"); n.kind != driver.KindVideo {
		t.Fatalf("kind %v, want KindVideo", n.kind)
	}
	want := driver.VideoConfig{Source: "asset:/clip.mp4", Paused: true, Volume: 1, Rate: 1, Repeat: true, ResizeMode: Contain,
		ProgressInterval: 250 * time.Millisecond}
	if cfg == nil || *cfg != want {
		t.Fatalf("config %+v, want %+v", cfg, want)
	}
	if !mask.Has(driver.EventLoad) || !mask.Has(driver.EventProgress) || !mask.Has(driver.EventEnd) || mask.Has(driver.EventLoadStart) {
		t.Fatalf("mask %b", mask)
	}
	if k := kinds(h.commit()); k[driver.ItemVideo] != 0 {
		t.Fatalf("resent an unchanged config: %v", k)
	}

	paused.Set(false)
	ref.Seek(-time.Second)
	ref.Seek(3 * time.Second)
	b = h.commit()
	var seeks []time.Duration
	for _, it := range b.Items {
		switch it.Kind {
		case driver.ItemVideo:
			if it.Video.Paused {
				t.Fatalf("still paused: %+v", it.Video)
			}
		case driver.ItemVideoSeek:
			seeks = append(seeks, it.Time)
		}
	}
	if k := kinds(b); k[driver.ItemVideo] != 1 || len(seeks) != 2 || seeks[0] != 0 || seeks[1] != 3*time.Second {
		t.Fatalf("items %v, seeks %v", k, seeks)
	}

	id := h.find("video").id
	h.tree.Dispatch(driver.Event{Kind: driver.EventLoad, ID: id, Duration: 90 * time.Second, Size: Size{Width: 640, Height: 360}})
	h.tree.Dispatch(driver.Event{Kind: driver.EventProgress, ID: id, Time: time.Second, Duration: 90 * time.Second})
	h.tree.Dispatch(driver.Event{Kind: driver.EventBuffer, ID: id, Buffering: true})
	h.tree.Dispatch(driver.Event{Kind: driver.EventEnd, ID: id})
	h.tree.Dispatch(driver.Event{Kind: driver.EventError, ID: id, Text: "no decoder"})
	if len(info) != 1 || info[0].Duration != 90*time.Second || info[0].Size.Width != 640 ||
		len(positions) != 1 || positions[0] != time.Second || ended != 1 || buffering != 1 {
		t.Fatalf("info %v, positions %v, ended %d, buffering %d", info, positions, ended, buffering)
	}
	if me, ok := failed.(*MediaError); !ok || me.Message != "no decoder" {
		t.Fatalf("error %#v", failed)
	}
}

func TestVideoDefaults(t *testing.T) {
	for _, c := range []struct {
		volume, rate       float32
		interval           time.Duration
		wantVolume, wantRt float32
		wantInterval       time.Duration
	}{
		{0, 0, 0, 1, 1, 250 * time.Millisecond},
		{0.5, 2, time.Millisecond, 0.5, 2, 16 * time.Millisecond},
		{-1, -1, time.Second, 1, 1, time.Second},
	} {
		if v, r, i := videoVolume(c.volume), videoRate(c.rate), videoInterval(c.interval); v != c.wantVolume || r != c.wantRt || i != c.wantInterval {
			t.Errorf("%v %v %v: got %v %v %v", c.volume, c.rate, c.interval, v, r, i)
		}
	}
}
