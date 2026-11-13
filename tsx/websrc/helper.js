(function (dispatch, release) {
  "use strict";
  const THROW = { throw: true }, MISSING = { missing: true };
  const goKey = Symbol("nsgo.go"), panicKey = Symbol("nsgo.panic");
  const reg = new FinalizationRegistry((id) => release(id));
  let thrown;
  const fn = (id) => {
    const f = function (...a) {
      const r = dispatch(id, this, new.target, ...a);
      if (r === THROW) { const e = thrown; thrown = undefined; throw e; }
      return r;
    };
    reg.register(f, id);
    return f;
  };
  const ctor = (id) => {
    const f = function (...a) {
      if (!new.target) throw new TypeError("Class constructor cannot be invoked without 'new'");
      const r = dispatch(id, this, new.target, ...a);
      if (r === THROW) { const e = thrown; thrown = undefined; throw e; }
      return r == null ? this : r;
    };
    reg.register(f, id);
    return f;
  };
  const toString = Object.prototype.toString, ev = eval;
  return {
    throw: THROW, missing: MISSING, goKey, panicKey,
    setThrown: (e) => { thrown = e; },
    fn, ctor,
    get: (o, k) => (k in o ? o[k] : MISSING),
    call: (f, t, ...a) => Reflect.apply(f, t, a),
    construct: (f, t, ...a) => Reflect.construct(f, a, t === undefined ? f : t),
    defineData: (o, k, v, w, c, e) => { Object.defineProperty(o, k, { value: v, writable: w, configurable: c, enumerable: e }); },
    defineAccessor: (o, k, g, s, c, e) => {
      const d = { configurable: c, enumerable: e };
      if (g !== undefined) d.get = g;
      if (s !== undefined) d.set = s;
      Object.defineProperty(o, k, d);
    },
    set: (o, k, v) => { o[k] = v; },
    goObject: (proto, id) => { const o = Object.create(proto); Object.defineProperty(o, goKey, { value: id }); reg.register(o, id); return o; },
    goID: (o) => (typeof o === "object" || typeof o === "function") && o !== null ? o[goKey] : undefined,
    newPromise: () => { let res, rej; const p = new Promise((a, b) => { res = a; rej = b; }); return [p, res, rej]; },
    track: (p, cb) => { p.then((v) => { cb(1, v); }, (e) => { cb(2, e); }); },
    run: (src) => ev(src),
    kind: (o) => {
      if (typeof o === "function") return 1;
      if (Array.isArray(o)) return 2;
      if (o instanceof Promise) return 3;
      if (o instanceof ArrayBuffer) return 4;
      if (o instanceof Date) return 5;
      if (o[goKey] !== undefined) return 6;
      if (o instanceof Error) return 7;
      return 0;
    },
    str: (v) => { try { return String(v); } catch (e) { return "[object]"; } },
    num: (v) => Number(v),
    className: (o) => toString.call(o).slice(8, -1),
    keys: (o) => Object.keys(o),
    array: (...a) => a,
    typeError: (m) => new TypeError(m),
    error: (m) => new Error(m),
    toObject: (v) => Object(v),
    setProto: (o, p) => { Object.setPrototypeOf(o, p); },
    getProto: (o) => Object.getPrototypeOf(o),
    del: (o, k) => delete o[k],
    bytes: (n) => new Uint8Array(n),
    buffer: (b) => (b instanceof ArrayBuffer ? new Uint8Array(b) : new Uint8Array(b.buffer, b.byteOffset, b.byteLength)),
    date: (ms) => new Date(ms),
    dateValue: (d) => d.getTime(),
    json: (o) => JSON.stringify(o),
    desc: (s) => s.description ?? "",
  };
})
