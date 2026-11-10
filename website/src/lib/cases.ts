// The apps the site lists at /showcases/, one Markdown document each in
// docs/public/cases/. A document is a `#` name, a paragraph about the app,
// a list of its details and a screenshot beside it; the folder's README
// says how to write one and the parser turns them into the page's data.
//
// A document that breaks the shape fails the build with its line, so a
// pull request adds a case by adding a document and an image.

import type { ImageMetadata } from 'astro';
import { platforms } from '../site';
import { readCollection } from './collection';

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

const folder = 'docs/public/cases';

/**
 * cases are the folder's documents in the order of their apps' names,
 * the README excepted. The module throws when the folder is empty or a
 * document is not the shape the README describes.
 */
export const cases: Case[] = read();

function read(): Case[] {
  return readCollection({
    folder,
    noun: 'app',
    keys: ['platforms', 'site', 'source', 'author'],
    documents,
    images,
    check(key, value) {
      switch (key) {
        case 'platforms':
          for (const name of value.split(',')) {
            const problem = unknownPlatform(name.trim());
            if (problem !== undefined) return problem;
          }
          return;
        case 'site':
        case 'source':
          if (!/^https?:\/\/\S+$/.test(value)) return `“${key}” needs an http or https address`;
          return;
        default:
          return;
      }
    },
  }).map((entry) => ({
    name: entry.name,
    summary: entry.summary,
    platforms: (entry.details.get('platforms')?.value ?? '')
      .split(',')
      .map((name) => name.trim())
      .filter((name) => name !== '')
      .map(platformOf),
    site: entry.details.get('site')?.value,
    source: entry.details.get('source')?.value,
    author: entry.details.get('author')?.value,
    screen: entry.screen,
  }));
}

function platformOf(name: string): Platform {
  const wanted = name.toLowerCase();
  const known = platforms.find(
    (platform) => platform.name.toLowerCase() === wanted || platform.zh?.toLowerCase() === wanted,
  );
  if (known === undefined) throw new Error(`“${name}” is not a platform`);
  return known;
}

function unknownPlatform(name: string): string | undefined {
  const wanted = name.toLowerCase();
  const known = platforms.some(
    (platform) => platform.name.toLowerCase() === wanted || platform.zh?.toLowerCase() === wanted,
  );
  if (known) return undefined;
  return `“${name}” is not a platform; write ${platforms.map((platform) => platform.name).join(', ')}`;
}
