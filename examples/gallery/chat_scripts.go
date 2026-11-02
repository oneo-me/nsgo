package main

import (
	"fmt"

	"github.com/oneo-me/nsgo/core"

	ui "github.com/oneo-me/nsgo/examples/gallery-ui"
)

// The chat's fictional models: the picker changes the voice and whether
// the reply thinks first.
type chatModel string

type toolState string

const (
	toolPending   toolState = "pending"
	toolRunning   toolState = "running"
	toolDone      toolState = "done"
	toolCancelled toolState = "cancelled"
)

type chatSource struct {
	title, site string
	tone        ui.Tone
}

// chatTool is a tool call: a web search, the weather or code run in a
// sandbox, by kind.
type chatTool struct {
	id, kind string
	state    toolState
	// search
	query   string
	sources []chatSource
	// weather
	city            string
	temp, high, low int
	condition       string
	hours           []int
	// code
	lang, code, output string
	ms                 int
}

type fileRef struct{ name, size string }

type reasoning struct {
	text    string
	seconds int
	open    bool
}

// msg is a row of the conversation: a date, the user's message or the
// assistant's reply, by kind.
type msg struct {
	kind, id string
	// date
	label string
	// user
	text    string
	image   core.ImageSource
	file    *fileRef
	waiting bool
	// assistant
	model     chatModel
	script    string
	reasoning *reasoning
	tools     []chatTool
	md        string
	status    string
	err       string
	rating    string
}

// chatStep is a step of a scripted reply: thinking, a tool call, words,
// a failure or a time offline.
type chatStep struct {
	think   string
	tool    *chatTool
	ms      int
	say     string
	fail    string
	offline int
}

type scenario struct {
	id, chip, prompt string
	model            chatModel
	image            core.ImageSource
	file             *fileRef
	steps            []chatStep
}

// chatAttachment is what the composer attaches to the next message: a
// photo, and a file.
type chatAttachment struct {
	image core.ImageSource
	file  fileRef
}

const chatPhoto = "https://picsum.photos/seed/harbor/640/480"

const chatGuide = `## Rendering a long list
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

` + "`" + `` + "`" + `` + "`" + `go
core.List{Count: count, EstimatedSize: 76,
	Row: func(cx *core.Scope, r core.ListRow) core.Node { return photoRow(r) }}
` + "`" + `` + "`" + `` + "`" + `

> A recycled row shows another item tomorrow. Anything it remembers on its own goes with it.

- Give the list a ` + "`" + `Key` + "`" + ` so rows keep their cells as items come and go.
- Prefer fixed heights for rows you can predict.`

const chatStory = `The lighthouse keeper had not spoken to anyone in eleven days when the radio crackled back to life. It was not a voice at first, only a pattern: three short bursts, a pause, then three again, as if someone on the other side of the fog were knocking politely on a door.

She wrote the pattern down in the margin of the logbook, next to the tide tables and the price of lamp oil. By morning the knocking had become a sentence, and by noon the sentence had become a question she had been waiting her whole life to be asked.

*Is the light still on?* it said.

She looked up at the great lens turning slowly above her, throwing its beam across a sea nobody sailed anymore, and she smiled for the first time in eleven days. **Yes,** she answered. **It always is.**`

