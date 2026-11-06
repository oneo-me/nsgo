// The nsgo module: the reactive API, the primitives, the environment,
// animation, gestures and services.

import type {
  Bounds, Edges, ImageAsset, ImageProps, JSX, ListProps, OS, OverlayProps, Point, ScrollProps, Size,
  TextProps, TouchProps, VideoProps, ViewProps, WebViewProps, YNode, GestureResponderEvent, ResponderGestureState,
} from "./types";

export type * from "./types";

export interface Accessor<T> { (): T }
export interface Signal<T> extends Accessor<T> {
  set(value: T): T;
  update(fn: (value: T) => T): T;
}
export interface SignalOptions<T> { equals?: false | ((a: T, b: T) => boolean) }
export interface Context<T> extends Accessor<T> {
  Provider: (props: { value: T; children?: JSX.Element }) => JSX.Element;
}
export type Component<P = {}> = (props: P) => JSX.Element;

export declare function signal<T>(value: T, options?: SignalOptions<T>): Signal<T>;
export declare function signal<T = undefined>(): Signal<T | undefined>;
export declare function computed<T>(fn: (previous: T | undefined) => T, options?: SignalOptions<T>): Accessor<T>;
export declare function effect(fn: () => void | (() => void)): void;
export declare function batch<T>(fn: () => T): T;
export declare function untracked<T>(fn: () => T): T;
export declare function cleanup(fn: () => void): void;
export declare function context<T>(defaultValue: T): Context<T>;
export declare function mount<P extends object>(app: Component<P>, props?: P): () => void;
export declare function split<T extends object, K extends readonly (keyof T)[]>(props: T, keys: K): [Pick<T, K[number]>, Omit<T, K[number]>];
export declare function ErrorBoundary(props: { fallback: JSX.Element | ((error: unknown, reset: () => void) => JSX.Element); children?: JSX.Element }): JSX.Element;

export type ResourceState = "unresolved" | "pending" | "ready" | "refreshing" | "errored";
export interface ResourceInfo { signal: AbortSignal; refetching: boolean }
export interface ResourceControls<T> {
  loading(): boolean;
  error(): unknown;
  state(): ResourceState;
  refetch(): void;
  mutate(value: T): T;
}
export interface Resource<T> extends Accessor<T | undefined>, ResourceControls<T> {}
/** A resource with an `initialValue`: it always has a value. */
export interface InitializedResource<T> extends Accessor<T>, ResourceControls<T> {}
export interface ResourceOptions<T> { initialValue?: T }
/** What a resource loads from: an accessor (tracked) or a value; undefined, null and false load nothing. */
export type ResourceSource<S> = S | false | null | undefined | (() => S | false | null | undefined);
export declare function resource<T>(fetcher: (info: ResourceInfo) => T | Promise<T>, options: ResourceOptions<T> & { initialValue: T }): InitializedResource<T>;
export declare function resource<T>(fetcher: (info: ResourceInfo) => T | Promise<T>, options?: ResourceOptions<T>): Resource<T>;
export declare function resource<S, T>(source: ResourceSource<S>, fetcher: (source: S, info: ResourceInfo) => T | Promise<T>, options: ResourceOptions<T> & { initialValue: T }): InitializedResource<T>;
export declare function resource<S, T>(source: ResourceSource<S>, fetcher: (source: S, info: ResourceInfo) => T | Promise<T>, options?: ResourceOptions<T>): Resource<T>;

export declare function View(props: ViewProps): YNode;
export declare function Text(props: TextProps): YNode;
export declare function Scroll(props: ScrollProps): YNode;
export declare function Overlay(props: OverlayProps): YNode;
export declare function List<T>(props: ListProps<T>): YNode;
export declare function WebView(props: WebViewProps): YNode;
export declare function Video(props: VideoProps): YNode;
export type ImageSize = Size;
export interface ImageModule {
  getSize(src: string | ImageAsset): Promise<ImageSize>;
  prefetch(src: string | ImageAsset): Promise<boolean>;
}
export declare const Image: ((props: ImageProps) => YNode) & ImageModule;

/** An item of a list whose viewability changed (onViewableItemsChange). */
export interface ViewToken<T> { item: T; key: string; index: number; isViewable: boolean }
export interface ViewabilityConfig {
  viewAreaCoveragePercentThreshold?: number;
  itemVisiblePercentThreshold?: number;
  minimumViewTime?: number;
}

