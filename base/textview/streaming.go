package textview

import (
	"cmp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// wordRune reports whether r is a letter or a digit of a script that
// spaces its words, where a star inside a word is no emphasis.
func wordRune(r rune) bool {
	return (unicode.IsLetter(r) || unicode.IsDigit(r)) &&
		!unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul)
}

// RepairStreaming closes the syntax that the last block of Markdown still
// arriving (a reply streamed token by token) leaves open, so it shows as
// it will once the rest arrives: `**bold` as bold, a backtick and code as
// code, `$x^2` and `\(x^2` as formulas, `[text](htt` as its text and a
// table's header as a table, instead of markers as text. A last line not
// yet ended whose meaning the next characters decide is held back: a
// list marker alone, digits that may become one, a line of - or = (an
// underline now, a rule, an item or text later), a fence's opening line
// and the start of its closer. The blocks before the last are complete
// and kept.
func RepairStreaming(markdown string) string {
	markdown = holdLine(markdown)
	start, ok := lastBlock(markdown)
	if !ok {
		return markdown
	}
	tail := markdown[start:]
	if strings.TrimSpace(tail) == "" {
		return markdown
	}
	repaired, ok := repairTable(tail)
	if !ok {
		repaired = repairInline(tail)
	}
	if repaired == tail {
		return markdown
	}
	// What the repair left may be such a line (`- **` leaves `-`).
	return holdLine(markdown[:start] + repaired)
}

// holdLine drops src's last line while it has not ended and the next
// characters decide what it is.
func holdLine(src string) string {
	at := strings.LastIndexByte(src, '\n') + 1
	line := src[at:]
	if line == "" {
		return src
	}
	ch, n := openFence(src[:at])
	if ch != 0 {
		// Inside a fence: the start of its closer.
		rest, ok := stripIndent(line)
		if m := run(rest, 0, is(ch)); ok && m > 0 && m == len(rest) && m < n {
			return src[:at]
		}
		return src
	}
	if _, _, ok := fenceOpens(stripQuotes(line)); ok {
		return src[:at]
	}
	if undecided(strings.TrimSpace(stripQuotes(line))) {
		return src[:at]
	}
	return src
}

// stripQuotes drops the quote markers before a line's content.
func stripQuotes(line string) string {
	for {
		rest, ok := strings.CutPrefix(strings.TrimLeft(line, " \t"), ">")
		if !ok {
			return line
		}
		line = rest
	}
}

// undecided reports whether a line's content is a list marker alone,
// digits that may become one, or a line of - or =.
func undecided(s string) bool {
	switch {
	case s == "":
		return false
	case s == "-" || s == "*" || s == "+":
		return true
	case strings.Trim(s, "-") == "" || strings.Trim(s, "=") == "":
		return true
	}
	digits := run(s, 0, func(b byte) bool { return b >= '0' && b <= '9' })
	switch {
	case digits == 0 || digits > 9:
		return false
	case digits == len(s):
		return true
	}
	return digits == len(s)-1 && (s[digits] == '.' || s[digits] == ')')
}

// openFence is the fence src ends inside, its character and length, 0
// when none.
func openFence(src string) (byte, int) {
	var fence byte
	var fenceLen int
	for start := 0; start < len(src); {
		end := strings.IndexByte(src[start:], '\n')
		if end < 0 {
			end = len(src)
		} else {
			end += start
		}
		line := stripContainers(src[start:end])
		if fence != 0 {
			if fenceCloses(line, fence, fenceLen) {
				fence = 0
			}
		} else if ch, n, ok := fenceOpens(line); ok {
			fence, fenceLen = ch, n
		}
		start = end + 1
	}
	return fence, fenceLen
}

// lastBlock returns where the last block of src starts (after its last
// blank line or fenced block), and false when src ends inside a fenced
// block of code or a $$ formula, which runs to the end and needs no
// repair.
func lastBlock(src string) (int, bool) {
	var fence byte
	var fenceLen, tail int
	math := false
	for start := 0; start < len(src); {
		end := strings.IndexByte(src[start:], '\n')
		if end < 0 {
			end = len(src)
		} else {
			end += start
		}
		line := stripContainers(src[start:end])
		if fence != 0 {
			if fenceCloses(line, fence, fenceLen) {
				fence, tail = 0, end+1
			}
		} else if math {
			if strings.HasPrefix(strings.TrimLeft(line, " \t"), "$$") {
				math, tail = false, end+1
			}
		} else if ch, n, ok := fenceOpens(line); ok {
			fence, fenceLen = ch, n
		} else if rest, ok := strings.CutPrefix(strings.TrimLeft(line, " \t"), "$$"); ok && !strings.Contains(rest, "$") {
			math = true
		} else if strings.TrimSpace(line) == "" {
			tail = end + 1
		}
		start = end + 1
	}
	return min(tail, len(src)), fence == 0 && !math
}

