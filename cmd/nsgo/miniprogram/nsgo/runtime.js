// NSGo's runtime in the mini program's logic layer: it loads the app's
// WebAssembly (the engine and the app, in Go) with Go's wasm_exec.js,
// gives Go the bridge it calls (nsgoMP: measure, submit, tick, device),
// applies each batch to the tree the page draws (tree.js) and hands the
// user's taps, touches, scrolls and edits back to Go.
//
// The mini program has neither globalThis's browser objects nor
// WebAssembly: Go runs in a global object of its own, g, holding what
// syscall/js and wasm_exec.js reach, the wx APIs among them.
//
// Everything the mini program provides comes through host, so tests run
// it with stand-ins.

'use strict'

const { Tree } = require('./tree.js')
const goExec = require('./wasm_exec.js')
const config = require('./config.js')

function utf8Encode(text) {
  const out = []
  for (let i = 0; i < text.length; i++) {
    let cp = text.charCodeAt(i)
    if (cp >= 0xd800 && cp < 0xdc00 && i + 1 < text.length) {
      const lo = text.charCodeAt(i + 1)
      if (lo >= 0xdc00 && lo < 0xe000) {
        cp = 0x10000 + ((cp - 0xd800) << 10) + (lo - 0xdc00)
        i++
      }
    }
    if (cp < 0x80) out.push(cp)
    else if (cp < 0x800) out.push(0xc0 | (cp >> 6), 0x80 | (cp & 0x3f))
    else if (cp < 0x10000) out.push(0xe0 | (cp >> 12), 0x80 | ((cp >> 6) & 0x3f), 0x80 | (cp & 0x3f))
    else out.push(0xf0 | (cp >> 18), 0x80 | ((cp >> 12) & 0x3f), 0x80 | ((cp >> 6) & 0x3f), 0x80 | (cp & 0x3f))
  }
  return new Uint8Array(out)
}

function utf8Decode(bytes) {
  let out = ''
  let chunk = []
  for (let i = 0; i < bytes.length; ) {
    const b = bytes[i++]
    let cp
    if (b < 0x80) cp = b
    else if (b < 0xe0) cp = ((b & 0x1f) << 6) | (bytes[i++] & 0x3f)
    else if (b < 0xf0) cp = ((b & 0x0f) << 12) | ((bytes[i++] & 0x3f) << 6) | (bytes[i++] & 0x3f)
    else cp = ((b & 0x07) << 18) | ((bytes[i++] & 0x3f) << 12) | ((bytes[i++] & 0x3f) << 6) | (bytes[i++] & 0x3f)
    if (cp >= 0x10000) {
      cp -= 0x10000
      chunk.push(0xd800 + (cp >> 10), 0xdc00 + (cp & 0x3ff))
    } else chunk.push(cp)
    if (chunk.length >= 4096) {
      out += String.fromCharCode.apply(null, chunk)
      chunk = []
    }
  }
  return out + String.fromCharCode.apply(null, chunk)
}

class TextEncoder {
  encode(text = '') {
    return utf8Encode(text)
  }
  encodeInto(text, dest) {
    const bytes = utf8Encode(text)
    const n = Math.min(bytes.length, dest.length)
    dest.set(bytes.subarray(0, n))
    return { read: text.length, written: n }
  }
}

class TextDecoder {
  // Go hands it DataViews of its memory.
  decode(bytes) {
    if (!bytes) return ''
    if (bytes instanceof ArrayBuffer) return utf8Decode(new Uint8Array(bytes))
    return utf8Decode(new Uint8Array(bytes.buffer, bytes.byteOffset, bytes.byteLength))
  }
}

/** The mini program's APIs the runtime uses. */
function wxHost() {
  /* global wx, WXWebAssembly */
  const canvas = wx.createOffscreenCanvas({ type: '2d', width: 1, height: 1 })
  const context = canvas.getContext('2d')
  return {
    wx,
    instantiate: (path, imports) => WXWebAssembly.instantiate(path, imports),
    loadSubpackage: (name) =>
      new Promise((resolve, reject) => {
        if (!wx.loadSubpackage) return resolve()
        wx.loadSubpackage({ name, success: resolve, fail: reject })
      }),
    measure(font, text) {
      context.font = font
      return context.measureText(text).width
    },
    device,
    now: () => Date.now(),
    setTimeout: (f, ms) => setTimeout(f, ms),
    clearTimeout: (t) => clearTimeout(t),
    on(name, f) {
      if (typeof wx[name] === 'function') wx[name](f)
    },
  }
}

/** What Go reads of the page and the device (miniprogram.go's
 * environment): the page's size, the safe area within it, the
 * appearance and the system. */
