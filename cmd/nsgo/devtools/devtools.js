"use strict";

// The page follows the app's messages from the dev server and sends it commands; everything it shows is built with textContent, never markup from the app.
const $ = (id) => document.getElementById(id);
const state = {
  surface: 0, tab: "elements", connected: false,
  elements: null, selected: 0, expanded: new Set(), rows: new Map(),
  owners: null, commits: [], paused: false, recording: false, steps: [], picking: false,
  dirty: { elements: false, state: false },
};

function el(tag, props, ...children) {
  const e = document.createElement(tag);
  for (const [k, v] of Object.entries(props || {})) {
    if (k === "class") e.className = v;
    else if (k.startsWith("on")) e.addEventListener(k.slice(2), v);
    else if (v !== undefined && v !== null && v !== false) e.setAttribute(k, v === true ? "" : v);
  }
  for (const c of children.flat()) if (c !== null && c !== undefined && c !== false) e.append(c instanceof Node ? c : String(c));
  return e;
}

function send(cmd) {
  return fetch("/_nsgo/devtools/command", { method: "POST", body: JSON.stringify({ surface: state.surface, ...cmd }) });
}

const sitePattern = /^(\/.*|[A-Za-z]:\\.*):(\d+)$/;
const isPath = (site) => !!site && sitePattern.test(site);

// A site is a whole path and a line, or NSGo's function for what it made itself; a path opens in the editor chosen.
function siteLink(site) {
  if (!site) return null;
  const m = sitePattern.exec(site);
  if (!m) return el("span", { class: "chip" }, site);
  const parts = m[1].split(/[\\/]/);
  const short = parts.slice(-2).join("/") + ":" + m[2];
  const editor = $("editor").value;
  const href = editor ? `${editor}://file${m[1].startsWith("/") ? "" : "/"}${m[1]}:${m[2]}` : undefined;
  return el("a", { class: "site", href, title: site }, short);
}

// Elements.

function label(n) {
  const bits = [el("span", { class: "kind" }, n.kind)];
  if (n.testID) bits.push(el("span", {}, " #" + n.testID));
  if (n.role) bits.push(el("span", { class: "dim" }, " " + n.role));
  const text = n.text || n.label;
  if (text) bits.push(el("span", { class: "dim" }, " " + JSON.stringify(text.length > 40 ? text.slice(0, 40) + "…" : text)));
  if (n.component && n.component !== "func") bits.push(el("span", { class: "chip" }, n.component));
  if (n.hidden) bits.push(el("span", { class: "chip" }, "hidden"));
  return bits;
}

function matches(n, q) {
  if (!q) return true;
  return [n.kind, n.testID, n.text, n.label, n.component, n.role].some((s) => s && s.toLowerCase().includes(q));
}

function renderElements() {
  const tree = $("element-tree");
  tree.replaceChildren();
  state.rows.clear();
  const root = state.elements;
  if (!root) return;
  const q = $("element-filter").value.trim().toLowerCase();
  const walk = (n, depth, parents) => {
    const kids = n.children || [];
    const hit = matches(n, q);
    const sub = [];
    for (const c of kids) sub.push(...walk(c, depth + 1, [...parents, n.id]));
    if (q && !hit && sub.length === 0) return [];
    const open = q || state.expanded.has(n.id) || depth < 2;
    const row = el("div", {
      class: "row" + (n.id === state.selected ? " selected" : ""), style: `padding-left:${8 + depth * 14}px`,
      onmouseenter: () => send({ op: "inspect", id: n.id }),
      onclick: () => select(n.id),
    }, el("span", { class: "twist", onclick: (e) => { e.stopPropagation(); toggle(n.id); } }, kids.length ? (open ? "▾" : "▸") : ""), label(n));
    state.rows.set(n.id, { row, node: n, parents });
    return [row, ...(open ? sub : [])];
  };
  tree.append(...walk(root, 0, []));
  tree.onmouseleave = () => send({ op: "inspect", id: state.selected });
  renderDetails();
}

