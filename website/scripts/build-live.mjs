// Builds what the docs' live boxes run (src/lib/live.mjs) as one app in
// public/live/, so a reader downloads and compiles NSGo once for every
// box of the site: examples/previews and each program a page marks, each
// a package of its own under a main that runs the one its launch
// parameters name. The app is written in _live/ inside the module, so its
// imports are this checkout's, and is the tool's site with its wasm
// compressed (web-build.mjs), whose nsgo.json the docs' pages read.
//
// public/live/ is generated: do not edit it.
import { mkdirSync, readFileSync, readdirSync, rmSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { fromMarkdown } from 'mdast-util-from-markdown';
import { parseLive, programId } from '../src/lib/live.mjs';
import { buildWeb, repo, website } from './web-build.mjs';

const docs = path.join(repo, 'docs', 'public');
const previews = path.join(repo, 'examples', 'previews');
// scratch's name starts with _ so ./... patterns skip it, and not with .,
// which no import path may.
const scratch = path.join(website, '_live');

// programs are the marked blocks' code by id, with the page that first
// marks each.
const programs = new Map();
function collect(node, page) {
  if (node.type === 'code' && node.lang === 'go' && parseLive(node.meta)?.kind === 'program') {
    const id = programId(node.value);
    if (!programs.has(id)) programs.set(id, { code: node.value, page });
  }
  for (const child of node.children ?? []) collect(child, page);
}
for (const lang of ['en', 'zh']) {
  for (const file of readdirSync(path.join(docs, lang), { recursive: true })) {
    if (!file.endsWith('.md')) continue;
    collect(fromMarkdown(readFileSync(path.join(docs, lang, file), 'utf8')), `${lang}/${file}`);
  }
}

// writePackage writes a main package's code as the package name in dir:
// its main stays, unused, beside the Root that the app's main runs.
function writePackage(dir, name, file, code) {
  if (!/^package main$/m.test(code)) throw new Error(`${dir}/${file}: not package main`);
  mkdirSync(dir, { recursive: true });
  writeFileSync(path.join(dir, path.basename(file)), code.replace(/^package main$/m, `package ${name}`));
}

rmSync(scratch, { recursive: true, force: true });
try {
  for (const file of readdirSync(previews)) {
    if (!file.endsWith('.go') || file.endsWith('_test.go')) continue;
    writePackage(path.join(scratch, 'previews'), 'previews', file, readFileSync(path.join(previews, file), 'utf8'));
  }
  const imports = [];
  const entries = [];
  for (const [id, { code, page }] of programs) {
    if (!/^func Root\(/m.test(code)) throw new Error(`${page}: a program needs a Root to run`);
    const name = `p${id}`;
    writePackage(path.join(scratch, name), name, 'app.go', `${code}\n`);
    imports.push(`\t${name} "github.com/oneo-me/nsgo/website/_live/${name}"`);
    entries.push(`\t${JSON.stringify(id)}: ${name}.Root, // ${page}`);
  }
  writeFileSync(
    path.join(scratch, 'app.go'),
    `// Command live runs the docs' live boxes: the program its launch
// parameter program names, else the component previews. Written by
// website/scripts/build-live.mjs.
package main

import (
\t"github.com/oneo-me/nsgo"
\t"github.com/oneo-me/nsgo/core"

\t"github.com/oneo-me/nsgo/website/_live/previews"
${imports.join('\n')}
)

// programs are the docs' program blocks' Roots by id.
var programs = map[string]func(cx *core.Scope) core.Node{
${entries.join('\n')}
}

func main() { nsgo.Run(Root) }

func Root(cx *core.Scope) core.Node {
\tid, _ := cx.HostParams()["program"].(string)
\tif root, ok := programs[id]; ok {
\t\treturn core.Func(root)
\t}
\treturn core.Func(previews.Root)
}
`,
  );
  writeFileSync(path.join(scratch, 'nsgo.yaml'), 'name: Live\nid: dev.oneo.nsgo.docs\nversion: 0.1.0\n');
  console.log(`Building the docs' previews and ${programs.size} programs for the web...`);
  buildWeb({ app: `./${path.relative(repo, scratch)}`, name: 'Live', output: path.join(website, 'public', 'live') });
} finally {
  rmSync(scratch, { recursive: true, force: true });
}
