// nsgo/ui's RichEditor over a session the page makes: a ProseMirror-style
// document with Tiptap's starter kit, a toolbar of its commands, and the
// document read back as Markdown as it changes.
import { signal, Text, View } from "nsgo";
import { command, createRichEditor, StarterKit, TableKit } from "nsgo/base";
import { IconButton, RichEditor, Toolbar, ToolbarSeparator } from "nsgo/ui";
import { StagePage, Status } from "../parts";

const start = `# Launch notes

The launch moves to **November 3**, after the final review.

- Draft the announcement
- Check the *pricing* page
- Book the room

> Ship it on a Tuesday.
`;

export function RichEditorDemo() {
  const editor = createRichEditor({ extensions: [StarterKit({ levels: [1, 2, 3] }), TableKit({})], markdown: start });
  const markdown = signal(editor.markdown());
  const words = () => markdown().split(/\s+/).filter(Boolean).length;
  const run = (name: string) => () => editor.run(command(name, {}));
  return (
    <StagePage
      panel={
        <>
          <Status label="Words" value={String(words())} testID="rich-words" />
          <Status label="Revision" value={(markdown(), String(editor.revision))} />
          <Text class="text-[12px] font-bold text-text">Markdown</Text>
          <Text testID="rich-markdown" class="select-text font-mono text-[11.5px] text-muted">{markdown()}</Text>
        </>
      }
    >
      <View class="flex-1 overflow-hidden rounded-lg border border-border bg-surface">
        <Toolbar testID="rich-toolbar" accessibilityLabel="Formatting" class="border-b border-line px-2 py-1">
          <IconButton testID="rich-bold" icon="bold" accessibilityLabel="Bold" onPress={run("toggleBold")} />
          <IconButton icon="italic" accessibilityLabel="Italic" onPress={run("toggleItalic")} />
          <IconButton icon="underline" accessibilityLabel="Underline" onPress={run("toggleUnderline")} />
          <ToolbarSeparator />
          <IconButton icon="list" accessibilityLabel="Bulleted list" onPress={run("toggleBulletList")} />
          <IconButton icon="list-ordered" accessibilityLabel="Numbered list" onPress={run("toggleOrderedList")} />
          <ToolbarSeparator />
          <IconButton icon="undo-2" accessibilityLabel="Undo" onPress={run("undo")} />
          <IconButton icon="redo-2" accessibilityLabel="Redo" onPress={run("redo")} />
        </Toolbar>
        <RichEditor testID="rich" editor={editor} class="flex-1" onChange={() => markdown.set(editor.markdown())} />
      </View>
    </StagePage>
  );
}
