# The nsgo tool

`cmd/nsgo` builds, runs, tests and packages apps from their `nsgo.yaml`, shelling out to `go` and, for Apple targets, Xcode's tools. It follows NSH's `nsh-cli` (`nsh/docs/dev/tooling.md` and `embedding.md`). Read this before changing `cmd/nsgo`.

## Output

Every command prints one line per step and the details only when a step fails, as NSH's CLI does (`nsh/crates/nsh-cli/src/ui.rs`, `nsh/docs/dev/tooling.md` "Output"): `✓` a step that finished, `→` where to find something, `•` what a device or server did, `warn` and `error` for problems. The tool's lines are indented two spaces, so the app's own lines stand apart. On a terminal a step that takes a while shows a spinner in its line with what it does at the moment (`stage`: compiling TSX, compiling Go, signing…), and colors are used unless `NO_COLOR` is set; redirected, only the result lines appear. All of it is stderr.

`nsgo build` and `nsgo dev` start with the NSGo version the tool was built from, the app's name and version and the targets, then what they do. `nsgo build` times each package and measures its directory, so each target's line reads `✓ built macos_arm64 (1.4s, 23.4 MB)`, numbered as `[2/5]` in a build of several targets, which ends with their number, time and total size. `nsgo dev` reports each build's time, the app's start and whether it watches for changes, which it does unless `dev.hot_reload` is false. A tool's output, Go's or Xcode's, appears indented under `error` only when it fails.

## Dev loop

`nsgo dev` (`dev.go`) starts a dev server on `-port` (8080), or the next free port, listening on every interface. Dev builds take the `nsgodev` tag, which links `nsgo`'s dev client (`dev_client.go`): it follows the server's `/_nsgo/events` and shows a failed build's output in an overlay over the running app, as it shows the app's own render failures; Command or Control+R posts `/_nsgo/reload`.

The server finds the app through `NSGO_DEV_SERVER`, which the simulator gets as `SIMCTL_CHILD_`; Android and iPhones, which pass no environment, get it with `-ldflags -X`, Android at `127.0.0.1` through `adb reverse`, removed when the run ends, an iPhone at the Mac's LAN address.

The web target is served from the same server; its page shows the overlay itself and loads each build. A run ends with the app, and without hot reload a failed first build ends it. `-release` builds as `nsgo build` does, keeping `dev.fps`'s monitor, for measuring. `-trace` builds with `-X nsgo.devTrace=1`: the dev client sets `engine.Trace` for every engine of the process and posts each commit to `/_nsgo/trace`, printed as `nsgo test -trace` prints a batch (`driver.WriteTrace`), labeled by its number, with the app's output. The web page's Go finds the server at the page's origin and leaves build failures to the page. `-device` picks a simulator by name or UDID, booting it, or an adb serial or an emulator's AVD name.

## TSX apps

The tool finds a package's TSX entries in its `compiler.App(src, "App.tsx")` calls (`bundle.go`), compiles each with `tsx/bundler` from the app's directory and adds the bundles to the package with a file calling `compiler.Bundled`, through `go -overlay`, built with the `nsgo_bundled` tag, which leaves the compiler out. The overlay names the package's directory as `$PWD` spells it and by its real path, since `go` names the files by either. `nsgo dev` compiles hot bundles ([TSX](tsx.md#developing)) and watches the files they compile from apart from the Go ones: a change of them alone compiles the TSX again and sends `bundle <entry>` over `/_nsgo/events`, which `/_nsgo/bundle?entry=` serves, with no Go build; a failed compile shows as a failed build does.

## Catalogs

`i18n.go`, `i18n_message.go`, `i18n_gen.go` and `i18n_build.go` check packages' catalogs and write their Go code ([languages](languages.md#the-tool)): `dev`, `build` and `test` before they compile, `check` without writing, and `nsgo i18n` for one directory. `dev` watches the YAML files of the directories it watches, and the app's catalogs before it imports them.

## Checking

`nsgo check` (`check.go`) checks the catalogs, runs `go vet ./...`, writes the declarations of the modules the app's TSX imports to `.nsgo/types`, with `.nsgo/tsconfig.json`, and runs TypeScript 7's checker (`tsgo`, a pinned version of `github.com/microsoft/typescript-go` that `go run` builds and caches) on the app's `tsconfig.json`, which extends it. The declarations come from the app's own program: the tool builds its main package with a file, added through the overlay, whose `init` runs after the package's others, so after every `tsx.Register`, writes them and exits before `main`. It checks with `go list` that the file is in the package before it builds, so the app itself never runs.

