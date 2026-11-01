// The tasks screen, tested from TSX as a user acts on it.
import { expect, open, test } from "nsgo/test";

test("adds a task and filters the done ones", async () => {
  const app = await open("App.tsx");
  await app.type("id:draft", "Buy milk");
  await app.press("text:Add");
  expect(app.shows("Buy milk")).toBe(true);
  await app.press("id:check-1");
  await app.press("text:Done");
  expect(app.shows("Buy milk")).toBe(false);
});