// stripContainers drops the quote and list markers before a line's
// content, so fences inside quotes and list items are seen.
func stripContainers(line string) string {
	for {
		trimmed := strings.TrimLeft(line, " \t")
		if rest, ok := strings.CutPrefix(trimmed, ">"); ok {
			line = rest
			continue
		}
		rest := ""
		if trimmed != "" && strings.IndexByte("-*+", trimmed[0]) >= 0 {
			rest = trimmed[1:]
		} else if d := run(trimmed, 0, isDigit); d > 0 && d < 10 && d < len(trimmed) &&
			(trimmed[d] == '.' || trimmed[d] == ')') {
			rest = trimmed[d+1:]
		}
		if !strings.HasPrefix(rest, " ") && !strings.HasPrefix(rest, "\t") {
			return trimmed
		}
		line = rest
	}
}

// fenceOpens returns the character and length of the code fence line
// opens (``` or ~~~).
func fenceOpens(line string) (byte, int, bool) {
	rest, ok := stripIndent(line)
	if !ok || rest == "" || rest[0] != '`' && rest[0] != '~' {
		return 0, 0, false
	}
	ch := rest[0]
	n := run(rest, 0, is(ch))
	// A backtick fence's info string has no backticks, else it is code in
	// a paragraph.
	if n < 3 || ch == '`' && strings.Contains(rest[n:], "`") {
		return 0, 0, false
	}
	return ch, n, true
}

// fenceCloses reports whether line closes a fence of n characters ch.
func fenceCloses(line string, ch byte, n int) bool {
	rest, ok := stripIndent(line)
	rest = strings.TrimRightFunc(rest, unicode.IsSpace)
	m := run(rest, 0, is(ch))
	return ok && m >= n && m == len(rest)
}

func stripIndent(line string) (string, bool) {
	n := run(line, 0, is(' '))
	return line[n:], n < 4
}

// run counts the bytes from at on that match.
func run(s string, at int, match func(byte) bool) int {
	n := 0
	for at+n < len(s) && match(s[at+n]) {
		n++
	}
	return n
}

func is(b byte) func(byte) bool { return func(c byte) bool { return c == b } }
func isDigit(c byte) bool       { return c >= '0' && c <= '9' }

// repairTable completes a delimiter row being typed under a table's
// header (`| a | b |` then `| --`) to the header's width, so the header
// shows as a table and a lone `-` under it is no heading.
func repairTable(tail string) (string, bool) {
	header, delimiter, ok := strings.Cut(strings.TrimRight(tail, "\n"), "\n")
	if !ok || strings.Contains(delimiter, "\n") || !strings.Contains(header, "|") {
		return "", false
	}
	delimiter = strings.TrimSpace(delimiter)
	if delimiter == "" || strings.Trim(delimiter, "|-: \t") != "" {
		return "", false
	}
	if !strings.Contains(delimiter, "-") {
		// Nothing a column could be made of yet: the header alone.
		return header, true
	}
	typed := cells(delimiter)
	var row strings.Builder
	row.WriteString("|")
	for i := range max(len(cells(header)), 1) {
		cell := ""
		if i < len(typed) {
			cell = strings.TrimSpace(typed[i])
		}
		row.WriteString(" ")
		if strings.HasPrefix(cell, ":") {
			row.WriteString(":")
		}
		row.WriteString("---")
		if len(cell) > 1 && strings.HasSuffix(cell, ":") {
			row.WriteString(":")
		}
		row.WriteString(" |")
	}
	return header + "\n" + row.String(), true
}

// cells splits a table row at its unescaped pipes; the outer ones are
// optional.
func cells(row string) []string {
	row = strings.TrimSpace(row)
	var out []string
	start, escaped := 0, false
	for i := 0; i < len(row); i++ {
		switch {
		case row[i] == '\\' && !escaped:
			escaped = true
			continue
		case row[i] == '|' && !escaped:
			out = append(out, row[start:i])
			start = i + 1
		}
		escaped = false
	}
	out = append(out, row[start:])
	if strings.HasPrefix(row, "|") {
		out = out[1:]
	}
	if strings.HasSuffix(row, "|") && len(row) > 1 && len(out) > 0 {
		out = out[:len(out)-1]
	}
	return out
}

// repairInline closes the inline syntax the last block leaves open.
func repairInline(tail string) string {
	text := closeMarkers(removeTag(removeLinks(closeTeX(tail))))
	if text != tail {
		text = strings.TrimRightFunc(text, unicode.IsSpace)
	}
	return text
}

