//go:build android

package microphone

import (
	"math"

	"github.com/nsgo-dev/nsgo/internal/recording"
)

// Supported says Android's AudioRecord runtime is available.
func Supported() bool { return recording.Current() != nil }

// Allowed reports whether RECORD_AUDIO is currently granted. Android may
// still show a prompt from CaptureWithOptions when this is false.
func Allowed() bool {
	b := recording.Current()
	return b != nil && b.Allowed()
}

// Authorize reports whether the Android runtime is available. The dangerous
// permission prompt is owned by the runtime's activity and is requested by
// CaptureWithOptions immediately before AudioRecord starts.
func Authorize() error {
	b := recording.Current()
	if b == nil {
		return recording.ErrUnavailable
	}
	return b.Authorize()
}

// Capture starts a mono PCM16 recording at rate hertz.
func Capture(rate int, samples func([]int16)) (*Stream, error) {
	s, _, err := CaptureWithOptions(CaptureOptions{Rate: rate, Samples: samples})
	return s, err
}

// CaptureWithOptions starts a long-running AudioRecord foreground service.
// Android reports the actual sample rate when it has to fall back.
func CaptureWithOptions(o CaptureOptions) (*Stream, CaptureInfo, error) {
	b := recording.Current()
	if b == nil {
		return nil, CaptureInfo{}, recording.ErrUnavailable
	}
	r, err := b.Capture(o.Rate, o.Samples, o.Level)
	if err != nil {
		return nil, CaptureInfo{}, err
	}
	return &Stream{stop: r.Stop, ended: r.Ended, SampleRate: r.SampleRate, Channels: r.Channels},
		CaptureInfo{SampleRate: r.SampleRate, Channels: r.Channels}, nil
}

// rms is kept here for callers that use a platform implementation without a
// native level callback in tests.
func rms(samples []int16) float32 {
	if len(samples) == 0 {
		return 0
	}
	var sum float64
	for _, sample := range samples {
		v := float64(sample) / 32768
		sum += v * v
	}
	return float32(math.Sqrt(sum / float64(len(samples))))
}
