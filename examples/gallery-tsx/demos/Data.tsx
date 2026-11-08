import { computed, signal, Text, token, View } from "nsgo";
import { DataTable, Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "nsgo/base";
import { Area, Bar, Grid, Line, Pie, Plot, PlotAxis } from "nsgo/base/plot";
import { Badge, CommitGraph, Icon, List, Tabs, Tree } from "nsgo/ui";
import { BoolKnob, ChoiceKnob, Events, Page, Playground, Row, StagePage, Status, useEvents } from "../parts";

const tasks = Array.from({ length: 200 }, (_, i) => ({ id: `task-${i}`, title: `Task ${i + 1}`, done: i % 3 === 0 }));

export function ListDemo() {
  const { last, log } = useEvents();
  const multiple = signal(false);
  const selected = signal<string[]>([]);
  return (
    <Events value={last()}>
      <Playground
        title="List"
        fill
        knobs={
          <>
            <BoolKnob name="Multiple" value={multiple} />
            <Status label="Selected" value={selected().join(", ")} testID="list-selected" />
          </>
        }
      >
        <List
          testID="list"
          accessibilityLabel="Tasks"
          count={tasks.length}
          key={(i) => tasks[i].id}
          rowAccessibilityLabel={(i) => tasks[i].title}
          multiple={multiple()}
          tapSelects
          selected={selected()}
          onSelectedChange={({ selected: s }) => selected.set(s)}
          onActivate={({ index }) => log(`Activated ${tasks[index].title}`)()}
          estimatedSize={32}
          style={{ height: 240 }}
          row={(i) => (
            <View class="h-8 flex-row items-center gap-2 px-2">
              <Text class="flex-1 text-[13px] text-text">{tasks[i].title}</Text>
              <If when={tasks[i].done}>
                <Badge text="done" variant="up" />
              </If>
            </View>
          )}
        />
      </Playground>
      <Playground title="Tree" fill>
        <Tree
          testID="tree"
          accessibilityLabel="Files"
          toggleAccessibilityLabel="Expand"
          defaultExpanded={["source"]}
          onActivate={({ id }) => log(`Activated ${id}`)()}
          style={{ height: 220 }}
          items={[
            {
              id: "source",
              text: "source",
              icon: "folder",
              children: [
                { id: "source/app", text: "app", icon: "folder", children: [{ id: "main.go", text: "main.go", icon: "file" }] },
                { id: "source/lib", text: "lib", icon: "folder", children: [{ id: "store.go", text: "store.go", icon: "file" }] },
              ],
            },
            { id: "docs", text: "docs", icon: "folder", children: [{ id: "guide.md", text: "guide.md", icon: "file" }] },
            { id: "README.md", text: "README.md", icon: "file" },
          ]}
        />
      </Playground>
    </Events>
  );
}

const invoices = [
  { id: "INV-001", status: "Paid", method: "Card", amount: "$250.00" },
  { id: "INV-002", status: "Pending", method: "Transfer", amount: "$150.00" },
  { id: "INV-003", status: "Unpaid", method: "Card", amount: "$350.00" },
  { id: "INV-004", status: "Paid", method: "Cash", amount: "$450.00" },
];

export function TableDemo() {
  const head = "h-8 justify-center border-b border-line px-3";
  const cell = "h-9 justify-center border-b border-line px-3";
  const headText = "text-[12px] font-semibold text-muted";
  const cellText = "text-[13px] text-text";
  return (
    <Page>
      <Playground title="Table" start>
        <Table accessibilityLabel="Invoices" class="w-full">
          <TableHeader>
            <TableRow>
              <TableHead class={head}><Text class={headText}>Invoice</Text></TableHead>
              <TableHead class={head}><Text class={headText}>Status</Text></TableHead>
              <TableHead class={head}><Text class={headText}>Method</Text></TableHead>
              <TableHead class={head}><Text class={headText}>Amount</Text></TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <Each of={invoices} key={(r) => r.id}>
              {(r) => (
                <TableRow>
                  <TableCell class={cell}><Text class={cellText}>{r().id}</Text></TableCell>
                  <TableCell class={cell}><Text class={cellText}>{r().status}</Text></TableCell>
                  <TableCell class={cell}><Text class={cellText}>{r().method}</Text></TableCell>
                  <TableCell class={cell}><Text class={cellText}>{r().amount}</Text></TableCell>
                </TableRow>
              )}
            </Each>
          </TableBody>
        </Table>
      </Playground>
    </Page>
  );
}

const tickers = Array.from({ length: 2000 }, (_, i) => ({
  symbol: `T${String(i).padStart(4, "0")}`,
  price: Math.round((((i * 37) % 900) / 3 + 10) * 100) / 100,
  change: (((i * 13) % 200) - 100) / 10,
  volume: ((i * 7919) % 90000) + 1000,
}));

export function DataTableDemo() {
  const stripe = signal(true);
  const movable = signal(true);
  const loading = signal(false);
  const sort = signal<{ key: string; direction?: "ascending" | "descending" }>({ key: "symbol" });
  const sorted = computed(() => {
    const { key, direction } = sort();
    const sign = direction === "descending" ? -1 : 1;
    return [...tickers].sort((a, b) => {
      const x = a[key as keyof typeof a];
      const y = b[key as keyof typeof b];
      return (x < y ? -1 : x > y ? 1 : 0) * sign;
    });
  });
  return (
    <StagePage
      panel={
        <>
          <BoolKnob name="Stripe" value={stripe} />
          <BoolKnob name="Movable" value={movable} />
          <BoolKnob name="Loading" value={loading} />
          <Status label="Sort" value={`${sort().key} ${sort().direction ?? "ascending"}`} testID="table-sort" />
        </>
      }
    >
      <View class="flex-1 overflow-hidden rounded-lg border border-border bg-surface">
        <DataTable
          testID="table"
          accessibilityLabel="Tickers"
          class="flex-1"
          stripe={stripe()}
          movableColumns={movable()}
          loading={loading()}
          sort={sort()}
          onSortChange={({ sort: s }) => sort.set(s)}
          columns={[
            { key: "symbol", title: "Symbol", width: 96, sortable: true, fixed: true },
            { key: "price", title: "Price", width: 96, sortable: true, align: "end" },
            { key: "change", title: "Change", width: 96, sortable: true, align: "end", positiveClass: "text-success", negativeClass: "text-down" },
            { key: "volume", title: "Volume", width: 110, sortable: true, align: "end" },
          ]}
          data={sorted()}
        />
      </View>
    </StagePage>
  );
}

const day = 86400000;
const commits = [
  { hash: "c6", parents: ["c5", "b2"], short: "6c209de", subject: "Merge the theme work", author: "Ada", refs: [{ name: "main", kind: "head" as const, current: true }] },
  { hash: "b2", parents: ["b1"], short: "aa79a9b", subject: "Color the icons with the theme", author: "Grace", refs: [{ name: "theme", kind: "branch" as const }] },
  { hash: "c5", parents: ["c4"], short: "dc89a56", subject: "Add a commit graph", author: "Ada" },
  { hash: "b1", parents: ["c4"], short: "743b5f8", subject: "Rebuild the color picker", author: "Grace" },
  { hash: "c4", parents: ["c3"], short: "89a44d3", subject: "Collapse breadcrumbs by width", author: "Alan", refs: [{ name: "v0.3.0", kind: "tag" as const }] },
  { hash: "c3", parents: ["c2"], short: "66dbbaa", subject: "Page looping carousels", author: "Ada" },
  { hash: "c2", parents: ["c1"], short: "d9b2890", subject: "Mark choices with the selected row", author: "Alan" },
  { hash: "c1", parents: [], short: "a213a64", subject: "Keep calendars one size", author: "Grace" },
].map((c, i) => ({ ...c, date: new Date(Date.UTC(2026, 10, 8) - i * day).toISOString() }));

export function CommitGraphDemo() {
  const { last, log } = useEvents();
  return (
    <StagePage panel={<Status label="Activated" value={last()} testID="commit-activated" />}>
      <View class="flex-1 overflow-hidden rounded-lg border border-border bg-surface">
        <CommitGraph
          testID="commits"
          accessibilityLabel="History"
          commits={commits}
          onActivate={({ commit }) => log(commit.subject ?? "")()}
          style={{ flexGrow: 1 }}
        />
      </View>
    </StagePage>
  );
}

const week = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"].map((d, i) => ({
  day: d,
  visits: 1200 + Math.round(Math.sin(i) * 500) + i * 90,
  last: 1000 + Math.round(Math.cos(i) * 400) + i * 60,
}));

