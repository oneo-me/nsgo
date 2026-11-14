// Package textruns splits a text at its spans' byte offsets, so a backend
// that draws a text span by span sees the bytes, style and link of each run.
package textruns

import "github.com/nsgo-dev/nsgo/internal/driver"

// Input is the text a backend splits: its bytes, the spans over them and the
// style the text takes where no span says otherwise.
type Input struct {
	Text  string
	Spans []driver.TextSpan
	Style driver.TextStyle
}

// Run is one run of the text: bytes [Start, End), the span that styles them,
// or the text's own style in a gap.
type Run struct {
	Start, End int
	Text       string
	Style      driver.TextStyle
	Link       bool
	Background driver.TextBackground
}

// Split returns in's text split at its spans' offsets, in order. A gap takes
// the text's style; a span starting before the text or inside the one before
// is clipped to what is left, and one ending past the text to its end.
func Split(in Input) []Run {
	if in.Text == "" || len(in.Spans) == 0 && in.Text == "" {
		return nil
	}
	var out []Run
	at := 0
	for _, sp := range in.Spans {
		start, end := max(sp.Start, at), min(sp.End, len(in.Text))
		if start >= end {
			continue
		}
		if start > at {
			out = append(out, Run{Start: at, End: start, Text: in.Text[at:start], Style: in.Style})
		}
		out = append(out, Run{
			Start: start, End: end, Text: in.Text[start:end],
			Style: sp.Style, Link: sp.Link, Background: sp.Background,
		})
		at = end
	}
	if at < len(in.Text) {
		out = append(out, Run{Start: at, End: len(in.Text), Text: in.Text[at:], Style: in.Style})
	}
	return out
}
