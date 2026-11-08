import { colorScheme, mount, setColorScheme, signal, Storage, Text, View, windowSize } from "nsgo";
import { createNavStack, NavPage, NavStack } from "nsgo/base";
import { IconButton, NavHeader, Sidebar, SidebarShell, Spacer, Toasts } from "nsgo/ui";
import { type Demo, demos, groups } from "./demos";
import { toasts } from "./toast";

const lastDemo = "lastDemo";

function find(name: string | null | undefined): Demo {
  return demos.find((d) => d.name === name) ?? demos[0];
}

const selected = signal(demos[0].name);
Storage.getItem(lastDemo).then((name) => selected.set(find(name).name));

function open(name: string) {
  selected.set(name);
  Storage.setItem(lastDemo, name);
}

function ThemeToggle() {
  const dark = () => colorScheme() === "dark";
  return (
    <IconButton
      testID="appearance"
      icon={dark() ? "sun" : "moon"}
      accessibilityLabel="Toggle the appearance"
      onPress={() => setColorScheme(dark() ? "light" : "dark")}
    />
  );
}

/** The selected demo, made anew as another is chosen. */
function DemoPane() {
  return (
    <Each of={[selected()]} key={(n) => n}>
      {(name) => {
        const Screen = find(name()).screen;
        return (
          <View testID={`demo-${name()}`} class="min-h-0 min-w-0 flex-1 bg-background">
            <Screen />
          </View>
        );
      }}
    </Each>
  );
}

function Wide() {
  return (
    <SidebarShell
      testID="shell"
      toggleAccessibilityLabel="Toggle the sidebar"
      resizeAccessibilityLabel="Resize the sidebar"
      defaultWidth={232}
      minWidth={212}
      maxWidth={360}
      sidebar={[
        <Sidebar
          testID="sidebar"
          selected={selected()}
          onSelect={({ id }) => open(id)}
          sections={groups.map((g) => ({
            title: g.name,
            collapsible: true,
            items: g.demos.map((d) => ({ id: d.name, text: d.name, icon: d.icon, testID: `nav-${d.name}` })),
          }))}
        />,
      ]}
      sidebarFooter={[<Spacer />, <ThemeToggle />]}
      header={[<Text testID="title" role="header" class="text-[13px] font-bold text-text">{selected()}</Text>]}
      content={[<DemoPane />]}
    />
  );
}

type Route = { demo?: string };
const nav = createNavStack<Route>([{}]);

function Index() {
  return (
    <View class="flex-1 bg-background">
      <Sidebar
        testID="sidebar"
        onSelect={({ id }) => {
          open(id);
          nav.push({ demo: id });
        }}
        sections={groups.map((g) => ({
          title: g.name,
          items: g.demos.map((d) => ({ id: d.name, text: d.name, icon: d.icon, testID: `nav-${d.name}` })),
        }))}
      />
    </View>
  );
}

/** The narrow layout: the demos' index, each pushed over it. */
function Narrow() {
  return (
    <NavStack state={nav} class="flex-1">
      {(route) =>
        route.demo ? (
          <NavPage
            title={route.demo}
            testID={`screen-${route.demo}`}
            header={<NavHeader title={route.demo} onBack={() => nav.pop()} backAccessibilityLabel="Back" />}
          >
            <DemoPane />
          </NavPage>
        ) : (
          <NavPage title="Gallery" testID="index" header={<NavHeader title="Gallery" trailing={[<ThemeToggle />]} />}>
            <Index />
          </NavPage>
        )
      }
    </NavStack>
  );
}

function App() {
  return (
    <View class="flex-1 bg-background text-text">
      <If when={windowSize().width >= 760} else={<Narrow />}>
        <Wide />
      </If>
      <Toasts ref={toasts.set} testID="toasts" placement="bottomEnd" closeAccessibilityLabel="Close" />
    </View>
  );
}

mount(App);