const shares = [
  { name: "macOS", value: 46 },
  { name: "iOS", value: 28 },
  { name: "Web", value: 16 },
  { name: "Other", value: 10 },
];

export function PlotDemo() {
  const kind = signal<"line" | "area" | "bar" | "pie">("line");
  const grid = signal(true);
  const tones = () => [token("accentFill"), token("info"), token("warning"), token("success")].map(String);
  const axis = "text-[12px] text-muted";
  return (
    <StagePage
      panel={
        <>
          <ChoiceKnob name="Mark" value={kind} options={["line", "area", "bar", "pie"]} />
          <BoolKnob name="Grid" value={grid} />
        </>
      }
    >
      <View class="flex-1 rounded-lg border border-border bg-surface p-4">
        <If
          when={kind() !== "pie"}
          else={
            <Plot data={shares} class="flex-1" accessibilityLabel="Share by platform" colorScale={{ type: "ordinal", range: tones() }}>
              <Pie value="value" color="name" />
            </Plot>
          }
        >
          <Plot
            testID="plot"
            data={week}
            x="day"
            y="visits"
            class="flex-1"
            accessibilityLabel="Visits"
            interactive
            xScale={{ type: kind() === "bar" ? "band" : "point" }}
            yScale={{ type: "linear", domain: [0, 2500] }}
            colorScale={{ type: "ordinal", range: tones() }}
          >
            <If when={grid()}>
              <Grid ticks={5} style={{ stroke: token("line") }} />
            </If>
            <PlotAxis placement="left" ticks={5} labelClass={axis} />
            <PlotAxis placement="bottom" labelClass={axis} format={(_, i) => week[i]?.day ?? ""} />
            <If when={kind() === "area"}>
              <Area curve="natural" backgroundStyle={{ backgroundColor: `${token("accentFill")}26` }} />
            </If>
            <If when={kind() !== "bar"}>
              <Line curve="natural" dot={kind() === "line"} style={{ stroke: token("accentFill"), strokeWidth: 2 }} />
            </If>
            <If when={kind() === "bar"}>
              <Bar value="visits" cornerRadii={4} />
            </If>
          </Plot>
        </If>
      </View>
    </StagePage>
  );
}
