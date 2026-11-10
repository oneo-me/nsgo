// The Benchmarks page's data: the results benchmark/runner writes, one
// file a desktop in benchmark/results/, and what the frameworks offer,
// which the page compares in a table.

import type { Lang } from '../site';

export type FrameworkId = 'nsgo' | 'mygo' | 'nsh' | 'gpui' | 'gpui-fast';

/**
 * The frameworks, each in a color of its own across the page's charts:
 * NSGo in the mark's blue, the others in quiet tints that tell them apart.
 */
export const frameworks: { id: FrameworkId; name: string; url: string; color: string }[] = [
  { id: 'nsgo', name: 'NSGo', url: 'https://github.com/oneo-me/nsgo', color: '#007aff' },
  { id: 'mygo', name: 'MyGo', url: 'https://github.com/egoist/mygo', color: '#d4a5b4' },
  { id: 'nsh', name: 'NSH', url: 'https://github.com/longbridge/nsh', color: '#9fc4af' },
  { id: 'gpui', name: 'GPUI Kit', url: 'https://github.com/longbridge/gpui-kit', color: '#b5acd4' },
  { id: 'gpui-fast', name: 'GPUI Fast', url: 'https://github.com/longbridge/gpui-fast', color: '#d8c39c' },
];

export type CaseId = 'counter' | 'quotes' | 'list';
export const caseIds: CaseId[] = ['counter', 'quotes', 'list'];

/** A program's medians, as benchmark/runner writes them. */
export interface Measure {
  startup: number;
  memory: number;
  cpu: number;
  compositor?: number;
  fps?: number;
  error?: string;
}

/** benchmark/results/<os>.json. */
export interface Results {
  date: string;
  machine: { os: string; arch: string; cpu: string; memory: number };
  runs: number;
  seconds: number;
  versions: Partial<Record<FrameworkId, string>>;
  size: Partial<Record<FrameworkId, number>>;
  cases: Partial<Record<CaseId, Partial<Record<FrameworkId, Measure>>>>;
}

export type DesktopId = 'darwin' | 'windows' | 'linux';

export const desktops: { id: DesktopId; name: string }[] = [
  { id: 'darwin', name: 'macOS' },
  { id: 'windows', name: 'Windows' },
  { id: 'linux', name: 'Linux' },
];

const files = import.meta.glob('../../../benchmark/results/*.json', {
  eager: true,
  import: 'default',
}) as Record<string, Results>;

/** The results by desktop, for those measured. */
export const results: Partial<Record<DesktopId, Results>> = Object.fromEntries(
  Object.entries(files).map(([path, data]) => [path.split('/').pop()!.replace(/\.json$/, ''), data]),
);

/** A capability: yes, no, or yes with a word on how. */
export type Support = boolean | { en: string; zh: string };

export interface FeatureRow {
  en: string;
  zh: string;
  values: Record<FrameworkId, Support>;
}

const all = (v: Support): Record<FrameworkId, Support> => ({ nsgo: v, mygo: v, nsh: v, gpui: v, 'gpui-fast': v });
const only = (...ids: FrameworkId[]): Record<FrameworkId, Support> =>
  Object.fromEntries(frameworks.map((f) => [f.id, ids.includes(f.id)])) as Record<FrameworkId, Support>;

/**
 * What each framework offers: the languages an app is written in, what
 * draws it, and where it runs.
 */
export const features: { en: string; zh: string; rows: FeatureRow[] }[] = [
  {
    en: 'Building apps',
    zh: '开发',
    rows: [
      {
        en: 'Language',
        zh: '语言',
        values: {
          nsgo: { en: 'Go, TSX', zh: 'Go、TSX' },
          mygo: { en: 'Go', zh: 'Go' },
          nsh: { en: 'Rust, TSX', zh: 'Rust、TSX' },
          gpui: { en: 'Rust', zh: 'Rust' },
          'gpui-fast': { en: 'Rust', zh: 'Rust' },
        },
      },
      {
        en: 'Drawn with',
        zh: '绘制方式',
        values: {
          nsgo: { en: 'Native views', zh: '原生视图' },
          mygo: { en: 'Its own GPU renderer', zh: '自绘 GPU 渲染' },
          nsh: { en: 'Native views', zh: '原生视图' },
          gpui: { en: 'GPUI on the GPU', zh: 'GPUI GPU 渲染' },
          'gpui-fast': { en: 'GPUI on the GPU, retained', zh: 'GPUI GPU 渲染，增量保留' },
        },
      },
      { en: 'Hot reload', zh: '热重载', values: only('nsgo', 'mygo', 'nsh') },
      { en: 'Screens loaded at run time, signed', zh: '运行时加载已签名的界面', values: only('nsgo', 'nsh') },
    ],
  },
  {
    en: 'Desktop',
    zh: '桌面',
    rows: [
      { en: 'macOS', zh: 'macOS', values: all(true) },
      { en: 'Windows', zh: 'Windows', values: all(true) },
      { en: 'Linux', zh: 'Linux', values: all(true) },
    ],
  },
  {
    en: 'Beyond the desktop',
    zh: '桌面之外',
    rows: [
      { en: 'Web', zh: 'Web', values: all(true) },
      { en: 'iOS', zh: 'iOS', values: only('nsgo', 'nsh') },
      { en: 'Android', zh: 'Android', values: only('nsgo', 'nsh') },
      { en: 'HarmonyOS', zh: '鸿蒙', values: only('nsh') },
      { en: 'App Clips', zh: '轻 App', values: only('nsgo') },
      { en: 'tvOS', zh: 'tvOS', values: only('nsgo') },
      { en: 'watchOS', zh: 'watchOS', values: only('nsgo') },
      { en: 'Home screen widgets', zh: '桌面小组件', values: only('nsgo') },
    ],
  },
  {
    en: 'Mini programs',
    zh: '小程序',
    rows: [
      { en: 'WeChat', zh: '微信', values: only('nsgo', 'nsh') },
      { en: 'QQ', zh: 'QQ', values: only('nsgo') },
      { en: 'Alipay', zh: '支付宝', values: only('nsgo') },
      { en: 'Douyin', zh: '抖音', values: only('nsgo') },
    ],
  },
];

export function supportText(v: Support, lang: Lang): string | undefined {
  return typeof v === 'object' ? v[lang] : undefined;
}

/** "1.2 MB", "860 kB". */
export function formatBytes(n: number): string {
  if (n >= 1e9) return `${(n / 1e9).toFixed(2)} GB`;
  if (n >= 1e6) return `${(n / 1e6).toFixed(1)} MB`;
  return `${Math.round(n / 1e3)} kB`;
}

export function formatMs(n: number): string {
  return n >= 1000 ? `${(n / 1000).toFixed(2)} s` : `${Math.round(n)} ms`;
}

export function formatPercent(n: number): string {
  return `${n.toFixed(1)}%`;
}
