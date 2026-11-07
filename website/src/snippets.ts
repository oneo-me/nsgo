import type { PrismTheme } from 'prism-react-renderer';

export const snippets = [
  {
    id: 'app',
    language: 'go',
    code: `package main

import (
	"fmt"

	"github.com/oneo-me/nsgo"
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
)

func App(cx *core.Scope) core.Node {
	count := core.NewSignal(cx, 0)
	return core.View{
		Style: core.Style{Padding: core.All(24), Gap: 12},
		Children: []core.Node{
			core.Text{ContentFunc: func() string {
				return fmt.Sprintf("Pressed %d times", count.Get())
			}},
			base.Button{
				Label:   "Press me",
				OnPress: func(core.PressEvent) {
					count.Update(func(n int) int { return n + 1 })
				},
				Style: core.Style{
					Padding:      core.Symmetric(6, 14),
					CornerRadius: 8,
					Background:   core.RGB(0x007aff),
					Color:        core.RGB(0xffffff),
				},
				Children: []core.Node{core.Text{Content: "Press me"}},
			},
		},
	}
}

func main() { nsgo.Run(App) }`,
  },
  {
    id: 'test',
    language: 'go',
    code: `package main

import (
	"testing"

	"github.com/oneo-me/nsgo/nsgotest"
)

func TestMain(m *testing.M) { nsgotest.Main(m) }

func TestPress(t *testing.T) {
	app := nsgotest.Start(t, App)
	app.Press("text:Press me")
	app.Find("text:Pressed 1 times")
}`,
  },
] as const;

// Code colours are the site's tokens, so they follow the appearance.
export const codeTheme: PrismTheme = {
  plain: {
    color: 'var(--code-text)',
    backgroundColor: 'transparent',
  },
  styles: [
    {
      types: ['comment', 'prolog', 'doctype', 'cdata'],
      style: { color: 'var(--code-comment)', fontStyle: 'italic' },
    },
    {
      types: ['punctuation'],
      style: { color: 'var(--code-punctuation)' },
    },
    {
      types: ['keyword', 'operator', 'boolean', 'atrule'],
      style: { color: 'var(--code-keyword)' },
    },
    {
      types: ['string', 'char', 'attr-value'],
      style: { color: 'var(--code-string)' },
    },
    {
      types: ['function', 'method'],
      style: { color: 'var(--code-function)' },
    },
    {
      types: ['type', 'class-name', 'builtin'],
      style: { color: 'var(--code-type)' },
    },
    {
      types: ['number'],
      style: { color: 'var(--code-number)' },
    },
    {
      types: ['property', 'constant', 'symbol'],
      style: { color: 'var(--code-property)' },
    },
  ],
};
