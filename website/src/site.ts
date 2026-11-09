import type { IconName } from './components/Icon.astro';

// The site's pages, languages and outside links. A page marked `dev` is a
// placeholder: it is built and linked only by the dev server.

export type Lang = 'en' | 'zh';
export const langs: Lang[] = ['en', 'zh'];

export type PageId = 'home' | 'docs' | 'showcases' | 'benchmarks';

export const pages: { id: PageId; path: string; dev?: boolean }[] = [
  { id: 'home', path: '' },
  { id: 'docs', path: 'docs/' },
  { id: 'showcases', path: 'showcases/', dev: true },
  { id: 'benchmarks', path: 'benchmarks/', dev: true },
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

const repo = 'https://github.com/oneo-me/nsgo';

export const links = {
  repo,
  issues: `${repo}/issues`,
  architecture: `${repo}/blob/main/ARCHITECTURE.md`,
  roadmap: `${repo}/blob/main/docs/roadmap.md`,
  changelog: `${repo}/blob/main/CHANGELOG.md`,
  embedding: `${repo}/blob/main/docs/architecture/embedding.md`,
  license: `${repo}/blob/main/LICENSE`,
  notice: `${repo}/blob/main/NOTICE`,
  /** The gallery's web build, opened full page in a new tab. */
  gallery: `${import.meta.env.BASE_URL}gallery/loader.html`,
  blog: 'https://oneo.me',
  x: 'https://x.com/oneo666',
  nsh: 'https://github.com/longbridge/nsh',
};

export const install = 'go get -tool github.com/oneo-me/nsgo/cmd/nsgo';

export const platforms: { name: string; icon: IconName; stack: string; target: string }[] = [
  { name: 'iOS', icon: 'smartphone', stack: 'UIKit · CoreText', target: 'ios' },
  { name: 'tvOS', icon: 'tv', stack: 'UIKit · CoreText', target: 'tvos' },
  { name: 'watchOS', icon: 'watch', stack: 'SwiftUI · CoreText', target: 'watchos' },
  { name: 'Android', icon: 'android', stack: 'Android Views', target: 'android_arm64' },
  { name: 'macOS', icon: 'laptop', stack: 'AppKit · CoreText', target: 'macos_arm64' },
  { name: 'Windows', icon: 'windows', stack: 'Direct2D · DirectWrite', target: 'windows_x64' },
  { name: 'Linux', icon: 'terminal', stack: 'GTK 4 · Pango', target: 'linux_x64' },
  { name: 'Web', icon: 'globe', stack: 'WebAssembly · DOM', target: 'web' },
  { name: 'Mini programs', icon: 'message', stack: 'WeChat · QQ · Alipay · Douyin', target: 'miniprogram_wechat' },
];
