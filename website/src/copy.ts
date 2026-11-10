import type { Lang, PageId } from './site';

// The site's words. Add English and Chinese together; Chinese headings,
// titles and kickers omit the trailing 。, body sentences keep it.

const en = {
  nav: {
    skip: 'Skip to content',
    primary: 'Primary',
    pages: {
      home: 'Home',
      docs: 'Docs',
      components: 'Components',
      showcases: 'Showcases',
      benchmarks: 'Benchmarks',
    } satisfies Record<PageId, string>,
    demo: 'Live demo',
    github: 'GitHub',
    menu: 'Open menu',
    close: 'Close menu',
    language: 'Language',
    theme: { label: 'Appearance', light: 'Light', dark: 'Dark', system: 'System' },
  },
  home: {
    title: 'NSGo — Write it once in Go, native on every platform',
    description:
      'NSGo is a cross-platform native UI framework. Write an app once in Go or TSX and run it on iOS and its App Clips, tvOS, watchOS, Android, macOS, Windows, Linux and the web with each platform’s own views, and Go apps in WeChat, QQ, Alipay and Douyin mini programs.',
    hero: {
      badge: 'Open source · Early preview',
      title: ['Write it once in Go.', 'Native on every platform.'],
      // Around the number of platforms.
      lead: ['NSGo is a cross-platform UI framework. Your app’s state, layout and styles run in Go, and ', ' platforms draw it with their own native views.'],
      demo: 'Try the live demo',
      start: 'Build your first app',
      runsOn: 'Runs on',
      kinds: { app: 'Apps', web: 'Web', mini: 'Mini programs' },
      // A platform's line under the row: Android draws with Android Views.
      drawsWith: [' draws with ', ''],
      desktopAlt: 'The NSGo gallery’s Stocks demo in its desktop layout, on the web backend',
      phoneAlt: 'The NSGo gallery’s components in its phone layout, on the web backend',
    },
    how: {
      eyebrow: 'How it works',
      title: 'Go owns the app. The platform draws it.',
      lead: 'No embedded browser and no custom renderer sit between your app and the screen: each update crosses to the main thread once, as one batch of only what changed.',
      nodes: [
        { label: 'Your app', title: 'Go or TSX', items: ['Components', 'Signals', 'Styles and Tailwind'] },
        { label: 'NSGo engine', title: 'In Go', items: ['Reactive graph', 'Flexbox and grid', 'Text preparation'] },
        { label: 'The platform', title: 'Native views', items: ['AppKit · UIKit', 'Android Views', 'Direct2D · GTK · DOM'] },
      ],
      links: ['renders', 'one batch per commit'],
    },
    features: {
      eyebrow: 'What you get',
      title: 'Everything an app needs, in one language.',
      native: {
        title: 'Truly native',
        body: 'Each platform draws with its own views, text, scrolling and accessibility.',
      },
      reactive: {
        title: 'A reactive engine',
        body: 'Signals update only what reads them. Animation runs on the platform’s frame clock.',
      },
      layout: {
        title: 'Layout and styles you know',
        body: 'Flexbox and grid, CSS-like styles, Tailwind classes, light and dark themes, right-to-left languages.',
      },
      components: {
        title: 'Components included',
        body: 'Unstyled in base, ready to use in ui: from buttons and menus to whole tools.',
        chips: ['DataTable', 'Charts', 'Code editor', 'Rich text', 'Terminal', 'Dock', 'Command palette', 'Date picker', 'Color picker', 'Toasts', 'Sheets', 'Menus'],
      },
      tsx: {
        title: 'Go or TSX',
        body: 'Components as Go values, or TSX with Tailwind classes, compiled ahead with no Node.',
      },
      tests: {
        title: 'Tests that act like users',
        body: 'Find a control by its text and press it, headless or in a real window.',
      },
      embed: {
        title: 'Embeds anywhere',
        body: 'Put an NSGo view inside an iOS, Android, desktop or web app.',
      },
    },
    code: {
      eyebrow: 'The API',
      title: 'A component is just Go.',
      lead: 'Elements are structs, state is a signal, and a test presses the button the way a user would.',
      points: [
        { title: 'No template language', body: 'A component is a function that returns elements.' },
        { title: 'Fine-grained updates', body: 'Pressing changes the one text that reads the count.' },
        { title: 'The same test everywhere', body: 'Headless in CI, or in a native window with screenshots.' },
      ],
      result: 'Result',
    },
    workflow: {
      eyebrow: 'Workflow',
      title: 'One tool, from first line to store.',
      lead: 'nsgo creates, runs, tests and packages apps. It installs with Go and runs wherever Go does.',
      steps: [
        { title: 'Create', body: 'A Go, TSX or mixed app.', command: 'go tool nsgo init' },
        { title: 'Run', body: 'Hot reload on every save.', command: 'go tool nsgo dev' },
        { title: 'Test', body: 'Headless or in a native window.', command: 'go tool nsgo test' },
        { title: 'Ship', body: 'Signed packages for each target.', command: 'go tool nsgo build' },
      ],
    },
    next: { eyebrow: 'Next steps', title: 'Keep exploring.' },
  },
  guides: {
    demo: { title: 'Live demo', body: 'The gallery of every component, full page in your browser.' },
    docs: { title: 'Documentation', body: 'Tutorials, the toolchain, components and core concepts for building apps with NSGo.' },
    components: { title: 'Components', body: 'Icon packs, controls and whole views from NSGo and the community.' },
    showcases: { title: 'Showcases', body: 'Apps people build with NSGo.' },
    benchmarks: { title: 'Benchmarks', body: 'How fast NSGo starts, lays out and updates.' },
    architecture: { title: 'Architecture', body: 'How the engine, backends and tool fit together.' },
    github: { title: 'GitHub', body: 'The source, issues and discussions.' },
  },
  docs: {
    title: 'Docs',
    description: 'Guides for building apps with NSGo.',
    pages: 'Pages',
    onThisPage: 'On this page',
    previous: 'Previous',
    next: 'Next',
    edit: 'Edit this page on GitHub',
    fallback: 'This page is not translated yet, so it shows the English one.',
    live: { loading: 'Loading the preview…', failed: 'The preview could not start.' },
  },
  components: {
    title: 'Components — NSGo',
    description: 'Component libraries for NSGo: icon packs, controls and whole views from NSGo and the community.',
    heading: 'Components beyond the core.',
    lead: 'Libraries from NSGo and the community that go beyond the built-in controls, from icon packs to whole views.',
    official: 'Official',
    visit: 'Open',
    source: 'Source',
    by: 'By',
    submit: {
      title: 'Built a component for NSGo?',
      body: 'Add a Markdown document to docs/public/components with a sentence, its import path and a screenshot, then open a pull request.',
      action: 'Add your component',
    },
  },
  showcases: {
    title: 'Showcases — NSGo',
    description: 'Apps and demos built with NSGo, and what they run on.',
    heading: 'Built with NSGo.',
    lead: 'Sample apps to try and apps people make with NSGo, on every platform it runs on.',
    platforms: 'Platforms',
    visit: 'Open',
    source: 'Source',
    by: 'By',
    submit: {
      title: 'Made something with NSGo?',
      body: 'Add a Markdown document to docs/public/cases with a sentence and a screenshot, then open a pull request.',
      action: 'Add your showcase',
    },
  },
  placeholder: {
    badge: 'Dev only',
    note: 'This page is a placeholder and is not published yet.',
    back: 'Back home',
    pages: {
      benchmarks: {
        title: 'Benchmarks — NSGo',
        heading: 'Benchmarks',
        lead: 'Start-up, layout, update and memory measured on each platform.',
      },
    },
  },
  cta: {
    title: 'Start building with NSGo.',
    body: 'Install the tool into your module, then create your first app.',
    primary: 'Get started',
    secondary: 'Try the live demo',
  },
  footer: {
    tagline: 'Native apps for every platform, written in Go.',
    columns: {
      product: 'Product',
      resources: 'Resources',
      community: 'Community',
      legal: 'Legal',
    },
    resources: { architecture: 'Architecture', roadmap: 'Roadmap', changelog: 'Changelog', embedding: 'Embedding' },
    community: { github: 'GitHub', issues: 'Issues', blog: 'Blog', x: 'X @oneo666' },
    legal: { license: 'Apache-2.0', notice: 'NOTICE', nsh: 'Native Shell' },
    bottom: '© 2026 ONEO. NSGo ports the design of Native Shell and is not affiliated with or endorsed by Longbridge.',
    top: 'Back to top',
  },
  notFound: {
    title: 'Page not found — NSGo',
    heading: 'This page does not exist.',
    body: 'It may have moved, or it has not been written yet.',
    home: 'Back home',
  },
  ui: { copy: 'Copy', copied: 'Copied' },
};

