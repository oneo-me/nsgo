import { existsSync } from 'node:fs';
import { defineConfig } from 'astro/config';
import { satteri } from '@astrojs/markdown-satteri';
import { docLinks } from './src/lib/doc-links.mjs';
import { livePreviews } from './src/lib/live.mjs';

const base = '/';

// GitHub Pages answers a folder of public/ with its index.html, as at
// /gallery/; the dev server does not, so it is taught to.
const publicIndexes = {
  name: 'public-indexes',
  configureServer(server) {
    const publicDir = server.config.publicDir;
    server.middlewares.use((req, res, next) => {
      const url = new URL(req.url ?? '/', 'http://localhost');
      const dir = url.pathname.replace(/\/$/, '');
      if (dir && existsSync(`${publicDir}${decodeURIComponent(dir)}/index.html`)) {
        if (!url.pathname.endsWith('/')) {
          res.writeHead(301, { Location: `${dir}/${url.search}` }).end();
          return;
        }
        req.url = `${dir}/index.html${url.search}`;
      }
      next();
    });
  },
};

// The site is deployed at the root of nsgo.dev on GitHub Pages. Every
// page is static; English is at the base and Chinese under zh/. The docs
// are the repository's docs/public/, whose marked code blocks run above
// themselves.
export default defineConfig({
  site: 'https://nsgo.dev',
  base,
  trailingSlash: 'always',
  devToolbar: { enabled: false },
  vite: { plugins: [publicIndexes] },
  markdown: {
    // Shiki's CSS-variables theme lets the site's tokens color code.
    shikiConfig: { theme: 'css-variables' },
    processor: satteri({ mdastPlugins: [docLinks({ base }), livePreviews({ base })] }),
  },
});
