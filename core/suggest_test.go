package core

import "testing"

func TestSuggestionRange(t *testing.T) {
	for _, tc := range []struct {
		name, value, want string
		caret, from, to   int
		ok                bool
	}{
		{name: "start", value: "@re", caret: 3, from: 0, to: 3, want: "re", ok: true},
		{name: "after emoji", value: "😀 @re", caret: 6, from: 3, to: 6, want: "re", ok: true},
		{name: "word prefix", value: "mail@re", caret: 7, ok: false},
		{name: "space closes", value: "@read me", caret: 8, ok: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			from, to, query, ok := suggestionRange(tc.value, tc.caret, SuggestOptions{})
			if from != tc.from || to != tc.to || query != tc.want || ok != tc.ok {
				t.Fatalf("got (%d, %d, %q, %v), want (%d, %d, %q, %v)", from, to, query, ok, tc.from, tc.to, tc.want, tc.ok)
			}
		})
	}
}

func TestSuggestionRangeAllowsSpaces(t *testing.T) {
	from, to, query, ok := suggestionRange("@release notes", UTF16Len("@release notes"), SuggestOptions{AllowSpaces: true})
	if !ok || from != 0 || to != UTF16Len("@release notes") || query != "release notes" {
		t.Fatalf("got (%d, %d, %q, %v)", from, to, query, ok)
	}
}