## Tests

`nsgo test` (`test.go`) runs `go test -json` and prints a line per test, with a test running the package's `*.test.tsx` files (`tsxtest.Run`) added through the overlay when it has any, a failure's output under it, a tally, and with `-junit` a JUnit report, a suite per package. `-run` filters; `-trace` sets `NSGO_TEST_TRACE`, under which `nsgotest` prints each batch under its step with its items and the commit's time.

`-target` runs the tests on a platform, a process per test ([testing](testing.md#on-a-platform)): macOS, Windows and Linux on this machine, the web in a headless Chrome, Chromium or Edge it finds (or `NSGO_BROWSER`) over the DevTools protocol (`cdp.go`, on `x/net/websocket`), the simulator through `simctl launch`, and Android through `am start` with the test in an intent extra and its output read back from the device's log.

## Packages

`nsgo build` strips symbols, paths and VCS stamps (`nsgo dev` keeps symbols unless `-release`) and writes a directory per target under `build.output`:

- macOS: `<name>.app`, signed ad hoc. iOS: `<name>.app` and `<name>.ipa`, signed with `sign.profile`, else unsigned. Android: `<name>.apk`, signed with `sign.keystore`, else the project's debug key.
- Windows and Linux: the executable with the resources beside it.
- With `-embed`, for apps of their own to embed ([embedding](embedding.md)): iOS's `<module>.xcframework` (of dynamic frameworks with `-dynamic`) with `<name>Resources`, Android's `<name>.aar`, and no package for other targets.
- Web: `index.html`, `manifest.webmanifest`, `nsgo.json`, which names the rest for the loader, and every other file named by its content (`app.<10 hex>.wasm`, `nsgo.<10 hex>.js`, `web.host`'s module as `host.<10 hex>.js` and each resource), so hosts cache them for good. The page mounts the app with the loader and `nsgo.json`'s content inline ([embedding](embedding.md#on-the-web)).

`resources/` ships at each package's resource root: beside the executable on Windows and Linux, beside the page on the web, in the bundle's `Contents/Resources` on macOS, at the bundle's root on iOS and in the APK's assets on Android. `imageload.ResourceRoot`, the web backend and Android's APK reader resolve `asset:` and font paths against it. App icons are drawn in Go; `actool` compiles Apple's `.icon` files, its output kept in the user cache by its version and the icon's content, and a Windows icon links from a `.syso` in the app's package directory, the only place Go links one from.

`build_number` sets `CFBundleVersion` and `versionCode`, else they follow `version`. `appearance` starts the app light or dark through `-X`, and sets `UIUserInterfaceStyle`, macOS's Aqua appearance, Android's activity theme and the page's `color-scheme`. `-run` starts the package of the first target this machine runs: the app, the iPhone's, the simulator's, the Android device's, or the site served with a CDN's cache headers. `-device` picks the device as `nsgo dev`'s does; Xcode signs the iPhone's package again for `-team`, as in the dev loop, since devices install only development builds.

## Store signing

`sign.profile` signs the iOS package: `security cms` opens the profile, whose `application-identifier` must cover the bundle ID; a wildcard profile signs the app's own ID, which devices require. The profile goes in as `embedded.mobileprovision`, and `codesign` signs with `sign.identity`, else the keychain's identity for one of the profile's certificates.

`sign.keystore` signs release APKs (`apk/keystore.go`): PKCS#12, with PBES2 (PBKDF2 and AES or 3DES) as keytool and OpenSSL 3 write it or PKCS#12's SHA-1 schemes with 3DES and 40-bit RC2 as older tools do, or JKS, whose key protector is a SHA-1 stream of the password. `NSGO_KEYSTORE_PASSWORD` and `NSGO_KEY_PASSWORD` give the passwords, never `nsgo.yaml`; `sign.key_alias` picks a key. APKs sign with RSA keys only.

## Verification

`go test ./cmd/nsgo/apk` checks the APK and, when `apksigner` is installed, its signature.
