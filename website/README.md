# NSGo website

The home page at <https://oneo-me.github.io/nsgo/>: React, TypeScript and
Vite. Its live gallery is the gallery's web build, compiled by `nsgo` and
loaded in an iframe behind a progress-reporting loader.

```sh
bun install
bun run dev       # build public/gallery/, then serve the site with HMR
bun run build     # typecheck, build public/gallery/ and dist/
bun run preview   # serve dist/
bun run gallery   # rebuild only the gallery artifacts
```

`scripts/build-gallery.mjs` runs `go tool nsgo build -target web
examples/gallery` from the repository root. It writes `public/gallery/`,
which is generated and ignored: `loader.html`, the content-named wasm, a
`.gzip` copy, `manifest.json` and the resources. The loader reports its
download, decompression, compilation and start progress to the page over
`postMessage`; the page's stage shows it and starts the gallery by itself
when it enters view and the visitor's connection allows it.

The theme follows the gallery's (`examples/gallery-ui/theme.go`): a warm charcoal
page, a warm paper light theme, one orange accent and flat surfaces. Hairlines
and type carry depth; there are no gradients or top highlights, and only the
stage window and the mobile menu cast a shadow.

The site is one page. Its sections are in `src/App.tsx`, copy in both
languages is in `src/copy.ts` (add English and Chinese together), and the
tokens and layout are in `src/styles.css`. Chinese headings, titles and
kickers omit the trailing `。`; body sentences keep it.

The [Website workflow](../.github/workflows/website.yml) builds `website/dist`
and publishes it to GitHub Pages. `vite.config.ts` uses a relative base, so
the same build works at a domain root and under `/nsgo/`.
