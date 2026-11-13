package driver

import "testing"

func TestHelpTextReadsTheDescriptionFirst(t *testing.T) {
	for _, c := range []struct{ desc, hint, want string }{
		{"", "", ""},
		{"Too short", "", "Too short"},
		{"", "Opens it", "Opens it"},
		{"Too short", "Type more", "Too short. Type more"},
		{"Too short!", "Type more", "Too short! Type more"},
	} {
		p := ViewProps{Description: c.desc, Hint: c.hint}
		if got := p.HelpText(); got != c.want {
			t.Errorf("HelpText(%q, %q) = %q, want %q", c.desc, c.hint, got, c.want)
		}
	}
}
