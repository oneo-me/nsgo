package audio

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestUnsupportedBackendReportsStableError(t *testing.T) {
	targetBackend = unsupportedBackend{}
	if _, err := Open("tone.wav"); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("Open error %v, want ErrUnsupported", err)
	}
	if _, err := PlayBytes([]byte("wav"), "audio/wav"); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("PlayBytes error %v, want ErrUnsupported", err)
	}
}

func TestSourcesAndOptionsAreValidated(t *testing.T) {
	if _, err := Open(""); !errors.Is(err, ErrUnreadableSource) {
		t.Errorf("empty path error %v, want ErrUnreadableSource", err)
	}
	if _, err := OpenBytes(nil, "audio/wav"); !errors.Is(err, ErrUnreadableSource) {
		t.Errorf("empty bytes error %v, want ErrUnreadableSource", err)
	}
	if _, err := OpenBytes([]byte("wav"), ""); !errors.Is(err, ErrUnsupportedFormat) {
		t.Errorf("empty MIME error %v, want ErrUnsupportedFormat", err)
	}
	for _, option := range []Option{WithVolume(-1), WithVolume(2), WithRate(0), WithRate(-1), WithProgressInterval(-time.Millisecond), WithSession(Session(99))} {
		if _, err := Open("tone.wav", option); !errors.Is(err, ErrInvalidOption) {
			t.Errorf("option error %v, want ErrInvalidOption", err)
		}
	}
}

func TestSoundLifecycle(t *testing.T) {
	backend := &fakeBackend{}
	previous := targetBackend
	targetBackend = backend
	t.Cleanup(func() { targetBackend = previous })

	sound, err := PlayFile("tone.wav", WithVolume(.4))
	if err != nil {
		t.Fatal(err)
	}
	if got := backend.soundOptions.Volume; got != .4 {
		t.Fatalf("volume %.2f, want .4", got)
	}
	ended := make(chan struct{})
	if err := sound.OnEnd(func() { close(ended) }); err != nil {
		t.Fatal(err)
	}
	backend.soundEnd()
	select {
	case <-ended:
	case <-time.After(time.Second):
		t.Fatal("sound end callback did not run")
	}
	if err := sound.Close(); err != nil {
		t.Fatal(err)
	}
	if err := sound.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	if err := sound.SetVolume(.5); !errors.Is(err, ErrClosed) {
		t.Errorf("SetVolume after Close: %v, want ErrClosed", err)
	}
}

