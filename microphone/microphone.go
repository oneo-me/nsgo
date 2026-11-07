// Package microphone records the system's default input. It reaches the
// system directly, as base/terminal does, and builds on every target,
// reporting where it cannot record.
package microphone

import "errors"

var (
	// ErrUnsupported says this target cannot record.
	ErrUnsupported = errors.New("microphone: not supported on this platform")
	// ErrDenied says the user or the system refused the app the microphone.
	ErrDenied = errors.New("microphone: access denied")
	// ErrNoUsageDescription says the process the system holds responsible
	// for this one, such as the app that started it, declares no reason for
	// recording, for which macOS would abort this process.
	ErrNoUsageDescription = errors.New("microphone: the responsible app has no NSMicrophoneUsageDescription")
)

// Stream is a recording in progress.
type Stream struct{ stop func() }

// Stop ends the recording. Once it returns no more samples arrive, and
// calling it again does nothing.
func (s *Stream) Stop() { s.stop() }
