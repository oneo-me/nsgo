# NSGo website

The site at <https://oneo-me.github.io/nsgo/>: Astro, with a React home page
and a Markdown docs section. The home page is a client-only React island; docs
pages are static Astro pages. Its Gallery links open the gallery's web build,
compiled by `nsgo`, in a new tab.

```sh
bun install
bun run dev       # build public/gallery/, then serve the site at /nsgo/
bun run build     # build the gallery, typecheck, then build dist/
bun run preview   # serve dist/
bun run gallery   # rebuild only the gallery artifacts
```

`astro.config.mjs` fixes the base at `/nsgo/` for GitHub Pages, so the dev
server also runs at <http://localhost:4321/nsgo/>. A build for another path
needs that value changed.

`src/pages/index.astro` renders `src/App.tsx` through `src/Home.tsx` as a
client-only island, so the home keeps its own theme and language state. Its
sections are in `src/App.tsx`, copy in both languages is in
`src/copy.ts` (add English and Chinese together), and the tokens and layout are
in `src/styles.css`. Chinese headings, titles and kickers omit the trailing
`。`; body sentences keep it.

Docs are Markdown in `src/content/docs/`, one file per page:

```md
---
title: Install NSGo
description: One sentence for search and the page lead.
order: 10
---

## First step
```

The layout renders the title and description, so start the body at `##`.
`order` sorts the sidebar. Link between docs with site URLs
(`/nsgo/docs/...`) or anchors; relative `.md` links are not rewritten. Docs are
English-only for now; the language switch stays on the home page. Their styles
are in `src/styles/docs.css`.

`scripts/build-gallery.mjs` runs `go tool nsgo build -target web
examples/gallery` from the repository root. It writes `public/gallery/`,
which is generated and ignored: `loader.html`, the content-named wasm, a
`.gzip` copy and the resources. `loader.html` reports its download,
decompression, compilation and start progress, then runs the gallery full
page; it follows the theme stored for the site.

The theme follows the gallery's (`ui/theme.go`): a warm charcoal
page, a warm paper light theme, one orange accent and flat surfaces. Hairlines
and type carry depth; there are no gradients or top highlights, and only the
mobile menu casts a shadow.

The [Website workflow](../.github/workflows/website.yml) builds `website/dist`
and publishes it to GitHub Pages.
