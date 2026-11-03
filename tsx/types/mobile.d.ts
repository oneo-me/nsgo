// The nsgo/mobile module: what iOS and Android have. Elsewhere its calls
// do nothing and its components render nothing.

import type { JSX, StyleProp, ViewStyle, YNode } from "./types";

export declare const isAvailable: boolean;
export interface InputAccessoryViewProps { nativeID: string; style?: StyleProp<ViewStyle>; class?: string; testID?: string; children?: JSX.Element }
export declare function InputAccessoryView(props: InputAccessoryViewProps): YNode | null;
export interface TabBarItem { title: string; icon?: string | null }
export interface TabBarProps { items?: TabBarItem[]; selectedIndex?: number; onChange?: (index: number) => void; style?: StyleProp<ViewStyle>; testID?: string }
export declare function TabBar(props: TabBarProps): JSX.Element;
export interface BackHandlerModule {
  /** Leaves the app. */
  exitApp(): void;
}
export interface BackListeners {
  /** Listens to the back button: `listener` returns true when it handled the press; the newest listener runs first. The returned function stops listening. */
  onBackPress(listener: () => boolean | void): () => void;
}
export declare const BackHandler: BackHandlerModule & BackListeners;
