// Builds what the docs' live previews run (src/lib/live.mjs):
// examples/previews into public/previews/, and each program a page marks
// into public/programs/<id>/, from its code with an nsgo.yaml of its own,
// written in .apps-build/ inside the module so its imports are this
// checkout's. Each folder is the tool's site with its wasm compressed
// (web-build.mjs), whose nsgo.json the docs' pages read.
//
// public/previews/ and public/programs/ are generated: do not edit them.
import { mkdirSync, readFileSync, readdirSync, rmSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { fromMarkdown } from 'mdast-util-from-markdown';
import { parseLive, programId } from '../src/lib/live.mjs';
import { buildWeb, repo, website } from './web-build.mjs';

const docs = path.join(repo, 'docs', 'public');
const programsDir = path.join(website, 'public', 'programs');
const scratch = path.join(website, '.apps-build');

console.log('Building the component previews for the web...');
buildWeb({ app: 'examples/previews', name: 'Previews', output: path.join(website, 'public', 'previews') });

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

rmSync(programsDir, { recursive: true, force: true });
rmSync(scratch, { recursive: true, force: true });
try {
  for (const [id, { code, page }] of programs) {
    // An app is named after its page: Todo for tutorials/todo.md.
    const slug = path.basename(page, '.md');
    const name = slug[0].toUpperCase() + slug.slice(1);
    console.log(`Building ${page}'s program for the web...`);
    const dir = path.join(scratch, id);
    mkdirSync(dir, { recursive: true });
    writeFileSync(path.join(dir, 'app.go'), `${code}\n`);
    writeFileSync(path.join(dir, 'nsgo.yaml'), `name: ${name}\nid: dev.oneo.nsgo.docs.${slug}\nversion: 0.1.0\n`);
    buildWeb({ app: `./${path.relative(repo, dir)}`, name, output: path.join(programsDir, id) });
  }
} finally {
  rmSync(scratch, { recursive: true, force: true });
}
