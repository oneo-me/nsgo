package audio

import "sync"

// Sound is a short-lived, one-shot audio cue.
type Sound struct {
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
	sound.mu.Lock()
	if sound.closed {
		sound.mu.Unlock()
		return ErrClosed
	}
	backend := sound.backend
	sound.mu.Unlock()
	return backend.stop()
}

// OnEnd registers the callback called when the cue finishes. Passing nil
// removes the callback. Callbacks run serially on the package callback queue.
func (sound *Sound) OnEnd(callback func()) error {
	sound.mu.Lock()
	defer sound.mu.Unlock()
	if sound.closed {
		return ErrClosed
	}
	sound.onEnd = callback
	return nil
}

// Close releases the native player. It is idempotent.
func (sound *Sound) Close() error {
	sound.mu.Lock()
	if sound.closed {
		sound.mu.Unlock()
		return nil
	}
	sound.closed = true
	backend := sound.backend
	sound.mu.Unlock()
	return backend.close()
}

func (sound *Sound) endedByBackend() {
	sound.mu.Lock()
	if sound.closed || sound.ended {
		sound.mu.Unlock()
		return
	}
	sound.ended = true
	sound.mu.Unlock()
	callbacks.add(func() {
		sound.mu.Lock()
		callback := sound.onEnd
		closed := sound.closed
		sound.mu.Unlock()
		if !closed && callback != nil {
			callback()
		}
	})
}
