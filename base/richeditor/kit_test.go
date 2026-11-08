package richeditor

import (
	"strings"
	"testing"
)

func kitSession(t *testing.T, c SessionConfig, extra ...Extension) *Session {
	t.Helper()
	c.Extensions = append(StarterKit(StarterOptions{}), TableKit(TableOptions{}), Mention(MentionOptions{Suggestion: SuggestionOptions{Class: "suggest", CaptureKeys: []string{"ArrowDown", "Enter"}}}))
	c.Extensions = append(c.Extensions, extra...)
	s, err := NewSession(c)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func typeAll(s *Session, text string) {
	for _, r := range text {
		s.TypeText(string(r), nil)
	}
}

func TestStarterKitTyping(t *testing.T) {
	s := kitSession(t, SessionConfig{})
	typeAll(s, "# Title")
	s.HandleKey("Enter")
	typeAll(s, "- one")
	s.HandleKey("Enter")
	typeAll(s, "two")
	s.HandleKey("Enter")
	s.HandleKey("Enter")
	typeAll(s, "a **bold** word")
	want := "# Title\n\n- one\n- two\n\na **bold** word\n"
	if got := s.Markdown(); got != want {
		t.Fatalf("got %q\nwant %q\ndoc %s", got, want, s.State().Doc)
	}
	if err := s.State().Doc.Check(); err != nil {
		t.Fatal(err)
	}
}

func TestDuplicateExtensionsAndPriority(t *testing.T) {
	if _, err := Build(Extension{Name: "a", Nodes: []NodeSpec{{Name: "doc", Content: "text*"}, {Name: "text"}}}, Extension{Name: "a"}); err == nil {
		t.Fatal("two extensions of one name")
	}
	var order []string
	app := Extension{Name: "app", Keys: func(*Schema) map[string]Command {
		return bind("Enter", func(*State, func(*Transaction)) bool {
			order = append(order, "app")
			return false
		})
	}}
	s := kitSession(t, SessionConfig{}, app)
	s.HandleKey("Enter")
	if len(order) != 1 {
		t.Fatal("an app's binding after the kit runs before it")
	}
}

func TestToggles(t *testing.T) {
	s := kitSession(t, SessionConfig{Markdown: "one\n"})
	if !s.RunNamed("toggleHeading", Attrs{"level": 2}) || !s.IsActive("heading", Attrs{"level": 2}) {
		t.Fatalf("heading: %s", s.State().Doc)
	}
	s.RunNamed("toggleHeading", Attrs{"level": 2})
	if s.IsActive("heading", nil) {
		t.Fatal("toggled off")
	}
	s.RunNamed("toggleBulletList", nil)
	if !s.IsActive("bulletList", nil) {
		t.Fatalf("list: %s", s.State().Doc)
	}
	s.RunNamed("toggleOrderedList", nil)
	if !s.IsActive("orderedList", nil) || s.IsActive("bulletList", nil) {
		t.Fatalf("converted: %s", s.State().Doc)
	}
	s.RunNamed("toggleOrderedList", nil)
	if s.IsActive("orderedList", nil) {
		t.Fatalf("lifted: %s", s.State().Doc)
	}
	s.RunNamed("toggleBlockquote", nil)
	if !s.IsActive("blockquote", nil) {
		t.Fatal("quote")
	}
	if s.Command("nope", nil) != nil {
		t.Fatal("an unknown command")
	}
}

func TestMarkdownRoundTrips(t *testing.T) {
	cases := []string{
		"# Title\n\nSome *em* and **strong** and `code` and ~~gone~~ text.\n",
		"- one\n- two\n  - nested\n\n1. first\n2. second\n",
		"- [x] done\n- [ ] todo\n",
		"> quoted\n>\n> twice\n",
		"```go\nfunc main() {}\n```\n",
		"[link](https://x.dev) here\n\n![pic](a.png)\n\n---\n\nend\n",
		"| a | b |\n| --- | --- |\n| c | d |\n",
	}
	for _, c := range cases {
		s := kitSession(t, SessionConfig{Markdown: c})
		if err := s.State().Doc.Check(); err != nil {
			t.Fatalf("%q: %v\n%s", c, err, s.State().Doc)
		}
		if got := s.Markdown(); strings.TrimSpace(got) != strings.TrimSpace(c) {
			t.Fatalf("got %q\nwant %q\ndoc %s", got, c, s.State().Doc)
		}
	}
	// A table a pipe table cannot hold goes as its HTML, and reads back.
	st := kitSession(t, SessionConfig{Markdown: "| a | b |\n| --- | --- |\n| c | d |\n"})
	if !st.Run(SetCellSelection(2, 2+st.State().Doc.Child(0).Child(0).Child(0).NodeSize())) || !st.Run(MergeCells) {
		t.Fatal("merge")
	}
	md := st.Markdown()
	if !strings.HasPrefix(md, "<table") {
		t.Fatalf("merged table as %q", md)
	}
	back := kitSession(t, SessionConfig{Markdown: md})
	if colspan(back.State().Doc.Child(0).Child(0).Child(0)) != 2 {
		t.Fatalf("read back %s", back.State().Doc)
	}
}

func TestJSONAndHTML(t *testing.T) {
	s := kitSession(t, SessionConfig{Markdown: "# T\n\n- [x] a **b**\n"})
	j := s.JSON()
	back, err := DocFromJSONString(s.State().Schema, j)
	if err != nil || !back.Eq(s.State().Doc) {
		t.Fatalf("%v\n%s\n%s", err, back, s.State().Doc)
	}
	h := kitSession(t, SessionConfig{HTML: s.HTML()})
	if h.JSON() != j {
		t.Fatalf("html read %s\nwant %s", h.JSON(), j)
	}
	bad := `{"type":"doc","content":[{"type":"nope"}]}`
	if _, err := DocFromJSONString(s.State().Schema, bad); err == nil {
		t.Fatal("an unknown type")
	}
	if _, err := DocFromJSONString(s.State().Schema, `{"type":"doc","content":[{"type":"text","text":"x"}]}`); err == nil {
		t.Fatal("content the schema refuses")
	}
}

func TestPasteAndCopy(t *testing.T) {
	s := kitSession(t, SessionConfig{Markdown: "ab\n"})
	s.Dispatch(s.State().Tr().SetSelection(TextSelectionAt(s.State().Doc, 2, 2)))
	s.Paste("x\ny", "")
	if got := s.Text(); got != "ax\n\nyb" {
		t.Fatalf("paste %q", got)
	}
	s = kitSession(t, SessionConfig{})
	s.Paste("# H\n\n- item\n", "")
	if got := s.Markdown(); got != "# H\n\n- item\n" {
		t.Fatalf("markdown paste %q %s", got, s.State().Doc)
	}
	s = kitSession(t, SessionConfig{})
	s.Paste("", "<p>a <b>b</b></p>")
	if got := s.Markdown(); got != "a **b**\n" {
		t.Fatalf("html paste %q", got)
	}
	s = kitSession(t, SessionConfig{Markdown: "one two\n"})
	s.Dispatch(s.State().Tr().SetSelection(TextSelectionAt(s.State().Doc, 1, 4)))
	text, html := s.Copy()
	if text != "one" || !strings.Contains(html, "one") {
		t.Fatalf("copy %q %q", text, html)
	}
	s.Paste("https://x.dev", "")
	if got := s.Markdown(); got != "[one](https://x.dev) two\n" {
		t.Fatalf("a URL over a selection: %q", got)
	}
	if text, _ := s.Cut(); text != "one" || s.Text() != " two" {
		t.Fatalf("cut %q %q", text, s.Text())
	}
}

func TestAutolink(t *testing.T) {
	s := kitSession(t, SessionConfig{})
	typeAll(s, "see https://x.dev/a. ")
	if got := s.Markdown(); got != "see [https://x.dev/a](https://x.dev/a). \n" && got != "see [https://x.dev/a.](https://x.dev/a.) \n" {
		t.Fatalf("autolink %q", got)
	}
	if ls := FindLinks("(a.io) me@b.dev www.c.org, not.a.link"); len(ls) != 3 || ls[0].Href != "https://a.io" || ls[1].Href != "mailto:me@b.dev" {
		t.Fatalf("links %+v", ls)
	}
	if IsAllowedURI("javascript:alert(1)", nil) {
		t.Fatal("javascript: is refused")
	}
	if !IsAllowedURI("https://x", nil) || !IsAllowedURI("/rel", nil) {
		t.Fatal("ordinary links are allowed")
	}
}

func TestMentionSuggestion(t *testing.T) {
	s := kitSession(t, SessionConfig{})
	typeAll(s, "hi @al")
	st := SuggestionStateOf(s.State(), "mention")
	if st == nil || !st.Active || st.Query != "al" || st.From != 4 || st.To != 7 {
		t.Fatalf("state %+v", st)
	}
	if !s.HandleKey("ArrowDown") || SuggestionStateOf(s.State(), "mention").KeyCount != 1 {
		t.Fatal("a captured key")
	}
	if !s.RunNamed("insertMention", Attrs{"id": "1", "label": "Alice"}) {
		t.Fatal("insert")
	}
	if got := s.Text(); got != "hi @Alice " {
		t.Fatalf("text %q", got)
	}
	doc := s.State().Doc
	m := doc.Child(0).Child(1)
	if m.Type.Name != "mention" || m.NodeSize() != 1 {
		t.Fatalf("mention %s", doc)
	}
	if SuggestionStateOf(s.State(), "mention").Active {
		t.Fatal("still active after inserting")
	}
	typeAll(s, "@x")
	if !s.HandleKey("Escape") || SuggestionStateOf(s.State(), "mention").Active {
		t.Fatal("Escape dismisses")
	}
	typeAll(s, "y")
	if SuggestionStateOf(s.State(), "mention").Active {
		t.Fatal("dismissed stays dismissed while typing on")
	}
	typeAll(s, " @z")
	if !SuggestionStateOf(s.State(), "mention").Active {
		t.Fatal("a new trigger is active")
	}
}

func TestCharacterCount(t *testing.T) {
	s := kitSession(t, SessionConfig{}, CharacterCount(CharacterCountOptions{Limit: 3}))
	typeAll(s, "abcde")
	if got := s.Text(); got != "abc" {
		t.Fatalf("typed past the limit: %q", got)
	}
	s = kitSession(t, SessionConfig{}, CharacterCount(CharacterCountOptions{Limit: 3}))
	s.Paste("abcdefg", "")
	if got := s.Text(); got != "abc" {
		t.Fatalf("paste %q", got)
	}
	if Characters(s.State(), false) != 3 || Words(s.State()) != 1 {
		t.Fatal("counts")
	}
}

func TestMoves(t *testing.T) {
	s := kitSession(t, SessionConfig{Markdown: "a😀 word\n\nnext\n"})
	s.Dispatch(s.State().Tr().SetSelection(TextSelectionAt(s.State().Doc, 2, 2)))
	s.MoveChar(1, false)
	if got := s.State().Selection.Head(); got != 4 {
		t.Fatalf("over a surrogate pair: %d", got)
	}
	s.MoveChar(-1, false)
	if got := s.State().Selection.Head(); got != 2 {
		t.Fatalf("back: %d", got)
	}
	s.MoveWord(1, false)
	if got := s.State().Selection.Head(); got != 9 {
		t.Fatalf("word: %d", got)
	}
	s.MoveChar(1, false)
	if got := s.State().Selection.Head(); got != 11 {
		t.Fatalf("to the next block: %d", got)
	}
	s.MoveChar(-1, true)
	if s.State().Selection.Empty() {
		t.Fatal("extended")
	}
}

func TestTrailingNodeAndTables(t *testing.T) {
	s := kitSession(t, SessionConfig{})
	if !s.RunNamed("insertTable", Attrs{"rows": 2, "cols": 2, "withHeaderRow": true}) {
		t.Fatal("insert")
	}
	doc := s.State().Doc
	if doc.Child(0).Type.Name != "table" || doc.LastChild().Type.Name != "paragraph" {
		t.Fatalf("doc %s", doc)
	}
	if !isInTable(s.State()) {
		t.Fatalf("the selection left the table: %T", s.State().Selection)
	}
	typeAll(s, "x")
	s.HandleKey("Tab")
	typeAll(s, "y")
	if got := doc.Child(0).Child(0).Child(0).Type.Name; got != "tableHeader" {
		t.Fatal(got)
	}
	if got := grid(s.State().Doc.Child(0)); len(got[0]) != 2 || got[0][0] != "x" || got[0][1] != "y" {
		t.Fatalf("grid %v", got)
	}
}
