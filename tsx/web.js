// The web APIs TSX apps get as globals (NSH's web/): events, aborts,
// encoding, URLs, blobs, form data, headers, requests, responses and
// streams. They are thin classes; what needs the system (fetch, UTF-8,
// base64, URL parsing) is Go's, through the natives object this script
// is called with.
(function (g, native) {
  "use strict";
  const hidden = (o, k, v) => Object.defineProperty(o, k, { value: v, writable: true, configurable: true });

  class DOMException extends Error {
    constructor(message = "", name = "Error") {
      super(message);
      hidden(this, "name", name);
    }
  }

  class Event {
    constructor(type, init = {}) {
      this.type = String(type);
      this.bubbles = !!init.bubbles;
      this.cancelable = !!init.cancelable;
      this.defaultPrevented = false;
      this.target = null;
      this.currentTarget = null;
      this.timeStamp = native.now();
      hidden(this, "_stop", false);
    }
    preventDefault() {
      if (this.cancelable) this.defaultPrevented = true;
    }
    stopPropagation() {}
    stopImmediatePropagation() {
      this._stop = true;
    }
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
        const handler = this["on" + event.type];
        const list = [...(this._listeners.get(event.type) ?? [])];
        if (typeof handler === "function") handler.call(this, event);
        for (const l of list) {
          if (event._stop) break;
          if (l.once) this.removeEventListener(event.type, l.listener);
          if (typeof l.listener === "function") l.listener.call(this, event);
          else l.listener.handleEvent(event);
        }
      } finally {
        event._dispatching = false;
        event.currentTarget = null;
      }
      return !event.defaultPrevented;
    }
  }

  class AbortSignal extends EventTarget {
    constructor() {
      super();
      this.aborted = false;
      this.reason = undefined;
      this.onabort = null;
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
      const bytes = this.encode(s);
      const n = Math.min(bytes.length, dest.length);
      dest.set(bytes.subarray(0, n));
      return { read: String(s).length, written: n };
    }
  }

  const bytesOf = (v) => {
    if (v instanceof ArrayBuffer) return new Uint8Array(v);
    if (ArrayBuffer.isView(v)) return new Uint8Array(v.buffer, v.byteOffset, v.byteLength);
    throw new TypeError("not a buffer source");
  };

  class TextDecoder {
    constructor(label = "utf-8", options = {}) {
      label = String(label).toLowerCase();
      if (label !== "utf-8" && label !== "utf8") throw new RangeError(`The encoding "${label}" is not supported`);
      this.fatal = !!options.fatal;
      this.ignoreBOM = !!options.ignoreBOM;
      hidden(this, "_pending", new Uint8Array(0));
    }
    get encoding() {
      return "utf-8";
    }
    decode(input, options = {}) {
      let bytes = input === undefined ? new Uint8Array(0) : bytesOf(input);
      if (this._pending.length) {
        const joined = new Uint8Array(this._pending.length + bytes.length);
        joined.set(this._pending);
        joined.set(bytes, this._pending.length);
        bytes = joined;
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
      this.type = options.type ? String(options.type).toLowerCase() : "";
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

  class Headers {
    constructor(init) {
      hidden(this, "_map", new Map());
      if (init instanceof Headers) init.forEach((v, k) => this.append(k, v));
      else if (init && typeof init[Symbol.iterator] === "function") for (const [k, v] of init) this.append(k, v);
      else if (init && typeof init === "object") for (const k of Object.keys(init)) this.append(k, init[k]);
    }
    append(k, v) {
      k = String(k).toLowerCase();
      v = String(v).trim();
      const old = this._map.get(k);
      this._map.set(k, old === undefined ? v : k === "set-cookie" ? old + "\n" + v : old + ", " + v);
    }
    set(k, v) {
      this._map.set(String(k).toLowerCase(), String(v).trim());
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
      return [...this._map.keys()].sort().map((k) => [k, this.get(k)])[Symbol.iterator]();
    }
    [Symbol.iterator]() {
      return this.entries();
    }
  }

  // A minimal WHATWG stream: a queue of chunks read in order.
  class ReadableStreamDefaultController {
    constructor(stream) {
      hidden(this, "_s", stream);
    }
    get desiredSize() {
      return this._s._state === "readable" ? 1 - this._s._queue.length : 0;
    }
    enqueue(chunk) {
      if (this._s._closeRequested || this._s._state !== "readable") throw new TypeError("The stream is closed");
      this._s._push(chunk);
    }
    close() {
      this._s._closeRequested = true;
      this._s._settle();
    }
    error(e) {
      this._s._fail(e);
    }
  }

  class ReadableStream {
    constructor(source = {}, strategy) {
      hidden(this, "_queue", []);
      hidden(this, "_waiting", []);
      hidden(this, "_state", "readable");
      hidden(this, "_closeRequested", false);
      hidden(this, "_error", undefined);
      hidden(this, "_source", source);
      hidden(this, "_locked", false);
      hidden(this, "_pulling", false);
      const c = new ReadableStreamDefaultController(this);
      hidden(this, "_controller", c);
      Promise.resolve(source.start?.(c)).then(
        () => this._pull(),
        (e) => this._fail(e),
      );
    }
    static from(iterable) {
      const it = (iterable[Symbol.asyncIterator] ?? iterable[Symbol.iterator]).call(iterable);
      return new ReadableStream({
        async pull(c) {
          const { value, done } = await it.next();
          if (done) c.close();
          else c.enqueue(value);
        },
      });
    }
    get locked() {
      return this._locked;
    }
    _push(chunk) {
      const w = this._waiting.shift();
      if (w) w.resolve({ value: chunk, done: false });
      else this._queue.push(chunk);
    }
    _settle() {
      if (this._closeRequested && !this._queue.length && this._state === "readable") {
        this._state = "closed";
        for (const w of this._waiting.splice(0)) w.resolve({ value: undefined, done: true });
      }
    }
    _fail(e) {
      if (this._state !== "readable") return;
      this._state = "errored";
      this._error = e;
      this._queue.length = 0;
      for (const w of this._waiting.splice(0)) w.reject(e);
    }
    async _pull() {
      if (this._pulling || this._state !== "readable" || this._closeRequested || !this._source.pull) return;
      if (!this._waiting.length && this._queue.length) return;
      this._pulling = true;
      try {
        await this._source.pull(this._controller);
      } catch (e) {
        this._fail(e);
      }
      this._pulling = false;
      if (this._waiting.length) this._pull();
    }
    _read() {
      if (this._queue.length) {
        const value = this._queue.shift();
        this._settle();
        return Promise.resolve({ value, done: false });
      }
      if (this._state === "closed") return Promise.resolve({ value: undefined, done: true });
      if (this._state === "errored") return Promise.reject(this._error);
      const p = new Promise((resolve, reject) => this._waiting.push({ resolve, reject }));
      this._pull();
      return p;
    }
    getReader() {
      if (this._locked) throw new TypeError("ReadableStream is locked");
      this._locked = true;
      const s = this;
      return {
        read: () => s._read(),
        releaseLock() {
          s._locked = false;
        },
        cancel: (reason) => s.cancel(reason),
        get closed() {
          return new Promise((resolve, reject) => {
            const check = () => (s._state === "closed" ? resolve() : s._state === "errored" ? reject(s._error) : setTimeout(check, 0));
            check();
          });
        },
      };
    }
    async cancel(reason) {
      this._queue.length = 0;
      this._closeRequested = true;
      this._settle();
      await this._source.cancel?.(reason);
    }
    pipeThrough({ writable, readable }) {
      this.pipeTo(writable);
      return readable;
    }
    async pipeTo(dest) {
      const reader = this.getReader(), writer = dest.getWriter();
      for (;;) {
        const { value, done } = await reader.read();
        if (done) break;
        await writer.write(value);
      }
      await writer.close();
    }
    tee() {
      const reader = this.getReader();
      const branches = [];
      const pull = async () => {
        const { value, done } = await reader.read();
        for (const c of branches) done ? c.close() : c.enqueue(value);
      };
      const make = () => new ReadableStream({ start: (c) => void branches.push(c), pull });
      return [make(), make()];
    }
    [Symbol.asyncIterator]() {
      const reader = this.getReader();
      return {
        async next() {
          const r = await reader.read();
          if (r.done) reader.releaseLock();
          return r;
        },
        async return(value) {
          reader.releaseLock();
          return { value, done: true };
        },
        [Symbol.asyncIterator]() {
          return this;
        },
      };
    }
    values() {
      return this[Symbol.asyncIterator]();
    }
  }

  class WritableStream {
    constructor(sink = {}) {
      hidden(this, "_sink", sink);
      hidden(this, "_locked", false);
      hidden(this, "_ready", Promise.resolve(sink.start?.({ error() {} })));
    }
    get locked() {
      return this._locked;
    }
    getWriter() {
      this._locked = true;
      const s = this;
      return {
        write: (chunk) => (s._ready = s._ready.then(() => s._sink.write?.(chunk, { error() {} }))),
        close: () => (s._ready = s._ready.then(() => s._sink.close?.())),
        abort: (reason) => Promise.resolve(s._sink.abort?.(reason)),
        releaseLock() {
          s._locked = false;
        },
        get ready() {
          return s._ready;
        },
      };
    }
  }

  class TransformStream {
    constructor(transformer = {}) {
      let controller;
      this.readable = new ReadableStream({ start: (c) => void (controller = c) });
      const tc = {
        enqueue: (chunk) => controller.enqueue(chunk),
        error: (e) => controller.error(e),
        terminate: () => controller.close(),
      };
      this.writable = new WritableStream({
        start: () => transformer.start?.(tc),
        write: (chunk) => (transformer.transform ? transformer.transform(chunk, tc) : tc.enqueue(chunk)),
        close: async () => {
          await transformer.flush?.(tc);
          controller.close();
        },
      });
    }
  }

  class TextEncoderStream extends TransformStream {
    constructor() {
      const e = new TextEncoder();
      super({ transform: (chunk, c) => c.enqueue(e.encode(chunk)) });
    }
  }

  class TextDecoderStream extends TransformStream {
    constructor(label, options) {
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
      this.url = from ? from.url : new URL(String(input), native.baseURL()).href;
      this.method = String(init.method ?? from?.method ?? "GET").toUpperCase();
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
      return new Request(this);
    }
  }

  class Response extends Body {
    constructor(body = null, init = {}) {
      super();
      this.status = init.status ?? 200;
      if (this.status < 200 || this.status > 599) throw new RangeError(`init["status"] must be in the range of 200 to 599, inclusive.`);
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
      return new Response(null, { status, headers: { location: String(url) } });
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
          (message) => reject(new TypeError(message)),
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
    ReadableStream, ReadableStreamDefaultController, WritableStream, TransformStream, TextEncoderStream,
    TextDecoderStream, fetch, structuredClone, performance,
  });
  g.self = g;
  return { newFile: (bytes, name, type) => new File([new Uint8Array(bytes)], name, { type }) };
})
