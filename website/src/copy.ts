export type Lang = 'en' | 'zh';

export const REPO = 'https://github.com/oneo-me/nsgo';

export const links = {
  repo: REPO,
  issues: `${REPO}/issues`,
  readme: `${REPO}#usage`,
  docs: `${import.meta.env.BASE_URL}docs/`,
  architecture: `${REPO}/blob/main/ARCHITECTURE.md`,
  roadmap: `${REPO}/blob/main/docs/roadmap.md`,
  changelog: `${REPO}/blob/main/CHANGELOG.md`,
  license: `${REPO}/blob/main/LICENSE`,
  notice: `${REPO}/blob/main/NOTICE`,
  embedding: `${REPO}/blob/main/docs/architecture/embedding.md`,
  blog: 'https://oneo.me',
  x: 'https://x.com/oneo666',
  nsh: 'https://github.com/longbridge/nsh',
  nshSite: 'https://nsh.dev',
};

const en = {
  meta: {
    title: 'NSGo — Native UI, written in Go',
    description:
      'NSGo is a cross-platform native UI engine written in Go. Signals, layout and text run in Go; iOS, Android, macOS, Windows, Linux and the web draw with their own native views.',
  },
  nav: {
    skip: 'Skip to content',
    primary: 'Primary',
    links: [
      { label: 'Gallery', href: '#gallery' },
      { label: 'Engine', href: '#engine' },
      { label: 'Platforms', href: '#platforms' },
      { label: 'Code', href: '#code' },
      { label: 'Toolchain', href: '#toolchain' },
      { label: 'Docs', href: links.docs },
    ],
    github: 'GitHub',
    menu: 'Open menu',
    close: 'Close menu',
    language: 'Switch to Chinese',
    languageShort: '中文',
    theme: {
      label: 'Appearance',
      system: 'Follow the system',
      light: 'Light',
      dark: 'Dark',
    },
  },
  hero: {
    eyebrow: 'Open source · Apache-2.0',
    titleTop: 'Native UI,',
    titleBefore: 'written in ',
    titleAccent: 'Go',
    titleAfter: '.',
    lead: 'NSGo is a cross-platform native UI engine. Signals, layout and text run in Go; every platform draws with its own views.',
    points: [
      { title: 'One codebase', body: 'iOS, Android, macOS, Windows, Linux and the web.' },
      { title: 'Native by default', body: 'Platform views, text and scrolling; the web uses the DOM.' },
      { title: 'Go end to end', body: 'Signals, flexbox and grid, animation and tests in one language.' },
    ],
    source: 'View on GitHub',
    architecture: 'Read the architecture',
    runsOn: 'Runs on',
    commandLabel: 'Start from the repository',
    command: 'go tool nsgo dev examples/gallery',
    commandCopy: 'Copy the command',
  },
  gallery: {
    title: 'gallery.wasm',
    wasm: 'WASM',
    idleKicker: 'WebAssembly gallery',
    idleTitle: 'The whole engine, in this browser.',
    idleBody:
      'Gallery is a real NSGo app: every element, component and service, compiled from Go to WebAssembly.',
    idleMeta: (download: string | null, engine: string | null) =>
      [
        download ? `${download} to download` : null,
        engine ? `${engine} engine` : null,
        'compiled locally',
      ]
        .filter(Boolean)
        .join(' · '),
    start: 'Load the gallery',
    startNote: 'The app follows the site’s appearance. Nothing is uploaded.',
    phases: {
      prepare: 'Preparing WebAssembly',
      download: 'Downloading the engine',
      decompress: 'Decompressing the engine',
      compile: 'Compiling WebAssembly',
      start: 'Starting the gallery',
      ready: 'Ready',
    },
    steps: ['Download', 'Compile', 'Run'],
    stepsLabel: 'Loading steps',
    loadingDetail: (loaded: string, total: string) => `${loaded} of ${total}`,
    loadingNote: 'The engine compiles in your browser. Nothing is uploaded.',
    cancel: 'Cancel',
    errorTitle: 'The gallery could not start.',
    errorBody: 'Check your connection and try again.',
    retry: 'Try again',
    ready: 'Running',
    caption: 'The real gallery, compiled from Go to WebAssembly.',
    open: 'Open full screen',
  },
  engine: {
    kicker: 'The engine',
    title: 'Native means the platform draws.',
    lead: 'NSGo owns the app model, not the pixels. Each commit produces one batch of mount items, and a thin backend applies it to the platform’s own views.',
    flow: [
      { label: '01 · Go', title: 'Reactive model', body: 'signals · layout · text' },
      { label: 'MountBatch', title: 'One commit', body: 'only what changed' },
      { label: 'Platform', title: 'Native views', body: 'AppKit · Android · DOM' },
    ],
    flowNote: 'Styles, text and layout resolve in Go. Animation runs on the platform’s frame clock.',
    layers: [
      {
        n: '01',
        title: 'Go owns the model',
        body: 'Components run once. Signals update what reads them, regions rebuild only what changed, and layout resolves the difference.',
      },
      {
        n: '02',
        title: 'One batch per commit',
        body: 'Styles, text, layout and mount items cross in one hop. Transitions and animated values run without a commit per frame.',
      },
      {
        n: '03',
        title: 'The platform draws',
        body: 'AppKit, Android Views, Direct2D, GTK 4 or DOM elements; CoreText, DirectWrite, Pango or the browser’s own text.',
      },
    ],
  },
  platforms: {
    kicker: 'Platforms',
    title: 'Six platforms. One toolchain.',
    lead: 'Every target builds on macOS with Go. iOS links with Xcode; x86_64 Android uses an NDK.',
    status: 'Runs',
    items: [
      {
        id: 'ios',
        name: 'iOS',
        stack: 'UIKit · CoreText',
        body: 'Linked with Xcode, running on devices and the simulator.',
        command: 'ios',
      },
      {
        id: 'android',
        name: 'Android',
        stack: 'Android Views · StaticLayout',
        body: 'A Go process behind a prebuilt Java runtime.',
        command: 'android_arm64',
      },
      {
        id: 'macos',
        name: 'macOS',
        stack: 'AppKit · CoreText',
        body: 'Reaches the system with purego; CGO stays off.',
        command: 'macos_arm64',
      },
      {
        id: 'windows',
        name: 'Windows',
        stack: 'Win32 · Direct2D · DirectWrite',
        body: 'COM and the window system through syscall.',
        command: 'windows_x64',
      },
      {
        id: 'linux',
        name: 'Linux',
        stack: 'GTK 4 · Pango · Cairo',
        body: 'Loaded with purego; GTK 4.14 or newer.',
        command: 'linux_x64',
      },
      {
        id: 'web',
        name: 'Web',
        stack: 'HTML · CSS · WebAssembly',
        body: 'DOM elements on the web backend, as in the gallery above.',
        command: 'web',
      },
    ],
    footnote: 'Apps also embed in iOS, Android and Go desktop apps.',
    footnoteLink: 'Embedding',
  },
  code: {
    kicker: 'The API',
    title: 'A component is a Go value.',
    lead: 'Elements are structs and signals are functions. No template language and no runtime layer sit between your code and the engine.',
    aside: [
      {
        title: 'No template language',
        body: 'A component is a Go type with a Render method; elements are ordinary struct literals.',
      },
      {
        title: 'No extra bindings',
        body: 'One go get -tool installs nsgo; go tool nsgo runs it anywhere Go runs.',
      },
      {
        title: 'Test like a user',
        body: 'nsgotest finds a control by its text and presses it, headless or in a native window.',
      },
    ],
    tabs: [{ file: 'main.go' }, { file: 'app_test.go' }],
    testNote: 'The same test runs headless or in a native window.',
    more: 'Getting started',
  },
  toolchain: {
    kicker: 'Toolchain',
    title: 'One tool, from source to store.',
    lead: 'nsgo calls Go and Xcode, then packages each target. One executable covers the loop.',
    terminalTitle: 'shell',
    commands: [
      { command: 'go tool nsgo dev examples/gallery', note: 'run with hot reload' },
      { command: 'go tool nsgo test -target macos', note: 'native input and screenshots' },
      { command: 'go tool nsgo build -target all examples/gallery', note: 'signed packages, one per target' },
    ],
    steps: [
      {
        n: '01',
        title: 'Run',
        body: 'Watches the packages the app imports and keeps the last good build running when a change fails.',
        api: 'nsgo dev',
      },
      {
        n: '02',
        title: 'Test',
        body: 'Runs the same tests headless or in a window, with native input and screenshots.',
        api: 'nsgo test',
      },
      {
        n: '03',
        title: 'Ship',
        body: 'Writes a signed .app, .ipa or .apk, Windows and Linux packages, or a static site. -embed builds a framework or AAR.',
        api: 'nsgo build',
      },
    ],
  },
  open: {
    kicker: 'Open source',
    title: 'Early, open and readable.',
    body: 'NSGo is an early Go port of Native Shell’s design. The engine, its tests and the gallery share one repository; ARCHITECTURE.md records every departure from NSH.',
    attribution: 'Based on the design of Native Shell by Longbridge.',
    links: [
      { label: 'Architecture', href: links.architecture },
      { label: 'Roadmap', href: links.roadmap },
      { label: 'Changelog', href: links.changelog },
    ],
    ctaTitle: 'Build something that feels native.',
    ctaBody: 'Start with the live gallery, then read the architecture or clone the repository.',
    view: 'View the gallery',
    github: 'Star on GitHub',
    note: 'Apache-2.0 · no stable API yet',
  },
  footer: {
    tagline: 'Native UI, written in Go.',
    columns: [
      {
        title: 'Project',
        links: [
          { label: 'Gallery', href: '#gallery' },
          { label: 'Architecture', href: links.architecture },
          { label: 'Roadmap', href: links.roadmap },
          { label: 'Changelog', href: links.changelog },
        ],
      },
      {
        title: 'Community',
        links: [
          { label: 'GitHub', href: links.repo },
          { label: 'Issues', href: links.issues },
          { label: 'Blog', href: links.blog },
          { label: 'X @oneo666', href: links.x },
        ],
      },
      {
        title: 'License',
        links: [
          { label: 'Apache-2.0', href: links.license },
          { label: 'NOTICE', href: links.notice },
          { label: 'Native Shell', href: links.nsh },
        ],
      },
    ],
    bottom: '© 2026 ONEO. NSGo is a port of Native Shell and is not affiliated with or endorsed by Longbridge.',
    top: 'Back to top',
  },
  ui: {
    copy: 'Copy',
    copied: 'Copied',
    copyCommand: 'Copy command',
  },
};

