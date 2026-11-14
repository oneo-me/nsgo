package textruns_test

import (
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/textruns"
)

func texts(runs []textruns.Run) []string {
	out := make([]string, len(runs))
	for i, r := range runs {
		out[i] = r.Text
	}
	return out
}

func TestSplitFillsGapsAndKeepsSpans(t *testing.T) {
	in := textruns.Input{
		Text:  "one two three",
		Spans: []driver.TextSpan{{Start: 4, End: 7, Style: driver.TextStyle{Color: 1}, Link: true}},
		Style: driver.TextStyle{Color: 2},
	}
	runs := textruns.Split(in)
	if got, want := texts(runs), []string{"one ", "two", " three"}; len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("runs %q, want %q", got, want)
	}
	if runs[0].Style.Color != 2 || runs[1].Style.Color != 1 || runs[2].Style.Color != 2 {
		t.Errorf("styles %v, want the text's in gaps and the span's inside", runs)
	}
	if !runs[1].Link {
		t.Error("the span's link was dropped")
	}
}

func TestSplitClipsASpanPastTheText(t *testing.T) {
	runs := textruns.Split(textruns.Input{Text: "abc", Spans: []driver.TextSpan{{Start: 1, End: 99}}})
	if got := texts(runs); len(got) != 2 || got[1] != "bc" {
		t.Fatalf("runs %q, want a then bc", got)
	}
}

func TestSplitClipsOverlappingSpans(t *testing.T) {
	runs := textruns.Split(textruns.Input{Text: "abcdef", Spans: []driver.TextSpan{
		{Start: 0, End: 4}, {Start: 2, End: 6},
	}})
	if got := texts(runs); len(got) != 2 || got[0] != "abcd" || got[1] != "ef" {
		t.Fatalf("runs %q, want abcd then ef", got)
	}
}

func TestSplitWithoutSpansIsOneRun(t *testing.T) {
	runs := textruns.Split(textruns.Input{Text: "abc"})
	if len(runs) != 1 || runs[0].Text != "abc" {
		t.Fatalf("runs %q, want the whole text", texts(runs))
	}
}

func TestSplitOfAnEmptyText(t *testing.T) {
	if runs := textruns.Split(textruns.Input{}); len(runs) != 0 {
		t.Fatalf("runs %q, want none", texts(runs))
	}
}
