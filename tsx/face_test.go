package tsx_test

import "testing"

// TestAFollowedPropKeepsTheComponent checks that a change to a prop the
// component follows through a function of its own — Progress's Value — leaves
// the component and what it holds where they are. A remake would drop the
// scroll below it to the top, and give a transition no frame to start from.
func TestAFollowedPropKeepsTheComponent(t *testing.T) {
	app := run(t, `import { mount, Scroll, signal, Text, View } from "nsgo";
import { Progress } from "nsgo/base";
const tick = signal(0);
mount(() => <View style={{ flex: 1 }}>
	<Text testID="tick" onPress={() => tick.set(tick() + 1)}>{() => "tick " + tick()}</Text>
	<Progress testID="bar" value={tick()} class="w-full">
		<Scroll testID="list" style={{ height: 100 }}>
			<View style={{ height: 400 }}><Text>content</Text></View>
		</Scroll>
	</Progress>
</View>);`)

	app.Scroll("id:list", 120)
	if got := app.Find("id:list").Offset.Y; got != 120 {
		t.Fatalf("scroll offset %v after scrolling, want 120", got)
	}

	app.Press("id:tick")
	app.Settle()
	if got := app.Find("id:list").Offset.Y; got != 120 {
		t.Fatalf("scroll offset %v after the value changed, want it kept at 120", got)
	}
}
