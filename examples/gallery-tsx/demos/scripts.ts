// The chat's scripted scenarios, which the Conversation page shows and the
// AI Chat app streams: a reply thinks, calls a tool, writes, fails or waits
// for the network, step by step.

export type Model = "Swift" | "Deep" | "Vision";

/** The fictional models' context windows. */
export const windows: Record<Model, number> = { Swift: 128_000, Deep: 1_000_000, Vision: 200_000 };

/** A count of tokens, short. */
export function tokens(n: number): string {
  if (n >= 999_500) return `${(n / 1e6).toFixed(1)}M`;
  if (n >= 1000) return `${Math.round(n / 1e3)}k`;
  return String(n);
}

export interface Tool {
  id: string;
  kind: "search" | "weather" | "code";
  query?: string;
  sources?: { title: string; site: string }[];
  city?: string;
  temp?: number;
  high?: number;
  low?: number;
  condition?: string;
  lang?: string;
  code?: string;
  output?: string;
}

export interface ScriptStep {
  think?: string;
  tool?: Tool;
  ms?: number;
  say?: string;
  fail?: string;
  offline?: number;
}

export interface Scenario {
  id: string;
  chip?: string;
  prompt: string;
  model: Model;
  image?: string;
  file?: { name: string; size: string };
  steps: ScriptStep[];
}

export const photo = "https://picsum.photos/seed/harbor/640/480";

const fence = "```";

const guide = `## Rendering a long list
A list only keeps the rows **near the viewport** alive; the rest exist as *data*. Three ideas make it fast:

1. **Windowing**: rows outside the window are not built at all.
2. **Recycling**: a row that scrolls out is reused for the one scrolling in.
3. **Measuring once**: heights are cached per item, so scrolling never lays out twice.

### What it costs
| Approach | Views | Memory |
| --- | --- | --- |
| Plain scroll view | 10,000 | 412 MB |
| Windowed list | 24 | 18 MB |

Keep per-row state in your data, never inside the row:

${fence}tsx
<List data={photos()} estimatedItemSize={76}>
  {(photo) => <PhotoRow photo={photo()} />}
</List>
${fence}

> A recycled row shows another item tomorrow. Anything it remembers on its own goes with it.

- Give the list a \`key\` so rows keep their cells as items come and go.
- Prefer fixed heights for rows you can predict.`;

const story = `The lighthouse keeper had not spoken to anyone in eleven days when the radio crackled back to life. It was not a voice at first, only a pattern: three short bursts, a pause, then three again, as if someone on the other side of the fog were knocking politely on a door.

She wrote the pattern down in the margin of the logbook, next to the tide tables and the price of lamp oil. By morning the knocking had become a sentence, and by noon the sentence had become a question she had been waiting her whole life to be asked.

*Is the light still on?* it said.

She looked up at the great lens turning slowly above her, throwing its beam across a sea nobody sailed anymore, and she smiled for the first time in eleven days. **Yes,** she answered. **It always is.**`;

