// The element and JSX types of nsgo: styles, props, events and nodes.

import type { AnimatedNode, AnimatedValue, ViewabilityConfig, ViewToken } from "./index";
import type { StatusBar } from "./base";

/** A style, or a list of styles (nested lists included) merged left to
 * right; falsy entries are skipped. */
export type StyleProp<T> = T | readonly (StyleProp<T> | false | null | undefined)[] | false | null | undefined;

export type ColorValue = string;
export type DimensionValue = number | `${number}%` | "auto";
export type LengthValue = number | `${number}%`;
export type EnvValue = `env(${string})` | `calc(${string})`;
type Animatable<T> = T | AnimatedNode;
type FlexAlign = "flex-start" | "flex-end" | "center" | "stretch" | "baseline" | "start" | "end" | "self-start" | "self-end" | "normal";
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
  alignItems?: FlexAlign;
  alignSelf?: FlexAlign | "auto";
  alignContent?: Distribute;
  justifyItems?: FlexAlign;
  justifySelf?: FlexAlign | "auto";
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

/** A shadow as numbers: the layers of boxShadow and textShadow. */
export interface ShadowLayer { x?: number; y?: number; blur?: number; spread?: number; color?: ColorValue; inset?: boolean }

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
  boxShadow?: string | ShadowLayer[];
  shadowColor?: Animatable<ColorValue>;
  shadowOffset?: { width: number; height: number };
  shadowOpacity?: Animatable<number>;
  shadowRadius?: Animatable<number>;
  filter?: string;
  backdropFilter?: string;
  maskImage?: string;
  glassEffect?: "none" | "regular" | "clear";
  mixBlendMode?: "normal" | "multiply" | "screen" | "overlay" | "darken" | "lighten" | "color-dodge" | "color-burn" | "hard-light" | "soft-light" | "difference" | "exclusion" | "hue" | "saturation" | "color" | "luminosity" | "plus-darker" | "plus-lighter";
  cursor?: "auto" | "default" | "pointer" | "text" | "vertical-text" | "wait" | "progress" | "help" | "crosshair" | "move" | "all-scroll" | "grab" | "grabbing" | "not-allowed" | "no-drop" | "zoom-in" | "zoom-out" | "col-resize" | "e-resize" | "w-resize" | "ew-resize" | "row-resize" | "n-resize" | "s-resize" | "ns-resize" | "nwse-resize" | "nw-resize" | "se-resize" | "nesw-resize" | "ne-resize" | "sw-resize" | "none";
  backfaceVisibility?: "visible" | "hidden";
  transform?: string | TransformStep[];
  translate?: number | string | (number | string)[];
  rotate?: number | string;
  scale?: number | string | number[];
  pointerEvents?: "auto" | "none" | "box-none" | "box-only";
  userSelect?: "auto" | "text" | "none";
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
  textAlign?: "auto" | "start" | "left" | "center" | "end" | "right" | "justify";
  textAlignVertical?: "auto" | "top" | "center" | "bottom";
  textDecorationLine?: "none" | "underline" | "line-through" | "underline line-through";
  textDecorationStyle?: "solid" | "double" | "dotted" | "dashed" | "wavy";
  textDecorationColor?: ColorValue;
  textDecorationThickness?: number | "auto" | "from-font";
  textUnderlineOffset?: number | "auto";
  textTransform?: "none" | "uppercase" | "lowercase" | "capitalize";
  textShadow?: string | ShadowLayer[];
  textShadowColor?: ColorValue;
  textShadowOffset?: { width: number; height: number };
  textShadowRadius?: number;
  writingDirection?: "auto" | "ltr" | "rtl";
  numberOfLines?: number;
  textOverflow?: "ellipsis" | "ellipsis-start" | "ellipsis-middle" | "clip";
  placeholderColor?: ColorValue;
  selectionColor?: ColorValue;
  caretColor?: ColorValue;
}

export interface ImageStyle extends ViewStyle {
  tintColor?: ColorValue;
  objectFit?: "cover" | "contain" | "fill" | "none" | "scale-down";
  objectPosition?: string;
}

