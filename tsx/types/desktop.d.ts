// The nsgo/desktop module: what macOS, Windows, Linux and browsers in the
// desktop idiom have. Elsewhere its calls do nothing.

import type { Accessor } from "./index";
import type { Point, Size } from "./types";

export declare const isAvailable: boolean;
export declare const titleBar: Accessor<{ height: number; left: number; right: number }>;
export type MenuRole = "about" | "hide" | "hideOthers" | "showAll" | "quit" | "undo" | "redo" | "cut" | "copy" | "paste" | "selectAll" | "minimize" | "zoom" | "toggleFullScreen" | "close";
export type MenuItem =
  | { type: "separator" }
  | { label?: string; role?: MenuRole; shortcut?: string;
      /** Whether the shortcut chooses it while a text field has the keyboard focus: "always" (default), "auto" (unless the field uses the key) or "notEditing". */
      when?: "always" | "auto" | "notEditing"; disabled?: boolean; checked?: boolean; onSelect?: () => void; submenu?: readonly (MenuItem | null | undefined | false)[] };
export declare const Menu: {
  setApplicationMenu(items: readonly (MenuItem | null | undefined | false)[] | null): void;
  popup(items: readonly (MenuItem | null | undefined | false)[], at?: Point): Promise<void>;
};
export declare const Window: {
  setTitle(title: string): void;
  setTitleBar(style: "default" | "hidden", height?: number | null): void;
  setMenuBar(mode: "shown" | "autoHide"): void;
  minimize(): void;
  toggleMaximize(): void;
  toggleFullScreen(): void;
  setFrame(frame: { origin?: Partial<Point>; size?: Partial<Size>; animated?: boolean }): void;
  setLevel(level: "normal" | "floating" | "desktopIcon"): void;
  orderFront(): void;
  close(): void;
};
