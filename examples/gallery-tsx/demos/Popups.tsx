import { signal, Text, View } from "nsgo";
import { Avatar } from "nsgo/ui";
import {
  AlertDialog,
  Button,
  Command,
  CommandDialog,
  ContextMenu,
  Dialog,
  HoverCard,
  Input,
  Menu,
  MenuItem,
  MenuLabel,
  MenuSeparator,
  Popover,
  Sheet,
  Submenu,
  Tooltip,
} from "nsgo/ui";
import { BoolKnob, ChoiceKnob, Page, Playground, Row, useEvents } from "../parts";

const placements = ["top", "bottom", "left", "right"] as const;
const label = "text-[13px] text-text";

export function DialogDemo() {
  const dialog = signal(false);
  const alert = signal(false);
  const sheet = signal(false);
  const escape = signal(true);
  const backdrop = signal(true);
  return (
    <Page>
      <Playground
        title="Dialog"
        knobs={
          <>
            <BoolKnob name="CloseOnEscape" value={escape} />
            <BoolKnob name="CloseOnBackdropPress" value={backdrop} />
          </>
        }
      >
        <Button text="Open a dialog" variant="secondary" onPress={() => dialog.set(true)} />
        <Dialog
          testID="dialog"
          open={dialog()}
          onOpenChange={({ open }) => dialog.set(open)}
          title="Rename the project"
          description="Its folder keeps its name."
          closeOnEscape={escape()}
          closeOnBackdropPress={backdrop()}
          footer={[
            <Button text="Cancel" variant="secondary" onPress={() => dialog.set(false)} />,
            <Button
              text="Save"
              onPress={() => dialog.set(false)}
            />,
          ]}
        >
          <Input placeholder="Project name" />
        </Dialog>
      </Playground>
      <Playground title="AlertDialog">
        <Button text="Delete…" variant="ghost" onPress={() => alert.set(true)} />
        <AlertDialog
          testID="alert"
          open={alert()}
          onOpenChange={({ open }) => alert.set(open)}
          title="Delete the project?"
          description="This cannot be undone."
          actions={[
            { text: "Cancel", variant: "secondary", onPress: () => alert.set(false) },
            {
              text: "Delete",
              variant: "accent",
              onPress: () => alert.set(false),
            },
          ]}
        />
      </Playground>
      <Playground title="Sheet">
        <Button text="Open a sheet" variant="secondary" onPress={() => sheet.set(true)} />
        <Sheet open={sheet()} onClose={() => sheet.set(false)}>
          <View class="gap-3 p-5">
            <Text class="text-[15px] font-semibold text-text">A sheet</Text>
            <Text class="text-[13px] text-muted">It rises from the bottom, and a press outside closes it.</Text>
            <Button text="Close" onPress={() => sheet.set(false)} />
          </View>
        </Sheet>
      </Playground>
    </Page>
  );
}

export function PopoverDemo() {
  const placement = signal<(typeof placements)[number]>("bottom");
  const open = signal(false);
  const hover = signal(false);
  return (
    <Page>
      <Playground title="Popover" knobs={<ChoiceKnob name="Placement" value={placement} options={placements} />}>
        <Popover
          testID="popover"
          accessibilityLabel="Details"
          trigger={{ text: "Open a popover", variant: "secondary" }}
          placement={placement()}
          open={open()}
          onOpenChange={({ open: o }) => open.set(o)}
        >
          <View class="w-56 gap-1 p-1">
            <Text class="text-[13px] font-semibold text-text">Popover</Text>
            <Text class="text-[12px] text-muted">It floats over the page, anchored to its trigger.</Text>
          </View>
        </Popover>
      </Playground>
      <Playground title="Tooltip" knobs={<ChoiceKnob name="Placement" value={placement} options={placements} />}>
        <Tooltip text="Saves the document" placement={placement()} trigger={<Button text="Hover me" variant="ghost" />} />
      </Playground>
      <Playground title="HoverCard">
        <HoverCard
          open={hover()}
          onOpenChange={({ open: o }) => hover.set(o)}
          placement="bottom"
          trigger={<Text class="text-[13px] text-accent">@ada</Text>}
        >
          <View class="w-56 flex-row items-center gap-3 p-1">
            <Avatar name="Ada Lovelace" size={36} />
            <View class="flex-1 gap-0.5">
              <Text class="text-[13px] font-semibold text-text">Ada Lovelace</Text>
              <Text class="text-[12px] text-muted">The first programmer.</Text>
            </View>
          </View>
        </HoverCard>
      </Playground>
    </Page>
  );
}

