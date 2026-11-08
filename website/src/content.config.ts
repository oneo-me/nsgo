import { defineCollection } from 'astro:content';
import { glob } from 'astro/loaders';
import { z } from 'astro/zod';

// The public docs are the repository's docs/public/, a folder per language
// (en, zh) mirroring each other path for path, readable on GitHub as they
// are. An entry's id is its path without .md, such as en/getting-started.
const docs = defineCollection({
  loader: glob({ base: '../docs/public', pattern: '{en,zh}/**/*.md' }),
  schema: z.object({
    title: z.string(),
    description: z.string().optional(),
    /** The sidebar's group, in the page's language. */
    group: z.string(),
    order: z.number(),
  }),
});

export const collections = { docs };
