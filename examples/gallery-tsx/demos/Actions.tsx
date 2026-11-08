import { signal, Text, View } from "nsgo";
import {
  Button,
  HoldToConfirm,
  IconButton,
  Link,
  ToggleButton,
  ToggleGroup,
  Toolbar,
  ToolbarGroup,
  ToolbarSeparator,
} from "nsgo/ui";
import { BoolKnob, ChoiceKnob, Events, NumberKnob, Page, Playground, Row, useEvents } from "../parts";

const buttonVariants = ["primary", "secondary", "accent", "ghost", "plain"] as const;
const sizes = ["small", "default", "large"] as const;

function ButtonCard(props: { log: (text: string) => () => void }) {
  const disabled = signal(false);
  const size = signal<(typeof sizes)[number]>("default");
  const icon = signal(true);
  return (
    <Playground
      title="Button"
      knobs={
        <>
          <ChoiceKnob name="Size" value={size} options={sizes} />
          <BoolKnob name="Icon" value={icon} />
          <BoolKnob name="Disabled" value={disabled} />
        </>
      }
    >
      <Row>
        <Each of={buttonVariants}>
          {(v) => (
            <Button
              text={v()}
              variant={v()}
              size={size()}
              disabled={disabled()}
              icon={icon() ? "plus" : undefined}
              onPress={props.log(`Pressed ${v()}`)}
            />
          )}
        </Each>
      </Row>
    </Playground>
  );
}

function IconButtonCard(props: { log: (text: string) => () => void }) {
  const size = signal(28);
  const selected = signal(false);
  const disabled = signal(false);
  return (
    <Playground
      title="IconButton"
      knobs={
        <>
          <NumberKnob name="Size" value={size} min={20} max={48} step={2} />
          <BoolKnob name="Selected" value={selected} />
          <BoolKnob name="Disabled" value={disabled} />
        </>
      }
    >
      <Row>
        <Each of={["plain", "primary", "raised"] as const}>
          {(v) => (
            <IconButton
              icon="star"
              accessibilityLabel={v()}
              variant={v()}
              size={size()}
              selected={selected()}
              disabled={disabled()}
              onPress={props.log(`Pressed ${v()}`)}
            />
          )}
        </Each>
      </Row>
    </Playground>
  );
}

function ToggleCard() {
  const bold = signal(true);
  const italic = signal(false);
  const underline = signal(false);
  const pin = signal(false);
  return (
    <Playground title="ToggleButton and ToggleGroup">
      <View class="items-center gap-3">
        <ToggleButton text="Pin" icon="pin" on={pin()} onChange={({ on }) => pin.set(on)} />
        <ToggleGroup accessibilityLabel="Text style">
          <ToggleButton icon="bold" accessibilityLabel="Bold" on={bold()} onChange={({ on }) => bold.set(on)} />
          <ToggleButton icon="italic" accessibilityLabel="Italic" on={italic()} onChange={({ on }) => italic.set(on)} />
          <ToggleButton icon="underline" accessibilityLabel="Underline" on={underline()} onChange={({ on }) => underline.set(on)} />
        </ToggleGroup>
      </View>
    </Playground>
  );
}

function HoldCard(props: { log: (text: string) => () => void }) {
  const variant = signal<"accent" | "danger" | "neutral">("danger");
  const seconds = signal(1);
  return (
    <Playground
      title="HoldToConfirm"
      note="Press and hold until the fill reaches the end."
      knobs={
        <>
          <ChoiceKnob name="Variant" value={variant} options={["accent", "danger", "neutral"]} />
          <NumberKnob name="Duration" value={seconds} min={0.5} max={3} step={0.5} />
        </>
      }
    >
      <HoldToConfirm
        text="Delete project"
        confirmedText="Deleted"
        icon="trash"
        variant={variant()}
        duration={seconds() * 1000}
        onConfirm={props.log("Confirmed")}
      />
    </Playground>
  );
}

export function ButtonDemo() {
  const { last, log } = useEvents();
  return (
    <Events value={last()}>
      <ButtonCard log={log} />
      <IconButtonCard log={log} />
      <ToggleCard />
      <HoldCard log={log} />
    </Events>
  );
}

export function LinkDemo() {
  const { last, log } = useEvents();
  const disabled = signal(false);
  return (
    <Events value={last()}>
      <Playground title="Link" knobs={<BoolKnob name="Disabled" value={disabled} />}>
        <View class="flex-row flex-wrap items-center justify-center gap-1">
          <Text class="text-[13px] text-text">Read the</Text>
          <Link text="documentation" disabled={disabled()} onPress={log("Pressed the link")} />
          <Text class="text-[13px] text-text">first.</Text>
        </View>
      </Playground>
    </Events>
  );
}

export function ToolbarDemo() {
  const { last, log } = useEvents();
  const axis = signal<"horizontal" | "vertical">("horizontal");
  const disabled = signal(false);
  return (
    <Events value={last()}>
      <Playground
        title="Toolbar"
        knobs={
          <>
            <ChoiceKnob name="Axis" value={axis} options={["horizontal", "vertical"]} />
            <BoolKnob name="Disabled" value={disabled} />
          </>
        }
      >
        <Toolbar axis={axis()} disabled={disabled()} accessibilityLabel="Formatting">
          <ToolbarGroup>
            <IconButton icon="undo-2" accessibilityLabel="Undo" onPress={log("Undo")} />
            <IconButton icon="redo-2" accessibilityLabel="Redo" onPress={log("Redo")} />
          </ToolbarGroup>
          <ToolbarSeparator />
          <ToolbarGroup>
            <IconButton icon="bold" accessibilityLabel="Bold" onPress={log("Bold")} />
            <IconButton icon="italic" accessibilityLabel="Italic" onPress={log("Italic")} />
          </ToolbarGroup>
          <ToolbarSeparator />
          <Button text="Share" variant="secondary" onPress={log("Share")} />
        </Toolbar>
      </Playground>
    </Events>
  );
}
