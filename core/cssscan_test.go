package core

import (
	"math/rand/v2"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// The scanners match what the regular expressions they replace matched.
var (
	remPattern    = regexp.MustCompile(`(-?[0-9]*\.?[0-9]+)rem\b`)
	varRefPattern = regexp.MustCompile(`var\(--([A-Za-z0-9_-]+)(?:\s*,\s*([^()]*))?\)`)
)

func TestScannersMatchTheirPatterns(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	pieces := []string{"rem", "re", "m", "1", "0", ".", "-", "-", " ", "\t", ",", "(", ")", "var(--", "var(", "--", "a", "_", "x", "px", "calc(", "2.5", "e"}
	cases := []string{"1rem", "-.25rem calc(100% - 2rem)", "1.2.3rem", "1remx 3rem_", "--1rem", "1.rem", "var(--a)", "var(--b-c , 1px 2px)",
		"var(--x,var(--y))", "var(--)", "var(--a ,)", "var(--a b)", "a var(--n,\t red ) b var(--m)"}
	for range 20000 {
		var b strings.Builder
		for range r.IntN(10) {
			b.WriteString(pieces[r.IntN(len(pieces))])
		}
		cases = append(cases, b.String())
	}
	for _, s := range cases {
		want := remPattern.ReplaceAllStringFunc(s, func(m string) string {
			n, _ := strconv.ParseFloat(strings.TrimSuffix(m, "rem"), 64)
			return "<" + strconv.FormatFloat(n, 'g', -1, 64) + ">"
		})
		if got := replaceRems(s, func(n float64) string { return "<" + strconv.FormatFloat(n, 'g', -1, 64) + ">" }); got != want {
			t.Fatalf("replaceRems(%q) = %q, the pattern's %q", s, got, want)
		}
		want = varRefPattern.ReplaceAllStringFunc(s, func(m string) string {
			g := varRefPattern.FindStringSubmatch(m)
			return "<" + g[1] + "|" + g[2] + ">"
		})
		if got := replaceVarRefs(s, func(name, fallback string) string { return "<" + name + "|" + fallback + ">" }); got != want {
			t.Fatalf("replaceVarRefs(%q) = %q, the pattern's %q", s, got, want)
		}
	}
}
