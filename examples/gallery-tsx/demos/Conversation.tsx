// nsgo/ui/chat as an app puts it together: the transcript with a turn in
// each state it can be in, a panel on the glass, and the composer with its
// attachments, choices and toolbar. Its controls keep their own state and
// nothing behind them runs.
import { Image, type JSX, signal, Text, token, View, windowSize } from "nsgo";
import { Icon, Switch, TextView } from "nsgo/ui";
import {
  type Activity,
  Attachment,
  AttachmentPreview,
  Bubble,
  Chip,
  ChipPopover,
  Composer,
  ContextMeter,
  ConversationState,
  EffortPicker,
  FloatingConversation,
  GlassPanel,
  IconAction,
  Message,
  Picker,
  QuotaSummary,
  type Quota,
  ResultSummary,
  Shimmer,
  type Status,
  type Step,
  Turn,
} from "nsgo/ui/chat";
import { type Model, photo, type Scenario, script, tokens, type Tool, toolOutput, toolRow, windows } from "./scripts";

/** Markdown as a reply writes it. */
export function Markdown(props: { text: string }) {
  return <TextView class="min-w-0" markdown={props.text} selectable />;
}

/** The user's message: its attachments above the words, in a bubble. */
export function Prompt(props: { s: Scenario }) {
  return (
    <Message outgoing>
      <Bubble>
        <If when={props.s.image}>
          <Image src={props.s.image!} accessibilityLabel="Photo" class="mb-1.5 h-[120px] w-[160px] rounded-lg object-cover" />
        </If>
        <If when={props.s.file}>
          <Attachment label={props.s.file!.name} detail={props.s.file!.size} />
        </If>
        <Text class="select-text text-[13px] leading-[19px] text-text">{props.s.prompt}</Text>
      </Bubble>
    </Message>
  );
}

export function thought(seconds: number, text: string): Activity {
  return { id: "think", icon: "brain", action: `Thought for ${seconds}s`, status: "completed", content: () => <Text class="select-text text-[13px] text-muted">{text}</Text> };
}

export function toolActivity(t: Tool, status: Status): Activity {
  const row = toolRow(t);
  return { id: t.id, icon: row.icon, action: row.action, detail: row.detail, status, mono: t.kind === "code", maxHeight: 320,
    content: () => <Text class="select-text font-mono text-[12px] text-muted">{toolOutput(t)}</Text> };
}

export function DateSeparator(props: { label: string }) {
  return (
    <View class="flex-row items-center gap-3">
      <View class="h-px flex-1 bg-line" />
      <Text class="text-[11.5px] font-medium text-faint">{props.label}</Text>
      <View class="h-px flex-1 bg-line" />
    </View>
  );
}

export function ReplyActions() {
  return (
    <>
      <IconAction accessibilityLabel="Copy" icon="copy" onPress={() => {}} />
      <IconAction accessibilityLabel="Regenerate" icon="refresh-cw" onPress={() => {}} />
      <IconAction accessibilityLabel="Good reply" icon="thumbs-up" onPress={() => {}} />
      <IconAction accessibilityLabel="Bad reply" icon="thumbs-down" onPress={() => {}} />
    </>
  );
}

