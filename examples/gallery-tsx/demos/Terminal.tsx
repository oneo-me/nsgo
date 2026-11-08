// nsgo/base's Terminal runs the user's shell with Ghostty's emulator, or on
// phones, which run no local programs, the gallery's demo session standing
// in for a remote one. The panel sets what the view and the session take
// and reads back what the shell reports; the context menu around it calls
// the terminal's ref. Where no terminal runs (the web) the page says why.
import { effect, platform, signal, Text, untracked, View } from "nsgo";
import { Terminal, type TerminalRef } from "nsgo/base";
import { Button, ContextMenu, MenuItem } from "nsgo/ui";
import { demoSession } from "gallery/native";
import { BoolKnob, ChoiceKnob, NumberKnob, StagePage, Status } from "../parts";

const themes = ["Automatic", "Catppuccin Mocha", "Catppuccin Latte", "Dracula", "Nord", "GitHub Light Default"] as const;
const cursors = ["block", "bar", "underline"] as const;
const fonts = ["Monospace", "Menlo", "Courier New"] as const;

export function TerminalDemo() {
  const phone = platform.os === "ios" || platform.os === "android";
  const session = signal<"Local shell" | "Demo session">(phone ? "Demo session" : "Local shell");
  const theme = signal<(typeof themes)[number]>("Automatic");
  const cursor = signal<(typeof cursors)[number]>("block");
  const font = signal<(typeof fonts)[number]>("Monospace");
  const size = signal(13);
  const padX = signal(6);
  const padY = signal(4);
  const keyBar = signal(true);
  const title = signal("—");
  const dir = signal("—");
  const cells = signal("—");
  const generation = signal(0);
  let ref: TerminalRef | undefined;

  // The cursor's shape changes in the running session.
  effect(() => {
    const c = cursor();
    untracked(() => ref?.setCursor(c));
  });
  const restart = () => {
    title.set("—");
    dir.set("—");
    generation.set(generation() + 1);
  };
  return (
    <StagePage
      panel={
        <>
          <If when={!phone}>
            <ChoiceKnob name="Conn" value={session} options={["Local shell", "Demo session"]} />
          </If>
          <ChoiceKnob name="Theme" value={theme} options={themes} />
          <ChoiceKnob name="FontFamily" value={font} options={fonts} />
          <NumberKnob name="FontSize" value={size} min={8} max={32} step={1} />
          <NumberKnob name="PaddingX" value={padX} min={0} max={40} step={1} />
          <NumberKnob name="PaddingY" value={padY} min={0} max={40} step={1} />
          <ChoiceKnob name="Cursor" value={cursor} options={cursors} />
          <BoolKnob name="KeyBar" value={keyBar} />
          <Status label="Title" value={title()} testID="terminal-title" />
          <Status label="Size" value={cells()} testID="terminal-size" />
          <Status label="Directory" value={dir()} testID="terminal-dir" />
          <Button testID="terminal-restart" text="Restart" variant="secondary" onPress={restart} />
        </>
      }
    >
      <View class="flex-1 overflow-hidden rounded-lg border border-border bg-surface">
        <Each of={[`${generation()} ${session()}`]} key={(k: string) => k}>
          {() => (
            <ContextMenu testID="terminal-menu" style={{ flex: 1 }} trigger={
              <Terminal
                testID="terminal"
                class="flex-1"
                accessibilityLabel="Terminal"
                autoFocus
                ref={(r: TerminalRef) => (ref = r)}
                conn={session() === "Demo session" ? demoSession() : undefined}
                cursor={untracked(cursor)}
                theme={theme() === "Automatic" ? "" : theme()}
                fontFamily={font() === "Monospace" ? "" : font()}
                fontSize={size()}
                paddingX={padX()}
                paddingY={padY()}
                noKeyBar={!keyBar()}
                keyBarClass="border-t border-line bg-raised px-1.5 py-1.5"
                keyClass="mr-1.5 h-[34px] min-w-10 items-center justify-center rounded-md border border-border bg-surface px-2.5 text-[15px] text-text active:bg-inset"
                keyOnClass="border-accent-fill bg-accent-fill text-on-accent"
                onTitle={({ title: t }) => {
                  title.set(t);
                  ref?.dir().then((d) => d && dir.set(d));
                }}
                onSizeChange={({ cols, rows }) => cells.set(`${cols} × ${rows}`)}
                unavailable={(message) => (
                  <View class="flex-1 items-center justify-center p-6">
                    <Text testID="terminal-unavailable" class="text-center text-[13px] text-muted">
                      {`No terminal runs here: ${message}`}
                    </Text>
                  </View>
                )}
              />
            }>
              <MenuItem text="Copy" icon="copy" onSelect={() => ref?.copy()} />
              <MenuItem text="Paste" onSelect={() => ref?.paste()} />
              <MenuItem text="Select All" onSelect={() => ref?.selectAll()} />
            </ContextMenu>
          )}
        </Each>
      </View>
    </StagePage>
  );
}
