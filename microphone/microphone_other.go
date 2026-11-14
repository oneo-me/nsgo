//go:build !darwin && !android

package microphone

import "github.com/nsgo-dev/nsgo/internal/recording"

// Supported says this target can record.
func Supported() bool { return recording.Current() != nil }

// Allowed reports false, as this target cannot record.
func Allowed() bool {
	b := recording.Current()
	return b != nil && b.Allowed()
}

// Authorize reports ErrUnsupported.
func Authorize() error {
	if b := recording.Current(); b != nil {
		return b.Authorize()
	}
	return ErrUnsupported
}

// Capture reports ErrUnsupported.
func Capture(rate int, samples func([]int16)) (*Stream, error) {
	s, _, err := CaptureWithOptions(CaptureOptions{Rate: rate, Samples: samples})
	return s, err
}

// CaptureWithOptions reports that this target has no native recorder.
func CaptureWithOptions(o CaptureOptions) (*Stream, CaptureInfo, error) {
	if b := recording.Current(); b != nil {
		r, err := b.Capture(o.Rate, o.Samples, o.Level)
		if err != nil {
			return nil, CaptureInfo{}, err
		}
		return &Stream{stop: r.Stop, ended: r.Ended, SampleRate: r.SampleRate, Channels: r.Channels}, CaptureInfo{SampleRate: r.SampleRate, Channels: r.Channels}, nil
	}
	return nil, CaptureInfo{}, ErrUnsupported
}