export type Copy = typeof en;

const zh: Copy = {
  nav: {
    skip: '跳到正文',
    primary: '主导航',
    pages: {
      home: '首页',
      docs: '文档',
      components: '组件',
      showcases: '案例',
      benchmarks: '性能',
    },
    demo: '在线体验',
    github: 'GitHub',
    menu: '打开菜单',
    close: '关闭菜单',
    language: '语言',
    theme: { label: '外观', light: '浅色', dark: '深色', system: '跟随系统' },
  },
  home: {
    title: 'NSGo — 用 Go 写一次，在每个平台原生运行',
    description:
      'NSGo 是一个跨平台原生 UI 框架。用 Go 或 TSX 写一次应用，就能以各平台自己的视图运行在 iOS 及其轻 App（App Clip）、tvOS、watchOS、Android、macOS、Windows、Linux、Web 上，Go 应用还能运行在微信、QQ、支付宝和抖音小程序中。',
    hero: {
      badge: '开源 · 早期预览',
      title: ['用 Go 写一次', '在每个平台原生运行'],
      lead: ['NSGo 是一个跨平台 UI 框架。应用的状态、布局和样式在 Go 中运行，再由 ', ' 个平台用各自的原生视图绘制。'],
      demo: '在线体验',
      start: '开始写第一个应用',
      runsOn: '运行于',
      kinds: { app: '客户端', web: '网页', mini: '小程序' },
      drawsWith: [' 上由 ', ' 绘制'],
      desktopAlt: 'NSGo Gallery 的股票演示，桌面布局，运行在 Web 后端',
      phoneAlt: 'NSGo Gallery 的组件列表，手机布局，运行在 Web 后端',
    },
    how: {
      eyebrow: '工作原理',
      title: 'Go 掌管应用，平台负责绘制',
      lead: '应用和屏幕之间没有内嵌浏览器，也没有自绘渲染器：每次更新只跨到主线程一次，一批只带变化的部分。',
      nodes: [
        { label: '你的应用', title: 'Go 或 TSX', items: ['组件', 'signal', '样式与 Tailwind'] },
        { label: 'NSGo 引擎', title: '运行在 Go 中', items: ['响应式图', 'flexbox 与 grid', '文本准备'] },
        { label: '平台', title: '原生视图', items: ['AppKit · UIKit', 'Android View', 'Direct2D · GTK · DOM'] },
      ],
      links: ['渲染', '每次提交一批'],
    },
    features: {
      eyebrow: '你能得到',
      title: '应用所需的一切，都在一门语言里',
      native: {
        title: '真正的原生',
        body: '每个平台用自己的视图、文本、滚动和无障碍来绘制。',
      },
      reactive: {
        title: '响应式引擎',
        body: 'signal 只更新读取它的地方，动画运行在平台的帧时钟上。',
      },
      layout: {
        title: '熟悉的布局与样式',
        body: 'flexbox 与 grid、类 CSS 样式、Tailwind 类名、浅色与深色主题、从右到左的语言。',
      },
      components: {
        title: '内置组件',
        body: 'base 里是无样式的，ui 里是开箱即用的：从按钮、菜单到完整的工具。',
        chips: ['DataTable', '图表', '代码编辑器', '富文本', '终端', 'Dock', '命令面板', '日期选择', '颜色选择', 'Toast', 'Sheet', '菜单'],
      },
      tsx: {
        title: 'Go 或 TSX',
        body: '用 Go 值写组件，或用 TSX 配合 Tailwind 类名，提前编译，不需要 Node。',
      },
      tests: {
        title: '像用户一样测试',
        body: '按文本找到控件并按下它，无窗口或在真实窗口中运行。',
      },
      embed: {
        title: '随处嵌入',
        body: '把 NSGo 视图放进 iOS、Android、桌面或 Web 应用。',
      },
    },
    code: {
      eyebrow: 'API',
      title: '组件就是 Go',
      lead: '元素是结构体，状态是 signal，测试像用户一样按下按钮。',
      points: [
        { title: '没有模板语言', body: '组件就是返回元素的函数。' },
        { title: '细粒度更新', body: '按下按钮，只有读取计数的那段文本会变。' },
        { title: '同一个测试，到处运行', body: '在 CI 中无窗口运行，或在原生窗口中运行并截图。' },
      ],
      result: '运行效果',
    },
    workflow: {
      eyebrow: '工作流',
      title: '一个工具，从第一行代码到上架',
      lead: 'nsgo 负责创建、运行、测试和打包应用。它随 Go 安装，在 Go 能运行的地方都能运行。',
      steps: [
        { title: '创建', body: 'Go、TSX 或混合应用。', command: 'go tool nsgo init' },
        { title: '运行', body: '每次保存都热重载。', command: 'go tool nsgo dev' },
        { title: '测试', body: '无窗口或原生窗口中运行。', command: 'go tool nsgo test' },
        { title: '发布', body: '为每个目标打出签名的包。', command: 'go tool nsgo build' },
      ],
    },
    next: { eyebrow: '下一步', title: '继续探索' },
  },
  guides: {
    demo: { title: '在线体验', body: '包含全部组件的 Gallery，在浏览器中整页运行。' },
    docs: { title: '文档', body: '用 NSGo 开发应用的教程、工具链、组件和核心概念。' },
    components: { title: '组件', body: '来自 NSGo 官方和社区的图标包、控件和完整视图。' },
    showcases: { title: '案例', body: '大家用 NSGo 做的应用。' },
    benchmarks: { title: '性能', body: 'NSGo 的启动、布局和更新有多快。' },
    architecture: { title: '架构', body: '引擎、后端和工具如何配合。' },
    github: { title: 'GitHub', body: '源码、问题和讨论。' },
  },
  docs: {
    title: '文档',
    description: '用 NSGo 构建应用的指南。',
    pages: '目录',
    onThisPage: '本页内容',
    previous: '上一页',
    next: '下一页',
    edit: '在 GitHub 上编辑此页',
    fallback: '这一页还没有中文版，下面显示的是英文原文。',
    live: { loading: '正在加载预览…', failed: '预览无法启动。' },
  },
  components: {
    title: '组件 — NSGo',
    description: 'NSGo 的组件库：来自 NSGo 官方和社区的图标包、控件和完整视图。',
    heading: '核心之外的组件',
    lead: '来自 NSGo 官方和社区的组件库，从图标包到完整视图，补上内置组件之外的能力。',
    official: '官方',
    visit: '打开',
    source: '源码',
    by: '作者',
    submit: {
      title: '为 NSGo 写了组件？',
      body: '在 docs/public/components 里加一个 Markdown 文档，写一句话、包路径并附上截图，然后提交 pull request。',
      action: '添加你的组件',
    },
  },
  showcases: {
    title: '案例 — NSGo',
    description: '用 NSGo 构建的应用与示例，以及它们运行的平台。',
    heading: '用 NSGo 构建',
    lead: '可以马上试试的示例，以及大家用 NSGo 做出的应用，运行在它支持的每个平台上。',
    platforms: '平台',
    visit: '打开',
    source: '源码',
    by: '作者',
    submit: {
      title: '用 NSGo 做了东西？',
      body: '在 docs/public/cases 里加一个 Markdown 文档，写一句话、附上截图，然后提交 pull request。',
      action: '添加你的案例',
    },
  },
  placeholder: {
    badge: '仅开发可见',
    note: '这是一个占位页面，尚未发布。',
    back: '返回首页',
    pages: {
      benchmarks: {
        title: '性能 — NSGo',
        heading: '性能',
        lead: '在每个平台上测量启动、布局、更新和内存。',
      },
    },
  },
  cta: {
    title: '开始用 NSGo 构建',
    body: '把工具安装到你的模块里，然后创建第一个应用。',
    primary: '快速开始',
    secondary: '在线体验',
  },
  footer: {
    tagline: '用 Go 编写，在每个平台原生运行。',
    columns: {
      product: '产品',
      resources: '资源',
      community: '社区',
      legal: '许可',
    },
    resources: { architecture: '架构', roadmap: '路线图', changelog: '更新日志', embedding: '嵌入' },
    community: { github: 'GitHub', issues: 'Issues', blog: '博客', x: 'X @oneo666' },
    legal: { license: 'Apache-2.0', notice: 'NOTICE', nsh: 'Native Shell' },
    bottom: '© 2026 ONEO。NSGo 移植自 Native Shell 的设计，与 Longbridge 无隶属关系，也未获其背书。',
    top: '回到顶部',
  },
  notFound: {
    title: '页面不存在 — NSGo',
    heading: '这个页面不存在',
    body: '它可能已经移动，或者还没有写好。',
    home: '返回首页',
  },
  ui: { copy: '复制', copied: '已复制' },
};

export const copies: Record<Lang, Copy> = { en, zh };