// Canvas
export type CanvasNumber = number | AnimatedNode;
export type CanvasPaint = string | CanvasGradient | AnimatedNode;
export interface CanvasSize { width: number; height: number }
export interface CanvasProps extends ViewProps {
  draw: (cx: CanvasRenderingContext2D, size: CanvasSize) => void;
}
export declare function Canvas(props: CanvasProps): YNode;
export type LineCap = "butt" | "round" | "square";
export type LineJoin = "miter" | "round" | "bevel";
export type CanvasTextAlign = "start" | "end" | "left" | "right" | "center";
export type CanvasTextBaseline = "alphabetic" | "top" | "hanging" | "middle" | "ideographic" | "bottom";
/** What measureText measures of a text. */
export interface TextMetrics { width: number; fontBoundingBoxAscent: number; fontBoundingBoxDescent: number }
export interface CanvasPathMethods {
  moveTo(x: CanvasNumber, y: CanvasNumber): void;
  lineTo(x: CanvasNumber, y: CanvasNumber): void;
  quadraticCurveTo(cpx: CanvasNumber, cpy: CanvasNumber, x: CanvasNumber, y: CanvasNumber): void;
  bezierCurveTo(cp1x: CanvasNumber, cp1y: CanvasNumber, cp2x: CanvasNumber, cp2y: CanvasNumber, x: CanvasNumber, y: CanvasNumber): void;
  arc(x: CanvasNumber, y: CanvasNumber, radius: CanvasNumber, startAngle: CanvasNumber, endAngle: CanvasNumber, counterclockwise?: boolean): void;
  arcTo(x1: number, y1: number, x2: number, y2: number, radius: number): void;
  ellipse(x: number, y: number, radiusX: number, radiusY: number, rotation: number, startAngle: number, endAngle: number, counterclockwise?: boolean): void;
  rect(x: CanvasNumber, y: CanvasNumber, w: CanvasNumber, h: CanvasNumber): void;
  roundRect(x: number, y: number, w: number, h: number, radii?: number | number[]): void;
  closePath(): void;
}
export declare class Path2D {
  constructor(path?: Path2D | string);
  addPath(path: Path2D): void;
}
export interface CanvasPath extends CanvasPathMethods {}
export interface Path2D extends CanvasPathMethods {}
export declare class CanvasGradient {
  addColorStop(offset: number, color: string): void;
}
/** What the context draws with: its state, which save and restore keep. */
export interface CanvasProperties {
  fillStyle: CanvasPaint;
  strokeStyle: CanvasPaint;
  lineWidth: CanvasNumber;
  lineCap: LineCap;
  lineJoin: LineJoin;
  miterLimit: number;
  lineDashOffset: CanvasNumber;
  globalAlpha: CanvasNumber;
  globalCompositeOperation: string;
  shadowColor: string;
  shadowBlur: number;
  shadowOffsetX: number;
  shadowOffsetY: number;
  font: string;
  fontVariantNumeric: string;
  textAlign: CanvasTextAlign;
  textBaseline: CanvasTextBaseline;
}
/** What the context does besides its paths. */
export interface CanvasContextMethods {
  translate(x: CanvasNumber, y: CanvasNumber): void;
  scale(x: CanvasNumber, y: CanvasNumber): void;
  rotate(angle: CanvasNumber): void;
  transform(a: CanvasNumber, b: CanvasNumber, c: CanvasNumber, d: CanvasNumber, e: CanvasNumber, f: CanvasNumber): void;
  setTransform(a: CanvasNumber, b: CanvasNumber, c: CanvasNumber, d: CanvasNumber, e: CanvasNumber, f: CanvasNumber): void;
  resetTransform(): void;
  beginPath(): void;
  fillRect(x: CanvasNumber, y: CanvasNumber, w: CanvasNumber, h: CanvasNumber): void;
  strokeRect(x: CanvasNumber, y: CanvasNumber, w: CanvasNumber, h: CanvasNumber): void;
  clearRect(x: CanvasNumber, y: CanvasNumber, w: CanvasNumber, h: CanvasNumber): void;
  save(): void;
  restore(): void;
  setLineDash(segments: number[]): void;
  getLineDash(): number[];
  fill(path?: Path2D | "nonzero" | "evenodd", rule?: "nonzero" | "evenodd"): void;
  stroke(path?: Path2D): void;
  clip(path?: Path2D | "nonzero" | "evenodd", rule?: "nonzero" | "evenodd"): void;
  fillText(text: string, x: CanvasNumber, y: CanvasNumber, maxWidth?: number): void;
  strokeText(text: string, x: CanvasNumber, y: CanvasNumber, maxWidth?: number): void;
  measureText(text: string): TextMetrics;
  isPointInPath(x: number, y: number, rule?: "nonzero" | "evenodd"): boolean;
  isPointInPath(path: Path2D, x: number, y: number, rule?: "nonzero" | "evenodd"): boolean;
  isPointInStroke(x: number, y: number): boolean;
  isPointInStroke(path: Path2D, x: number, y: number): boolean;
  createLinearGradient(x0: number, y0: number, x1: number, y1: number): CanvasGradient;
  createRadialGradient(x0: number, y0: number, r0: number, x1: number, y1: number, r1: number): CanvasGradient;
  createConicGradient(startAngle: number, x: number, y: number): CanvasGradient;
  drawImage(image: string | ImageAsset, ...args: CanvasNumber[]): void;
}
export declare class CanvasRenderingContext2D {
  readonly canvas: { readonly width: number; readonly height: number };
}
export interface CanvasRenderingContext2D extends CanvasProperties, CanvasContextMethods, CanvasPathMethods {}

