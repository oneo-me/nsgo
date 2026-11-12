package textview

import (
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/nsgo-dev/nsgo/core"
)

// The cases are NSH's (crates/nsh-base/src/text_view/html/tests.rs).

func p(content ...Inline) Paragraph { return Paragraph{Content: content} }

func cell(text string) TableCell {
	if text == "" {
		return TableCell{}
	}
	return TableCell{Content: []Inline{Text{text}}}
}

func checkBlocks(t *testing.T, source string, got, want []Block) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%q:\n got %#v\nwant %#v", source, got, want)
	}
}

func checkHTML(t *testing.T, source string, want ...Block) {
	t.Helper()
	checkBlocks(t, source, ParseHTML(source).Blocks, want)
}

func TestHTMLParagraphsAndContainers(t *testing.T) {
	checkHTML(t, "<p>One</p><p>Two</p>", p(Text{"One"}), p(Text{"Two"}))
	// Inline runs between blocks of a container are paragraphs.
	checkHTML(t, "<div>before<section><p>inner</p></section>after <span>span</span></div>",
		p(Text{"before"}), p(Text{"inner"}), p(Text{"after span"}))
	// Empty paragraphs are dropped.
	checkHTML(t, "<p>  </p><div>\n</div><p><br></p>")
	// A bare fragment is a paragraph.
	checkHTML(t, "Hello <b>world</b>", p(Text{"Hello "}, Strong{[]Inline{Text{"world"}}}))
}

func TestHTMLWhitespaceCollapses(t *testing.T) {
	checkHTML(t, "<p>\n   a \t\n b  <em> c </em>  d\u00a0\u00a0e   </p>",
		p(Text{"a b "}, Emphasis{[]Inline{Text{"c"}}}, Text{" d\u00a0\u00a0e"}))
	// No space at the start of a line after a break, none before it.
	checkHTML(t, "<p>a <br>  b</p>", p(Text{"a"}, Break{}, Text{"b"}))
	// A space at the end of a link goes after it.
	checkHTML(t, `<p><a href="/x">link </a>next</p>`,
		p(Link{URL: "/x", Content: []Inline{Text{"link"}}}, Text{" next"}))
	// Texts merge.
	checkHTML(t, "<p>a<span>b</span>c</p>", p(Text{"abc"}))
}

func TestHTMLInlineElements(t *testing.T) {
	in := func(c string) []Inline { return []Inline{Text{c}} }
	checkHTML(t, "<p><strong>s</strong><b>b</b><em>e</em><i>i</i><cite>c</cite><del>d</del><s>s</s>"+
		"<u>u</u><ins>n</ins><mark>m</mark><sup>2</sup><sub>0</sub>"+
		"<code>let  x</code><tt>tt</tt><kbd><kbd>Ctrl</kbd>+<kbd>C</kbd></kbd>"+
		`<a href="https://a.b" title="T">l</a><a name="x">plain</a>`+
		`<img src="i.png" alt="A" title="I"><q>q</q><abbr title="x">ab</abbr><small>sm</small></p>`,
		p(Strong{in("s")}, Strong{in("b")}, Emphasis{in("e")}, Emphasis{in("i")}, Emphasis{in("c")},
			Strikethrough{in("d")}, Strikethrough{in("s")}, Underline{in("u")}, Underline{in("n")},
			Mark{Content: in("m")}, Superscript{in("2")}, Subscript{in("0")}, Code{"let x"}, Code{"tt"},
			Keyboard{"Ctrl+C"}, Link{URL: "https://a.b", Title: "T", Content: in("l")}, Text{"plain"},
			Image{Src: "i.png", Alt: "A", Title: "I"}, Text{"\u201cq\u201dab"}, Small{in("sm")}))
}

func TestHTMLEntities(t *testing.T) {
	checkHTML(t, "<p>a &amp; b &lt;c&gt; &copy; &#x4e2d;</p>", p(Text{"a & b <c> \u00a9 \u4e2d"}))
}

