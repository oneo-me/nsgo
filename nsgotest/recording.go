package nsgotest

import (
	"errors"
	"sync"

	"github.com/nsgo-dev/nsgo/internal/recording"
)

// Microphone is the deterministic recorder used by headless tests. Emit
// supplies PCM and a level to every active Capture call; End ends them.
type Microphone struct {
	Allowed bool
	Rate    int

	Starts   int
	Stops    int
	mu       sync.Mutex
	sessions map[*microphoneSession]struct{}
}

// Microphone configures Start with a fake microphone.
func MicrophoneInput(m *Microphone) Option {
	return func(a *App) { a.microphone = m }
}

func (m *Microphone) Emit(samples []int16, level float32) {
	m.mu.Lock()
	all := make([]*microphoneSession, 0, len(m.sessions))
	for s := range m.sessions {
		all = append(all, s)
	}
	m.mu.Unlock()
	for _, s := range all {
		if s.level != nil {
			s.level(level)
		}
		if s.samples != nil {
			s.samples(samples)
		}
	}
}

func (m *Microphone) End() {
	m.mu.Lock()
	all := make([]*microphoneSession, 0, len(m.sessions))
	for s := range m.sessions {
		all = append(all, s)
	}
	m.mu.Unlock()
	for _, s := range all {
		s.stop()
	}
}

type microphoneSession struct {
	owner   *Microphone
	samples func([]int16)
	level   func(float32)
	ended   chan struct{}
	once    sync.Once
}

func (s *microphoneSession) stop() {
	s.once.Do(func() {
		s.owner.mu.Lock()
		delete(s.owner.sessions, s)
		s.owner.Stops++
		s.owner.mu.Unlock()
		close(s.ended)
	})
}

type microphoneBackend struct{ m *Microphone }

func (b microphoneBackend) Allowed() bool { return b.m.Allowed }
func (b microphoneBackend) Authorize() error {
	if !b.m.Allowed {
		return errors.New("microphone: access denied")
	}
	return nil
}
func (b microphoneBackend) Capture(rate int, samples func([]int16), level func(float32)) (*recording.Session, error) {
	if err := b.Authorize(); err != nil {
		return nil, err
	}
	if rate <= 0 {
		rate = b.m.Rate
	}
	if rate <= 0 {
		rate = 44100
	}
	s := &microphoneSession{owner: b.m, samples: samples, level: level, ended: make(chan struct{})}
	b.m.mu.Lock()
	if b.m.sessions == nil {
		b.m.sessions = make(map[*microphoneSession]struct{})
	}
	b.m.sessions[s] = struct{}{}
	b.m.Starts++
	b.m.mu.Unlock()
	return &recording.Session{Stop: s.stop, Ended: s.ended, SampleRate: rate, Channels: 1}, nil
}
