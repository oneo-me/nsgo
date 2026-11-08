// The nsgo/base module: base's components. The runtime's own come
// first; the Go components after them are declared from their types.

import type { Accessor, Easing } from "./index";
import type {
  AndroidDialogHostProps, ColorValue, CommonProps, DialogHostProps, InputHostProps, IOSDialogHostProps, IOSInputHostProps,
  IOSNavPageHostProps, IOSProps, JSX, ListItemsProps, NavPageHostProps, StyleProp, TextStyle, TransitionShader, ViewStyle, YNode,
} from "./types";

export interface IOSInputProps extends IOSProps, IOSInputHostProps {}
export interface InputProps extends Omit<CommonProps, "ios">, InputHostProps {
  style?: StyleProp<TextStyle>;
  ios?: IOSInputProps & Record<string, unknown>;
  onChange?: (e: { value: string }) => void;
  onSubmit?: (e: { text: string }) => void;
  onSelectionChange?: (e: { start: number; end: number }) => void;
  onPaste?: (e: import("./types").PasteEvent) => void;
  onCaretMove?: (e: import("./types").CaretEvent) => void;
  onKeyPress?: (e: { key: string }) => void;
  onCompositionChange?: (e: { composing: boolean; text: string }) => void;
  onContentSizeChange?: (e: { size: import("./types").Size }) => void;
}
export declare function Input(props: InputProps): YNode;

export type StatusBarForeground = "auto" | "light" | "dark";
export interface StatusBar { foreground?: StatusBarForeground; hidden?: boolean; animated?: boolean }

export type DialogChangeReason = "cancel" | "backdropPress" | "swipe";
export interface DialogProps extends DialogHostProps {
  testID?: string;
  open?: boolean;
  animation?: "slide" | "fade" | "none";
  transitionShader?: TransitionShader;
  onOpen?: () => void;
  onClose?: () => void;
  onCancel?: () => void;
  onOpenChange?: (e: { open: boolean; reason: DialogChangeReason }) => void;
  style?: StyleProp<ViewStyle>;
  class?: string;
  backdropStyle?: StyleProp<ViewStyle>;
  backdropClass?: string;
  backdropTestID?: string;
  accessibilityLabel?: string;
  children?: JSX.Element;
  ios?: IOSDialogHostProps;
  android?: AndroidDialogHostProps;
}
export declare function Dialog(props: DialogProps): JSX.Element;

export interface NavStackState<R> {
  routes: Accessor<readonly R[]>;
  push(route: R): void;
  pop(count?: number): void;
  popToRoot(): void;
  replace(route: R): void;
  setRoutes(routes: R[]): void;
  remove(route: R): void;
}
export declare function createNavStack<R>(routes: R[]): NavStackState<R>;
export interface NavStackProps<R> {
  state: NavStackState<R>;
  testID?: string;
  style?: StyleProp<ViewStyle>;
  class?: string;
  children: (route: R, index: Accessor<number>) => JSX.Element;
}
export declare function NavStack<R>(props: NavStackProps<R>): YNode;
/** The colors of a page's system header. */
export interface HeaderStyle { backgroundColor?: ColorValue; color?: ColorValue; tintColor?: ColorValue }
export interface NavPageProps extends NavPageHostProps {
  testID?: string;
  header?: JSX.Element;
  headerLeft?: JSX.Element;
  headerRight?: JSX.Element;
  headerTitle?: JSX.Element;
  headerStyle?: HeaderStyle;
  /** The system header's colors as classes: bg-*, text-* for the title, tint-* for the buttons; headerStyle wins over them. */
  headerClass?: string;
  onDismiss?: () => void;
  onBackPress?: () => void;
  style?: StyleProp<ViewStyle>;
  class?: string;
  children?: JSX.Element;
  ios?: IOSNavPageHostProps;
}
export declare function NavPage(props: NavPageProps): YNode;