func TestHTMLHeadingAnchors(t *testing.T) {
	checkHTML(t, `<h1>Getting started!</h1><h2>Intro</h2><h3 id="own">Intro</h3><h2>Intro</h2>`,
		Heading{Level: 1, Content: []Inline{Text{"Getting started!"}}, Anchor: "getting-started"},
		Heading{Level: 2, Content: []Inline{Text{"Intro"}}, Anchor: "intro"},
		Heading{Level: 3, Content: []Inline{Text{"Intro"}}, Anchor: "own"},
		Heading{Level: 2, Content: []Inline{Text{"Intro"}}, Anchor: "intro-1"})
}

func TestHTMLQuotesRulesImagesAndAlerts(t *testing.T) {
	checkHTML(t, `<blockquote><p>q</p>tail</blockquote><hr><img src="a.png">`,
		Blockquote{Children: []Block{p(Text{"q"}), p(Text{"tail"})}}, Rule{}, p(Image{Src: "a.png"}))
	checkHTML(t, `<div class="markdown-alert markdown-alert-warning">`+
		`<p class="markdown-alert-title"><svg><path/></svg>Warning</p><p>Careful</p></div>`,
		Blockquote{Children: []Block{p(Text{"Careful"})}, Alert: AlertWarning})
}

func TestHTMLLists(t *testing.T) {
	checkHTML(t, `<ol start="3"><li>a</li><li>b<ul><li>c</li></ul></li></ol>`,
		List{Ordered: true, Start: 3, Items: []ListItem{
			{Children: []Block{p(Text{"a"})}},
			{Children: []Block{p(Text{"b"}), List{Items: []ListItem{{Children: []Block{p(Text{"c"})}}}}}},
		}})
	checkHTML(t, "<ul><li><p>a</p></li></ul>", List{Loose: true, Items: []ListItem{{Children: []Block{p(Text{"a"})}}}})
	checkHTML(t, `<ul><li><input type="checkbox" checked disabled> done</li><li><input type=checkbox> todo</li>`+
		`<li><p><input type="checkbox"> in p</p></li><li>plain</li></ul>`,
		List{Loose: true, Items: []ListItem{
			{Task: true, Done: true, Children: []Block{p(Text{"done"})}},
			{Task: true, Children: []Block{p(Text{"todo"})}},
			{Task: true, Children: []Block{p(Text{"in p"})}},
			{Children: []Block{p(Text{"plain"})}},
		}})
}

func TestHTMLPreKeepsText(t *testing.T) {
	checkHTML(t, "<pre><code class=\"hljs language-rust\">fn main() {\n    <span>x</span>  &lt;y&gt;\n}\n</code></pre>",
		CodeBlock{Language: "rust", Code: "fn main() {\n    x  <y>\n}"})
	checkHTML(t, "<pre>a\n\n  b</pre>", CodeBlock{Code: "a\n\n  b"})
}

func TestHTMLTables(t *testing.T) {
	checkHTML(t, `<table><caption>Cap</caption><thead><tr><th align="left">A</th>`+
		`<th style="text-align: center">B</th><th>C</th></tr></thead>`+
		`<tbody><tr><td>1</td><td>2</td></tr><tr><td>3</td><td>4</td><td align=right>5</td><td>6</td></tr></tbody></table>`,
		p(Text{"Cap"}),
		Table{Align: []ColumnAlign{AlignLeft, AlignCenter, AlignNone, AlignNone},
			Head: []TableCell{cell("A"), cell("B"), cell("C"), cell("")},
			Rows: [][]TableCell{{cell("1"), cell("2"), cell(""), cell("")}, {cell("3"), cell("4"), cell("5"), cell("6")}}})
	c := cell("c")
	c.ColSpan = 2
	checkHTML(t, `<table><tr><td>a</td><td>b</td></tr><tr><td colspan=2 style='text-align:right'>c</td></tr></table>`,
		Table{Align: []ColumnAlign{AlignRight, AlignRight}, Head: []TableCell{cell("a"), cell("b")}, Rows: [][]TableCell{{c}}})
}