export interface Point { x: number; y: number }
export interface Size { width: number; height: number }
export interface Bounds { origin: Point; size: Size }
export interface Edges { top: number; right: number; bottom: number; left: number }
export interface Modifiers { control: boolean; alt: boolean; shift: boolean; platform: boolean }
export interface PressEvent { position: Point; pagePosition: Point; modifiers: Modifiers; clickCount: number }
export type MouseButton = "left" | "right" | "middle" | "back" | "forward";
export interface MouseEvent { position: Point; pagePosition: Point; modifiers: Modifiers; pressedButton: MouseButton | null }
export interface ContextMenuEvent { position: Point; pagePosition: Point; modifiers: Modifiers }
export interface ScrollEvent { offset: Point; contentSize: Size; size: Size; zoomScale: number }
export interface CaretEvent { bounds: Bounds; pageBounds: Bounds }
export interface PasteEvent { text: string; html: string; files: File[]; uris: string[] }
export interface PanEvent { cancelled: boolean; translation: Point; velocity: Point; value?: Partial<Point> }
export interface OverlayPosition { placement: "top" | "bottom" | "left" | "right"; bounds: Bounds }
export interface TextLayoutLine { text: string; bounds: Bounds; ascender: number; descender: number; capHeight: number; xHeight: number }
export interface WebViewDownload { state: "started" | "finished" | "failed"; id: string; url: string; filename: string; path: string; bytes?: number; message?: string }
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

export type AccessibilityRole = "none" | "button" | "link" | "search" | "image" | "imagebutton" | "keyboardkey" | "text" | "adjustable" | "header" | "summary" | "alert" | "checkbox" | "combobox" | "menu" | "menubar" | "menuitem" | "progressbar" | "radio" | "radiogroup" | "scrollbar" | "spinbutton" | "switch" | "tab" | "tablist" | "timer" | "toolbar" | "list" | "grid" | "table" | "row" | "cell" | "columnheader" | "rowgroup" | "listbox" | "option" | "group" | "region" | "navigation" | "tree" | "treeitem" | "menuitemcheckbox" | "menuitemradio";
export interface AccessibilityState { disabled?: boolean; selected?: boolean; checked?: boolean | "mixed"; pressed?: boolean | "mixed"; busy?: boolean; expanded?: boolean }

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
  focusVisibleStyle?: StyleProp<ViewStyle>;
  focusWithinStyle?: StyleProp<ViewStyle>;
  expandedStyle?: StyleProp<ViewStyle>;
  pressedStyle?: StyleProp<ViewStyle>;
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
  onMouseEnter?: (e: MouseEvent) => void;
  onMouseLeave?: (e: MouseEvent) => void;
  onMouseMove?: (e: MouseEvent) => void;
  onContextMenu?: (e: ContextMenuEvent) => void;
  onFileDragEnter?: () => void;
  onFileDragLeave?: () => void;
  onFileDrop?: (e: { uris: string[] }) => void;
  onLayout?: (e: { bounds: Bounds }) => void;
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

export type BackdropMaterial = "ultraThin" | "thin" | "regular" | "thick" | "chrome";
export type LiveRegion = "none" | "polite" | "assertive";
export type PointerEvents = "auto" | "none" | "box-none" | "box-only";
export type RecursiveArray<T> = readonly (T | RecursiveArray<T>)[];
/** Props of one platform (`ios={{…}}`, …), ignored elsewhere. */
export interface IOSHostProps { backdropMaterial?: BackdropMaterial }
export interface AndroidHostProps { ripple?: { color?: ColorValue; borderless?: boolean; radius?: number; foreground?: boolean } }
export interface DesktopHostProps { windowDrag?: boolean }
export interface IOSProps extends IOSHostProps {}
export interface AndroidProps extends AndroidHostProps {}
export interface DesktopProps extends DesktopHostProps {}
export interface WebProps {}

export interface CommonProps extends TouchProps, HostProps, StateStyleProps {
  ref?: YNode | ((node: YNode) => void);
  children?: JSX.Element;
  ios?: IOSProps & Record<string, unknown>;
  android?: AndroidProps & Record<string, unknown>;
  desktop?: DesktopProps & Record<string, unknown>;
  web?: WebProps & Record<string, unknown>;
}

/** A view's own props. */
export interface ViewHostProps {
  selectable?: boolean;
  /** As accessibilityState's: for checkedStyle and checked: classes. */
  checked?: boolean;
  /** As accessibilityState's: for selectedStyle and selected: classes. */
  selected?: boolean;
}
export interface ViewProps extends CommonProps, ViewHostProps {
  style?: StyleProp<ViewStyle>;
}

/** A text's own props. */
export interface TextHostProps {
  allowFontScaling?: boolean;
  maxFontSizeMultiplier?: number;
  adjustsFontSizeToFit?: boolean;
  minimumFontScale?: number;
  onTextLayout?: (e: { lines: TextLayoutLine[] }) => void;
}
export interface TextProps extends CommonProps, TextHostProps {
  style?: StyleProp<TextStyle>;
}