function toggle(id) {
  state.expanded.has(id) ? state.expanded.delete(id) : state.expanded.add(id);
  renderElements();
}

function select(id) {
  state.selected = id;
  const found = state.rows.get(id);
  if (found) for (const p of found.parents) state.expanded.add(p);
  renderElements();
  send({ op: "inspect", id });
  const now = state.rows.get(id);
  if (now) now.row.scrollIntoView({ block: "nearest" });
}

function renderDetails() {
  const box = $("element-details");
  const found = state.rows.get(state.selected);
  if (!found) {
    box.replaceChildren(el("p", { class: "hint" }, "Hover a row to outline its view on the device, or press Pick and choose one there."));
    return;
  }
  const n = found.node;
  const [x, y, w, h] = n.bounds;
  const rows = [
    ["View", `${n.kind} ${n.id}`],
    ["Component", n.component ? [n.component, " ", siteLink(n.site)] : null],
    ["Selector", n.selector ? el("code", {}, n.selector) : el("span", { class: "hint" }, "none: give it a TestID")],
    ["Bounds", `${x}, ${y}, ${w} × ${h}`],
    ["Role", n.role], ["Label", n.label], ["Text", n.text], ["Hidden", n.hidden ? "yes" : null],
  ].filter(([, v]) => v);
  const style = (n.style || []).map((p) => el("tr", {}, el("td", {}, p.name), el("td", {}, p.value)));
  box.replaceChildren(
    el("table", {}, rows.map(([k, v]) => el("tr", {}, el("th", {}, k), el("td", {}, v)))),
    el("h4", {}, "Style"),
    style.length ? el("table", {}, style) : el("p", { class: "hint" }, "No style of its own."),
  );
}

// State.

function renderOwners() {
  const tree = $("owner-tree");
  tree.replaceChildren();
  if (!state.owners) return;
  const q = $("state-filter").value.trim().toLowerCase();
  const own = $("state-own").checked;
  const has = (s) => s && s.toLowerCase().includes(q);
  // A computation that holds nothing, such as an element's binding, counts on its owner's row rather than taking one.
  const leaf = (o) => (o.computations || []).length > 0 && !(o.values || []).length && !(o.children || []).length;
  const walk = (o, depth) => {
    // A value NSGo made for itself names NSGo's function rather than a file: hidden unless asked for.
    const values = (o.values || []).filter((v) => (own || isPath(v.site)) && (!q || has(v.site) || has(v.type) || has(v.text) || has(o.name)));
    // An owner of NSGo's own shows only when asked for: what the app has under it moves up in its place.
    const hidden = !own && depth > 0 && values.length === 0 && !isPath(o.site);
    const kids = o.children || [];
    const leaves = kids.filter(leaf);
    const sub = [];
    for (const c of kids) if (!leaf(c)) sub.push(...walk(c, hidden ? depth : depth + 1));
    if (q && values.length === 0 && sub.length === 0 && !has(o.name) && !has(o.site)) return [];
    if (hidden) return sub;
    const runs = leaves.reduce((n, c) => n + c.computations.reduce((m, x) => m + x.runs, 0), 0);
    const kinds = {};
    for (const c of leaves) for (const x of c.computations) kinds[x.kind] = (kinds[x.kind] || 0) + 1;
    const counted = Object.entries(kinds).map(([k, n]) => `${n} ${k}${n === 1 ? "" : "s"}`).join(", ");
    const comps = o.computations || [];
    const head = el("div", { class: "row", style: `padding-left:${8 + depth * 14}px` },
      el("span", { class: "kind" }, o.name), siteLink(o.site),
      comps.map((c) => el("span", { class: "chip" }, `ran ${c.runs}×`)),
      leaves.length ? el("span", { class: "chip", title: "The computations it owns that hold nothing, such as its elements' bindings" }, `${counted}, ran ${runs}×`) : null);
    const vals = values.map((v) => el("div", { class: "row value", style: `padding-left:${22 + depth * 14}px` },
      el("span", { class: "dim" }, v.type + " "), valueField(v), siteLink(v.site)));
    return [head, ...vals, ...sub];
  };
  tree.append(...walk(state.owners, 0));
}

