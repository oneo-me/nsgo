package main

import "regexp"

// The documents the Text View demo shows, adapted from NSH's gallery.

const tvMarkdown = `# NSGo

**NSGo** runs apps written in *Go* and renders them with each platform's ~~web views~~ own views. Press <kbd>⌘K</kbd> to search the docs, or read [the guide](https://github.com/nsgo-dev/nsgo).

## How it works

1. The Go toolchain builds the app for each platform
2. The engine runs it on its own goroutine
   - the reactive graph and the shadow tree are Go
   - layout ports Taffy (flex, grid and block)

- [x] One build tool, no NDK or SDK to install
- [x] Hot reload on save
- [ ] A styled component library (that one is yours)

| Platform | Views | Text |
|:---------|:------|-----:|
| iOS      | UIKit | CoreText |
| Android  | Android views | StaticLayout |
| Linux    | GTK 4 | Pango |

` + "`" + `` + "`" + `` + "`" + `go
func Root(cx *core.Scope) core.Node {
	return core.Text{Content: "Hello, NSGo"}
}
` + "`" + `` + "`" + `` + "`" + `

> The engine never waits for the main thread, and the main thread never waits for the engine.

![Green mountains under low clouds](https://picsum.photos/id/1018/1200/750)

> [!TIP]
> Long-press any paragraph to select and copy it.

> [!WARNING]
> Apps are never rebuilt once delivered, so nothing that reached ` + "`" + `main` + "`" + ` is removed.

A list stays smooth at $60$ frames a second, $16.7\,\text{ms}$ each.

$$
t_{\text{frame}} = \tfrac{1000}{60}\,\text{ms}
$$

<details>
<summary>Why Go</summary>

It builds every target from one machine, runs **native code** with no interpreter, and keeps the app's logic in one language.

</details>

---

Numbers come from the gallery's benchmarks.[^1]

[^1]: Release builds on a mid-range phone and a 2021 laptop.
`

