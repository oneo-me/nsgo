// The nsgo/test module: an app's tests in TSX, which nsgo test runs
// headless or on a platform.

/** The manual clock `open` gives the app (`clock: {}`). */
export interface ClockOptions {
  /** Its wall time at the start (`Date.now()`): a `Date` or milliseconds since the Unix epoch. */
  time?: Date | number | null;
}
/** The system's appearance. */
export type ColorScheme = "light" | "dark";
/** The platform a headless app is opened as. */
export type Platform = "ios" | "android" | "macos" | "windows" | "linux" | "web";
/** Safe area insets in points. */
export interface SafeArea { top?: number; right?: number; bottom?: number; left?: number }
/** The embedding app a test plays (`open`'s `host`). */
export interface HostOptions {
  /** What the app reads as `host.params`. */
  params?: Record<string, unknown> | null;
  /** The handlers of the app's `host.call` and `host.post`, by name: a value answers every call; a function gets the data and returns the answer, or a promise of it, and what it throws rejects the call. A call of a name not listed fails; a post of one is dropped. */
  handlers?: Record<string, unknown | ((data: unknown) => unknown)> | null;
}
export interface OpenOptions {
  storage?: Record<string, string> | null;
  width?: number | null;
  height?: number | null;
  safeArea?: SafeArea | null;
  colorScheme?: ColorScheme | null;
  locale?: string | null;
  reducedMotion?: boolean | null;
  platform?: Platform | null;
  /** A manual clock: the app's time moves only as the test moves it (`app.clock`). Without one, its time passes as the test waits. */
  clock?: ClockOptions | null;
  /** The embedding app the test plays; without one, the page's host module nsgo.yaml's `web.host` names answers the app headless. */
  host?: HostOptions | null;
  /** What requests get, by "<METHOD> <URL>" (a bare URL is a GET; * matches any text): a response, or a function of the request giving one, which runs as a step waits; a request none lists fails. "live": the network. */
  network?: "live" | Record<string, StubResponse | ((request: StubRequest) => StubResponse | Promise<StubResponse>)> | null;
  /** The text on the clipboard at the start. */
  clipboard?: string | null;
  /** Whether the platform recognizes speech: without it, Speech sessions end with "unavailable". */
  speech?: boolean | null;
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
export interface SpanStyle { color: string; backgroundColor?: string | null; fontSize: number; fontWeight: number; italic: boolean; underline: boolean; lineThrough: boolean }
/** What a wait waits for. */
export type WaitState = "shown" | "gone";
export interface WaitOptions { timeout?: number | null; state?: WaitState | null }
export interface Dialog { title?: string | null; message?: string | null; buttons: string[]; cancelable: boolean; sheet: boolean }
/** How far a drag moves the finger, in points. */
export interface DragBy { x?: number; y?: number }
/** What a test pastes: what the clipboard holds as the user pastes. */
export interface PasteContent { text?: string | null; uris?: string[] | null }
/** A call or a post the app made to the embedding app the test plays. */
export interface HostMessage { name: string; data: unknown }
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
/** The app a test opened. A step resolves once the app is quiet and rejects, with what is on screen, when it cannot be taken or the app threw. */
export declare class App {
  private constructor();
  press(selector: string): Promise<void>;
  longPress(selector: string): Promise<void>;
  type(selector: string, text: string): Promise<void>;
  submit(selector: string): Promise<void>;
  clear(selector: string): Promise<void>;
  scroll(selector: string, distance: number): Promise<void>;
  drag(selector: string, by: DragBy): Promise<void>;
  rightClick(selector: string): Promise<void>;
  hover(selector: string): Promise<void>;
  paste(selector: string, content: PasteContent): Promise<void>;
  /** Chooses an action of the context menu showing; a submenu's after its label ("Share > Mail"). */
  chooseMenuItem(label: string): Promise<void>;
  closeMenu(): Promise<void>;
  back(): Promise<void>;
  wait(ms: number): Promise<void>;
  waitFor(selector: string, options?: WaitOptions | null): Promise<void>;
  waitUntil(condition: () => boolean, options?: WaitOptions | null): Promise<void>;
  setViewport(width: number, height: number): Promise<void>;
  setColorScheme(scheme: ColorScheme): Promise<void>;
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
  /** The speech session running, null when none is. */
  speech(): { lang: string; interim: boolean } | null;
  /** Has the session running hear `text`, the whole transcript so far, final once it will not change. */
  hearSpeech(text: string, isFinal?: boolean): Promise<void>;
  /** Ends the session running with what it heard and `error`, a SpeechErrorCode, if any. */
  endSpeech(error?: string | null): Promise<void>;
  /** The text on the clipboard, null when it holds none. */
  clipboard(): string | null;
  /** The requests the app made, in order. */
  requests(): StubRequest[];
  /** The style of the characters a selector ending in text: names. */
  spanStyle(selector: string): SpanStyle;
  readonly keyboard: Keyboard;
  readonly clock: Clock;
  readonly host: AppHost;
}
/** The embedding app the test plays for an app: its events to the app and what the app sent it. */
export declare class AppHost {
  private constructor();
  /** Sends event `name` with `data` to the app, and resolves once the app is quiet. */
  send(name: string, data?: unknown | null): Promise<void>;
  /** The app's calls of the names `open`'s `host.handlers` lists, in order. */
  calls(): HostMessage[];
  /** The app's posts of the names `open`'s `host.handlers` lists, in order. */
  posts(): HostMessage[];
}
/** The app's clock, which the test moves. */
export declare class Clock {
  private constructor();
  /** Moves the clock by `ms` milliseconds, a display frame at a time. */
  runFor(ms: number): Promise<void>;
  /** Moves the clock by `ms` milliseconds at once: the timers due meanwhile run once each. */
  fastForward(ms: number): Promise<void>;
  /** Sets the wall time (`Date.now()`, `new Date()`) to `time`, moving no timer. */
  setSystemTime(time: Date | number): void;
}
/** The app's hardware keyboard. */
export declare class Keyboard {
  private constructor();
  /** Presses `keys`, a keystroke as keymaps name it ("mod+k", "shift+/", "escape"). */
  press(keys: string): Promise<void>;
}
/** The checks of what a promise rejects with. */
export declare class Rejection {
  private constructor();
  toThrow(message?: string | RegExp | null): Promise<void>;
}
export declare class Expectation {
  private constructor();
  readonly not: Expectation;
  readonly rejects: Rejection;
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
