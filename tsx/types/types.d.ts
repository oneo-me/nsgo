// The element and JSX types of nsgo: styles, props, events and nodes.

import type { AnimatedNode, AnimatedValue, ViewabilityConfig } from "./index";
import type { StatusBar } from "./base";

/** A style, or a list of styles (nested lists included) merged left to
 * right; falsy entries are skipped. */
export type StyleProp<T> = T | readonly (StyleProp<T> | false | null | undefined)[] | false | null | undefined;

export type ColorValue = string;
export type DimensionValue = number | `${number}%` | "auto";
export type LengthValue = number | `${number}%`;
export type EnvValue = `env(${string})` | `calc(${string})`;
type Animatable<T> = T | AnimatedNode;
type Align = "flex-start" | "flex-end" | "center" | "stretch" | "baseline" | "start" | "end" | "self-start" | "self-end" | "normal";
type Distribute = "flex-start" | "flex-end" | "center" | "stretch" | "space-between" | "space-around" | "space-evenly" | "start" | "end" | "normal";
type Time = number | `${number}ms` | `${number}s`;
type TimingFunction = "linear" | "ease" | "ease-in" | "ease-out" | "ease-in-out" | `cubic-bezier(${string})`;
type SizeValue = DimensionValue | "min-content" | "max-content" | "fit-content";

export type TransformStep =
  | { translateX: number | `${number}%` | AnimatedNode }
  | { translateY: number | `${number}%` | AnimatedNode }
  | { scale: Animatable<number> }
  | { scaleX: Animatable<number> }
  | { scaleY: Animatable<number> }
  | { rotate: number | string | AnimatedNode }
  | { rotateZ: number | string | AnimatedNode }
  | { rotateX: number | string | AnimatedNode }
  | { rotateY: number | string | AnimatedNode }
  | { skewX: number | string }
  | { skewY: number | string }
  | { perspective: number };

export interface LayoutStyle {
  display?: "flex" | "grid" | "block" | "flow-root" | "none";
  boxSizing?: "border-box" | "content-box";
  position?: "relative" | "absolute" | "static" | "fixed" | "sticky";
  flexDirection?: "row" | "column" | "row-reverse" | "column-reverse";
  flexWrap?: "wrap" | "nowrap" | "wrap-reverse";
  flex?: number;
  flexGrow?: number;
  flexShrink?: number;
  flexBasis?: SizeValue;
  justifyContent?: Distribute;
  alignItems?: Align;
  alignSelf?: Align | "auto";
  alignContent?: Distribute;
  justifyItems?: Align;
  justifySelf?: Align | "auto";
  width?: SizeValue;
  height?: SizeValue;
  minWidth?: LengthValue;
  minHeight?: LengthValue;
  maxWidth?: LengthValue;
  maxHeight?: LengthValue;
  aspectRatio?: number;
  top?: DimensionValue | AnimatedNode;
  right?: DimensionValue;
  bottom?: DimensionValue;
  left?: DimensionValue | AnimatedNode;
  start?: DimensionValue;
  end?: DimensionValue;
  margin?: DimensionValue | EnvValue;
  marginHorizontal?: DimensionValue | EnvValue;
  marginVertical?: DimensionValue | EnvValue;
  marginTop?: DimensionValue | EnvValue;
  marginRight?: DimensionValue | EnvValue;
  marginBottom?: DimensionValue | EnvValue;
  marginLeft?: DimensionValue | EnvValue;
  marginStart?: DimensionValue;
  marginEnd?: DimensionValue;
  padding?: LengthValue | EnvValue;
  paddingHorizontal?: LengthValue | EnvValue;
  paddingVertical?: LengthValue | EnvValue;
  paddingTop?: LengthValue | EnvValue;
  paddingRight?: LengthValue | EnvValue;
  paddingBottom?: LengthValue | EnvValue;
  paddingLeft?: LengthValue | EnvValue;
  paddingStart?: LengthValue;
  paddingEnd?: LengthValue;
  gap?: LengthValue;
  rowGap?: LengthValue;
  columnGap?: LengthValue;
  zIndex?: number;
  direction?: "inherit" | "ltr" | "rtl";
  gridTemplateColumns?: string;
  gridTemplateRows?: string;
  gridAutoColumns?: string;
  gridAutoRows?: string;
  gridAutoFlow?: "row" | "column" | "dense" | "row dense" | "column dense";
  gridColumn?: number | string;
  gridColumnStart?: number | string;
  gridColumnEnd?: number | string;
  gridRow?: number | string;
  gridRowStart?: number | string;
  gridRowEnd?: number | string;
}

