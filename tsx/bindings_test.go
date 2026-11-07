package tsx_test

import (
	"testing"
	"time"
)

// Each binding of nsgo/base in an app: what TSX passes reaches the Go
// component, and what it reports comes back in the event's shape.

func TestSelectReportsItsValue(t *testing.T) {
	app := run(t, `import { mount, signal, Text, View } from "nsgo";
import { Select } from "nsgo/base";
function App() {
	const value = signal(null), open = signal("closed");
	return <View>
		<Select testID="select" style={{ width: 100, height: 30 }} items={[{ value: "a", title: "Apple" }, { value: "b", title: "Banana" }]}
			value={value()} onChange={(e) => value.set(e.value)} onOpenChange={(e) => open.set(String(e.open))} />
		<Text>{() => "value " + value() + " open " + open()}</Text>
	</View>;
}
mount(App);`)
	app.Press("id:select")
	app.Settle()
	app.WaitFor("text:open true", time.Second)
	app.Press("text:Banana")
	app.WaitFor("text:value b open false", time.Second)
}

func TestCommandFiltersAndRuns(t *testing.T) {
	app := run(t, `import { mount, signal, Text, View } from "nsgo";
import { Command } from "nsgo/base";
function App() {
	const ran = signal("");
	return <View>
		<Command testID="command" defaultQuery="ban" items={[{ id: "a", label: "Apple", keywords: ["fruit"] }, { id: "b", label: "Banana", group: "Long", shortcut: "mod+b" }]}
			onSelect={(e) => ran.set(e.id)} />
		<Text>{() => "ran " + ran()}</Text>
	</View>;
}
mount(App);`)
	if app.Exists("text:Apple") || !app.Exists("text:Banana") {
		t.Fatalf("filtering shows %q", app.Texts())
	}
	app.Press("text:Banana")
	app.WaitFor("text:ran b", time.Second)
}

func TestNumericControlsReportTheirValues(t *testing.T) {
	app := run(t, `import { mount, signal, Text, View } from "nsgo";
import { NumberInput, OtpInput, Pagination, Slider } from "nsgo/base";
function App() {
	const n = signal(1), page = signal(2), otp = signal(""), done = signal("");
	return <View>
		<NumberInput testID="n" value={n()} min={0} max={3} step={2} incrementButton={<Text>More</Text>}
			onChange={(e) => n.set(e.value)} />
		<Pagination page={page()} totalPages={5} nextAccessibilityLabel="Next" onChange={(e) => page.set(e.page)} />
		<OtpInput testID="otp" length={2} value={otp()} accessibilityLabel="Code" onChange={(e) => otp.set(e.value)}
			onComplete={(e) => done.set(e.value)} />
		<Slider testID="range" defaultValue={[10, 20]} thumbTestID="end" startThumbTestID="start" thumbStyle={{ width: 10, height: 10 }} />
		<Slider testID="one" value={30} accessibilityLabel="One" valueText={(v) => v + " units"} thumbStyle={{ width: 10, height: 10 }} />
		<Text>{() => ["n", n(), "page", page(), "otp", otp(), "done", done()].join(" ")}</Text>
	</View>;
}
mount(App);`)
	app.Press("text:More")
	app.WaitFor("text:n 3 page 2 otp  done ", time.Second)
	app.Press("text:Next")
	app.WaitFor("text:n 3 page 3 otp  done ", time.Second)
	if !app.Exists("id:start") || !app.Exists("id:end") {
		t.Errorf("a range has no two thumbs: %q", app.Texts())
	}
	if v := app.Find("label:One"); v.Props.Value.Text != "30 units" {
		t.Errorf("the slider says %+v", v.Props.Value)
	}
}

func TestTreeExpandsAndSelects(t *testing.T) {
	app := run(t, `import { mount, signal, Text, View } from "nsgo";
import { Tree } from "nsgo/base";
function App() {
	const open = signal(["root"]), chose = signal("");
	return <View style={{ height: 300 }}>
		<Tree items={[{ id: "root", label: "Root", children: [{ id: "leaf", label: "Leaf" }] }]} expanded={open()}
			onExpandedChange={(e) => open.set(e.expanded)} onSelectedChange={(e) => chose.set(e.selected)} />
		<Text>{() => "chose " + chose()}</Text>
	</View>;
}
mount(App);`)
	app.Press("text:Leaf")
	app.WaitFor("text:chose leaf", time.Second)
}

