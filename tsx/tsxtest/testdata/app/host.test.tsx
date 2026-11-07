import { expect, open, test } from "nsgo/test";

const host = {
  params: { user: "ada" },
  handlers: {
    greet: (data) => "hello " + data.who,
    later: async () => "waited",
    fixed: 7,
    broken: () => { throw new Error("no"); },
    note: null,
  },
};

test("plays the embedding app", async () => {
  const app = await open("Host.tsx", { host });
  await app.press("id:greet");
  await app.waitFor("said hello ada");
  await app.press("id:later");
  await app.waitFor("said waited");
  await app.press("id:fixed");
  await app.waitFor("said 7");
  await app.press("id:broken");
  await app.waitFor("said failed: no");
  await app.press("id:missing");
  expect(app.texts().join(" ")).toContain("failed");
  await app.press("id:post");
  expect(app.host.posts()).toEqual([{ name: "note", data: "hello" }]);
  await app.host.send("tick", 3);
  await app.waitFor("event 3");
});
