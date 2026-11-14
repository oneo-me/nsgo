// Package audio plays short sounds and long-form audio without rendering a
// view. The package is portable; each target supplies a backend, and a target
// without one reports ErrUnsupported.
package audio

import (
	"errors"
	"fmt"
	"math"
	"time"
)

// Session controls how playback shares the process's audio session.
type Session uint8

const (
	// AudioSessionDefault lets the package choose: a Sound mixes with other audio, as AudioSessionMixWithOthers does, so a cue never interrupts it, and a Player leaves the platform's existing session policy alone.
	AudioSessionDefault Session = iota
	// AudioSessionPlayback asks for an exclusive playback session while audio
	// is audible.
	AudioSessionPlayback
	// AudioSessionMixWithOthers keeps other audio playing while this package
	// produces sound.
	AudioSessionMixWithOthers
)

// Options controls a sound or player when it is opened.
//
// The zero value uses AudioSessionDefault, full volume, normal speed, no loop and a 250 ms progress interval.
type Options struct {
	Session          Session
	Volume           float64
	Rate             float64
	Loop             bool
	ProgressInterval time.Duration
}

const (
	defaultVolume           = 1
	defaultRate             = 1
	defaultProgressInterval = 250 * time.Millisecond
	minProgressInterval     = 50 * time.Millisecond
)

// Option changes an Options value passed to Open, OpenBytes, PlayFile or
// PlayBytes.
type Option func(*Options)

// WithSession selects the audio session policy.
func WithSession(session Session) Option {
	return func(options *Options) { options.Session = session }
}

// WithVolume sets the initial volume from 0 (silent) to 1 (full volume).
func WithVolume(volume float64) Option {
	return func(options *Options) { options.Volume = volume }
}

// WithRate sets the initial playback rate. A rate must be greater than zero.
func WithRate(rate float64) Option {
	return func(options *Options) { options.Rate = rate }
}

// WithLoop makes a player restart when it reaches the end. It has no effect
// on a Sound, which is always a one-shot cue.
func WithLoop(loop bool) Option {
	return func(options *Options) { options.Loop = loop }
}

// WithProgressInterval requests how often a Player reports progress. The
// backend clamps intervals below 50 ms to 50 ms.
func WithProgressInterval(interval time.Duration) Option {
	return func(options *Options) { options.ProgressInterval = interval }
}

// Errors returned by this package. Backends may wrap these values with
// details from the target API; callers can use errors.Is to classify them.
var (
	// ErrUnsupported says this target has no audio backend.
	ErrUnsupported = errors.New("audio: not supported on this platform")
	// ErrUnreadableSource says a path or encoded byte source could not be read.
	ErrUnreadableSource = errors.New("audio: source cannot be read")
	// ErrUnsupportedFormat says the source's codec or MIME type is not known.
	ErrUnsupportedFormat = errors.New("audio: format is not supported")
	// ErrDeviceUnavailable says the output device cannot be used.
	ErrDeviceUnavailable = errors.New("audio: output device is unavailable")
	// ErrClosed says an operation was attempted after Close.
	ErrClosed = errors.New("audio: player is closed")
	// ErrInvalidOption says an option has an invalid value.
	ErrInvalidOption = errors.New("audio: invalid option")
)

// Open opens a long-form player from path. It returns after the backend has
// accepted the source; metadata and decoding may arrive asynchronously.
func Open(path string, options ...Option) (*Player, error) {
	if path == "" {
		return nil, fmt.Errorf("%w: empty path", ErrUnreadableSource)
	}
	settings, err := resolveOptions(options)
	if err != nil {
		return nil, err
	}
	return openPlayer(source{path: path}, settings)
}

// OpenBytes opens a long-form player from encoded bytes and their MIME type.
// The bytes are copied before the backend is called, so the caller may reuse
// or modify its buffer after this function returns.
func OpenBytes(data []byte, mime string, options ...Option) (*Player, error) {
	src, err := bytesSource(data, mime)
	if err != nil {
		return nil, err
	}
	settings, err := resolveOptions(options)
	if err != nil {
		return nil, err
	}
	return openPlayer(src, settings)
}

// PlayFile starts a short-lived cue from path.
func PlayFile(path string, options ...Option) (*Sound, error) {
	if path == "" {
		return nil, fmt.Errorf("%w: empty path", ErrUnreadableSource)
	}
	settings, err := resolveOptions(options)
	if err != nil {
		return nil, err
	}
	return playSound(source{path: path}, settings)
}

// PlayBytes starts a short-lived cue from encoded bytes and their MIME type.
// The bytes are copied before the backend is called.
func PlayBytes(data []byte, mime string, options ...Option) (*Sound, error) {
	src, err := bytesSource(data, mime)
	if err != nil {
		return nil, err
	}
	settings, err := resolveOptions(options)
	if err != nil {
		return nil, err
	}
	return playSound(src, settings)
}

func bytesSource(data []byte, mime string) (source, error) {
	if len(data) == 0 {
		return source{}, fmt.Errorf("%w: empty byte source", ErrUnreadableSource)
	}
	if mime == "" {
		return source{}, fmt.Errorf("%w: empty MIME type", ErrUnsupportedFormat)
	}
	return source{data: append([]byte(nil), data...), mime: mime}, nil
}

func resolveOptions(overrides []Option) (Options, error) {
	options := Options{Volume: defaultVolume, Rate: defaultRate, ProgressInterval: defaultProgressInterval}
	for _, override := range overrides {
		if override != nil {
			override(&options)
		}
	}
	if options.Session > AudioSessionMixWithOthers {
		return Options{}, fmt.Errorf("%w: unknown session %d", ErrInvalidOption, options.Session)
	}
	if math.IsNaN(options.Volume) || math.IsInf(options.Volume, 0) || options.Volume < 0 || options.Volume > 1 {
		return Options{}, fmt.Errorf("%w: volume must be between 0 and 1", ErrInvalidOption)
	}
	if math.IsNaN(options.Rate) || math.IsInf(options.Rate, 0) || options.Rate <= 0 {
		return Options{}, fmt.Errorf("%w: rate must be greater than zero", ErrInvalidOption)
	}
	if options.ProgressInterval == 0 {
		options.ProgressInterval = defaultProgressInterval
	} else if options.ProgressInterval < minProgressInterval {
		if options.ProgressInterval < 0 {
			return Options{}, fmt.Errorf("%w: progress interval cannot be negative", ErrInvalidOption)
		}
		options.ProgressInterval = minProgressInterval
	}
	return options, nil
}
