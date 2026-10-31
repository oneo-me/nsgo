package textview_test

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/oneo-me/nsgo/base/textview"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/nsgotest"
)

const doc = "# Title\n\n[Docs](https://nsgo.dev) and **bold**.\n\n- one\n- [x] two\n\n```go\nx := 1\n```\n\n" +
	"| A | B |\n|---|---|\n| 1 | 2 |\n\n> [!TIP]\n> Use it.\n\n:::callout\nInside\n:::\n\nNote[^1].\n\n[^1]: The note."

func TestTextViewBuildsBlocks(t *testing.T) {
	var pressed []string
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return textview.TextView{Markdown: doc, Selectable: true, TestID: "doc",
			Extensions:  []textview.Extension{{Name: "callout", Container: "callout"}},
			OnLinkPress: func(url string) { pressed = append(pressed, url) },
			Highlighter: func(c textview.CodeBlock) []textview.Highlight {
				return []textview.Highlight{{Start: 0, End: 1, Style: core.Style{Color: core.RGB(0xff0000)}}, {Start: 0, End: 2}}
			},
			RenderCustom: func(cx *core.Scope, node textview.Custom) core.Node {
				return core.Text{Content: "custom " + node.Name, TestID: "custom"}
			},
			CodeBlockActions: []string{textview.CopyAction},
			RenderCodeBlockAction: func(cx *core.Scope, a textview.CodeBlockAction) core.Node {
				return core.Text{ContentFunc: func() string {
					if a.Done() {
						return "copied"
					}
					return a.Name
				}}
			},
			OnCodeBlockAction: func(name string, code textview.CodeBlock) { pressed = append(pressed, name+" "+code.Code) },
		}
	})
	if !app.Find("id:doc").Props.SelectionArea {
		t.Error("a selectable text view is no selection area")
	}
	app.Find("role:header Title")
	for _, text := range []string{"Docs and bold.", "•", "one", "two", "x := 1", "A", "2", "Tip", "Use it.", "custom callout", "Note[1].", "1.", "The note."} {
		app.Find("text:" + text)
	}
	if !app.Find("role:checkbox").Props.Checked {
		t.Error("the done task is not checked")
	}
	app.Press("text:Docs and bold.")
	app.Press("label:copy")
	if want := []string{"https://nsgo.dev", "copy x := 1"}; strings.Join(pressed, "|") != strings.Join(want, "|") {
		t.Errorf("pressed %q, want %q", pressed, want)
	}
	if app.Clipboard != "x := 1" || !app.Exists("text:copied") {
		t.Errorf("clipboard %q, copied shown %v", app.Clipboard, app.Exists("text:copied"))
	}
	app.Advance(2 * time.Second)
	if app.Exists("text:copied") {
		t.Error("the copy action stays done")
	}
}

func TestTextViewStreamsWithACaret(t *testing.T) {
	var md *core.Signal[string]
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		md = core.NewSignal(cx, "")
		return textview.TextView{MarkdownFunc: md.Get, Options: textview.Options{Streaming: true},
			StreamingCaret: core.View{TestID: "caret", Style: core.Style{Width: core.Pt(6), Height: core.Pt(12)}}}
	})
	app.Find("id:caret")
	md.Set("Some **bold")
	app.Settle()
	if p := app.Find("id:caret").Parent.Parent; p.Kind != nsgotest.KindText {
		t.Errorf("the caret is not in the paragraph's text: %v", p.Kind)
	}
	app.Find("text:Some bold")
	md.Set("Some **bold**\n\n- one\n- two")
	app.Settle()
	if n := len(app.Query("id:caret")); n != 1 {
		t.Fatalf("%d carets", n)
	}
	if !strings.Contains(app.Find("id:caret").Parent.Parent.Text, "two") {
		t.Error("the caret is not at the end of the last item")
	}
	md.Set("Done\n\n---")
	app.Settle()
	if p := app.Find("id:caret").Parent.Parent; p != nil && p.Kind == nsgotest.KindText {
		t.Error("after a rule the caret is not on a line of its own")
	}
}

func TestTextViewOptionsAndPlugins(t *testing.T) {
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		textview.Provide(cx, textview.Plugin{
			Extension: textview.Extension{Name: "mention", Pattern: regexp.MustCompile(`@\w+`)},
			Render:    func(cx *core.Scope, n textview.Custom) core.Node { return core.Text{Content: "user " + n.Text} },
		})
		return core.View{Children: []core.Node{
			textview.TextView{Markdown: "---\ntitle: x\n---\nHi @ada, $5 is <b>cheap</b>.\n\n> [!NOTE]\n> Plain.",
				Options:    textview.Options{Frontmatter: true, NoHTML: true, NoAlerts: true, NoMath: true},
				CustomSpan: func(n textview.Custom) core.Span { return core.Span{Content: "user " + n.Text} },
				RenderCustom: func(cx *core.Scope, n textview.Custom) core.Node {
					return core.Text{Content: n.Name + ": " + n.Text, TestID: n.Name}
				}},
			textview.TextView{Markdown: "<Chart kind=\"bar\" />\n\nIt is {1 + 2}.", Options: textview.Options{MDX: true},
				CustomSpan: func(n textview.Custom) core.Span { return core.Span{Content: "[" + n.Name + ":" + n.Text + "]"} },
				RenderCustom: func(cx *core.Scope, n textview.Custom) core.Node {
					return core.Text{Content: n.Name + " " + n.Attributes["kind"], TestID: "mdx-" + n.Name}
				}},
		}}
	})
	if got := app.Find("id:frontmatter").Text; got != "frontmatter: title: x" {
		t.Errorf("front matter %q", got)
	}
	app.Find("text:Hi user @ada, $5 is <b>cheap</b>.")
	app.Find("text:[!NOTE]")
	app.Find("text:chart bar")
	app.Find("text:It is [expression:1 + 2].")
}

