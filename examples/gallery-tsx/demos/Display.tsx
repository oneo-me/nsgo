import { Image as PlainImage, signal, Text, token, View } from "nsgo";
import { Avatar, AvatarGroup, Badge, Button, Carousel, Image, ImageCompare } from "nsgo/ui";
import { BoolKnob, ChoiceKnob, NumberKnob, Page, Playground, Row } from "../parts";

const badgeVariants = ["neutral", "accent", "info", "warn", "danger", "up", "down", "outline"] as const;

export function BadgeDemo() {
  return (
    <Page>
      <Playground title="Badge">
        <Row>
          <Each of={badgeVariants}>{(v) => <Badge text={v()} variant={v()} />}</Each>
        </Row>
      </Playground>
    </Page>
  );
}

const people = ["Ada Lovelace", "Grace Hopper", "Alan Turing", "Edsger Dijkstra", "Barbara Liskov", "Donald Knuth"];

export function AvatarDemo() {
  const size = signal(40);
  const status = signal<"none" | "online" | "offline">("online");
  const max = signal(4);
  return (
    <Page>
      <Playground
        title="Avatar"
        knobs={
          <>
            <NumberKnob name="Size" value={size} min={24} max={72} step={4} />
            <ChoiceKnob name="Status" value={status} options={["none", "online", "offline"]} />
          </>
        }
      >
        <Row>
          <Avatar name="Ada Lovelace" size={size()} status={status()} />
          <Avatar name="Grace Hopper" size={size()} status={status()} />
        </Row>
      </Playground>
      <Playground title="AvatarGroup" knobs={<NumberKnob name="Max" value={max} min={2} max={6} />}>
        <AvatarGroup
          accessibilityLabel="Team"
          max={max()}
          size={32}
          members={people.map((name) => ({ name }))}
        />
      </Playground>
    </Page>
  );
}

const photo = (tone: string) =>
  `data:image/svg+xml;utf8,<svg xmlns="http://www.w3.org/2000/svg" width="480" height="300"><defs><linearGradient id="g" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="${tone}"/><stop offset="1" stop-color="%23111827"/></linearGradient></defs><rect width="480" height="300" fill="url(%23g)"/><circle cx="360" cy="80" r="42" fill="%23fde68a"/></svg>`;

export function ImageDemo() {
  const mode = signal<"cover" | "contain" | "stretch" | "center">("cover");
  const position = signal(0.5);
  return (
    <Page>
      <Playground title="Image" fill knobs={<ChoiceKnob name="ResizeMode" value={mode} options={["cover", "contain", "stretch", "center"]} />}>
        <Image source={photo("%233b82f6")} resizeMode={mode()} aspectRatio={1.6} accessibilityLabel="A sunrise" />
      </Playground>
      <Playground title="ImageCompare" note="Drag the divider between the two images." fill>
        <ImageCompare
          accessibilityLabel="Before and after"
          beforeAccessibilityLabel="Before"
          afterAccessibilityLabel="After"
          aspectRatio={1.6}
          position={position()}
          onChange={({ position: p }) => position.set(p)}
          before={<Image source={photo("%23ef4444")} aspectRatio={1.6} />}
          after={<Image source={photo("%2310b981")} aspectRatio={1.6} />}
        />
      </Playground>
    </Page>
  );
}

const slides = ["Mountains", "Rivers", "Forests", "Deserts", "Oceans"];

export function CarouselDemo() {
  const index = signal(0);
  const looping = signal(false);
  const width = signal(0);
  return (
    <Page>
      <Playground
        title="Carousel"
        fill
        knobs={
          <>
            <BoolKnob name="Looping" value={looping} />
            <NumberKnob name="SlideWidth" value={width} min={0} max={300} step={20} />
          </>
        }
      >
        <Carousel
          accessibilityLabel="Places"
          previousAccessibilityLabel="Previous"
          nextAccessibilityLabel="Next"
          count={slides.length}
          index={index()}
          onChange={({ index: i }) => index.set(i)}
          looping={looping()}
          slideWidth={width()}
          slide={(i) => (
            <View class="h-44 items-center justify-center rounded-lg bg-inset">
              <Text class="text-[15px] font-semibold text-text">{slides[i]}</Text>
            </View>
          )}
        />
      </Playground>
    </Page>
  );
}
