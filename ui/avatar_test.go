package ui

import (
	"strconv"
	"testing"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/nsgotest"
)

// TestAvatar shows initials until the photo loads, and its status dot.
func TestAvatar(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			return core.View{Style: core.Style{Direction: core.Row, Gap: 8}, Children: []core.Node{
				Avatar{Name: "ada lovelace", TestID: "plain", Status: AvatarOnline, Label: "Ada Lovelace, online"},
				Avatar{Name: "Grace Hopper", Image: core.Src("https://example.com/grace.png"), TestID: "photo"},
			}}
		})
		plain := app.Find("role:image Ada Lovelace, online")
		if got, want := plain.Frame.Size.Width, avatarSize(0); got != want {
			t.Fatalf("avatar = %v wide, want %v", got, want)
		}
		if len(plain.Children) != 2 {
			t.Fatalf("avatar holds %d views, want the initials and the dot", len(plain.Children))
		}
		app.Find("text:AL")
		app.Find("text:GH")
		app.Load(viewOf(app.Find("photo").Children[1]), core.Size{Width: 96, Height: 96})
		if app.Exists("text:GH") {
			t.Fatal("the initials show over the loaded photo")
		}
	})
}

// viewOf is a selector of v.
func viewOf(v *nsgotest.View) string { return "view:" + strconv.Itoa(int(v.ID)) }
