import type { IconName } from './components/Icon.astro';

// The site's pages, languages and outside links. A page marked `dev` is a
// placeholder: it is built and linked only by the dev server.

export type Lang = 'en' | 'zh';
export const langs: Lang[] = ['en', 'zh'];

export type PageId = 'home' | 'docs' | 'components' | 'showcases' | 'benchmarks';

export const pages: { id: PageId; path: string; dev?: boolean }[] = [
  { id: 'home', path: '' },
  { id: 'docs', path: 'docs/' },
  { id: 'components', path: 'components/' },
  { id: 'showcases', path: 'showcases/' },
  { id: 'benchmarks', path: 'benchmarks/' },
];

export function shown(id: PageId): boolean {
  const page = pages.find((page) => page.id === id);
  return page !== undefined && (!page.dev || import.meta.env.DEV);
}

/** The pages the header lists after Home. */
export const navPages = pages.filter((page) => page.id !== 'home' && shown(page.id));

/** English lives at the base, other languages under their code. */
export function localePath(lang: Lang, path = ''): string {
  return `${import.meta.env.BASE_URL}${lang === 'en' ? '' : `${lang}/`}${path}`;
}

export function pageUrl(lang: Lang, id: PageId): string {
  return localePath(lang, pages.find((page) => page.id === id)!.path);
}

/** A docs page by its file's path below the language, `index` for the first. */
export function docUrl(lang: Lang, id: string): string {
  return localePath(lang, id === 'index' ? 'docs/' : `docs/${id}/`);
}

/** getStaticPaths for a page under src/pages/[...lang]/. */
export function langPaths(id: PageId) {
  if (!shown(id)) return [];
  return langs.map((lang) => ({ params: { lang: lang === 'en' ? undefined : lang } }));
}

export function langOf(param: string | undefined): Lang {
  return param === 'zh' ? 'zh' : 'en';
}

const repo = 'https://github.com/nsgo-dev/nsgo';

export const links = {
  repo,
  issues: `${repo}/issues`,
  architecture: `${repo}/blob/main/ARCHITECTURE.md`,
  roadmap: `${repo}/blob/main/docs/roadmap.md`,
  changelog: `${repo}/blob/main/CHANGELOG.md`,
  license: `${repo}/blob/main/LICENSE`,
  notice: `${repo}/blob/main/NOTICE`,
  /** The gallery's web build, opened full page in a new tab. */
  gallery: `${import.meta.env.BASE_URL}gallery/`,
  /** The folder a contributor adds a case to. */
  cases: `${repo}/tree/main/docs/public/cases`,
  /** The folder a contributor adds a component to. */
  components: `${repo}/tree/main/docs/public/components`,
  /** The Benchmarks page's apps and runner. */
  benchmark: `${repo}/tree/main/benchmark`,
  /** The community's catalog of apps, packages and guides around NSGo. */
  awesome: 'https://awesomensgo.dev',
  blog: 'https://oneo.me',
  x: 'https://x.com/oneo666',
  nsh: 'https://github.com/longbridge/nsh',
};

export const install = 'go get -tool github.com/nsgo-dev/nsgo/cmd/nsgo';

/**
 * The platforms NSGo runs on, each shown on its own, with its kind and what
 * draws it; zh names one in Chinese where its users call it otherwise, and
 * experimental marks one not ready for most apps.
 */
export type PlatformKind = 'app' | 'web' | 'mini';

export const platforms: {
  name: string;
  zh?: string;
  kind: PlatformKind;
  icon: IconName;
  stack: string;
  experimental?: boolean;
}[] = [
  { name: 'iOS', kind: 'app', icon: 'smartphone', stack: 'UIKit · CoreText' },
  { name: 'App Clips', zh: '轻 App', kind: 'app', icon: 'scan', stack: 'UIKit · CoreText' },
  { name: 'tvOS', kind: 'app', icon: 'tv', stack: 'UIKit · CoreText' },
  { name: 'watchOS', kind: 'app', icon: 'watch', stack: 'SwiftUI · CoreText' },
  { name: 'macOS', kind: 'app', icon: 'laptop', stack: 'AppKit · CoreText' },
  { name: 'Android', kind: 'app', icon: 'android', stack: 'Android Views' },
  { name: 'Windows', kind: 'app', icon: 'windows', stack: 'Direct2D · DirectWrite' },
  { name: 'Linux', kind: 'app', icon: 'terminal', stack: 'GTK 4 · Pango' },
  { name: 'Web', kind: 'web', icon: 'globe', stack: 'WebAssembly · DOM' },
  { name: 'WeChat', zh: '微信', kind: 'mini', icon: 'chats', stack: 'WebAssembly · WXML', experimental: true },
  { name: 'QQ', kind: 'mini', icon: 'bird', stack: 'WebAssembly · QML', experimental: true },
  { name: 'Alipay', zh: '支付宝', kind: 'mini', icon: 'wallet', stack: 'WebAssembly · AXML', experimental: true },
  { name: 'Douyin', zh: '抖音', kind: 'mini', icon: 'music', stack: 'WebAssembly · TTML', experimental: true },
];

/** platformName is how lang names a platform. */
export const platformName = (platform: (typeof platforms)[number], lang: string) =>
  (lang === 'zh' && platform.zh) || platform.name;

/**
 * platformRows are the rows the home page shows platforms in, by kind: apps
 * on their own, then the web and mini programs.
 */
export const platformRows: PlatformKind[][] = [['app'], ['web', 'mini']];

/** platformsOf are the platforms of a kind, with their place among all of them. */
export const platformsOf = (kind: PlatformKind) =>
  platforms.flatMap((platform, index) => (platform.kind === kind ? [{ platform, index }] : []));
