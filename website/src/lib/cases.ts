// The apps the site lists at /showcases/, one Markdown document each in
// docs/public/cases/. A document is a `#` name, a paragraph about the app,
// a list of its details and a screenshot beside it; the folder's README
// says how to write one and the parser turns them into the page's data.
//
// A document that breaks the shape fails the build with its line, so a
// pull request adds a case by adding a document and an image.

import type { Image, List, Paragraph, PhrasingContent, Root } from 'mdast';
import type { ImageMetadata } from 'astro';
import { fromMarkdown } from 'mdast-util-from-markdown';
import { platforms } from '../site';

type Platform = (typeof platforms)[number];

export interface Case {
  name: string;
  /** What it does, inline Markdown rendered to HTML. */
  summary: string;
  platforms: Platform[];
  site?: string;
  source?: string;
  author?: string;
  screen: { src: string; width: number; height: number; alt: string };
}

/** The documents over the screenshots they share a folder with. */
const documents = import.meta.glob('../../../docs/public/cases/*.md', {
  query: '?raw',
  import: 'default',
  eager: true,
}) as Record<string, string>;

const images = import.meta.glob('../../../docs/public/cases/*.{jpg,jpeg,png,webp}', {
  eager: true,
  import: 'default',
}) as Record<string, ImageMetadata>;

// Screenshots are looked up by name in any case, so a document may write
// `Orbit.jpg` for `orbit.jpg`.
const screens = new Map<string, ImageMetadata>(
  Object.entries(images).map(([path, image]) => [fileName(path).toLowerCase(), image]),
);

const folder = 'docs/public/cases';

/** One app as its document writes it, while it is being read. */
interface Draft {
  name: string;
  line: number;
  summary?: string;
  screen?: Case['screen'];
  details: Set<string>;
  platforms: Platform[];
  site?: string;
  source?: string;
  author?: string;
}

class CaseError extends Error {
  constructor(file: string, line: number | undefined, message: string) {
    super(`${folder}/${file}:${line ?? 1}: ${message}`);
  }
}

/**
 * cases are the folder's documents in the order of their apps' names,
 * the README excepted. The module throws when the folder is empty or a
 * document is not the shape the README describes.
 */
export const cases: Case[] = read();

function read(): Case[] {
  const names = new Map<string, string>();
  const entries: Case[] = [];
  for (const path of Object.keys(documents).sort()) {
    const file = fileName(path);
    if (file.toLowerCase() === 'readme.md') continue;
    entries.push(readCase(file, documents[path], names));
  }
  if (entries.length === 0) {
    throw new Error(`${folder}: add a case, starting with \`# Name\`; see ${folder}/README.md`);
  }
  return entries.sort((a, b) => a.name.localeCompare(b.name));
}

function readCase(file: string, markdown: string, names: Map<string, string>): Case {
  const tree: Root = fromMarkdown(markdown);
  let draft: Draft | undefined;

  for (const node of tree.children) {
    // Comments, front matter and rules are not part of a case.
    if (node.type === 'html' || node.type === 'yaml' || node.type === 'thematicBreak') continue;
    const line = node.position?.start.line;

    if (node.type === 'heading') {
      if (node.depth !== 1) {
        throw new CaseError(file, line, 'a case is one app named by a `#` heading; it has no other headings');
      }
      if (draft !== undefined) {
        throw new CaseError(file, line, 'a case has one name; start another app in a document of its own');
      }
      const name = text(node.children).trim();
      if (name === '') throw new CaseError(file, line, 'the `#` heading needs the app’s name');
      const other = names.get(name.toLowerCase());
      if (other !== undefined) throw new CaseError(file, line, `“${name}” is already a case in ${other}`);
      names.set(name.toLowerCase(), file);
      draft = { name, line: line ?? 1, details: new Set(), platforms: [] };
      continue;
    }

    if (draft === undefined) {
      throw new CaseError(file, line, 'start the document with the app’s name, as `# Name`');
    }

    if (node.type === 'paragraph') {
      const image = onlyImage(node);
      if (image !== undefined) {
        addScreen(file, draft, node, image);
      } else if (draft.summary === undefined) {
        draft.summary = inline(node.children);
      } else {
        throw new CaseError(file, line, `“${draft.name}” has more than one paragraph; keep it to one sentence`);
      }
      continue;
    }

    if (node.type === 'list') {
      addDetails(file, draft, node);
      continue;
    }

    throw new CaseError(file, line, 'a case is a `#` name, one paragraph, a list of details and a screenshot');
  }

  if (draft === undefined) {
    throw new CaseError(file, 1, 'the document has no case; start it with `# Name`');
  }
  if (draft.summary === undefined) {
    throw new CaseError(file, draft.line, `“${draft.name}” needs a paragraph under its name saying what it does`);
  }
  if (draft.screen === undefined) {
    throw new CaseError(
      file,
      draft.line,
      `“${draft.name}” needs a screenshot, written as \`![what it shows](file.jpg)\``,
    );
  }
  return {
    name: draft.name,
    summary: draft.summary,
    platforms: draft.platforms,
    site: draft.site,
    source: draft.source,
    author: draft.author,
    screen: draft.screen,
  };
}