func TestHTMLDetailsAndDefinitions(t *testing.T) {
	checkHTML(t, "<details open><summary>More <b>info</b></summary><p>Body</p>text</details>",
		Details{Open: true, Summary: []Inline{Text{"More "}, Strong{[]Inline{Text{"info"}}}},
			Children: []Block{p(Text{"Body"}), p(Text{"text"})}})
	checkHTML(t, "<details>x</details>", Details{Children: []Block{p(Text{"x"})}})
	checkHTML(t, "<dl><dt>Term</dt><dd>Meaning</dd></dl>", p(Strong{[]Inline{Text{"Term"}}}), p(Text{"Meaning"}))
	checkHTML(t, `<figure><img src="a.png" alt="A"><figcaption>Caption</figcaption></figure>`,
		p(Image{Src: "a.png", Alt: "A"}), p(Text{"Caption"}))
}

func TestHTMLWholePage(t *testing.T) {
	checkHTML(t, "<!DOCTYPE html><html><head><title>Title</title><style>p{}</style>"+
		"<script>alert(1)</script></head><body><!-- comment --><nav hidden>menu</nav>"+
		"<article><h1>Story</h1><p>Text<script>x()</script></p>"+
		"<template><p>t</p></template><noscript>ns</noscript></article></body></html>",
		Heading{Level: 1, Content: []Inline{Text{"Story"}}, Anchor: "story"}, p(Text{"Text"}))
}

func TestHTMLMalformed(t *testing.T) {
	checkHTML(t, "<p>a<b>bold<i>both</p><p>next",
		p(Text{"a"}, Strong{[]Inline{Text{"bold"}, Emphasis{[]Inline{Text{"both"}}}}}),
		p(Strong{[]Inline{Emphasis{[]Inline{Text{"next"}}}}}))
	deep := strings.Repeat("<div><span>", 5000) + "x"
	for _, s := range []string{"", "<", "</p>", "<table><td>x", "<ul><li><ul>", "<<>>&&;", "<pre>", "<details><summary>", deep} {
		ParseHTML(s)
		htmlInlines(s, nil)
	}
}

func TestHTMLTagExtensions(t *testing.T) {
	ext := Extension{Name: "callout", Tag: "Callout"}
	source := `<Callout kind="tip"><p>Use <b>it</b></p></Callout><p>See <callout kind=x>this</callout>.</p>`
	checkBlocks(t, source, ParseHTML(source, ext).Blocks, []Block{
		Custom{Name: "callout", Attributes: map[string]string{"kind": "tip"}, Text: "Use it",
			Children: []Block{p(Text{"Use "}, Strong{[]Inline{Text{"it"}}})}},
		p(Text{"See "}, Custom{Name: "callout", Attributes: map[string]string{"kind": "x"}, Text: "this",
			Content: []Inline{Text{"this"}}}, Text{"."}),
	})
	// Closed as JSX closes an empty element.
	source = `<Callout kind="a" /><p>after</p>`
	checkBlocks(t, source, ParseHTML(source, ext).Blocks, []Block{
		Custom{Name: "callout", Attributes: map[string]string{"kind": "a"}}, p(Text{"after"}),
	})
}

func TestHTMLPatternExtensions(t *testing.T) {
	ext := Extension{Name: "mention", Pattern: regexp.MustCompile(`@(\w+)`)}
	source := "<p>hi <b>@ann</b> <code>@not</code></p>"
	checkBlocks(t, source, ParseHTML(source, ext).Blocks, []Block{
		p(Text{"hi "}, Strong{[]Inline{Custom{Name: "mention", Text: "@ann", Attributes: map[string]string{"1": "ann"}}}},
			Text{" "}, Code{"@not"}),
	})
}

