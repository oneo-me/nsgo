//go:build darwin

package apple

import (
	"testing"

	"github.com/oneo-me/nsgo/internal/driver"
)

// An animation's frames that move or fade a view set only its transform
// and opacity; any other change draws the whole style again.
func TestOnlyMovesSetsJustTransformAndOpacity(t *testing.T) {
	base := driver.ViewStyle{Background: driver.RGB(0x336699), CornerRadius: 8, Opacity: 1,
		Effects: &driver.Effects{Shadows: []driver.BoxShadow{{Blur: 4, Color: driver.RGB(0)}}}}
	moved := base
	moved.Transform = driver.Transform{TranslateX: 12, Rotate: 30}
	moved.Opacity = 0.5
	recolored := base
	recolored.Background = driver.RGB(0x993366)
	filtered := base
	filtered.Effects = &driver.Effects{Filters: []driver.Filter{{Kind: driver.FilterBlur, Amount: 2}}}
	faded := filtered
	faded.Opacity = 0.5
	turned := filtered
	turned.Transform.Rotate = 10

	for _, c := range []struct {
		name string
		a, b driver.ViewStyle
		want bool
	}{
		{"moved and faded", base, moved, true},
		{"recolored", base, recolored, false},
		{"other effects", base, filtered, false},
		// iOS draws an opacity under filters with them.
		{"faded under filters", filtered, faded, false},
		{"turned under filters", filtered, turned, true},
	} {
		if got := onlyMoves(&c.a, &c.b); got != c.want {
			t.Errorf("%s: onlyMoves = %v, want %v", c.name, got, c.want)
		}
	}
}
