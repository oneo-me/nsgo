package ui

import (
	"testing"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/icons/lucide"
)

// TestSize holds each size's measures, the zero value the standard one.
func TestSize(t *testing.T) {
	for _, c := range []struct {
		size   Size
		height core.Points
		font   core.Points
	}{{SizeDefault, ButtonHeight, 13}, {SizeSmall, 24, 12}, {SizeLarge, 32, 14}} {
		if got := c.size.Height(); got != c.height {
			t.Errorf("size %d is %v tall, want %v", c.size, got, c.height)
		}
		if got := c.size.text().FontSize; got != c.font {
			t.Errorf("size %d sets text at %v, want %v", c.size, got, c.font)
		}
	}
	if Size(0) != SizeDefault {
		t.Fatal("the zero value is not the standard size")
	}
}

// TestSizedControls lays Button and Tabs out at each size, as tall as the
// size says on every device, an icon's button as a square.
func TestSizedControls(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		for _, size := range []Size{SizeDefault, SizeSmall, SizeLarge} {
			app, _ := start(t, e, func(*core.Scope) core.Node {
				return core.View{Style: core.Style{AlignItems: core.AlignStart, Gap: 8}, Children: []core.Node{
					Button{Text: "Text", Size: size, TestID: "button"},
					Button{Icon: lucide.Plus, Label: "Add", Size: size, TestID: "icon"},
					Tabs[int]{Size: size, Options: []Option[int]{{Value: 0, Label: "One", TestID: "tab"}}},
				}}
			})
			want := size.Height()
			for _, id := range []string{"button", "icon", "tab"} {
				if h := app.Find(id).Frame.Size.Height; h != want {
					t.Errorf("size %d: %s is %v tall, want %v", size, id, h, want)
				}
			}
			if f := app.Find("icon").Children[0].Frame.Size; f.Width != want {
				t.Errorf("size %d: icon's button = %v, want a %v square", size, f, want)
			}
		}
	})
}

// TestIconButtonVariants gives each look its default size.
func TestIconButtonVariants(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		app, _ := start(t, e, func(*core.Scope) core.Node {
			return core.View{Children: []core.Node{
				IconButton{Icon: lucide.Plus, Label: "Plain", TestID: "plain"},
				IconButton{Variant: IconButtonPrimary, Icon: lucide.Plus, Label: "Primary", TestID: "primary"},
				IconButton{Variant: IconButtonRaised, Icon: lucide.Plus, Label: "Raised", Size: 40, TestID: "raised"},
			}}
		})
		if f := app.Find("plain").Frame.Size; f.Width != 28 {
			t.Fatalf("plain = %v, want 28 points across", f)
		}
		if f := app.Find("primary").Children[0].Frame.Size; f.Width != 32 {
			t.Fatalf("primary = %v, want 32 points across", f)
		}
		if f := app.Find("raised").Children[0].Frame.Size; f.Width != 40 {
			t.Fatalf("raised = %v, want 40 points across", f)
		}
	})
}

// TestButtonSide lets a button of an icon alone line up with what is not a
// control's height.
func TestButtonSide(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		app, _ := start(t, e, func(*core.Scope) core.Node {
			return Button{Icon: lucide.Plus, Label: "Add", Side: 20, TestID: "icon"}
		})
		if f := app.Find("icon").Children[0].Frame.Size; f.Width != 20 || f.Height != 20 {
			t.Fatalf("icon's button = %v, want a 20 point square", f)
		}
	})
}
