package ui

import (
	"testing"

	"github.com/oneo-me/nsgo/core"
)

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
