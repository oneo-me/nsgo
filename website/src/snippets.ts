// The home page's code, after README.md's Usage.

export const goApp = `package main

import (
	"fmt"

	"github.com/oneo-me/nsgo"
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
)

func Root(cx *core.Scope) core.Node {
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

func main() { nsgo.Run(Root) }`;

export const tsxApp = `import { mount, signal, Text, View } from "nsgo";

function App() {
  const count = signal(0);
  return (
    <View class="gap-3 p-6">
      <Text>Pressed {count()} times</Text>
      <View
        class="self-start rounded-lg bg-blue-500 px-3.5 py-1.5"
        onPress={() => count.update((n) => n + 1)}
      >
        <Text class="text-white">Press me</Text>
      </View>
    </View>
  );
}

mount(App);`;

export const goTest = `func TestMain(m *testing.M) { nsgotest.Main(m) }

func TestPress(t *testing.T) {
	app := nsgotest.Start(t, Root)
	app.Press("text:Press me")
	app.Find("text:Pressed 1 times")
}`;
