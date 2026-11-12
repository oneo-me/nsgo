// Package highlight colors the code blocks of a TextView with chroma's
// lexers and NSH's reading palette, light and dark. It is a package of
// its own because chroma's lexers add about 4.5 MB to an app.
//
//	textview.TextView{Markdown: reply, Highlighter: highlight.New(cx)}
package highlight

import (
	"strings"
	"sync"
	"time"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"

	"github.com/nsgo-dev/nsgo/base/internal/syntax"
	"github.com/nsgo-dev/nsgo/base/textview"
	"github.com/nsgo-dev/nsgo/core"
)

// New returns a highlighter colored for the system's appearance, so an
// appearance change colors the code again without lexing it again. A
// language chroma does not know leaves the code plain.
//
// Lexing stays off the engine goroutine unless it is cheap: a block
// lexed before is colored from the cache, and one whose language lexes
// fast enough to take under 2 ms is lexed on the spot. Others show at
// once, plain or with the colors of a block they grew from (code that
// streams in), and are lexed on a goroutine of their own; the result
// colors them again.
func New(cx *core.Scope) textview.Highlighter {
	ready := core.NewSignal(cx, 0)
	var pending sync.Map
	return func(c textview.CodeBlock) []textview.Highlight {
		palette := &syntax.Light
		if cx.Dark() {
			palette = &syntax.Dark
		}
		ready.Get()
		toks, ok := cached(c.Language, c.Code)
		switch {
		case ok:
		case cheap(c.Language, len(c.Code)):
			toks = tokens(c.Language, c.Code)
		default:
			toks = grownFrom(c.Language, c.Code)
			key := [2]string{c.Language, c.Code}
			if _, running := pending.LoadOrStore(key, true); !running {
				go func() {
					tokens(c.Language, c.Code)
					pending.Delete(key)
					cx.Post(func() { ready.Update(func(n int) int { return n + 1 }) })
				}()
			}
		}
		out := make([]textview.Highlight, len(toks))
		for i, t := range toks {
			out[i] = textview.Highlight{Start: t.start, End: t.end, Style: palette[t.kind]}
		}
		return out
	}
}

// budget is how long a block may take to lex on the engine goroutine.
const budget = 2 * time.Millisecond

// speeds are each language's lexing speed as measured, in time per byte.
var speeds sync.Map

// cheap reports whether size bytes of language lex within budget, as the
// language lexed before; one not measured yet never is.
func cheap(language string, size int) bool {
	if language == "" || size == 0 {
		return true
	}
	v, ok := speeds.Load(strings.ToLower(language))
	return ok && time.Duration(float64(v.(time.Duration))*float64(size)) < budget
}

// token is a run of code of one kind, in bytes.
type token struct {
	start, end int
	kind       syntax.Kind
}

// cache keeps the tokens of the last blocks lexed, for every view in the
// process: a block shown again, or colored for the other appearance, is
// not lexed again.
var cache = struct {
	sync.Mutex
	entries map[[2]string][]token
	order   [][2]string
}{entries: map[[2]string][]token{}}

const cacheSize = 64

// cached returns code's tokens if they are in the cache.
func cached(language, code string) ([]token, bool) {
	if language == "" || code == "" {
		return nil, true
	}
	cache.Lock()
	defer cache.Unlock()
	toks, ok := cache.entries[[2]string{language, code}]
	return toks, ok
}

// grownFrom returns the tokens of the longest cached block of language
// that code starts with, which still color that part of it.
func grownFrom(language, code string) []token {
	cache.Lock()
	defer cache.Unlock()
	var best []token
	at := -1
	for _, k := range cache.order {
		if k[0] == language && len(k[1]) > at && strings.HasPrefix(code, k[1]) {
			best, at = cache.entries[k], len(k[1])
		}
	}
	return best
}

func tokens(language, code string) []token {
	if toks, ok := cached(language, code); ok {
		return toks
	}
	key := [2]string{language, code}
	start := time.Now()
	toks := lex(language, code)
	speeds.Store(strings.ToLower(language), time.Since(start)/time.Duration(max(len(code), 1)))
	cache.Lock()
	defer cache.Unlock()
	if _, ok := cache.entries[key]; !ok {
		if len(cache.order) == cacheSize {
			delete(cache.entries, cache.order[0])
			cache.order = cache.order[1:]
		}
		cache.order = append(cache.order, key)
		cache.entries[key] = toks
	}
	return toks
}

func lex(language, code string) []token {
	lexer := lexers.Get(language)
	if lexer == nil {
		return nil
	}
	it, err := chroma.Coalesce(lexer).Tokenise(&chroma.TokeniseOptions{State: "root"}, code)
	if err != nil {
		return nil
	}
	var out []token
	at := 0
	for t := it(); t != chroma.EOF; t = it() {
		end := at + len(t.Value)
		if k := syntax.KindOf(t.Type); k != syntax.Plain && end <= len(code) {
			if n := len(out); n > 0 && out[n-1].kind == k && out[n-1].end == at {
				out[n-1].end = end
			} else {
				out = append(out, token{at, end, k})
			}
		}
		at = end
	}
	return out
}