func TestPlayerCallbacksAndState(t *testing.T) {
	backend := &fakeBackend{}
	previous := targetBackend
	targetBackend = backend
	t.Cleanup(func() { targetBackend = previous })

	player, err := Open("song.mp3", WithProgressInterval(time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	if got := backend.playerOptions.ProgressInterval; got != minProgressInterval {
		t.Fatalf("progress interval %s, want %s", got, minProgressInterval)
	}
	progress := make(chan time.Duration, 1)
	ended := make(chan struct{})
	if err := player.OnProgress(func(position, duration time.Duration) { progress <- position + duration }); err != nil {
		t.Fatal(err)
	}
	if err := player.OnEnd(func() { close(ended) }); err != nil {
		t.Fatal(err)
	}
	backend.report(2*time.Second, 10*time.Second)
	select {
	case got := <-progress:
		if got != 12*time.Second {
			t.Errorf("progress sum %s, want 12s", got)
		}
	case <-time.After(time.Second):
		t.Fatal("progress callback did not run")
	}
	if player.Position() != 2*time.Second || player.Duration() != 10*time.Second {
		t.Errorf("state is %s/%s, want 2s/10s", player.Position(), player.Duration())
	}
	backend.playerEnd()
	select {
	case <-ended:
	case <-time.After(time.Second):
		t.Fatal("player end callback did not run")
	}
	if !player.Ended() || player.Playing() {
		t.Errorf("ended=%v playing=%v, want true/false", player.Ended(), player.Playing())
	}
	if err := player.Close(); err != nil {
		t.Fatal(err)
	}
	if err := player.Play(); !errors.Is(err, ErrClosed) {
		t.Errorf("Play after Close: %v, want ErrClosed", err)
	}
}

func TestPlayerPositionFollowsALoop(t *testing.T) {
	backend := &fakeBackend{}
	previous := targetBackend
	targetBackend = backend
	t.Cleanup(func() { targetBackend = previous })

	player, err := Open("song.mp3", WithLoop(true))
	if err != nil {
		t.Fatal(err)
	}
	progress := make(chan time.Duration, 2)
	if err := player.OnProgress(func(position, _ time.Duration) { progress <- position }); err != nil {
		t.Fatal(err)
	}
	if err := player.Play(); err != nil {
		t.Fatal(err)
	}
	backend.report(9*time.Second, 10*time.Second)
	backend.report(time.Second, 10*time.Second)
	var last time.Duration
	deadline := time.After(time.Second)
	for last != time.Second {
		select {
		case last = <-progress:
		case <-deadline:
			t.Fatalf("last progress %s, want 1s after the loop", last)
		}
	}
	if got := player.Position(); got != time.Second {
		t.Errorf("Position %s after the loop, want 1s", got)
	}
	if err := player.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestPlayerPausedByTheSystem(t *testing.T) {
	backend := &fakeBackend{}
	previous := targetBackend
	targetBackend = backend
	t.Cleanup(func() { targetBackend = previous })

	player, err := Open("song.mp3")
	if err != nil {
		t.Fatal(err)
	}
	if err := player.Play(); err != nil {
		t.Fatal(err)
	}
	if !player.Playing() {
		t.Fatal("Playing is false after Play")
	}
	backend.player.mu.Lock()
	backend.player.active = false
	backend.player.mu.Unlock()
	backend.playerPaused()
	if player.Playing() {
		t.Error("Playing is true after the system paused the player")
	}
	if err := player.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestPlayerCloseWaitsForCallsInProgress(t *testing.T) {
	backend := &fakeBackend{}
	previous := targetBackend
	targetBackend = backend
	t.Cleanup(func() { targetBackend = previous })

	player, err := Open("song.mp3")
	if err != nil {
		t.Fatal(err)
	}
	var wait sync.WaitGroup
	for range 4 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for player.Play() == nil {
				player.Position()
				player.Playing()
			}
		}()
	}
	time.Sleep(10 * time.Millisecond)
	if err := player.Close(); err != nil {
		t.Fatal(err)
	}
	wait.Wait()
	backend.player.mu.Lock()
	defer backend.player.mu.Unlock()
	if backend.player.afterClose != 0 {
		t.Errorf("%d backend calls after close", backend.player.afterClose)
	}
}

type fakeBackend struct {
	soundOptions   Options
	playerOptions  Options
	soundEnd       func()
	playerProgress func(time.Duration, time.Duration)
	playerPaused   func()
	playerEnd      func()
	sound          fakeSound
	player         fakePlayer
}

// report moves the fake player to position and reports it, as a native player does on its progress timer.
func (backend *fakeBackend) report(position, duration time.Duration) {
	backend.player.mu.Lock()
	backend.player.pos = position
	backend.player.dur = duration
	backend.player.mu.Unlock()
	backend.playerProgress(position, duration)
}

func (backend *fakeBackend) openSound(_ source, options Options, end func()) (soundBackend, error) {
	backend.soundOptions = options
	backend.soundEnd = end
	return &backend.sound, nil
}

func (backend *fakeBackend) openPlayer(_ source, options Options, callbacks playerCallbacks) (playerBackend, error) {
	backend.playerOptions = options
	backend.playerProgress = callbacks.progress
	backend.playerPaused = callbacks.paused
	backend.playerEnd = callbacks.end
	return &backend.player, nil
}

type fakeSound struct {
	closed bool
}

func (sound *fakeSound) setVolume(float64) error { return nil }
func (sound *fakeSound) stop() error             { return nil }
func (sound *fakeSound) close() error {
	sound.closed = true
	return nil
}

type fakePlayer struct {
	mu         sync.Mutex
	pos        time.Duration
	dur        time.Duration
	active     bool
	closed     bool
	afterClose int
}

func (player *fakePlayer) play() error {
	player.mu.Lock()
	if player.closed {
		player.afterClose++
	}
	player.active = true
	player.mu.Unlock()
	return nil
}
func (player *fakePlayer) pause() error {
	player.mu.Lock()
	player.active = false
	player.mu.Unlock()
	return nil
}
func (player *fakePlayer) stop() error {
	player.mu.Lock()
	player.active = false
	player.pos = 0
	player.mu.Unlock()
	return nil
}
func (player *fakePlayer) seek(position time.Duration) error {
	player.mu.Lock()
	player.pos = position
	player.mu.Unlock()
	return nil
}
func (player *fakePlayer) setVolume(float64) error { return nil }
func (player *fakePlayer) setRate(float64) error   { return nil }
func (player *fakePlayer) setLoop(bool) error      { return nil }
func (player *fakePlayer) position() time.Duration {
	player.mu.Lock()
	defer player.mu.Unlock()
	if player.closed {
		player.afterClose++
	}
	return player.pos
}
func (player *fakePlayer) duration() time.Duration {
	player.mu.Lock()
	defer player.mu.Unlock()
	return player.dur
}
func (player *fakePlayer) playing() bool {
	player.mu.Lock()
	defer player.mu.Unlock()
	return player.active
}
func (player *fakePlayer) close() error {
	player.mu.Lock()
	player.closed = true
	player.active = false
	player.mu.Unlock()
	return nil
}
