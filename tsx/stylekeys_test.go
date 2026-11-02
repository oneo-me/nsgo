package tsx

import (
	"testing"

	"github.com/oneo-me/nsgo/core"
)

func TestTransitionPropertiesNameLayout(t *testing.T) {
	for s, layout := range map[string]bool{"width": true, "max-height, opacity": true, "inset": true, "all": true,
		"colors": false, "transform": false} {
		if got := transitionProperties(s)&core.TransitionLayout != 0; got != layout {
			t.Errorf("%q moves layout: %v", s, got)
		}
	}
}
