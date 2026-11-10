// nsgo.js, the loader, runs an app `nsgo build -target web` built, in a
// page of its own or in an element of another page:
//
//   import { mount } from "https://example.com/app/nsgo.0123456789.js";
//   const player = await mount(element, "https://example.com/app/nsgo.json", {
//     params: { user: "Ada" },
//     appearance: "dark",
//     host: { call(name, args) { ... } },
//   });
//   player.hostEvent("message", { text: "Order filled" });
//   player.setAppearance("light");
//
// nsgo.json names the app's files relative to itself. The host answers
// the app's calls: call(name, args) returns the result or a promise of
// it, and a throw or a rejection fails the call with its message; posts
// go to post(name, args) when the host has one, else to call, whose
// answer nobody hears. openURL(url), when the host has it, sees the URLs
// the app opens and returns, or resolves with, whether it took one.
// Without a host option the host is nsgo.json's host module's, whose
// createHost(player) returns it, or a promise of it.
//
// appearance ("light", "dark" or "system"), locale (a BCP 47 tag) and
// fontScale stand in for the system's, as a native host's settings do;
// the player's setAppearance, setLocale and setFontScale change them.
// module, a compiled WebAssembly.Module or a promise of one, spares
// fetching and compiling the wasm, so a page showing the app many times,
// or the wasm compressed, compiles it once.

let apps = 0;

// mount runs the app nsgo.json at url describes in element, or the whole
// page when element is null; config, when given, is that file's content,
// which spares fetching it, and url (the page's when null) still what its
// names are relative to. It resolves with the player once the app runs.
export async function mount(element, url, options = {}) {
  const base = new URL(url ?? ".", document.baseURI);
  const config = options.config ?? (await (await fetch(base)).json());
  const at = (name) => new URL(name, base).href;
  const wasm = options.module ? null : fetch(at(config.wasm));
  if (typeof globalThis.Go !== "function") await loadScript(at(config.exec));

  const key = `nsgoHost${++apps}`;
  const environment = {
    appearance: options.appearance ?? "system",
    locale: options.locale ?? "",
    fontScale: options.fontScale ?? 0,
  };
  const bridge = {
    params: JSON.stringify(options.params ?? {}),
    environment: JSON.stringify(environment),
    events: [],
  };
  const setting = (name) => (value) => {
    environment[name] = value;
    bridge.environment = JSON.stringify(environment);
    bridge.setEnvironment?.(bridge.environment);
  };
  const player = {
    // hostEvent sends the app an event; a payload that is not JSON
    // throws here.
    hostEvent(name, payload) {
      const data = JSON.stringify(payload ?? null);
      if (data === undefined) throw new TypeError(`the event ${name}'s payload is not JSON`);
      if (bridge.deliver) bridge.deliver(String(name), data);
      else bridge.events.push([String(name), data]);
    },
    // setAppearance shows the app "light" or "dark", or "system" to
    // follow the system again; an appearance the app forces still wins.
    setAppearance: setting("appearance"),
    // setLocale has the app format for a BCP 47 tag, or "" for the
    // browser's language.
    setLocale: setting("locale"),
    // setFontScale sets the text size multiplier, or 0 for the browser's.
    setFontScale: setting("fontScale"),
  };
  let host = options.host;
  if (host === undefined && config.host) {
    const module = await import(at(config.host));
    host = await module.createHost?.(player);
    if (typeof host?.call !== "function") throw new Error("the app's host could not be loaded");
  }
  // An app in an element or with a host has the bridge; one filling the
  // page alone runs with no host.
  const bridged = element != null || host != null;
  if (bridged) globalThis[key] = bridge;
  if (host != null) {
    if (typeof host.call !== "function") throw new TypeError("a host has a call function");
    bridge.call = (id, name, data, post) => {
      const args = JSON.parse(data);
      const fn = post && typeof host.post === "function" ? host.post : host.call;
      const answer = Promise.resolve().then(() => fn.call(host, name, args));
      if (post) {
        answer.catch((e) => console.error(`nsgo: the host's post of ${name}:`, e));
        return;
      }
      answer.then(
        (value) => {
          let text;
          try {
            text = JSON.stringify(value ?? null);
          } catch (e) {
            bridge.reply(id, false, `the host's answer is not JSON: ${e.message}`);
            return;
          }
          bridge.reply(id, true, text);
        },
        (e) => bridge.reply(id, false, String(e?.message ?? e)),
      );
    };
    if (typeof host.openURL === "function") {
      bridge.openURL = (id, url) =>
        Promise.resolve()
          .then(() => host.openURL(url))
          .then(
            (handled) => bridge.opened(id, handled === true),
            () => bridge.opened(id, false),
          );
    }
  }

  const go = new Go();
  // The page's nsgo- parameters are the engine's flags: ?nsgo-fps&nsgo-lang=de.
  for (const [k, v] of new URLSearchParams(location.search)) {
    if (k.startsWith("nsgo-")) go.argv.push(v === "" ? `--${k}` : `--${k}=${v}`);
  }
  go.env = { NSGO_BASE: at(".") };
  if (config.resources) go.env.NSGO_RESOURCES = JSON.stringify(config.resources);
  if (bridged) go.env.NSGO_HOST = key;
  if (element) {
    element.dataset.nsgoMount = key;
    go.env.NSGO_MOUNT = `[data-nsgo-mount="${key}"]`;
  }
  const instance = options.module
    ? await WebAssembly.instantiate(await options.module, go.importObject)
    : (await WebAssembly.instantiateStreaming(wasm, go.importObject)).instance;
  go.run(instance);
  return player;
}

const scripts = new Map();

// loadScript runs the classic script at url once.
function loadScript(url) {
  if (!scripts.has(url)) {
    scripts.set(
      url,
      new Promise((resolve, reject) => {
        const script = document.createElement("script");
        script.src = url;
        script.onload = resolve;
        script.onerror = () => reject(new Error(`${url} could not be loaded`));
        document.head.append(script);
      }),
    );
  }
  return scripts.get(url);
}