function device() {
  const win = wx.getWindowInfo ? wx.getWindowInfo() : wx.getSystemInfoSync()
  const base = wx.getAppBaseInfo ? wx.getAppBaseInfo() : wx.getSystemInfoSync()
  const dev = wx.getDeviceInfo ? wx.getDeviceInfo() : wx.getSystemInfoSync()
  const safe = win.safeArea || { top: 0, left: 0, right: win.windowWidth, bottom: win.screenHeight }
  // The page starts under WeChat's navigation bar, or at the screen's
  // top with a custom one, whose capsule button the app keeps clear of.
  let top = 0
  const pageTop = win.screenHeight - win.windowHeight
  if (config.customNavigation) {
    const menu = wx.getMenuButtonBoundingClientRect ? wx.getMenuButtonBoundingClientRect() : null
    const status = win.statusBarHeight || safe.top || 0
    top = menu && menu.bottom ? menu.bottom + (menu.top - status) : status
  }
  const os = osOf(dev.platform, dev.system)
  return {
    width: win.windowWidth,
    height: win.windowHeight,
    pixelRatio: win.pixelRatio || 2,
    dark: base.theme === 'dark',
    fontScale: base.fontSizeScaleFactor || 1,
    language: base.language || 'zh_CN',
    os,
    osVersion: String(dev.system || '').replace(/^[^0-9]*/, ''),
    tablet: (dev.deviceOrientation && Math.min(win.screenWidth, win.screenHeight) >= 600) || false,
    safeTop: top,
    safeLeft: Math.max(0, safe.left || 0),
    safeRight: Math.max(0, win.windowWidth - (safe.right || win.windowWidth)),
    safeBottom: Math.max(0, win.windowHeight + pageTop - (safe.bottom || win.screenHeight)),
  }
}

/** NSGo's OS of wx.getDeviceInfo()'s platform (ios, android, ohos, mac,
 * windows; devtools, whose system names the simulated device's). */
function osOf(platform = '', system = '') {
  const name = platform === 'devtools' ? String(system).split(' ')[0].toLowerCase() : platform
  if (name === 'android' || name === 'ohos') return 'android'
  if (name === 'mac' || name === 'macos') return 'macos'
  if (name === 'windows') return 'windows'
  return 'ios'
}

/** The global object Go runs in. */
function goGlobal(host, bridge) {
  const started = Date.now()
  const g = {
    Object,
    Array,
    ArrayBuffer,
    DataView,
    Uint8Array,
    Uint8ClampedArray,
    Int8Array,
    Uint16Array,
    Int16Array,
    Uint32Array,
    Int32Array,
    Float32Array,
    Float64Array,
    Date,
    JSON,
    Math,
    Promise,
    Error,
    String,
    Number,
    Boolean,
    console,
    TextEncoder,
    TextDecoder,
    setTimeout: (f, ms) => host.setTimeout(f, ms),
    clearTimeout: (t) => host.clearTimeout(t),
    performance: { now: () => Date.now() - started },
    crypto: {
      getRandomValues(a) {
        for (let i = 0; i < a.length; i++) a[i] = Math.floor(Math.random() * 256)
        return a
      },
    },
    wx: host.wx,
    nsgoMP: bridge,
  }
  g.globalThis = g
  return g
}

/**
 * Starts the app in page, a Page whose data has root and ov; resolves to
 * the running app (event, stop).
 */
function start(page, host = wxHost()) {
  const tree = new Tree()
  let flushQueued = false
  let stopped = false
  let frameTimer = null
  const flush = () => {
    flushQueued = false
    if (stopped) return
    const data = tree.take()
    if (Object.keys(data).length > 0) page.setData(data)
  }
  const queueFlush = () => {
    if (!flushQueued) {
      flushQueued = true
      Promise.resolve().then(flush)
    }
  }
  let ready
  const started = new Promise((resolve) => (ready = resolve))
  const bridge = {
    // Set by Go: event(name, id, ...).
    event: null,
    ready: () => ready(),
    device: () => host.device(),
    measure: (font, text) => host.measure(font, text),
    submit(json) {
      tree.add(JSON.parse(json))
      queueFlush()
    },
    tick() {
      if (frameTimer != null || stopped) return
      frameTimer = host.setTimeout(() => {
        frameTimer = null
        if (!stopped) bridge.event('frame', 0)
      }, 16)
    },
    navigationColors(front, back) {
      if (!config.customNavigation && host.wx && host.wx.setNavigationBarColor) {
        host.wx.setNavigationBarColor({ frontColor: front, backgroundColor: back })
      }
    },
  }
  const g = goGlobal(host, bridge)
  goExec(g, TextEncoder, TextDecoder, g.performance, g.crypto, { Instance: Object })
  const go = new g.Go()
  go.env = Object.assign({}, config.env)
  return host
    .loadSubpackage(config.subpackage)
    .then(() => host.instantiate(config.wasm, go.importObject))
    .then((result) => {
      go.run(result.instance || result).catch((err) => console.error('nsgo: the app stopped', err))
      return started
    })
    .then(() => {
      const send = (...args) => {
        if (!stopped && bridge.event) bridge.event(...args)
      }
      host.on('onThemeChange', () => send('device', 0))
      host.on('onWindowResize', () => send('device', 0))
      host.on('onAppShow', () => send('lifecycle', 0, true))
      host.on('onAppHide', () => send('lifecycle', 0, false))
      return {
        tree,
        /** An event of the page: its name, the view's ID, then what it carries. */
        event(name, id, ...rest) {
          if (name === 'input') tree.typed(id, rest[0])
          if (name === 'scroll') tree.scrolled(id, rest[0], rest[1])
          send(name, id, ...rest)
        },
        stop() {
          stopped = true
          if (frameTimer != null) host.clearTimeout(frameTimer)
        },
      }
    })
}

module.exports = { start, device, osOf, utf8Encode, utf8Decode, goGlobal }