func TestHTMLMath(t *testing.T) {
	checkHTML(t, `<p>Area <span class="katex"><math><semantics><mi>r</mi>`+
		`<annotation encoding="application/x-tex">\pi r^2</annotation></semantics></math>`+
		`<span class="katex-html" aria-hidden="true">πr2</span></span></p>`,
		p(Text{"Area "}, Math{TeX: `\pi r^2`}))
	checkHTML(t, `<math display="block"><annotation encoding="application/x-tex">x^2</annotation></math>`, Math{TeX: "x^2"})
}

func TestHTMLFragments(t *testing.T) {
	check := func(source string, want ...Inline) {
		t.Helper()
		if got := htmlInlines(source, nil); !reflect.DeepEqual(got, want) {
			t.Errorf("%q:\n got %#v\nwant %#v", source, got, want)
		}
	}
	check("<b>x</b>", Strong{[]Inline{Text{"x"}}})
	check("<br>", Break{})
	check(" <sup>2</sup> ", Text{" "}, Superscript{[]Inline{Text{"2"}}}, Text{" "})
	check("<p>a</p><p>b</p>", Text{"a"}, Break{}, Text{"b"})
	checkBlocks(t, "div", htmlBlocks("<div>\n<p>x</p>\n</div>", nil), []Block{p(Text{"x"})})
}

func TestHTMLImageLengths(t *testing.T) {
	for s, want := range map[string]core.Length{"100px": core.Pt(100), "100%": core.Pct(100), "56%": core.Pct(56),
		"240": core.Pt(240), " 32 px ": core.Pt(32), "12.5": core.Pt(12.5)} {
		if got, ok := parseImageLength(s); !ok || got != want {
			t.Errorf("parseImageLength(%q) = %v, %v", s, got, ok)
		}
	}
	for _, s := range []string{"", "auto", "2em", "-4", "px", "%", "NaN", "inf"} {
		if _, ok := parseImageLength(s); ok {
			t.Errorf("parseImageLength(%q) is a length", s)
		}
	}
	checkHTML(t, `<img src="https://example.com/image.png" alt="Example" width="100" height="200" title="Example Image" />`,
		p(Image{Src: "https://example.com/image.png", Alt: "Example", Title: "Example Image", Width: core.Pt(100), Height: core.Pt(200)}))
	checkHTML(t, `<img src="a.png" style="width: 80%" />`, p(Image{Src: "a.png", Width: core.Pct(80)}))
	checkHTML(t, `<img src="a.png" width="10" style="width: 50px; height: 20px">`,
		p(Image{Src: "a.png", Width: core.Pt(10), Height: core.Pt(20)}))
	checkHTML(t, `<img src="a.png" width="auto" height="2em">`, p(Image{Src: "a.png"}))
	checkHTML(t, `<p>Avatars <img src="a.png" alt="A" width="32" height="32" /> and <img src="b.png" width="32" height="32"> here.</p>`,
		p(Text{"Avatars "}, Image{Src: "a.png", Alt: "A", Width: core.Pt(32), Height: core.Pt(32)}, Text{" and "},
			Image{Src: "b.png", Width: core.Pt(32), Height: core.Pt(32)}, Text{" here."}))
	checkHTML(t, `<a href="https://x.y"><img src="a.png" width="20"></a>`,
		p(Link{URL: "https://x.y", Content: []Inline{Image{Src: "a.png", Width: core.Pt(20)}}}))
}

