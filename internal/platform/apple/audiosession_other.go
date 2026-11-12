//go:build darwin && !ios

package apple

import "github.com/nsgo-dev/nsgo/internal/driver"

// audioPlaying does nothing on macOS, which plays a video's sound as it
// plays any.
func audioPlaying(*Video, driver.AudioSession) {}
