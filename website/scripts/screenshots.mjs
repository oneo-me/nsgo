// Takes the home page's screenshots of the gallery from its web build:
// the desktop layout and the phone layout, in English and Chinese, into
// src/assets/screens/. Run `bun run gallery` first; it needs Google Chrome
// and macOS's sips.
//
// The gallery is served from public/gallery/ and driven through Chrome's
// DevTools protocol, which waits for the wasm to start before capturing.
import { execFileSync, spawn } from 'node:child_process';
import { existsSync, mkdtempSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const website = path.resolve(here, '..');
const publicDir = path.join(website, 'public');
const outDir = path.join(website, 'src', 'assets', 'screens');
const chrome = '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';
// Long enough to download, compile and start the gallery from disk.
const startWait = 30000;

if (!existsSync(path.join(publicDir, 'gallery', 'index.html'))) {
  throw new Error('public/gallery/ is missing: run `bun run gallery` first');
}

const server = Bun.serve({
  port: 0,
  async fetch(request) {
    const { pathname } = new URL(request.url);
    const file = Bun.file(path.join(publicDir, decodeURIComponent(pathname.replace(/^\//, ''))));
    return (await file.exists()) ? new Response(file) : new Response('Not found', { status: 404 });
  },
});

const shots = [
  { name: 'desktop', width: 1440, height: 900, mobile: false, resize: ['-Z', '1800'], quality: '82' },
  { name: 'phone', width: 390, height: 844, mobile: true, resize: [], quality: '85' },
];
const locales = { en: 'en-US', zh: 'zh-CN' };

async function capture(url, shot, locale, out, port) {
  const profile = mkdtempSync(path.join(tmpdir(), 'nsgo-shot-'));
  const browser = spawn(
    chrome,
    ['--headless=new', `--lang=${locale}`, `--accept-lang=${locale}`, `--remote-debugging-port=${port}`, `--user-data-dir=${profile}`, 'about:blank'],
    { stdio: 'ignore' },
  );
  try {
    let page;
    for (let i = 0; i < 50 && !page; i++) {
      try {
        const targets = await (await fetch(`http://127.0.0.1:${port}/json`)).json();
        page = targets.find((target) => target.type === 'page');
      } catch {
        await Bun.sleep(200);
      }
    }
    const socket = new WebSocket(page.webSocketDebuggerUrl);
    await new Promise((resolve) => (socket.onopen = resolve));
    let id = 0;
    const pending = new Map();
    socket.onmessage = (event) => {
      const message = JSON.parse(String(event.data));
      pending.get(message.id)?.(message.result);
      pending.delete(message.id);
    };
    const send = (method, params = {}) =>
      new Promise((resolve) => {
        pending.set(++id, resolve);
        socket.send(JSON.stringify({ id, method, params }));
      });

    await send('Emulation.setLocaleOverride', { locale });
    await send('Emulation.setDeviceMetricsOverride', {
      width: shot.width,
      height: shot.height,
      deviceScaleFactor: 2,
      mobile: shot.mobile,
    });
    if (shot.mobile) await send('Emulation.setTouchEmulationEnabled', { enabled: true });
    await send('Page.navigate', { url });
    await Bun.sleep(startWait);
    const { data } = await send('Page.captureScreenshot', { format: 'png' });
    const png = `${out}.png`;
    await Bun.write(png, Buffer.from(data, 'base64'));
    execFileSync('sips', ['-s', 'format', 'jpeg', '-s', 'formatOptions', shot.quality, ...shot.resize, png, '--out', out], {
      stdio: 'ignore',
    });
    rmSync(png);
    socket.close();
  } finally {
    browser.kill();
    rmSync(profile, { recursive: true, force: true });
  }
}

try {
  const url = `http://localhost:${server.port}/gallery/?theme=light`;
  // Each capture runs its own Chrome on its own debugging port.
  const jobs = Object.entries(locales).flatMap(([lang, locale]) =>
    shots.map((shot) => ({ shot, locale, out: path.join(outDir, `${shot.name}-${lang}.jpg`) })),
  );
  await Promise.all(jobs.map((job, index) => capture(url, job.shot, job.locale, job.out, 9320 + index)));
  console.log(`Screenshots written to ${path.relative(website, outDir)}/`);
} finally {
  server.stop(true);
}
