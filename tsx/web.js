// The web APIs TSX apps get as globals (NSH's web/): events, aborts,
// encoding, URLs, blobs, form data, headers, requests, responses and
// streams. They are thin classes; what needs the system (fetch, UTF-8,
// base64, URL parsing) is Go's, through the natives object this script
// is called with.
(function (g, native) {
  "use strict";
  const hidden = (o, k, v) => Object.defineProperty(o, k, { value: v, writable: true, configurable: true });

  // The legacy codes of DOMException's names.
  const domCodes = {
    IndexSizeError: 1, HierarchyRequestError: 3, WrongDocumentError: 4, InvalidCharacterError: 5,
    NoModificationAllowedError: 7, NotFoundError: 8, NotSupportedError: 9, InvalidStateError: 11, SyntaxError: 12,
    InvalidModificationError: 13, NamespaceError: 14, InvalidAccessError: 15, TypeMismatchError: 17,
    SecurityError: 18, NetworkError: 19, AbortError: 20, URLMismatchError: 21, QuotaExceededError: 22,
    TimeoutError: 23, InvalidNodeTypeError: 24, DataCloneError: 25,
  };

  class DOMException extends Error {
    constructor(message = "", name = "Error") {
      super(String(message));
      hidden(this, "name", String(name));
    }
    get code() {
      return domCodes[this.name] ?? 0;
    }
    get [Symbol.toStringTag]() {
      return "DOMException";
    }
  }

  class Event {
    constructor(type, init = {}) {
      if (arguments.length === 0) throw new TypeError("Event needs a type");
      this.type = String(type);
      this.bubbles = !!init.bubbles;
      this.cancelable = !!init.cancelable;
      this.composed = !!init.composed;
      this.isTrusted = false;
      this.defaultPrevented = false;
      this.eventPhase = Event.NONE;
      this.target = null;
      this.currentTarget = null;
      this.timeStamp = native.now();
      hidden(this, "_stop", false);
    }
    get returnValue() {
      return !this.defaultPrevented;
    }
    preventDefault() {
      if (this.cancelable) this.defaultPrevented = true;
    }
    stopPropagation() {}
    stopImmediatePropagation() {
      this._stop = true;
    }
  }
  for (const [name, value] of [["NONE", 0], ["CAPTURING_PHASE", 1], ["AT_TARGET", 2], ["BUBBLING_PHASE", 3]]) {
    Object.defineProperty(Event, name, { value, enumerable: true });
    Object.defineProperty(Event.prototype, name, { value, enumerable: true });
  }

  class CustomEvent extends Event {
    constructor(type, init = {}) {
      super(type, init);
      this.detail = init.detail ?? null;
    }
  }

  class MessageEvent extends Event {
    constructor(type, init = {}) {
      super(type, init);
      this.data = init.data;
    }
  }

  class EventTarget {
    constructor() {
      hidden(this, "_listeners", new Map());
    }
    addEventListener(type, listener, options) {
      if (!listener) return;
      const once = typeof options === "object" && !!options?.once;
      const list = this._listeners.get(type) ?? [];
      if (list.some((l) => l.listener === listener)) return;
      list.push({ listener, once });
      this._listeners.set(type, list);
      const signal = typeof options === "object" ? options?.signal : undefined;
      signal?.addEventListener("abort", () => this.removeEventListener(type, listener));
    }
    removeEventListener(type, listener) {
      const list = this._listeners.get(type);
      if (list) this._listeners.set(type, list.filter((l) => l.listener !== listener));
    }
    dispatchEvent(event) {
      // An event in dispatch is not dispatched again, as the DOM has it.
      if (event._dispatching) throw new DOMException("The event is already being dispatched", "InvalidStateError");
      event._dispatching = true;
      try {
        event.target = event.currentTarget = this;
        event.eventPhase = Event.AT_TARGET;
        // An event handler (onabort) is a listener where it was first set.
        const list = [...(this._listeners.get(event.type) ?? [])];
        for (const l of list) {
          if (event._stop) break;
          if (l.once) this.removeEventListener(event.type, l.listener);
          if (typeof l.listener === "function") l.listener.call(this, event);
          else l.listener.handleEvent(event);
        }
      } finally {
        event._dispatching = false;
        event.eventPhase = Event.NONE;
        event.currentTarget = null;
      }
      return !event.defaultPrevented;
    }
  }

  // handlerProperty gives cls an event handler property, on<type>, which
  // runs as a listener added where it was first set.
  const handlerProperty = (cls, type) => {
    Object.defineProperty(cls.prototype, "on" + type, {
      get() {
        return this["_on" + type] ?? null;
      },
      set(fn) {
        if (this["_on" + type] === undefined) {
          this.addEventListener(type, (e) => {
            const h = this["_on" + type];
            if (typeof h === "function") h.call(this, e);
          });
        }
        hidden(this, "_on" + type, typeof fn === "function" ? fn : null);
      },
    });
  };

  class AbortSignal extends EventTarget {
    constructor() {
      super();
      this.aborted = false;
      this.reason = undefined;
    }
    throwIfAborted() {
      if (this.aborted) throw this.reason;
    }
    static abort(reason) {
      const c = new AbortController();
      c.abort(reason);
      return c.signal;
    }
    static timeout(ms) {
      const c = new AbortController();
      setTimeout(() => c.abort(new DOMException("The operation timed out.", "TimeoutError")), ms);
      return c.signal;
    }
    static any(signals) {
      const c = new AbortController();
      for (const s of signals) {
        if (s.aborted) {
          c.abort(s.reason);
          break;
        }
        s.addEventListener("abort", () => c.abort(s.reason), { once: true });
      }
      return c.signal;
    }
  }

  handlerProperty(AbortSignal, "abort");

  class AbortController {
    constructor() {
      this.signal = new AbortSignal();
    }
    abort(reason = new DOMException("This operation was aborted", "AbortError")) {
      const s = this.signal;
      if (s.aborted) return;
      s.aborted = true;
      s.reason = reason;
      s.dispatchEvent(new Event("abort"));
    }
  }

  class TextEncoder {
    get encoding() {
      return "utf-8";
    }
    encode(s = "") {
      return new Uint8Array(native.utf8Encode(String(s)));
    }
    encodeInto(s, dest) {
      // Whole characters while they fit: read counts UTF-16 units.
      s = String(s);
      let read = 0, written = 0;
      for (const ch of s) {
        const bytes = this.encode(ch);
        if (written + bytes.length > dest.length) break;
        dest.set(bytes, written);
        written += bytes.length;
        read += ch.length;
      }
      return { read, written };
    }
  }

  const bytesOf = (v) => {
    if (v instanceof ArrayBuffer) return new Uint8Array(v);
    if (ArrayBuffer.isView(v)) return new Uint8Array(v.buffer, v.byteOffset, v.byteLength);
    throw new TypeError("not a buffer source");
  };

  // The labels of the encodings TextDecoder reads, as the Encoding
  // standard names them.
  const encodingLabels = {};
  for (const [name, labels] of [
    ["utf-8", "unicode-1-1-utf-8 unicode11utf8 unicode20utf8 utf-8 utf8 x-unicode20utf8"],
    ["utf-16le", "csunicode iso-10646-ucs-2 ucs-2 unicode unicodefeff utf-16 utf-16le"],
    ["utf-16be", "unicodefffe utf-16be"],
    ["windows-1252", "ansi_x3.4-1968 ascii cp1252 cp819 csisolatin1 ibm819 iso-8859-1 iso-ir-100 iso8859-1 " +
      "iso88591 iso_8859-1 iso_8859-1:1987 l1 latin1 us-ascii windows-1252 x-cp1252"],
  ]) {
    for (const l of labels.split(" ")) encodingLabels[l] = name;
  }
  // windows-1252's characters for 0x80 to 0x9f; the other bytes are
  // their code points.
  const cp1252 = [0x20ac, 0x81, 0x201a, 0x192, 0x201e, 0x2026, 0x2020, 0x2021, 0x2c6, 0x2030, 0x160, 0x2039, 0x152,
    0x8d, 0x17d, 0x8f, 0x90, 0x2018, 0x2019, 0x201c, 0x201d, 0x2022, 0x2013, 0x2014, 0x2dc, 0x2122, 0x161, 0x203a,
    0x153, 0x9d, 0x17e, 0x178];

  class TextDecoder {
    constructor(label = "utf-8", options = {}) {
      const name = encodingLabels[String(label).trim().toLowerCase()];
      if (!name) throw new RangeError(`The encoding "${label}" is not supported`);
      hidden(this, "_encoding", name);
      this.fatal = !!options.fatal;
      this.ignoreBOM = !!options.ignoreBOM;
      hidden(this, "_pending", new Uint8Array(0));
    }
    get encoding() {
      return this._encoding;
    }
    decode(input, options = {}) {
      let bytes = input === undefined ? new Uint8Array(0) : bytesOf(input);
      if (this._pending.length) {
        const joined = new Uint8Array(this._pending.length + bytes.length);
        joined.set(this._pending);
        joined.set(bytes, this._pending.length);
        bytes = joined;
      }
      if (this._encoding === "windows-1252") {
        this._pending = new Uint8Array(0);
        let out = "";
        for (const b of bytes) out += String.fromCharCode(b >= 0x80 && b < 0xa0 ? cp1252[b - 0x80] : b);
        return out;
      }
      if (this._encoding !== "utf-8") {
        const end = options.stream ? bytes.length & ~1 : bytes.length;
        this._pending = bytes.slice(end);
        const be = this._encoding === "utf-16be";
        let out = "";
        for (let i = 0; i + 1 < end; i += 2) out += String.fromCharCode(be ? bytes[i] << 8 | bytes[i + 1] : bytes[i + 1] << 8 | bytes[i]);
        if (end % 2 === 1) {
          if (this.fatal) throw new TypeError("The encoded data was not valid.");
          out += "\ufffd";
        }
        return !this.ignoreBOM && out.charCodeAt(0) === 0xfeff ? out.slice(1) : out;
      }
      let end = bytes.length;
      if (options.stream) {
        // Keep an incomplete sequence at the end for the next call.
        let i = end - 1, back = 0;
        while (i >= 0 && back < 3 && (bytes[i] & 0xc0) === 0x80) i--, back++;
        if (i >= 0) {
          const b = bytes[i], need = b >= 0xf0 ? 4 : b >= 0xe0 ? 3 : b >= 0xc0 ? 2 : 1;
          if (need > back + 1) end = i;
        }
      }
      this._pending = bytes.slice(end);
      const out = native.utf8Decode(bytes.slice(0, end).buffer, this.fatal);
      if (out === null) throw new TypeError("The encoded data was not valid.");
      return !this.ignoreBOM && out.charCodeAt(0) === 0xfeff ? out.slice(1) : out;
    }
  }

  const atob = (s) => {
    const out = native.atob(String(s));
    if (out === null) throw new DOMException("The string to be decoded is not correctly encoded.", "InvalidCharacterError");
    return out;
  };
  const btoa = (s) => {
    const out = native.btoa(String(s));
    if (out === null) throw new DOMException("The string to be encoded contains characters outside of the Latin1 range.", "InvalidCharacterError");
    return out;
  };

  class URLSearchParams {
    constructor(init = "") {
      hidden(this, "_list", []);
      hidden(this, "_url", null);
      if (typeof init === "string") {
        this._parse(init);
      } else if (init instanceof URLSearchParams) {
        this._list = init._list.map((p) => [...p]);
      } else if (init && typeof init[Symbol.iterator] === "function") {
        for (const [k, v] of init) this._list.push([String(k), String(v)]);
      } else if (init && typeof init === "object") {
        for (const k of Object.keys(init)) this._list.push([k, String(init[k])]);
      }
    }
    _parse(s) {
      this._list = [];
      if (s.startsWith("?")) s = s.slice(1);
      for (const part of s.split("&")) {
        if (!part) continue;
        const i = part.indexOf("=");
        const k = i < 0 ? part : part.slice(0, i), v = i < 0 ? "" : part.slice(i + 1);
        this._list.push([native.formDecode(k), native.formDecode(v)]);
      }
    }
    _changed() {
      if (this._url) this._url._setSearch(this.toString());
    }
    get size() {
      return this._list.length;
    }
    append(k, v) {
      this._list.push([String(k), String(v)]);
      this._changed();
    }
    delete(k, v) {
      this._list = this._list.filter(([a, b]) => a !== String(k) || (v !== undefined && b !== String(v)));
      this._changed();
    }
    get(k) {
      return this._list.find(([a]) => a === String(k))?.[1] ?? null;
    }
    getAll(k) {
      return this._list.filter(([a]) => a === String(k)).map(([, b]) => b);
    }
    has(k, v) {
      return this._list.some(([a, b]) => a === String(k) && (v === undefined || b === String(v)));
    }
    set(k, v) {
      k = String(k);
      const i = this._list.findIndex(([a]) => a === k);
      if (i < 0) this._list.push([k, String(v)]);
      else {
        this._list[i][1] = String(v);
        this._list = this._list.filter(([a], j) => a !== k || j === i);
      }
      this._changed();
    }
    sort() {
      this._list.sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0));
      this._changed();
    }
    forEach(fn, self) {
      for (const [k, v] of this._list) fn.call(self, v, k, this);
    }
    keys() {
      return this._list.map(([k]) => k)[Symbol.iterator]();
    }
    values() {
      return this._list.map(([, v]) => v)[Symbol.iterator]();
    }
    entries() {
      return this._list.map((p) => [...p])[Symbol.iterator]();
    }
    [Symbol.iterator]() {
      return this.entries();
    }
    toString() {
      return this._list.map(([k, v]) => native.formEncode(k) + "=" + native.formEncode(v)).join("&");
    }
  }

  class URL {
    constructor(url, base) {
      const parts = native.parseURL(String(url), base === undefined ? undefined : String(base));
      if (!parts) throw new TypeError(`Invalid URL: ${url}`);
      hidden(this, "_p", parts);
      hidden(this, "_params", null);
    }
    static canParse(url, base) {
      return !!native.parseURL(String(url), base === undefined ? undefined : String(base));
    }
    static parse(url, base) {
      try {
        return new URL(url, base);
      } catch {
        return null;
      }
    }
    _set(k, v) {
      const p = native.setURL(this._p.href, k, String(v));
      if (p) this._p = p;
      if (k === "search" && this._params) this._params._parse(this._p.search);
    }
    _setSearch(s) {
      const p = native.setURL(this._p.href, "search", s);
      if (p) this._p = p;
    }
    get href() { return this._p.href; }
    set href(v) {
      const p = native.parseURL(String(v));
      if (!p) throw new TypeError(`Invalid URL: ${v}`);
      this._p = p;
      if (this._params) this._params._parse(p.search);
    }
    get origin() { return this._p.origin; }
    get protocol() { return this._p.protocol; }
    set protocol(v) { this._set("protocol", v); }
    get username() { return this._p.username; }
    set username(v) { this._set("username", v); }
    get password() { return this._p.password; }
    set password(v) { this._set("password", v); }
    get host() { return this._p.host; }
    set host(v) { this._set("host", v); }
    get hostname() { return this._p.hostname; }
    set hostname(v) { this._set("hostname", v); }
    get port() { return this._p.port; }
    set port(v) { this._set("port", v); }
    get pathname() { return this._p.pathname; }
    set pathname(v) { this._set("pathname", v); }
    get search() { return this._p.search; }
    set search(v) { this._set("search", v); }
    get hash() { return this._p.hash; }
    set hash(v) { this._set("hash", v); }
    get searchParams() {
      if (!this._params) {
        this._params = new URLSearchParams(this._p.search);
        this._params._url = this;
      }
      return this._params;
    }
    toString() { return this.href; }
    toJSON() { return this.href; }
    // A blob: URL names a blob's bytes in this VM until revoked: fetch
    // reads them and an image shows them.
    static createObjectURL(blob) {
      if (!(blob instanceof Blob)) throw new TypeError("createObjectURL takes a Blob");
      return native.blobURL(blob._bytes.slice(), blob.type);
    }
    static revokeObjectURL(url) {
      native.revokeBlobURL(String(url));
    }
  }

  // Blob parts are kept as bytes; a Blob made of others shares nothing.
  const partBytes = (part) => {
    if (part instanceof Blob) return part._bytes;
    if (part instanceof ArrayBuffer || ArrayBuffer.isView(part)) return bytesOf(part).slice();
    return new TextEncoder().encode(String(part));
  };

  class Blob {
    constructor(parts = [], options = {}) {
      const chunks = Array.from(parts, partBytes);
      const size = chunks.reduce((n, c) => n + c.length, 0);
      const bytes = new Uint8Array(size);
      let at = 0;
      for (const c of chunks) bytes.set(c, at), (at += c.length);
      hidden(this, "_bytes", bytes);
      // A type with a character outside U+0020 to U+007E is none, as
      // the File API says.
      const type = options.type === undefined ? "" : String(options.type);
      this.type = /^[\x20-\x7e]*$/.test(type) ? type.toLowerCase() : "";
    }
    get size() {
      return this._bytes.length;
    }
    slice(start = 0, end = this.size, type = "") {
      const b = new Blob([], { type });
      b._bytes = this._bytes.slice(start, end);
      return b;
    }
    async arrayBuffer() {
      return this._bytes.slice().buffer;
    }
    async bytes() {
      return this._bytes.slice();
    }
    async text() {
      return new TextDecoder().decode(this._bytes);
    }
    stream() {
      const bytes = this._bytes.slice();
      return new ReadableStream({
        start(c) {
          if (bytes.length) c.enqueue(bytes);
          c.close();
        },
      });
    }
  }

  class File extends Blob {
    constructor(parts, name, options = {}) {
      super(parts, options);
      this.name = String(name);
      this.lastModified = options.lastModified ?? Date.now();
      this.webkitRelativePath = "";
    }
  }

  class FormData {
    constructor() {
      hidden(this, "_list", []);
    }
    _entry(name, value, filename) {
      if (value instanceof Blob) {
        if (!(value instanceof File) || filename !== undefined)
          value = new File([value], filename ?? (value instanceof File ? value.name : "blob"), { type: value.type });
        return [String(name), value];
      }
      return [String(name), String(value)];
    }
    append(name, value, filename) {
      this._list.push(this._entry(name, value, filename));
    }
    set(name, value, filename) {
      const e = this._entry(name, value, filename);
      const i = this._list.findIndex(([k]) => k === e[0]);
      if (i < 0) return void this._list.push(e);
      this._list[i] = e;
      this._list = this._list.filter(([k], j) => k !== e[0] || j === i);
    }
    delete(name) {
      this._list = this._list.filter(([k]) => k !== String(name));
    }
    get(name) {
      return this._list.find(([k]) => k === String(name))?.[1] ?? null;
    }
    getAll(name) {
      return this._list.filter(([k]) => k === String(name)).map(([, v]) => v);
    }
    has(name) {
      return this._list.some(([k]) => k === String(name));
    }
    forEach(fn, self) {
      for (const [k, v] of this._list) fn.call(self, v, k, this);
    }
    keys() {
      return this._list.map(([k]) => k)[Symbol.iterator]();
    }
    values() {
      return this._list.map(([, v]) => v)[Symbol.iterator]();
    }
    entries() {
      return this._list.map((e) => [...e])[Symbol.iterator]();
    }
    [Symbol.iterator]() {
      return this.entries();
    }
  }

  // A header's name is an HTTP token; its value, trimmed, has no NUL,
  // CR or LF.
  const headerName = (k) => {
    k = String(k);
    if (!/^[!#$%&'*+\-.^_`|~0-9A-Za-z]+$/.test(k)) throw new TypeError(`Invalid header name: "${k}"`);
    return k.toLowerCase();
  };
  const headerValue = (v) => {
    v = String(v).replace(/^[\t\n\r ]+|[\t\n\r ]+$/g, "");
    if (/[\0\r\n]/.test(v)) throw new TypeError(`Invalid header value: "${v}"`);
    return v;
  };

  class Headers {
    constructor(init) {
      hidden(this, "_map", new Map());
      if (init instanceof Headers) init.forEach((v, k) => this.append(k, v));
      else if (init && typeof init[Symbol.iterator] === "function") {
        for (const pair of init) {
          const kv = [...pair];
          if (kv.length !== 2) throw new TypeError("A header pair has a name and a value");
          this.append(kv[0], kv[1]);
        }
      } else if (init && typeof init === "object") for (const k of Object.keys(init)) this.append(k, init[k]);
    }
    append(k, v) {
      k = headerName(k);
      v = headerValue(v);
      const old = this._map.get(k);
      this._map.set(k, old === undefined ? v : k === "set-cookie" ? old + "\n" + v : old + ", " + v);
    }
    set(k, v) {
      this._map.set(headerName(k), headerValue(v));
    }
    get(k) {
      const v = this._map.get(String(k).toLowerCase());
      return v === undefined ? null : v.replaceAll("\n", ", ");
    }
    getSetCookie() {
      return this._map.get("set-cookie")?.split("\n") ?? [];
    }
    has(k) {
      return this._map.has(String(k).toLowerCase());
    }
    delete(k) {
      this._map.delete(String(k).toLowerCase());
    }
    forEach(fn, self) {
      for (const [k, v] of this.entries()) fn.call(self, v, k, this);
    }
    keys() {
      return [...this.entries()].map(([k]) => k)[Symbol.iterator]();
    }
    values() {
      return [...this.entries()].map(([, v]) => v)[Symbol.iterator]();
    }
    entries() {
      // Set-Cookie lists each value, as the Fetch standard sorts and
      // combines.
      const out = [];
      for (const k of [...this._map.keys()].sort()) {
        if (k === "set-cookie") for (const v of this.getSetCookie()) out.push([k, v]);
        else out.push([k, this.get(k)]);
      }
      return out[Symbol.iterator]();
    }
    [Symbol.iterator]() {
      return this.entries();
    }
  }

  // Streams (WHATWG Streams): readable streams with a default or a byte
  // controller, default and BYOB readers, writable and transform streams,
  // piping and the queuing strategies, after the standard's algorithms.
  const resolved = (v) => Promise.resolve(v);
  const rejected = (e) => Promise.reject(e);
  const deferred = () => {
    const d = {};
    d.promise = new Promise((resolve, reject) => ((d.resolve = resolve), (d.reject = reject)));
    return d;
  };
  // A promise whose rejection is handled, as the standard marks them.
  const quiet = (p) => (p.catch(() => {}), p);
  const strategyOf = (strategy, defaultHWM) => {
    const hwm = strategy?.highWaterMark === undefined ? defaultHWM : Number(strategy.highWaterMark);
    if (Number.isNaN(hwm) || hwm < 0) throw new RangeError("Invalid highWaterMark");
    const size = strategy?.size;
    return { hwm, size: size ? (c) => Number(size(c)) : () => 1 };
  };
  const call = (o, name, args, fallback) => {
    const f = o?.[name];
    if (typeof f !== "function") return resolved(fallback);
    try {
      return resolved(f.apply(o, args));
    } catch (e) {
      return rejected(e);
    }
  };

  class ByteLengthQueuingStrategy {
    constructor({ highWaterMark }) {
      hidden(this, "_hwm", Number(highWaterMark));
    }
    get highWaterMark() {
      return this._hwm;
    }
    get size() {
      return (chunk) => chunk.byteLength;
    }
  }
  class CountQueuingStrategy {
    constructor({ highWaterMark }) {
      hidden(this, "_hwm", Number(highWaterMark));
    }
    get highWaterMark() {
      return this._hwm;
    }
    get size() {
      return () => 1;
    }
  }

  // A queue of chunks with their sizes.
  const queue = () => ({ items: [], total: 0 });
  const enqueueSized = (q, value, size) => {
    if (!(size >= 0) || size === Infinity) throw new RangeError("Invalid chunk size");
    q.items.push({ value, size });
    q.total += size;
  };
  const dequeue = (q) => {
    const it = q.items.shift();
    q.total = Math.max(q.total - it.size, 0);
    return it.value;
  };

  class ReadableStreamDefaultController {
    constructor(key) {
      if (key !== streamKey) throw new TypeError("Illegal constructor");
    }
    get desiredSize() {
      return desiredSize(this);
    }
    enqueue(chunk) {
      if (!canClose(this)) throw new TypeError("The stream cannot take chunks");
      const s = this._stream;
      if (s._reader && s._reader._requests?.length) {
        s._reader._requests.shift().resolve({ value: chunk, done: false });
      } else {
        let size;
        try {
          size = this._size(chunk);
          enqueueSized(this._queue, chunk, size);
        } catch (e) {
          errorReadable(s, e);
          throw e;
        }
      }
      pullIfNeeded(this);
    }
    close() {
      if (!canClose(this)) throw new TypeError("The stream cannot close");
      this._closeRequested = true;
      if (!this._queue.items.length) closeReadable(this._stream);
    }
    error(e) {
      errorReadable(this._stream, e);
    }
  }
  const streamKey = {};
  const canClose = (c) => !c._closeRequested && c._stream._state === "readable";
  const desiredSize = (c) => {
    const st = c._stream._state;
    if (st === "errored") return null;
    if (st === "closed") return 0;
    return c._hwm - c._queue.total;
  };
  const pullIfNeeded = (c) => {
    const s = c._stream;
    if (!c._started || c._closeRequested || s._state !== "readable") return;
    const wanted = (s._reader?._requests?.length ?? 0) > 0 || (c._pullIntos?.length ?? 0) > 0 || desiredSize(c) > 0;
    if (!wanted) return;
    if (c._pulling) {
      c._pullAgain = true;
      return;
    }
    c._pulling = true;
    call(c._source, "pull", [c]).then(
      () => {
        c._pulling = false;
        if (c._pullAgain) {
          c._pullAgain = false;
          pullIfNeeded(c);
        }
      },
      (e) => errorReadable(s, e),
    );
  };
  const closeReadable = (s) => {
    if (s._state !== "readable") return;
    s._state = "closed";
    const r = s._reader;
    if (!r) return;
    for (const req of r._requests.splice(0)) req.resolve(req.closeValue ?? { value: undefined, done: true });
    r._closed.resolve();
  };
  const errorReadable = (s, e) => {
    if (s._state !== "readable") return;
    s._state = "errored";
    s._error = e;
    const c = s._controller;
    c._queue = queue();
    if (c._pullIntos) c._pullIntos.length = 0;
    const r = s._reader;
    if (!r) return;
    for (const req of r._requests.splice(0)) req.reject(e);
    r._closed.reject(e);
    quiet(r._closed.promise);
  };
  const cancelReadable = (s, reason) => {
    s._disturbed = true;
    if (s._state === "closed") return resolved();
    if (s._state === "errored") return rejected(s._error);
    const c = s._controller;
    c._queue = queue();
    if (c._pullIntos) c._pullIntos.length = 0;
    closeReadable(s);
    return call(c._source, "cancel", [reason]).then(() => undefined);
  };

  class ReadableByteStreamController {
    constructor(key) {
      if (key !== streamKey) throw new TypeError("Illegal constructor");
    }
    get desiredSize() {
      return desiredSize(this);
    }
    get byobRequest() {
      const p = this._pullIntos[0];
      if (!p) return null;
      if (!this._request) {
        const req = Object.create(ReadableStreamBYOBRequest.prototype);
        hidden(req, "_controller", this);
        hidden(req, "_view", new Uint8Array(p.buffer, p.byteOffset + p.filled, p.byteLength - p.filled));
        this._request = req;
      }
      return this._request;
    }
    enqueue(chunk) {
      if (!ArrayBuffer.isView(chunk)) throw new TypeError("A byte stream takes ArrayBuffer views");
      if (!canClose(this)) throw new TypeError("The stream cannot take chunks");
      const bytes = new Uint8Array(chunk.buffer, chunk.byteOffset, chunk.byteLength).slice();
      this._request = null;
      this._queue.items.push({ value: bytes, size: bytes.byteLength });
      this._queue.total += bytes.byteLength;
      fillPending(this);
      pullIfNeeded(this);
    }
    close() {
      if (!canClose(this)) throw new TypeError("The stream cannot close");
      this._closeRequested = true;
      const p = this._pullIntos[0];
      if (p && p.filled % p.elementSize) {
        const e = new TypeError("Insufficient bytes to fill elements in the given buffer");
        errorReadable(this._stream, e);
        throw e;
      }
      if (!this._queue.total) {
        for (const pi of this._pullIntos.splice(0)) pi.done(true);
        closeReadable(this._stream);
      }
    }
    error(e) {
      errorReadable(this._stream, e);
    }
  }

  class ReadableStreamBYOBRequest {
    constructor() {
      throw new TypeError("Illegal constructor");
    }
    get view() {
      return this._view;
    }
    respond(n) {
      const c = this._controller, p = c._pullIntos[0];
      if (!p) throw new TypeError("No request to respond to");
      if (n > p.byteLength - p.filled) throw new RangeError("The response is larger than the view");
      p.filled += Number(n);
      c._request = null;
      settlePullInto(c, p);
    }
    respondWithNewView(view) {
      const c = this._controller, p = c._pullIntos[0];
      if (!p) throw new TypeError("No request to respond to");
      if (view.byteOffset !== p.byteOffset + p.filled) throw new RangeError("The view does not start where the request's does");
      p.buffer = view.buffer;
      p.filled += view.byteLength;
      c._request = null;
      settlePullInto(c, p);
    }
  }
  // settlePullInto hands a pull-into its bytes once they fill an element,
  // or all of them once the stream closed.
  const settlePullInto = (c, p) => {
    if (c._stream._state === "closed" || c._closeRequested) {
      c._pullIntos.shift();
      p.done(c._stream._state === "closed" && p.filled === 0);
      return;
    }
    if (p.filled < p.elementSize && p.readerType === "byob") {
      pullIfNeeded(c);
      return;
    }
    c._pullIntos.shift();
    p.done(false);
    fillPending(c);
    pullIfNeeded(c);
  };
  // fillPending moves queued bytes into the reads waiting for them.
  const fillPending = (c) => {
    const s = c._stream;
    const r = s._reader;
    if (r instanceof ReadableStreamDefaultReader) {
      while (r._requests.length && c._queue.items.length) {
        const bytes = dequeue(c._queue);
        r._requests.shift().resolve({ value: bytes, done: false });
      }
      // A default read on an auto-allocated buffer takes the first bytes.
      while (c._pullIntos.length && c._pullIntos[0].readerType === "default" && c._queue.items.length) {
        const p = c._pullIntos[0];
        copyInto(c, p);
        c._pullIntos.shift();
        p.done(false);
      }
    } else {
      while (c._pullIntos.length && c._queue.items.length) {
        const p = c._pullIntos[0];
        copyInto(c, p);
        if (p.filled < p.elementSize) break;
        c._pullIntos.shift();
        p.done(false);
      }
    }
    if (c._closeRequested && !c._queue.total) {
      for (const pi of c._pullIntos.splice(0)) pi.done(true);
      closeReadable(s);
    }
  };
  const copyInto = (c, p) => {
    const room = p.byteLength - p.filled;
    let wanted = Math.min(room, c._queue.total);
    wanted -= ((p.filled + wanted) % p.elementSize) * (p.filled + wanted >= p.elementSize ? 1 : 0);
    const dest = new Uint8Array(p.buffer, p.byteOffset + p.filled, wanted);
    let at = 0;
    while (at < wanted) {
      const head = c._queue.items[0];
      const n = Math.min(wanted - at, head.value.byteLength);
      dest.set(head.value.subarray(0, n), at);
      at += n;
      c._queue.total -= n;
      if (n === head.value.byteLength) c._queue.items.shift();
      else head.value = head.value.subarray(n);
    }
    p.filled += wanted;
  };

  class ReadableStreamDefaultReader {
    constructor(stream) {
      if (!(stream instanceof ReadableStream)) throw new TypeError("Not a ReadableStream");
      if (stream._reader) throw new TypeError("ReadableStream is locked");
      setupReader(this, stream);
    }
    get closed() {
      return this._closed.promise;
    }
    read() {
      const s = this._stream;
      if (!s) return rejected(new TypeError("The reader is released"));
      s._disturbed = true;
      if (s._state === "closed") return resolved({ value: undefined, done: true });
      if (s._state === "errored") return rejected(s._error);
      const c = s._controller;
      if (c._queue.items.length) {
        const value = dequeue(c._queue);
        if (c instanceof ReadableByteStreamController) fillPending(c);
        else if (c._closeRequested && !c._queue.items.length) closeReadable(s);
        pullIfNeeded(c);
        return resolved({ value, done: false });
      }
      const d = deferred();
      if (c instanceof ReadableByteStreamController && c._autoAllocate) {
        // A default read of a byte stream pulls into a buffer of its own.
        const pi = { buffer: new ArrayBuffer(c._autoAllocate), byteOffset: 0, byteLength: c._autoAllocate, filled: 0,
          elementSize: 1, readerType: "default" };
        pi.done = (closed) =>
          d.resolve(closed ? { value: undefined, done: true } : { value: new Uint8Array(pi.buffer, 0, pi.filled), done: false });
        c._pullIntos.push(pi);
      } else {
        this._requests.push(d);
      }
      pullIfNeeded(c);
      return d.promise;
    }
    cancel(reason) {
      if (!this._stream) return rejected(new TypeError("The reader is released"));
      return cancelReadable(this._stream, reason);
    }
    releaseLock() {
      releaseReader(this);
    }
  }

  class ReadableStreamBYOBReader {
    constructor(stream) {
      if (!(stream instanceof ReadableStream)) throw new TypeError("Not a ReadableStream");
      if (!(stream._controller instanceof ReadableByteStreamController)) throw new TypeError("Not a byte stream");
      if (stream._reader) throw new TypeError("ReadableStream is locked");
      setupReader(this, stream);
    }
    get closed() {
      return this._closed.promise;
    }
    read(view) {
      const s = this._stream;
      if (!s) return rejected(new TypeError("The reader is released"));
      if (!ArrayBuffer.isView(view) || view.byteLength === 0) return rejected(new TypeError("A BYOB read needs a view"));
      s._disturbed = true;
      if (s._state === "errored") return rejected(s._error);
      const Ctor = view.constructor;
      const elementSize = view.BYTES_PER_ELEMENT ?? 1;
      const d = deferred();
      const p = {
        buffer: view.buffer, byteOffset: view.byteOffset, byteLength: view.byteLength, filled: 0, elementSize,
        readerType: "byob",
        done: (closed) => {
          const out = new Ctor(p.buffer, p.byteOffset, Math.floor(p.filled / elementSize));
          d.resolve({ value: out, done: closed });
        },
      };
      if (s._state === "closed") {
        p.done(true);
        return d.promise;
      }
      const c = s._controller;
      c._pullIntos.push(p);
      if (c._pullIntos.length === 1) fillPending(c);
      pullIfNeeded(c);
      return d.promise;
    }
    cancel(reason) {
      if (!this._stream) return rejected(new TypeError("The reader is released"));
      return cancelReadable(this._stream, reason);
    }
    releaseLock() {
      releaseReader(this);
    }
  }
  const setupReader = (r, s) => {
    hidden(r, "_stream", s);
    hidden(r, "_requests", []);
    hidden(r, "_closed", deferred());
    s._reader = r;
    if (s._state === "closed") r._closed.resolve();
    if (s._state === "errored") {
      r._closed.reject(s._error);
      quiet(r._closed.promise);
    }
  };
  const releaseReader = (r) => {
    const s = r._stream;
    if (!s) return;
    const e = new TypeError("The reader was released");
    for (const req of r._requests.splice(0)) req.reject(e);
    if (s._state === "readable") r._closed.reject(e);
    else hidden(r, "_closed", (() => {
      const d = deferred();
      d.reject(e);
      return d;
    })());
    quiet(r._closed.promise);
    s._reader = null;
    r._stream = null;
  };

  class ReadableStream {
    constructor(source = {}, strategy = {}) {
      source ??= {};
      hidden(this, "_state", "readable");
      hidden(this, "_reader", null);
      hidden(this, "_error", undefined);
      hidden(this, "_disturbed", false);
      const bytes = source.type === "bytes";
      if (source.type !== undefined && !bytes) throw new TypeError(`Invalid type "${source.type}"`);
      const c = Object.create((bytes ? ReadableByteStreamController : ReadableStreamDefaultController).prototype);
      hidden(c, "_stream", this);
      hidden(c, "_source", source);
      hidden(c, "_queue", queue());
      hidden(c, "_started", false);
      hidden(c, "_closeRequested", false);
      hidden(c, "_pulling", false);
      hidden(c, "_pullAgain", false);
      if (bytes) {
        const st = strategyOf(strategy, 0);
        hidden(c, "_hwm", st.hwm);
        hidden(c, "_pullIntos", []);
        hidden(c, "_request", null);
        hidden(c, "_autoAllocate", source.autoAllocateChunkSize ?? 0);
      } else {
        const st = strategyOf(strategy, 1);
        hidden(c, "_hwm", st.hwm);
        hidden(c, "_size", st.size);
      }
      hidden(this, "_controller", c);
      let start;
      try {
        start = resolved(source.start?.(c));
      } catch (e) {
        throw e;
      }
      start.then(
        () => {
          c._started = true;
          pullIfNeeded(c);
        },
        (e) => errorReadable(this, e),
      );
    }
    static from(iterable) {
      const it = iterable[Symbol.asyncIterator] ? iterable[Symbol.asyncIterator]() : iterable[Symbol.iterator]();
      return new ReadableStream(
        {
          async pull(c) {
            const { value, done } = await it.next();
            if (done) c.close();
            else c.enqueue(await value);
          },
          async cancel(reason) {
            await it.return?.(reason);
          },
        },
        { highWaterMark: 0 },
      );
    }
    get locked() {
      return !!this._reader;
    }
    getReader(options = {}) {
      if (options?.mode === "byob") return new ReadableStreamBYOBReader(this);
      if (options?.mode !== undefined) throw new TypeError(`Invalid mode "${options.mode}"`);
      return new ReadableStreamDefaultReader(this);
    }
    cancel(reason) {
      if (this._reader) return rejected(new TypeError("ReadableStream is locked"));
      return cancelReadable(this, reason);
    }
    pipeThrough(transform, options) {
      if (this._reader) throw new TypeError("ReadableStream is locked");
      if (transform.writable._writer) throw new TypeError("WritableStream is locked");
      quiet(this.pipeTo(transform.writable, options));
      return transform.readable;
    }
    pipeTo(dest, options = {}) {
      if (!(dest instanceof WritableStream)) return rejected(new TypeError("Not a WritableStream"));
      if (this._reader) return rejected(new TypeError("ReadableStream is locked"));
      if (dest._writer) return rejected(new TypeError("WritableStream is locked"));
      const { preventClose = false, preventAbort = false, preventCancel = false, signal } = options ?? {};
      const reader = this.getReader(), writer = dest.getWriter();
      const done = deferred();
      let finished = false, pending = resolved();
      const finish = (err, isError) => {
        if (finished) return;
        finished = true;
        pending.then(() => {
          reader.releaseLock();
          writer.releaseLock();
          isError ? done.reject(err) : done.resolve();
        }, () => {
          reader.releaseLock();
          writer.releaseLock();
          isError ? done.reject(err) : done.resolve();
        });
      };
      const abort = () => {
        const reason = signal.reason;
        const actions = [];
        if (!preventAbort && dest._state === "writable") actions.push(writer.abort(reason));
        if (!preventCancel && this._state === "readable") actions.push(reader.cancel(reason));
        Promise.all(actions).then(() => finish(reason, true), (e) => finish(e, true));
      };
      if (signal) {
        if (signal.aborted) {
          abort();
          return done.promise;
        }
        signal.addEventListener("abort", abort, { once: true });
      }
      quiet(reader.closed.then(
        () => {
          if (preventClose) finish();
          else pending.then(() => writer.close()).then(() => finish(), (e) => finish(e, true));
        },
        (e) => {
          if (!preventAbort) quiet(writer.abort(e));
          finish(e, true);
        },
      ));
      quiet(writer.closed.catch((e) => {
        if (!preventCancel) quiet(reader.cancel(e));
        finish(e, true);
      }));
      const step = () => {
        if (finished) return;
        writer.ready.then(() => {
          if (finished) return;
          reader.read().then(({ value, done: end }) => {
            if (end || finished) return;
            pending = quiet(writer.write(value));
            step();
          }, () => {});
        }, () => {});
      };
      step();
      return done.promise;
    }
    tee() {
      const src = this;
      const reader = this.getReader();
      let reading = false, cancelled = [false, false], reasons = [];
      const branches = [];
      const cancelDone = deferred();
      const pull = () => {
        if (reading) return resolved();
        reading = true;
        reader.read().then(({ value, done }) => {
          reading = false;
          for (let i = 0; i < 2; i++) {
            if (cancelled[i]) continue;
            if (done) branches[i]._controller.close();
            else branches[i]._controller.enqueue(value);
          }
        }, () => (reading = false));
        return resolved();
      };
      const make = (i) =>
        new ReadableStream({
          pull,
          cancel(reason) {
            cancelled[i] = true;
            reasons[i] = reason;
            if (cancelled[0] && cancelled[1]) cancelReadable(src, reasons).then(cancelDone.resolve, cancelDone.reject);
            return cancelDone.promise;
          },
        });
      branches.push(make(0), make(1));
      quiet(reader.closed.catch((e) => {
        for (const b of branches) errorReadable(b, e);
      }));
      return branches;
    }
    values({ preventCancel = false } = {}) {
      const reader = this.getReader();
      const it = Object.create(asyncIteratorProto);
      hidden(it, "_reader", reader);
      hidden(it, "_preventCancel", preventCancel);
      return it;
    }
    [Symbol.asyncIterator](options) {
      return this.values(options);
    }
  }
  const asyncIteratorProto = Object.create(Object.prototype, {
    [Symbol.asyncIterator]: {
      value() {
        return this;
      },
    },
    next: {
      value() {
        const r = this._reader;
        if (!r._stream) return resolved({ value: undefined, done: true });
        return r.read().then((res) => {
          if (res.done) r.releaseLock();
          return res;
        }, (e) => {
          r.releaseLock();
          throw e;
        });
      },
    },
    return: {
      value(value) {
        const r = this._reader;
        if (!r._stream) return resolved({ value, done: true });
        const cancelled = this._preventCancel ? resolved() : r.cancel(value);
        r.releaseLock();
        return cancelled.then(() => ({ value, done: true }));
      },
    },
  });

  class WritableStreamDefaultController {
    constructor() {
      throw new TypeError("Illegal constructor");
    }
    get signal() {
      return this._abort.signal;
    }
    error(e) {
      if (this._stream._state === "writable") startErroring(this._stream, e);
    }
  }

  class WritableStream {
    constructor(sink = {}, strategy = {}) {
      sink ??= {};
      if (sink.type !== undefined) throw new RangeError("Invalid type");
      hidden(this, "_state", "writable");
      hidden(this, "_error", undefined);
      hidden(this, "_writer", null);
      hidden(this, "_writes", []); // the writes' promises, oldest first
      hidden(this, "_inFlight", null);
      hidden(this, "_close", null);
      hidden(this, "_inFlightClose", null);
      hidden(this, "_pendingAbort", null);
      hidden(this, "_backpressure", false);
      const st = strategyOf(strategy, 1);
      const c = Object.create(WritableStreamDefaultController.prototype);
      hidden(c, "_stream", this);
      hidden(c, "_sink", sink);
      hidden(c, "_queue", queue());
      hidden(c, "_hwm", st.hwm);
      hidden(c, "_size", st.size);
      hidden(c, "_started", false);
      hidden(c, "_abort", new AbortController());
      hidden(this, "_controller", c);
      updateBackpressure(this);
      call(sink, "start", [c]).then(
        () => {
          c._started = true;
          advanceWrites(c);
        },
        (e) => {
          c._started = true;
          dealWithRejection(this, e);
        },
      );
    }
    get locked() {
      return !!this._writer;
    }
    getWriter() {
      return new WritableStreamDefaultWriter(this);
    }
    abort(reason) {
      if (this._writer) return rejected(new TypeError("WritableStream is locked"));
      return abortWritable(this, reason);
    }
    close() {
      if (this._writer) return rejected(new TypeError("WritableStream is locked"));
      return closeWritable(this);
    }
  }
  const writableDesired = (c) => c._hwm - c._queue.total;
  const updateBackpressure = (s) => {
    const back = writableDesired(s._controller) <= 0;
    const w = s._writer;
    if (w && back !== s._backpressure) {
      if (back) hidden(w, "_ready", deferred());
      else w._ready.resolve();
    }
    s._backpressure = back;
  };
  const startErroring = (s, reason) => {
    s._state = "erroring";
    s._error = reason;
    const w = s._writer;
    if (w) rejectReady(w, reason);
    if (!s._inFlight && !s._inFlightClose && s._controller._started) finishErroring(s);
  };
  const rejectReady = (w, reason) => {
    if (w._ready.settled) hidden(w, "_ready", deferred());
    w._ready.reject(reason);
    w._ready.settled = true;
    quiet(w._ready.promise);
  };
  const finishErroring = (s) => {
    s._state = "errored";
    s._controller._queue = queue();
    const e = s._error;
    for (const d of s._writes.splice(0)) d.reject(e);
    const abort = s._pendingAbort;
    s._pendingAbort = null;
    const rejectClosed = () => {
      if (s._close) s._close.reject(e), (s._close = null);
      if (s._writer) {
        s._writer._closed.reject(e);
        quiet(s._writer._closed.promise);
      }
    };
    if (!abort) return rejectClosed();
    if (abort.wasAlreadyErroring) {
      abort.d.reject(e);
      return rejectClosed();
    }
    call(s._controller._sink, "abort", [abort.reason]).then(
      () => (abort.d.resolve(), rejectClosed()),
      (err) => (abort.d.reject(err), rejectClosed()),
    );
  };
  const dealWithRejection = (s, e) => {
    if (s._state === "writable") startErroring(s, e);
    else if (s._state === "erroring" && !s._inFlight && !s._inFlightClose) finishErroring(s);
  };
  const advanceWrites = (c) => {
    const s = c._stream;
    if (!c._started || s._inFlight || s._inFlightClose) return;
    if (s._state === "erroring") return finishErroring(s);
    if (!c._queue.items.length) return;
    const head = c._queue.items[0];
    if (head.value === closeSentinel) {
      dequeue(c._queue);
      s._inFlightClose = s._close;
      s._close = null;
      call(c._sink, "close", []).then(
        () => {
          s._inFlightClose.resolve();
          s._inFlightClose = null;
          if (s._state === "erroring") s._error = undefined, s._pendingAbort?.d.resolve(), (s._pendingAbort = null);
          s._state = "closed";
          if (s._writer) s._writer._closed.resolve();
        },
        (e) => {
          s._inFlightClose.reject(e);
          s._inFlightClose = null;
          s._pendingAbort?.d.reject(e);
          s._pendingAbort = null;
          dealWithRejection(s, e);
        },
      );
      return;
    }
    s._inFlight = s._writes.shift();
    call(c._sink, "write", [head.value, c]).then(
      () => {
        s._inFlight.resolve();
        s._inFlight = null;
        dequeue(c._queue);
        if (s._state === "erroring") return finishErroring(s);
        if (!s._close && s._state === "writable") updateBackpressure(s);
        advanceWrites(c);
      },
      (e) => {
        s._inFlight.reject(e);
        s._inFlight = null;
        if (s._state === "writable") c._queue = queue();
        dealWithRejection(s, e);
      },
    );
  };
  const closeSentinel = {};
  const abortWritable = (s, reason) => {
    if (s._state === "closed" || s._state === "errored") return resolved();
    s._controller._abort.abort(reason);
    if (s._state === "closed" || s._state === "errored") return resolved();
    if (s._pendingAbort) return s._pendingAbort.d.promise;
    const wasAlreadyErroring = s._state === "erroring";
    const d = deferred();
    s._pendingAbort = { d, reason: wasAlreadyErroring ? undefined : reason, wasAlreadyErroring };
    if (!wasAlreadyErroring) startErroring(s, reason);
    return d.promise;
  };
  const closeWritable = (s) => {
    if (s._state === "closed" || s._state === "errored") return rejected(new TypeError("The stream is closed"));
    if (s._close || s._inFlightClose) return rejected(new TypeError("The stream is closing"));
    const d = deferred();
    s._close = d;
    if (s._writer && s._backpressure && s._state === "writable") s._writer._ready.resolve();
    s._controller._queue.items.push({ value: closeSentinel, size: 0 });
    advanceWrites(s._controller);
    return d.promise;
  };

  class WritableStreamDefaultWriter {
    constructor(stream) {
      if (!(stream instanceof WritableStream)) throw new TypeError("Not a WritableStream");
      if (stream._writer) throw new TypeError("WritableStream is locked");
      hidden(this, "_stream", stream);
      stream._writer = this;
      hidden(this, "_ready", deferred());
      hidden(this, "_closed", deferred());
      const st = stream._state;
      if (st === "writable") {
        if (!stream._backpressure || stream._close || stream._inFlightClose) this._ready.resolve();
      } else if (st === "erroring") {
        rejectReady(this, stream._error);
      } else if (st === "closed") {
        this._ready.resolve();
        this._closed.resolve();
      } else {
        rejectReady(this, stream._error);
        this._closed.reject(stream._error);
        quiet(this._closed.promise);
      }
    }
    get closed() {
      return this._closed.promise;
    }
    get ready() {
      return this._ready.promise;
    }
    get desiredSize() {
      const s = this._stream;
      if (!s) throw new TypeError("The writer is released");
      if (s._state === "errored" || s._state === "erroring") return null;
      if (s._state === "closed") return 0;
      return writableDesired(s._controller);
    }
    write(chunk) {
      const s = this._stream;
      if (!s) return rejected(new TypeError("The writer is released"));
      const c = s._controller;
      let size = 1;
      try {
        size = c._size(chunk);
      } catch (e) {
        c.error(e);
        return rejected(e);
      }
      if (s._state === "errored" || s._state === "erroring") return rejected(s._error);
      if (s._close || s._inFlightClose || s._state === "closed") return rejected(new TypeError("The stream is closing"));
      const d = deferred();
      s._writes.push(d);
      try {
        enqueueSized(c._queue, chunk, size);
      } catch (e) {
        s._writes.pop();
        startErroring(s, e);
        return rejected(e);
      }
      if (!s._close && s._state === "writable") updateBackpressure(s);
      advanceWrites(c);
      return d.promise;
    }
    close() {
      if (!this._stream) return rejected(new TypeError("The writer is released"));
      return closeWritable(this._stream);
    }
    abort(reason) {
      if (!this._stream) return rejected(new TypeError("The writer is released"));
      return abortWritable(this._stream, reason);
    }
    releaseLock() {
      const s = this._stream;
      if (!s) return;
      const e = new TypeError("The writer was released");
      rejectReady(this, e);
      if (s._state === "writable" || s._state === "erroring") this._closed.reject(e);
      else hidden(this, "_closed", (() => {
        const d = deferred();
        d.reject(e);
        return d;
      })());
      quiet(this._closed.promise);
      s._writer = null;
      this._stream = null;
    }
  }

  class TransformStreamDefaultController {
    constructor() {
      throw new TypeError("Illegal constructor");
    }
    get desiredSize() {
      return this._readable._controller.desiredSize;
    }
    enqueue(chunk) {
      const rc = this._readable._controller;
      if (!canClose(rc)) throw new TypeError("The readable side cannot take chunks");
      rc.enqueue(chunk);
    }
    error(e) {
      transformError(this, e);
    }
    terminate() {
      const rc = this._readable._controller;
      if (canClose(rc)) rc.close();
      const e = new TypeError("The transform stream was terminated");
      const s = this._writable;
      if (s._state === "writable") startErroring(s, e);
    }
  }
  const transformError = (tc, e) => {
    errorReadable(tc._readable, e);
    const s = tc._writable;
    if (s._state === "writable") startErroring(s, e);
  };

  class TransformStream {
    constructor(transformer = {}, writableStrategy = {}, readableStrategy = {}) {
      transformer ??= {};
      const tc = Object.create(TransformStreamDefaultController.prototype);
      const startDone = deferred();
      this.writable = new WritableStream(
        {
          start: () => startDone.promise,
          write: (chunk) => {
            if (typeof transformer.transform !== "function") {
              tc.enqueue(chunk);
              return;
            }
            return call(transformer, "transform", [chunk, tc]).catch((e) => {
              transformError(tc, e);
              throw e;
            });
          },
          close: () =>
            call(transformer, "flush", [tc]).then(
              () => {
                const rc = this.readable._controller;
                if (canClose(rc)) rc.close();
              },
              (e) => {
                transformError(tc, e);
                throw e;
              },
            ),
          abort: (reason) => {
            errorReadable(this.readable, reason);
            return call(transformer, "cancel", [reason]);
          },
        },
        writableStrategy,
      );
      const rs = strategyOf(readableStrategy, 0);
      this.readable = new ReadableStream(
        {
          cancel: (reason) => {
            const s = this.writable;
            if (s._state === "writable") startErroring(s, reason);
            return call(transformer, "cancel", [reason]);
          },
        },
        { highWaterMark: rs.hwm, size: readableStrategy?.size },
      );
      hidden(tc, "_readable", this.readable);
      hidden(tc, "_writable", this.writable);
      call(transformer, "start", [tc]).then(startDone.resolve, (e) => {
        transformError(tc, e);
        startDone.reject(e);
      });
    }
  }

  class TextEncoderStream extends TransformStream {
    constructor() {
      const enc = new TextEncoder();
      let high = "";
      super({
        transform: (chunk, c) => {
          let s = high + String(chunk);
          high = "";
          const last = s.charCodeAt(s.length - 1);
          if (last >= 0xd800 && last <= 0xdbff) (high = s.slice(-1)), (s = s.slice(0, -1));
          if (s) c.enqueue(enc.encode(s));
        },
        flush: (c) => {
          if (high) c.enqueue(enc.encode("�"));
        },
      });
    }
    get encoding() {
      return "utf-8";
    }
  }

  class TextDecoderStream extends TransformStream {
    constructor(label = "utf-8", options = {}) {
      const d = new TextDecoder(label, options);
      super({
        transform: (chunk, c) => {
          const s = d.decode(chunk, { stream: true });
          if (s) c.enqueue(s);
        },
        flush: (c) => {
          const s = d.decode();
          if (s) c.enqueue(s);
        },
      });
      hidden(this, "_decoder", d);
    }
    get encoding() {
      return this._decoder.encoding;
    }
    get fatal() {
      return this._decoder.fatal;
    }
    get ignoreBOM() {
      return this._decoder.ignoreBOM;
    }
  }

  // A body: bytes, or a stream read once.
  const bodyOf = (init) => {
    if (init === undefined || init === null) return { bytes: null, type: null };
    if (init instanceof ReadableStream) return { stream: init, type: null };
    if (init instanceof Blob) return { bytes: init._bytes, type: init.type || null };
    if (init instanceof URLSearchParams)
      return { bytes: new TextEncoder().encode(init.toString()), type: "application/x-www-form-urlencoded;charset=UTF-8" };
    if (init instanceof FormData) {
      const boundary = "----nsgo" + Math.random().toString(16).slice(2);
      const parts = [];
      for (const [k, v] of init) {
        let head = `--${boundary}\r\nContent-Disposition: form-data; name="${k}"`;
        if (v instanceof File) head += `; filename="${v.name}"\r\nContent-Type: ${v.type || "application/octet-stream"}`;
        parts.push(head + "\r\n\r\n", v instanceof Blob ? v : String(v), "\r\n");
      }
      parts.push(`--${boundary}--\r\n`);
      return { bytes: new Blob(parts)._bytes, type: `multipart/form-data; boundary=${boundary}` };
    }
    if (init instanceof ArrayBuffer || ArrayBuffer.isView(init)) return { bytes: bytesOf(init).slice(), type: null };
    return { bytes: new TextEncoder().encode(String(init)), type: "text/plain;charset=UTF-8" };
  };

  class Body {
    _initBody(init, headers) {
      const b = bodyOf(init);
      hidden(this, "_bytes", b.bytes ?? null);
      hidden(this, "_stream", b.stream ?? null);
      hidden(this, "_used", false);
      if (b.type && !headers.has("content-type")) headers.set("content-type", b.type);
    }
    get body() {
      if (this._stream) return this._stream;
      if (this._bytes === null) return null;
      const bytes = this._bytes;
      this._stream = new ReadableStream({
        start(c) {
          if (bytes.length) c.enqueue(bytes);
          c.close();
        },
      });
      return this._stream;
    }
    get bodyUsed() {
      return this._used;
    }
    async _consume() {
      if (this._used) throw new TypeError("Body has already been consumed.");
      this._used = true;
      if (this._stream && this._bytes === null) {
        const chunks = [], reader = this._stream.getReader();
        for (;;) {
          const { value: c, done } = await reader.read();
          if (done) break;
          chunks.push(typeof c === "string" ? new TextEncoder().encode(c) : bytesOf(c));
        }
        return new Blob(chunks)._bytes;
      }
      return this._bytes ?? new Uint8Array(0);
    }
    async arrayBuffer() {
      return (await this._consume()).slice().buffer;
    }
    async bytes() {
      return (await this._consume()).slice();
    }
    async blob() {
      const b = new Blob([await this._consume()], { type: this.headers.get("content-type") ?? "" });
      return b;
    }
    async text() {
      return new TextDecoder().decode(await this._consume());
    }
    async json() {
      return JSON.parse(await this.text());
    }
    async formData() {
      const type = this.headers.get("content-type") ?? "";
      const text = await this.text();
      const fd = new FormData();
      if (type.startsWith("application/x-www-form-urlencoded")) {
        for (const [k, v] of new URLSearchParams(text)) fd.append(k, v);
        return fd;
      }
      const m = /boundary=("?)([^";]+)\1/.exec(type);
      if (!m) throw new TypeError("Could not parse content as FormData.");
      for (const part of text.split("--" + m[2]).slice(1, -1)) {
        const [head, ...rest] = part.replace(/^\r\n/, "").split("\r\n\r\n");
        const value = rest.join("\r\n\r\n").replace(/\r\n$/, "");
        const name = /name="([^"]*)"/.exec(head)?.[1];
        const filename = /filename="([^"]*)"/.exec(head)?.[1];
        if (name === undefined) continue;
        if (filename !== undefined) {
          const ct = /Content-Type: ([^\r\n]*)/i.exec(head)?.[1] ?? "";
          fd.append(name, new File([value], filename, { type: ct }));
        } else fd.append(name, value);
      }
      return fd;
    }
  }

  class Request extends Body {
    constructor(input, init = {}) {
      super();
      const from = input instanceof Request ? input : null;
      if (from && from._used) throw new TypeError("Request body is already used");
      // A VM has no page whose URL a relative one resolves against.
      this.url = from ? from.url : new URL(String(input)).href;
      let method = String(init.method ?? from?.method ?? "GET");
      if (!/^[!#$%&'*+\-.^_`|~0-9A-Za-z]+$/.test(method)) throw new TypeError(`Invalid method: "${method}"`);
      if (/^(DELETE|GET|HEAD|OPTIONS|PATCH|POST|PUT)$/i.test(method)) method = method.toUpperCase();
      if (/^(CONNECT|TRACE|TRACK)$/i.test(method)) throw new TypeError(`Forbidden method: "${method}"`);
      this.method = method;
      this.headers = new Headers(init.headers ?? from?.headers);
      this.signal = init.signal ?? from?.signal ?? new AbortController().signal;
      this.redirect = init.redirect ?? from?.redirect ?? "follow";
      this.credentials = init.credentials ?? "same-origin";
      this.cache = init.cache ?? "default";
      this.mode = init.mode ?? "cors";
      this.referrer = init.referrer ?? "about:client";
      this.keepalive = !!init.keepalive;
      if ((this.method === "GET" || this.method === "HEAD") && init.body != null)
        throw new TypeError("Request with GET/HEAD method cannot have body.");
      this._initBody(init.body !== undefined ? init.body : from ? from._bytes : undefined, this.headers);
    }
    clone() {
      if (this._used) throw new TypeError("Request body is already used");
      return new Request(this);
    }
  }

  class Response extends Body {
    constructor(body = null, init = {}) {
      super();
      this.status = init.status ?? 200;
      if (this.status < 200 || this.status > 599) throw new RangeError(`init["status"] must be in the range of 200 to 599, inclusive.`);
      if (body !== null && body !== undefined && [101, 103, 204, 205, 304].includes(this.status))
        throw new TypeError(`Response with status ${this.status} cannot have a body.`);
      this.statusText = init.statusText ?? "";
      this.headers = new Headers(init.headers);
      this.type = "default";
      this.url = "";
      this.redirected = false;
      this._initBody(body, this.headers);
    }
    get ok() {
      return this.status >= 200 && this.status < 300;
    }
    clone() {
      if (this._used) throw new TypeError("Response body is already used");
      let body = this._bytes;
      if (this._stream && body === null) {
        const [a, b] = this._stream.tee();
        this._stream = a;
        body = b;
      }
      const r = new Response(body, this);
      r.url = this.url;
      r.type = this.type;
      r.redirected = this.redirected;
      return r;
    }
    static json(data, init = {}) {
      const headers = new Headers(init.headers);
      if (!headers.has("content-type")) headers.set("content-type", "application/json");
      return new Response(JSON.stringify(data), { ...init, headers });
    }
    static error() {
      const r = new Response(null, { status: 200 });
      r.status = 0;
      r.type = "error";
      return r;
    }
    static redirect(url, status = 302) {
      if (![301, 302, 303, 307, 308].includes(status)) throw new RangeError(`Invalid redirect status ${status}`);
      return new Response(null, { status, headers: { location: new URL(String(url)).href } });
    }
  }

  // fetch hands the request to Go, which answers with the status, the
  // headers and the body's bytes.
  const fetch = (input, init) =>
    new Promise((resolve, reject) => {
      let req;
      try {
        req = new Request(input, init);
      } catch (e) {
        return reject(e);
      }
      if (req.signal.aborted) return reject(req.signal.reason);
      if (req.url.startsWith("blob:")) {
        const blob = native.blobOf(req.url);
        if (!blob || req.method !== "GET") return reject(new TypeError(`Failed to fetch ${req.url}`));
        const r = new Response(new Uint8Array(blob.bytes), {
          headers: { "content-type": blob.type, "content-length": String(blob.bytes.byteLength) },
        });
        r.url = req.url;
        r.type = "basic";
        return resolve(r);
      }
      const send = (bytes) => {
        const cancel = native.fetch(
          req.url,
          req.method,
          [...req.headers],
          bytes,
          req.redirect,
          (res) => {
            const r = new Response(res.status === 204 || res.status === 304 || req.method === "HEAD" ? null : new Uint8Array(res.body), {
              status: res.status < 200 ? 200 : res.status,
              statusText: res.statusText,
              headers: res.headers,
            });
            r.status = res.status;
            r.url = res.url;
            r.redirected = res.redirected;
            r.type = "basic";
            resolve(r);
          },
          // Go makes the TypeError, with a refusal's code.
          (error) => reject(error),
        );
        req.signal.addEventListener("abort", () => {
          cancel();
          reject(req.signal.reason);
        }, { once: true });
      };
      if (req._stream && req._bytes === null) req.arrayBuffer().then((b) => send(b), reject);
      else send(req._bytes ? req._bytes.slice().buffer : null);
    });

  const structuredClone = (v, seen = new Map()) => {
    if (typeof v === "function" || typeof v === "symbol") throw new DOMException("could not be cloned", "DataCloneError");
    if (v === null || typeof v !== "object") return v;
    if (seen.has(v)) return seen.get(v);
    let out;
    if (v instanceof Date) out = new Date(v.getTime());
    else if (v instanceof RegExp) out = new RegExp(v.source, v.flags);
    else if (v instanceof ArrayBuffer) out = v.slice(0);
    else if (ArrayBuffer.isView(v)) out = new v.constructor(v);
    else if (v instanceof Blob) out = v;
    else if (v instanceof Map) {
      out = new Map();
      seen.set(v, out);
      for (const [k, x] of v) out.set(structuredClone(k, seen), structuredClone(x, seen));
      return out;
    } else if (v instanceof Set) {
      out = new Set();
      seen.set(v, out);
      for (const x of v) out.add(structuredClone(x, seen));
      return out;
    } else if (Array.isArray(v)) {
      out = [];
      seen.set(v, out);
      for (const x of v) out.push(structuredClone(x, seen));
      return out;
    } else {
      out = {};
      seen.set(v, out);
      for (const k of Object.keys(v)) out[k] = structuredClone(v[k], seen);
      return out;
    }
    seen.set(v, out);
    return out;
  };

  const performance = { now: () => native.now(), timeOrigin: Date.now() - native.now() };

  Object.assign(g, {
    DOMException, Event, CustomEvent, MessageEvent, EventTarget, AbortSignal, AbortController, TextEncoder,
    TextDecoder, atob, btoa, URL, URLSearchParams, Blob, File, FormData, Headers, Request, Response,
    ReadableStream, ReadableStreamDefaultController, ReadableByteStreamController, ReadableStreamBYOBRequest,
    ReadableStreamDefaultReader, ReadableStreamBYOBReader, WritableStream, WritableStreamDefaultController,
    WritableStreamDefaultWriter, TransformStream, TransformStreamDefaultController, ByteLengthQueuingStrategy,
    CountQueuingStrategy, TextEncoderStream, TextDecoderStream, fetch, structuredClone, performance,
  });
  g.self = g;
  return { newFile: (bytes, name, type) => new File([new Uint8Array(bytes)], name, { type }) };
})