export type AlertDialogRole = "default" | "cancel" | "destructive";
export type AlertDialogPresentation = "alert" | "actionSheet";
export interface AlertDialogAction { label: string; role?: AlertDialogRole; id?: string | null }
export interface AlertDialogOptions {
  title?: string | null;
  description?: string | null;
  actions: AlertDialogAction[];
  presentation?: AlertDialogPresentation;
  closeOnEscape?: boolean | null;
  closeOnBackdropPress?: boolean | null;
}
export interface AlertDialogModule { show(options: AlertDialogOptions): Promise<string | null> }
export declare const AlertDialog: AlertDialogModule;

export interface ToastEntry { id: string; value: unknown; status: ToastTransitionStatus }
export interface ToastAdvance { changed: boolean; presented: string[]; ending: string[]; removed: ToastEntry[] }
export interface ToastMotion { duration?: number | null; exitDuration?: number | null; collapsedPeek?: number | null; expandedGap?: number | null; collapsedScaleStep?: number | null; collapsedVisible?: number | null }
export declare class ToastManager {
  constructor(motion?: ToastMotion | null);
  push(id: string, value: unknown, options?: ToastOptions | null): unknown | null;
  dismiss(id: string): boolean;
  dismissAll(): string[];
  advance(paused?: boolean | null): ToastAdvance;
  entries(): ToastEntry[];
  visible(limit: number): ToastEntry[];
  get(id: string): unknown | null;
  len(): number;
  isEmpty(): boolean;
  dispose(): void;
}
/** How long a toast shows, in milliseconds; null keeps it. */
export interface ToastOptions { timeout?: number | null }
/** Where toasts stack on the screen. */
export type Anchor = "topLeft" | "topCenter" | "topRight" | "bottomLeft" | "bottomCenter" | "bottomRight" | "leftCenter" | "rightCenter";
export interface ToastStackProps {
  motion?: ToastMotion;
  placement?: Anchor;
  closeOnEscape?: boolean;
  onExpandedChange?: (e: { expanded: boolean }) => void;
  onCancel?: () => void;
  class?: string;
  style?: StyleProp<ViewStyle>;
  testID?: string;
  children?: JSX.Element;
}
export declare function ToastStack(props: ToastStackProps): JSX.Element;

/** A list of choices when it takes multiple, selected, defaultSelected or onSelectedChange. */
export interface ListProps<T> extends Pick<ListItemsProps<T>, "each" | "children" | "keyExtractor" | "onLoadMore" | "header" | "footer" | "empty"> {
  /** Any number of rows are chosen. */
  multiple?: boolean;
  /** The chosen rows' keys: keyExtractor's, else the items. */
  selected?: readonly unknown[];
  defaultSelected?: readonly unknown[];
  onSelectedChange?: (e: { selected: unknown[] }) => void;
  onActivate?: (e: { item: T; index: number }) => void;
  /** A tap only selects on touch screens, and a second activates. */
  tapSelects?: boolean;
  /** What assistive technology reads for a row, and what typeahead matches. */
  rowLabel?: (item: T, index: number) => string;
  numColumns?: number;
  estimatedItemSize?: number;
  gap?: number;
  accessibilityLabel?: string;
  testID?: string;
  class?: string;
  style?: StyleProp<ViewStyle>;
  listClass?: string;
  listStyle?: StyleProp<ViewStyle>;
  itemClass?: string;
  itemStyle?: StyleProp<ViewStyle>;
}
export declare function List<T>(props: ListProps<T>): JSX.Element;

export interface ResizablePanelProps {
  size?: number | null;
  minSize?: number;
  maxSize?: number | null;
  fixed?: boolean;
  visible?: boolean;
  onResize?: (e: { size: number }) => void;
  class?: string;
  style?: StyleProp<ViewStyle>;
  testID?: string;
  children?: JSX.Element;
}
export declare function ResizablePanel(props: ResizablePanelProps): JSX.Element;

export type { Easing };

/** The rows or columns of a DataTable in view, from start to before end. */
export interface DataTableRange { start: number; end: number }
/** A cell, a row (no column) or a column (no row) of a DataTable. */
export interface DataTableTarget { row?: number | null; column?: number | null }
/** How a DiffView shows its texts: side by side, or one above the other. */
export type DiffLayout = "split" | "unified";
/** What a press on a TextView's link does: reported to onLinkPress, or opened. */
export type TextViewLinks = "report" | "open";
