// One view of the app at the frame NSGo laid it out at, as WeChat's
// nsgo-node is, for Alipay: what the user does with it goes to the page
// through the onNsgo prop each parent hands its children, [name, id,
// ...]. Every view binds every handler, as Alipay names them statically;
// an event bubbling up is kept by the innermost view that takes it.
// Keyframes do not run: Alipay's components have no animate.

// The phases of a touch event, as Go numbers them.
const PHASES = { touchstart: 0, touchmove: 1, touchend: 2, touchcancel: 3 }

const fingers = (list) => Array.prototype.map.call(list || [], (t) => [t.identifier, t.pageX, t.pageY])

// The time of the event of each kind a view kept last, so the views it
// bubbles through further out leave it.
const kept = {}

const keep = (kind, e) => {
  if (kept[kind] === e.timeStamp) return false
  kept[kind] = e.timeStamp
  return true
}

const point = (e) => {
  const d = e.detail || {}
  const t = (e.changedTouches && e.changedTouches[0]) || (e.touches && e.touches[0]) || {}
  return [d.pageX != null ? d.pageX : d.x != null ? d.x : t.pageX, d.pageY != null ? d.pageY : d.y != null ? d.y : t.pageY]
}

Component({
  props: { node: null, onNsgo: () => {} },
  didUnmount() {
    if (this.scrollTimer) clearTimeout(this.scrollTimer)
  },
  methods: {
    send(...detail) {
      this.props.onNsgo(detail)
    },
    // A child's event, on its way to the page.
    onNsgo(detail) {
      this.props.onNsgo(detail)
    },
    id() {
      return this.props.node.id
    },
    onTap(e) {
      if (this.props.node.p && keep('tap', e)) this.send('tap', this.id(), ...point(e))
    },
    onLongPress(e) {
      if (this.props.node.lp && keep('longpress', e)) this.send('longpress', this.id(), ...point(e))
    },
    onTouch(e) {
      const n = this.props.node
      const phase = PHASES[String(e.type).toLowerCase()]
      if (n.io && (phase === 0 || phase >= 2) && keep('press' + phase, e)) {
        this.send(phase === 0 ? 'pressin' : 'pressout', this.id(), ...point(e))
      }
      if (!n.tf) return
      // The views the event bubbles through send it again: Go keeps the
      // first, the innermost, by the event's time and phase.
      this.send('touch', this.id(), Math.round(e.timeStamp * 4) + phase, phase,
        fingers(e.changedTouches), fingers(e.touches), e.timeStamp)
    },
    onScroll(e) {
      const id = this.id()
      this.send('scroll', id, e.detail.scrollLeft, e.detail.scrollTop)
      // Scroll views report no end: one comes 150 ms after the last move.
      if (this.scrollTimer) clearTimeout(this.scrollTimer)
      this.scrollTimer = setTimeout(() => {
        this.scrollTimer = null
        this.send('scrollend', id)
      }, 150)
    },
    // Alipay's scroll views report no drags: the finger going down and up
    // on one stands for them.
    onDragStart() {
      this.send('dragstart', this.id())
    },
    onDragEnd() {
      this.send('dragend', this.id())
    },
    onRefresh() {
      this.send('refresh', this.id())
    },
    onInput(e) {
      this.send('input', this.id(), e.detail.value)
    },
    onConfirm(e) {
      this.send('confirm', this.id(), e.detail.value)
    },
    onFocus() {
      this.send('focus', this.id())
    },
    onBlur() {
      this.send('blur', this.id())
    },
    onLoad(e) {
      this.send('imgload', this.id(), e.detail.width, e.detail.height)
    },
    onError(e) {
      this.send('imgerror', this.id(), e.detail.errMsg || 'the image did not load')
    },
  },
})
