import { signal, Text, View } from "nsgo";
import { ResizablePanel } from "nsgo/base";
import { Accordion, Collapsible, DockArea, DockTabLabel, ResizablePanelGroup } from "nsgo/ui";
import { BoolKnob, ChoiceKnob, NumberKnob, Page, Playground, StagePage, Status } from "../parts";

const faq = [
  { text: "What is NSGo?", body: "A native UI engine with a Go backend." },
  { text: "Which targets does it run on?", body: "macOS, iOS, Android, Windows, Linux and the web." },
  { text: "Can I write screens in TSX?", body: "Yes: this gallery is written in TSX." },
];

function Body(props: { text: string }) {
  return <Text class="pb-2 text-[13px] text-muted">{props.text}</Text>;
}

export function AccordionDemo() {
  const open = signal<number[]>([0]);
  const disabled = signal(false);
  const expanded = signal(false);
  const variant = signal<"arrowRight" | "arrowLeft">("arrowRight");
  return (
    <Page>
      <Playground title="Accordion" fill knobs={<BoolKnob name="Disabled" value={disabled} />}>
        <Accordion
          testID="accordion"
          open={(i) => open().includes(i)}
          onOpenChange={({ index, open: o }) => open.set(o ? [...open(), index] : open().filter((n) => n !== index))}
          items={faq.map((f, i) => ({ text: f.text, disabled: disabled() && i === 2, content: <Body text={f.body} /> }))}
        />
      </Playground>
      <Playground title="Collapsible" fill knobs={<ChoiceKnob name="Variant" value={variant} options={["arrowRight", "arrowLeft"]} />}>
        <Collapsible
          text="Advanced options"
          variant={variant()}
          open={expanded()}
          onOpenChange={({ open: o }) => expanded.set(o)}
          content={<Body text="What it folds away shows here." />}
        />
      </Playground>
    </Page>
  );
}

function Pane(props: { title: string; note: string; raised?: boolean }) {
  return (
    <View class={`flex-1 justify-center gap-1 p-3 ${props.raised ? "bg-raised" : "bg-surface"}`}>
      <Text class="text-[13px] font-semibold text-text">{props.title}</Text>
      <Text class="text-[12px] text-muted">{props.note}</Text>
    </View>
  );
}

export function ResizableDemo() {
  const sidebar = signal(220);
  const consoleHeight = signal(140);
  const shown = signal(true);
  const last = signal("—");
  return (
    <StagePage
      panel={
        <>
          <BoolKnob name="Visible" value={shown} />
          <NumberKnob name="Files" value={sidebar} min={120} max={360} step={10} />
          <NumberKnob name="Console" value={consoleHeight} min={60} max={300} step={10} />
          <Status label="Last resize" value={last()} testID="resize-last" />
        </>
      }
    >
      <View class="flex-1 overflow-hidden rounded-lg border border-border bg-background">
        <ResizablePanelGroup
          testID="panels"
          handleAccessibilityLabel="Resize the sidebar"
          style={{ flexGrow: 1 }}
          onResize={({ sizes }) => last.set(`row ${sizes.map(Math.round).join(", ")}`)}
        >
          <ResizablePanel size={sidebar()} minSize={120} maxSize={360} onResize={({ size }) => sidebar.set(size)}>
            <Pane title="Files" note="120 to 360 pt" raised />
          </ResizablePanel>
          <ResizablePanel minSize={160}>
            <ResizablePanelGroup
              axis="vertical"
              handleAccessibilityLabel="Resize the console"
              style={{ flexGrow: 1 }}
              onResize={({ sizes }) => last.set(`column ${sizes.map(Math.round).join(", ")}`)}
            >
              <ResizablePanel minSize={80}>
                <Pane title="Editor" note="Takes the room the others leave" />
              </ResizablePanel>
              <ResizablePanel size={consoleHeight()} minSize={60} visible={shown()} onResize={({ size }) => consoleHeight.set(size)}>
                <Pane title="Console" note="Hidden, it keeps its size" raised />
              </ResizablePanel>
            </ResizablePanelGroup>
          </ResizablePanel>
        </ResizablePanelGroup>
      </View>
    </StagePage>
  );
}

const panels: Record<string, { title: string; icon: string }> = {
  files: { title: "Files", icon: "folder" },
  readme: { title: "README.md", icon: "file-text" },
  main: { title: "main.go", icon: "code" },
  preview: { title: "Preview", icon: "eye" },
  terminal: { title: "Terminal", icon: "terminal" },
};

export function DockDemo() {
  const locked = signal(false);
  return (
    <StagePage panel={<BoolKnob name="Locked" value={locked} />}>
      <View class="flex-1 overflow-hidden rounded-lg border border-border bg-background">
        <DockArea
          testID="dock"
          class="flex-1"
          compact={false}
          locked={locked()}
          panels={Object.keys(panels).map((id) => ({ id, closable: id !== "files" }))}
          defaultLayout={{
            center: {
              type: "split",
              axis: "horizontal",
              children: [
                { type: "tabs", panels: ["readme", "main"], activeIndex: 1 },
                { type: "tabs", panels: ["preview"] },
              ],
            },
            left: { layout: { type: "tabs", panels: ["files"] }, size: 200 },
            bottom: { layout: { type: "tabs", panels: ["terminal"] }, size: 160 },
          }}
          renderTab={(tab) => <DockTabLabel tab={tab} text={panels[tab.panel].title} icon={panels[tab.panel].icon} closeAccessibilityLabel="Close" />}
          renderPanel={(panel) => <Pane title={panels[panel.id].title} note={`The panel ${panel.id}`} />}
        />
      </View>
    </StagePage>
  );
}
