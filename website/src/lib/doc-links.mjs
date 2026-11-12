// Rewrites the relative .md links between docs/public's pages, which work
// on GitHub, into the site's URLs: `testing.md#run` on a Chinese page
// becomes /zh/docs/testing/#run. A Sätteri mdast plugin.
import path from 'node:path';
import { fileURLToPath } from 'node:url';

export function docLinks({ base }) {
  return (context) => {
    const file = context.fileURL ? fileURLToPath(context.fileURL) : '';
    const match = file.split(path.sep).join('/').match(/docs\/public\/(en|zh)\/(.*)\.md$/);
    if (!match) return null;
    const [, lang, id] = match;
    return {
      name: 'nsgo-doc-links',
      link(node, ctx) {
        const href = node.url;
        if (/^[a-z]+:|^\/|^#/i.test(href)) return;
        const [target, hash] = href.split('#');
        if (!target.endsWith('.md')) return;
        const linked = path.posix.normalize(path.posix.join(path.posix.dirname(id), target)).replace(/\.md$/, '');
        const page = linked === 'index' ? 'docs/' : `docs/${linked.replace(/\/index$/, '')}/`;
        ctx.setProperty(node, 'url', `${base}${lang === 'en' ? '' : `${lang}/`}${page}${hash ? `#${hash}` : ''}`);
      },
    };
  };
}
