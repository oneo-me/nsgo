// Reads a folder of Markdown documents that list one entry each: a `#`
// name, a paragraph about it, a list of details and a screenshot beside
// the document. The Showcases and Components pages share that shape; each
// says which details it takes and checks their values.
//
// A document that breaks the shape fails the build with its line, so a
// pull request adds an entry by adding a document and an image.

import type { Image, List, Paragraph, PhrasingContent, Root } from 'mdast';
import type { ImageMetadata } from 'astro';
import { fromMarkdown } from 'mdast-util-from-markdown';

/** A screenshot from a document, resolved to the image beside it. */
export interface Screen {
  src: string;
  width: number;
  height: number;
  alt: string;
}

/** What a document says for one detail and the line it says it on. */
export interface Detail {
  value: string;
  line: number;
}

/** One document, as the reader hands it to the page that lists it. */
export interface Entry {
  name: string;
  /** The paragraph under the name, inline Markdown rendered to HTML. */
  summary: string;
  screen: Screen;
  details: Map<string, Detail>;
}

export interface CollectionOptions {
  /** The folder from the repository root, which errors name. */
  folder: string;
  /** What one document describes, `app` or `component`, in errors. */
  noun: string;
  /** The detail keys it takes, in the order errors list them. */
  keys: readonly string[];
  /** The details every document needs, each with what it is for. */
  required?: Readonly<Record<string, string>>;
  /** The folder's documents and images, globbed by the caller. */
  documents: Record<string, string>;
  images: Record<string, ImageMetadata>;
  /** check answers what is wrong with one detail's value, if anything. */
  check?: (key: string, value: string) => string | undefined;
}

/** One document while it is being read. */
interface Draft {
  name: string;
  line: number;
  summary?: string;
  screen?: Screen;
  details: Map<string, Detail>;
}

/**
 * readCollection reads the folder's documents in the order of their
 * names, the README excepted. It throws when the folder is empty or a
 * document is not the shape the folder's README describes.
 */
export function readCollection(options: CollectionOptions): Entry[] {
  const { folder, documents } = options;
  const names = new Map<string, string>();
  const entries: Entry[] = [];
  for (const path of Object.keys(documents).sort()) {
    const file = fileName(path);
    if (file.toLowerCase() === 'readme.md') continue;
    entries.push(readDocument(file, documents[path], names, options));
  }
  if (entries.length === 0) {
    throw new Error(`${folder}: add a document, starting with \`# Name\`; see ${folder}/README.md`);
  }
  return entries.sort((a, b) => a.name.localeCompare(b.name));
}

function readDocument(
  file: string,
  markdown: string,
  names: Map<string, string>,
  options: CollectionOptions,
): Entry {
  const { folder, noun, keys, required = {}, images, check } = options;
  const tree: Root = fromMarkdown(markdown);
  function fail(line: number | undefined, message: string): never {
    throw new Error(`${folder}/${file}:${line ?? 1}: ${message}`);
  }
  let draft: Draft | undefined;

  for (const node of tree.children) {
    // Comments, front matter and rules are not part of an entry.
    if (node.type === 'html' || node.type === 'yaml' || node.type === 'thematicBreak') continue;
    const line = node.position?.start.line;

    if (node.type === 'heading') {
      if (node.depth !== 1) {
        fail(line, `a ${noun} is named by a \`#\` heading; it has no other headings`);
      }
      if (draft !== undefined) {
        fail(line, 'a document lists one entry; start another in a document of its own');
      }
      const name = text(node.children).trim();
      if (name === '') fail(line, 'the `#` heading needs a name');
      const other = names.get(name.toLowerCase());
      if (other !== undefined) fail(line, `“${name}” is already listed in ${other}`);
      names.set(name.toLowerCase(), file);
      draft = { name, line: line ?? 1, details: new Map() };
      continue;
    }

    if (draft === undefined) {
      fail(line, 'start the document with the name, as `# Name`');
    }

    if (node.type === 'paragraph') {
      const image = onlyImage(node);
      if (image !== undefined) {
        addScreen(folder, draft, node, image, images, fail);
      } else if (draft.summary === undefined) {
        draft.summary = inline(node.children);
      } else {
        fail(line, `“${draft.name}” has more than one paragraph; keep it to one sentence`);
      }
      continue;
    }

    if (node.type === 'list') {
      addDetails(draft, node, keys, check, fail);
      continue;
    }

    fail(line, 'a document is a `#` name, one paragraph, a list of details and a screenshot');
  }

  if (draft === undefined) {
    fail(1, `the document lists no entry; start it with \`# Name\``);
  }
  if (draft.summary === undefined) {
    fail(draft.line, `“${draft.name}” needs a paragraph under its name saying what it does`);
  }
  if (draft.screen === undefined) {
    fail(
      draft.line,
      `“${draft.name}” needs a screenshot, written as \`![what it shows](file.jpg)\``,
    );
  }
  for (const [key, hint] of Object.entries(required)) {
    if (draft.details.has(key)) continue;
    fail(draft.line, `“${draft.name}” needs ${hint}; add \`- ${key}: …\``);
  }
  return { name: draft.name, summary: draft.summary, screen: draft.screen, details: draft.details };
}