// inlineCode returns the inline code spans of text, from their first
// backtick to after their last, with tex those of TeX formulas (\(…\),
// \[…\]) too, and the start and length of an opening backtick run nothing
// closes, its start -1 when there is none.
func inlineCode(text string, tex bool) (spans [][2]int, open, openLen int) {
	for i := 0; i < len(text); {
		switch text[i] {
		case '\\':
			if end, ok := texEnd(text, i); tex && ok {
				spans = append(spans, [2]int{i, end})
				i = end
				continue
			}
			i += 2
			continue
		case '`':
		default:
			i++
			continue
		}
		n := run(text, i, is('`'))
		closed := -1
		for j := i + n; j < len(text); {
			if text[j] != '`' {
				j++
				continue
			}
			m := run(text, j, is('`'))
			if m == n {
				closed = j + m
				break
			}
			j += m
		}
		if closed < 0 {
			return spans, i, n
		}
		spans = append(spans, [2]int{i, closed})
		i = closed
	}
	return spans, -1, 0
}

// closeTeX closes a TeX formula left open (`\(x`, `\[` and lines of
// it), or removes its opener while nothing follows it.
func closeTeX(text string) string {
	spans, open, _ := inlineCode(text, false)
	for i := 0; i < len(text); {
		if !outside(spans, open, i) || text[i] != '\\' {
			i++
			continue
		}
		if i+1 >= len(text) || text[i+1] != '(' && text[i+1] != '[' {
			i += 2
			continue
		}
		if end, ok := texEnd(text, i); ok {
			i = end
			continue
		}
		body := text[i+2:]
		if strings.TrimSpace(body) == "" {
			return strings.TrimRightFunc(text[:i], unicode.IsSpace)
		}
		out := strings.TrimRightFunc(text, unicode.IsSpace)
		switch {
		case text[i+1] == '(':
			return out + `\)`
		case strings.Contains(body, "\n"):
			// A display formula of lines of its own: its closer on a line
			// of its own.
			return out + "\n" + `\]`
		}
		return out + `\]`
	}
	return text
}

// outside reports whether byte i is outside code.
func outside(spans [][2]int, open, i int) bool {
	for _, s := range spans {
		if i >= s[0] && i < s[1] {
			return false
		}
	}
	return open < 0 || i < open
}

// removeLinks shows links not finished, `[text` and `[text](partial`, as
// their text, and removes images and footnote references not finished.
func removeLinks(text string) string {
	spans, open, _ := inlineCode(text, true)
	var remove [][2]int
	var brackets []int
scan:
	for i := 0; i < len(text); i++ {
		if !outside(spans, open, i) {
			continue
		}
		switch text[i] {
		case '\\':
			i++
		case '[':
			brackets = append(brackets, i)
		case ']':
			if len(brackets) == 0 {
				continue
			}
			start := brackets[len(brackets)-1]
			brackets = brackets[:len(brackets)-1]
			if i+1 < len(text) && text[i+1] == '(' && destinationEnd(text, i+2) < 0 {
				// The destination runs to the end: drop it.
				if start > 0 && text[start-1] == '!' {
					remove = append(remove, [2]int{start - 1, len(text)})
				} else {
					remove = append(remove, [2]int{start, start + 1}, [2]int{i, len(text)})
				}
				brackets = nil
				break scan
			}
		}
	}
	for _, start := range brackets {
		image := start > 0 && text[start-1] == '!'
		switch {
		case image:
			remove = append(remove, [2]int{start - 1, len(text)})
		case start+1 < len(text) && text[start+1] == '^':
			// A footnote's label being typed shows nothing.
			remove = append(remove, [2]int{start, len(text)})
		default:
			remove = append(remove, [2]int{start, start + 1})
		}
	}
	if len(remove) == 0 {
		return text
	}
	slices.SortFunc(remove, func(a, b [2]int) int { return cmp.Or(cmp.Compare(a[0], b[0]), cmp.Compare(a[1], b[1])) })
	var out strings.Builder
	at := 0
	for _, r := range remove {
		if r[0] > at {
			out.WriteString(text[at:r[0]])
		}
		at = max(at, r[1])
	}
	if at < len(text) {
		out.WriteString(text[at:])
	}
	return out.String()
}

// destinationEnd returns where a link destination starting at from ends,
// after its `)`, or -1.
func destinationEnd(text string, from int) int {
	depth := 0
	for i := from; i < len(text); i++ {
		switch text[i] {
		case '\\':
			i++
		case '(':
			depth++
		case ')':
			if depth == 0 {
				return i + 1
			}
			depth--
		}
	}
	return -1
}

// removeTag removes an HTML tag or autolink being typed at the end: `<`,
// `<di`, `<a href="x`, `</`.
func removeTag(text string) string {
	spans, open, _ := inlineCode(text, true)
	lt := strings.LastIndexByte(text, '<')
	if lt < 0 || !outside(spans, open, lt) || strings.Contains(text[lt:], ">") || lt > 0 && text[lt-1] == '\\' {
		return text
	}
	after := text[lt+1:]
	tagged := after == "" || after[0] == '/' || after[0] == '!' ||
		after[0] >= 'a' && after[0] <= 'z' || after[0] >= 'A' && after[0] <= 'Z'
	if tagged && !strings.Contains(after, "\n") {
		return strings.TrimRight(text[:lt], " \t")
	}
	return text
}

