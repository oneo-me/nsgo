import { describe, expect, open, test, beforeEach } from "nsgo/test";

let opened = 0;
beforeEach(() => { opened++; });

describe("the counter", () => {
  test("counts presses", async () => {
    const app = await open("App.tsx");
    expect(app.shows("Pressed 0")).toBe(true);
    await app.press("role:button[Increment]");
    await app.press("id:count");
    expect(app.texts()).toContain("Pressed 2");
    expect(opened).toBeGreaterThan(0);
  });
});

test("types and saves", async () => {
  const app = await open("App.tsx", { storage: { kept: "1" } });
  await app.type("id:name", "Ada");
  await app.waitFor("Hello Ada");
  await app.press("id:save");
  await app.wait(10);
  expect(app.storage()).toEqual({ kept: "1", name: "Ada" });
  expect(() => { throw new Error("boom"); }).toThrow("boom");
  expect(app.value("id:name")).toBe("Ada");
});

test.skip("is skipped", () => { throw new Error("ran"); });
