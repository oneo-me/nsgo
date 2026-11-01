// The globals of a TSX app: control flow, timers and the console. The
// web APIs (fetch, URL, Blob, streams, ...) are TypeScript's DOM lib's.

/** `<If when={…} else={…}>…</If>`: its children while `when` is truthy,
 * else `else`; a function child gets an accessor of `when`'s value. */
declare function If<T>(props: {
  when: T;
  else?: import("./types").JSX.Element;
  children?: import("./types").JSX.Element | ((value: () => NonNullable<T>) => import("./types").JSX.Element);
}): import("./types").JSX.Element;
/** `<Each of={items} key={…}>{(item, index) => …}</Each>`: a row per
 * item, kept by key (default: the item; "index": by position). */
declare function Each<T>(props: {
  of: readonly T[] | null | undefined;
  key?: "index" | ((item: T, index: number) => unknown);
  children: (item: () => T, index: () => number) => import("./types").JSX.Element;
}): import("./types").JSX.Element;
