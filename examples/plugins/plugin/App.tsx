import { host, mount, signal, Storage, Text, View } from "nsgo";

// The plugin keeps its waves in its own part of the app's storage, and
// asks the host who to greet through the host's handler "greet", the
// only one its capabilities name.
const waves = signal(0);
const greeting = signal("…");
Storage.getItem("waves").then((n) => waves.set(Number(n ?? 0)));
host.call("greet").then((g) => greeting.set(String(g)), (e) => greeting.set(e.code));

function wave() {
  waves.set(waves() + 1);
  Storage.setItem("waves", String(waves()));
}

function Greeter() {
  return (
    <View class="gap-3 p-4 rounded-xl bg-neutral-100 dark:bg-neutral-800">
      <Text testID="greeting" class="text-lg font-semibold text-neutral-900 dark:text-white">
        {greeting}
      </Text>
      <Text testID="waves" class="text-sm text-neutral-600 dark:text-neutral-300">
        {() => `Waved ${waves()} times`}
      </Text>
      <View
        testID="wave"
        class="self-start px-3 py-2 rounded-lg bg-neutral-900 dark:bg-white"
        onPress={wave}
      >
        <Text class="text-sm font-medium text-white dark:text-neutral-900">Wave</Text>
      </View>
    </View>
  );
}

mount(Greeter);
