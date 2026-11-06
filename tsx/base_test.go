package tsx_test

import (
	"strings"
	"testing"
	"time"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/nsgotest"
	"github.com/oneo-me/nsgo/tsx"
	_ "github.com/oneo-me/nsgo/tsx/textview"
)

// Badge is an app's own Go component, registered for TSX.
type Badge struct {
	Count   func() int
	OnClear func(left int)
	Class   string
	TestID  string
}

func (b Badge) Render(cx *core.Scope) core.Node {
	return core.View{Class: b.Class, TestID: b.TestID, OnPress: func(core.PressEvent) {
		if b.OnClear != nil {
			b.OnClear(b.Count())
		}
	}, Children: []core.Node{core.Text{ContentFunc: func() string { return "badge " + itoa(b.Count()) }}}}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for ; n > 0; n /= 10 {
		s = string(rune('0'+n%10)) + s
	}
	return s
}

func init() {
	tsx.Register("app/native", map[string]core.Node{"Badge": Badge{}}, tsx.Event("OnClear", "left"))
}

func TestAppComponents(t *testing.T) {
	app := run(t, `import { mount, signal, Text } from "nsgo";
import { Badge } from "app/native";
function App() {
	const n = signal(3), cleared = signal(-1);
	return <>
		<Badge testID="b" count={n()} onClear={({ left }) => { cleared.set(left); n.set(0); }} />
		<Text>{() => "cleared " + cleared()}</Text>
	</>;
}
mount(App);`)
	if !app.Exists("text:badge 3") {
		t.Fatalf("texts %q", app.Texts())
	}
	app.Press("id:b")
	for _, want := range []string{"badge 0", "cleared 3"} {
		if !app.Exists("text:" + want) {
			t.Errorf("no %q among %q", want, app.Texts())
		}
	}
}

// A Go component's events pass one object, its fields named by
// tsx.Event, the declarations saying so.
func TestEventsPassOneObject(t *testing.T) {
	decls := tsx.Declarations()["types/nsgo/base.d.ts"]
	for _, want := range []string{
		"onChange?: (e: { checked: boolean }) => void",
		"onOpenChange?: (e: { open: boolean; reason: DialogChangeReason }) => void",
		"onColumnResize?: (e: { key: string; width: number }) => void",
		"onVisibleRowsChange?: (e: { range: DataTableRange }) => void",
		"onLayout?: (e: { bounds: Bounds }) => void",
	} {
		if !strings.Contains(decls, want) {
			t.Errorf("no %q in nsgo/base's declarations", want)
		}
	}
	defer func() {
		if recover() == nil {
			t.Error("an event of several unnamed arguments registers")
		}
	}()
	tsx.Register("app/unnamed", map[string]core.Node{"Bad": badEvent{}})
}

// badEvent has an event tsx.Event does not name.
type badEvent struct {
	OnPair func(a, b int)
}

func (badEvent) Render(*core.Scope) core.Node { return core.View{} }

func TestBaseControls(t *testing.T) {
	app := run(t, `import { mount, signal, Text, View } from "nsgo";
import { Button, Checkbox, Switch, Tabs, Tab, Progress } from "nsgo/base";
function App() {
	const on = signal(false), checked = signal(false), tab = signal(0), presses = signal(0);
	return <View>
		<Button testID="button" onPress={() => presses.set(presses() + 1)}><Text>Press</Text></Button>
		<Checkbox testID="check" checked={checked()} onChange={(e) => checked.set(e.checked)} indicator={<Text>✓</Text>}><Text>Agree</Text></Checkbox>
		<Switch testID="switch" checked={on()} onChange={(e) => on.set(e.checked)} trackClass="w-10 h-6" thumbClass="size-5" />
		<Tabs>
			<Tab testID="t0" selected={tab() === 0} onPress={() => tab.set(0)}><Text>One</Text></Tab>
			<Tab testID="t1" selected={tab() === 1} onPress={() => tab.set(1)}><Text>Two</Text></Tab>
		</Tabs>
		<Progress testID="progress" value={40} />
		<Text>{() => [presses(), checked(), on(), tab()].join(" ")}</Text>
	</View>;
}
mount(App);`)
	app.Press("id:button")
	app.Press("id:check")
	app.Press("id:switch")
	app.Press("id:t1")
	if !app.Exists("text:1 true true 1") {
		t.Errorf("after the presses %q", app.Texts())
	}
	if v := app.Find("id:check"); v.Props.Role != driver.RoleCheckbox {
		t.Errorf("the checkbox's role is %v", v.Props.Role)
	}
}

