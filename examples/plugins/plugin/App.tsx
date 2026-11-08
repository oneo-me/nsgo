import { mount, signal, Text, View } from "nsgo";

const waves = signal(0);

function Greeter() {
  return (
    <View class="gap-3 p-4 rounded-xl bg-neutral-100 dark:bg-neutral-800">
      <Text class="text-lg font-semibold text-neutral-900 dark:text-white">Hello from a signed package</Text>
      <Text testID="waves" class="text-sm text-neutral-600 dark:text-neutral-300">
        {() => `Waved ${waves()} times`}
      </Text>
      <View
        testID="wave"
        class="self-start px-3 py-2 rounded-lg bg-neutral-900 dark:bg-white"
        onPress={() => waves.set(waves() + 1)}
      >
        <Text class="text-sm font-medium text-white dark:text-neutral-900">Wave</Text>
      </View>
    </View>
  );
}

mount(Greeter);
