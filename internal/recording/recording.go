// Package recording is the small platform registration seam used by the
// standalone microphone API. It contains no platform or view dependencies.
package recording

import (
	"errors"
	"sync"
)

var ErrUnavailable = errors.New("microphone: no recorder is registered")

type Info struct {
	SampleRate int
	Channels   int
}

type Session struct {
	Stop       func()
	Ended      <-chan struct{}
	SampleRate int
	Channels   int
}

type Backend interface {
	Capture(rate int, samples func([]int16), level func(float32)) (*Session, error)
	Allowed() bool
	Authorize() error
}

var backends struct {
	sync.RWMutex
	value Backend
}

func Register(b Backend) {
	backends.Lock()
	backends.value = b
	backends.Unlock()
}

func Current() Backend {
	backends.RLock()
	b := backends.value
	backends.RUnlock()
	return b
}