var scenarios = []scenario{
	{id: "story", chip: "Tell me a short story", prompt: "Tell me a short story about a lighthouse.", model: "Swift", steps: []chatStep{{say: chatStory}}},
	{id: "guide", chip: "Explain list virtualization", prompt: "How do recycled lists stay fast? Use a table and an example.", model: "Swift", steps: []chatStep{{say: chatGuide}}},
	{id: "reason", chip: "Think it through", prompt: "A bat and a ball cost 1.10 in total. The bat costs 1.00 more than the ball. How much is the ball?", model: "Deep", steps: []chatStep{
		{think: "The quick answer is 0.10, but let me check it. If the ball were 0.10, the bat would be 1.10 and the total 1.20, which is too much. Call the ball x. Then the bat is x + 1.00, so x + x + 1.00 = 1.10, which gives 2x = 0.10 and x = 0.05. Check: 0.05 + 1.05 = 1.10, and the difference is exactly 1.00. That works."},
		{say: "The ball costs **0.05**.\n\nIf the ball is *x*, the bat is *x* + 1.00, so 2*x* + 1.00 = 1.10 and *x* = 0.05. The intuitive answer, 0.10, would make the total 1.20."},
	}},
	{id: "search", chip: "Search the web", prompt: "What changed in the latest release of the Harbor runtime?", model: "Swift", steps: []chatStep{
		{tool: &chatTool{id: "search", kind: "search", state: toolPending, query: "Harbor runtime release notes", sources: []chatSource{
			{"Harbor 4.2 release notes", "harbor.dev", ui.ToneAccent},
			{"What's new in the Harbor scheduler", "blog.harbor.dev", ui.ToneInfo},
			{"Benchmarks: Harbor 4.1 vs 4.2", "perfweekly.io", ui.ToneWarn},
			{"Migrating to the new config format", "docs.harbor.dev", ui.ToneUp},
		}}, ms: 1600},
		{say: "Harbor **4.2** shipped last week. The highlights, from the release notes and two write-ups:\n\n- A new **work-stealing scheduler** that cuts tail latency by about 30 %.\n- Configuration moved to a single `harbor.toml`; the old flags still work for one more release.\n- Cold starts are roughly **2× faster** thanks to lazy module loading.\n\nThe benchmark post notes the gains are smallest on single-core machines."},
	}},
	{id: "weather", chip: "Weather in Lisbon", prompt: "Do I need a jacket in Lisbon this evening?", model: "Swift", steps: []chatStep{
		{tool: &chatTool{id: "weather", kind: "weather", state: toolPending, city: "Lisbon", temp: 21, condition: "Partly cloudy", high: 24, low: 16, hours: []int{22, 23, 24, 23, 21, 19, 17, 16}}, ms: 1000},
		{say: "It's **21°** now and drops to about **16°** after sunset, with a breeze off the river. A light jacket is a good idea for the evening."},
	}},
	{id: "code", chip: "Run some code", prompt: "How many primes are there below 10,000?", model: "Deep", steps: []chatStep{
		{think: "Counting by hand is error-prone. A sieve of Eratosthenes up to 10,000 is quick to run, so I will run one and report the count."},
		{tool: &chatTool{id: "code", kind: "code", state: toolPending, lang: "go", code: "n := 10_000\nsieve := make([]bool, n)\ncount := 0\nfor i := 2; i < n; i++ {\n\tif !sieve[i] {\n\t\tcount++\n\t\tfor j := i * i; j < n; j += i {\n\t\t\tsieve[j] = true\n\t\t}\n\t}\n}\nfmt.Println(count)", output: "1229", ms: 38}, ms: 1800},
		{say: "There are **1,229** primes below 10,000. I counted them with a sieve of Eratosthenes, which runs in a few milliseconds."},
	}},
	{id: "vision", chip: "Describe a photo", prompt: "What's in this photo? And summarize the attached notes.", model: "Vision", image: core.Src(chatPhoto), file: &fileRef{"trip-notes.pdf", "2.4 MB"}, steps: []chatStep{
		{say: "**The photo** shows a harbor at dusk: a few moored boats, low warm light on the water and a line of buildings along the far shore.\n\n**trip-notes.pdf** (4 pages) is an itinerary:\n- Day 1: arrive, walk the old town.\n- Day 2: boat tour of the harbor, the one in your photo.\n- Day 3: museum, then the evening train back."},
	}},
	{id: "error", chip: "Fail a reply", prompt: "Summarize this week's meeting notes.", model: "Swift", steps: []chatStep{
		{say: "Here's a summary of the four meetings this week. The planning session agreed on"}, {fail: "The model is overloaded right now (error 529)."},
	}},
	{id: "offline", chip: "Go offline", prompt: "Remind me what we decided about the launch date.", model: "Swift", steps: []chatStep{
		{offline: 2600}, {say: "You decided to launch on **Tuesday, November 3**, after the final review on Monday afternoon."},
	}},
}