func TestNavStackPushesAndPops(t *testing.T) {
	app := run(t, `import { mount, Text } from "nsgo";
import { createNavStack, NavStack, NavPage } from "nsgo/base";
const nav = createNavStack(["home"]);
function App() {
	return <NavStack state={nav}>
		{(route, index) => <NavPage title={route} testID={"page-" + route}>
			<Text testID={"open-" + route} onPress={() => nav.push(route + "+")}>{() => route + " at " + index()}</Text>
		</NavPage>}
	</NavStack>;
}
mount(App);`)
	app.Press("id:open-home")
	app.Settle()
	if !app.Exists("text:home+ at 1") {
		t.Fatalf("after a push %q", app.Texts())
	}
	app.Back()
	app.Settle()
	if app.Exists("text:home+ at 1") {
		t.Errorf("after back %q", app.Texts())
	}
}

func TestResizablePanelsAndPopovers(t *testing.T) {
	app := run(t, `import { mount, signal, Text, View } from "nsgo";
import { ResizablePanelGroup, ResizablePanel, Popover } from "nsgo/base";
function App() {
	const open = signal(false);
	return <View style={{ flex: 1 }}>
		<ResizablePanelGroup testID="group" axis="horizontal" style={{ height: 100 }}>
			<ResizablePanel size={100}><Text>Left</Text></ResizablePanel>
			<ResizablePanel><Text>Right</Text></ResizablePanel>
		</ResizablePanelGroup>
		<Popover open={open()} onOpenChange={(e) => open.set(e.open)} trigger={<Text testID="trigger">Open</Text>} popupClass="p-2">
			<Text>Popped</Text>
		</Popover>
	</View>;
}
mount(App);`)
	if !app.Exists("text:Left") || !app.Exists("text:Right") {
		t.Fatalf("the panels show %q", app.Texts())
	}
	if w := app.Find("text:Left").Frame.Size.Width; w > 100 {
		t.Errorf("the left panel is %v wide", w)
	}
	app.Press("id:trigger")
	app.WaitFor("text:Popped", time.Second)
}

func TestDataTableAndPlot(t *testing.T) {
	app := run(t, `import { mount, Text, View } from "nsgo";
import { DataTable } from "nsgo/base";
import { Plot, Line } from "nsgo/base/plot";
const rows = [{ name: "Ada", age: 36 }, { name: "Linus", age: 28 }];
mount(() => <View style={{ flex: 1 }}>
	<DataTable testID="table" style={{ height: 200 }} data={rows}
		columns={[{ key: "name", title: "Name", width: 100 }, { key: "age", title: "Age", width: 60 }]} />
	<Plot testID="plot" data={rows} x="name" y="age" style={{ height: 100 }}>
		<Line class="stroke-blue-500" />
	</Plot>
</View>);`)
	for _, want := range []string{"Name", "Age", "Ada", "36", "Linus", "28"} {
		if !app.Exists("text:" + want) {
			t.Errorf("no %q among %q", want, app.Texts())
		}
	}
	if !app.Exists("id:plot") {
		t.Errorf("no plot:\n%s", app.Dump())
	}
}