func TestAccordionOpensItsItems(t *testing.T) {
	app := run(t, `import { mount, signal, Text, View } from "nsgo";
import { Accordion, AccordionItem, AccordionPanel, AccordionTrigger } from "nsgo/base";
function App() {
	const value = signal([]);
	return <View>
		<Accordion value={value()} onChange={(e) => value.set(e.value)}>
			<AccordionItem value="one">
				<AccordionTrigger><Text>One</Text></AccordionTrigger>
				<AccordionPanel><Text>Inside</Text></AccordionPanel>
			</AccordionItem>
		</Accordion>
		<Text>{() => "open " + value().join()}</Text>
	</View>;
}
mount(App);`)
	app.Press("text:One")
	app.WaitFor("text:open one", time.Second)
}

func TestTimeFieldShowsItsValue(t *testing.T) {
	app := run(t, `import { mount } from "nsgo";
import { Calendar, TimeField } from "nsgo/base";
mount(() => <>
	<TimeField defaultValue="09:41" hourCycle="h23" accessibilityLabel="Time" />
	<Calendar defaultValue="2026-11-08" />
</>);`)
	if !app.Exists("text:09") || !app.Exists("text:41") {
		t.Errorf("the field shows %q", app.Texts())
	}
	if !app.Exists("text:2026") {
		t.Errorf("the calendar shows %q", app.Texts())
	}
}

func TestToastManagerTracksItsEntries(t *testing.T) {
	app := run(t, `import { mount, signal, Text, View } from "nsgo";
import { ToastManager, ToastStack, Toast } from "nsgo/base";
const toasts = new ToastManager();
function App() {
	const said = signal("");
	const report = () => said.set(toasts.entries().map((e) => e.id + ":" + e.value + ":" + e.status).join() + " " + toasts.len());
	return <View>
		<Text testID="push" onPress={() => { toasts.push("a", "Saved", { timeout: null }); toasts.push("b", "Sent"); report(); }}>Push</Text>
		<Text testID="dismiss" onPress={() => { said.set(toasts.dismiss("nope") + " " + toasts.dismiss("a")); }}>Dismiss</Text>
		<Text testID="get" onPress={() => { said.set(String(toasts.get("b")) + " " + toasts.visible(1).length + " " + toasts.isEmpty()); }}>Get</Text>
		<Text testID="all" onPress={() => { said.set(toasts.dismissAll().join()); }}>All</Text>
		<Text>{() => "said " + said()}</Text>
	</View>;
}
mount(App);`)
	app.Press("id:push")
	app.WaitFor("text:said a:Saved:starting,b:Sent:starting 2", time.Second)
	app.Press("id:get")
	app.WaitFor("text:said Sent 1 false", time.Second)
	app.Press("id:dismiss")
	app.WaitFor("text:said false true", time.Second)
	app.Press("id:all")
	app.WaitFor("text:said b", time.Second)
}

func TestAlertDialogNeedsActions(t *testing.T) {
	app := run(t, `import { mount, signal, Text } from "nsgo";
import { AlertDialog } from "nsgo/base";
function App() {
	const said = signal("");
	return <>
		<Text testID="ask" onPress={() => AlertDialog.show({ title: "Empty" }).catch((e) => said.set(e.message))}>Ask</Text>
		<Text>{() => "said " + said()}</Text>
	</>;
}
mount(App);`)
	app.Press("id:ask")
	app.WaitFor("text:said AlertDialog.show needs at least one of its actions", time.Second)
}

func TestAnimatedCompositesRunInOrder(t *testing.T) {
	app := run(t, `import { mount, signal, createAnimatedValue, Animated, Easing, Text, View } from "nsgo";
function App() {
	const a = createAnimatedValue(0), b = createAnimatedValue(0), said = signal("");
	const timing = (v, to) => Animated.timing(v, { toValue: to, duration: 50, easing: Easing.inOut(Easing.cubic) });
	const go = (name, anim) => <Text testID={name} onPress={() => anim().start(async ({ finished }) => said.set(name + " " + finished + " " + Math.round(await a.getValue()) + " " + Math.round(await b.getValue())))}>{name}</Text>;
	return <View>
		{go("sequence", () => Animated.sequence([timing(a, 1), Animated.delay(10), timing(b, 2)]))}
		{go("parallel", () => Animated.parallel([timing(a, 3), timing(b, 4)]))}
		{go("stagger", () => Animated.stagger(10, [timing(a, 5), timing(b, 6)]))}
		{go("spring", () => Animated.spring(a, { toValue: 7, stiffness: 400, damping: 40 }))}
		<Text>{() => "said " + said()}</Text>
	</View>;
}
mount(App);`)
	for _, step := range []struct{ press, want string }{
		{"sequence", "said sequence true 1 2"},
		{"parallel", "said parallel true 3 4"},
		{"stagger", "said stagger true 5 6"},
		{"spring", "said spring true 7 6"},
	} {
		app.Press("id:" + step.press)
		app.FastForward(5 * time.Second)
		app.WaitFor("text:"+step.want, time.Second)
	}
}

