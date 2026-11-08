import { signal, Text, View } from "nsgo";
import { Editor } from "nsgo/base";
import { DiffView, TextView } from "nsgo/ui";
import { BoolKnob, ChoiceKnob, NumberKnob, StagePage, Status } from "../parts";

const source = `package main

import "fmt"

// main says hello.
func main() {
	for i := range 3 {
		fmt.Println("hello", i)
	}
}
`;

const changed = `package main

import (
	"fmt"
	"os"
)

// main says hello to whoever it is named.
func main() {
	for i := range 3 {
		fmt.Println("hello", os.Args[1], i)
	}
}
`;

export function EditorDemo() {
  const value = signal(source);
  const numbers = signal(true);
  const wrap = signal(false);
  const readOnly = signal(false);
  const tab = signal(4);
  return (
    <StagePage
      panel={
        <>
          <BoolKnob name="LineNumbers" value={numbers} />
          <BoolKnob name="SoftWrap" value={wrap} />
          <BoolKnob name="ReadOnly" value={readOnly} />
          <NumberKnob name="TabSize" value={tab} min={2} max={8} step={2} />
          <Status label="Lines" value={String(value().split("\n").length)} testID="editor-lines" />
        </>
      }
    >
      <View class="flex-1 overflow-hidden rounded-lg border border-border bg-surface">
        <Editor
          testID="editor"
          class="flex-1"
          language="go"
          value={value()}
          onChange={({ value: v }) => value.set(v)}
          lineNumbers={numbers()}
          softWrap={wrap()}
          readOnly={readOnly()}
          tabSize={tab()}
        />
      </View>
    </StagePage>
  );
}

export function DiffDemo() {
  const layout = signal<"split" | "unified">("split");
  const numbers = signal(true);
  const wrap = signal(false);
  return (
    <StagePage
      panel={
        <>
          <ChoiceKnob name="Layout" value={layout} options={["split", "unified"]} />
          <BoolKnob name="LineNumbers" value={numbers} />
          <BoolKnob name="SoftWrap" value={wrap} />
        </>
      }
    >
      <View class="flex-1 overflow-hidden rounded-lg border border-border bg-surface">
        <DiffView
          testID="diff"
          class="flex-1"
          oldValue={source}
          newValue={changed}
          language="go"
          layout={layout()}
          lineNumbers={numbers()}
          softWrap={wrap()}
        />
      </View>
    </StagePage>
  );
}

const markdown = `# TextView

Markdown and HTML as **native** text, with _emphasis_, \`code\` and [links](https://github.com/oneo-me/nsgo).

## Lists

- Paragraphs and headings
- Tables and code blocks
- [x] Tasks

| Target | Backend |
| --- | --- |
| macOS | AppKit |
| Web | Canvas |

\`\`\`go
fmt.Println("hello")
\`\`\`

> A quotation, for good measure.
`;

export function TextViewDemo() {
  const selectable = signal(true);
  return (
    <StagePage panel={<BoolKnob name="Selectable" value={selectable} />}>
      <View class="flex-1 overflow-hidden rounded-lg border border-border bg-surface p-4">
        <TextView testID="textview" class="flex-1" markdown={markdown} selectable={selectable()} />
      </View>
    </StagePage>
  );
}
