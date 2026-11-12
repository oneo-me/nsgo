package richeditor_test

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
	"github.com/nsgo-dev/nsgo/tsx/compiler"
	_ "github.com/nsgo-dev/nsgo/tsx/richeditor"
	_ "github.com/nsgo-dev/nsgo/tsx/ui/richeditor"
)

func run(t *testing.T, source string) *nsgotest.App {
	t.Helper()
	fsys := fstest.MapFS{"App.tsx": {Data: []byte(source)}}
	return nsgotest.Start(t, compiler.App(fsys, "App.tsx"), nsgotest.Environment(core.Environment{OS: core.OSMacOS, Idiom: core.IdiomDesktop}))
}

func TestSessionsRunCommandsAsData(t *testing.T) {
	app := run(t, `import { mount, Text } from "nsgo";
import { createRichEditor, StarterKit, command, chain, toggleMark, setTextSelection, insertText, selectAll } from "nsgo/base";
const out = [];
const editor = createRichEditor({ extensions: [StarterKit({ levels: [1, 2] })], markdown: "hello" });
out.push(editor.size, editor.revision);
editor.run(setTextSelection(6));
out.push(editor.run(chain(insertText(" world"), selectAll(), toggleMark("bold"))), editor.markdown().trim());
out.push(editor.isActive("bold"), editor.can(command("toggleItalic")), editor.undoDepth());
editor.run(command("undo"));
out.push(editor.markdown().trim(), editor.selection.type);
const fail = (f) => { try { f(); return "no throw"; } catch (e) { return e.constructor.name + ": " + e.message; } };
out.push(fail(() => editor.run(command("nope"))), fail(() => editor.coordsAtPos(99)), String(editor.coordsAtPos(1)));
editor.setContent({ type: "doc", content: [{ type: "heading", attrs: { level: 2 }, content: [{ type: "text", text: "T" }] }] });
const attrs = editor.getAttributes("heading");
out.push(editor.markdown().trim(), attrs.level + "," + attrs.textAlign);
mount(() => <Text testID="out">{out.join(" | ")}</Text>);`)
	want := "7 | 0 | true | **hello world** | true | true | 1 | hello | text | " +
		`TypeError: the editor has no command "nope" | RangeError: position 99 outside 0..7 | null | ## T | 2,null`
	if got := app.Find("id:out").Text; got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

func TestDefineExtensionAndPlugins(t *testing.T) {
	app := run(t, `import { mount, signal, Text, View } from "nsgo";
import { createRichEditor, defineExtension, StarterKit, RichEditor, command, wrapIn, insertText } from "nsgo/base";
const Callout = defineExtension({
	name: "callout",
	addOptions: () => ({ tone: "info" }),
	node: { content: "paragraph+", group: "block", defining: true, attrs: { tone: { default: "info" } } },
	addKeyboardShortcuts: () => ({ "Mod-Shift-c": command("setCallout") }),
	addCommands: (options) => ({ setCallout: () => wrapIn("callout", { tone: options.tone }) }),
	addInputRules: () => [{ find: /^!!\s$/, type: "wrapping" }, { find: /->$/, type: "text", replace: "→" }],
});
const keys = [];
const updates = signal(0);
const editor = createRichEditor({
	extensions: [StarterKit({}), Callout.configure({ tone: "warn" })],
	markdown: "one",
	plugins: [{
		key: "spy",
		keys: { "Mod-j": insertText("J") },
		handleKeyDown: (ed, key) => { keys.push(key); return key === "Mod-k"; },
		decorations: (ed) => [{ from: 1, to: 2, class: "underline" }, { from: 2, to: 2, key: "w", widget: () => <Text testID="widget">*</Text> }],
	}],
});
editor.on("update", () => updates.set(updates() + 1));
const out = [Callout.name, Callout.options.tone, Callout.configure({ tone: "warn" }).options.tone, editor.can(command("setCallout"))];
function App() {
	return <View>
		<RichEditor testID="rich" editor={editor} style={{ width: 400, height: 200 }} classes={{ content: "p-2" }} />
		<Text testID="updates">{() => "updates " + updates()}</Text>
		<Text testID="out">{out.join(" | ")}</Text>
		<View testID="check" onPress={() => result.set(check())} style={{ width: 20, height: 20 }} />
		<Text testID="result">{() => result()}</Text>
	</View>;
}
const result = signal("");
const check = () => {
	editor.run(command("setCallout"));
	return [editor.docJSON().content[0].type, editor.docJSON().content[0].attrs.tone, keys.join(","), editor.text().includes("J")].join(" ");
};
mount(App);`)
	if got := app.Find("id:out").Text; got != "callout | info | warn | true" {
		t.Errorf("out %q", got)
	}
	app.Find("id:widget")
	app.Press("id:rich-editor-content")
	app.Type("role:textbox", " ->")
	if !app.Exists("text:updates 3") {
		t.Errorf("updates after typing: %q", app.Texts())
	}
	if !strings.Contains(strings.Join(app.Texts(), "\n"), "→") {
		t.Errorf("the text rule did not run: %q", app.Texts())
	}
	app.Key("mod+j")
	app.Key("mod+k")
	app.Press("id:check")
	if got := app.Find("id:result").Text; got != "callout warn Mod-k true" {
		t.Errorf("check %q", got)
	}
}

func TestStyledRichEditor(t *testing.T) {
	app := run(t, `import { mount } from "nsgo";
import { createRichEditor } from "nsgo/base";
import { RichEditor } from "nsgo/ui";
const editor = createRichEditor({ markdown: "# Title\n\nbody", editable: false });
mount(() => <RichEditor editor={editor} style={{ width: 400, height: 200 }} />);`)
	if h := app.Find("text:Title").Frame.Size.Height; h < 30 {
		t.Fatalf("the heading is %v tall, without ui's look", h)
	}
}
