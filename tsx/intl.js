// Intl, and the locale methods of Number, Date and String, on the
// natives intl.go gives this script.
(function (g, native) {
  "use strict";
  const resolved = (locales, options, extra) => ({ locale: native.locale(locales), ...(options ?? {}), ...extra });

  class NumberFormat {
    constructor(locales, options = {}) {
      this._locales = locales;
      this._options = options ?? {};
    }
    format(x) {
      return native.number(Number(x), this._locales, this._options);
    }
    formatToParts(x) {
      return [{ type: "literal", value: this.format(x) }];
    }
    resolvedOptions() {
      return resolved(this._locales, this._options, { numberingSystem: "latn" });
    }
    static supportedLocalesOf(locales) {
      return [].concat(locales ?? []);
    }
  }

  class DateTimeFormat {
    constructor(locales, options = {}) {
      this._locales = locales;
      this._options = options ?? {};
    }
    format(d = Date.now()) {
      return native.date(+new Date(d), this._locales, this._options, "");
    }
    formatToParts(d) {
      return [{ type: "literal", value: this.format(d) }];
    }
    resolvedOptions() {
      return resolved(this._locales, this._options, { calendar: "gregory", timeZone: this._options.timeZone ?? "UTC" });
    }
    static supportedLocalesOf(locales) {
      return [].concat(locales ?? []);
    }
  }

  class PluralRules {
    constructor(locales, options = {}) {
      this._locales = locales;
      this._options = options ?? {};
    }
    select(n) {
      return native.plural(Number(n), this._locales, this._options);
    }
    resolvedOptions() {
      return resolved(this._locales, this._options);
    }
  }

  class Collator {
    constructor(locales, options = {}) {
      this._locales = locales;
      this._options = options ?? {};
      this.compare = (a, b) => native.compare(String(a), String(b), this._locales, this._options);
    }
    resolvedOptions() {
      return resolved(this._locales, this._options);
    }
  }

  const units = ["year", "quarter", "month", "week", "day", "hour", "minute", "second"];
  class RelativeTimeFormat {
    constructor(locales, options = {}) {
      this._locales = locales;
      this._options = options ?? {};
    }
    format(value, unit) {
      unit = String(unit).replace(/s$/, "");
      if (!units.includes(unit)) throw new RangeError(`Invalid unit argument: ${unit}`);
      const n = Number(value);
      if (this._options.numeric === "auto") {
        const words = { day: ["yesterday", "today", "tomorrow"], year: ["last year", "this year", "next year"],
          month: ["last month", "this month", "next month"], week: ["last week", "this week", "next week"] }[unit];
        if (words && Math.abs(n) <= 1 && Number.isInteger(n)) return words[n + 1];
        if (n === 0 && ["hour", "minute", "second"].includes(unit)) return unit === "second" ? "now" : `this ${unit}`;
      }
      const abs = Math.abs(n);
      const text = `${native.number(abs, this._locales, {})} ${unit}${abs === 1 ? "" : "s"}`;
      return n < 0 || Object.is(n, -0) ? `${text} ago` : `in ${text}`;
    }
    resolvedOptions() {
      return resolved(this._locales, this._options);
    }
  }

  class ListFormat {
    constructor(locales, options = {}) {
      this._options = options ?? {};
    }
    format(list) {
      const items = [...list].map(String);
      const word = this._options.type === "disjunction" ? "or" : "and";
      if (items.length < 2) return items.join("");
      if (items.length === 2) return `${items[0]} ${word} ${items[1]}`;
      return `${items.slice(0, -1).join(", ")}, ${word} ${items[items.length - 1]}`;
    }
  }

  g.Intl = { NumberFormat, DateTimeFormat, PluralRules, Collator, RelativeTimeFormat, ListFormat,
    getCanonicalLocales: (l) => [].concat(l ?? []).map((x) => native.locale(x)) };

  const define = (proto, name, fn) => Object.defineProperty(proto, name, { value: fn, writable: true, configurable: true });
  define(Number.prototype, "toLocaleString", function (locales, options) {
    return native.number(Number(this), locales, options ?? {});
  });
  define(BigInt.prototype, "toLocaleString", function (locales, options) {
    return native.number(Number(this), locales, options ?? {});
  });
  define(Date.prototype, "toLocaleString", function (locales, options) {
    return native.date(+this, locales, options ?? {}, "");
  });
  define(Date.prototype, "toLocaleDateString", function (locales, options) {
    return native.date(+this, locales, options ?? {}, "date");
  });
  define(Date.prototype, "toLocaleTimeString", function (locales, options) {
    return native.date(+this, locales, options ?? {}, "time");
  });
  define(String.prototype, "localeCompare", function (that, locales, options) {
    return native.compare(String(this), String(that), locales, options ?? {});
  });
})
