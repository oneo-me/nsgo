package webstyle

import (
	"testing"
	"time"

	"github.com/oneo-me/nsgo/internal/driver"
)

func TestDefaultsWriteNothing(t *testing.T) {
	vs := driver.DefaultViewStyle
	if Fill(0) != "" || Opacity(1) != "" || Transform(driver.Transform{}) != "" || Transition(driver.Transition{}) != "" ||
		Radius(&vs) != "" || Shadow(&vs) != "" || Filter(nil) != "" {
		t.Fatal("a default value wrote CSS")
	}
}

func TestValues(t *testing.T) {
	if got := Color(driver.RGB(0xff8000)); got != "rgba(255,128,0,1)" {
		t.Errorf("Color = %q", got)
	}
	if got := Px(1.5); got != "1.5px" {
		t.Errorf("Px = %q", got)
	}
	vs := driver.ViewStyle{Corners: driver.Corners{TopLeft: 1, TopRight: 2, BottomRight: 3, BottomLeft: 4}}
	if got := Radius(&vs); got != "1px 2px 3px 4px" {
		t.Errorf("Radius = %q", got)
	}
	tr := driver.Transition{Duration: 200 * time.Millisecond, Properties: driver.TransitionOpacity}
	if got := Transition(tr); got != "opacity 200ms linear 0ms" {
		t.Errorf("Transition = %q", got)
	}
	if got := Decoration(driver.TextDecorationUnderline | driver.TextDecorationLineThrough); got != "underline line-through" {
		t.Errorf("Decoration = %q", got)
	}
	if got := Transform(driver.Transform{TranslateX: 2, Rotate: 90}); got != "translate(2px, 0px) rotate(90deg) scale(1, 1)" {
		t.Errorf("Transform = %q", got)
	}
}
