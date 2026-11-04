// The focus timer, which App.tsx imports with import(): the web loads it
// as a chunk of its own when the page first shows it.
import { computed, effect, signal, Text, View } from "nsgo";
import { FocusLog } from "app/native";
import { Button, Card } from "./components";

const SESSION = 25 * 60;

export function Focus() {
  const left = signal(SESSION);
  const running = signal(false);
  // Finished and reset sessions go to a log kept in Go.
  const log = new FocusLog();
  const logged = signal(log.summary);
  const record = () => {
    log.record(SESSION - left());
    logged.set(log.summary);
  };
  // A second at a time while running; the interval goes when it stops.
  effect(() => {
    if (!running()) return;
    const id = setInterval(() => {
      left.update((s) => s - 1);
      if (left() === 0) {
        running.set(false);
        record();
      }
    }, 1000);
    return () => clearInterval(id);
  });
  const two = (n: number) => String(n).padStart(2, "0");
  const progress = computed(() => 1 - left() / SESSION);
  const state = () => (running() ? "Running" : left() === SESSION ? "Ready" : "Paused");
  return (
    <Card title="Focus" subtitle={`A 25 minute session · ${logged()}`}>
      <View class="items-center gap-2 py-2">
        <Text testID="clock" class="text-6xl font-bold tracking-tight text-zinc-900 dark:text-white">
          {two(Math.floor(left() / 60))}:{two(left() % 60)}
        </Text>
        <View class="flex-row items-center gap-2">
          <View
            class="size-2 rounded-full"
            classList={{ "animate-pulse bg-emerald-500": running(), "bg-zinc-300 dark:bg-zinc-600": !running() }}
          />
          <Text class="text-xs font-medium uppercase tracking-widest text-zinc-500">{state()}</Text>
        </View>
      </View>
      <View class="h-2 overflow-hidden rounded-full bg-zinc-100 dark:bg-zinc-800">
        <View class="h-full rounded-full bg-linear-to-r from-indigo-500 to-fuchsia-500" style={{ width: `${progress() * 100}%` }} />
      </View>
      <View class="flex-row gap-2">
        <View class="flex-1">
          <Button label={running() ? "Pause" : "Start"} onPress={() => running.set(!running())} />
        </View>
        <View class="flex-1">
          <Button
            label="Reset"
            variant="ghost"
            disabled={left() === SESSION}
            onPress={() => {
              running.set(false);
              record();
              left.set(SESSION);
            }}
          />
        </View>
      </View>
    </Card>
  );
}