function addScreen(file: string, draft: Draft, node: Paragraph, image: Image) {
  if (draft.screen !== undefined) {
    throw new CaseError(file, node.position?.start.line, `“${draft.name}” has more than one screenshot; keep one`);
  }
  const name = fileName(image.url);
  const screen = screens.get(name.toLowerCase());
  if (screen === undefined) {
    throw new CaseError(
      file,
      node.position?.start.line,
      `no screenshot “${name}” in ${folder}${screens.size === 0 ? '' : `; it has ${fileNames()}`}`,
    );
  }
  draft.screen = {
    src: screen.src,
    width: screen.width,
    height: screen.height,
    alt: image.alt?.trim() || draft.name,
  };
}

function addDetails(file: string, draft: Draft, node: List) {
  for (const item of node.children) {
    const line = item.position?.start.line;
    const paragraph =
      item.children.length === 1 && item.children[0].type === 'paragraph' ? item.children[0] : undefined;
    if (paragraph === undefined) {
      throw new CaseError(file, line, 'write each detail on one line, as `- key: value`');
    }
    const value = text(paragraph.children);
    const colon = value.indexOf(':');
    const key = colon === -1 ? '' : value.slice(0, colon).trim().toLowerCase();
    const rest = colon === -1 ? '' : value.slice(colon + 1).trim();
    if (!['platforms', 'site', 'source', 'author'].includes(key)) {
      throw new CaseError(
        file,
        line,
        `unknown detail “${key || value}”; write platforms, site, source or author as \`- key: value\``,
      );
    }
    if (draft.details.has(key)) {
      throw new CaseError(file, line, `“${draft.name}” lists ${key} twice`);
    }
    draft.details.add(key);
    if (rest === '') throw new CaseError(file, line, `“${key}” needs a value after its colon`);
    switch (key) {
      case 'platforms':
        draft.platforms = rest.split(',').map((name) => platform(file, line, name.trim()));
        break;
      case 'site':
      case 'source':
        draft[key] = address(file, line, key, rest);
        break;
      case 'author':
        draft.author = rest;
        break;
    }
  }
}

function platform(file: string, line: number | undefined, name: string): Platform {
  const wanted = name.toLowerCase();
  const known = platforms.find(
    (platform) => platform.name.toLowerCase() === wanted || platform.zh?.toLowerCase() === wanted,
  );
  if (known === undefined) {
    throw new CaseError(
      file,
      line,
      `“${name}” is not a platform; write ${platforms.map((platform) => platform.name).join(', ')}`,
    );
  }
  return known;
}

function address(file: string, line: number | undefined, key: string, value: string): string {
  if (!/^https?:\/\/\S+$/.test(value)) {
    throw new CaseError(file, line, `“${key}” needs an http or https address`);
  }
  return value;
}

/** onlyImage reads a paragraph that is nothing but one screenshot. */
function onlyImage(node: Paragraph): Image | undefined {
  if (node.children.length !== 1) return undefined;
  return node.children[0].type === 'image' ? node.children[0] : undefined;
}

/** fileName is the path's last part, as a document names a screenshot. */
function fileName(path: string): string {
  return path.split('/').pop() ?? path;
}

function fileNames(): string {
  return Object.keys(images)
    .map((path) => `“${fileName(path)}”`)
    .join(', ');
}

/** text is the prose of inline Markdown, without its formatting. */
function text(nodes: PhrasingContent[]): string {
  return nodes
    .map((node) => {
      switch (node.type) {
        case 'text':
        case 'inlineCode':
          return node.value;
        case 'image':
          return node.alt ?? '';
        case 'break':
          return ' ';
        default:
          return 'children' in node ? text(node.children) : '';
      }
    })
    .join('');
}

/** inline renders the summary's Markdown, its code and links included. */
function inline(nodes: PhrasingContent[]): string {
  return nodes
    .map((node) => {
      switch (node.type) {
        case 'text':
          return escape(node.value);
        case 'inlineCode':
          return `<code>${escape(node.value)}</code>`;
        case 'strong':
          return `<strong>${inline(node.children)}</strong>`;
        case 'emphasis':
          return `<em>${inline(node.children)}</em>`;
        case 'delete':
          return `<del>${inline(node.children)}</del>`;
        case 'link':
          // A case comes from a pull request, so it links only where a page
          // may: http, https, mail, the site or an anchor.
          if (!/^(https?:\/\/|mailto:|\/|#)/.test(node.url)) return inline(node.children);
          return `<a href="${escapeAttribute(node.url)}"${
            /^https?:\/\//.test(node.url) ? ' target="_blank" rel="noopener noreferrer"' : ''
          }>${inline(node.children)}</a>`;
        case 'break':
          return ' ';
        case 'image':
          return escape(node.alt ?? '');
        default:
          return 'children' in node ? inline(node.children) : '';
      }
    })
    .join('');
}

const entities: Record<string, string> = { '&': '&amp;', '<': '&lt;', '>': '&gt;' };

function escape(value: string): string {
  return value.replace(/[&<>]/g, (character) => entities[character]);
}

function escapeAttribute(value: string): string {
  return escape(value).replace(/"/g, '&quot;');
}
