import { mount, signal, Storage, Text, View } from "nsgo";
import { Input } from "nsgo/base";

function App() {
  const count = signal(0), name = signal("");
  return <View>
    <Text testID="count" role="button" accessibilityLabel="Increment" onPress={() => count.set(count() + 1)}>{() => `Pressed ${count()}`}</Text>
    <Input testID="name" onChange={name.set} />
    <Text>{() => `Hello ${name()}`}</Text>
    <Text testID="save" onPress={() => Storage.setItem("name", name())}>Save</Text>
  </View>;
}
mount(App);