func TestAnimatedNodesCompute(t *testing.T) {
	app := run(t, `import { mount, signal, createAnimatedValue, createMotion, Animated, LayoutAnimation, Text, View } from "nsgo";
function App() {
	const a = createAnimatedValue(6), b = createAnimatedValue(2), said = signal("");
	const sum = Animated.add(a, b), diff = Animated.subtract(a, 1), product = Animated.multiply(a, b);
	const quotient = Animated.divide(a, b), rest = Animated.modulo(a, 4);
	const spring = Animated.sampleSpring({ from: 0, toValue: 1, stiffness: 100, damping: 10 });
	const decay = Animated.sampleDecay({ from: 0, velocity: 1 });
	const motion = createMotion({ initial: { opacity: 0.25 } });
	return <View>
		<View testID="bound" style={{ width: sum, height: 10, opacity: quotient, left: product, top: diff, right: rest }} />
		<View testID="moved" style={{ opacity: motion.opacity, width: 10, height: 10 }} />
		<Text testID="layout" onPress={() => { LayoutAnimation.configureNext(LayoutAnimation.create(50), () => said.set("laid out")); b.set(3); }}>Layout</Text>
		<Text testID="offset" onPress={async () => { a.setOffset(4); const with_ = await a.getValue(); a.flattenOffset(); said.set(with_ + " " + await a.getValue()); }}>Offset</Text>
		<Text testID="stop" onPress={async () => { Animated.timing(a, { toValue: 100, duration: 1000 }).start(); said.set("stopped " + (await a.stop() < 100)); }}>Stop</Text>
		<Text>{() => "said " + said()}</Text>
		<Text>{() => "samples " + (spring.length > 0) + " " + (decay.length > 0)}</Text>
	</View>;
}
mount(App);`)
	if !app.Exists("text:samples true true") {
		t.Errorf("sampling shows %q", app.Texts())
	}
	if v := app.Find("id:bound"); v.Animated.Set == 0 {
		t.Errorf("the view binds no animated values: %+v", v.Animated)
	}
	if v := app.Find("id:moved"); v.Animated.Set == 0 {
		t.Errorf("the motion binds no animated values: %+v", v.Animated)
	}
	app.Press("id:offset")
	app.WaitFor("text:said 10 10", time.Second)
	app.Press("id:layout")
	app.FastForward(time.Second)
	app.WaitFor("text:said laid out", time.Second)
	app.Press("id:stop")
	app.WaitFor("text:said stopped true", time.Second)
}

func TestBaseListChoosesRows(t *testing.T) {
	app := run(t, `import { mount, signal, Text, View } from "nsgo";
import { List } from "nsgo/base";
function App() {
	const chose = signal(["Plum"]), active = signal("");
	return <View style={{ height: 300 }}>
		<List accessibilityLabel="Fruits" each={["Apple", "Pear", "Plum"]} selected={chose()} style={{ height: 200 }}
			itemStyle={{ height: 30 }} rowLabel={(item) => item}
			onSelectedChange={(e) => chose.set(e.selected)} onActivate={(e) => active.set(e.item + " " + e.index)}>
			{(item) => <Text>{item}</Text>}
		</List>
		<Text>{() => "chose " + chose().join()}</Text>
		<Text>{() => "active " + active()}</Text>
	</View>;
}
mount(App);`)
	if !app.Find("role:option Plum").Props.Selected {
		t.Fatal("the selected prop chose no row")
	}
	app.Press("role:option Pear")
	app.WaitFor("text:chose Pear", time.Second)
	if !app.Find("role:option Pear").Props.Selected || app.Find("role:option Plum").Props.Selected {
		t.Fatal("the rows do not follow the selection")
	}
	app.Key("enter")
	app.WaitFor("text:active Pear 1", time.Second)
}

func TestBaseListKeepsAControlledSelection(t *testing.T) {
	app := run(t, `import { mount, Text } from "nsgo";
import { List } from "nsgo/base";
mount(() => <List each={[{ id: "a" }, { id: "b" }]} keyExtractor={(item) => item.id} selected={["a"]} rowLabel={(item) => item.id}
	style={{ height: 200 }}>{(item) => <Text>{() => item().id}</Text>}</List>);`)
	app.Press("role:option b")
	app.Settle()
	if app.Find("role:option b").Props.Selected || !app.Find("role:option a").Props.Selected {
		t.Fatal("the list left the selected prop")
	}
}

func TestBaseListWithoutChoices(t *testing.T) {
	app := run(t, `import { mount, Text } from "nsgo";
import { List } from "nsgo/base";
mount(() => <List each={[1, 2]} empty="None" style={{ height: 200 }}>{(item) => <Text>{() => "row " + item()}</Text>}</List>);`)
	app.Find("text:row 2")
	if app.Exists("role:option") {
		t.Fatal("a list with no selection props has options")
	}
}
