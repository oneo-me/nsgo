import { defineConfig } from 'astro/config';
import { satteri } from '@astrojs/markdown-satteri';
import { docLinks } from './src/lib/doc-links.mjs';

const base = '/nsgo/';

// The site is deployed under the repository path on GitHub Pages. Every
// page is static; English is at the base and Chinese under zh/. The docs
// are the repository's docs/public/.
export default defineConfig({
  site: 'https://oneo-me.github.io',
  base,
  trailingSlash: 'always',
  markdown: {
    // Shiki's CSS-variables theme lets the site's tokens color code.
    shikiConfig: { theme: 'css-variables' },
    processor: satteri({ mdastPlugins: [docLinks({ base })] }),
  },
});