// retried is what a retried reply says.
var retried = scenario{id: "retry", model: "Swift", steps: []chatStep{{say: "Here's a summary of the four meetings this week:\n\n1. **Planning**: the launch moves to November 3.\n2. **Design review**: the new onboarding is approved with two copy changes.\n3. **Infra sync**: the database migration runs on Saturday night.\n4. **Retro**: fewer meetings next sprint."}}}

func findScenario(id string) (scenario, bool) {
	for _, s := range scenarios {
		if s.id == id {
			return s, true
		}
	}
	return scenario{}, false
}

// freeform is a free-form prompt: the model echoes it in its own voice.
func freeform(prompt string, model chatModel) scenario {
	quote := prompt
	if r := []rune(prompt); len(r) > 60 {
		quote = string(r[:57]) + "…"
	}
	var steps []chatStep
	if model == "Deep" {
		steps = append(steps, chatStep{think: fmt.Sprintf("The user asked: %q. This is a simulated assistant, so I will explain that and show what a structured answer would look like.", quote)})
	}
	kind := "quick"
	switch model {
	case "Deep":
		kind = "careful"
	case "Vision":
		kind = "visual"
	}
	steps = append(steps, chatStep{say: fmt.Sprintf("You asked: *%s*\n\nThis assistant is simulated, so here is the shape a %s answer would take:\n\n- **A direct answer** in the first sentence.\n- **The reasoning** behind it, kept short.\n- **A next step** you can take.\n\nTry the suggestions for streaming, tools, reasoning and errors.", quote, kind)})
	return scenario{id: "free", prompt: prompt, model: model, steps: steps}
}

var pastPrompts = [][2]string{
	{"Can you suggest a name for a hiking club?", "How about **Switchback Society**, **Trail Mix** or **The Ridge Walkers**? The first one sounds the friendliest."},
	{"What's a quick dinner with eggs and spinach?", "A **frittata**: sauté the spinach, pour in six beaten eggs, cook on low for 8 minutes, then finish under the grill for 3."},
	{"Convert 3 cups of flour to grams.", "About **375 g** of all-purpose flour (125 g per cup, spooned and leveled)."},
	{"Is it 'affect' or 'effect' here: the rain will ___ the game?", "**Affect**: it's the verb. *Effect* is usually the noun, as in *the effect of the rain*."},
	{"Give me a one-line summary of photosynthesis.", "Plants use sunlight to turn water and carbon dioxide into sugar, releasing oxygen."},
	{"How do I center a view in a flex container?", "Set `AlignItems: core.AlignCenter` and `JustifyContent: core.JustifyCenter` on the container."},
	{"What's the capital of Australia?", "**Canberra**, not Sydney: it was chosen as a compromise between Sydney and Melbourne."},
	{"Write a haiku about Mondays.", "*Alarm sings too soon*\n\n*coffee steams against the glass*\n\n*the week opens slow*"},
}

// chatHistory is a long conversation over three days, for scrolling.
func chatHistory(nextID func() string) []msg {
	var out []msg
	for day, label := range []string{"Monday, October 12", "Yesterday", "Today"} {
		out = append(out, msg{kind: "date", id: nextID(), label: label})
		for i := range 20 {
			p := pastPrompts[(day*20+i)%len(pastPrompts)]
			out = append(out, msg{kind: "user", id: nextID(), text: p[0]}, msg{kind: "assistant", id: nextID(), model: "Swift", script: "history", md: p[1], status: "done"})
		}
	}
	return out
}
