package audio

import (
	"math"
	"sync"
	"time"
)

// Player controls long-form audio.
type Player struct {
	// calls is held for reading across each call into the backend and for writing while Close releases it, so no call reaches a released native player.
	calls          sync.RWMutex
	mu             sync.Mutex
	backend        playerBackend
	closed         bool
	ended          bool
	position       time.Duration
	duration       time.Duration
	playing        bool
	progressQueued bool
	err            error
	onEnd          func()
	onProgress     func(position, duration time.Duration)
}

// Play starts or resumes playback.
func (player *Player) Play() error {
	if err := player.command(func(backend playerBackend) error { return backend.play() }); err != nil {
		return err
	}
	player.mu.Lock()
	player.playing = true
	player.ended = false
	player.mu.Unlock()
	return nil
}

// Pause pauses playback while keeping the current position.
func (player *Player) Pause() error {
	if err := player.command(func(backend playerBackend) error { return backend.pause() }); err != nil {
		return err
	}
	player.mu.Lock()
	player.playing = false
	player.mu.Unlock()
	return nil
}

// Stop stops playback and returns to the beginning.
func (player *Player) Stop() error {
	if err := player.command(func(backend playerBackend) error { return backend.stop() }); err != nil {
		return err
	}
	player.mu.Lock()
	player.playing = false
	player.ended = false
	player.position = 0
	player.mu.Unlock()
	return nil
}

// Seek moves playback to position. A backend may queue this until metadata is
// available.
func (player *Player) Seek(position time.Duration) error {
	if position < 0 {
		return ErrInvalidOption
	}
	if err := player.command(func(backend playerBackend) error { return backend.seek(position) }); err != nil {
		return err
	}
	player.mu.Lock()
	player.position = position
	player.ended = false
	player.mu.Unlock()
	return nil
}

// SetVolume changes the volume from 0 (silent) to 1 (full volume).
func (player *Player) SetVolume(volume float64) error {
	if err := validVolume(volume); err != nil {
		return err
	}
	return player.command(func(backend playerBackend) error { return backend.setVolume(volume) })
}

// SetRate changes the playback rate. The rate must be greater than zero.
func (player *Player) SetRate(rate float64) error {
	if math.IsNaN(rate) || math.IsInf(rate, 0) || rate <= 0 {
		return ErrInvalidOption
	}
	return player.command(func(backend playerBackend) error { return backend.setRate(rate) })
}

// SetLoop controls whether playback restarts at the end.
func (player *Player) SetLoop(loop bool) error {
	return player.command(func(backend playerBackend) error { return backend.setLoop(loop) })
}

// Position returns the backend's current position. It moves back when playback seeks, stops or loops. After Close it returns the last position the player knew.
func (player *Player) Position() time.Duration {
	player.calls.RLock()
	defer player.calls.RUnlock()
	player.mu.Lock()
	if player.closed {
		position := player.position
		player.mu.Unlock()
		return position
	}
	backend := player.backend
	player.mu.Unlock()
	return backend.position()
}

// Duration returns the latest known duration, or zero while metadata is not
// available.
func (player *Player) Duration() time.Duration {
	player.calls.RLock()
	defer player.calls.RUnlock()
	player.mu.Lock()
	if player.closed {
		duration := player.duration
		player.mu.Unlock()
		return duration
	}
	backend := player.backend
	duration := player.duration
	player.mu.Unlock()
	if current := backend.duration(); current > duration {
		duration = current
	}
	return duration
}

// Playing says whether the backend is currently producing audio.
func (player *Player) Playing() bool {
	player.calls.RLock()
	defer player.calls.RUnlock()
	player.mu.Lock()
	if player.closed {
		player.mu.Unlock()
		return false
	}
	backend := player.backend
	playing := player.playing
	player.mu.Unlock()
	return backend.playing() || playing
}

// Ended says whether the source reached its end.
func (player *Player) Ended() bool {
	player.mu.Lock()
	defer player.mu.Unlock()
	return player.ended
}

// Err returns the asynchronous backend error, if one has occurred.
func (player *Player) Err() error {
	player.mu.Lock()
	defer player.mu.Unlock()
	return player.err
}

// OnProgress registers a callback for progress updates. Passing nil removes
// the callback. Callbacks run serially on the package callback queue.
func (player *Player) OnProgress(callback func(position, duration time.Duration)) error {
	player.mu.Lock()
	defer player.mu.Unlock()
	if player.closed {
		return ErrClosed
	}
	player.onProgress = callback
	return nil
}

// OnEnd registers the callback called when playback reaches its end. Passing
// nil removes the callback. Callbacks run serially on the package callback
// queue.
func (player *Player) OnEnd(callback func()) error {
	player.mu.Lock()
	defer player.mu.Unlock()
	if player.closed {
		return ErrClosed
	}
	player.onEnd = callback
	return nil
}

// Close releases the native player. It is idempotent, waits for the player's calls already in progress, and prevents callbacks that have not begun from reaching app code.
func (player *Player) Close() error {
	player.mu.Lock()
	if player.closed {
		player.mu.Unlock()
		return nil
	}
	player.closed = true
	player.playing = false
	backend := player.backend
	player.mu.Unlock()
	player.calls.Lock()
	defer player.calls.Unlock()
	return backend.close()
}

func (player *Player) command(command func(playerBackend) error) error {
	player.calls.RLock()
	defer player.calls.RUnlock()
	player.mu.Lock()
	if player.closed {
		player.mu.Unlock()
		return ErrClosed
	}
	backend := player.backend
	player.mu.Unlock()
	return command(backend)
}

func (player *Player) progressed(position, duration time.Duration) {
	if position < 0 {
		position = 0
	}
	if duration < 0 {
		duration = 0
	}
	player.mu.Lock()
	if player.closed {
		player.mu.Unlock()
		return
	}
	if duration > player.duration {
		player.duration = duration
	}
	player.position = position
	if player.progressQueued {
		player.mu.Unlock()
		return
	}
	player.progressQueued = true
	player.mu.Unlock()
	callbacks.add(player.deliverProgress)
}

// deliverProgress reports the latest position and duration, so reports that arrive while an earlier one waits in the queue coalesce into it.
func (player *Player) deliverProgress() {
	player.mu.Lock()
	player.progressQueued = false
	closed := player.closed
	callback := player.onProgress
	position, duration := player.position, player.duration
	player.mu.Unlock()
	if !closed && callback != nil {
		callback(position, duration)
	}
}

func (player *Player) pausedByBackend() {
	player.mu.Lock()
	player.playing = false
	player.mu.Unlock()
}

func (player *Player) endedByBackend() {
	player.mu.Lock()
	if player.closed {
		player.mu.Unlock()
		return
	}
	player.ended = true
	player.playing = false
	callback := player.onEnd
	player.mu.Unlock()
	callbacks.add(func() {
		player.mu.Lock()
		closed := player.closed
		player.mu.Unlock()
		if !closed && callback != nil {
			callback()
		}
	})
}

func (player *Player) failed(err error) {
	if err == nil {
		return
	}
	player.mu.Lock()
	if !player.closed {
		player.err = err
		player.playing = false
	}
	player.mu.Unlock()
}

func validVolume(volume float64) error {
	if math.IsNaN(volume) || math.IsInf(volume, 0) || volume < 0 || volume > 1 {
		return ErrInvalidOption
	}
	return nil
}
