// Mounts the docs' live previews (src/lib/live.mjs): each box runs its
// demo or program of the docs' one app once it nears the window. The
// app's wasm is fetched, decompressed and compiled once a page, and each
// box runs a Go program of its own, in the site's appearance and the
// page's language.
import { currentTheme } from './theme';

interface Config {
  exec: string;
  loader: string;
  gzip: string;
  resources?: Record<string, string>;
}

interface Player {
  setAppearance(appearance: string): void;
}

type Mount = (element: HTMLElement, url: string, options: object) => Promise<Player>;

interface App {
  url: string;
  config: Config;
  module: WebAssembly.Module;
  mount: Mount;
}

const apps = new Map<string, Promise<App>>();
const players: Player[] = [];

// load prepares the app whose site is at dir once.
function load(dir: string): Promise<App> {
  let app = apps.get(dir);
  if (!app) {
    app = (async () => {
      const url = new URL('nsgo.json', new URL(dir, location.href)).href;
      const config = (await (await fetch(url)).json()) as Config;
      const response = await fetch(new URL(config.gzip, url));
      if (!response.ok) throw new Error(`${config.gzip}: ${response.status}`);
      let bytes = await response.arrayBuffer();
      // A host may already have decoded it; decompress only while the
      // gzip magic is there.
      const head = new Uint8Array(bytes, 0, 2);
      if (head[0] === 0x1f && head[1] === 0x8b) {
        bytes = await new Response(new Blob([bytes]).stream().pipeThrough(new DecompressionStream('gzip'))).arrayBuffer();
      }
      const [module, loader] = await Promise.all([
        WebAssembly.compile(bytes),
        import(/* @vite-ignore */ new URL(config.loader, url).href) as Promise<{ mount: Mount }>,
      ]);
      return { url, config, module, mount: loader.mount };
    })();
    apps.set(dir, app);
  }
  return app;
}

async function start(box: HTMLElement, locale: string, failed: string) {
  try {
    const app = await load(box.dataset.live!);
    const player = await app.mount(box, app.url, {
      config: app.config,
      module: app.module,
      params: box.dataset.program ? { program: box.dataset.program } : { demo: box.dataset.demo },
      appearance: currentTheme(),
      locale,
    });
    players.push(player);
    box.classList.add('is-running');
  } catch (error) {
    console.error(error);
    box.classList.add('is-failed');
    box.dataset.status = failed;
  }
}

const observer = new IntersectionObserver(
  (entries) => {
    for (const entry of entries) {
      if (!entry.isIntersecting) continue;
      const box = entry.target as HTMLElement;
      observer.unobserve(box);
      const prose = box.closest<HTMLElement>('.prose');
      void start(box, prose?.dataset.locale ?? 'en', prose?.dataset.liveFailed ?? '');
    }
  },
  { rootMargin: '400px 0px' },
);

// mountLive mounts the boxes in root as they near the window.
export function mountLive(root: ParentNode) {
  root.querySelectorAll<HTMLElement>('.live').forEach((box) => {
    box.dataset.status = box.closest<HTMLElement>('.prose')?.dataset.liveLoading ?? '';
    observer.observe(box);
  });
}

// hasLive reports whether an app runs or starts on the page. A Go program
// cannot be stopped, so a page with one is left by loading the next.
export function hasLive() {
  return apps.size > 0;
}

document.addEventListener('nsgo-theme', () => {
  for (const player of players) player.setAppearance(currentTheme());
});
