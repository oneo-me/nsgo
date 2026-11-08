import { signal, Text, View } from "nsgo";
import {
  Calendar,
  Checkbox,
  ColorPicker,
  ColorWell,
  Combobox,
  DatePicker,
  Field,
  Input,
  NumberInput,
  OtpInput,
  Radio,
  RadioGroup,
  Rating,
  Select,
  Slider,
  Switch,
  TagInput,
  TimeField,
} from "nsgo/ui";
import { BoolKnob, ChoiceKnob, NumberKnob, Page, Playground, Row, Status } from "../parts";

const fruits = ["Apple", "Banana", "Cherry", "Grape", "Lemon", "Mango", "Orange", "Peach"].map((f) => ({ value: f.toLowerCase(), label: f }));

export function InputDemo() {
  const text = signal("");
  const disabled = signal(false);
  const multiline = signal(false);
  const masked = signal(false);
  const number = signal(42);
  const code = signal("");
  const tags = signal(["design", "native"]);
  return (
    <Page>
      <Playground
        title="Input"
        fill
        knobs={
          <>
            <BoolKnob name="Multiline" value={multiline} />
            <BoolKnob name="Masked" value={masked} />
            <BoolKnob name="Disabled" value={disabled} />
            <Status label="Value" value={text()} testID="input-value" />
          </>
        }
      >
        <Input
          testID="input"
          value={text()}
          onChange={({ value }) => text.set(value)}
          placeholder="Type here"
          multiline={multiline()}
          masked={masked()}
          disabled={disabled()}
        />
      </Playground>
      <Playground title="Field" note="A text box that keeps its bordered shape, for forms." fill>
        <Field placeholder="Search the docs" />
      </Playground>
      <Playground title="NumberInput" knobs={<Status label="Value" value={String(number())} />}>
        <NumberInput value={number()} onChange={({ value }) => number.set(value)} min={0} max={100} step={1} accessibilityLabel="Quantity" />
      </Playground>
      <Playground title="OtpInput" knobs={<Status label="Value" value={code()} />}>
        <OtpInput length={6} groups={2} value={code()} onChange={({ value }) => code.set(value)} accessibilityLabel="Code" />
      </Playground>
      <Playground title="TagInput" fill knobs={<Status label="Tags" value={tags().join(", ")} />}>
        <TagInput tags={tags()} onChange={({ value }) => tags.set(value)} placeholder="Add a tag" accessibilityLabel="Tags" />
      </Playground>
    </Page>
  );
}

export function CheckboxDemo() {
  const a = signal(true);
  const b = signal(false);
  const disabled = signal(false);
  const choice = signal("email");
  const on = signal(true);
  const size = signal<"small" | "default" | "large">("default");
  const all = () => (a() && b() ? true : a() || b() ? "mixed" : false);
  return (
    <Page>
      <Playground title="Checkbox" knobs={<BoolKnob name="Disabled" value={disabled} />}>
        <View class="gap-1">
          <Checkbox
            text="Select all"
            checked={all() === true}
            indeterminate={all() === "mixed"}
            disabled={disabled()}
            onChange={({ checked }) => {
              a.set(checked);
              b.set(checked);
            }}
          />
          <Checkbox text="Receipts" checked={a()} disabled={disabled()} onChange={({ checked }) => a.set(checked)} />
          <Checkbox text="Newsletter" checked={b()} disabled={disabled()} onChange={({ checked }) => b.set(checked)} />
        </View>
      </Playground>
      <Playground title="Radio">
        <RadioGroup accessibilityLabel="Contact">
          <Each of={["email", "phone", "post"]}>
            {(v) => <Radio text={v()} checked={choice() === v()} onChange={() => choice.set(v())} />}
          </Each>
        </RadioGroup>
      </Playground>
      <Playground title="Switch" knobs={<ChoiceKnob name="Size" value={size} options={["small", "default", "large"]} />}>
        <Switch checked={on()} onChange={({ checked }) => on.set(checked)} size={size()} accessibilityLabel="Enabled" />
      </Playground>
    </Page>
  );
}

export function SelectDemo() {
  const value = signal("banana");
  const searchable = signal(false);
  const clearable = signal(false);
  const picked = signal(["apple", "peach"]);
  const multiple = signal(true);
  return (
    <Page>
      <Playground
        title="Select"
        knobs={
          <>
            <BoolKnob name="Searchable" value={searchable} />
            <BoolKnob name="OnClear" value={clearable} />
            <Status label="Value" value={value()} testID="select-value" />
          </>
        }
      >
        <Select
          testID="select"
          accessibilityLabel="Fruit"
          options={fruits}
          value={value()}
          onChange={({ value: v }) => value.set(v)}
          placeholder="Choose a fruit"
          searchable={searchable()}
          searchPlaceholder="Search"
          empty="No fruit"
          onClear={clearable() ? () => value.set("") : undefined}
        />
      </Playground>
      <Playground
        title="Combobox"
        knobs={
          <>
            <BoolKnob name="Multiple" value={multiple} />
            <Status label="Value" value={picked().join(", ")} />
          </>
        }
      >
        <Combobox
          accessibilityLabel="Fruit"
          options={fruits}
          multiple={multiple()}
          value={picked()}
          onChange={({ value: v }) => picked.set(v)}
          placeholder="Search fruit"
          empty="No fruit"
        />
      </Playground>
    </Page>
  );
}

