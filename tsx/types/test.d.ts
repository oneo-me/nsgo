// The nsgo/test module: an app's tests in TSX, which nsgo test runs
// headless or on a platform.

export interface OpenOptions {
  storage?: Record<string, string> | null;
  width?: number | null;
  height?: number | null;
  safeArea?: { top?: number; right?: number; bottom?: number; left?: number } | null;
  colorScheme?: "light" | "dark" | null;
  locale?: string | null;
  reducedMotion?: boolean | null;
  platform?: "ios" | "android" | "macos" | "windows" | "linux" | "web" | null;
  host?: { params?: Record<string, unknown> | null; handlers?: Record<string, unknown | ((data: unknown) => unknown)> | null } | null;
  /** What requests get, by "<METHOD> <URL>" (a bare URL is a GET; * matches any text): a response, or a function of the request giving one, which runs as a step waits; a request none lists fails. "live": the network. */
  network?: "live" | Record<string, StubResponse | ((request: StubRequest) => StubResponse | Promise<StubResponse>)> | null;
  /** The text on the clipboard at the start. */
  clipboard?: string | null;
}
/** A response open's network gives a request. */
export interface StubResponse {
  /** The status code (200). */
  status?: number;
  headers?: Record<string, string>;
  /** A string, or a value sent as JSON (with content-type: application/json unless headers name one). */
  body?: unknown;
  /** The request fails with this message instead. */
  error?: string;
  /** How long the response takes, in milliseconds. */
  delay?: number;
}
export interface StubRequest { method: string; url: string; headers: Record<string, string>; body: string | null }
export interface Pick { multiple: boolean; limit: number; maxDimension: number | null; quality: number | null; format: "compatible" | "current" }
export interface PickFile { name: string; /** Its media type; none: read from its name. */ type?: string | null; data: Uint8Array | ArrayBuffer | number[] }
/** An action of the context menu the app shows; a submenu's after its label ("Share > Mail"). */
export interface MenuEntry { label: string; disabled: boolean; checked?: boolean }
/** The style of a span as the app resolved it; colors are #rrggbbaa. */
export interface SpanStyle { color: string; fontSize: number; fontWeight: number; italic: boolean; underline: boolean; lineThrough: boolean }
export interface WaitOptions { timeout?: number | null; state?: "shown" | "gone" | null }
export interface Dialog { title?: string | null; message?: string | null; buttons: string[]; cancelable: boolean; sheet: boolean }
export declare function test(name: string, body: () => void | Promise<void>): void;
export declare namespace test {
  function skip(name: string, body: () => void | Promise<void>): void;
  function only(name: string, body: () => void | Promise<void>): void;
  function todo(name: string): void;
}
export declare function describe(name: string, body: () => void): void;
export declare namespace describe {
  function skip(name: string, body: () => void): void;
  function only(name: string, body: () => void): void;
}
export declare function beforeAll(body: () => void | Promise<void>): void;
export declare function afterAll(body: () => void | Promise<void>): void;
export declare function beforeEach(body: () => void | Promise<void>): void;
export declare function afterEach(body: () => void | Promise<void>): void;
export declare function open(entry: string, options?: OpenOptions | null): Promise<App>;
export declare function expect(value: unknown): Expectation;
export interface App {
  press(selector: string): Promise<void>;
  longPress(selector: string): Promise<void>;
  type(selector: string, text: string): Promise<void>;
  submit(selector: string): Promise<void>;
  clear(selector: string): Promise<void>;
  scroll(selector: string, distance: number): Promise<void>;
  drag(selector: string, by: { x?: number; y?: number }): Promise<void>;
  rightClick(selector: string): Promise<void>;
  hover(selector: string): Promise<void>;
  paste(selector: string, content: { text?: string | null; uris?: string[] | null }): Promise<void>;
  /** Chooses an action of the context menu showing; a submenu's after its label ("Share > Mail"). */
  chooseMenuItem(label: string): Promise<void>;
  closeMenu(): Promise<void>;
  back(): Promise<void>;
  wait(ms: number): Promise<void>;
  waitFor(selector: string, options?: WaitOptions | null): Promise<void>;
  waitUntil(condition: () => boolean, options?: WaitOptions | null): Promise<void>;
  setViewport(width: number, height: number): Promise<void>;
  setColorScheme(scheme: "light" | "dark"): Promise<void>;
  screenshot(path?: string | null): Promise<void>;
  count(selector: string): number;
  shows(selector: string): boolean;
  enabled(selector: string): boolean;
  texts(): string[];
  value(selector: string): string | null;
  storage(): Record<string, string>;
  dialogs(): Dialog[];
  /** Presses button index of the dialog showing. */
  answer(index: number): void;
  /** Answers the dialogs from now on with the text of the button to press, or null to close them. */
  onDialog(handler: ((dialog: Dialog) => string | null) | null): void;
  /** Answers the picks from now on with the files the user picks, or null to cancel. */
  onPick(handler: ((pick: Pick) => PickFile[] | null) | null): void;
  picks(): Pick[];
  /** The context menu showing, null when none is. */
  menu(): MenuEntry[] | null;
  /** The URLs the app opened: recorded, never opened. */
  openedUrls(): string[];
  /** The text on the clipboard, null when it holds none. */
  clipboard(): string | null;
  /** The requests the app made, in order. */
  requests(): StubRequest[];
  /** The style of the characters a selector ending in text: names. */
  spanStyle(selector: string): SpanStyle;
  readonly keyboard: { press(keys: string): Promise<void> };
  readonly clock: { runFor(ms: number): Promise<void>; fastForward(ms: number): Promise<void> };
}
export interface Expectation {
  readonly not: Expectation;
  readonly rejects: { toThrow(message?: string | RegExp | null): Promise<void> };
  toBe(expected: unknown): void;
  toEqual(expected: unknown): void;
  toContain(item: unknown): void;
  toMatch(pattern: string | RegExp): void;
  toHaveLength(length: number): void;
  toBeNull(): void;
  toBeTruthy(): void;
  toBeFalsy(): void;
  toBeGreaterThan(n: number): void;
  toBeGreaterThanOrEqual(n: number): void;
  toBeLessThan(n: number): void;
  toBeLessThanOrEqual(n: number): void;
  toThrow(message?: string | RegExp | null): void;
}
