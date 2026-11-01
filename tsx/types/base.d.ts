// The nsgo/base module: base's components. The runtime's own come
// first; the Go components after them are declared from their types.

import type { Accessor, Easing } from "./index";
import type { CommonProps, JSX, StyleProp, TextStyle, ViewStyle, YNode } from "./types";

export interface InputProps extends CommonProps {
  style?: StyleProp<TextStyle>;
  value?: string;
  defaultValue?: string;
  placeholder?: string;
  masked?: boolean;
  keyboardType?: "default" | "number-pad" | "decimal-pad" | "numeric" | "numbers-and-punctuation" | "email-address" | "phone-pad" | "url";
  returnKeyType?: "default" | "done" | "go" | "next" | "search" | "send";
  autoCapitalize?: "none" | "sentences" | "words" | "characters";
  autoCorrect?: boolean;
  multiline?: boolean;
  maxLength?: number;
  readonly?: boolean;
  submitBehavior?: "submit" | "blurAndSubmit" | "newline";
  pasteBehavior?: "insert" | "report";
  autoFocus?: boolean;
  caret?: boolean;
  autoComplete?: string;
  selectedRange?: { start: number; end?: number };
  keyboardAppearance?: "default" | "light" | "dark";
  inputAccessoryViewID?: string;
  textContentType?: string;
  onChange?: (text: string) => void;
  onSubmit?: (e: { text: string }) => void;
  onSelectionChange?: (e: { start: number; end: number }) => void;
  onPaste?: (e: import("./types").PasteEvent) => void;
  onCaretMove?: (e: import("./types").CaretEvent) => void;
  onKeyPress?: (e: { key: string }) => void;
  onCompositionChange?: (e: { composing: boolean; text: string }) => void;
  onContentSizeChange?: (e: import("./types").Size) => void;
}
export declare function Input(props: InputProps): YNode;

export type StatusBarForeground = "auto" | "light" | "dark";
export interface StatusBar { foreground?: StatusBarForeground; hidden?: boolean; animated?: boolean }

export type DialogChangeReason = "cancel" | "backdropPress" | "swipe";
export interface DialogProps {
  testID?: string;
  open?: boolean;
  animation?: "slide" | "fade" | "none";
  presentation?: "fullScreen" | "pageSheet" | "formSheet";
  closeOnEscape?: boolean;
  closeOnBackdropPress?: boolean;
  closeOnSwipe?: boolean;
  statusBar?: StatusBar;
  onOpen?: () => void;
  onClose?: () => void;
  onCancel?: () => void;
  onOpenChange?: (open: boolean, reason: DialogChangeReason) => void;
  style?: StyleProp<ViewStyle>;
  class?: string;
  backdropStyle?: StyleProp<ViewStyle>;
  backdropClass?: string;
  accessibilityLabel?: string;
  children?: JSX.Element;
  ios?: { supportedOrientations?: ("portrait" | "portrait-upside-down" | "landscape" | "landscape-left" | "landscape-right")[] };
  android?: { statusBarTranslucent?: boolean };
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
export interface NavPageProps {
  testID?: string;
  title?: string;
  header?: JSX.Element;
  headerLeft?: JSX.Element;
  headerRight?: JSX.Element;
  headerTitle?: JSX.Element;
  headerVisible?: boolean;
  headerLargeTitle?: boolean;
  headerTransparent?: boolean;
  headerBackTitle?: string;
  headerBackVisible?: boolean;
  headerStyle?: { backgroundColor?: string; color?: string; tintColor?: string };
  /** The system header's colors as classes: bg-*, text-* for the title, tint-* for the buttons; headerStyle wins over them. */
  headerClass?: string;
  /** How the page enters and leaves. */
  animation?: "default" | "slide" | "fade" | "none";
  popOnSwipe?: boolean;
  popOnBack?: boolean;
  statusBar?: StatusBar;
  onDismiss?: () => void;
  onBackPress?: () => void;
  style?: StyleProp<ViewStyle>;
  class?: string;
  children?: JSX.Element;
  ios?: { headerRightSystemItem?: "add" | "done" | "close" | "edit" | "compose" | "search" | "share" | "trash" | "save" | "cancel" };
}
export declare function NavPage(props: NavPageProps): YNode;

export interface AlertDialogAction { label: string; role?: "default" | "cancel" | "destructive"; id?: string | null }
export interface AlertDialogOptions {
  title?: string | null;
  description?: string | null;
  actions: AlertDialogAction[];
  presentation?: "alert" | "actionSheet";
  closeOnEscape?: boolean | null;
  closeOnBackdropPress?: boolean | null;
}
export declare const AlertDialog: { show(options: AlertDialogOptions): Promise<string | null> };

export type ToastTransitionStatus = "starting" | "present" | "ending";
export interface ToastEntry { id: string; value: unknown; status: ToastTransitionStatus }
export interface ToastAdvance { changed: boolean; presented: string[]; ending: string[]; removed: ToastEntry[] }
export interface ToastMotion { duration?: number | null; exitDuration?: number | null; collapsedPeek?: number | null; expandedGap?: number | null; collapsedScaleStep?: number | null; collapsedVisible?: number | null }
export declare class ToastManager {
  constructor(motion?: ToastMotion | null);
  push(id: string, value: unknown, options?: { timeout?: number | null } | null): unknown | null;
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
export interface ToastStackProps {
  motion?: ToastMotion;
  placement?: "topLeft" | "topCenter" | "topRight" | "bottomLeft" | "bottomCenter" | "bottomRight" | "leftCenter" | "rightCenter";
  closeOnEscape?: boolean;
  onExpandedChange?: (expanded: boolean) => void;
  onCancel?: () => void;
  class?: string;
  style?: StyleProp<ViewStyle>;
  testID?: string;
  children?: JSX.Element;
}
export declare function ToastStack(props: ToastStackProps): JSX.Element;

export interface ResizablePanelProps {
  size?: number | null;
  minSize?: number;
  maxSize?: number | null;
  fixed?: boolean;
  visible?: boolean;
  onResize?: (size: number) => void;
  class?: string;
  style?: StyleProp<ViewStyle>;
  testID?: string;
  children?: JSX.Element;
}
export declare function ResizablePanel(props: ResizablePanelProps): JSX.Element;

export type { Easing };
