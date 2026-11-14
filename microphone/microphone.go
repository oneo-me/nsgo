// Package microphone records the system's default input. It reaches the
// system directly, as base/terminal does, and builds on every target,
// reporting where it cannot record.
package microphone

import (
	"encoding/binary"
	"errors"
	"os"
	"sync"
)

var (
	// ErrUnsupported says this target cannot record.
	ErrUnsupported = errors.New("microphone: not supported on this platform")
	// ErrDenied says the user or the system refused the app the microphone.
	ErrDenied = errors.New("microphone: access denied")
	// ErrNoUsageDescription says the process the system holds responsible
	// for this one, such as the app that started it, declares no reason for
	// recording, for which the system would abort this process.
	ErrNoUsageDescription = errors.New("microphone: the responsible app has no NSMicrophoneUsageDescription")
)

// Stream is a recording in progress.
type Stream struct {
	stop  func()
	ended <-chan struct{}
	// SampleRate is the rate the device actually opened. It can differ from
	// the requested rate on platforms that only expose native rates.
	SampleRate int
	// Channels is the number of interleaved channels in samples. Capture is
	// mono today, so this is normally one.
	Channels int
}

// Stop ends the recording. Once it returns no more samples arrive, and
// calling it again does nothing.
func (s *Stream) Stop() { s.stop() }

// Ended closes once no more samples will arrive: after Stop, or once the
// system took the microphone away, as iOS does for a call or when the app
// leaves the screen. The recording still needs its Stop then.
func (s *Stream) Ended() <-chan struct{} { return s.ended }

// CaptureOptions configures CaptureWithOptions. Samples and Level are called
// from the platform's recording thread; callbacks must return promptly.
type CaptureOptions struct {
	Rate    int
	Samples func([]int16)
	// Level receives a normalized RMS level in [0, 1], at least ten times a
	// second while data arrives when the platform can provide it.
	Level func(float32)
}

// CaptureInfo describes the format the device opened after fallback.
type CaptureInfo struct {
	SampleRate int
	Channels   int
}

// CaptureWAV records directly to path while still delivering optional PCM
// and level callbacks. Samples are streamed; the file is never held in
// memory. The WAV header is finalized when the stream stops or ends.
func CaptureWAV(path string, o CaptureOptions) (*Stream, CaptureInfo, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, CaptureInfo{}, err
	}
	if _, err := f.Write(make([]byte, 44)); err != nil {
		f.Close()
		return nil, CaptureInfo{}, err
	}
	var mu sync.Mutex
	var dataBytes uint32
	finish := func(info CaptureInfo) {
		mu.Lock()
		defer mu.Unlock()
		if f == nil {
			return
		}
		header := make([]byte, 44)
		copy(header, "RIFF")
		binary.LittleEndian.PutUint32(header[4:], 36+dataBytes)
		copy(header[8:], "WAVEfmt ")
		binary.LittleEndian.PutUint32(header[16:], 16)
		binary.LittleEndian.PutUint16(header[20:], 1)
		binary.LittleEndian.PutUint16(header[22:], uint16(info.Channels))
		binary.LittleEndian.PutUint32(header[24:], uint32(info.SampleRate))
		binary.LittleEndian.PutUint32(header[28:], uint32(info.SampleRate*info.Channels*2))
		binary.LittleEndian.PutUint16(header[32:], uint16(info.Channels*2))
		binary.LittleEndian.PutUint16(header[34:], 16)
		copy(header[36:], "data")
		binary.LittleEndian.PutUint32(header[40:], dataBytes)
		_, _ = f.WriteAt(header, 0)
		_ = f.Close()
		f = nil
	}
	original := o.Samples
	o.Samples = func(samples []int16) {
		mu.Lock()
		if f != nil && len(samples) > 0 {
			bytes := make([]byte, len(samples)*2)
			for i, sample := range samples {
				binary.LittleEndian.PutUint16(bytes[i*2:], uint16(sample))
			}
			if _, err := f.Write(bytes); err == nil {
				dataBytes += uint32(len(bytes))
			}
		}
		mu.Unlock()
		if original != nil {
			original(samples)
		}
	}
	s, info, err := CaptureWithOptions(o)
	if err != nil {
		_ = f.Close()
		return nil, CaptureInfo{}, err
	}
	var once sync.Once
	closeFile := func() { once.Do(func() { finish(info) }) }
	oldStop := s.stop
	s.stop = func() { oldStop(); closeFile() }
	go func() { <-s.ended; closeFile() }()
	return s, info, nil
}