// closeMarkers closes inline code, emphasis (`**`, `*`, `__`, `_`, `~~`)
// and formulas (`$$`, `$`) left open, in the reverse order they opened; a
// marker with nothing after it yet is removed instead. `$5` opens
// nothing.
func closeMarkers(text string) string {
	spans, openCode, openLen := inlineCode(text, true)
	// A run of markers at the end after a space opens nothing yet
	// (`a **`): removed.
	trimmed := strings.TrimRightFunc(text, unicode.IsSpace)
	runStart := len(strings.TrimRight(trimmed, "*_~"))
	if runStart < len(trimmed) && (openCode < 0 || openCode > runStart) {
		if prev, _ := utf8.DecodeLastRuneInString(trimmed[:runStart]); runStart == 0 || unicode.IsSpace(prev) {
			return closeMarkers(strings.TrimRightFunc(trimmed[:runStart], unicode.IsSpace))
		}
	}

	type marker struct {
		token string
		at    int
	}
	var stack []marker
	scanEnd := len(text)
	if openCode >= 0 {
		scanEnd = openCode
	}
	for i := 0; i < scanEnd; {
		if s := slices.IndexFunc(spans, func(s [2]int) bool { return s[0] == i }); s >= 0 {
			i = spans[s][1]
			continue
		}
		b := text[i]
		if b == '\\' {
			i += 2
			continue
		}
		if b != '*' && b != '_' && b != '~' && b != '$' {
			i++
			continue
		}
		n := min(run(text, i, is(b)), scanEnd-i)
		prev, _ := utf8.DecodeLastRuneInString(text[:i])
		next, _ := utf8.DecodeRuneInString(text[i+n:])
		hasPrev, hasNext := i > 0, i+n < len(text)
		canOpen := hasNext && !unicode.IsSpace(next) && (b != '_' || !hasPrev || !isAlnum(prev)) &&
			(b != '$' || next < '0' || next > '9') &&
			// A lone star inside a word (a*b) is text until it closes, and
			// is never closed for it; CJK text keeps emphasis there.
			(b != '*' || n != 1 || !hasPrev || !wordRune(prev) || !wordRune(next))
		canClose := hasPrev && !unicode.IsSpace(prev) && (b != '_' || !hasNext || !isAlnum(next))
		var opened []string
		for _, token := range markerTokens(b, n) {
			at := -1
			for k := len(stack) - 1; k >= 0; k-- {
				if stack[k].token == token {
					at = k
					break
				}
			}
			switch {
			case at >= 0 && canClose:
				stack = slices.Delete(stack, at, at+1)
			case canOpen:
				opened = append(opened, token)
			}
		}
		// Opening `***`: the strong outside, the emphasis inside.
		for k := len(opened) - 1; k >= 0; k-- {
			stack = append(stack, marker{opened[k], i})
		}
		i += n
	}

	out := text
	// A code span with nothing in it yet: its backticks removed.
	codeCloser := 0
	if openCode >= 0 {
		if strings.TrimSpace(text[openCode+openLen:]) == "" {
			out = text[:openCode]
		} else {
			codeCloser = openLen
		}
	}
	// Markers with nothing after them yet: removed.
	for codeCloser == 0 && len(stack) > 0 {
		at := stack[len(stack)-1].at
		if strings.TrimSpace(strings.TrimLeft(out[at:], "*_~$")) != "" {
			break
		}
		out = out[:at]
		stack = stack[:len(stack)-1]
	}
	if codeCloser == 0 && len(stack) == 0 {
		if len(out) != len(text) {
			out = strings.TrimRightFunc(out, unicode.IsSpace)
		}
		return out
	}
	out = strings.TrimRightFunc(out, unicode.IsSpace) + strings.Repeat("`", codeCloser)
	for k := len(stack) - 1; k >= 0; k-- {
		out += stack[k].token
	}
	return out
}

func markerTokens(b byte, n int) []string {
	single, double := string(b), string([]byte{b, b})
	switch {
	case b == '~' && n == 2:
		return []string{double}
	case b == '~':
		return nil
	case b == '$' && n <= 2:
		return []string{strings.Repeat("$", n)}
	case b == '$':
		return nil
	case n == 1:
		return []string{single}
	case n == 2:
		return []string{double}
	case n == 3:
		return []string{single, double}
	}
	return nil
}

func isAlnum(r rune) bool { return unicode.IsLetter(r) || unicode.IsNumber(r) }