export interface ImageAsset {
  url: string;
  /** Its size in points. */
  size: Size;
  /** The pixel densities it ships in, 1 being the imported file: `[1, 2, 3]` when `logo@2x.png` and
   * `logo@3x.png` sit next to `logo.png`. The image loads the one for the screen's pixel ratio. */
  scales?: number[];
  /** The URLs of the densities past 1, by density, which the bundle carries. */
  variants?: Record<string, string>;
}
/** An image's own props. */
export interface ImageHostProps {
  src?: string | ImageAsset;
  placeholder?: string | ImageAsset;
  blurRadius?: number;
  fadeDuration?: number;
  alt?: string;
  onLoadStart?: () => void;
  onProgress?: (e: { loaded: number; total: number }) => void;
  onLoad?: (e: { size: Size }) => void;
  onError?: (e: { message: string }) => void;
  onLoadEnd?: () => void;
}
export interface ImageProps extends CommonProps, ImageHostProps {
  style?: StyleProp<ImageStyle>;
}

export type SnapAlignment = "start" | "center" | "end";
export type KeyboardDismissMode = "none" | "on-drag" | "interactive";
/** What scroll views and lists share. */
export interface ScrollingHostProps {
  pagingEnabled?: boolean;
  snapToInterval?: number;
  snapToOffsets?: number[];
  snapToAlignment?: SnapAlignment;
  decelerationRate?: "normal" | "fast" | number;
  bounces?: boolean;
  contentInset?: Partial<Edges>;
  contentOffset?: Partial<Point>;
  keyboardShouldPersistTaps?: "never" | "always" | "handled" | boolean;
  keyboardDismissMode?: KeyboardDismissMode;
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
  onScrollDragStart?: (e: { offset: Point }) => void;
  onScrollDragEnd?: (e: { offset: Point }) => void;
  onMomentumStart?: (e: { offset: Point }) => void;
  onMomentumEnd?: (e: { offset: Point }) => void;
  /** It came to rest: a drag that set nothing in motion ended, or a glide, a snap, an animated scroll or the wheel's turns did. */
  onScrollEnd?: (e: { offset: Point }) => void;
  onRefresh?: () => void;
}