// The environment
export interface PlatformInfo {
  readonly os: OS;
  readonly version: string;
  readonly idiom: "phone" | "pad" | "desktop" | "mac" | "tv" | "";
  readonly catalyst: boolean;
  select<T>(spec: PlatformSelectSpec<T>): T | undefined;
}
/** What platform.select picks from: the platform's own, else its family's (mobile, desktop, native), else default. */
export interface PlatformSelectSpec<T> { ios?: T; android?: T; macos?: T; windows?: T; linux?: T; web?: T; mobile?: T; native?: T; desktop?: T; default?: T }
export declare const windowSize: Accessor<Size>;
export declare const colorScheme: Accessor<"light" | "dark">;
export declare function setColorScheme(scheme: "light" | "dark" | "system"): void;
export declare const fontScale: Accessor<number>;
export declare const pixelRatio: Accessor<number>;
export declare const locale: Accessor<string>;
export declare const platform: PlatformInfo;
export declare const reduceMotion: Accessor<boolean>;
export declare const screenReaderEnabled: Accessor<boolean>;
export declare const safeArea: Accessor<Edges>;
export declare function safeAreaInsets(): Accessor<Edges>;
export declare const keyboardFrame: Accessor<{ height: number; duration: number }>;
/** The color of the theme's role `name`, `#rrggbbaa`; undefined for none. */
/** The app's design tokens, by name: tokens.d.ts, which nsgo writes from its theme, fills it in. */
export interface Tokens {}
/** A design token of the theme shown here: a color as #rrggbbaa, a length in points, another value as written. */
export declare function token<K extends keyof Tokens>(name: K): Tokens[K];

// Services
export declare function announce(message: string): void;
export declare function setAccessibilityFocus(node: YNode): void;
export declare function openUrl(url: string): Promise<boolean>;
export interface KeyboardModule { dismiss(): void }
export declare const Keyboard: KeyboardModule;
export interface KeyBindingOptions { when?: "always" | "notEditing"; repeat?: boolean }
export interface Keymap {
  bind(keystroke: string, handler: (e: { keystroke: string }) => void, options?: KeyBindingOptions): () => void;
  bind(bindings: Record<string, (e: { keystroke: string }) => void>, options?: KeyBindingOptions): () => void;
  label(keystroke: string): string;
}
export declare const keymap: Keymap;
export interface ClipboardImage { url: string; size: Size }
export interface ClipboardModule {
  writeText(text: string): Promise<void>;
  readText(): Promise<string>;
  hasText(): Promise<boolean>;
  writeImage(src: string): Promise<void>;
  readImage(): Promise<ClipboardImage | null>;
  hasImage(): Promise<boolean>;
  readFiles(): Promise<string[]>;
}
export declare const Clipboard: ClipboardModule;
export interface StorageModule {
  getItem(key: string): Promise<string | null>;
  setItem(key: string, value: string): Promise<void>;
  removeItem(key: string): Promise<void>;
  keys(): Promise<string[]>;
  clear(): Promise<void>;
}
export declare const Storage: StorageModule;
/** The formats a pick asks for: "compatible" converts to JPEG, "current" keeps the photos' own. */
export type PickFormat = "compatible" | "current";
export interface PickImagesOptions { multiple?: boolean; limit?: number; maxDimension?: number; quality?: number; format?: PickFormat }
export interface PickedImage { file: File; size: Size }
export interface PickerModule { pickImages(options?: PickImagesOptions | null): Promise<PickedImage[]> }
export declare const Picker: PickerModule;

