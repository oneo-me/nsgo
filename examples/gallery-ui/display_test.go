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

// TestAvatarGroup shows the first members and counts the rest, and
// spreads under the pointer on desktops.
func TestAvatarGroup(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			var members []AvatarGroupMember
			for _, name := range []string{"Ada", "Grace", "Alan", "Edsger", "Barbara", "Donald"} {
				members = append(members, AvatarGroupMember{Name: name})
			}
			return AvatarGroup{TestID: "team", Label: "Team", Max: 3, Members: members,
				MoreLabel: func(n int) string { return "3 more" }}
		})
		app.Find("role:image Ada")
		app.Find("role:image Alan")
		if app.Exists("role:image Edsger") {
			t.Fatal("a member past Max shows")
		}
		app.Find("role:image 3 more")
		if e.Idiom != core.IdiomDesktop {
			return
		}
		first := app.Find("role:image Ada").Parent.Parent
		app.Hover("team")
		if got := first.Style.Transform.TranslateX; got >= 0 {
			t.Fatalf("hovered first slot moves %v, want to the left", got)
		}
	})
}

// viewOf is a selector of v.
func viewOf(v *nsgotest.View) string { return "view:" + strconv.Itoa(int(v.ID)) }
