// A photo stream of a thousand remote photos in a recycled List: rows on
// phones and tiles on wider screens, a page more as the end comes near,
// and a refresh by pulling down, or the button, which the mouse needs.
import { Image, List, platform, signal, Text, View, windowSize } from "nsgo";
import { Badge, Button, Icon } from "nsgo/ui";

interface Photo {
  id: number;
  title: string;
  author: string;
  views: number;
}

const authors = ["Ada", "Linus", "Grace", "Ken", "Barbara"];
const places = ["Harbor", "Ridge", "Old town", "Dunes", "Canal", "Orchard", "Quarry", "Pier", "Meadow"];

/** How many photos a page loads. */
const page = 200;

function makePhotos(from: number, n: number): Photo[] {
  return Array.from({ length: n }, (_, i) => {
    const id = from + i;
    return { id, title: `${places[id % places.length]} ${String(id).padStart(4, "0")}`, author: authors[id % authors.length], views: ((id * 7919) % 9800) + 120 };
  });
}

/** 1234 as "1.2k". */
function compact(n: number): string {
  return n >= 1000 ? `${(n / 1000).toFixed(1)}k` : String(n);
}

function Byline(props: { photo: Photo }) {
  return (
    <View class="flex-row items-center gap-1">
      <Text class="text-[12px] text-muted line-clamp-1">{props.photo.author}</Text>
      <Text class="text-[12px] text-faint">·</Text>
      <Icon name="eye" size={12} />
      <Text class="text-[12px] text-muted">{compact(props.photo.views)}</Text>
    </View>
  );
}

function PhotoRow(props: { photo: Photo }) {
  return (
    <View role="button" accessibilityLabel="Photo" class="h-[76px] flex-row items-center gap-3 px-4">
      <Image src={`https://picsum.photos/seed/${props.photo.id}/112/112`} class="h-14 w-14 rounded-[10px] border border-border bg-raised object-cover" />
      <View class="min-w-0 flex-1 gap-1 border-b border-line py-3">
        <Text class="text-[15px] font-medium text-text line-clamp-1">{props.photo.title}</Text>
        <Byline photo={props.photo} />
      </View>
    </View>
  );
}

function Tile(props: { photo: Photo }) {
  return (
    <View role="button" accessibilityLabel="Photo" class="min-w-0 flex-1 gap-2">
      <Image src={`https://picsum.photos/seed/${props.photo.id}/400/300`} class="aspect-[4/3] w-full rounded-[10px] border border-border bg-raised object-cover" />
      <View class="gap-0.5 px-0.5">
        <Text class="text-[13px] font-medium text-text line-clamp-1">{props.photo.title}</Text>
        <Byline photo={props.photo} />
      </View>
    </View>
  );
}

function Stat(props: { label: string; value: string; testID?: string }) {
  return (
    <View class="w-1/3 gap-0.5">
      <Text class="text-[11px] uppercase tracking-wide text-faint">{props.label}</Text>
      <Text testID={props.testID} class="text-[15px] font-semibold text-text">{props.value}</Text>
    </View>
  );
}

export function PhotosDemo() {
  const photos = signal(makePhotos(1, 1000));
  const refreshing = signal(false);
  const refreshes = signal(0);
  const desktop = platform.idiom !== "phone" && platform.idiom !== "pad";
  const columns = () => (windowSize().width >= 1200 ? 4 : windowSize().width >= 760 ? 3 : 1);

  const refresh = () => {
    if (refreshing()) return;
    refreshing.set(true);
    setTimeout(() => {
      photos.set(makePhotos(1, 1000));
      refreshes.set(refreshes() + 1);
      refreshing.set(false);
    }, 800);
  };
  const loadMore = () => {
    const now = photos();
    photos.set([...now, ...makePhotos(now.length + 1, page)]);
  };
  /** The rows the list recycles: a photo each, or a row of tiles. */
  const rows = () => {
    const n = columns(), all = photos(), out: Photo[][] = [];
    for (let i = 0; i < all.length; i += n) out.push(all.slice(i, i + n));
    return out;
  };

  const hero = (
    <View class="px-4 pb-4 pt-2">
      <View class="gap-3 rounded-xl border border-border bg-surface p-4">
        <View class="flex-row items-center gap-3">
          <View class="h-10 w-10 items-center justify-center rounded-xl border border-border bg-raised">
            <Icon name="image" size={22} />
          </View>
          <View class="min-w-0 flex-1 gap-0.5">
            <Text class="text-[15px] font-semibold text-text">Photo stream</Text>
            <Text class="text-[13px] text-muted">A recycled list of remote photos</Text>
          </View>
          <Badge text="Remote" variant="info" />
        </View>
        <View class="flex-row items-baseline gap-1">
          <Text testID="photos-count" class="text-[40px] font-semibold tracking-tight text-text">{photos().length.toLocaleString("en")}</Text>
          <Text class="text-[13px] font-medium text-accent">photos</Text>
        </View>
        <View class="flex-row border-t border-line pt-2.5">
          <Stat label="Pages" value={String(Math.ceil(photos().length / page))} />
          <Stat label="Per page" value={String(page)} />
          <Stat label="Refreshes" value={String(refreshes())} testID="photos-refreshes" />
        </View>
      </View>
      <View class="flex-row items-center gap-2 pt-3">
        <Icon name="arrow-down" size={14} />
        <Text class="min-w-0 flex-1 text-[13px] text-faint">
          {desktop ? `Scrolling to the end loads ${page} more` : `Pull to refresh · the end loads ${page} more`}
        </Text>
        <Button testID="photos-refresh" text={refreshing() ? "Refreshing…" : "Refresh"} variant="secondary" disabled={refreshing()} onPress={refresh} />
      </View>
    </View>
  );

  return (
    <View class="min-h-0 flex-1 bg-background">
      <List
        testID="photos"
        class="flex-1"
        each={rows()}
        keyExtractor={(row) => `${row[0].id}-${row.length}`}
        refreshing={refreshing()}
        onRefresh={refresh}
        onLoadMore={loadMore}
        header={hero}
      >
        {(row) => (
          <If when={columns() > 1} else={<PhotoRow photo={row()[0]} />}>
            <View class="flex-row gap-4 px-4 pb-4">
              {row().map((p) => <Tile photo={p} />)}
              {Array.from({ length: columns() - row().length }, () => <View class="flex-1" />)}
            </View>
          </If>
        )}
      </List>
    </View>
  );
}
