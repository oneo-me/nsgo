// nsgo/test's registry and expectations (NSH's nsh/test module): test
// files declare tests, Go runs them one by one, and expect throws what
// failed. Called with the natives tsxtest.go gives it.
(function (native) {
  "use strict";
  const tests = [];
  const stack = [{ name: "", before: [], after: [], beforeAll: [], afterAll: [], skip: false }];
  let only = false;
  const top = () => stack[stack.length - 1];
  const add = (name, body, mode) => {
    if (mode === "only") only = true;
    tests.push({ name: stack.slice(1).map((s) => s.name).concat(name).join(" › "), body, mode,
      suites: stack.slice(), skip: mode === "skip" || mode === "todo" || stack.some((s) => s.skip) });
  };
  const test = (name, body) => add(name, body, "");
  test.skip = (name, body) => add(name, body, "skip");
  test.only = (name, body) => add(name, body, "only");
  test.todo = (name) => add(name, null, "todo");
  const describe = (name, body, skip, solo) => {
    stack.push({ name, before: [], after: [], beforeAll: [], afterAll: [], skip: !!skip, only: !!solo });
    try {
      body();
    } finally {
      stack.pop();
    }
  };
  describe.skip = (name, body) => describe(name, body, true);
  describe.only = (name, body) => {
    only = true;
    describe(name, body, false, true);
  };

  const show = (v) => {
    try {
      return typeof v === "string" ? JSON.stringify(v) : JSON.stringify(v) ?? String(v);
    } catch {
      return String(v);
    }
  };
  const equal = (a, b) => {
    if (Object.is(a, b)) return true;
    if (typeof a !== "object" || typeof b !== "object" || a === null || b === null) return false;
    if (Array.isArray(a) !== Array.isArray(b)) return false;
    const ka = Object.keys(a), kb = Object.keys(b);
    return ka.length === kb.length && ka.every((k) => equal(a[k], b[k]));
  };
  const matches = (message, pattern) =>
    pattern == null || (pattern instanceof RegExp ? pattern.test(message) : message.includes(String(pattern)));
  const expect = (value, negate = false) => {
    const check = (ok, what) => {
      if (ok === negate) throw new Error(`expected ${show(value)} ${negate ? "not " : ""}${what}`);
    };
    return {
      get not() {
        return expect(value, !negate);
      },
      get rejects() {
        return {
          async toThrow(pattern) {
            try {
              await (typeof value === "function" ? value() : value);
            } catch (e) {
              if (matches(String(e?.message ?? e), pattern)) return;
              throw new Error(`expected a rejection matching ${show(pattern)}, got ${show(e?.message ?? e)}`);
            }
            throw new Error("expected a rejection");
          },
        };
      },
      toBe: (x) => check(Object.is(value, x), `to be ${show(x)}`),
      toEqual: (x) => check(equal(value, x), `to equal ${show(x)}`),
      toContain: (x) => check(typeof value === "string" ? value.includes(x) : [...value].some((v) => equal(v, x)), `to contain ${show(x)}`),
      toMatch: (p) => check(p instanceof RegExp ? p.test(value) : String(value).includes(p), `to match ${show(String(p))}`),
      toHaveLength: (n) => check(value?.length === n, `to have length ${n}`),
      toBeNull: () => check(value === null, "to be null"),
      toBeTruthy: () => check(!!value, "to be truthy"),
      toBeFalsy: () => check(!value, "to be falsy"),
      toBeGreaterThan: (n) => check(value > n, `to be greater than ${n}`),
      toBeGreaterThanOrEqual: (n) => check(value >= n, `to be at least ${n}`),
      toBeLessThan: (n) => check(value < n, `to be less than ${n}`),
      toBeLessThanOrEqual: (n) => check(value <= n, `to be at most ${n}`),
      toThrow: (pattern) => {
        let thrown = false, message = "";
        try {
          value();
        } catch (e) {
          thrown = true;
          message = String(e?.message ?? e);
        }
        check(thrown && matches(message, pattern), `to throw${pattern == null ? "" : " " + show(String(pattern))}`);
      },
    };
  };

  return {
    module: {
      test, describe, expect, open: native.open,
      beforeAll: (f) => top().beforeAll.push(f),
      afterAll: (f) => top().afterAll.push(f),
      beforeEach: (f) => top().before.push(f),
      afterEach: (f) => top().after.push(f),
    },
    // The tests, with whether each runs, for Go.
    tests: () => tests.map((t) => ({ name: t.name,
      run: !t.skip && (!only || t.mode === "only" || t.suites.some((s) => s.only)) })),
    // run(i) runs test i with its suites' hooks: a promise.
    run: async (i) => {
      const t = tests[i];
      for (const s of t.suites) for (const f of s.beforeAll.splice(0)) await f();
      for (const s of t.suites) for (const f of s.before) await f();
      try {
        await t.body();
      } finally {
        for (const s of [...t.suites].reverse()) for (const f of s.after) await f();
      }
    },
  };
});
