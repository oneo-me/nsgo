// The tree of views the render layer draws, kept in the logic layer as
// plain objects, and the setData paths that bring the page up to date
// after the batches of a turn. internal/platform/miniprogram's mount.go
// lists the operations.
//
// A node is { id, k, f, s, a, tr, ... , c }: its kind, frame
// [x, y, width, height], style, animated style, frame transition, what
// it reports, its text, scroll, image, field or picture, and its children
// in drawing order (the last on top). The page's data is { root, ov }:
// the root view's node and the modals, which show over it wherever the
// app inserted them, as native platforms show them in windows of their
// own.

'use strict'

// The bits of op e.
const PRESS = 1
const PRESS_IN_OUT = 2
const LONG_PRESS = 4
const FOLLOW = 8
const CAPTURE = 16

function node(id, kind) {
  return { id, k: kind, f: [0, 0, 0, 0], s: '', a: '', tr: '', p: false, io: false, lp: false, tf: false, tc: false, c: [] }
}

class Tree {
  constructor() {
    this.root = null
    this.nodes = new Map()
    // Kept apart from the nodes, which setData copies whole.
    this.parents = new Map()
    this.modals = []
    this.back = false
    this.changes = null
  }

  /** Applies a batch's operations, keeping what changed for take. */
  add(ops) {
    if (!this.changes) this.changes = { structure: new Set(), fields: new Map(), page: {} }
    const { structure, fields, page } = this.changes
    const touch = (id, field) => {
      if (!fields.has(id)) fields.set(id, new Set())
      fields.get(id).add(field)
    }
    const set = (id, field, value) => {
      const n = this.nodes.get(id)
      if (!n) return
      n[field] = value
      touch(id, field)
    }
    for (const op of ops) {
      const id = op[1]
      switch (op[0]) {
        case 'c':
          this.nodes.set(id, node(id, op[2]))
          break
        case 'root':
          this.root = this.nodes.get(id) || null
          page.root = true
          break
        case 'i': {
          const parent = this.nodes.get(id)
          const child = this.nodes.get(op[2])
          if (!parent || !child) break
          this.detach(child, structure)
          this.parents.set(child.id, parent.id)
          if (child.k === 'modal') {
            this.modals.push(child)
            page.ov = true
            break
          }
          const before = op[3] == null ? -1 : parent.c.findIndex((n) => n.id === op[3])
          if (before < 0) parent.c.push(child)
          else parent.c.splice(before, 0, child)
          structure.add(parent.id)
          break
        }
        case 'r': {
          const child = this.nodes.get(id)
          if (child) this.detach(child, structure)
          break
        }
        case 'd': {
          const child = this.nodes.get(id)
          if (child) this.detach(child, structure)
          this.nodes.delete(id)
          fields.delete(id)
          if (this.root === child) {
            this.root = null
            page.root = true
          }
          break
        }
        case 'f':
          set(id, 'f', [op[2], op[3], op[4], op[5]])
          break
        case 'e': {
          const bits = op[2]
          set(id, 'p', (bits & PRESS) !== 0)
          set(id, 'io', (bits & PRESS_IN_OUT) !== 0)
          set(id, 'lp', (bits & LONG_PRESS) !== 0)
          set(id, 'tf', (bits & FOLLOW) !== 0)
          set(id, 'tc', (bits & CAPTURE) !== 0)
          break
        }
        case 'ss':
          set(id, 'cs', [op[2], op[3]])
          break
        case 'so': {
          // A scroll view scrolls when scroll-top or scroll-left change: an
          // offset equal to the one set last, after the user scrolled away
          // from it, is set a hair apart so it still changes.
          const n = this.nodes.get(id)
          if (!n) break
          let [x, y] = [op[2], op[3]]
          if (n.so && n.so[0] === x && n.so[1] === y) y += 0.01
          set(id, 'so', [x, y, !!op[4]])
          break
        }
        case 'im':
          set(id, 'im', op[2] ? { src: op[2], mode: op[3] } : null)
          break
        case 'sel':
          set(id, 'sel', [op[2], op[3]])
          break
        case 'bk':
          this.back = op[2]
          page.back = true
          break
        case 'iv':
          set(id, 'v', op[2])
          break
        default:
          // s, a, tr, ka, t, sc, fd, fo, cv: the value as it is.
          set(id, op[0], op[2])
      }
    }
  }

  /** The user edited field id to text: what it shows now. */
  typed(id, text) {
    const n = this.nodes.get(id)
    if (n) n.v = text
  }

  /** Scroll view id is at x, y, as the user left it. */
  scrolled(id, x, y) {
    const n = this.nodes.get(id)
    if (n && n.so) n.so = [x, y, false]
  }

  detach(child, structure) {
    const parentId = this.parents.get(child.id)
    if (parentId == null) return
    this.parents.delete(child.id)
    if (child.k === 'modal') {
      const i = this.modals.indexOf(child)
      if (i >= 0) this.modals.splice(i, 1)
      if (!this.changes) this.changes = { structure: new Set(), fields: new Map(), page: {} }
      this.changes.page.ov = true
      return
    }
    const parent = this.nodes.get(parentId)
    if (!parent) return
    const i = parent.c.indexOf(child)
    if (i >= 0) parent.c.splice(i, 1)
    structure.add(parent.id)
  }

  /** The data path of node id ('root.c[2].c[0]', 'ov[1].c[0]'), or null
   * when it is not shown. */
  path(id) {
    const steps = []
    let current = id
    for (;;) {
      const n = this.nodes.get(current)
      if (!n) return null
      if (n === this.root) return 'root' + steps.reverse().join('')
      if (n.k === 'modal') {
        const i = this.modals.indexOf(n)
        return i < 0 ? null : `ov[${i}]` + steps.reverse().join('')
      }
      const parentId = this.parents.get(current)
      if (parentId == null) return null
      const parent = this.nodes.get(parentId)
      const i = parent ? parent.c.indexOf(n) : -1
      if (i < 0) return null
      steps.push(`.c[${i}]`)
      current = parentId
    }
  }

  /** Whether node id goes to the page with an ancestor whose children
   * are sent whole. */
  covered(id, structure) {
    const own = this.nodes.get(id)
    if (own && own.k === 'modal') return false
    let current = this.parents.get(id)
    while (current != null) {
      if (structure.has(current)) return true
      const n = this.nodes.get(current)
      if (!n || n === this.root || n.k === 'modal') return false
      current = this.parents.get(current)
    }
    return false
  }

  /** The setData object of what changed since the last take. */
  take() {
    const changes = this.changes
    this.changes = null
    if (!changes) return {}
    const { structure, fields, page } = changes
    const data = {}
    if (page.root) data.root = this.root
    if (page.ov) data.ov = this.modals
    if (page.back) data.back = this.back
    if (page.root && page.ov) return data
    for (const id of structure) {
      if (this.covered(id, structure)) continue
      const path = this.path(id)
      if (path == null) continue
      if (page.root && path.startsWith('root')) continue
      if (page.ov && path.startsWith('ov')) continue
      data[`${path}.c`] = this.nodes.get(id).c
    }
    for (const [id, names] of fields) {
      // Sent with a parent's children: the node object is the same one,
      // already up to date.
      if (this.covered(id, structure)) continue
      const path = this.path(id)
      if (path == null) continue
      if (page.root && path.startsWith('root')) continue
      if (page.ov && path.startsWith('ov')) continue
      const n = this.nodes.get(id)
      for (const name of names) data[`${path}.${name}`] = n[name]
    }
    return data
  }
}

module.exports = { Tree }
