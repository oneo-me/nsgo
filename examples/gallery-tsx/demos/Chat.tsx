// A chat assistant on nsgo/ui/chat, streaming scripted replies: a reply
// thinks, calls a tool, writes word by word, fails or waits for the
// network, as its scenario says, and Stop ends it. The wide layout lists
// conversations beside it.
import { signal, Text, View, windowSize } from "nsgo";
import { Button, Icon } from "nsgo/ui";
import {
  type Activity,
  AttachmentPreview,
  ChipPopover,
  Composer,
  ConversationState,
  FloatingConversation,
  IconAction,
  type Status,
  type Step,
  Turn,
} from "nsgo/ui/chat";
import { DateSeparator, Markdown, Prompt, ReplyActions, thought, toolActivity } from "./Conversation";
import { freeform, type Model, photo, type Scenario, scenario, scenarios, streamTokens, tokens, type Tool, windows } from "./scripts";

interface Reply {
  id: string;
  s: Scenario;
  status: Status;
  reason: string;
  thinking: boolean;
  seconds: number;
  tools: { tool: Tool; done: boolean }[];
  md: string;
  err: string;
}

const history = ["Launch plan", "Trip to Lisbon", "Harbor runtime notes", "Primes below 10,000", "A lighthouse story"];

export function ChatDemo() {
  const state = new ConversationState();
  const wide = () => windowSize().width >= 900;
  const replies = signal<Reply[]>([]);
  const draft = signal("");
  const model = signal<Model>("Swift");
  const modelOpen = signal(false);
  const attached = signal(false);
  let ids = 0;
  // The reply being written, and how to end it.
  let live: { id: string; cancelled: boolean } | undefined;

  const update = (id: string, change: (r: Reply) => Partial<Reply>) =>
    replies.set(replies().map((r) => (r.id === id ? { ...r, ...change(r) } : r)));
  const running = () => replies().some((r) => r.status === "running" || r.status === "waiting");

  const run = (s: Scenario) => {
    const id = `r${++ids}`;
    const run = { id, cancelled: false };
    live = run;
    replies.set([...replies(), { id, s, status: "running", reason: "", thinking: false, seconds: 0, tools: [], md: "", err: "" }]);
    const after = (ms: number, fn: () => void) => setTimeout(() => { if (!run.cancelled) fn(); }, ms);
    const stream = (text: string, into: "md" | "reason", then: () => void) => {
      const parts = streamTokens(text);
      const next = (i: number) => {
        if (i >= parts.length) return then();
        after(18 + Math.random() * 30, () => {
          update(id, (r) => ({ [into]: r[into] + parts[i] }));
          next(i + 1);
        });
      };
      next(0);
    };
    const step = (i: number) => {
      const st = s.steps[i];
      if (!st) {
        update(id, () => ({ status: "completed" }));
        live = undefined;
        return;
      }
      if (st.think) {
        const began = Date.now();
        update(id, () => ({ thinking: true }));
        stream(st.think, "reason", () => {
          update(id, () => ({ thinking: false, seconds: Math.max(1, Math.round((Date.now() - began) / 1000)) }));
          step(i + 1);
        });
      } else if (st.tool) {
        const tool = st.tool;
        update(id, (r) => ({ tools: [...r.tools, { tool, done: false }] }));
        after(st.ms ?? 1000, () => {
          update(id, (r) => ({ tools: r.tools.map((t) => (t.tool.id === tool.id ? { tool, done: true } : t)) }));
          step(i + 1);
        });
      } else if (st.say) {
        stream(st.say, "md", () => step(i + 1));
      } else if (st.fail) {
        update(id, () => ({ status: "failed", err: st.fail! }));
        live = undefined;
      } else if (st.offline) {
        update(id, () => ({ status: "waiting" }));
        after(st.offline, () => {
          update(id, () => ({ status: "running" }));
          step(i + 1);
        });
      }
    };
    step(0);
  };
  const send = () => {
    const text = draft().trim();
    if (!text && !attached()) return;
    const found = scenarios.find((s) => s.prompt === text);
    const s = found ?? freeform(text || "What's in this photo?", model());
    run(attached() ? { ...s, image: photo } : s);
    draft.set("");
    attached.set(false);
  };
  const stop = () => {
    if (!live) return;
    live.cancelled = true;
    update(live.id, () => ({ status: "cancelled" }));
    live = undefined;
  };

  const steps = (r: Reply): Step[] => {
    const activities: Activity[] = [];
    if (r.reason) {
      activities.push(r.thinking
        ? { id: "think", icon: "brain", action: "Thinking", status: "running", content: () => <Text class="select-text text-[13px] text-muted">{r.reason}</Text> }
        : thought(r.seconds, r.reason));
    }
    for (const t of r.tools) activities.push(toolActivity(t.tool, t.done ? "completed" : "running"));
    return activities.length ? [{ id: "work", title: r.tools.length ? `Ran ${r.tools.length} tool${r.tools.length > 1 ? "s" : ""}` : "Thought", activities }] : [];
  };

  const ModelRow = (props: { name: Model }) => (
    <View role="button" accessibilityLabel={props.name} class="flex-row items-center justify-between rounded-md px-2 py-1.5 hover:bg-inset"
      classList={{ "bg-inset": model() === props.name }}
      onPress={() => { model.set(props.name); modelOpen.set(false); }}>
      <Text class="text-[13px] text-text">{props.name}</Text>
      <Text class="text-[12px] text-muted">{tokens(windows[props.name])}</Text>
    </View>
  );

  const empty = (
    <View testID="chat-empty" class="items-center gap-4 pt-16">
      <View class="h-12 w-12 items-center justify-center rounded-2xl border border-border bg-raised">
        <Icon name="sparkles" size={24} />
      </View>
      <Text class="text-[20px] font-semibold text-text">What can I help with?</Text>
      <View class="max-w-[560px] flex-row flex-wrap justify-center gap-2">
        {scenarios.filter((s) => s.chip).map((s) => (
          <Button testID={`chat-try-${s.id}`} text={s.chip!} variant="secondary" size="small" onPress={() => { model.set(s.model); run(s); }} />
        ))}
      </View>
    </View>
  );

  const transcript = (
    <View selectable class="min-w-0 gap-12">
      <If when={replies().length > 0} else={empty}>
        <DateSeparator label="Today" />
      </If>
      <Each of={replies()} key={(r: Reply) => r.id}>
        {(r: () => Reply) => (
          <Turn
            testID={`chat-${r().id}`}
            conversation={state}
            status={r().status}
            prompt={() => <Prompt s={r().s} />}
            steps={steps(r())}
            summary="Worked for a moment"
            working={r().status === "waiting" ? "Waiting for a connection…" : "Working…"}
            hasFinal={r().md !== ""}
            final={() => <Markdown text={r().md} />}
            footer={r().status === "failed" ? () => (
              <View class="flex-row items-center gap-2">
                <Text class="min-w-0 flex-1 text-[13px] text-danger">{r().err}</Text>
                <Button text="Retry" variant="secondary" size="small" onPress={() => run(scenario("guide"))} />
              </View>
            ) : r().status === "cancelled" ? () => <Text class="text-[13px] text-faint">You stopped this reply.</Text> : undefined}
            showActions={r().status === "completed"}
            actions={() => <ReplyActions />}
          />
        )}
      </Each>
    </View>
  );

  const composer = (
    <Composer
      testID="chat-composer"
      glass
      state={state}
      draft={draft()}
      onDraftChange={({ text }) => draft.set(text)}
      accessibilityLabel="Message"
      placeholder={`Message ${model()}`}
      sendAccessibilityLabel="Send"
      stopAccessibilityLabel="Stop"
      running={running()}
      onSend={send}
      onStop={stop}
      hasAttachments={attached()}
      attachments={() => <AttachmentPreview accessibilityLabel="Photo" removeAccessibilityLabel="Remove" image={photo} onRemove={() => attached.set(false)} />}
      accessory={() => (
        <ChipPopover testID="chat-model" chip={{ text: model(), icon: "sparkles", caret: true, yield: true }} open={modelOpen()}
          onOpenChange={({ open }) => modelOpen.set(open)} accessibilityLabel="Choose a model" width={240}>
          <View class="gap-px p-2">
            <ModelRow name="Swift" />
            <ModelRow name="Deep" />
            <ModelRow name="Vision" />
          </View>
        </ChipPopover>
      )}
      trailing={() => <IconAction accessibilityLabel="Attach a photo" icon="paperclip" onPress={() => attached.set(true)} />}
    />
  );

  return (
    <View class="min-h-0 flex-1 flex-row bg-background">
      <If when={wide()}>
        <View testID="chat-history" class="w-[220px] shrink-0 gap-1 border-r border-line p-3">
          <Button testID="chat-new" text="New chat" icon="plus" variant="secondary" onPress={() => { stop(); replies.set([]); }} />
          <Text class="px-2 pb-1 pt-3 text-[11px] font-semibold uppercase tracking-wide text-faint">Recent</Text>
          {history.map((h) => <Text class="rounded-md px-2 py-1.5 text-[13px] text-muted line-clamp-1">{h}</Text>)}
        </View>
      </If>
      <View class="min-h-0 min-w-0 flex-1">
        <FloatingConversation testID="chat" state={state} jumpAccessibilityLabel="Jump to the latest message" composer={[composer]}>
          {transcript}
        </FloatingConversation>
      </View>
    </View>
  );
}
