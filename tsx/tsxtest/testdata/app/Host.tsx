import { host, mount, signal, Text, View } from "nsgo";

function App() {
  const said = signal(""), events = signal("");
  host.on("tick", (n) => events.set(String(n)));
  const ask = async (name) => {
    try {
      said.set(String(await host.call(name, { who: host.params.user })));
    } catch (e) {
      said.set("failed: " + e.message);
    }
  };
  return <View>
    <Text testID="greet" onPress={() => ask("greet")}>greet</Text>
    <Text testID="later" onPress={() => ask("later")}>later</Text>
    <Text testID="fixed" onPress={() => ask("fixed")}>fixed</Text>
    <Text testID="broken" onPress={() => ask("broken")}>broken</Text>
    <Text testID="missing" onPress={() => ask("missing")}>missing</Text>
    <Text testID="post" onPress={() => host.post("note", "hello")}>post</Text>
    <Text>{() => "said " + said()}</Text>
    <Text>{() => "event " + events()}</Text>
  </View>;
}

mount(App);
