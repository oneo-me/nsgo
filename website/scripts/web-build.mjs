// Builds an app for the web with the nsgo tool and puts its site in a
// folder of public/, its wasm compressed.
//
// The tool writes index.html, wasm_exec.js, the loader (nsgo.js),
// resources, a content-named wasm and nsgo.json, which names them. This
// keeps all but the wasm, which it writes as <wasm>.gzip: Go's wasm is
// large, and GitHub Pages can take longer than deploy-pages' ten-minute
// limit to sync it uncompressed. The extension is .gzip, not .gz: static
// servers send a .gz with Content-Encoding: gzip, and the pages want the
// compressed bytes themselves.
import { execFileSync } from 'node:child_process';
import { cpSync, existsSync, mkdirSync, readFileSync, readdirSync, rmSync, statSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { gzipSync } from 'node:zlib';

export const website = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
export const repo = path.resolve(website, '..');

const mb = (bytes) => `${(bytes / (1024 * 1024)).toFixed(1)} MB`;

function findSite(dir) {
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const candidate = path.join(dir, entry.name);
    if (entry.isDirectory() && existsSync(path.join(candidate, 'index.html'))) return candidate;
  }
  throw new Error(`No web build found under ${dir}`);
}

// buildWeb builds app, a package path from the repository's root, into
// output and returns its nsgo.json with gzip, the compressed wasm's name,
// and gzipSize in place of wasm.
export function buildWeb({ app, name, output }) {
  const scratch = path.join(website, '.web-build', path.basename(output));
  rmSync(scratch, { recursive: true, force: true });
  rmSync(output, { recursive: true, force: true });
  mkdirSync(scratch, { recursive: true });
  try {
    execFileSync(
      'go',
      ['tool', 'nsgo', 'build', '-target', 'web', '-name', name, '-open=false', '-output', scratch, app],
      { cwd: repo, stdio: 'inherit' },
    );
    cpSync(findSite(scratch), output, { recursive: true });
  } finally {
    rmSync(scratch, { recursive: true, force: true });
  }

  const { wasm, ...config } = JSON.parse(readFileSync(path.join(output, 'nsgo.json'), 'utf8'));
  const wasmPath = path.join(output, wasm);
  const size = statSync(wasmPath).size;
  const compressed = gzipSync(readFileSync(wasmPath), { level: 9 });
  config.gzip = `${wasm}.gzip`;
  config.gzipSize = compressed.length;
  writeFileSync(path.join(output, config.gzip), compressed);
  rmSync(wasmPath);
  writeFileSync(path.join(output, 'nsgo.json'), `${JSON.stringify(config, null, 2)}\n`);
  console.log(`${name} ready: ${wasm} ${mb(size)} (${mb(compressed.length)} gzip)`);
  return config;
}
