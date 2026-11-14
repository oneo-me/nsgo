//go:build !darwin && !js

package widget

import "time"

func publish(string, []Entry, time.Time) error { return ErrUnsupported }
func available() bool                          { return false }