const commands = [
  { id: "new", label: "New file", group: "File", shortcut: "mod+n", icon: "file-text" },
  { id: "open", label: "Open…", group: "File", shortcut: "mod+o", icon: "folder" },
  { id: "search", label: "Search", group: "Edit", shortcut: "mod+f", icon: "search" },
  { id: "theme", label: "Toggle the appearance", group: "View", keywords: ["dark", "light"], icon: "moon" },
  { id: "sidebar", label: "Toggle the sidebar", group: "View", shortcut: "mod+b", icon: "panel-left" },
];

export function CommandDemo() {
  const inline = useEvents();
  const palette = useEvents();
  const open = signal(false);
  return (
    <Page>
      <Playground title="Command" start event={inline.last()}>
        <Command
          testID="command"
          accessibilityLabel="Commands"
          items={commands}
          placeholder="Type a command"
          empty="No results"
          onSelect={({ id }) => inline.log(`Ran ${id}`)()}
          style={{ height: 280 }}
        />
      </Playground>
      <Playground title="CommandDialog" event={palette.last()}>
        <Button text="Open the palette" variant="secondary" onPress={() => open.set(true)} />
        <CommandDialog
          testID="palette"
          open={open()}
          onOpenChange={({ open: o }) => open.set(o)}
          command={{
            items: commands,
            placeholder: "Type a command",
            empty: "No results",
            autoFocus: true,
            onSelect: (id: string) => {
              open.set(false);
              palette.log(`Ran ${id}`)();
            },
          }}
        />
      </Playground>
    </Page>
  );
}

export function MenuDemo() {
  const menu = useEvents();
  const context = useEvents();
  const wrap = signal(true);
  const ctx = signal(false);
  return (
    <Page>
      <Playground title="Menu" event={menu.last()}>
        <Menu testID="menu" accessibilityLabel="File" trigger={{ text: "File", variant: "secondary" }}>
          <MenuLabel text="Document" />
          <MenuItem text="New" icon="file-text" shortcut="mod+n" onSelect={menu.log("New")} />
          <MenuItem text="Open…" icon="folder" shortcut="mod+o" onSelect={menu.log("Open")} />
          <MenuSeparator />
          <MenuItem text="Word wrap" checked={wrap()} onSelect={() => wrap.set(!wrap())} closeOnSelect={false} />
          <Submenu text="Share" icon="share">
            <MenuItem text="Copy link" onSelect={menu.log("Copy link")} />
            <MenuItem text="Email" onSelect={menu.log("Email")} />
          </Submenu>
          <MenuSeparator />
          <MenuItem text="Delete" icon="trash" variant="danger" onSelect={menu.log("Delete")} />
        </Menu>
      </Playground>
      <Playground title="ContextMenu" note="Right-click or long-press the area." fill event={context.last()}>
        <ContextMenu
          testID="context"
          open={ctx()}
          onOpenChange={({ open }) => ctx.set(open)}
          trigger={
            <View class="h-28 items-center justify-center rounded-lg border border-dashed border-border">
              <Text class={label}>Right-click here</Text>
            </View>
          }
        >
          <MenuItem text="Cut" shortcut="mod+x" onSelect={context.log("Cut")} />
          <MenuItem text="Copy" shortcut="mod+c" onSelect={context.log("Copy")} />
          <MenuItem text="Paste" shortcut="mod+v" onSelect={context.log("Paste")} />
        </ContextMenu>
      </Playground>
    </Page>
  );
}
