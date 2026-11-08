import { signal, Text, View } from "nsgo";
import { Button, Meter, Progress, ProgressRing, Skeleton, SkeletonContent, Spinner } from "nsgo/ui";
import { BoolKnob, ChoiceKnob, NumberKnob, Page, Playground, Row } from "../parts";
import { toasts } from "../toast";

export function ProgressDemo() {
  const value = signal(60);
  const indeterminate = signal(false);
  const ring = signal(40);
  const spinner = signal<"ring" | "spokes">("ring");
  const spinnerSize = signal(24);
  const meter = signal<"neutral" | "accent" | "info" | "warn" | "danger" | "up" | "down">("accent");
  const loading = signal(true);
  return (
    <Page>
      <Playground
        title="Progress"
        fill
        knobs={
          <>
            <NumberKnob name="Value" value={value} min={0} max={100} step={5} />
            <BoolKnob name="Indeterminate" value={indeterminate} />
          </>
        }
      >
        <Progress value={value()} indeterminate={indeterminate()} accessibilityLabel="Uploading" />
      </Playground>
      <Playground title="ProgressRing" knobs={<NumberKnob name="Size" value={ring} min={20} max={80} step={4} />}>
        <ProgressRing value={value()} indeterminate={indeterminate()} size={ring()} accessibilityLabel="Uploading" />
      </Playground>
      <Playground
        title="Spinner"
        knobs={
          <>
            <ChoiceKnob name="Variant" value={spinner} options={["ring", "spokes"]} />
            <NumberKnob name="Size" value={spinnerSize} min={12} max={48} step={2} />
          </>
        }
      >
        <Spinner variant={spinner()} size={spinnerSize()} accessibilityLabel="Loading" />
      </Playground>
      <Playground
        title="Meter"
        fill
        knobs={<ChoiceKnob name="Variant" value={meter} options={["neutral", "accent", "info", "warn", "danger", "up", "down"]} />}
      >
        <Meter value={value() / 100} variant={meter()} marker={0.8} />
      </Playground>
      <Playground title="Skeleton" fill knobs={<BoolKnob name="Loading" value={loading} />}>
        <SkeletonContent loading={loading()} placeholder={{ lines: 3, avatar: true }}>
          <Text class="text-[13px] text-text">The content, once it has loaded.</Text>
        </SkeletonContent>
      </Playground>
    </Page>
  );
}

const types = ["plain", "info", "success", "warning", "error"] as const;

export function ToastDemo() {
  const type = signal<(typeof types)[number]>("success");
  const action = signal(false);
  let n = 0;
  const show = () => {
    n++;
    toasts()?.show({
      type: type(),
      title: `Toast ${n}`,
      description: "Saved a moment ago.",
      action: action() ? "Undo" : "",
      duration: 5000,
    });
  };
  return (
    <Page>
      <Playground
        title="Toast"
        knobs={
          <>
            <ChoiceKnob name="Type" value={type} options={types} />
            <BoolKnob name="Action" value={action} />
          </>
        }
      >
        <Row>
          <Button text="Show a toast" onPress={show} />
          <Button text="Dismiss all" variant="secondary" onPress={() => toasts()?.dismissAll()} />
        </Row>
      </Playground>
    </Page>
  );
}