function addScreen(
  folder: string,
  draft: Draft,
  node: Paragraph,
  image: Image,
  images: Record<string, ImageMetadata>,
  fail: (line: number | undefined, message: string) => never,
) {
  if (draft.screen !== undefined) {
    fail(node.position?.start.line, `“${draft.name}” has more than one screenshot; keep one`);
  }
  const name = fileName(image.url);
  const screen = findImage(images, name);
  if (screen === undefined) {
    fail(
      node.position?.start.line,
      `no screenshot “${name}” in ${folder}; it has ${fileNames(images) || 'none'}`,
    );
  }
  draft.screen = {
    src: screen.src,
    width: screen.width,
    height: screen.height,
    alt: image.alt?.trim() || draft.name,
  };
}

function findImage(images: Record<string, ImageMetadata>, name: string): ImageMetadata | undefined {
  return Object.entries(images).find(([path]) => fileName(path).toLowerCase() === name.toLowerCase())?.[1];
}

function addDetails(
  draft: Draft,
  node: List,
  keys: readonly string[],
  check: ((key: string, value: string) => string | undefined) | undefined,
  fail: (line: number | undefined, message: string) => never,
) {
  for (const item of node.children) {
    const line = item.position?.start.line;
    const paragraph =
      item.children.length === 1 && item.children[0].type === 'paragraph' ? item.children[0] : undefined;
    if (paragraph === undefined) {
      fail(line, 'write each detail on one line, as `- key: value`');
    }
    const value = text(paragraph.children);
    const colon = value.indexOf(':');
    const key = colon === -1 ? '' : value.slice(0, colon).trim().toLowerCase();
    const rest = colon === -1 ? '' : value.slice(colon + 1).trim();
    if (!keys.includes(key)) {
      fail(
        line,
        `unknown detail “${key || value}”; write ${keys.map((key) => `\`${key}\``).join(', ')} as \`- key: value\``,
      );
    }
    if (draft.details.has(key)) {
      fail(line, `“${draft.name}” lists ${key} twice`);
    }
    if (rest === '') fail(line, `“${key}” needs a value after its colon`);
    const problem = check?.(key, rest);
    if (problem !== undefined) fail(line, problem);
    draft.details.set(key, { value: rest, line: line ?? 1 });
  }
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

function fileNames(images: Record<string, ImageMetadata>): string {
  return Object.keys(images)
    .map((path) => `“${fileName(path)}”`)
    .join(', ');
}

/** text is the prose of inline Markdown, without its formatting. */
export function text(nodes: PhrasingContent[]): string {
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
export function inline(nodes: PhrasingContent[]): string {
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
          // An entry comes from a pull request, so it links only where a
          // page may: http, https, mail, the site or an anchor.
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
