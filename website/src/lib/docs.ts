import { getCollection, type CollectionEntry } from 'astro:content';
import type { Lang } from '../site';

export type DocEntry = CollectionEntry<'docs'>;

export interface Doc {
  /** The page's path below its language, such as getting-started. */
  id: string;
  entry: DocEntry;
  /** True when the language has no translation and shows English. */
  fallback: boolean;
}

export interface DocGroup {
  title: string;
  docs: Doc[];
}

/** A language's pages in the sidebar's order: English's pages, each in the language when translated. */
export async function getDocs(lang: Lang): Promise<Doc[]> {
  const entries = await getCollection('docs');
  // The loader names en/index.md `en` and en/testing.md `en/testing`.
  const byId = (prefix: string) =>
    new Map(
      entries
        .filter((entry) => entry.id === prefix || entry.id.startsWith(`${prefix}/`))
        .map((entry) => [entry.id === prefix ? 'index' : entry.id.slice(prefix.length + 1), entry] as const),
    );
  const english = byId('en');
  const translated = byId(lang);
  return [...english.entries()]
    .map(([id, entry]) => {
      const own = translated.get(id);
      return { id, entry: own ?? entry, fallback: !own };
    })
    .sort((a, b) => a.entry.data.order - b.entry.data.order);
}

/** The sidebar's groups, in the order of their first pages. */
export function groupDocs(docs: Doc[]): DocGroup[] {
  const groups: DocGroup[] = [];
  for (const doc of docs) {
    const title = doc.entry.data.group;
    const group = groups.find((existing) => existing.title === title);
    if (group) group.docs.push(doc);
    else groups.push({ title, docs: [doc] });
  }
  return groups;
}

/** getStaticPaths for a language's docs: index is the docs' root. */
export async function docPaths(lang: Lang) {
  const docs = await getDocs(lang);
  return docs.map((doc, index) => ({
    params: { slug: doc.id === 'index' ? undefined : doc.id },
    props: { doc, docs, prev: docs[index - 1], next: docs[index + 1] },
  }));
}

