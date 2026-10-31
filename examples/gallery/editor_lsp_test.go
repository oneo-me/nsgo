package main

import (
	"strings"
	"testing"

	"github.com/oneo-me/nsgo/base/editor"
)

// TestLanguageServerReadsTheSample checks the simulated server finds in
// the TypeScript sample what NSH's finds.
func TestLanguageServerReadsTheSample(t *testing.T) {
	m := analyze(lspSample)
	var got []string
	for _, p := range m.lint() {
		got = append(got, p.diagnostic.Message)
	}
	t.Logf("problems: %q", got)
	for _, want := range []string{"TODO: read the coupons from the server", "Expected '===' and instead saw '=='.",
		"Unexpected var, use let or const instead.", "'unused' is declared but its value is never read.",
		"Property 'prnt' does not exist on type 'Receipt'. Did you mean 'print'?"} {
		found := false
		for _, g := range got {
			found = found || g == want
		}
		if !found {
			t.Errorf("no problem %q", want)
		}
	}
	line := func(prefix string) int {
		for i, l := range m.lines {
			if strings.HasPrefix(l, prefix) {
				return i
			}
		}
		t.Fatalf("no line %q", prefix)
		return 0
	}
	call := line("const label = describe")
	if h := m.hover(editor.Position{Line: call, Column: 16}); h == nil || !strings.Contains(h.markdown, "function describe(order: Order): string") ||
		!strings.Contains(h.markdown, "coupon taken off") {
		t.Errorf("hover over describe: %+v", h)
	}
	if d := m.definition(editor.Position{Line: call, Column: 16}); d == nil || d.rng.Start.Line != line("function describe") {
		t.Errorf("definition of describe: %+v", d)
	}
	if c := m.colors(); len(c) != 2 || c[0].color != 0xff8800ff {
		t.Errorf("colors %+v", c)
	}
	inside := line("  return order.items.reduce")
	items, _ := m.completion(editor.CompletionRequest{Position: editor.Position{Line: inside, Column: 15}, Character: "."})
	var names []string
	for _, it := range items {
		names = append(names, it.Label)
	}
	if strings.Join(names, " ") != "id items coupon" {
		t.Errorf("members of order: %q", names)
	}
	if s := m.inline(editor.Position{Line: line("    console.log"), Column: 12}); s != "" {
		t.Errorf("no suggestion mid line, got %q", s)
	}
	m2 := analyze(lspSample + "console.")
	if s := m2.inline(editor.Position{Line: len(m2.lines) - 1, Column: 8}); s != "log(label);" {
		t.Errorf("after console.: %q", s)
	}
	fixes := 0
	for _, a := range codeActions(editor.Range{Start: editor.Position{Line: line("  var total"), Column: 3}, End: editor.Position{Line: line("  var total"), Column: 3}}, m.lint()) {
		if a.kind == "quickfix" {
			fixes++
		}
	}
	if fixes != 2 {
		t.Errorf("%d fixes at var, want let and const", fixes)
	}
}