export function DatePickerDemo() {
  const date = signal<string | null>("2026-11-08");
  const span = signal<[string | null, string | null]>(["2026-11-03", "2026-11-12"]);
  const time = signal("09:30");
  const range = signal(false);
  const cleanable = signal(true);
  const months = signal(1);
  const second = signal(false);
  return (
    <Page>
      <Playground
        title="DatePicker"
        knobs={
          <>
            <BoolKnob name="Range" value={range} />
            <BoolKnob name="Cleanable" value={cleanable} />
            <Status label="Value" value={String(range() ? span() : date())} />
          </>
        }
      >
        <DatePicker
          accessibilityLabel="Date"
          range={range()}
          cleanable={cleanable()}
          value={range() ? span() : date()}
          onChange={({ value }) => {
            if (Array.isArray(value)) span.set(value);
            else date.set(value);
          }}
        />
      </Playground>
      <Playground title="Calendar" knobs={<NumberKnob name="NumberOfMonths" value={months} min={1} max={2} />}>
        <Calendar value={date()} numberOfMonths={months()} onChange={({ value }) => date.set(value as string | null)} />
      </Playground>
      <Playground
        title="TimeField"
        knobs={
          <>
            <BoolKnob name="Seconds" value={second} />
            <Status label="Value" value={time()} />
          </>
        }
      >
        <TimeField
          accessibilityLabel="Time"
          value={time()}
          precision={second() ? "second" : "minute"}
          onChange={({ value }) => time.set(value)}
        />
      </Playground>
    </Page>
  );
}

export function ColorPickerDemo() {
  const color = signal("#0a84ff");
  const text = signal("#0a84ff");
  const alpha = signal(true);
  const format = signal<"hex" | "rgb" | "hsl" | "oklch">("hex");
  const disabled = signal(false);
  return (
    <Page>
      <Playground
        title="ColorPicker"
        knobs={
          <>
            <ChoiceKnob name="Format" value={format} options={["hex", "rgb", "hsl", "oklch"]} />
            <BoolKnob name="Alpha" value={alpha} />
            <BoolKnob name="Disabled" value={disabled} />
            <Status label="Value" value={text()} testID="color-value" />
          </>
        }
      >
        <ColorPicker
          value={color()}
          format={format()}
          alpha={alpha()}
          disabled={disabled()}
          onChange={({ value, formatted }) => {
            color.set(value);
            text.set(formatted);
          }}
        />
      </Playground>
      <Playground title="ColorWell" note="A swatch that opens the picker.">
        <ColorWell accessibilityLabel="Accent" value={color()} onChange={({ value }) => color.set(value)} />
      </Playground>
    </Page>
  );
}

export function SliderDemo() {
  const value = signal(40);
  const low = signal(20);
  const high = signal(70);
  const step = signal(1);
  const disabled = signal(false);
  const rating = signal(3);
  const readonly = signal(false);
  const max = signal(5);
  return (
    <Page>
      <Playground
        title="Slider"
        fill
        knobs={
          <>
            <NumberKnob name="Step" value={step} min={1} max={20} />
            <BoolKnob name="Disabled" value={disabled} />
            <Status label="Value" value={String(value())} testID="slider-value" />
          </>
        }
      >
        <Slider
          accessibilityLabel="Volume"
          min={0}
          max={100}
          step={step()}
          disabled={disabled()}
          value={value()}
          onChange={({ value: v }) => value.set(v)}
        />
      </Playground>
      <Playground title="Range slider" fill knobs={<Status label="Range" value={`${low()} – ${high()}`} />}>
        <Slider
          accessibilityLabel="Price"
          lowAccessibilityLabel="Lowest"
          highAccessibilityLabel="Highest"
          min={0}
          max={100}
          value={low()}
          high={high()}
          onChange={({ value: v }) => low.set(v)}
          onHighChange={({ value: v }) => high.set(v)}
        />
      </Playground>
      <Playground
        title="Rating"
        knobs={
          <>
            <NumberKnob name="Max" value={max} min={3} max={10} />
            <BoolKnob name="Readonly" value={readonly} />
          </>
        }
      >
        <Rating accessibilityLabel="Rating" value={rating()} max={max()} readonly={readonly()} onChange={({ value: v }) => rating.set(v)} />
      </Playground>
    </Page>
  );
}
