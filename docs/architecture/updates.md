# Updates

An app updates the way its platform and the channel it reached the user through do: package `updater` asks, the platform's own updater answers, and `nsgo build` decides from `nsgo.yaml`'s `updates` what each package carries. Read this before changing `updater`, `internal/platform/updates`, a backend's `updates.go` or the tool's ([tool](tool.md#updates)).

NSH's `nsh/updater` downloads, verifies and swaps in the next version itself. NSGo leaves that to Sparkle, WinSparkle and the stores, with their windows (decided by the user, 2026-11-10): they handle installs and restarts better than an app's own code would, and stores forbid self-updating apps anyway.

## Requests and answers

`CheckForUpdates` is the core's part of `updater.Check` and `CheckInBackground`: `ItemCheckForUpdates`, in the background when `On`, which the platform answers with `EventUpdate` ([services](services.md#requests-and-answers)).

- Where the platform's updater shows what it found in windows of its own, Sparkle's or WinSparkle's, the answer is `Finished` and `updater` shows nothing.
- Elsewhere it carries a `driver.Update`: a newer version and its page, none, or why the check failed. `updater` says it in an alert in its catalog's words and opens the page with `OpenURL`.
- Web, mini programs and watchOS never answer: a page loads its newest version, and a watch app follows its iPhone app.

| Target | Updater |
| --- | --- |
| macOS | Sparkle's `SPUStandardUpdaterController`, started when Info.plist names a feed. Installing, it quits the app as `Quit` does |
| Windows | WinSparkle from beside the executable; its shutdown request closes the main window as `Quit` does |
| Linux, Android | `internal/platform/updates` reads the target's appcast with the backend's HTTP client, verifies the package's SHA-256 and Ed25519 signature, then `updater` stages it and hands it to the platform installer |
| iOS | The App Store's lookup by the bundle's ID in the locale's country, against `CFBundleShortVersionString` |

## Schedule

While `updates.automatic` is on, Sparkle and WinSparkle check daily on their own schedule, where the user's choice in their settings wins over `nsgo.yaml`'s.

Linux, Android and iOS have no such schedule, so `nsgo.Run` calls `updater.CheckDaily` as the app starts and whenever it becomes active again, since a phone's app stays in memory for days. It checks in the background unless one ran within the day, so *Later* lasts the day across restarts, and tries a failed check again an hour later. The time lives in a storage partition of its own (`nsgo.updater`), which the app's `Storage` neither lists nor clears.

## Channels

One platform reaches users through several channels, and the channel, not the platform, decides who updates the app. `updates.channel`, or `nsgo build -channel`, names it per build:

- `direct`, the default with a key: the app's own downloads update themselves. macOS and Windows packages carry Sparkle and WinSparkle, and each release writes signed appcasts.
- `store`: a store or package manager updates the app. Packages carry no updater and no appcast; iOS still asks the App Store, which needs no key.
- `none`: nothing checks, for packagers who repackage the app.

A store bans an app that carries a self-updater or sends its users to other downloads, so a store's build must be a separate one, never a direct build uploaded as it is. Downloads that a package manager fetches unchanged from the release, as Homebrew casks and winget manifests do, stay direct: Sparkle and WinSparkle update them, the cask says `auto_updates true`, and winget compares the `DisplayVersion` the installer writes.

| Platform | Direct | Store |
| --- | --- | --- |
| macOS | GitHub or a website, Homebrew casks: Sparkle | The Mac App Store, from its sandboxed package |
| Windows | GitHub or a website, winget: WinSparkle | The Microsoft Store, from the MSIX; a Store listing of the setup may keep WinSparkle |
| Linux | GitHub or a website: the release's page | Distribution repositories and the AUR (`none`), Flathub from the Flatpak manifest, and Snap |
| Android | GitHub or a website: the release's page | Google Play from the App Bundle, and other stores |
| iOS, tvOS | — | The App Store; TestFlight checks nothing |

## Who installed the app

A build cannot know every channel it ends up in, so where the system says who installed the app, the backend checks that first, as the app starts, and leaves an app a store installed to that store (`Config.Store`): its checks answer with that store as the reason and the daily schedule never runs.

| Target | Sign | Store |
| --- | --- | --- |
| Android | The installing package, which the runtime passes as `NSGO_INSTALLER` | Any but the system's package installer and adb, so an unknown installer counts as a store |
| Linux | `/.flatpak-info`, `SNAP` | Flatpak, Snap |
| iOS | A receipt named `sandboxReceipt` | TestFlight and development builds on a device, whose versions the App Store does not list; the simulator still asks |

Linux cannot tell a distribution's package from a downloaded one, so its packagers build with `-channel none`. macOS and Windows need no sign: their store packages are separate builds without an updater.

## Limits

- The direct channel reads `releases/latest/download/` by default, so every release must carry the appcast of each target it updates: a target a release leaves out answers 404 until one carries it again.
- WinSparkle saves `updates.automatic` as the user's choice on the first start, so a later change in `nsgo.yaml` reaches only new installs.
- An app that closes its main window into the tray keeps running through WinSparkle's shutdown request; the installer then stops it.
- Linux installs the appcast's Debian or Arch package through `pkexec`; Android hands its APK to `mobile.InstallPackage`, which asks the user and keeps store installs out of the direct channel.

## Verification

- `updater`'s alerts and schedule are tested with `nsgotest`; the appcast reader, version order, App Store answers and the installer rules run headless, and the tool's tests cover channels, flags and signed appcasts.
- End to end against a local appcast (2026-11-11), with a copy of the tool that took an `http` feed, Sparkle on macOS downloaded 0.2.0 in the background and installed it, ad hoc signed, as 0.1.0 quit.
- WinSparkle on Windows 11 x64 offered 0.2.0, verified the download, ran the setup with `/S /RELAUNCH` and started 0.2.0 (2026-11-11).
- Linux under Wayland offered 0.2.0 in `updater`'s alert as the app started, and with `SNAP` set asked nothing. iOS's check reaches the App Store from the simulator, and a development build on an iPhone, whose receipt is `sandboxReceipt`, checked nothing.
- Android's emulators, API 29 and 34, checked an APK installed by adb or the package installer and left one installed as Google Play's alone; Android refuses cleartext, so an appcast has not been read there.
- Direct Linux and Android packages now download only after the user chooses Update, verify the appcast's length, SHA-256 and Ed25519 signature, and stage the file atomically before installation; Linux restarts after its package manager succeeds.
- Not run yet: a Developer ID or notarized macOS update, Windows on arm64, a Flatpak, a TestFlight build, Android reading an `https` appcast, and a native test that installs an update.
