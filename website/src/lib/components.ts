// The component libraries the site lists at /components/, one Markdown
// document each in docs/public/components/. A document is a `#` name, a
// paragraph about the library, a list of its details and a screenshot
// beside it; the folder's README says how to write one and the parser
// turns them into the page's data.
//
// A document that breaks the shape fails the build with its line, so a
// pull request adds a component by adding a document and an image.

import type { ImageMetadata } from 'astro';
import { readCollection } from './collection';

export interface Component {
  name: string;
  /** What it gives an app, inline Markdown rendered to HTML. */
  summary: string;
  /** What it provides, such as Icons or Charts. */
  kind: string;
  /** The Go import path apps `go get`. */
  package: string;
  /** Whether NSGo maintains it. */
  official: boolean;
  site?: string;
  source?: string;
  author?: string;
  screen: { src: string; width: number; height: number; alt: string };
}

/** The documents over the screenshots they share a folder with. */
const documents = import.meta.glob('../../../docs/public/components/*.md', {
  query: '?raw',
  import: 'default',
  eager: true,
}) as Record<string, string>;

const images = import.meta.glob('../../../docs/public/components/*.{jpg,jpeg,png,webp}', {
  eager: true,
  import: 'default',
}) as Record<string, ImageMetadata>;

const folder = 'docs/public/components';

/**
 * components are the folder's documents in the order of their libraries'
 * names, the README excepted. The module throws when the folder is empty
 * or a document is not the shape the README describes.
 */
export const components: Component[] = read();

function read(): Component[] {
  return readCollection({
    folder,
    noun: 'component',
    keys: ['kind', 'package', 'site', 'source', 'author', 'official'],
    required: {
      kind: 'a word for what it provides, such as Icons',
      package: 'the Go import path apps `go get`',
    },
    documents,
    images,
    check(key, value) {
      switch (key) {
        case 'package':
          if (!/^\S+\/\S+$/.test(value)) {
            return `“${key}” needs the Go import path, as \`github.com/you/your-component\``;
          }
          return;
        case 'site':
        case 'source':
          if (!/^https?:\/\/\S+$/.test(value)) return `“${key}” needs an http or https address`;
          return;
        case 'official':
          if (value !== 'true' && value !== 'false') return '“official” is `true` or `false`';
          return;
        default:
          return;
      }
    },
  }).map((entry) => ({
    name: entry.name,
    summary: entry.summary,
    kind: entry.details.get('kind')!.value,
    package: entry.details.get('package')!.value,
    official: entry.details.get('official')?.value === 'true',
    site: entry.details.get('site')?.value,
    source: entry.details.get('source')?.value,
    author: entry.details.get('author')?.value,
    screen: entry.screen,
  }));
}
