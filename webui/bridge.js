// AI ENV 浏览器模式：在没有 Wails 窗口时，替界面提供 window.go（经 HTTP 调用后端）与 window.runtime
;(function () {
  'use strict'
  var listeners = {}
  window.__AIENV_WEB__ = true

  function call(service, method, args) {
    return fetch('/__aienv/call', {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json', 'X-AIENV-Web': '1' },
      body: JSON.stringify({ service: service, method: method, args: args }),
    }).then(function (r) {
      if (r.status === 401) {
        location.reload()
        throw new Error('需要重新登录')
      }
      return r.json()
    }).then(function (d) {
      if (d.error !== undefined && d.error !== null) throw d.error
      return d.result
    })
  }

  function serviceProxy(service) {
    return new Proxy({}, {
      get: function (_, method) {
        if (typeof method !== 'string' || method === 'then') return undefined
        return function () { return call(service, method, Array.prototype.slice.call(arguments)) }
      },
    })
  }
  var services = {}
  window.go = {
    main: new Proxy({}, {
      get: function (_, service) {
        if (typeof service !== 'string' || service === 'then') return undefined
        return services[service] || (services[service] = serviceProxy(service))
      },
    }),
  }

  function on(name, cb, max) {
    var entry = { cb: cb, left: max }
    ;(listeners[name] = listeners[name] || []).push(entry)
    return function () {
      var list = listeners[name] || []
      var i = list.indexOf(entry)
      if (i >= 0) list.splice(i, 1)
    }
  }
  function dispatch(name, data) {
    var list = (listeners[name] || []).slice()
    list.forEach(function (entry) {
      try { entry.cb.apply(null, data || []) } catch (e) { console.error(e) }
      if (entry.left > 0 && --entry.left === 0) {
        var l = listeners[name] || []
        var i = l.indexOf(entry)
        if (i >= 0) l.splice(i, 1)
      }
    })
  }
  var noop = function () {}
  var no = function () { return Promise.resolve(false) }
  window.runtime = {
    EventsOnMultiple: on,
    EventsOn: function (n, cb) { return on(n, cb, -1) },
    EventsOnce: function (n, cb) { return on(n, cb, 1) },
    EventsOff: function (n) { Array.prototype.slice.call(arguments).forEach(function (k) { delete listeners[k] }) },
    EventsOffAll: function () { listeners = {} },
    EventsEmit: function (n) { dispatch(n, Array.prototype.slice.call(arguments, 1)) },
    BrowserOpenURL: function (u) { window.open(u, '_blank', 'noopener') },
    ClipboardGetText: function () { return navigator.clipboard ? navigator.clipboard.readText() : Promise.resolve('') },
    ClipboardSetText: function (t) { return navigator.clipboard ? navigator.clipboard.writeText(t).then(function () { return true }) : Promise.resolve(false) },
    WindowSetTitle: function (t) { document.title = t },
    WindowReload: function () { location.reload() },
    WindowReloadApp: function () { location.reload() },
    WindowMinimise: noop, WindowUnminimise: noop, WindowHide: noop, WindowShow: noop, WindowCenter: noop,
    WindowToggleMaximise: noop, WindowMaximise: noop, WindowUnmaximise: noop, WindowFullscreen: noop, WindowUnfullscreen: noop,
    WindowSetAlwaysOnTop: noop, WindowSetSize: noop, WindowSetMinSize: noop, WindowSetMaxSize: noop, WindowSetPosition: noop,
    WindowSetBackgroundColour: noop, WindowSetSystemDefaultTheme: noop, WindowSetLightTheme: noop, WindowSetDarkTheme: noop,
    WindowIsMaximised: no, WindowIsMinimised: no, WindowIsFullscreen: no, WindowIsNormal: function () { return Promise.resolve(true) },
    WindowGetSize: function () { return Promise.resolve({ w: window.innerWidth, h: window.innerHeight }) },
    WindowGetPosition: function () { return Promise.resolve({ x: 0, y: 0 }) },
    WindowPrint: function () { window.print() },
    Quit: noop, Hide: noop, Show: noop,
    OnFileDrop: noop, OnFileDropOff: noop, CanResolveFilePaths: function () { return false }, ResolveFilePaths: noop,
    LogPrint: noop, LogTrace: noop, LogDebug: noop, LogInfo: noop, LogWarning: noop, LogError: noop, LogFatal: noop,
    Environment: function () { return Promise.resolve({ buildType: 'production', platform: 'web', arch: '' }) },
    ScreenGetAll: function () { return Promise.resolve([]) },
  }

  // ---- 后端发来的事件：普通事件交给界面；aienv:web:* 由这里处理（打开链接、下载、选文件、通知）
  function pickFile(req) {
    var mask = document.createElement('div')
    mask.setAttribute('style', 'position:fixed;inset:0;z-index:2147483647;background:rgba(0,0,0,.45);display:flex;align-items:center;justify-content:center;font:14px system-ui,sans-serif')
    var box = document.createElement('div')
    box.setAttribute('style', 'background:#fff;color:#111;border-radius:12px;padding:20px;width:min(420px,calc(100vw - 32px));box-shadow:0 10px 40px rgba(0,0,0,.3)')
    var title = document.createElement('p')
    title.textContent = req.title || '选择文件'
    title.setAttribute('style', 'margin:0 0 12px;font-weight:600')
    var input = document.createElement('input')
    input.type = 'file'
    if (req.accept) input.accept = req.accept
    input.setAttribute('style', 'display:block;width:100%;margin-bottom:12px')
    var status = document.createElement('p')
    status.setAttribute('style', 'margin:0 0 12px;color:#666;font-size:12px')
    status.textContent = '文件会上传到运行 AI ENV 的机器上处理'
    var cancel = document.createElement('button')
    cancel.textContent = '取消'
    cancel.setAttribute('style', 'padding:6px 14px;border:1px solid #ccc;border-radius:8px;background:#fff;cursor:pointer')
    box.appendChild(title); box.appendChild(input); box.appendChild(status); box.appendChild(cancel)
    mask.appendChild(box)
    document.body.appendChild(mask)
    var done = false
    function close() { if (mask.parentNode) mask.parentNode.removeChild(mask) }
    cancel.onclick = function () {
      if (done) return
      done = true
      fetch('/__aienv/upload/' + req.token, { method: 'DELETE', credentials: 'same-origin', headers: { 'X-AIENV-Web': '1' } }).finally(close)
    }
    input.onchange = function () {
      var f = input.files && input.files[0]
      if (!f || done) return
      done = true
      status.textContent = '正在上传 ' + f.name + '…'
      fetch('/__aienv/upload/' + req.token + '?name=' + encodeURIComponent(f.name), {
        method: 'POST', credentials: 'same-origin', headers: { 'X-AIENV-Web': '1' }, body: f,
      }).finally(close)
    }
  }
  function internal(name, data) {
    var v = data && data[0]
    switch (name) {
      case 'aienv:web:open':
        window.open(v, '_blank', 'noopener')
        return true
      case 'aienv:web:download': {
        var a = document.createElement('a')
        a.href = v.url
        a.download = v.name || ''
        document.body.appendChild(a)
        a.click()
        a.remove()
        return true
      }
      case 'aienv:web:pick-file':
        pickFile(v)
        return true
      case 'aienv:web:notify':
        if ('Notification' in window) {
          var show = function () { try { new Notification(v.title || 'AI ENV', { body: v.body || '' }) } catch (e) {} }
          if (Notification.permission === 'granted') show()
          else if (Notification.permission !== 'denied') Notification.requestPermission().then(function (p) { if (p === 'granted') show() })
        }
        return true
    }
    return false
  }
  var source
  function connect() {
    source = new EventSource('/__aienv/events')
    source.onmessage = function (e) {
      try {
        var msg = JSON.parse(e.data)
        if (!internal(msg.name, msg.data)) dispatch(msg.name, msg.data)
      } catch (err) { console.error(err) }
    }
  }
  connect()
})()
