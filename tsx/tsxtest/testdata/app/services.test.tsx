import { expect, open, test } from "nsgo/test";

test("stubs the network", async () => {
  const app = await open("Services.tsx", { network: { "POST https://api.example.com/todos*": { status: 201, body: [1, 2] } } });
  await app.press("id:fetch");
  await app.waitFor("201 2");
  expect(app.requests()).toEqual([{ method: "POST", url: "https://api.example.com/todos?page=1",
    headers: { "content-type": "text/plain;charset=UTF-8" }, body: "new" }]);
  await app.press("id:offline");
  await app.waitFor("offline");
});

test("answers dialogs and picks", async () => {
  const app = await open("Services.tsx");
  app.onDialog(d => { expect(d.title).toBe("Delete?"); return "Delete"; });
  await app.press("id:ask");
  await app.waitFor("chose Delete");
  app.onPick(() => [{ name: "cat.png", data: new Uint8Array([1, 2, 3]) }]);
  await app.press("id:pick");
  await app.waitFor("cat.png:image/png:3");
  expect(app.picks()).toEqual([{ multiple: true, limit: 3, maxDimension: null, quality: null, format: "compatible" }]);
});

test("records URLs, the clipboard and menus", async () => {
  const app = await open("Services.tsx", { clipboard: "before", platform: "macos" });
  expect(app.clipboard()).toBe("before");
  await app.press("id:copy");
  expect(app.clipboard()).toBe("copied");
  await app.press("id:open");
  expect(app.openedUrls()).toEqual(["https://example.com/docs"]);
  await app.press("id:menu");
  expect(app.menu()).toEqual([{ label: "Rename", disabled: false }, { label: "Share > Mail", disabled: false, checked: true }]);
  await app.chooseMenuItem("Share > Mail");
  await app.waitFor("mailed");
  expect(app.menu()).toBeNull();
  expect(app.spanStyle("text:mailed").fontWeight).toBe(400);
});

test("answers requests with the test's functions", async () => {
  const app = await open("Services.tsx", { network: {
    "POST https://api.example.com/todos*": (req) => ({ status: 202, body: [req.body, req.method] }),
    "GET https://elsewhere.example.com/": async () => ({ error: "unreachable" }),
  } });
  await app.press("id:fetch");
  await app.waitFor("202 2");
  await app.press("id:offline");
  await app.waitFor("offline");
});