const tvHTML = `<article>
  <header>
    <h1 id="nsgo">NSGo: native UI from Go</h1>
    <p><small>Posted by the NSGo team &middot; 6 min read &middot; <a href="https://github.com/nsgo-dev/nsgo" title="Repository">GitHub</a></small></p>
  </header>

  <section>
    <h2 id="highlights">Highlights</h2>
    <p>
      NSGo    is a <b>Go</b> port of NSH's design:   apps written in Go
      <i>run natively</i>, render with <u>each platform's own views</u>, and the
      <s>NDK</s> <mark>pure Go</mark> toolchain needs nothing else. Hot reload is
      <mark color="yellow">about 120 ms</mark>, and nothing needs installing before it
      <mark style="background-color: #fecaca">runs</mark>.
    </p>
    <p>
      Styles mean what they mean in a browser: <code>Pct(50)</code> or <code>LinearGradient(90, …)</code>, spacing in
      steps of 2<sup>2</sup> pt, and text sets H<sub>2</sub>O as you would expect. Press <kbd>Ctrl</kbd> + <kbd>B</kbd>
      to fold the gallery's sidebar, or run <code>go tool nsgo dev examples/gallery</code> to try it. Our first rule says it
      best: <q>The main thread never waits for the engine</q>. The <abbr title="Garbage collected">GC</abbr>-friendly
      engine behind it is open source.
    </p>
    <figure>
      <a href="https://picsum.photos/id/1015/600/300"><img src="https://picsum.photos/id/1015/320/160" alt="A river valley under clouds" width="320" height="160"></a>
      <figcaption>The gallery on a phone, a desktop and the web, one app. <em>Tap to open it full size.</em></figcaption>
    </figure>
    <figure>
      <img src="https://picsum.photos/id/1036/1200/750" alt="A tent in the snow below mountains" width="320" height="200">
      <figcaption>Where the renderer was written. <em>Press it to zoom in.</em></figcaption>
    </figure>
  </section>

  <section>
    <h2 id="getting-started">Getting started</h2>
    <h3>Step by step</h3>
    <ol start="3">
      <li>Create an app (steps 1 and 2 were installing Go and Xcode)</li>
      <li>Run <b>go tool nsgo dev</b> and choose a target:
        <ul>
          <li><i>Desktop</i>: AppKit, Windows or GTK 4, the default</li>
          <li><i>Mobile</i>: a simulator or a device
            <ol>
              <li>iOS with UIKit views</li>
              <li>or Android with Android views</li>
            </ol>
          </li>
        </ul>
      </li>
      <li>Edit, save, and watch it reload</li>
    </ol>

    <h3>Checklist</h3>
    <ul>
      <li><input type="checkbox" checked> Write the app in Go</li>
      <li><input type="checkbox" checked> Test it headless with nsgotest</li>
      <li><input type="checkbox"> Package it for every store</li>
    </ul>

    <h4>Terms</h4>
    <dl>
      <dt>Host</dt>
      <dd>The native app that embeds an NSGo app.</dd>
      <dt>Package</dt>
      <dd>What <code>nsgo build</code> writes for a target: an app, an APK, a web site.</dd>
    </dl>
  </section>

  <section>
    <h2 id="components">Components</h2>
    <blockquote>
      <p>Nothing NSGo ships has a look of its own: the app's classes and styles draw everything.</p>
      <p>&mdash; <cite>NSGo's design rules</cite></p>
    </blockquote>
    <p>An app is a module that mounts a component:</p>
    <pre><code class="language-go">func Counter(cx *core.Scope) core.Node {
	count := core.NewSignal(cx, 0)
	return core.View{OnPress: func(core.PressEvent) { count.Update(inc) },
		Children: []core.Node{core.Text{ContentFunc: func() string {
			return fmt.Sprintf("Pressed %d times", count.Get())
		}}}}
}</code></pre>
  </section>

  <section>
    <h2 id="performance">Performance</h2>
    <table>
      <thead>
        <tr><th align="left">Operation</th><th align="right">Platform</th><th align="right">Time</th><th style="text-align: center">Items</th></tr>
      </thead>
      <tbody>
        <tr><td>Counter, first frame</td><td align="right">headless</td><td align="right">0.6 ms</td><td align="center">49</td></tr>
        <tr><td>Counter, one tap</td><td align="right">headless</td><td align="right">0.15 ms</td><td align="center">8</td></tr>
        <tr><td>List row while scrolling</td><td align="right">Android</td><td align="right">0.35 ms</td><td align="center">4</td></tr>
      </tbody>
      <tfoot>
        <tr><td colspan="3">Baselines from NSH's docs/dev/performance.md</td><td align="center">&#10003;</td></tr>
      </tfoot>
    </table>
  </section>

  <details>
    <summary>Known issues</summary>
    <p>Zooming scroll views run on iOS and Android only for now. The desktops follow in a later release.</p>
  </details>
  <details open>
    <summary>Thanks</summary>
    <p>
      NSH, the design NSGo ports, is created by <a href="https://github.com/sunli829"><img src="https://avatars.githubusercontent.com/u/20092316?s=48" alt="sunli829" style="width: 20px; height: 20px"></a> <a href="https://github.com/sunli829">@sunli829</a>
      and <a href="https://github.com/huacnlee"><img src="https://avatars.githubusercontent.com/u/5518?s=48" alt="huacnlee" width="20" height="20"></a> <a href="https://github.com/huacnlee">@huacnlee</a>,
      with&nbsp;&hearts;.
    </p>
  </details>
</article>`

const tvExtended = `Thanks @huacnlee and @sunli829 for the review!

` + "`" + `` + "`" + `` + "`" + `chart
Mon 12
Tue 18
Wed 9
Thu 22
Fri 15
` + "`" + `` + "`" + `` + "`" + `

<Callout kind="tip">

Extensions turn the app's own syntax into **custom nodes**.

</Callout>

:::note[Heads up]
Containers wrap Markdown, like *this*.
:::
`

const tvReply = `Here is how to put **an NSGo app** in your iOS app:

1. Build the framework with ` + "`" + `go tool nsgo build -embed` + "`" + `
2. Add the *xcframework* to the project
3. Show the app in its view

| Step | Where |
|------|------:|
| SDK | your Mac |
| View | your app |

` + "`" + `` + "`" + `` + "`" + `sh
go tool nsgo build -target ios_simulator -embed my-app
` + "`" + `` + "`" + `` + "`" + `

That's it. Ping me if anything looks off.`

var mentionPattern = regexp.MustCompile(`@(\w+)`)
