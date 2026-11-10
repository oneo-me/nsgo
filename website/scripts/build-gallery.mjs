// Builds the gallery for the web and puts it in public/gallery/ for the
// site's Live demo links: the tool's site, its wasm compressed
// (web-build.mjs), behind loader.html, which reports its download,
// decompression, compilation and start progress and is also written over
// index.html. nsgo.json goes, as nothing mounts the gallery.
//
// public/gallery/ is generated: do not edit it.
import { readFileSync, rmSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { buildWeb, website } from './web-build.mjs';

const galleryDir = path.join(website, 'public', 'gallery');

console.log('Building the gallery for the web...');
const app = buildWeb({ app: 'examples/gallery', name: 'Gallery', output: galleryDir });
const index = readFileSync(path.join(galleryDir, 'index.html'), 'utf8');
const icon = index.match(/<link rel="icon" href="([^"]+)">/)?.[1] ?? '';
const loader = readFileSync(path.join(website, 'scripts', 'gallery-loader.html'), 'utf8')
  .replaceAll('__EXEC__', app.exec)
  .replaceAll('__GZIP__', app.gzip)
  .replaceAll('__RESOURCES__', JSON.stringify(app.resources ?? {}))
  .replaceAll('__GZIP_SIZE__', String(app.gzipSize))
  .replaceAll('__ICON__', icon);
writeFileSync(path.join(galleryDir, 'loader.html'), loader);
writeFileSync(path.join(galleryDir, 'index.html'), loader);
rmSync(path.join(galleryDir, 'nsgo.json'));