function valueField(v) {
  if (!v.editable) return el("span", {}, v.text);
  const input = el("input", { value: v.text, title: "Enter sets the signal" });
  input.addEventListener("keydown", (e) => {
    if (e.key === "Enter") send({ op: "set", signal: v.id, value: input.value });
  });
  return input;
}

// Profiler.

function renderCommits() {
  const list = $("commits");
  const max = Math.max(1, ...state.commits.map((c) => c.ms));
  list.replaceChildren(el("table", {},
    el("tr", {}, el("th", {}, "#"), el("th", {}, "ms"), el("th", {}), el("th", {}, "items"), el("th", {}, "after"), el("th", {}, "ran")),
    state.commits.slice().reverse().map((c) => {
      const writes = (c.causes && c.causes.Writes) || [];
      const ran = c.causes ? `${c.causes.Renders} bindings, ${c.causes.Effects} effects, ${c.causes.Memos} memos` : "";
      return el("tr", {},
        el("td", { class: "num" }, c.n), el("td", { class: "num" }, c.ms.toFixed(2)),
        el("td", { style: "width:80px" }, el("div", { class: "bar", style: `width:${(80 * c.ms) / max}px` })),
        el("td", { class: "num" }, c.items),
        el("td", {}, writes.slice(0, 3).map((w) => el("div", {}, w.Site + (w.Count > 1 ? ` ×${w.Count}` : ""))), writes.length > 3 ? el("div", { class: "dim" }, `and ${writes.length - 3} more`) : null),
        el("td", { class: "dim" }, ran));
    })));
}

function renderHot(hot) {
  $("hot").replaceChildren(el("table", {},
    el("tr", {}, el("th", {}, "runs"), el("th", {}, "kind"), el("th", {}, "made at")),
    (hot || []).map((c) => el("tr", {}, el("td", { class: "num" }, c.runs), el("td", {}, c.kind), el("td", {}, siteLink(c.site) || el("span", { class: "dim" }, "—"))))));
}

// Recorder.

function testCode() {
  const steps = [];
  for (const s of state.steps) {
    const last = steps[steps.length - 1];
    if (s.action === "Type" && last && last.action === "Type" && last.selector === s.selector) last.text += s.text;
    else steps.push({ ...s });
  }
  const lines = steps.map((s) => {
    if (!s.selector) return `\t// a ${s.action.toLowerCase()} on a view with no selector: give it a TestID`;
    const sel = JSON.stringify(s.selector);
    return s.action === "Type" ? `\tapp.Type(${sel}, ${JSON.stringify(s.text)})` : `\tapp.${s.action}(${sel})`;
  });
  return ["func TestRecorded(t *testing.T) {", "\tapp := nsgotest.Start(t, Root)", ...lines, "}", ""].join("\n");
}

function renderRecorder() {
  $("record").textContent = state.recording ? "Stop" : "Record";
  $("record").classList.toggle("on", state.recording);
  $("test-code").textContent = testCode();
}

// Messages.

let refreshTimer = 0;
function refreshSoon() {
  if (refreshTimer) return;
  refreshTimer = setTimeout(() => {
    refreshTimer = 0;
    if (state.tab === "elements" && $("elements-live").checked) send({ op: "elements" });
    if (state.tab === "state" && $("state-live").checked) send({ op: "owners" });
  }, 700);
}

// The page learns the app's surfaces first; their reply asks for what the tab shows.
function hello() {
  state.surface = -1;
  send({ op: "surfaces" });
}

