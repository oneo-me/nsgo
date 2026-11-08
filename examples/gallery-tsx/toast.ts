import { signal } from "nsgo";
import type { ToastsRef } from "nsgo/ui";

/** The app's toasts, which the Toast demo drives. */
export const toasts = signal<ToastsRef | null>(null);
