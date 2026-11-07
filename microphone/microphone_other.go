//go:build !darwin

package microphone

// Supported says this target can record.
func Supported() bool { return false }

// Allowed reports false, as this target cannot record.
func Allowed() bool { return false }

// Authorize reports ErrUnsupported.
func Authorize() error { return ErrUnsupported }

// Capture reports ErrUnsupported.
func Capture(int, func([]int16)) (*Stream, error) { return nil, ErrUnsupported }