function onMessage(m) {
  switch (m.type) {
    case "connected": hello(); break;
    case "surfaces": {
      const sel = $("surface");
      const ids = m.data || [];
      sel.replaceChildren(...ids.map((id, i) => el("option", { value: id }, i === 0 ? "Main window" : `Window ${id}`)));
      // The first surface the app has, unless the page shows one it still has; then what the tab shows of it.
      if (!ids.includes(state.surface) && ids.length) {
        state.surface = ids[0];
        state.elements = null;
        showTab(state.tab);
      }
      sel.value = state.surface;
      send({ op: "highlight", on: $("highlight").checked });
      break;
    }
    case "elements": if (m.surface === state.surface) { state.elements = m.data; renderElements(); } break;
    case "owners": state.owners = m.data; renderOwners(); break;
    case "hover": { const r = state.rows.get(m.id); for (const x of state.rows.values()) x.row.style.outline = ""; if (r) { r.row.style.outline = "1px solid var(--accent)"; r.row.scrollIntoView({ block: "nearest" }); } break; }
    case "picked": state.picking = false; $("pick").classList.remove("on"); showTab("elements"); send({ op: "elements" }); state.selected = m.id; setTimeout(() => select(m.id), 300); break;
    case "step": state.steps.push(m.step); renderRecorder(); break;
    case "set": if (m.error) alert(m.error); send({ op: "owners" }); break;
    case "hot": if (state.tab === "profiler") renderHot(m.hot); break;
    case "commit":
      state.connected = true;
      if (!state.paused) { state.commits.push(m.commit); if (state.commits.length > 500) state.commits.shift(); if (state.tab === "profiler") renderCommits(); }
      refreshSoon();
      break;
    case "error": console.warn(m.error); break;
  }
  setStatus(true);
}

function setStatus(on) {
  const s = $("status");
  s.textContent = on ? "connected" : "disconnected";
  s.className = "status " + (on ? "on" : "off");
}

function showTab(name) {
  state.tab = name;
  for (const b of document.querySelectorAll("#tabs button")) b.classList.toggle("active", b.dataset.tab === name);
  for (const t of document.querySelectorAll(".tab")) t.classList.toggle("active", t.id === name);
  if (name === "elements") send({ op: "elements" });
  if (name === "state") send({ op: "owners" });
  if (name === "profiler") { renderCommits(); send({ op: "hot", n: 40 }); }
  if (name === "recorder") renderRecorder();
}

function connect() {
  const es = new EventSource("/_nsgo/devtools/stream");
  es.onmessage = (e) => { try { onMessage(JSON.parse(e.data)); } catch (err) { console.error(err); } };
  es.onerror = () => setStatus(false);
}

for (const b of document.querySelectorAll("#tabs button")) b.addEventListener("click", () => showTab(b.dataset.tab));
$("surface").addEventListener("change", (e) => { state.surface = Number(e.target.value); state.elements = null; showTab(state.tab); });
$("pick").addEventListener("click", () => { state.picking = !state.picking; $("pick").classList.toggle("on", state.picking); send({ op: "pick", on: state.picking }); });
$("highlight").addEventListener("change", (e) => send({ op: "highlight", on: e.target.checked }));
$("editor").value = localStorage.getItem("nsgo-devtools-editor") ?? "vscode";
$("editor").addEventListener("change", (e) => { localStorage.setItem("nsgo-devtools-editor", e.target.value); renderElements(); renderOwners(); });
$("element-filter").addEventListener("input", renderElements);
$("state-filter").addEventListener("input", renderOwners);
$("state-own").addEventListener("change", renderOwners);
$("commits-pause").addEventListener("click", (e) => { state.paused = !state.paused; e.target.textContent = state.paused ? "Resume" : "Pause"; });
$("commits-clear").addEventListener("click", () => { state.commits = []; renderCommits(); });
$("record").addEventListener("click", () => { state.recording = !state.recording; send({ op: "record", on: state.recording }); renderRecorder(); });
$("steps-clear").addEventListener("click", () => { state.steps = []; renderRecorder(); });
$("copy-test").addEventListener("click", () => navigator.clipboard.writeText(testCode()));
setInterval(() => { if (state.tab === "profiler") send({ op: "hot", n: 40 }); }, 1000);
connect();
renderRecorder();