export function ConversationDemo() {
  const state = new ConversationState();
  const wide = () => windowSize().width >= 760;
  const draft = signal("");
  const writing = signal(true);
  const listening = signal(false);
  const hasPhoto = signal(true);
  const hasFile = signal(true);
  const model = signal<Model>("Swift");
  const modelOpen = signal(false);
  const effort = signal("medium");
  const tier = signal("standard");
  const branch = signal("main");
  const worktree = signal(true);

  const turn = (status: Status, s: Scenario, steps: Step[], final: string, footer?: () => JSX.Element, id?: string) => (
    <Turn
      testID={id}
      status={status}
      conversation={state}
      prompt={() => <Prompt s={s} />}
      steps={steps}
      summary="Worked for 12s"
      working="Running · 12s"
      hasFinal={final !== ""}
      final={() => <Markdown text={final} />}
      footer={footer}
      showActions={status === "completed"}
      actions={() => <ReplyActions />}
    />
  );
  const note = (text: string) => () => <Text class="text-[13px] text-faint">{text}</Text>;
  const work = (title: string, ...activities: Activity[]): Step[] => [{ id: "work", title, activities }];

  const guide = script("guide"), vision = script("vision"), reason = script("reason"), weather = script("weather");
  const failing = script("error"), offline = script("offline"), search = script("search");
  const files: Scenario = { id: "files", prompt: "Draft the launch plan and what goes with it.", model: "Swift", steps: [] };
  const results = [
    { id: "plan", label: "launch-plan.md", detail: "12 KB" },
    { id: "checklist", label: "checklist.md", detail: "3 KB" },
    { id: "announcement", label: "announcement.md", detail: "2 KB" },
    { id: "budget", label: "budget.csv", detail: "8 KB" },
    { id: "timeline", label: "timeline.png", detail: "164 KB" },
  ];
  const transcript = (
    <View selectable class="min-w-0 gap-12">
      <DateSeparator label="Today" />
      {turn("completed", guide.s, [], guide.say, undefined, "turn-guide")}
      {turn("completed", vision.s, [], vision.say)}
      {turn("completed", reason.s, work("Ran 1 tool", thought(5, reason.think)), reason.say, undefined, "turn-reason")}
      {turn("completed", weather.s, work("Ran 1 tool", toolActivity(weather.tool!, "completed")), weather.say)}
      {turn("completed", files, [], "The launch plan is ready, with the checklist and the announcement beside it.", () => (
        <ResultSummary title={`${results.length} files written`} description="In the project's docs folder" moreLabel="Show more" lessLabel="Show less"
          items={results} conversation={state} />
      ))}
      {turn("failed", failing.s, [], failing.say, () => (
        <View class="flex-row items-center gap-2">
          <Icon name="circle-alert" size={14} color={token("danger")} />
          <Text class="text-[13px] text-danger">The model is overloaded right now (error 529).</Text>
        </View>
      ))}
      {turn("cancelled", reason.s, [], "The ball costs", note("You stopped this reply."))}
      {turn("waiting", offline.s, [], "", note("Waiting for a connection…"))}
      {turn("running", search.s, [...work("Searching the web", toolActivity(search.tool!, "running")),
        { id: "reply", message: () => <Markdown text={search.say} /> }], "", undefined, "turn-running")}
    </View>
  );

  const levels = [{ id: "off", label: "Off" }, { id: "low", label: "Low" }, { id: "medium", label: "Medium" }, { id: "high", label: "High" }, { id: "max", label: "Max" }];
  const tiers = [{ id: "standard", label: "Standard" }, { id: "fast", label: "Fast" }];
  const branches = [{ id: "main", label: "main" }, { id: "feature/chat-ui", label: "feature/chat-ui" }, { id: "release/0.2026.10", label: "release/0.2026.10" }];
  const rows = [
    { label: "5-hour limit", detail: "Resets in 4h 32m", value: "63% left", fraction: 0.63 },
    { label: "Weekly limit", detail: "Resets Monday", value: "18% left", fraction: 0.18 },
    { label: "Credits", value: "$12.40", fraction: -1 },
  ];
  const quota: Quota = {
    id: "demo", title: "Account", refreshLabel: "Refresh", openLabel: "Open usage", countdown: "04:32",
    rows,
    onRefresh: () => {}, onOpen: () => {},
  };
  const ModelRow = (props: { name: Model }) => (
    <View role="button" accessibilityLabel={props.name} class="flex-row items-center justify-between rounded-md px-2 py-1.5 hover:bg-inset"
      classList={{ "bg-inset": model() === props.name }}
      onPress={() => { model.set(props.name); modelOpen.set(false); }}>
      <Text class="text-[13px] text-text">{props.name}</Text>
      <Text class="text-[12px] text-muted">{tokens(windows[props.name])}</Text>
    </View>
  );
  const meter = "42.6k / 128k (33%)";

  const composer = (
    <View class="gap-1.5">
      <GlassPanel testID="conversation-panel">
        <View class="flex-row items-center gap-2">
          <Icon name="circle-alert" size={14} color={token("danger")} />
          <Text class="text-[12.5px] text-danger">Your message sends when the connection is back.</Text>
        </View>
      </GlassPanel>
      <Composer
        testID="conversation-composer"
        glass
        state={state}
        draft={draft()}
        onDraftChange={({ text }) => draft.set(text)}
        accessibilityLabel="Message"
        placeholder="Ask anything"
        sendAccessibilityLabel="Send"
        stopAccessibilityLabel="Stop"
        running={writing()}
        onSend={() => { draft.set(""); writing.set(true); }}
        onStop={() => writing.set(false)}
        hasAttachments={hasPhoto() || hasFile()}
        attachments={() => (
          <>
            <If when={hasPhoto()}>
              <AttachmentPreview accessibilityLabel="Photo" removeAccessibilityLabel="Remove" image={photo} onRemove={() => hasPhoto.set(false)} />
            </If>
            <If when={hasFile()}>
              <Attachment label="trip-notes.pdf" detail="2.4 MB" removeAccessibilityLabel="Remove" onRemove={() => hasFile.set(false)} />
            </If>
          </>
        )}
        accessory={() => (
          <>
            <ChipPopover testID="conversation-model" chip={{ text: model(), icon: "sparkles", caret: true, yield: true }} open={modelOpen()}
              onOpenChange={({ open }) => modelOpen.set(open)} accessibilityLabel="Choose a model" width={240}>
              <View class="gap-px p-2">
                <ModelRow name="Swift" />
                <ModelRow name="Deep" />
                <ModelRow name="Vision" />
              </View>
            </ChipPopover>
            <EffortPicker accessibilityLabel="Effort" icon="brain" levels={levels} selected={effort()} onChange={({ id }) => effort.set(id)} off />
            <If when={wide()}>
              <Picker icon="zap" options={tiers} selected={tier()} onChange={({ id }) => tier.set(id)} width={200}
                accessibilityLabel={tier() === "fast" ? "Fast" : "Standard"} />
            </If>
          </>
        )}
        trailing={() => (
          <>
            <IconAction accessibilityLabel="Choose a photo" icon="paperclip" onPress={() => { hasPhoto.set(true); hasFile.set(true); }} />
            <If when={wide()}>
              <IconAction accessibilityLabel="Voice input" icon="mic" active={listening()} onPress={() => listening.set(!listening())} />
            </If>
            <ContextMeter accessibilityLabel="Context window" title="Context window" tooltip={meter} value={meter} fraction={0.33} />
          </>
        )}
        footer={() => (
          <>
            <If when={wide()}>
              <Chip text="MacBook Pro" icon="laptop" />
            </If>
            <Chip text="nsgo" icon="folder" />
            <Picker accessibilityLabel={branch()} icon="git-branch" options={branches} selected={branch()} onChange={({ id }) => branch.set(id)}
              searchAccessibilityLabel="Search branches" emptyText="No results" searchable={2} />
            <If when={wide()}>
              <View class="ml-[7px] flex-row items-center gap-1.5">
                <Switch size="small" accessibilityLabel="Worktree" checked={worktree()} onChange={({ checked }) => worktree.set(checked)} />
                <Text class="text-[12.5px] text-muted">Worktree</Text>
              </View>
            </If>
            <View class="min-w-0 grow" />
            <If when={wide() && writing()}>
              <Shimmer text="Streaming…" size={12.5} />
            </If>
            <QuotaSummary accessibilityLabel="Usage limits" tooltip={`Account\n${rows[0].label} · ${rows[0].value}`}
              row={rows[0]} quotas={[quota]} />
          </>
        )}
      />
    </View>
  );

  return (
    <View class="min-h-0 flex-1 bg-background">
      <FloatingConversation testID="conversation" state={state} jumpAccessibilityLabel="Jump to the latest message" composer={[composer]}>
        {transcript}
      </FloatingConversation>
    </View>
  );
}