type BorderStyle = "solid" | "dashed" | "dotted" | "none" | "hidden";

export interface ViewStyle extends LayoutStyle {
  backgroundColor?: Animatable<ColorValue>;
  backgroundImage?: string;
  borderWidth?: number;
  borderTopWidth?: number;
  borderRightWidth?: number;
  borderBottomWidth?: number;
  borderLeftWidth?: number;
  borderStartWidth?: number;
  borderEndWidth?: number;
  borderColor?: Animatable<ColorValue>;
  borderTopColor?: ColorValue;
  borderRightColor?: ColorValue;
  borderBottomColor?: ColorValue;
  borderLeftColor?: ColorValue;
  borderStartColor?: ColorValue;
  borderEndColor?: ColorValue;
  borderStyle?: BorderStyle;
  borderRadius?: number;
  borderTopLeftRadius?: number;
  borderTopRightRadius?: number;
  borderBottomRightRadius?: number;
  borderBottomLeftRadius?: number;
  borderTopStartRadius?: number;
  borderTopEndRadius?: number;
  borderBottomEndRadius?: number;
  borderBottomStartRadius?: number;
  borderCurve?: "circular" | "continuous";
  outlineWidth?: number;
  outlineColor?: ColorValue;
  outlineStyle?: BorderStyle;
  outlineOffset?: number;
  opacity?: Animatable<number>;
  visibility?: "visible" | "hidden" | "collapse";
  overflow?: "visible" | "hidden";
  boxShadow?: string;
  shadowColor?: Animatable<ColorValue>;
  shadowOffset?: { width: number; height: number };
  shadowOpacity?: Animatable<number>;
  shadowRadius?: Animatable<number>;
  filter?: string;
  backdropFilter?: string;
  maskImage?: string;
  glassEffect?: "none" | "regular" | "clear";
  mixBlendMode?: "normal" | "multiply" | "screen" | "overlay" | "darken" | "lighten" | "color-dodge" | "color-burn" | "hard-light" | "soft-light" | "difference" | "exclusion" | "hue" | "saturation" | "color" | "luminosity" | "plus-darker" | "plus-lighter";
  cursor?: "auto" | "default" | "pointer" | "text" | "vertical-text" | "wait" | "progress" | "help" | "crosshair" | "move" | "all-scroll" | "grab" | "grabbing" | "not-allowed" | "no-drop" | "zoom-in" | "zoom-out" | "col-resize" | "e-resize" | "w-resize" | "ew-resize" | "row-resize" | "n-resize" | "s-resize" | "ns-resize" | "none";
  backfaceVisibility?: "visible" | "hidden";
  transform?: TransformStep[];
  transformOrigin?: string | (number | string)[];
  transitionProperty?: string;
  transitionDuration?: Time;
  transitionDelay?: Time;
  transitionTimingFunction?: TimingFunction;
  animationName?: "none" | "spin" | "ping" | "pulse" | "bounce";
  animationDuration?: Time;
  animationDelay?: Time;
  animationTimingFunction?: TimingFunction;
  animationIterationCount?: number | "infinite";
  fill?: "none" | "currentColor" | ColorValue;
  stroke?: "none" | "currentColor" | ColorValue;
  strokeWidth?: number | string;
  strokeDasharray?: number | string;
  verticalAlign?: "baseline" | "text-top" | "text-bottom" | "middle";
}

export interface TextStyle extends ViewStyle {
  color?: Animatable<ColorValue>;
  fontSize?: number;
  fontWeight?: "normal" | "bold" | number | `${number}`;
  fontFamily?: string;
  fontStyle?: "normal" | "italic";
  fontVariant?: ("small-caps" | "oldstyle-nums" | "lining-nums" | "tabular-nums" | "proportional-nums")[];
  lineHeight?: number;
  letterSpacing?: number;
  textAlign?: "auto" | "left" | "center" | "right" | "justify";
  textAlignVertical?: "auto" | "top" | "center" | "bottom";
  textDecorationLine?: "none" | "underline" | "line-through" | "underline line-through";
  textDecorationStyle?: "solid" | "double" | "dotted" | "dashed" | "wavy";
  textDecorationColor?: ColorValue;
  textDecorationThickness?: number | "auto" | "from-font";
  textUnderlineOffset?: number | "auto";
  textTransform?: "none" | "uppercase" | "lowercase" | "capitalize";
  textShadowColor?: ColorValue;
  textShadowOffset?: { width: number; height: number };
  textShadowRadius?: number;
  writingDirection?: "auto" | "ltr" | "rtl";
  placeholderColor?: ColorValue;
  selectionColor?: ColorValue;
}

