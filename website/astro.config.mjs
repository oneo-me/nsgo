import { defineConfig } from 'astro/config';
import react from '@astrojs/react';

// The site is deployed under the repository path on GitHub Pages. The home
// page is a client-only React island; docs pages are static Astro pages.
export default defineConfig({
  site: 'https://oneo-me.github.io',
  base: '/nsgo/',
  integrations: [react()],
  markdown: {
    // Shiki's CSS-variables theme lets docs.css reuse the site's code palette.
    shikiConfig: { theme: 'css-variables' },
  },
  vite: {
    build: {
      // The gallery's wasm and its gzip copy are generated into public/ by
      // scripts/build-gallery.mjs; Vite only copies them.
      chunkSizeWarningLimit: 700,
    },
  },
});
