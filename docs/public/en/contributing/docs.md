---
title: Docs and the website
description: Write a page of these docs, translate it, and run the website.
group: Contributing
order: 83
---

## A page

These docs are the Markdown in `docs/public/`, readable on GitHub as they are. `en/` and `zh/` mirror each other path for path; a page missing in Chinese shows the English one with a notice. Each page starts with front matter:

```yaml
---
title: Testing
description: Test an app the way a user uses it, headless or in a native window.
group: Toolchain
order: 21
---
```

- `group` is the page's group in the sidebar, in the page's language, and `order` places it. The groups follow a reader's way in: tutorials, the toolchain, components, core concepts, then writing components and contributing. A page without a group, such as the introduction, comes before them.
- The layout shows the title and the description, so the body starts at `##`.

Link between pages with relative `.md` links (`[Testing](testing.md#run-on-a-platform)`); the site turns them into its URLs. Each page answers one question; how NSGo works inside stays in `docs/architecture/`.

## Showcases

The Showcases page lists the apps in `docs/public/cases/`, a Markdown document and a screenshot each; the folder's README says how to write one.

## The website

The website is an Astro site in `website/`. It needs [Bun](https://bun.com) 1.4 or later:

```sh
cd website
bun install
bun run dev    # at http://localhost:4321/nsgo/
bun run build  # type-check and build, as the deployment does
```

`website/README.md` says how the site is built and published.