export type Copy = typeof en;

const zh: Copy = {
  meta: {
    title: 'NSGo — 用 Go 编写的原生界面',
    description:
      'NSGo 是一个用 Go 编写的跨平台原生 UI 引擎。响应式状态、布局和文本在 Go 中运行，iOS、Android、macOS、Windows、Linux 和 Web 用各自的原生视图绘制。',
  },
  nav: {
    skip: '跳到正文',
    primary: '主导航',
    links: [
      { label: 'Gallery', href: '#gallery' },
      { label: '引擎', href: '#engine' },
      { label: '平台', href: '#platforms' },
      { label: '代码', href: '#code' },
      { label: '工具链', href: '#toolchain' },
      { label: '文档', href: links.docs },
    ],
    github: 'GitHub',
    menu: '打开菜单',
    close: '关闭菜单',
    language: '切换到英文',
    languageShort: 'EN',
    theme: {
      label: '外观',
      system: '跟随系统',
      light: '浅色',
      dark: '深色',
    },
  },
  hero: {
    eyebrow: '开源 · Apache-2.0',
    titleTop: '原生界面，',
    titleBefore: '用 ',
    titleAccent: 'Go',
    titleAfter: ' 编写',
    lead: 'NSGo 是一个跨平台原生 UI 引擎。响应式状态、布局和文本在 Go 中运行，每个平台用自己的视图绘制。',
    points: [
      { title: '一份代码', body: 'iOS、Android、macOS、Windows、Linux 和 Web。' },
      { title: '真正的原生', body: '平台自己的视图、文本和滚动；Web 上则是 DOM。' },
      { title: '全程 Go', body: 'signal、flexbox 与 grid、动画和测试，都在一门语言里。' },
    ],
    source: '在 GitHub 上查看',
    architecture: '阅读架构',
    runsOn: '运行于',
    commandLabel: '从仓库开始',
    command: 'go tool nsgo dev examples/gallery',
    commandCopy: '复制命令',
  },
  gallery: {
    title: 'gallery.wasm',
    wasm: 'WASM',
    idleKicker: 'WebAssembly Gallery',
    idleTitle: '整个引擎，就在这个浏览器里',
    idleBody: 'Gallery 是一个真实的 NSGo 应用：每个元素、组件和服务，都由 Go 编译为 WebAssembly。',
    idleMeta: (download, engine) =>
      [download ? `下载 ${download}` : null, engine ? `${engine} 引擎` : null, '本地编译']
        .filter(Boolean)
        .join(' · '),
    start: '载入 Gallery',
    startNote: '应用会跟随网站的外观。不会上传任何内容。',
    phases: {
      prepare: '准备 WebAssembly',
      download: '下载引擎',
      decompress: '解压引擎',
      compile: '编译 WebAssembly',
      start: '启动 Gallery',
      ready: '就绪',
    },
    steps: ['下载', '编译', '运行'],
    stepsLabel: '加载步骤',
    loadingDetail: (loaded, total) => `${loaded} / ${total}`,
    loadingNote: '引擎在你的浏览器里编译，不会上传任何内容。',
    cancel: '取消',
    errorTitle: 'Gallery 无法启动',
    errorBody: '请检查网络连接后重试。',
    retry: '重试',
    ready: '运行中',
    caption: '真实的 Gallery，由 Go 编译为 WebAssembly。',
    open: '全屏打开',
  },
  engine: {
    kicker: '引擎',
    title: '原生，意味着由平台来绘制',
    lead: 'NSGo 拥有应用模型，而不是像素。每次提交产生一批挂载项，由薄薄的后端应用到平台自己的视图上。',
    flow: [
      { label: '01 · Go', title: '响应式模型', body: 'signal · 布局 · 文本' },
      { label: 'MountBatch', title: '一次提交', body: '只发送变化的部分' },
      { label: '平台', title: '原生视图', body: 'AppKit · Android · DOM' },
    ],
    flowNote: '样式、文本和布局在 Go 中解析，动画运行在平台的帧时钟上。',
    layers: [
      {
        n: '01',
        title: 'Go 拥有模型',
        body: '组件只运行一次。signal 更新读取它的地方，region 只重建变化的部分，布局计算出差异。',
      },
      {
        n: '02',
        title: '每次提交一批',
        body: '样式、文本、布局和挂载项一次跨到主线程。过渡和动画值无需每帧提交。',
      },
      {
        n: '03',
        title: '平台负责绘制',
        body: 'AppKit、Android View、Direct2D、GTK 4 或 DOM；CoreText、DirectWrite、Pango 或浏览器自己的文本。',
      },
    ],
  },
  platforms: {
    kicker: '平台',
    title: '六个平台，一套工具链',
    lead: '每个目标都能在装有 Go 的 macOS 上构建。iOS 额外需要 Xcode；x86_64 Android 需要 NDK。',
    status: '可运行',
    items: [
      {
        id: 'ios',
        name: 'iOS',
        stack: 'UIKit · CoreText',
        body: '用 Xcode 链接，在设备和模拟器上运行。',
        command: 'ios',
      },
      {
        id: 'android',
        name: 'Android',
        stack: 'Android View · StaticLayout',
        body: 'Go 进程运行在预构建的 Java 运行时之后。',
        command: 'android_arm64',
      },
      {
        id: 'macos',
        name: 'macOS',
        stack: 'AppKit · CoreText',
        body: '通过 purego 访问系统，无需 cgo。',
        command: 'macos_arm64',
      },
      {
        id: 'windows',
        name: 'Windows',
        stack: 'Win32 · Direct2D · DirectWrite',
        body: '通过 syscall 使用 COM 和窗口系统。',
        command: 'windows_x64',
      },
      {
        id: 'linux',
        name: 'Linux',
        stack: 'GTK 4 · Pango · Cairo',
        body: '用 purego 加载，需要 GTK 4.14 或更新版本。',
        command: 'linux_x64',
      },
      {
        id: 'web',
        name: 'Web',
        stack: 'HTML · CSS · WebAssembly',
        body: 'Web 后端把应用映射为 DOM 元素，就像上面的 Gallery。',
        command: 'web',
      },
    ],
    footnote: '同一份代码也能嵌入 iOS、Android 和 Go 桌面应用。',
    footnoteLink: '嵌入方式',
  },
  code: {
    kicker: 'API',
    title: '组件就是一个 Go 值',
    lead: '元素是结构体，signal 是函数。没有模板语言，也没有横在你和引擎之间的运行时层。',
    aside: [
      {
        title: '没有模板语言',
        body: '组件是带 Render 方法的 Go 类型，元素是普通的结构体字面量。',
      },
      {
        title: '没有额外的绑定',
        body: '一条 go get -tool 安装 nsgo；go tool nsgo 在 Go 能运行的地方运行。',
      },
      {
        title: '像用户一样测试',
        body: 'nsgotest 按文本找到控件并按下它，可在无窗口或原生窗口中运行。',
      },
    ],
    tabs: [{ file: 'main.go' }, { file: 'app_test.go' }],
    testNote: '同一个测试，可以在无窗口环境运行，也可以在原生窗口中运行。',
    more: '快速开始',
  },
  toolchain: {
    kicker: '工具链',
    title: '一个工具，从源码到商店',
    lead: 'nsgo 调用 Go 和 Xcode，为每个目标打包。一条命令覆盖整个循环。',
    terminalTitle: 'shell',
    commands: [
      { command: 'go tool nsgo dev examples/gallery', note: '热重载运行' },
      { command: 'go tool nsgo test -target macos', note: '原生输入与截图' },
      { command: 'go tool nsgo build -target all examples/gallery', note: '每个目标一个签名包' },
    ],
    steps: [
      {
        n: '01',
        title: '运行',
        body: '监听应用导入的包，构建失败时保留上一个可用版本继续运行。',
        api: 'nsgo dev',
      },
      {
        n: '02',
        title: '测试',
        body: '无窗口或原生窗口运行同一组测试，支持原生输入和截图。',
        api: 'nsgo test',
      },
      {
        n: '03',
        title: '发布',
        body: '写出签名的 .app、.ipa、.apk，Windows 和 Linux 的包目录，或一个静态网站；-embed 打包框架或 AAR。',
        api: 'nsgo build',
      },
    ],
  },
  open: {
    kicker: '开源',
    title: '早期，开放，可阅读',
    body: 'NSGo 是 Native Shell 设计的早期 Go 移植。引擎、测试和 Gallery 在同一个仓库里；ARCHITECTURE.md 记录每一处对 NSH 的偏离。',
    attribution: '基于 Longbridge 的 Native Shell 设计。',
    links: [
      { label: '架构', href: links.architecture },
      { label: '路线图', href: links.roadmap },
      { label: '更新日志', href: links.changelog },
    ],
    ctaTitle: '做一个真正原生的应用',
    ctaBody: '从在线 Gallery 开始，然后阅读架构或克隆仓库。',
    view: '查看 Gallery',
    github: '在 GitHub 上关注',
    note: 'Apache-2.0 · 尚无稳定 API',
  },
  footer: {
    tagline: '用 Go 编写的原生界面',
    columns: [
      {
        title: '项目',
        links: [
          { label: 'Gallery', href: '#gallery' },
          { label: '架构', href: links.architecture },
          { label: '路线图', href: links.roadmap },
          { label: '更新日志', href: links.changelog },
        ],
      },
      {
        title: '社区',
        links: [
          { label: 'GitHub', href: links.repo },
          { label: 'Issues', href: links.issues },
          { label: '博客', href: links.blog },
          { label: 'X @oneo666', href: links.x },
        ],
      },
      {
        title: '许可',
        links: [
          { label: 'Apache-2.0', href: links.license },
          { label: 'NOTICE', href: links.notice },
          { label: 'Native Shell', href: links.nsh },
        ],
      },
    ],
    bottom: '© 2026 ONEO。NSGo 是 Native Shell 的移植，与 Longbridge 无隶属关系，也未获其背书。',
    top: '回到顶部',
  },
  ui: {
    copy: '复制',
    copied: '已复制',
    copyCommand: '复制命令',
  },
};

export const copies: Record<Lang, Copy> = { en, zh };
