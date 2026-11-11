// Reads a folder of Markdown documents that list one entry each: a `#`
// name, a paragraph about it, a list of details and a screenshot beside
// the document. The Showcases and Components pages share that shape; each
// says which details it takes and checks their values.
//
// The folder's own documents are English. A translation sits in the
// language's folder (`zh/`) under the English document's file name, and
// says what reads differently in its language: its name, its paragraph,
// optionally its screenshot, and the details the collection marks
// translatable.
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
  /** True when the document is in the page's language, false when it shows English. */
  translated: boolean;
}

/** The documents of a folder in every language the site shows. */
export interface Collection {
  /** The English entries, in the order of their names. */
  entries: Entry[];
  /** in answers the entries in one language, or the English ones it lacks. */
  in(language: string): Entry[];
}

export interface CollectionOptions {
  /** The folder from the repository root, which errors name. */
  folder: string;
  /** What one document describes, `app` or `component`, in errors. */
  noun: string;
  /** The detail keys it takes, in the order errors list them. */
  keys: readonly string[];
  /** The details every English document needs, each with what it is for. */
  required?: Readonly<Record<string, string>>;
  /** The detail keys a translation may say in its language, such as a component's kind. */
  translatable?: readonly string[];
  /** The folder's English documents and images, globbed by the caller. */
  documents: Record<string, string>;
  images: Record<string, ImageMetadata>;
  /** Each language's translations, keyed by the language and then by file name. */
  translations?: Readonly<Record<string, Record<string, string>>>;
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

/** What readDocument knows about the document's language. */
interface DocumentState {
  /** The language a translation is written in; undefined for English. */
  language?: string;
  /** The document a translation translates. */
  base?: Entry;
  /** The names the language's documents already list. */
  names?: Map<string, string>;
}

/**
 * readCollection reads the folder's English documents in the order of
 * their names, the README excepted, and each language's translations over
 * them. It throws when the folder is empty or a document is not the shape
 * the folder's README describes.
 */
export function readCollection(options: CollectionOptions): Collection {
  const { folder, documents, translations = {} } = options;
  const names = new Map<string, string>();
  const english = new Map<string, Entry>();
  for (const path of Object.keys(documents).sort()) {
    const file = fileName(path);
    if (file.toLowerCase() === 'readme.md') continue;
    english.set(file, readDocument(file, documents[path], options, { names }));
  }
  if (english.size === 0) {
    throw new Error(`${folder}: add a document, starting with \`# Name\`; see ${folder}/README.md`);
  }

  const languages = new Map<string, Map<string, Entry>>();
  for (const language of Object.keys(translations).sort()) {
    const translated = new Map<string, Entry>();
    const translatedNames = new Map<string, string>();
    for (const path of Object.keys(translations[language]).sort()) {
      const file = fileName(path);
      const base = english.get(file);
      if (base === undefined) {
        throw new Error(
          `${folder}/${language}/${file}:1: there is no ${folder}/${file} to translate; ` +
            'a translation has the English document’s file name',
        );
      }
      translated.set(
        file,
        readTranslation(file, translations[language][path], base, language, options, translatedNames),
      );
    }
    languages.set(language, translated);
  }

  const ordered = [...english.entries()].sort(([, a], [, b]) => byName(a, b));
  return {
    entries: ordered.map(([, entry]) => entry),
    in(language) {
      const translated = language === 'en' ? undefined : languages.get(language);
      return ordered.map(([file, entry]) => {
        if (language === 'en') return entry;
        return translated?.get(file) ?? { ...entry, translated: false };
      });
    },
  };
}

/**
 * readTranslations groups the documents globbed from `<folder>/<language>/`
 * by language and file name, failing on a folder that is not a language of
 * the site.
 */
export function readTranslations(
  documents: Record<string, string>,
  folder: string,
  languages: readonly string[],
): Record<string, Record<string, string>> {
  const translations: Record<string, Record<string, string>> = {};
  for (const [path, markdown] of Object.entries(documents)) {
    const parts = path.split('/');
    const language = parts[parts.length - 2];
    if (language === 'en') {
      throw new Error(`${folder}/en: English documents live at the folder’s root, not in en/`);
    }
    if (!languages.includes(language)) {
      const known = languages.length === 0 ? 'none yet' : languages.join(', ');
      throw new Error(`${folder}/${language}: ${language} is not a language of the site; translations are in ${known}`);
    }
    (translations[language] ??= {})[fileName(path)] = markdown;
  }
  return translations;
}

function readTranslation(
  file: string,
  markdown: string,
  base: Entry,
  language: string,
  options: CollectionOptions,
  names: Map<string, string>,
): Entry {
  const entry = readDocument(file, markdown, options, { language, base, names });
  return {
    ...entry,
    details: new Map([...base.details, ...entry.details]),
    translated: true,
  };
}

function readDocument(
  file: string,
  markdown: string,
  options: CollectionOptions,
  state: DocumentState,
): Entry {
  const { folder, noun, keys, required = {}, images, check } = options;
  const { language, base, names } = state;
  const translated = language !== undefined;
  const source = translated ? `${folder}/${language}/${file}` : `${folder}/${file}`;
  const tree: Root = fromMarkdown(markdown);
  function fail(line: number | undefined, message: string): never {
    throw new Error(`${source}:${line ?? 1}: ${message}`);
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
      const other = names?.get(name.toLowerCase());
      if (other !== undefined) fail(line, `“${name}” is already listed in ${other}`);
      names?.set(name.toLowerCase(), translated ? `${language}/${file}` : file);
      draft = { name, line: line ?? 1, details: new Map() };
      continue;
    }

    if (draft === undefined) {
      fail(line, 'start the document with the name, as `# Name`');
    }

    if (node.type === 'paragraph') {
      const image = onlyImage(node);
      if (image !== undefined) {
        addScreen(draft, node, image, images, folder, language, fail);
      } else if (draft.summary === undefined) {
        draft.summary = inline(node.children);
      } else {
        fail(line, `“${draft.name}” has more than one paragraph; keep it to one sentence`);
      }
      continue;
    }

    if (node.type === 'list') {
      addDetails(draft, node, keys, options.translatable ?? [], translated, check, fail);
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
  const screen = draft.screen ?? base?.screen;
  if (screen === undefined) {
    fail(
      draft.line,
      `“${draft.name}” needs a screenshot, written as \`![what it shows](file.jpg)\``,
    );
  }
  if (!translated) {
    for (const [key, hint] of Object.entries(required)) {
      if (draft.details.has(key)) continue;
      fail(draft.line, `“${draft.name}” needs ${hint}; add \`- ${key}: …\``);
    }
  }
  return {
    name: draft.name,
    summary: draft.summary,
    screen,
    details: draft.details,
    translated: !translated,
  };
}

function addScreen(
  draft: Draft,
  node: Paragraph,
  image: Image,
  images: Record<string, ImageMetadata>,
  folder: string,
  language: string | undefined,
  fail: (line: number | undefined, message: string) => never,
) {
  if (draft.screen !== undefined) {
    fail(node.position?.start.line, `“${draft.name}” has more than one screenshot; keep one`);
  }
  const name = fileName(image.url);
  const screen = findImage(images, name, folder, language);
  if (screen === undefined) {
    const where = language === undefined ? folder : `${folder}/${language} or ${folder}`;
    fail(
      node.position?.start.line,
      `no screenshot “${name}” in ${where}; it has ${fileNames(images) || 'none'}`,
    );
  }
  draft.screen = {
    src: screen.src,
    width: screen.width,
    height: screen.height,
    alt: image.alt?.trim() || draft.name,
  };
}

function findImage(
  images: Record<string, ImageMetadata>,
  name: string,
  folder: string,
  language: string | undefined,
): ImageMetadata | undefined {
  const wanted = name.toLowerCase();
  const marker = `/${folder}/`;
  let own: ImageMetadata | undefined;
  let root: ImageMetadata | undefined;
  for (const [path, image] of Object.entries(images)) {
    if (fileName(path).toLowerCase() !== wanted) continue;
    const normalized = path.replace(/\\/g, '/');
    const at = normalized.lastIndexOf(marker);
    if (at === -1) continue;
    const parts = normalized.slice(at + marker.length).split('/');
    if (parts.length === 1) {
      root = image;
    } else if (language !== undefined && parts.length === 2 && parts[0] === language) {
      own = image;
    }
  }
  return own ?? root;
}

function addDetails(
  draft: Draft,
  node: List,
  keys: readonly string[],
  translatable: readonly string[],
  translated: boolean,
  check: ((key: string, value: string) => string | undefined) | undefined,
  fail: (line: number | undefined, message: string) => never,
) {
  const allowed = translated ? translatable : keys;
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
    if (!allowed.includes(key)) {
      if (translated) {
        const only = allowed.length === 0 ? 'no details' : `only ${allowed.map((key) => `\`${key}\``).join(', ')}`;
        fail(line, `a translation gives ${only}; write \`${key || value}\` in the English document`);
      }
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

function byName(a: Entry, b: Entry): number {
  return a.name.localeCompare(b.name);
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
