// A live market of made-up stocks: a watchlist whose prices tick each
// second with a sparkline each, and a stock's page with its chart, its
// day's numbers and an order ticket. On phones the page goes over the list.
import { cleanup, List, signal, Text, token, View, windowSize } from "nsgo";
import { Area, Line, Plot, PlotAxis, Grid } from "nsgo/base/plot";
import { Badge, Button, IconButton, NumberInput, Tabs } from "nsgo/ui";
import { toasts } from "../toast";

interface Stock {
  symbol: string;
  name: string;
  open: number;
  prices: number[];
  volume: number;
}

const listed: [string, string, number][] = [
  ["ORCA", "Orca Robotics", 182.4], ["LUMN", "Lumen Grid", 64.1], ["TIDE", "Tidewater Labs", 23.75], ["VOLT", "Volta Motors", 241.9],
  ["PINE", "Pinecone Foods", 41.2], ["NOVA", "Nova Optics", 118.6], ["KITE", "Kite Air", 12.9], ["SAGE", "Sage Health", 76.3],
  ["BOLT", "Bolt Logistics", 33.4], ["QUIL", "Quill Media", 9.85],
];

/** A seeded random walk, so the market starts the same each time. */
function mulberry(seed: number) {
  return () => {
    seed |= 0;
    seed = (seed + 0x6d2b79f5) | 0;
    let t = Math.imul(seed ^ (seed >>> 15), 1 | seed);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}
const rand = mulberry(42);

function walk(p: number): number {
  return Math.max(0.5, Math.round(p * (1 + (rand() - 0.5) * 0.012) * 100) / 100);
}

function makeMarket(): Stock[] {
  return listed.map(([symbol, name, price]) => {
    const prices = [price];
    for (let i = 1; i < 90; i++) prices.push(walk(prices[i - 1]));
    return { symbol, name, open: prices[0], prices, volume: Math.round(2e5 + rand() * 4e6) };
  });
}

const last = (s: Stock) => s.prices[s.prices.length - 1];
const change = (s: Stock) => (last(s) - s.open) / s.open;
const money = (n: number) => n.toLocaleString("en", { style: "currency", currency: "USD" });
const percent = (n: number) => `${n >= 0 ? "+" : ""}${(n * 100).toFixed(2)}%`;

function Sparkline(props: { stock: Stock }) {
  const up = () => change(props.stock) >= 0;
  return (
    <Plot data={props.stock.prices.slice(-40).map((p, i) => ({ i, p }))} x="i" y="p" class="h-7 w-[72px]" accessibilityLabel={`${props.stock.symbol} today`}>
      <Line style={{ stroke: up() ? token("success") : token("danger"), strokeWidth: 1.5 }} />
    </Plot>
  );
}

function Row(props: { stock: Stock; active: boolean; onPress: () => void }) {
  const up = () => change(props.stock) >= 0;
  return (
    <View testID={`stock-${props.stock.symbol}`} role="button" accessibilityLabel={props.stock.name} selected={props.active}
      class="h-14 flex-row items-center gap-3 border-b border-line px-4 selected:bg-inset" onPress={props.onPress}>
      <View class="min-w-0 flex-1 gap-0.5">
        <Text class="text-[14px] font-semibold text-text">{props.stock.symbol}</Text>
        <Text class="text-[12px] text-muted line-clamp-1">{props.stock.name}</Text>
      </View>
      <Sparkline stock={props.stock} />
      <View class="w-[92px] items-end gap-0.5">
        <Text testID={`price-${props.stock.symbol}`} class="font-mono text-[14px] text-text">{money(last(props.stock))}</Text>
        <Badge text={percent(change(props.stock))} variant={up() ? "up" : "down"} />
      </View>
    </View>
  );
}

function Fact(props: { label: string; value: string }) {
  return (
    <View class="w-1/4 min-w-[110px] gap-0.5 py-1.5">
      <Text class="text-[11px] uppercase tracking-wide text-faint">{props.label}</Text>
      <Text class="font-mono text-[13px] text-text">{props.value}</Text>
    </View>
  );
}

function Ticket(props: { stock: Stock }) {
  const side = signal<"buy" | "sell">("buy");
  const shares = signal(10);
  const place = () => {
    toasts()?.show({
      type: "success",
      title: `${side() === "buy" ? "Bought" : "Sold"} ${shares()} ${props.stock.symbol}`,
      description: `At ${money(last(props.stock))}, ${money(shares() * last(props.stock))} in all.`,
      duration: 4000,
    });
  };
  return (
    <View testID="ticket" class="gap-3 rounded-xl border border-border bg-surface p-4">
      <Text class="text-[13px] font-bold text-text">Order</Text>
      <Tabs testID="ticket-side" accessibilityLabel="Side" equal options={[{ value: "buy", label: "Buy" }, { value: "sell", label: "Sell" }]}
        value={side()} onChange={({ value }) => side.set(value as "buy" | "sell")} />
      <View class="flex-row items-center gap-3">
        <Text class="min-w-0 flex-1 text-[13px] text-muted">Shares</Text>
        <NumberInput testID="ticket-shares" accessibilityLabel="Shares" value={shares()} onChange={({ value }) => shares.set(value)} min={1} max={10000} step={1} />
      </View>
      <View class="flex-row items-center border-t border-line pt-3">
        <Text class="min-w-0 flex-1 text-[13px] text-muted">Estimated total</Text>
        <Text testID="ticket-total" class="font-mono text-[14px] font-semibold text-text">{money(shares() * last(props.stock))}</Text>
      </View>
      <Button testID="ticket-place" text={`${side() === "buy" ? "Buy" : "Sell"} ${props.stock.symbol}`} variant={side() === "buy" ? "primary" : "secondary"} onPress={place} />
    </View>
  );
}

function Detail(props: { stock: Stock; running: boolean; onToggle: () => void; onBack?: () => void }) {
  const s = () => props.stock;
  const up = () => change(s()) >= 0;
  const data = () => s().prices.map((p, i) => ({ i, p }));
  const high = () => Math.max(...s().prices), low = () => Math.min(...s().prices);
  return (
    <View testID="stock-detail" class="min-h-0 flex-1 gap-4 p-4">
      <View class="flex-row items-center gap-3">
        <If when={props.onBack}>
          <IconButton testID="stock-back" icon="chevron-left" accessibilityLabel="Back" onPress={() => props.onBack?.()} />
        </If>
        <View class="min-w-0 flex-1 gap-0.5">
          <Text testID="stock-name" class="text-[20px] font-semibold text-text">{s().name}</Text>
          <Text class="text-[12px] text-muted">{s().symbol} · Made-up market</Text>
        </View>
        <Button testID="market-pause" text={props.running ? "Pause" : "Resume"} variant="secondary" size="small" onPress={props.onToggle} />
      </View>
      <View class="flex-row items-baseline gap-2">
        <Text class="font-mono text-[32px] font-semibold text-text">{money(last(s()))}</Text>
        <Text class="font-mono text-[14px]" style={{ color: up() ? token("success") : token("danger") }}>{percent(change(s()))}</Text>
      </View>
      <View class="min-h-[180px] flex-1 rounded-xl border border-border bg-surface p-3">
        <Plot testID="stock-chart" data={data()} x="i" y="p" class="flex-1" accessibilityLabel={`${s().symbol} price`} interactive
          yScale={{ type: "linear", domain: [low() * 0.995, high() * 1.005] }}>
          <Grid ticks={4} style={{ stroke: token("line") }} />
          <PlotAxis placement="right" ticks={4} labelClass="text-[11px] text-muted" format={(v) => Number(v).toFixed(2)} />
          <Area backgroundStyle={{ backgroundColor: `${up() ? token("success") : token("danger")}22` }} />
          <Line style={{ stroke: up() ? token("success") : token("danger"), strokeWidth: 2 }} />
        </Plot>
      </View>
      <View class="flex-row flex-wrap">
        <Fact label="Open" value={money(s().open)} />
        <Fact label="High" value={money(high())} />
        <Fact label="Low" value={money(low())} />
        <Fact label="Volume" value={s().volume.toLocaleString("en")} />
      </View>
      <Ticket stock={s()} />
    </View>
  );
}

export function StocksDemo() {
  const market = signal(makeMarket());
  const selected = signal<string | null>(null);
  const running = signal(true);
  const wide = () => windowSize().width >= 900;
  const timer = setInterval(() => {
    if (!running()) return;
    market.set(market().map((s) => ({ ...s, prices: [...s.prices.slice(1), walk(last(s))], volume: s.volume + Math.round(rand() * 4000) })));
  }, 1000);
  cleanup(() => clearInterval(timer));
  const find = (symbol: string | null) => market().find((s) => s.symbol === symbol);
  const shown = () => find(selected()) ?? (wide() ? market()[0] : undefined);

  const list = (
    <View testID="watchlist" class="min-h-0" classList={{ "w-[360px] shrink-0 border-r border-line": wide(), "flex-1": !wide() }}>
      <View class="flex-row items-center gap-2 px-4 pb-2 pt-4">
        <Text class="min-w-0 flex-1 text-[13px] font-bold text-text">Watchlist</Text>
        <Text testID="market-breadth" class="text-[12px] text-muted">
          {`${market().filter((s) => change(s) >= 0).length} up · ${market().filter((s) => change(s) < 0).length} down`}
        </Text>
      </View>
      <List each={market()} keyExtractor={(s) => s.symbol} class="flex-1">
        {(s) => <Row stock={s()} active={shown()?.symbol === s().symbol} onPress={() => selected.set(s().symbol)} />}
      </List>
    </View>
  );
  return (
    <View class="min-h-0 flex-1 flex-row bg-background">
      <If when={wide() || !shown()}>{list}</If>
      <If when={shown()}>
        <Detail stock={shown()!} running={running()} onToggle={() => running.set(!running())}
          onBack={wide() ? undefined : () => selected.set(null)} />
      </If>
    </View>
  );
}
