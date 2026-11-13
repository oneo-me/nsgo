package textview

import (
	"strings"
	"testing"
)

func checkRepair(t *testing.T, cases [][2]string) {
	t.Helper()
	for _, c := range cases {
		if got := RepairStreaming(c[0]); got != c[1] {
			t.Errorf("RepairStreaming(%q) = %q, want %q", c[0], got, c[1])
		}
	}
}

func TestRepairEmphasis(t *testing.T) {
	checkRepair(t, [][2]string{
		{"Some **bold", "Some **bold**"},
		{"Some *it", "Some *it*"},
		{"Some __bold", "Some __bold__"},
		{"Some _it", "Some _it_"},
		{"Some ~~gone", "Some ~~gone~~"},
		{"**bold *both", "**bold *both***"},
		{"***both", "***both***"},
		{"**done** and *open", "**done** and *open*"},
		{"**done**", "**done**"},
		{"snake_case_name", "snake_case_name"},
		{"2 * 3 * 4", "2 * 3 * 4"},
		{"- item\n- **bo", "- item\n- **bo**"},
		{"**bold ", "**bold**"},
		{"Text **", "Text"},
		{"Text **bold** *", "Text **bold**"},
		{`escaped \*star`, `escaped \*star`},
		{"Costs $5 and *more", "Costs $5 and *more*"},
	})
}

func TestRepairCode(t *testing.T) {
	checkRepair(t, [][2]string{
		{"Use `cod", "Use `cod`"},
		{"Use `", "Use"},
		{"Use `a*b` and **x", "Use `a*b` and **x**"},
		{"**bold `code", "**bold `code`**"},
		{"``a ` b", "``a ` b``"},
		// In a fence nothing changes: it runs to the end.
		{"```go\na := **b", "```go\na := **b"},
		{"> ```\n> **x", "> ```\n> **x"},
		{"- ~~~\n  *x", "- ~~~\n  *x"},
		// Earlier blocks are complete.
		{"**a\n\nb", "**a\n\nb"},
		{"```\n**x\n```\n**y", "```\n**x\n```\n**y**"},
	})
}

func TestRepairMath(t *testing.T) {
	checkRepair(t, [][2]string{
		{"Formula $$x^2", "Formula $$x^2$$"},
		{"Inline $x^2", "Inline $x^2$"},
		{"Costs $5 and more", "Costs $5 and more"},
		{"$$\nx^2", "$$\nx^2"},
		{"> $$\n> x **y", "> $$\n> x **y"},
		{"$$\nx\n$$\nSo $y", "$$\nx\n$$\nSo $y$"},
		{"Text $", "Text $"},
		{"**bold $x", "**bold $x$**"},
	})
}

func TestRepairTeX(t *testing.T) {
	checkRepair(t, [][2]string{
		{`Euler \(e^{i`, `Euler \(e^{i\)`},
		{`Display \[x^2`, `Display \[x^2\]`},
		{"\\[\nx = 1\n+ 2", "\\[\nx = 1\n+ 2\n\\]"},
		{`Opened \(`, "Opened"},
		{`Done \(x\) and **b`, `Done \(x\) and **b**`},
		// Inside a formula, markers are TeX's.
		{`\(a*b`, `\(a*b\)`},
		{`\(a_1\) x`, `\(a_1\) x`},
		{"`\\(` code", "`\\(` code"},
	})
}

func TestRepairLinksAndTags(t *testing.T) {
	checkRepair(t, [][2]string{
		{"See [the docs](https://exa", "See the docs"},
		{"See [the docs", "See the docs"},
		{"See [the docs]", "See [the docs]"},
		{"See [a](b) and [c](d", "See [a](b) and c"},
		{"An image ![alt](http://x", "An image"},
		{"An image ![al", "An image"},
		{"Note[^1", "Note"},
		{"[", ""},
		{"Some <di", "Some"},
		{`Some <a href="x`, "Some"},
		{"a < b", "a < b"},
		{"Visit <https://exa", "Visit"},
		{"<b>x</b>", "<b>x</b>"},
		{"`[x`", "`[x`"},
	})
}