export type HostValue = null | boolean | number | string | HostValue[] | { [key: string]: HostValue };
export interface HostSubscription { (): void; remove(): void }
/** The parameters the embedding app started the app with. */
export type HostParams = { readonly [key: string]: HostValue };
export interface HostCallOptions { timeout?: number | null }
/** The embedding app's events, and whether one embeds the app. */
export interface HostEvents {
  available(): boolean;
  on<T = HostValue>(name: string, listener: (data: T) => void): HostSubscription;
}
/** The embedding app's functions the app calls. */
export interface hostModule {
  call<T = HostValue>(name: string, data?: unknown, options?: HostCallOptions | null): Promise<T>;
  post(name: string, data?: unknown): void;
}
export interface Host extends HostEvents, hostModule {
  readonly params: HostParams;
}
export declare const host: Host;

export interface NativeComponentSpec<P, E> { props: readonly (keyof P & string)[]; events?: readonly (keyof E & string)[] }
export type NativeEventHandlers<E> = { [K in keyof E & string as `on${Capitalize<K>}`]?: (data: E[K]) => void };
/** The props of a native component: a view's, its own (numbers and strings animated too) and its events' handlers. */
export type NativeComponentProps<P, E> = Omit<ViewProps, "children"> & { [K in keyof P]: NonNullable<P[K]> extends number | string ? P[K] | AnimatedNode : P[K] } & NativeEventHandlers<E>;
export declare function nativeComponent<P extends object = {}, E extends object = {}>(name: string, spec: NativeComponentSpec<P, E>): (props: NativeComponentProps<P, E>) => YNode;
/** The exports of the module the app's Go code registered as specifier (tsx.Register, tsx.RegisterFunctions), as an import gives them; the app declares T itself. Throws a ReferenceError where none is registered. */
export declare function nativeModule<T extends object>(specifier: string): T;

// Gestures
export interface PanResponderGestureState extends ResponderGestureState { stateID: number }
export type PanHandler<R = void> = (e: GestureResponderEvent, gestureState: PanResponderGestureState) => R;
export interface PanResponderCallbacks {
  onStartShouldSetPanResponder?: PanHandler<boolean>;
  onStartShouldSetPanResponderCapture?: PanHandler<boolean>;
  onMoveShouldSetPanResponder?: PanHandler<boolean>;
  onMoveShouldSetPanResponderCapture?: PanHandler<boolean>;
  onPanResponderGrant?: PanHandler;
  onPanResponderReject?: PanHandler;
  onPanResponderStart?: PanHandler;
  onPanResponderMove?: PanHandler;
  onPanResponderEnd?: PanHandler;
  onPanResponderRelease?: PanHandler;
  onPanResponderTerminate?: PanHandler;
  onPanResponderTerminationRequest?: PanHandler<boolean>;
  /** Whether the grant stops the scroll views around from taking the touches (default true). */
  onShouldBlockNativeResponder?: PanHandler<boolean>;
}
export interface PanResponderInstance { panHandlers: TouchProps }
export declare const PanResponder: { create(config: PanResponderCallbacks): PanResponderInstance };