export interface ImageStyle extends ViewStyle {
  tintColor?: ColorValue;
  objectPosition?: string;
}

export interface Point { x: number; y: number }
export interface Size { width: number; height: number }
export interface Bounds { origin: Point; size: Size }
export interface Edges { top: number; right: number; bottom: number; left: number }
export interface Modifiers { control: boolean; alt: boolean; shift: boolean; platform: boolean }
export interface PressEvent { position: Point; pagePosition: Point; modifiers: Modifiers; clickCount: number }
export interface HoverEvent { position: Point; pagePosition: Point; modifiers: Modifiers }
export interface ContextMenuEvent { position: Point; pagePosition: Point; modifiers: Modifiers }
export interface ScrollEvent { offset: Point; contentSize: Size; size: Size; zoomScale: number }
export interface CaretEvent { bounds: Bounds; pageBounds: Bounds }
export interface PasteEvent { text: string; files: File[]; uris: string[] }
export interface PanEvent { cancelled: boolean; translationX: number; translationY: number; velocityX: number; velocityY: number; valueX?: number; valueY?: number }
export interface OverlayPosition { placement: "top" | "bottom" | "left" | "right"; bounds: Bounds }
export interface TextLayoutLine { text: string; bounds: Bounds; ascender: number; descender: number; capHeight: number; xHeight: number }
export interface WebViewNavigation { url: string; title: string; loading: boolean; canGoBack: boolean; canGoForward: boolean }

export interface TouchPoint { identifier: number; locationX: number; locationY: number; pageX: number; pageY: number; target: number; timestamp: number }
export interface GestureResponderEvent extends TouchPoint {
  touches: TouchPoint[];
  changedTouches: TouchPoint[];
  nativeEvent: GestureResponderEvent;
  stopPropagation(): void;
}
export interface ResponderGestureState { moveX: number; moveY: number; x0: number; y0: number; dx: number; dy: number; vx: number; vy: number; numberActiveTouches: number }
export interface PanActivation { activeOffsetX?: number; activeOffsetY?: number; failOffsetX?: number; failOffsetY?: number }

export type AccessibilityRole = "none" | "button" | "link" | "search" | "image" | "imagebutton" | "keyboardkey" | "text" | "adjustable" | "header" | "summary" | "alert" | "checkbox" | "combobox" | "menu" | "menubar" | "menuitem" | "progressbar" | "radio" | "radiogroup" | "scrollbar" | "spinbutton" | "switch" | "tab" | "tablist" | "timer" | "toolbar" | "list" | "grid" | "table" | "row" | "cell" | "columnheader" | "rowgroup";
export interface AccessibilityState { disabled?: boolean; selected?: boolean; checked?: boolean | "mixed"; busy?: boolean; expanded?: boolean }

export interface TouchProps {
  onTouchStart?: (e: GestureResponderEvent) => void;
  onTouchMove?: (e: GestureResponderEvent) => void;
  onTouchEnd?: (e: GestureResponderEvent) => void;
  onTouchCancel?: (e: GestureResponderEvent) => void;
  onTouchStartCapture?: (e: GestureResponderEvent) => void;
  onTouchMoveCapture?: (e: GestureResponderEvent) => void;
  onTouchEndCapture?: (e: GestureResponderEvent) => void;
  onTouchCancelCapture?: (e: GestureResponderEvent) => void;
  onStartShouldSetResponder?: (e: GestureResponderEvent, gestureState: ResponderGestureState) => boolean;
  onMoveShouldSetResponder?: (e: GestureResponderEvent, gestureState: ResponderGestureState) => boolean;
  onStartShouldSetResponderCapture?: (e: GestureResponderEvent, gestureState: ResponderGestureState) => boolean;
  onMoveShouldSetResponderCapture?: (e: GestureResponderEvent, gestureState: ResponderGestureState) => boolean;
  onResponderGrant?: (e: GestureResponderEvent) => boolean | void;
  onResponderReject?: (e: GestureResponderEvent) => void;
  onResponderStart?: (e: GestureResponderEvent) => void;
  onResponderMove?: (e: GestureResponderEvent) => void;
  onResponderEnd?: (e: GestureResponderEvent) => void;
  onResponderRelease?: (e: GestureResponderEvent) => void;
  onResponderTerminate?: (e: GestureResponderEvent) => void;
  onResponderTerminationRequest?: (e: GestureResponderEvent) => boolean;
}

