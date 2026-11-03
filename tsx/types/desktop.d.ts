// The nsgo/desktop module: what macOS, Windows, Linux and browsers in the
// desktop idiom have. Elsewhere its calls do nothing.

import type { Accessor } from "./index";
import type { Point, Size } from "./types";

export declare const isAvailable: boolean;
/** The system's title bar area, in which the app's own bar leaves room for the window controls. */
export interface TitleBarArea { height: number; left: number; right: number }
export declare const titleBar: Accessor<TitleBarArea>;
export type MenuRole = "about" | "hide" | "hideOthers" | "showAll" | "quit" | "undo" | "redo" | "cut" | "copy" | "paste" | "selectAll" | "minimize" | "zoom" | "toggleFullScreen" | "close";
export type MenuItem =
  | { type: "separator" }
  | { label?: string; role?: MenuRole; shortcut?: string;
      /** Whether the shortcut chooses it while a text field has the keyboard focus: "always" (default), "auto" (unless the field uses the key) or "notEditing". */
      when?: "always" | "auto" | "notEditing"; disabled?: boolean; checked?: boolean; onSelect?: () => void; submenu?: readonly (MenuItem | null | undefined | false)[] };
export interface MenuModule {
  setApplicationMenu(items: readonly (MenuItem | null | undefined | false)[] | null): void;
  popup(items: readonly (MenuItem | null | undefined | false)[], at?: Point): Promise<void>;
}
export declare const Menu: MenuModule;
/** Whether the window's menu bar shows (Windows, Linux). */
export type MenuBarMode = "shown" | "autoHide";
/** How the window's title bar looks. */
export type TitleBarStyle = "default" | "hidden";
/** A window's frame: the part of it to move, in points; a field left out keeps its value. */
export interface WindowFrame { origin?: Partial<Point>; size?: Partial<Size>; animated?: boolean }
/** Where the window sits among the others. */
export type WindowLevel = "normal" | "floating" | "desktopIcon";
export interface WindowModule {
  setTitle(title: string): void;
  setTitleBar(style: TitleBarStyle, height?: number | null): void;
  setMenuBar(mode: MenuBarMode): void;
  minimize(): void;
  toggleMaximize(): void;
  toggleFullScreen(): void;
  setFrame(frame: WindowFrame): void;
  setLevel(level: WindowLevel): void;
  orderFront(): void;
  close(): void;
}
export declare const Window: WindowModule;