func TestTextViewRebuildsOnlyChangedBlocks(t *testing.T) {
	var md *core.Signal[string]
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		md = core.NewSignal(cx, "First paragraph.\n\nSecond")
		return textview.TextView{MarkdownFunc: md.Get}
	})
	first := app.Find("text:First paragraph.").ID
	md.Set("First paragraph.\n\nSecond, longer")
	app.Settle()
	if app.Find("text:First paragraph.").ID != first {
		t.Error("an unchanged block was built again")
	}
	app.Find("text:Second, longer")
	md.Set("First paragraph.\n\n| a |\n|---|\n| b |")
	app.Settle()
	if app.Exists("text:Second") || !app.Exists("text:b") {
		t.Error("a block that changed kind was not replaced")
	}
}

func TestTextViewShowsHTML(t *testing.T) {
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return textview.TextView{HTML: "<h2>Title</h2><p>Press <kbd>Ctrl</kbd> and <mark>see</mark> x<sup>2</sup>.</p>" +
			"<details><summary>More</summary><p>Hidden</p></details>"}
	})
	app.Find("role:header Title")
	app.Find("text:Press Ctrl and see x2.")
	app.Find("text:▸")
	if app.Exists("text:Hidden") {
		t.Error("a closed disclosure shows its blocks")
	}
	app.Press("role:button")
	app.Settle()
	app.Find("text:▾")
	app.Find("text:Hidden")
}

func TestTextViewRendersFormulasAndInlineNodes(t *testing.T) {
	var shown []string
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return textview.TextView{
			Markdown:   "Energy $E=mc^2$ and ![dot](dot.png) and @ada.\n\n$$\nx^2\n$$",
			Extensions: []textview.Extension{{Name: "mention", Pattern: regexp.MustCompile(`@(\w+)`)}},
			RenderMath: func(cx *core.Scope, m textview.Math, display bool) core.Node {
				shown = append(shown, fmt.Sprint(m.TeX, " ", display))
				return core.View{TestID: "math " + m.TeX, Style: core.Style{Width: core.Pt(10), Height: core.Pt(10)}}
			},
			RenderCustom: func(cx *core.Scope, node textview.Custom) core.Node {
				return core.View{TestID: "custom " + node.Text, Style: core.Style{Width: core.Pt(10), Height: core.Pt(10)}}
			},
		}
	})
	app.Find("id:math E=mc^2")
	app.Find("id:math x^2")
	app.Find("id:custom @ada")
	app.Find("role:image")
	if got := strings.Join(shown, "|"); got != "E=mc^2 false|x^2 true" {
		t.Errorf("rendered %q", got)
	}
	app.Find("text:Energy ￼ and ￼ and ￼.")
}

func TestTextViewReportsImagePressesAndOpensLinksThatLeave(t *testing.T) {
	var links []string
	var images []string
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		return textview.TextView{
			Markdown: "![Alone](a.png)\n\n[![Linked](b.png)](https://b.example)\n\nSee [the site](https://nsgo.dev), [mail](mailto:x@nsgo.dev), " +
				"[a section](#title) and [a page](docs/page.md) ![inline](c.png).",
			OpenLinks:    true,
			Classes:      textview.Classes{textview.PartImage: "size-10"},
			OnLinkPress:  func(url string) { links = append(links, url) },
			OnImagePress: func(im textview.Image) { images = append(images, im.Alt) },
		}
	})
	app.Press("label:Alone")
	app.Press("label:Linked")
	app.Press("label:inline")
	for _, l := range []string{"the site", "mail", "a section", "a page"} {
		app.Press("text:" + l)
	}
	if want := "Alone|inline"; strings.Join(images, "|") != want {
		t.Errorf("images pressed %q, want %q", images, want)
	}
	if want := "https://b.example|https://nsgo.dev|mailto:x@nsgo.dev|#title|docs/page.md"; strings.Join(links, "|") != want {
		t.Errorf("links pressed %q, want %q", links, want)
	}
	if want := "https://b.example|https://nsgo.dev|mailto:x@nsgo.dev"; strings.Join(app.OpenedURLs, "|") != want {
		t.Errorf("opened %q, want the links that leave the document", app.OpenedURLs)
	}
}
