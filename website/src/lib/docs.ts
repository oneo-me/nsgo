import { getCollection, type CollectionEntry } from 'astro:content';

export type Doc = CollectionEntry<'docs'>;

export async function getDocs(): Promise<Doc[]> {
  const docs = await getCollection('docs');
  return docs.sort(
    (a, b) => a.data.order - b.data.order || a.data.title.localeCompare(b.data.title),
  );
}

export function docUrl(id: string): string {
  return `${import.meta.env.BASE_URL}docs/${id}/`;
}

export function docNeighbours(docs: Doc[], id: string): { prev?: Doc; next?: Doc } {
  const index = docs.findIndex((doc) => doc.id === id);
  if (index === -1) return {};
  return { prev: docs[index - 1], next: docs[index + 1] };
}
