package audio

import "sync"

// Sound is a short-lived, one-shot audio cue.
type Sound struct {
	// calls is held for reading across each call into the backend and for writing while Close releases it, so no call reaches a released native player.
	calls   sync.RWMutex
	mu      sync.Mutex
	backend soundBackend
	closed  bool
	ended   bool
	onEnd   func()
}

// SetVolume changes the cue's volume from 0 (silent) to 1 (full volume).
func (sound *Sound) SetVolume(volume float64) error {
	if err := validVolume(volume); err != nil {
		return err
	}
	sound.calls.RLock()
	defer sound.calls.RUnlock()
	sound.mu.Lock()
	if sound.closed {
		sound.mu.Unlock()
		return ErrClosed
	}
	backend := sound.backend
	sound.mu.Unlock()
	return backend.setVolume(volume)
}

// Stop stops the cue. It is safe to call Stop more than once.
func (sound *Sound) Stop() error {
	sound.calls.RLock()
	defer sound.calls.RUnlock()
	sound.mu.Lock()
	if sound.closed {
		sound.mu.Unlock()
		return ErrClosed
	}
	backend := sound.backend
	sound.mu.Unlock()
	return backend.stop()
}

// OnEnd registers the callback called when the cue finishes. A callback registered after the cue has finished is called once, at once, so a cue that ends before OnEnd is not missed. Passing nil removes the callback. Callbacks run serially on the package callback queue.
func (sound *Sound) OnEnd(callback func()) error {
	sound.mu.Lock()
	defer sound.mu.Unlock()
	if sound.closed {
		return ErrClosed
	}
	sound.onEnd = callback
	if sound.ended && callback != nil {
		callbacks.add(func() { sound.deliverEnd(callback) })
	}
	return nil
}

// Close releases the native player. It is idempotent and waits for the cue's calls already in progress.
func (sound *Sound) Close() error {
	sound.mu.Lock()
	if sound.closed {
		sound.mu.Unlock()
		return nil
	}
	sound.closed = true
	backend := sound.backend
	sound.mu.Unlock()
	sound.calls.Lock()
	defer sound.calls.Unlock()
	return backend.close()
}

func (sound *Sound) endedByBackend() {
	sound.mu.Lock()
	if sound.closed || sound.ended {
		sound.mu.Unlock()
		return
	}
	sound.ended = true
	callback := sound.onEnd
	sound.mu.Unlock()
	if callback != nil {
		callbacks.add(func() { sound.deliverEnd(callback) })
	}
}

func (sound *Sound) deliverEnd(callback func()) {
	sound.mu.Lock()
	closed := sound.closed
	sound.mu.Unlock()
	if !closed {
		callback()
	}
}
