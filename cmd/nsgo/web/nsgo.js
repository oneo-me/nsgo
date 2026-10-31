// nsgo.js, the loader, runs an app `nsgo build -target web` built, in a
// page of its own or in an element of another page:
//
//   import { mount } from "https://example.com/app/nsgo.0123456789.js";
//   const player = await mount(element, "https://example.com/app/nsgo.json", {
//     params: { user: "Ada" },
//     host: { call(name, args) { ... } },
//   });
//   player.hostEvent("message", { text: "Order filled" });
//
// nsgo.json names the app's files relative to itself. The host answers
// the app's calls: call(name, args) returns the result or a promise of
// it, and a throw or a rejection fails the call with its message; posts
// go to post(name, args) when the host has one, else to call, whose
// answer nobody hears. openURL(url), when the host has it, sees the URLs
// the app opens and returns, or resolves with, whether it took one.
// Without a host option the host is nsgo.json's host module's, whose
// createHost(player) returns it, or a promise of it.

let apps = 0;

// mount runs the app nsgo.json at url describes in element, or the whole
// page when element is null; config, when given, is that file's content,
// which spares fetching it, and url (the page's when null) still what its
// names are relative to. It resolves with the player once the app runs.
export async function mount(element, url, options = {}) {
  const base = new URL(url ?? ".", document.baseURI);
  const config = options.config ?? (await (await fetch(base)).json());
  const at = (name) => new URL(name, base).href;
  const wasm = fetch(at(config.wasm));
  if (typeof globalThis.Go !== "function") await loadScript(at(config.exec));

  const key = `nsgoHost${++apps}`;
  const bridge = { params: JSON.stringify(options.params ?? {}), events: [] };
  const player = {
    // hostEvent sends the app an event; a payload that is not JSON
    // throws here.
    hostEvent(name, payload) {
      const data = JSON.stringify(payload ?? null);
      if (data === undefined) throw new TypeError(`the event ${name}'s payload is not JSON`);
      if (bridge.deliver) bridge.deliver(String(name), data);
      else bridge.events.push([String(name), data]);
    },
  };
  let host = options.host;
  if (host === undefined && config.host) {
    const module = await import(at(config.host));
    host = await module.createHost?.(player);
    if (typeof host?.call !== "function") throw new Error("the app's host could not be loaded");
  }
  if (host != null) {
    if (typeof host.call !== "function") throw new TypeError("a host has a call function");
    globalThis[key] = bridge;
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
  go.env = { NSGO_BASE: at(".") };
  if (config.resources) go.env.NSGO_RESOURCES = JSON.stringify(config.resources);
  if (host != null) go.env.NSGO_HOST = key;
  if (element) {
    element.dataset.nsgoMount = key;
    go.env.NSGO_MOUNT = `[data-nsgo-mount="${key}"]`;
  }
  const { instance } = await WebAssembly.instantiateStreaming(wasm, go.importObject);
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