func TestRepairTables(t *testing.T) {
	checkRepair(t, [][2]string{
		{"| a | b |\n| -", "| a | b |\n| --- | --- |"},
		{"| a | b |\n|:-", "| a | b |\n| :--- | --- |"},
		{"| a | b |\n| --- | --: |", "| a | b |\n| --- | ---: |"},
		{"| a | b |\n|", "| a | b |"},
		{"| a | b |\n| --- | --- |\n| 1 |", "| a | b |\n| --- | --- |\n| 1 |"},
		// A line of - not yet ended is held: a rule, an item or text later.
		{"Title\n---", "Title\n"},
		{"Title\n---\n", "Title\n---\n"},
	})
}

func TestRepairNeverPanics(t *testing.T) {
	for _, in := range []string{
		"", "*", "**", "***", "_", "`", "``", "$", "$$", "~~", "[", "![", "<", "](", `\`,
		"| \n-", "中文**粗", "**中文", "é*", "\n\n\n", "> ", "1. ", "- [", "a\\", "`\\",
	} {
		RepairStreaming(in)
	}
	checkRepair(t, [][2]string{{"中文**粗", "中文**粗**"}})
}

func FuzzRepairStreaming(f *testing.F) {
	for _, s := range []string{"**a `b", "[x](y", "| a |\n| -", "```\n*x"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) { RepairStreaming(s) })
}

// A repaired reply parses as the finished one does.
func TestRepairParsesAsFinished(t *testing.T) {
	for _, c := range [][2]string{
		{"Some **bold", "Some **bold**"},
		{"See [the docs](https://exa", "See the docs"},
		{"| a | b |\n| -", "| a | b |\n| --- | --- |"},
		{"Inline $x^2", "Inline $x^2$"},
		{"Formula $$x^2", "Formula $$x^2$$"},
		{"So \\(x", "So $x$"},
	} {
		got, want := Parse(RepairStreaming(c[0])), Parse(c[1])
		if !equal(got, want) {
			t.Errorf("Parse(RepairStreaming(%q)) = %#v, want %#v", c[0], got, want)
		}
	}
}

// No prefix of a reply streamed word by word shows its markers as text,
// except a table's header before its delimiter row, which no one can tell
// from text yet.
func TestRepairEveryPrefix(t *testing.T) {
	reply := "Some **bold words**, *emphasis*, `inline code` and a [link](https://nsgo.dev).\n\n" +
		"- An item with ~~struck words~~\n\n| Name | Value |\n|---|--:|\n| a | 1 |\n\nDone."
	words := strings.SplitAfter(reply, " ")
	for i := range words {
		prefix := strings.Join(words[:i+1], "")
		text := Parse(RepairStreaming(prefix)).PlainText()
		if strings.ContainsAny(text, "*`[]~") {
			t.Errorf("%q shows %q", prefix, text)
		}
	}
}

// A last line whose meaning the next characters decide is held back
// until it ends.
func TestRepairHoldsUndecidedLines(t *testing.T) {
	checkRepair(t, [][2]string{
		{"para\n\n-", "para\n\n"},
		{"- one\n-", "- one\n"},
		{"- one\n- ", "- one\n"},
		{"1. one\n2.", "1. one\n"},
		{"1. one\n2", "1. one\n"},
		{"> -", ""},
		{"Title\n--", "Title\n"},
		{"Title\n==", "Title\n"},
		{"Text\n- **", "Text\n"},
		// Ended, it is what it is.
		{"para\n\n-\n", "para\n\n-\n"},
		{"Title\n--\n", "Title\n--\n"},
		// Text that only starts with digits shows.
		{"2026 was", "2026 was"},
		{"- item", "- item"},
	})
}

func TestRepairHoldsFences(t *testing.T) {
	checkRepair(t, [][2]string{
		{"Look:\n```pyth", "Look:\n"},
		{"Look:\n```", "Look:\n"},
		{"```go\nx := 1\n``", "```go\nx := 1\n"},
		{"````\nx\n```", "````\nx\n"},
		{"```go\nx := 1\n", "```go\nx := 1\n"},
	})
}

func TestRepairLeavesStarsInsideWords(t *testing.T) {
	checkRepair(t, [][2]string{
		{"a*b", "a*b"},
		{"x*y and *more", "x*y and *more*"},
	})
}