export const scenarios: Scenario[] = [
  { id: "story", chip: "Tell me a short story", prompt: "Tell me a short story about a lighthouse.", model: "Swift", steps: [{ say: story }] },
  { id: "guide", chip: "Explain list virtualization", prompt: "How do recycled lists stay fast? Use a table and an example.", model: "Swift", steps: [{ say: guide }] },
  {
    id: "reason", chip: "Think it through", model: "Deep",
    prompt: "A bat and a ball cost 1.10 in total. The bat costs 1.00 more than the ball. How much is the ball?",
    steps: [
      { think: "The quick answer is 0.10, but let me check it. If the ball were 0.10, the bat would be 1.10 and the total 1.20, which is too much. Call the ball x. Then the bat is x + 1.00, so x + x + 1.00 = 1.10, which gives 2x = 0.10 and x = 0.05. Check: 0.05 + 1.05 = 1.10, and the difference is exactly 1.00. That works." },
      { say: "The ball costs **0.05**.\n\nIf the ball is *x*, the bat is *x* + 1.00, so 2*x* + 1.00 = 1.10 and *x* = 0.05. The intuitive answer, 0.10, would make the total 1.20." },
    ],
  },
  {
    id: "search", chip: "Search the web", model: "Swift", prompt: "What changed in the latest release of the Harbor runtime?",
    steps: [
      {
        tool: { id: "search", kind: "search", query: "Harbor runtime release notes", sources: [
          { title: "Harbor 4.2 release notes", site: "harbor.dev" },
          { title: "What's new in the Harbor scheduler", site: "blog.harbor.dev" },
          { title: "Benchmarks: Harbor 4.1 vs 4.2", site: "perfweekly.io" },
          { title: "Migrating to the new config format", site: "docs.harbor.dev" },
        ] },
        ms: 1600,
      },
      { say: "Harbor **4.2** shipped last week. The highlights, from the release notes and two write-ups:\n\n- A new **work-stealing scheduler** that cuts tail latency by about 30 %.\n- Configuration moved to a single `harbor.toml`; the old flags still work for one more release.\n- Cold starts are roughly **2× faster** thanks to lazy module loading.\n\nThe benchmark post notes the gains are smallest on single-core machines." },
    ],
  },
  {
    id: "weather", chip: "Weather in Lisbon", model: "Swift", prompt: "Do I need a jacket in Lisbon this evening?",
    steps: [
      { tool: { id: "weather", kind: "weather", city: "Lisbon", temp: 21, condition: "Partly cloudy", high: 24, low: 16 }, ms: 1000 },
      { say: "It's **21°** now and drops to about **16°** after sunset, with a breeze off the river. A light jacket is a good idea for the evening." },
    ],
  },
  {
    id: "code", chip: "Run some code", model: "Deep", prompt: "How many primes are there below 10,000?",
    steps: [
      { think: "Counting by hand is error-prone. A sieve of Eratosthenes up to 10,000 is quick to run, so I will run one and report the count." },
      {
        tool: { id: "code", kind: "code", lang: "go", output: "1229",
          code: "n := 10_000\nsieve := make([]bool, n)\ncount := 0\nfor i := 2; i < n; i++ {\n\tif !sieve[i] {\n\t\tcount++\n\t\tfor j := i * i; j < n; j += i {\n\t\t\tsieve[j] = true\n\t\t}\n\t}\n}\nfmt.Println(count)" },
        ms: 1800,
      },
      { say: "There are **1,229** primes below 10,000. I counted them with a sieve of Eratosthenes, which runs in a few milliseconds." },
    ],
  },
  {
    id: "vision", chip: "Describe a photo", model: "Vision", prompt: "What's in this photo? And summarize the attached notes.",
    image: photo, file: { name: "trip-notes.pdf", size: "2.4 MB" },
    steps: [{ say: "**The photo** shows a harbor at dusk: a few moored boats, low warm light on the water and a line of buildings along the far shore.\n\n**trip-notes.pdf** (4 pages) is an itinerary:\n- Day 1: arrive, walk the old town.\n- Day 2: boat tour of the harbor, the one in your photo.\n- Day 3: museum, then the evening train back." }],
  },
  {
    id: "error", chip: "Fail a reply", model: "Swift", prompt: "Summarize this week's meeting notes.",
    steps: [{ say: "Here's a summary of the four meetings this week. The planning session agreed on" }, { fail: "The model is overloaded right now (error 529)." }],
  },
  {
    id: "offline", chip: "Go offline", model: "Swift", prompt: "Remind me what we decided about the launch date.",
    steps: [{ offline: 2600 }, { say: "You decided to launch on **Tuesday, November 3**, after the final review on Monday afternoon." }],
  },
];

export function scenario(id: string): Scenario {
  return scenarios.find((s) => s.id === id) ?? scenarios[0];
}

/** What a scenario says, calls and thinks, at its end. */
export function script(id: string): { s: Scenario; say: string; tool?: Tool; think: string } {
  const s = scenario(id);
  let say = "", think = "", tool: Tool | undefined;
  for (const st of s.steps) {
    if (st.say) say = st.say;
    if (st.tool) tool = st.tool;
    if (st.think) think = st.think;
  }
  return { s, say, tool, think };
}

/** A free-form prompt: the model echoes it in its own voice. */
export function freeform(prompt: string, model: Model): Scenario {
  const quote = prompt.length > 60 ? prompt.slice(0, 57) + "…" : prompt;
  const steps: ScriptStep[] = [];
  if (model === "Deep") {
    steps.push({ think: `The user asked: "${quote}". This is a simulated assistant, so I will explain that and show what a structured answer would look like.` });
  }
  const kind = model === "Deep" ? "careful" : model === "Vision" ? "visual" : "quick";
  steps.push({ say: `You asked: *${quote}*\n\nThis assistant is simulated, so here is the shape a ${kind} answer would take:\n\n- **A direct answer** in the first sentence.\n- **The reasoning** behind it, kept short.\n- **A next step** you can take.\n\nTry the suggestions for streaming, tools, reasoning and errors.` });
  return { id: "free", prompt, model, steps };
}

/** A tool call's row in a turn: its icon, verb and target. */
export function toolRow(t: Tool): { icon: string; action: string; detail: string } {
  switch (t.kind) {
    case "search":
      return { icon: "globe", action: "Searched the web", detail: t.query ?? "" };
    case "weather":
      return { icon: "sun", action: "Weather", detail: t.city ?? "" };
  }
  return { icon: "terminal", action: "Ran code", detail: `${t.lang} · sandboxed` };
}

/** What a tool call shows when its row opens. */
export function toolOutput(t: Tool): string {
  switch (t.kind) {
    case "search":
      return (t.sources ?? []).map((s) => `${s.title} — ${s.site}`).join("\n");
    case "weather":
      return `${t.temp}° · ${t.condition}\nHigh ${t.high}° · Low ${t.low}°`;
  }
  return `${t.code}\n\n→ ${t.output}`;
}

/** Splits text into the pieces a model streams: words and the spaces after them. */
export function streamTokens(text: string): string[] {
  return text.match(/\S+\s*|\s+/g) ?? [];
}
