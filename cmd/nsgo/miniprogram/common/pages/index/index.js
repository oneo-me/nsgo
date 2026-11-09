// The app's page: starts the app (nsgo/runtime.js), which keeps root and
// ov up to date, and hands it the nsgo events of the views.
const { start } = require('../../nsgo/runtime.js')

Page({
  data: { root: null, ov: [], back: false },

  onLoad() {
    start(this)
      .then((app) => {
        this.app = app
      })
      .catch((error) => console.error('nsgo: the app did not start', error))
  },

  onUnload() {
    if (this.app) this.app.stop()
  },

  // An nsgo event: WeChat, QQ and Douyin's component event, whose detail
  // it is, or what Alipay's component hands its onNsgo prop.
  onNsgo(e) {
    if (this.app) this.app.event(...(Array.isArray(e) ? e : e.detail))
  },

  // The page container left with the back: the app goes back, and shows
  // it again while it has more to go back through.
  onBack() {
    this.setData({ back: false })
    if (this.app) this.app.event('back', 0)
  },
})
