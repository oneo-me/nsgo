import { defineConfig } from 'astro/config';
import { satteri } from '@astrojs/markdown-satteri';
import { docLinks } from './src/lib/doc-links.mjs';
import { livePreviews } from './src/lib/live.mjs';

const base = '/nsgo/';

// The site is deployed under the repository path on GitHub Pages. Every
// page is static; English is at the base and Chinese under zh/. The docs
// are the repository's docs/public/, whose marked code blocks run above
// themselves.
export default defineConfig({
  site: 'https://oneo-me.github.io',
  base,
  trailingSlash: 'always',
  devToolbar: { enabled: false },
  markdown: {
    // Shiki's CSS-variables theme lets the site's tokens color code.
    shikiConfig: { theme: 'css-variables' },
    processor: satteri({ mdastPlugins: [docLinks({ base }), livePreviews({ base })] }),
  },
});
