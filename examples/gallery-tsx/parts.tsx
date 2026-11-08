// What the demos share: cards, a playground's stage, and the knobs that set
// a component's props beside it.
import { type JSX, Scroll, signal, type Signal, Text, View, windowSize } from "nsgo";
import { Select, Slider, Switch, Tabs } from "nsgo/ui";

/** The gallery's text sizes, in the look of ui's cards. */
const title = "text-[13px] font-bold text-text";
const note = "text-[12px] text-muted";

/** A demo's page: its cards in a column. */
export function Page(props: { children?: JSX.Element }) {
  return (
    <Scroll class="flex-1">
      <View class="w-full max-w-[880px] gap-5 self-center p-5">{props.children}</View>
    </Scroll>
  );
}

/** A card: a bold title with a note, above a bordered surface. */
export function Card(props: { title: string; note?: string; children?: JSX.Element; footer?: JSX.Element; flush?: boolean }) {
  return (
    <View class="gap-2" testID={`card-${props.title}`}>
      <View class="gap-0.5 px-0.5">
        <Text class={title} role="header">{props.title}</Text>
        <If when={props.note}>
          <Text class={note}>{props.note}</Text>
        </If>
      </View>
      <View class="overflow-hidden rounded-[10px] border border-border bg-surface">
        <If when={props.flush} else={<View class="gap-3 p-3.5">{props.children}</View>}>
          {props.children}
        </If>
        <If when={props.footer}>
          <View class="gap-0.5 border-t border-line px-3.5 py-2.5">{props.footer}</View>
        </If>
      </View>
    </View>
  );
}

/**
 * A card of a component above the knobs that set its props: the stage
 * centers its forms, `fill` stretches a form with no width of its own
 * across a centered column, and `start` lays what lists its results
 * downward across the stage from the top. `event`, the last thing its
 * forms reported, shows under them for what shows no change of its own.
 */
export function Playground(props: {
  title: string;
  note?: string;
  fill?: boolean;
  start?: boolean;
  knobs?: JSX.Element;
  event?: string;
  children?: JSX.Element;
}) {
  const box = () => (props.start ? "items-stretch" : props.fill ? "w-full max-w-[360px] items-stretch" : "max-w-full items-center");
  return (
    <Card title={props.title} note={props.note} flush footer={props.knobs}>
      <View testID={`stage-${props.title}`} class={`min-h-24 gap-2 p-4 ${props.start ? "items-stretch justify-start" : "items-center justify-center"}`}>
        <View testID={`form-${props.title}`} class={box()}>{props.children}</View>
      </View>
      <If when={props.event !== undefined}>
        <View class="flex-row items-center gap-2 border-t border-line px-4 py-2">
          <Text class={note}>Last event</Text>
          <Text testID="event" class="shrink font-mono text-[12px] text-text" style={{ numberOfLines: 1 }}>{props.event || "—"}</Text>
        </View>
      </If>
    </Card>
  );
}

/** A complex component's page: it fills the page, its options in a panel beside it where the pane is wide enough, under it where not. */
export function StagePage(props: { panel?: JSX.Element; children?: JSX.Element }) {
  const wide = () => windowSize().width >= 1000;
  return (
    <View class="min-h-0 min-w-0 flex-1" classList={{ "flex-row": wide() }}>
      <View testID="stage" class="m-4 min-h-[240px] min-w-0 flex-1">{props.children}</View>
      <If when={props.panel}>
        <Scroll testID="stage-panel" class={wide() ? "w-[332px] shrink-0" : "max-h-[312px] shrink-0"}>
          <View class="gap-3 p-4">{props.panel}</View>
        </Scroll>
      </If>
    </View>
  );
}

/** Variants side by side, on more rows where they do not fit. */
export function Row(props: { children?: JSX.Element }) {
  return <View class="flex-row flex-wrap items-center justify-center gap-3">{props.children}</View>;
}

/** A value the demo reads back, after its label. */
export function Status(props: { label: string; value: string; testID?: string }) {
  return (
    <View class="flex-row gap-2 py-1">
      <Text class={note}>{props.label}</Text>
      <Text testID={props.testID} class="flex-1 text-end font-mono text-[12px] text-text">{props.value}</Text>
    </View>
  );
}

/** A control under the prop it sets. */
function Knob(props: { name: string; tight?: boolean; children?: JSX.Element }) {
  return (
    <View class="min-h-7 flex-row items-center gap-3">
      <View class="min-w-[88px] shrink-0">
        <Text class="font-mono text-[12px] text-code">{props.name}</Text>
      </View>
      <View class={`min-w-0 flex-1 shrink ${props.tight ? "items-end" : "items-stretch"}`}>{props.children}</View>
    </View>
  );
}

/** Sets a boolean prop. */
export function BoolKnob(props: { name: string; value: Signal<boolean> }) {
  return (
    <Knob name={props.name} tight>
      <Switch size="small" accessibilityLabel={props.name} checked={props.value()} onChange={({ checked }) => props.value.set(checked)} />
    </Knob>
  );
}

/** Sets a prop to one of its names: small tabs while they fit, else a select. */
export function ChoiceKnob<T extends string>(props: { name: string; value: Signal<T>; options: readonly T[] }) {
  const options = () => props.options.map((o) => ({ value: o, label: o }));
  return (
    <If
      when={props.options.length <= 4}
      else={
        <Knob name={props.name}>
          <Select
            testID={`knob-${props.name}`}
            accessibilityLabel={props.name}
            options={options()}
            value={props.value()}
            onChange={({ value }) => props.value.set(value as T)}
            style={{ width: "100%" }}
          />
        </Knob>
      }
    >
      <Knob name={props.name} tight>
        <Tabs
          size="small"
          accessibilityLabel={props.name}
          options={options()}
          value={props.value()}
          onChange={({ value }) => props.value.set(value as T)}
          style={{ alignSelf: "flex-end" }}
        />
      </Knob>
    </If>
  );
}

/** Sets a number from min to max: its value, then a slider. */
export function NumberKnob(props: { name: string; value: Signal<number>; min: number; max: number; step?: number }) {
  return (
    <Knob name={props.name} tight>
      <View class="max-w-full flex-row items-center gap-2">
        <Text class="min-w-8 text-end text-[12px] text-muted tabular-nums">{String(Math.round(props.value() * 1000) / 1000)}</Text>
        <Slider
          accessibilityLabel={props.name}
          min={props.min}
          max={props.max}
          step={props.step ?? 1}
          value={props.value()}
          onChange={({ value }) => props.value.set(value)}
          style={{ width: 180, flexShrink: 1 }}
        />
      </View>
    </Knob>
  );
}

/** What a playground's forms report: `log` makes a handler that shows `text` as its `event`. */
export function useEvents() {
  const last = signal("");
  return { last, log: (text: string) => () => last.set(text) };
}
