# Public docs

The guides the website publishes at <https://nsgo.dev/docs/>,
for people building apps with NSGo, then for authors of components and
people working on NSGo. They read as they are on
GitHub; `website/` builds them into the site.

- `en/` and `zh/` mirror each other path for path. A page missing in
  Chinese shows the English one there, with a notice.
- `cases/` holds the apps the website's Showcases page lists, one Markdown
  document each; it is not part of the docs' navigation. Its root documents
  are English, and a language's folder (`zh/`) holds their translations.
- `components/` holds the component libraries the website's Components
  page lists, one Markdown document each beside its screenshot; it is not
  part of the docs' navigation. Its root documents are English too, with
  translations in a language's folder.
- Front matter gives each page its `title`, an optional `description`, its
  sidebar `group`, in the page's language, and its `order` in the sidebar.
  Pages without a group, the introduction and the comparison, come first.
  The groups follow a reader's way in: Get started (the setup in
  `getting-started.md`, then the examples in `tutorials/`), Toolchain, Components (`ui/`),
  Core concepts, Writing components (`authoring/`) and Contributing
  (`contributing/`).
- `index.md` is the docs' first page. Link between pages with relative
  `.md` links (`[Testing](testing.md#run-on-a-platform)`); the site rewrites
  them.
- The layout shows the title and description, so a page's body starts at
  `##`.
- A Go block runs above itself on the site when its info string marks
  it: `` ```go preview=button/basic `` shows that demo of
  `examples/previews`, which holds the block's code with what it leaves
  out, and `` ```go program `` builds the block as a whole `app.go`, whose
  `main` the site skips to run its `Root`.
  `height=240` sets the box's height in pixels. GitHub ignores the marks.
  Both languages mark the same blocks; a demo shows Chinese words on
  Chinese pages, and `examples/previews`' tests fail on a mark naming no
  demo.

How the engine works, and why, is in [ARCHITECTURE.md](../../ARCHITECTURE.md)
and `docs/architecture/`, which stay internal.