/** Styles while the element is in a state, over its style and classes. */
export interface StateStyleProps {
  activeStyle?: StyleProp<ViewStyle>;
  hoverStyle?: StyleProp<ViewStyle>;
  focusStyle?: StyleProp<ViewStyle>;
  disabledStyle?: StyleProp<ViewStyle>;
  checkedStyle?: StyleProp<ViewStyle>;
  indeterminateStyle?: StyleProp<ViewStyle>;
  selectedStyle?: StyleProp<ViewStyle>;
}

export interface HostProps {
  pan?: { x?: AnimatedValue; y?: AnimatedValue };
  pinch?: { scale?: AnimatedValue };
  rotation?: { angle?: AnimatedValue };
  panActivation?: PanActivation;
  pointerEvents?: "auto" | "none" | "box-none" | "box-only";
  hitSlop?: number | Partial<Edges>;
  class?: string;
  classList?: Record<string, boolean | null | undefined>;
  delayLongPress?: number;
  accessible?: boolean;
  accessibilityLabel?: string;
  accessibilityHint?: string;
  role?: AccessibilityRole;
  accessibilityState?: AccessibilityState;
  accessibilityValue?: { min?: number; max?: number; now?: number; text?: string };
  accessibilityElementsHidden?: boolean;
  importantForAccessibility?: "auto" | "yes" | "no" | "no-hide-descendants";
  accessibilityActions?: { name: string; label?: string }[];
  accessibilityLiveRegion?: "none" | "polite" | "assertive";
  testID?: string;
  tooltip?: JSX.Element;
  focusable?: boolean;
  disabled?: boolean;
  onPress?: (e: PressEvent) => void;
  onLongPress?: (e: PressEvent) => void;
  onPressDown?: (e: PressEvent) => void;
  onPressUp?: (e: PressEvent) => void;
  onHoverIn?: () => void;
  onHoverOut?: () => void;
  onHoverMove?: (e: HoverEvent) => void;
  onContextMenu?: (e: ContextMenuEvent) => void;
  onMouseMove?: (e: Point) => void;
  onFileDragEnter?: () => void;
  onFileDragLeave?: () => void;
  onFileDrop?: (e: { uris: string[] }) => void;
  onLayout?: (bounds: Bounds) => void;
  onPanStart?: (e: PanEvent) => void;
  onPanEnd?: (e: PanEvent) => void;
  onPinchStart?: (e: { scale: number; velocity: number }) => void;
  onPinchEnd?: (e: { scale: number; velocity: number }) => void;
  onRotationStart?: (e: { rotation: number; velocity: number }) => void;
  onRotationEnd?: (e: { rotation: number; velocity: number }) => void;
  onAccessibilityAction?: (e: { actionName: string }) => void;
  onFocus?: () => void;
  onBlur?: () => void;
}

/** Props of one platform (`ios={{…}}`, …), ignored elsewhere. */
export interface IOSProps { backdropMaterial?: "ultraThin" | "thin" | "regular" | "thick" | "chrome" }
export interface AndroidProps { ripple?: { color?: ColorValue; borderless?: boolean; radius?: number; foreground?: boolean } }
export interface DesktopProps { windowDrag?: boolean }
export interface WebProps {}

export interface CommonProps extends TouchProps, HostProps, StateStyleProps {
  ref?: YNode | ((node: YNode) => void);
  children?: JSX.Element;
  ios?: IOSProps & Record<string, unknown>;
  android?: AndroidProps & Record<string, unknown>;
  desktop?: DesktopProps & Record<string, unknown>;
  web?: WebProps & Record<string, unknown>;
}

