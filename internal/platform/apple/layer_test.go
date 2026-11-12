//go:build darwin

package apple

import (
	"math"
	"testing"
	"time"

	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/driver"
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

// A transition's turn stays on the transform origin all the way, though
// the anchor point sits at the corner as AppKit keeps it: its steps hold
// the origin still and turn no further than Core Animation can tween. A
// new target goes on from the angle reached, not the last one drawn.
func TestTransformTransitionTurnsAroundTheOrigin(t *testing.T) {
	layer := objc.ID(objc.GetClass("CALayer")).Send(selNew)
	defer layer.Send(selRelease)
	defer ForgetTransform(layer)
	layer.Send(sel("setAnchorPoint:"), CGPoint{})
	layer.Send(sel("setBounds:"), CGRect{Size: CGSize{Width: 40, Height: 40}})
	transition := driver.Transition{Duration: time.Hour, Properties: driver.TransitionTransform}
	closed := driver.ViewStyle{Opacity: 1}
	open := driver.ViewStyle{Opacity: 1, Transform: driver.Transform{Rotate: 180}, Transition: transition}
	StyleLayer(layer, nil, &closed, 0)

	steps := func() []transform3D {
		values := layer.Send(sel("animationForKey:"), NSString("nsgo.transform")).Send(sel("values"))
		out := make([]transform3D, objc.Send[int](values, sel("count")))
		for i := range out {
			out[i] = objc.Send[transform3D](values.Send(sel("objectAtIndex:"), i), sel("CATransform3DValue"))
		}
		return out
	}
	angle := func(m transform3D) float64 { return math.Atan2(m.M12, m.M11) * 180 / math.Pi }
	StyleLayer(layer, &closed, &open, 0)
	got := steps()
	if len(got) < 3 {
		t.Fatalf("%d steps turn half a circle", len(got))
	}
	for i, m := range got {
		// (20, 20) is the origin, multiplied as CATransform3D's row vectors.
		if x, y := 20*m.M11+20*m.M21+m.M41, 20*m.M12+20*m.M22+m.M42; math.Abs(x-20) > 1e-6 || math.Abs(y-20) > 1e-6 {
			t.Errorf("step %d shows the origin at (%g, %g)", i, x, y)
		}
		if i > 0 {
			if d := math.Abs(math.Mod(angle(m)-angle(got[i-1])+540, 360) - 180); d > turnStep(math.Hypot(20, 20))+1e-6 {
				t.Errorf("step %d turns %g degrees", i, d)
			}
		}
	}
	if a := angle(got[len(got)-1]); math.Abs(math.Abs(a)-180) > 1e-6 {
		t.Errorf("the last step is at %g degrees", a)
	}

	// Halfway through its hour, the turn goes back.
	moves[layer].begin -= moves[layer].duration / 2
	back := closed
	back.Transition = transition
	StyleLayer(layer, &open, &back, 0)
	if a := angle(steps()[0]); math.Abs(a-90) > 1e-3 {
		t.Errorf("the turn back starts at %g degrees, want 90", a)
	}
}
