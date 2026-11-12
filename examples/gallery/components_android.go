//go:build android

package main

import (
	"github.com/nsgo-dev/nsgo/core"
)

// seekBar is Android's SeekBar, as a native component. The Go process
// cannot make Android views, so View names the class the runtime builds
// by reflection; ViewProps turns the value prop into the bar's setters and
// ViewEvent its listener's callbacks into "change" events.
type seekBar struct {
	cx *core.NativeContext
}

// seekSteps is the bar's maximum: its progress is an int.
const seekSteps = 1000

func init() {
	components[sliderComponent] = func(cx *core.NativeContext) core.NativeComponent {
		return &seekBar{cx: cx}
	}
}

func (s *seekBar) View() any { return "android.widget.SeekBar" }

func (s *seekBar) SetProp(string, any) {}

// ViewProps implements core.AndroidPropMapper: value 0..1 sets the progress
// of a bar from 0 to seekSteps.
func (s *seekBar) ViewProps(name string, value any) []core.AndroidViewProp {
	v, ok := value.(float64)
	if name != "value" || !ok {
		return nil
	}
	return []core.AndroidViewProp{{Name: "max", Value: seekSteps}, {Name: "progress", Value: int(v*seekSteps + 0.5)}}
}

// ViewEvent implements core.AndroidEventMapper: SeekBar's
// onProgressChanged(bar, progress, fromUser) reports the user's changes.
func (s *seekBar) ViewEvent(name string, args []any) {
	if name != "onProgressChanged" || len(args) < 3 {
		return
	}
	progress, ok := args[1].(float64)
	if fromUser, _ := args[2].(bool); ok && fromUser {
		s.cx.Emit("change", progress/seekSteps)
	}
}

func (s *seekBar) Command(name string, args any) {
	// The value prop follows the app's state, which the event resets.
	if name == "reset" {
		s.cx.Emit("change", 0.0)
	}
}

func (s *seekBar) Close() {}
