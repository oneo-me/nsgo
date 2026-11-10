# Components

The component libraries the website's [Components](https://oneo-me.github.io/nsgo/components/)
page lists: NSGo's own add-ons and the community's, one Markdown document
each, in the order of their names. The site reads every file here but this
one.

To add yours, copy the Kubai Icons document and change it: its name as
`#`, a paragraph on what it gives an app, a list of its details, and a
screenshot beside it in this folder.

```md
# Your component

What it gives an app, in one sentence.

- kind: Icons
- package: github.com/you/your-component
- site: https://example.com
- source: https://github.com/you/your-component
- author: Your name
- official: true

![What the screenshot shows](your-component.jpg)
```

`kind` is a short word for what it provides, such as `Icons`, `Charts` or
`Widgets`, and `package` is the Go import path apps `go get`; both are
required. The screenshot answers what a reader gets, such as the
library's gallery or a view using it. `site`, `source`, `author` and
`official` are optional; `official: true` marks one NSGo maintains. The
screenshot belongs beside the document as a reasonably small JPEG, PNG or
WebP. A pull request with the document and the image is all it takes.
