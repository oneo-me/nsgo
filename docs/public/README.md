# Public docs

The guides the website publishes at <https://oneo-me.github.io/nsgo/docs/>,
written for people building apps with NSGo. They read as they are on
GitHub; `website/` builds them into the site.

- `en/` and `zh/` mirror each other path for path. A page missing in
  Chinese shows the English one there, with a notice.
- `cases/` holds the apps the website's Showcases page lists, one Markdown
  document each; it is not part of the docs' navigation.
- Front matter gives each page its `title`, an optional `description`, its
  sidebar `group`, in the page's language, and its `order` in the sidebar.
- `index.md` is the docs' first page. Link between pages with relative
  `.md` links (`[Testing](testing.md#run-on-a-platform)`); the site rewrites
  them.
- The layout shows the title and description, so a page's body starts at
  `##`.

How the engine works, and why, is in [ARCHITECTURE.md](../../ARCHITECTURE.md)
and `docs/architecture/`, which stay internal.
