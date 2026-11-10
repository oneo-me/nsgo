# NSGo website

The site at <https://oneo-me.github.io/nsgo/>: static Astro pages in English
and Chinese, with no UI framework, and the docs built from the repository's
`docs/public/`, whose marked code blocks run in the page. Its Live demo
links open the gallery's web build, compiled by `nsgo`, full page in a new
tab.

```sh
bun install
bun run dev          # build the gallery and the docs' apps, then serve the site at /nsgo/
bun run build        # build them, typecheck, then build dist/
bun run preview      # serve dist/
bun run gallery      # rebuild only the gallery
bun run live         # rebuild only the docs' apps
bun run screenshots  # retake the home page's screens of the gallery
```

`astro.config.mjs` fixes the base at `/nsgo/` for GitHub Pages, so the dev
server also runs at <http://localhost:4321/nsgo/>. A build for another path
needs that value changed.

## Pages

| Page | Path | Published |
| --- | --- | --- |
| Home | `/` | Yes |
| Docs | `/docs/` | Yes |
| Showcases | `/showcases/` | Yes |
| Benchmarks | `/benchmarks/` | Dev only, a placeholder |

`src/site.ts` lists the pages; one marked `dev` is built, linked from the
header, the footer and the home page's next steps, and marked with a dot
only by the dev server. Publishing a page means filling it and dropping
`dev`.

Each page but the docs is a file in `src/pages/[...lang]/` whose
`getStaticPaths` is `langPaths(id)`: English at the base, Chinese under
`zh/`. A first visit from a Chinese browser to an English page goes to its
Chinese twin, and the language menu remembers the choice in `nsgo-lang`.

Every page's words are in `src/copy.ts`; add English and Chinese together.
Chinese headings, titles and kickers omit the trailing `。`; body sentences
keep it. `src/layouts/Layout.astro` gives every page the header, the call to
action and the footer.

`scripts/web-build.mjs` builds an app with `go tool nsgo build -target web`
from the repository root into a folder of `public/`, keeping its wasm only
as a `.gzip` copy. `scripts/build-gallery.mjs` writes `public/gallery/`
with `loader.html`, which reports its download, decompression, compilation
and start progress and follows the site's appearance. All of `public/`'s
builds are generated and ignored.

## Showcases

The Showcases page lists apps built with NSGo, read from the Markdown
documents in `docs/public/cases/`. Each document is one app: a `#` name, a
paragraph about it, a list of `platforms`, `site`, `source` and `author`
details, and a screenshot beside it. The folder's README says how to write
one and the page's call to action opens it, so a showcase is one document
and one image in a pull request. `src/lib/cases.ts` parses them and fails the
build with the document's line when one breaks that shape.

## Docs

The docs are the Markdown in the repository's `docs/public/`, whose README
says how to write them. `src/content.config.ts` loads `en/` and `zh/`;
`src/pages/docs/` and `src/pages/zh/docs/` render every English page in each
language, the Chinese one where it exists and the English one with a notice
where it does not (`src/lib/docs.ts`). `src/lib/doc-links.mjs`, a Sätteri
plugin, turns the relative `.md` links between pages into the site's URLs.
`src/lib/live.mjs` puts a box before each code block marked
`preview=<demo>` or `program` (`docs/public/README.md`), and
`scripts/build-live.mjs` builds what the boxes run: `examples/previews`,
one app for every component example, into `public/previews/`, and each
program block's code into `public/programs/<hash>/`, so a program whose
code a translation shares is built once. `src/scripts/live.ts` mounts a
box's app through the tool's loader once it nears the window, compiling
each wasm once a page, in the site's appearance, which it follows, and the
page's language. A Go program cannot be stopped, so leaving a page whose
apps started loads the next page whole.
A page has the sidebar of its language's groups, under the pages without one, its contents, the previous
and next pages, an edit link and copy buttons on its code; the styles are in
`src/styles/docs.css`.

## Look

White by default; the header's appearance menu chooses light, dark or the
system's, stored in `nsgo-theme`, which the gallery's loader also reads. The mark gives the
site its look: its blue gradient (`#4da3ff` to `#007aff`) for the primary
buttons, the lit platform, the engine and the call to action, and its skewed speed lines
(`SpeedLines.astro`) before section names and behind the hero. Cards are
soft and rounded, code takes GitHub's light or dark colors with the
appearance, and behind the hero a WebGL shader (`HeroField.astro`) drifts
faint blue through its dot grid, still when less motion is asked for. Real
screens of the gallery show what NSGo makes. The tokens and layout are in
`src/styles/global.css`.

The home page's screens are `src/assets/screens/`, the gallery's desktop
and phone layouts in English and Chinese. `bun run screenshots` retakes
them from `public/gallery/` (run `bun run gallery` first) with Google
Chrome and macOS's `sips`; retake them when the gallery's look changes.

## Docs: goals

- **Versions from 1.0.** Until 1.0 the docs show `main`. From 1.0 each
  minor release keeps its docs, built from its release tag so
  `docs/public/` stays the only copy: `/docs/` is the latest release,
  `/docs/1.2/` an older one and `/docs/next/` `main`, with a version switch
  that keeps the page when it exists.
- **Translations kept current.** The build lists Chinese pages that are
  missing or older than the English.
- **Search** across a language and version, built with the site.
- **Checks.** The build fails on a broken link or anchor, and code blocks
  marked as programs compile through `go tool nsgo check`.

## Deployment

The [Website workflow](../.github/workflows/website.yml) builds `website/dist`
and publishes it to GitHub Pages.