/** A scroll view's own props. */
export interface ScrollHostProps {
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
export interface ScrollProps extends CommonProps, ScrollingHostProps, ScrollHostProps {
  style?: StyleProp<ViewStyle>;
}

/** Where an overlay goes beside its anchor, and how it lines up with it. */
export type Placement = "top" | "bottom" | "left" | "right";
export type Align = "start" | "center" | "end";
/** An overlay's own props. */
export interface OverlayHostProps {
  pointerEvents?: "auto" | "none" | "box-none" | "box-only";
  anchor?: YNode | Point | Bounds | null;
  placement?: Placement;
  align?: Align;
  offset?: number;
  margin?: number;
  onPressOutside?: (e: { pagePosition: Point }) => void;
  onCancel?: () => void;
  onPosition?: (e: OverlayPosition) => void;
}
export interface OverlayProps extends CommonProps, OverlayHostProps {
  style?: StyleProp<ViewStyle>;
}

/** A list's header, footer, empty view or separator. */
export type ListSlot = (() => JSX.Element) | JSX.Element;
export type ListViewport = "own" | "parent";
/** A list's content's paddings and gaps. */
export interface ListContentStyle { padding?: number; paddingHorizontal?: number; paddingVertical?: number; paddingTop?: number; paddingRight?: number; paddingBottom?: number; paddingLeft?: number; gap?: number; rowGap?: number; columnGap?: number }
/** A list's own props but its items'. */
export interface ListHostProps {
  header?: ListSlot;
  footer?: ListSlot;
  empty?: ListSlot;
  separator?: ListSlot;
  estimatedItemSize?: number;
  numColumns?: number;
  inverted?: boolean;
  maintainVisibleContentPosition?: { autoscrollToBottomThreshold?: number };
  stickyHeaderIndices?: number[];
  stickyHeaderOffset?: number | AnimatedNode;
  viewport?: ListViewport;
  /** The content's paddings and gaps. */
  contentStyle?: ListContentStyle;
  /** The content's classes: paddings and gaps. */
  contentClass?: string;
  viewabilityConfig?: ViewabilityConfig;
  initialScrollIndex?: number;
  onLoadMore?: () => void;
  loadMoreThreshold?: number;
  onLoadMoreAtStart?: () => void;
  loadMoreAtStartThreshold?: number;
}
export interface ListProps<T> extends Omit<ViewProps, "children">, ScrollingHostProps, ListHostProps {
  each: readonly T[];
  keyExtractor?: (item: T, index: number) => string;
  onViewableItemsChange?: (info: { viewableItems: ViewToken<T>[]; changed: ViewToken<T>[] }) => void;
  children: (item: () => T, index: () => number) => JSX.Element;
}
/** What a list of items takes. */
export type ListItemsProps<T> = Pick<ListProps<T>, "each" | "children" | "keyExtractor" | "stickyHeaderIndices" | "initialScrollIndex" | "onViewableItemsChange" | "viewabilityConfig" | "refreshing" | "onRefresh" | "onLoadMore" | "loadMoreThreshold" | "header" | "footer" | "empty">;

/** A web view's own props. */
export interface WebViewHostProps {
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
  onDownload?: (e: WebViewDownload) => void;
}
export interface IOSWebViewHostProps { allowsBackForwardNavigationGestures?: boolean }
export interface DesktopWebViewHostProps { allowsBackForwardNavigationGestures?: boolean }
export interface WebViewProps extends Omit<ViewProps, "children" | "ios" | "desktop">, WebViewHostProps {
  ios?: IOSWebViewHostProps & IOSHostProps & Record<string, unknown>;
  desktop?: DesktopWebViewHostProps & DesktopHostProps & Record<string, unknown>;
}

export type AudioSession = "default" | "playback" | "mixWithOthers";
/** A video's own props. */
export interface VideoHostProps {
  src?: string;
  paused?: boolean;
  muted?: boolean;
  volume?: number;
  rate?: number;
  repeat?: boolean;
  controls?: boolean;
  progressUpdateInterval?: number;
  audioSession?: AudioSession;
  onLoadStart?: () => void;
  onProgress?: (e: { currentTime: number; duration: number }) => void;
  onLoad?: (e: { duration: number; size: Size }) => void;
  onError?: (e: { message: string }) => void;
  onEnd?: () => void;
  onBuffer?: (e: { isBuffering: boolean }) => void;
}
export interface VideoProps extends Omit<ViewProps, "children">, VideoHostProps {}

/** A shader's uniform: a number, a boolean, a CSS color, an animated
 * value, or a list of them (a vector, an array of vec4f). */
export type UniformValue = number | boolean | string | AnimatedNode | UniformValue[];
/** The values of the members of a shader's `struct Uniforms`, by name. */
export type Uniforms = Record<string, UniformValue>;
/** What every shader element takes. */
export interface ShaderHostProps {
  uniforms?: Uniforms;
  /** Stops its clock, `nsh.time`. */
  paused?: boolean;
}
/** An effect's: `nsh.progress`, and how far beyond its box, in points, it
 * draws and its pictures reach. */
export interface ShaderEffectHostProps {
  progress?: number | AnimatedNode;
  maxSampleOffset?: number;
}
/** A transition's: it runs when `value` changes (not its first), for
 * `duration` milliseconds (300 by default); without a `source` it
 * cross-fades, unless `fallback` is "none". */
export interface ShaderTransitionHostProps {
  value?: string | number | boolean;
  duration?: number;
  fallback?: "crossfade" | "none";
  maxSampleOffset?: number;
}
/** `<Shader>`: a view whose box a WGSL shader draws, under its children. */
export interface ShaderProps extends ViewProps, ShaderHostProps {
  /** The WGSL defining `fn shade(uv: vec2f) -> vec4f`. */
  source: string;
}
/** `<ShaderEffect>`: a view whose children draw through a WGSL shader,
 * which reads what they draw with `sample_content(uv)`. */
export interface ShaderEffectProps extends ViewProps, ShaderHostProps, ShaderEffectHostProps {
  source?: string;
}
/** `<ShaderTransition>`: a view whose children change through a WGSL
 * shader when its `value` changes: `sample_from(uv)` is what they drew,
 * `sample_to(uv)` what they draw now, `nsh.progress` 0 to 1. */
export interface ShaderTransitionProps extends ViewProps, ShaderHostProps, ShaderTransitionHostProps {
  source?: string;
}
/** `<ScreenEffect>`: the whole surface drawn through a WGSL shader, which
 * reads it with `sample_content(uv)`. It takes no space and has no
 * children. */
export interface ScreenEffectProps extends ShaderHostProps, ShaderEffectHostProps {
  source?: string;
}
/** `<ScreenTransition>`: the whole surface changing through a WGSL shader
 * when its `value` changes. */
export interface ScreenTransitionProps extends ShaderHostProps, ShaderTransitionHostProps {
  source?: string;
}
/** A page's or a dialog's shader: it comes and goes through it instead of
 * its `animation`, the lower side `sample_from`, the upper `sample_to`,
 * for `duration` milliseconds (350 by default). */
export interface TransitionShader {
  source: string;
  uniforms?: Uniforms;
  duration?: number;
}

// The base components' host props (nsgo/base's Input, Dialog, NavPage).
export type KeyboardType = "default" | "number-pad" | "decimal-pad" | "numeric" | "numbers-and-punctuation" | "email-address" | "phone-pad" | "url";
export type ReturnKeyType = "default" | "done" | "go" | "next" | "search" | "send";
export type AutoCapitalize = "none" | "sentences" | "words" | "characters";
export type SubmitBehavior = "submit" | "blurAndSubmit" | "newline";
export type PasteBehavior = "insert" | "report";
export type KeyboardAppearance = "default" | "light" | "dark";
/** What a field asks the system to fill it in with, as HTML's autocomplete names it. */
export type AutoComplete = "off" | "username" | "password" | "current-password" | "new-password" | "one-time-code" | "email" | "tel" | "name" | "given-name" | "family-name" | "nickname" | "organization" | "street-address" | "postal-code" | "country" | "cc-number" | "cc-exp" | "cc-csc" | (string & {});
/** iOS's name of what a field holds. */
export type TextContentType = "none" | "username" | "password" | "newPassword" | "oneTimeCode" | "emailAddress" | "telephoneNumber" | "name" | "givenName" | "familyName" | "nickname" | "organizationName" | "fullStreetAddress" | "postalCode" | "countryName" | "creditCardNumber" | (string & {});
/** An input's own props. */
export interface InputHostProps {
  value?: string;
  defaultValue?: string;
  placeholder?: string;
  masked?: boolean;
  keyboardType?: KeyboardType;
  returnKeyType?: ReturnKeyType;
  autoCapitalize?: AutoCapitalize;
  autoCorrect?: boolean;
  multiline?: boolean;
  maxLength?: number;
  readonly?: boolean;
  submitBehavior?: SubmitBehavior;
  pasteBehavior?: PasteBehavior;
  autoFocus?: boolean;
  caret?: boolean;
  autoComplete?: AutoComplete;
  selectedRange?: { start: number; end?: number };
  keyboardAppearance?: KeyboardAppearance;
  inputAccessoryViewID?: string;
  textContentType?: AutoComplete | TextContentType;
}
export interface IOSInputHostProps { keyboardAppearance?: KeyboardAppearance; inputAccessoryViewID?: string; textContentType?: AutoComplete | TextContentType }
export type ModalPresentation = "fullScreen" | "pageSheet" | "formSheet";
/** A dialog's own props. */
export interface DialogHostProps { statusBar?: StatusBar; presentation?: ModalPresentation; closeOnEscape?: boolean; closeOnBackdropPress?: boolean; closeOnSwipe?: boolean }
export interface IOSDialogHostProps { supportedOrientations?: ("portrait" | "portrait-upside-down" | "landscape" | "landscape-left" | "landscape-right")[] }
export interface AndroidDialogHostProps { statusBarTranslucent?: boolean }
export type ScreenAnimation = "default" | "slide" | "fade" | "none";
export type HeaderSystemItem = "add" | "done" | "close" | "edit" | "compose" | "search" | "share" | "trash" | "save" | "cancel";
/** A page's own props. */
export interface NavPageHostProps { title?: string; headerVisible?: boolean; headerLargeTitle?: boolean; headerTransparent?: boolean; headerBackTitle?: string; headerBackVisible?: boolean; popOnSwipe?: boolean; popOnBack?: boolean; animation?: ScreenAnimation; transitionShader?: TransitionShader; statusBar?: StatusBar }
export interface IOSNavPageHostProps { headerRightSystemItem?: HeaderSystemItem }

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
  /** WebView: gives the page input as the user's own, whose events are trusted (isTrusted): a click in CSS pixels of the viewport, text typed into what has the focus, or a key. It takes the keyboard focus; macOS only, elsewhere it does nothing. */
  sendInput(input: { type: "click"; x: number; y: number } | { type: "text"; text: string } | { type: "key"; key: "Enter" | "Tab" | "Escape" | "Backspace" }): void;
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
  focusVisibleStyle?: StyleProp<ViewStyle>;
  focusWithinStyle?: StyleProp<ViewStyle>;
  expandedStyle?: StyleProp<ViewStyle>;
  pressedStyle?: StyleProp<ViewStyle>;
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
