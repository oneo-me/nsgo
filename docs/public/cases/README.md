# Cases

The apps the website's [Showcases](https://oneo-me.github.io/nsgo/showcases/)
page lists, one Markdown document per app, in the order of their names.
The site reads every file here but this one.

To add yours, copy a case and change it: its name as `#`, a paragraph on
what the app does, a list of its details, and a screenshot beside it in
this folder.

```md
# Your app

What it does, in one sentence.

- platforms: iOS, macOS, Web
- site: https://example.com
- source: https://github.com/you/your-app
- author: Your name

![What the screenshot shows](your-app.jpg)
```

`platforms` lists the site's platform names, comma-separated: `iOS`,
`App Clips`, `tvOS`, `watchOS`, `macOS`, `Android`, `Windows`, `Linux`,
`Web`, `WeChat`, `QQ`, `Alipay` and `Douyin`. `site`, `source` and `author`
are optional; a screenshot is not, and belongs beside the document with a
reasonably small JPEG, PNG or WebP. A pull request with the document and
the image is all it takes.
