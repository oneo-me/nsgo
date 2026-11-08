import { signal, Text, View } from "nsgo";
import { createNavStack, NavPage, NavStack } from "nsgo/base";
import { Breadcrumbs, Button, NavHeader, Pagination, Sidebar, Stepper, Tabs } from "nsgo/ui";
import { BoolKnob, ChoiceKnob, Events, NumberKnob, Page, Playground, Row, StagePage, useEvents } from "../parts";

const sections = [
  { value: "account", label: "Account", icon: "user" },
  { value: "billing", label: "Billing", icon: "credit-card" },
  { value: "team", label: "Team", icon: "users" },
];

export function TabsDemo() {
  const tab = signal("account");
  const variant = signal<"segmented" | "underline">("segmented");
  const size = signal<"small" | "default" | "large">("default");
  const equal = signal(false);
  return (
    <Page>
      <Playground
        title="Tabs"
        fill={equal()}
        knobs={
          <>
            <ChoiceKnob name="Variant" value={variant} options={["segmented", "underline"]} />
            <ChoiceKnob name="Size" value={size} options={["small", "default", "large"]} />
            <BoolKnob name="Equal" value={equal} />
          </>
        }
      >
        <View class="items-stretch gap-3">
          <Tabs
            testID="tabs"
            accessibilityLabel="Settings"
            options={sections}
            value={tab()}
            onChange={({ value }) => tab.set(value)}
            variant={variant()}
            size={size()}
            equal={equal()}
          />
          <Text class="text-center text-[13px] text-muted">The {tab()} tab.</Text>
        </View>
      </Playground>
    </Page>
  );
}

export function BreadcrumbsDemo() {
  const { last, log } = useEvents();
  const path = signal(["Home", "Projects", "NSGo", "Gallery", "Components"]);
  const page = signal(3);
  const total = signal(12);
  const compact = signal(false);
  const step = signal(1);
  const vertical = signal(false);
  return (
    <Events value={last()}>
      <Playground title="Breadcrumbs" note="Collapses by the width it has." fill>
        <Breadcrumbs items={path()} onSelect={({ index }) => log(`Went to ${path()[index]}`)()} />
      </Playground>
      <Playground
        title="Pagination"
        knobs={
          <>
            <NumberKnob name="TotalPages" value={total} min={1} max={30} />
            <BoolKnob name="Compact" value={compact} />
          </>
        }
      >
        <Pagination page={page()} totalPages={total()} compact={compact()} onChange={({ page: p }) => page.set(p)} accessibilityLabel="Pages" />
      </Playground>
      <Playground
        title="Stepper"
        fill
        knobs={
          <>
            <NumberKnob name="Current" value={step} min={0} max={3} />
            <BoolKnob name="Vertical" value={vertical} />
          </>
        }
      >
        <Stepper
          accessibilityLabel="Checkout"
          vertical={vertical()}
          current={step()}
          onSelect={({ step: s }) => step.set(s)}
          steps={[{ label: "Cart" }, { label: "Address", description: "Where it goes" }, { label: "Payment" }, { label: "Done" }]}
        />
      </Playground>
    </Events>
  );
}

export function SidebarDemo() {
  const selected = signal("inbox");
  return (
    <Page>
      <Playground title="Sidebar" fill>
        <View class="h-72 rounded-lg border border-border p-2">
          <Sidebar
            testID="demo-sidebar"
            selected={selected()}
            onSelect={({ id }) => selected.set(id)}
            sections={[
              {
                title: "Mail",
                collapsible: true,
                items: [
                  { id: "inbox", text: "Inbox", icon: "inbox" },
                  { id: "sent", text: "Sent", icon: "send" },
                  { id: "trash", text: "Trash", icon: "trash" },
                ],
              },
              { title: "Labels", collapsible: true, items: [{ id: "work", text: "Work", icon: "tag" }, { id: "home", text: "Home", icon: "tag" }] },
            ]}
          />
        </View>
      </Playground>
    </Page>
  );
}

export function NavStackDemo() {
  const nav = createNavStack<number>([1]);
  return (
    <StagePage>
      <View class="flex-1 overflow-hidden rounded-lg border border-border">
        <NavStack state={nav} class="flex-1" testID="stack">
          {(n) => (
            <NavPage
              title={`Page ${n}`}
              testID={`page-${n}`}
              header={<NavHeader title={`Page ${n}`} onBack={n > 1 ? () => nav.pop() : undefined} backAccessibilityLabel="Back" />}
            >
              <View class="flex-1 items-center justify-center gap-3 bg-background">
                <Text class="text-[28px] font-bold text-text">{n}</Text>
                <Row>
                  <Button text="Push" onPress={() => nav.push(n + 1)} />
                  <Button text="Pop to root" variant="secondary" disabled={n === 1} onPress={() => nav.popToRoot()} />
                </Row>
              </View>
            </NavPage>
          )}
        </NavStack>
      </View>
    </StagePage>
  );
}
