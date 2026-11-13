(function (now) {
  const D = globalThis.Date;
  const AppDate = function Date(...a) {
    if (!new.target) return new D(now()).toString();
    return Reflect.construct(D, a.length ? a : [now()], new.target === AppDate ? D : new.target);
  };
  AppDate.prototype = D.prototype;
  Object.defineProperty(D.prototype, "constructor", { value: AppDate, writable: true, configurable: true });
  AppDate.now = () => Math.floor(now());
  AppDate.parse = D.parse;
  AppDate.UTC = D.UTC;
  globalThis.Date = AppDate;
})