func TestHTMLMarkColors(t *testing.T) {
	marks := func(source string) []core.Color {
		var out []core.Color
		for _, in := range ParseHTML(source).Blocks[0].(Paragraph).Content {
			if m, ok := in.(Mark); ok {
				out = append(out, m.Color)
			}
		}
		return out
	}
	for source, want := range map[string][]core.Color{
		"<p>Hello <mark>world</mark></p>": {0},
		`<p><mark color="blue">blue</mark> and <mark style="background-color: #336699">hex</mark></p>`: {core.RGB(0x3b82f6), core.RGB(0x336699)},
		`<p><mark color="#abc" style="background-color: red">a</mark></p>`:                             {core.RGB(0xaabbcc)},
		`<p><mark style="background-color: rgb(1 2 3)">a</mark><mark color="nope">b</mark></p>`:        {core.RGB(0x010203), 0},
		`<p><mark style="background-color: hsl(0 100% 50% / 50%)">a</mark></p>`:                        {core.Color(0xff000080)},
	} {
		if got := marks(source); !reflect.DeepEqual(got, want) {
			t.Errorf("%q: colors %08x, want %08x", source, got, want)
		}
	}
}

func TestMarkdownHTML(t *testing.T) {
	check := func(source string, want ...Block) {
		t.Helper()
		checkBlocks(t, source, Parse(source).Blocks, want)
	}
	strong := func(c ...Inline) Inline { return Strong{c} }
	check("Press <kbd>Ctrl</kbd> + <b>x</b>, E=mc<sup>2</sup><br>next",
		p(Text{"Press "}, Keyboard{"Ctrl"}, Text{" + "}, strong(Text{"x"}), Text{", E=mc"},
			Superscript{[]Inline{Text{"2"}}}, Break{}, Text{"next"}))
	// Flow HTML with Markdown between its tags.
	check("<details>\n<summary>More</summary>\n\nHidden **text**\n\n</details>\n\nAfter",
		Details{Summary: []Inline{Text{"More"}}, Children: []Block{p(Text{"Hidden "}, strong(Text{"text"}))}},
		p(Text{"After"}))
	check("a <strong>b *c* <em>d</em></strong> e <b>f</b> <i>g</i> <del>h</del> <br> <strong>unclosed",
		p(Text{"a "}, strong(Text{"b "}, Emphasis{[]Inline{Text{"c"}}}, Text{" "}, Emphasis{[]Inline{Text{"d"}}}),
			Text{" e "}, strong(Text{"f"}), Text{" "}, Emphasis{[]Inline{Text{"g"}}}, Text{" "},
			Strikethrough{[]Inline{Text{"h"}}}, Text{" "}, Break{}, Text{" unclosed"}))
	check(`<a href="https://x.y" title="T">**go**</a> <mark color="blue">`+"`x`</mark>",
		p(Link{URL: "https://x.y", Title: "T", Content: []Inline{strong(Text{"go"})}}, Text{" "},
			Mark{Content: []Inline{Code{"x"}}, Color: core.RGB(0x3b82f6)}))
	// Elements with literal content stay HTML.
	check("<code>*x*</code>", p(Code{"*x*"}))
	check(`Before <img src="https://example.com/avatar.png" alt="Avatar" width="32" height="32" /> after.`,
		p(Text{"Before "}, Image{Src: "https://example.com/avatar.png", Alt: "Avatar", Width: core.Pt(32), Height: core.Pt(32)},
			Text{" after."}))
	check(`<img src="a.png" alt="A" width="100%" />`, p(Image{Src: "a.png", Alt: "A", Width: core.Pct(100)}))
	check("<div>\nblock\n</div>", p(Text{"block"}))
	check(`Plain <mark>highlighted</mark> text and <mark data_id="1" color="#ff000059">red</mark>.`,
		p(Text{"Plain "}, Mark{Content: []Inline{Text{"highlighted"}}}, Text{" text and "},
			Mark{Content: []Inline{Text{"red"}}, Color: core.Color(0xff000059)}, Text{"."}))
	// A tag extension holding Markdown.
	source := "<Callout kind=\"tip\">\n\n**Use** it\n\n</Callout>"
	checkBlocks(t, source, Parse(source, Extension{Name: "callout", Tag: "Callout"}).Blocks, []Block{
		Custom{Name: "callout", Attributes: map[string]string{"kind": "tip"}, Text: "Use it",
			Children: []Block{p(strong(Text{"Use"}), Text{" it"})}},
	})
}
