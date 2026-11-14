//go:build js && wasm

package harmonyos

import (
	"encoding/json"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/textruns"
	"github.com/nsgo-dev/nsgo/internal/platform/webstyle"
)

// textRun is one styled run of a text with spans, as the host draws a Span
// for it.
type textRun struct {
	T string  `json:"t"`
	C string  `json:"c,omitempty"`
	S float32 `json:"s,omitempty"`
	W int     `json:"w,omitempty"`
	I bool    `json:"i,omitempty"`
	F string  `json:"f,omitempty"`
}

// textRunsJSON draws a text's spans as one run each, taking the text's own
// font and color where a span does not set its own. It returns "" when the
// text has no spans, which the host draws as one Text.
func textRunsJSON(t *driver.TextContent) string {
	if t == nil || len(t.Spans) == 0 {
		return ""
	}
	runs := textruns.Split(textruns.Input{Text: t.Text, Spans: t.Spans, Style: t.Style})
	if len(runs) == 0 {
		return ""
	}
	base := t.Style.Font
	if base.Size == 0 {
		base.Size = 16
	}
	if base.Weight == 0 {
		base.Weight = 400
	}
	out := make([]textRun, 0, len(runs))
	for _, r := range runs {
		f := r.Style.Font
		if f.Size == 0 {
			f.Size = base.Size
		}
		if f.Weight == 0 {
			f.Weight = base.Weight
		}
		if f.Family == "" {
			f.Family = base.Family
		}
		c := r.Style.Color
		if c == 0 {
			c = t.Style.Color
		}
		out = append(out, textRun{T: r.Text, C: webstyle.Color(c), S: float32(f.Size),
			W: int(f.Weight), I: f.Italic, F: f.Family})
	}
	b, err := json.Marshal(out)
	if err != nil {
		return ""
	}
	return string(b)
}
