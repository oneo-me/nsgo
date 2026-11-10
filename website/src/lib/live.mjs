// Live previews: code blocks of the docs that the site shows running
// above themselves, marked in their info string.
//
//   ```go preview=button/basic height=240
//       a demo of examples/previews, the component previews' one app,
//       named in its launch parameter demo
//   ```go program height=480
//       a whole app.go, which scripts/build-live.mjs builds on its own
//
// height is the preview's in CSS pixels. The previews' build is in
// public/previews/ and each program's in public/programs/<id>/, its id
// the hash of its code, so a page's languages share a program whose code
// they share.
import { createHash } from 'node:crypto';

export const defaultHeight = 160;

// parseLive reads a code block's mark from its meta, or returns null for
// a block without one.
export function parseLive(meta) {
  const fields = Object.fromEntries(
    (meta ?? '')
      .split(/\s+/)
      .filter(Boolean)
      .map((field) => {
        const at = field.indexOf('=');
        return at < 0 ? [field, ''] : [field.slice(0, at), field.slice(at + 1)];
      }),
  );
  const height = Number(fields.height) || defaultHeight;
  if (fields.preview) return { kind: 'preview', demo: fields.preview, height };
  if ('program' in fields) return { kind: 'program', height };
  return null;
}

// programId names a program's build after its code.
export function programId(code) {
  return createHash('sha256').update(code).digest('hex').slice(0, 12);
}

// livePreviews puts an empty box before each marked block, which the
// docs' pages mount the block's app in (src/scripts/live.ts). A Sätteri
// mdast plugin.
export function livePreviews({ base }) {
  return () => ({
    name: 'nsgo-live-previews',
    code(node, ctx) {
      if (node.lang !== 'go') return;
      const live = parseLive(node.meta);
      if (!live) return;
      const app = live.kind === 'preview' ? 'previews/' : `programs/${programId(node.value)}/`;
      const demo = live.kind === 'preview' ? ` data-demo="${live.demo}"` : '';
      ctx.insertBefore(node, {
        type: 'html',
        value: `<div class="live live--${live.kind}" data-live="${base}${app}"${demo} style="--live-height: ${live.height}px"></div>`,
      });
    },
  });
}