// Animation
export type EasingCurveName = "quad" | "cubic" | "poly" | "sin" | "circle" | "exp" | "elastic" | "back" | "bounce";
export interface EasingCurve { curve: EasingCurveName; param?: number; mode: "in" | "out" | "inOut" }
export type StepPosition = "jump-start" | "jump-end" | "jump-none" | "jump-both" | "start" | "end";
export interface EasingSteps { steps: number; position?: StepPosition }
export type Easing = "linear" | "ease" | "easeIn" | "easeOut" | "easeInOut" | [number, number, number, number] | EasingCurve | EasingSteps;
export type Extrapolation = "extend" | "clamp" | "identity";
export interface SpringConfig { toValue: number; stiffness?: number; damping?: number; mass?: number; velocity?: number }
export interface DecayConfig { velocity?: number; deceleration?: number; clamp?: [min: number, max: number]; bounce?: number }
export interface Keyframe { offset?: number; value: number | string; easing?: Easing }
export type KeyframeValue = number | string | Keyframe;
export type PlaybackDirection = "normal" | "reverse" | "alternate" | "alternate-reverse";
export interface KeyframeTiming { duration?: number; delay?: number; easing?: Easing; iterations?: number | "infinite"; direction?: PlaybackDirection }
export interface KeyframesConfig extends KeyframeTiming { keyframes: KeyframeValue[] }
export declare class AnimatedNode {
  interpolate(config: { inputRange: number[]; outputRange: number[] | string[]; extrapolate?: Extrapolation; extrapolateLeft?: Extrapolation; extrapolateRight?: Extrapolation; easing?: Easing }): AnimatedNode;
  diffClamp(min: number, max: number, options?: { snap?: boolean }): AnimatedNode;
  add(b: AnimatedNode | number): AnimatedNode;
  sub(b: AnimatedNode | number): AnimatedNode;
  mul(b: AnimatedNode | number): AnimatedNode;
  div(b: AnimatedNode | number): AnimatedNode;
  mod(b: AnimatedNode | number): AnimatedNode;
}
export declare class AnimatedValue extends AnimatedNode {
  constructor(value?: number);
  set(value: number): void;
  stop(): Promise<number>;
  getValue(): Promise<number>;
  setOffset(offset: number): void;
  flattenOffset(): void;
  extractOffset(): void;
  timing(to: number, options?: { duration?: number; easing?: Easing; delay?: number }): Promise<boolean>;
  spring(to: number, options?: { stiffness?: number; damping?: number; mass?: number; velocity?: number }): Promise<boolean>;
  decay(options?: DecayConfig): Promise<boolean>;
  keyframes(keyframes: KeyframeValue[], options?: KeyframeTiming): Promise<boolean>;
}
export declare function createAnimatedValue(value?: number): AnimatedValue;
export interface CompositeAnimation {
  start(callback?: (result: { finished: boolean }) => void): Promise<boolean>;
  stop(): void;
  reset(): void;
}
export interface AnimatedModule {
  timing(value: AnimatedValue, config: { toValue: number; duration?: number; easing?: Easing; delay?: number }): CompositeAnimation;
  spring(value: AnimatedValue, config: SpringConfig): CompositeAnimation;
  decay(value: AnimatedValue, config?: DecayConfig): CompositeAnimation;
  sequence(list: CompositeAnimation[]): CompositeAnimation;
  parallel(list: CompositeAnimation[], config?: { stopTogether?: boolean }): CompositeAnimation;
  stagger(ms: number, list: CompositeAnimation[], config?: { from?: "first" | "last" | "center" | number }): CompositeAnimation;
  delay(ms: number): CompositeAnimation;
  /** Runs animation again as it ends; resetBeforeIteration (default true) puts its values back at their start first. */
  loop(animation: CompositeAnimation, config?: { iterations?: number; resetBeforeIteration?: boolean }): CompositeAnimation;
  keyframes(value: AnimatedValue, config: KeyframesConfig): CompositeAnimation;
  add(a: AnimatedNode, b: AnimatedNode | number): AnimatedNode;
  subtract(a: AnimatedNode, b: AnimatedNode | number): AnimatedNode;
  multiply(a: AnimatedNode, b: AnimatedNode | number): AnimatedNode;
  divide(a: AnimatedNode, b: AnimatedNode | number): AnimatedNode;
  modulo(a: AnimatedNode, b: AnimatedNode | number): AnimatedNode;
  diffClamp(a: AnimatedNode, min: number, max: number, options?: { snap?: boolean }): AnimatedNode;
  sampleSpring(config: SpringConfig & { from?: number }): number[];
  sampleDecay(config: DecayConfig & { from?: number }): number[];
}
export declare const Animated: AnimatedModule;
export interface EasingCurves {
  quad: EasingCurve;
  cubic: EasingCurve;
  sin: EasingCurve;
  circle: EasingCurve;
  exp: EasingCurve;
  bounce: EasingCurve;
  poly(n?: number): EasingCurve;
  elastic(bounciness?: number): EasingCurve;
  back(s?: number): EasingCurve;
}
export interface EasingModule extends EasingCurves {
  linear: Easing;
  ease: Easing;
  bezier(x1: number, y1: number, x2: number, y2: number): Easing;
  steps(count: number, position?: StepPosition): EasingSteps;
  in(e: Easing): Easing;
  out(e: Easing): Easing;
  inOut(e: Easing): Easing;
  sample(easing: Easing, count: number): number[];
}
export declare const Easing: EasingModule;
export type LayoutAnimationType = "easeInEaseOut" | "spring" | "linear" | "easeIn" | "easeOut" | "keyboard";
export type LayoutAnimationProperty = "opacity" | "scaleXY" | "scale";
export interface LayoutAnimationAnimation { type?: LayoutAnimationType; property?: LayoutAnimationProperty; duration?: number; delay?: number; springDamping?: number }
export interface LayoutAnimationConfig { duration: number; create?: LayoutAnimationAnimation; update?: LayoutAnimationAnimation; delete?: LayoutAnimationAnimation }
export type LayoutAnimationTypes = { readonly [K in LayoutAnimationType]: K };
export interface LayoutAnimationProperties { readonly opacity: "opacity"; readonly scaleXY: "scaleXY" }
export interface LayoutAnimationPresets { easeInEaseOut: LayoutAnimationConfig; linear: LayoutAnimationConfig; spring: LayoutAnimationConfig }
export interface LayoutAnimationModule {
  configureNext(config: LayoutAnimationConfig, onEnd?: () => void): void;
  create(duration: number, type?: LayoutAnimationType, property?: LayoutAnimationProperty): LayoutAnimationConfig;
  Types: LayoutAnimationTypes;
  Properties: LayoutAnimationProperties;
  Presets: LayoutAnimationPresets;
  easeInEaseOut(onEnd?: () => void): void;
  linear(onEnd?: () => void): void;
  spring(onEnd?: () => void): void;
}
export declare const LayoutAnimation: LayoutAnimationModule;
export type MotionChannel = "opacity" | "translateX" | "translateY" | "rotate" | "rotateX" | "rotateY" | "scale" | "scaleX" | "scaleY";
export type MotionName = "fadeIn" | "fadeInUp" | "fadeInDown" | "fadeInLeft" | "fadeInRight" | "zoomIn" | "popIn" | "bounceIn" | "slideInUp" | "slideInDown" | "flipInX" | "flipInY" | "fadeOut" | "fadeOutUp" | "fadeOutDown" | "fadeOutLeft" | "fadeOutRight" | "zoomOut" | "popOut" | "slideOutDown" | "slideOutUp" | "shake" | "headShake" | "wiggle" | "swing" | "pulse" | "heartbeat" | "rubberBand" | "jello" | "tada" | "jump" | "flash" | "spin" | "float" | "breathe" | "sway" | "bob" | "glow";
/** A keyframe of a motion: the channels' values at its offset. */
export type MotionFrame = { [C in MotionChannel]?: number | string } & { offset?: number; easing?: Easing };
export interface MotionSpec extends KeyframeTiming {
  keyframes: { [C in MotionChannel]?: KeyframeValue[] } | MotionFrame[];
  ignoreReduceMotion?: boolean;
}
export interface MotionPreset extends MotionSpec { kind: "enter" | "exit" | "attention" | "loop" }
export interface MotionPlayOptions extends KeyframeTiming { ignoreReduceMotion?: boolean }
export interface Motion {
  readonly opacity: AnimatedValue;
  readonly translateX: AnimatedValue;
  readonly translateY: AnimatedValue;
  readonly rotate: AnimatedValue;
  readonly rotateX: AnimatedValue;
  readonly rotateY: AnimatedValue;
  readonly scale: AnimatedValue;
  readonly scaleX: AnimatedValue;
  readonly scaleY: AnimatedValue;
  readonly style: { opacity: AnimatedValue; transform: MotionTransform };
  readonly transform: MotionTransform;
  play(motion: MotionName | MotionSpec, options?: MotionPlayOptions): Promise<boolean>;
  animation(motion: MotionName | MotionSpec, options?: MotionPlayOptions): CompositeAnimation;
  stop(): void;
  reset(): void;
}
/** A motion's transform, its channels' values in order. */
export type MotionTransform = import("./types").TransformStep[];
export interface MotionOptions extends MotionPlayOptions { initial?: { [C in MotionChannel]?: number }; enter?: MotionName | MotionSpec }
export declare function createMotion(options?: MotionOptions): Motion;
export declare const motions: Readonly<Record<MotionName, MotionPreset>>;

export type { Bounds, Point, Size, Edges };