export interface ViewProps extends CommonProps {
  style?: StyleProp<ViewStyle>;
  selectable?: boolean;
  /** As accessibilityState's: for checkedStyle and checked: classes. */
  checked?: boolean;
  /** As accessibilityState's: for selectedStyle and selected: classes. */
  selected?: boolean;
}

export interface TextProps extends CommonProps {
  style?: StyleProp<TextStyle>;
  numberOfLines?: number;
  ellipsizeMode?: "tail" | "head" | "middle" | "clip";
  allowFontScaling?: boolean;
  maxFontSizeMultiplier?: number;
  selectable?: boolean;
  adjustsFontSizeToFit?: boolean;
  minimumFontScale?: number;
  onTextLayout?: (e: { lines: TextLayoutLine[] }) => void;
}

export interface ImageAsset { url: string; size: Size; scales?: number[] }
export type ResizeMode = "cover" | "contain" | "stretch" | "center" | "scale-down";

export interface ImageProps extends CommonProps {
  style?: StyleProp<ImageStyle>;
  src?: string | ImageAsset;
  resizeMode?: ResizeMode;
  placeholder?: string | ImageAsset;
  blurRadius?: number;
  fadeDuration?: number;
  alt?: string;
  onLoadStart?: () => void;
  onProgress?: (e: { loaded: number; total: number }) => void;
  onLoad?: (e: Size) => void;
  onError?: (e: { message: string }) => void;
  onLoadEnd?: () => void;
}

export interface ScrollingProps {
  pagingEnabled?: boolean;
  snapToInterval?: number;
  snapToOffsets?: number[];
  snapToAlignment?: "start" | "center" | "end";
  decelerationRate?: "normal" | "fast" | number;
  bounces?: boolean;
  contentInset?: Partial<Edges>;
  contentOffset?: Partial<Point>;
  keyboardShouldPersistTaps?: "never" | "always" | "handled" | boolean;
  keyboardDismissMode?: "none" | "on-drag" | "interactive";
  scrollEventThrottle?: number;
  disableIntervalMomentum?: boolean;
  refreshIndicator?: { height: number };
  scrollable?: boolean;
  horizontal?: boolean;
  scrollbar?: boolean;
  refreshing?: boolean;
  scrollValue?: AnimatedValue;
  pullValue?: AnimatedValue;
  onScroll?: (e: ScrollEvent) => void;
  onScrollDragStart?: (e: Point) => void;
  onScrollDragEnd?: (e: Point) => void;
  onMomentumStart?: (e: Point) => void;
  onMomentumEnd?: (e: Point) => void;
  /** It came to rest: a drag that set nothing in motion ended, or a glide, a snap, an animated scroll or the wheel's turns did. */
  onScrollEnd?: (e: Point) => void;
  onRefresh?: () => void;
}

export interface ScrollProps extends CommonProps, ScrollingProps {
  style?: StyleProp<ViewStyle>;
  /** The style of a view around the content, which scrolls with it: flexGrow: 1 makes it at least as tall as the viewport. Without it or contentClass the children are the scroll view's own. */
  contentStyle?: StyleProp<ViewStyle>;
  /** The classes of the content's view. */
  contentClass?: string;
  minimumZoomScale?: number;
  maximumZoomScale?: number;
  zoomScale?: number;
  bouncesZoom?: boolean;
  zoomable?: boolean;
  centerContent?: boolean;
}

export interface OverlayProps extends CommonProps {
  style?: StyleProp<ViewStyle>;
  anchor?: YNode | Point | Bounds | null;
  placement?: "top" | "bottom" | "left" | "right";
  align?: "start" | "center" | "end";
  offset?: number;
  margin?: number;
  onPressOutside?: (e: { pagePosition: Point }) => void;
  onCancel?: () => void;
  onPosition?: (e: OverlayPosition) => void;
}