func TestAlertDialogAndToasts(t *testing.T) {
	app := run(t, `import { mount, signal, Text } from "nsgo";
import { AlertDialog, ToastManager } from "nsgo/base";
const toasts = new ToastManager();
function App() {
	const chose = signal("");
	return <>
		<Text testID="ask" onPress={async () => chose.set(String(await AlertDialog.show({ title: "Delete?",
			actions: [{ label: "Cancel", role: "cancel" }, { label: "Delete", role: "destructive", id: "del" }] })))}>Ask</Text>
		<Text testID="toast" onPress={() => { toasts.push("a", "Saved"); chose.set(String(toasts.len())); }}>Toast</Text>
		<Text>{() => "chose " + chose()}</Text>
	</>;
}
mount(App);`)
	app.Press("id:ask")
	if a := app.Alerts(); len(a) != 1 || a[0].Title != "Delete?" {
		t.Fatalf("alerts %+v", a)
	}
	app.Answer(1)
	app.WaitFor("text:chose del", time.Second)
	app.Press("id:toast")
	if !app.Exists("text:chose 1") {
		t.Errorf("after a toast %q", app.Texts())
	}
}

func TestTextViewRendersMarkdown(t *testing.T) {
	app := run(t, `import { mount } from "nsgo";
import { TextView } from "nsgo/base";
mount(() => <TextView markdown={"# Title\n\nSome **bold** text."} classes={{ heading1: "text-2xl" }} />);`)
	if !app.Exists("text:Title") {
		t.Errorf("the document shows %q", app.Texts())
	}
}

// TestDockAreaTakesNSHsShapes gives a DockArea NSH's gallery's layout,
// panels and render functions, and reads back what it reports in them.
func TestDockAreaTakesNSHsShapes(t *testing.T) {
	app := run(t, `import { mount, signal, Text, View } from "nsgo";
import { DockArea, Tabs, Tab, Button } from "nsgo/base";
function App() {
	const layout = signal({
		center: { type: "split", axis: "horizontal", children: [
			{ type: "tabs", panels: ["readme", "main"], activeIndex: 1 },
			{ type: "tabs", panels: ["preview"] },
		], sizes: [300, null] },
		left: { layout: { type: "tabs", panels: ["files"] }, size: 200 },
	});
	const report = signal(""), zoomed = signal("none"), width = signal(0);
	let area;
	return <View style={{ width: 1000, height: 600 }}>
		<Text>{() => "report " + report()}</Text>
		<Text>{() => "zoomed " + zoomed()}</Text>
		<Text>{() => "tabs " + width()}</Text>
		<Tabs testID="tabs" onLayout={({ bounds }) => width.set(bounds.size.width)} style={{ width: 120, height: 10 }} />
		<Button testID="zoom" onPress={async () => area.toggleZoom(await area.groupOf("main"))}><Text>Zoom</Text></Button>
		<Button testID="unzoom" onPress={() => area.toggleZoom(null)}><Text>Unzoom</Text></Button>
		<DockArea ref={(r) => (area = r)} style={{ flex: 1 }} layout={layout()}
			onLayoutChange={({ layout: next }) => {
				layout.set(next);
				const c = next.center, first = c.children[0];
				report.set([c.type, c.axis, c.sizes[0], c.sizes[1], first.type, first.activeIndex, typeof first.id,
					next.left.layout.panels[0], next.left.open, next.left.collapsible].join(","));
			}}
			onZoomChange={({ panel }) => zoomed.set(String(panel))}
			panels={[{ id: "preview", closable: false }]}
			renderTab={(tab) => <Text testID={"tab-" + tab.panel}>{tab.panel + tab.index + (tab.active ? "*" : "") + (tab.closable ? "x" : "")}</Text>}
			renderPanel={(panel) => <Text>{"panel " + panel.id}</Text>} />
	</View>;
}
mount(App);`, nsgotest.Size(1000, 700))
	for _, want := range []string{"readme0x", "main1*x", "preview0*", "files0*", "panel main", "panel preview", "tabs 120"} {
		if !app.Exists("text:" + want) {
			t.Errorf("no %q among %q", want, app.Texts())
		}
	}
	app.Press("text:readme0x")
	for _, want := range []string{"readme0*x", "main1x", "report split,horizontal,300,,tabs,0,number,files,true,true"} {
		if !app.Exists("text:" + want) {
			t.Errorf("after the press, no %q among %q", want, app.Texts())
		}
	}
	app.Press("id:zoom")
	// The group shows readme since its press.
	app.WaitFor("text:zoomed readme", time.Second)
	app.Press("id:unzoom")
	app.WaitFor("text:zoomed null", time.Second)
}
