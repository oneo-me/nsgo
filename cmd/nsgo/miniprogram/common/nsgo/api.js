// The mini program's APIs as Go calls them: WeChat's names, options and
// results, whichever platform runs the app. WeChat, QQ and Douyin share
// them, give or take a few; Alipay's are translated one by one. What a
// platform lacks is left undefined, and Go answers the request as failed.

'use strict'

/** The platform's global object of APIs, by config.js's platform. */
function globalOf(platform) {
  /* global wx, qq, tt, my */
  switch (platform) {
    case 'qq':
      return typeof qq !== 'undefined' ? qq : undefined
    case 'douyin':
      return typeof tt !== 'undefined' ? tt : undefined
    case 'alipay':
      return typeof my !== 'undefined' ? my : undefined
  }
  return typeof wx !== 'undefined' ? wx : undefined
}

/** The APIs of platform, read from base, its global object. */
function apiOf(platform, base) {
  if (!base) throw new Error('nsgo: no ' + platform + ' APIs in this mini program')
  if (platform === 'wechat') return base
  const api = Object.create(base)
  // Older QQ and Douyin versions pick images alone.
  if (typeof base.chooseMedia !== 'function' && typeof base.chooseImage === 'function') {
    api.chooseMedia = (o) =>
      base.chooseImage({
        count: o.count,
        sourceType: o.sourceType,
        sizeType: o.sizeType,
        success: (r) => o.success({ tempFiles: imageFiles(r) }),
        fail: o.fail,
      })
  }
  if (platform === 'alipay') alipay(api, base)
  return api
}

/** The files chooseImage picked, as chooseMedia's tempFiles. */
function imageFiles(r) {
  if (r.tempFiles) return r.tempFiles.map((f) => ({ tempFilePath: f.tempFilePath || f.path }))
  return (r.tempFilePaths || r.apFilePaths || []).map((p) => ({ tempFilePath: p }))
}

/** Alipay's APIs under WeChat's names. */
function alipay(api, my) {
  const call = (name, o) => (typeof my[name] === 'function' ? my[name](o) : o.fail && o.fail({ errMsg: name + ' is missing' }))
  api.setClipboardData = (o) => call('setClipboard', { text: o.data, success: o.success, fail: o.fail })
  api.getClipboardData = (o) =>
    call('getClipboard', { success: (r) => o.success({ data: r.text }), fail: o.fail })
  api.setNavigationBarTitle = (o) => call('setNavigationBar', { title: o.title, success: o.success, fail: o.fail })
  api.setNavigationBarColor = (o) =>
    call('setNavigationBar', { backgroundColor: o.backgroundColor, success: o.success, fail: o.fail })
  api.hideKeyboard = (o) => {
    if (typeof my.hideKeyboard === 'function') my.hideKeyboard()
    if (o && o.success) o.success({})
  }
  api.showActionSheet = (o) =>
    call('showActionSheet', {
      title: o.alertText,
      items: o.itemList,
      success: (r) => (r.index >= 0 ? o.success({ tapIndex: r.index }) : o.fail({ errMsg: 'cancel' })),
      fail: o.fail,
    })
  api.showModal = (o) => {
    if (!o.showCancel) {
      return call('alert', {
        title: o.title,
        content: o.content,
        buttonText: o.confirmText,
        success: () => o.success({ confirm: true }),
        fail: o.fail,
      })
    }
    call('confirm', {
      title: o.title,
      content: o.content,
      confirmButtonText: o.confirmText,
      cancelButtonText: o.cancelText,
      success: (r) => o.success({ confirm: !!r.confirm }),
      fail: o.fail,
    })
  }
  api.chooseMedia = (o) =>
    call('chooseImage', {
      count: o.count,
      sourceType: o.sourceType,
      sizeType: o.sizeType,
      success: (r) => o.success({ tempFiles: imageFiles(r) }),
      fail: o.fail,
    })
  // Alipay picks no files from chats.
  api.chooseMessageFile = undefined
  api.openDocument = (o) =>
    call('openDocument', {
      filePath: o.filePath,
      fileType: String(o.filePath).split('.').pop(),
      success: o.success,
      fail: o.fail,
    })
  api.getStorageSync = (key) => {
    const r = my.getStorageSync({ key })
    return r ? r.data : undefined
  }
  api.setStorageSync = (key, data) => my.setStorageSync({ key, data })
  api.removeStorageSync = (key) => my.removeStorageSync({ key })
  api.getFileSystemManager = () => {
    const fs = my.getFileSystemManager()
    return {
      readFileSync(filePath) {
        const r = fs.readFileSync({ filePath })
        return r && r.data !== undefined ? r.data : r
      },
      writeFileSync(filePath, data) {
        return fs.writeFileSync({ filePath, data })
      },
    }
  }
}

module.exports = { globalOf, apiOf }