export interface ListProps<T> extends Omit<ViewProps, "children">, ScrollingProps {
  each: readonly T[];
  header?: (() => JSX.Element) | JSX.Element;
  footer?: (() => JSX.Element) | JSX.Element;
  empty?: (() => JSX.Element) | JSX.Element;
  separator?: (() => JSX.Element) | JSX.Element;
  estimatedItemSize?: number;
  numColumns?: number;
  inverted?: boolean;
  maintainVisibleContentPosition?: { autoscrollToBottomThreshold?: number };
  stickyHeaderIndices?: number[];
  stickyHeaderOffset?: number | AnimatedNode;
  viewport?: "own" | "parent";
  /** The content's paddings and gaps. */
  contentStyle?: { padding?: number; paddingHorizontal?: number; paddingVertical?: number; paddingTop?: number; paddingRight?: number; paddingBottom?: number; paddingLeft?: number; gap?: number; rowGap?: number; columnGap?: number };
  /** The content's classes: paddings and gaps. */
  contentClass?: string;
  viewabilityConfig?: ViewabilityConfig;
  initialScrollIndex?: number;
  keyExtractor?: (item: T, index: number) => string;
  onViewableItemsChange?: (info: { viewableItems: { item: T; key: string; index: number; isViewable: boolean }[]; changed: { item: T; key: string; index: number; isViewable: boolean }[] }) => void;
  onLoadMore?: () => void;
  loadMoreThreshold?: number;
  onLoadMoreAtStart?: () => void;
  loadMoreAtStartThreshold?: number;
  children: (item: () => T, index: () => number) => JSX.Element;
}

export interface WebViewProps extends Omit<ViewProps, "children"> {
  src?: string;
  html?: string;
  baseUrl?: string;
  injectedJavaScript?: string;
  injectedJavaScriptBeforeContentLoaded?: string;
  userAgent?: string;
  scrollable?: boolean;
  javaScriptEnabled?: boolean;
  allowsBackForwardNavigationGestures?: boolean;
  onLoadStart?: (e: WebViewNavigation) => void;
  onLoad?: (e: WebViewNavigation) => void;
  onError?: (e: { message: string }) => void;
  onMessage?: (e: { data: string }) => void;
  onNavigationStateChange?: (e: WebViewNavigation) => void;
}

export interface VideoProps extends Omit<ViewProps, "children"> {
  src?: string;
  resizeMode?: ResizeMode;
  paused?: boolean;
  muted?: boolean;
  volume?: number;
  rate?: number;
  repeat?: boolean;
  controls?: boolean;
  progressUpdateInterval?: number;
  audioSession?: "default" | "playback" | "mixWithOthers";
  onLoadStart?: () => void;
  onProgress?: (e: { currentTime: number; duration: number }) => void;
  onLoad?: (e: { duration: number; size: Size }) => void;
  onError?: (e: { message: string }) => void;
  onEnd?: () => void;
  onBuffer?: (e: { isBuffering: boolean }) => void;
}

/** A node: what an element is, with its view's methods. */
export interface YNode {
  focus(): void;
  blur(): void;
  setSelection(start: number, end?: number): void;
  command(name: string, args?: unknown): void;
  scrollTo(options: { x?: number; y?: number; animated?: boolean }): void;
  zoomTo(options: { scale: number; x?: number; y?: number; animated?: boolean }): void;
  scrollToOffset(options: { offset: number; animated?: boolean }): void;
  scrollToIndex(options: { index: number; animated?: boolean; viewPosition?: number; viewOffset?: number }): void;
  scrollToEnd(options?: { animated?: boolean }): void;
  measure(): Promise<{ bounds: Bounds; pagePosition: Point } | null>;
  reload(): void;
  goBack(): void;
  goForward(): void;
  stopLoading(): void;
  injectJavaScript(script: string): void;
  postMessage(data: string): void;
  seek(seconds: number): void;
}

/** What a Go component's host takes beside its own props. */
export interface ComponentHostProps {
  class?: string;
  classList?: Record<string, boolean | null | undefined>;
  style?: StyleProp<ViewStyle>;
  activeStyle?: StyleProp<ViewStyle>;
  hoverStyle?: StyleProp<ViewStyle>;
  focusStyle?: StyleProp<ViewStyle>;
  disabledStyle?: StyleProp<ViewStyle>;
  selectedStyle?: StyleProp<ViewStyle>;
  checkedStyle?: StyleProp<ViewStyle>;
  indeterminateStyle?: StyleProp<ViewStyle>;
  testID?: string;
  ref?: unknown;
}

export type OS = "ios" | "android" | "macos" | "windows" | "linux" | "web";

export declare namespace JSX {
  type Element = YNode | ArrayElement | (() => Element) | string | number | boolean | null | undefined;
  interface ArrayElement extends Array<Element> {}
  interface ElementChildrenAttribute { children: {} }
  interface IntrinsicElements {}
}

export type { StatusBar };
