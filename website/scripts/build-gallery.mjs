// Builds the gallery for the web and puts it in public/gallery/ for the
// site's Demo page.
//
// The nsgo tool writes index.html, wasm_exec.js, resources, a
// content-named wasm and nsgo.json, which names them. This script adds:
//
//   loader.html    the same app behind a progress-reporting loader,
//                  also written over index.html
//   app.wasm.gzip  the engine compressed, because Go's wasm is large.
//                  The extension is .gzip, not .gz: static servers send a
//                  .gz with Content-Encoding: gzip, and the loader wants
//                  the compressed bytes themselves.
//
// It then removes the uncompressed wasm and nsgo.json, which names it:
// the wasm is most of the site's bytes, and GitHub Pages can take longer
// than deploy-pages' ten-minute limit to sync it.
//
// public/gallery/ is generated: do not edit it.
import { execFileSync } from 'node:child_process';
import {
  cpSync,
  existsSync,
  mkdirSync,
  readFileSync,
  readdirSync,
  rmSync,
  statSync,
  writeFileSync,
} from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { gzipSync } from 'node:zlib';

const here = path.dirname(fileURLToPath(import.meta.url));
const website = path.resolve(here, '..');
const repo = path.resolve(website, '..');
const galleryDir = path.join(website, 'public', 'gallery');
const buildDir = path.join(website, '.gallery-build');

function findSite(dir) {
  const entries = readdirSync(dir, { withFileTypes: true });
  for (const entry of entries) {
    if (!entry.isDirectory()) continue;
    const candidate = path.join(dir, entry.name);
    if (existsSync(path.join(candidate, 'index.html'))) return candidate;
  }
  throw new Error(`No web build found under ${dir}`);
}

console.log('Building the gallery for the web...');
rmSync(buildDir, { recursive: true, force: true });
rmSync(galleryDir, { recursive: true, force: true });
mkdirSync(buildDir, { recursive: true });

try {
  execFileSync(
    'go',
    [
      'tool',
      'nsgo',
      'build',
      '-target',
      'web',
      '-name',
      'Gallery',
      '-open=false',
      '-output',
      buildDir,
      'examples/gallery',
    ],
    { cwd: repo, stdio: 'inherit' },
  );

  const site = findSite(buildDir);
  mkdirSync(galleryDir, { recursive: true });
  cpSync(site, galleryDir, { recursive: true });

  // nsgo.json is what the tool writes for a page showing the app: its
  // wasm, wasm_exec.js and resources, named by their content.
  const app = JSON.parse(readFileSync(path.join(galleryDir, 'nsgo.json'), 'utf8'));
  const exec = app.exec;
  const wasm = app.wasm;
  const resources = JSON.stringify(app.resources ?? {});
  const index = readFileSync(path.join(galleryDir, 'index.html'), 'utf8');
  const icon = index.match(/<link rel="icon" href="([^"]+)">/)?.[1] ?? '';

  const wasmPath = path.join(galleryDir, wasm);
  const wasmBytes = statSync(wasmPath).size;
  const gzip = `${wasm}.gzip`;
  const compressed = gzipSync(readFileSync(wasmPath), { level: 9 });
  writeFileSync(path.join(galleryDir, gzip), compressed);

  const loader = readFileSync(path.join(here, 'gallery-loader.html'), 'utf8')
    .replaceAll('__EXEC__', exec)
    .replaceAll('__GZIP__', gzip)
    .replaceAll('__RESOURCES__', resources)
    .replaceAll('__GZIP_SIZE__', String(compressed.length))
    .replaceAll('__ICON__', icon);
  writeFileSync(path.join(galleryDir, 'loader.html'), loader);
  writeFileSync(path.join(galleryDir, 'index.html'), loader);
  rmSync(wasmPath);
  rmSync(path.join(galleryDir, 'nsgo.json'));

  const mb = (bytes) => `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  console.log(`Gallery ready: ${wasm} ${mb(wasmBytes)} (${mb(compressed.length)} gzip)`);
} finally {
  rmSync(buildDir, { recursive: true, force: true });
}
