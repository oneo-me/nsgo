---
title: Platform services
description: Alerts, the clipboard, files, photos, sharing, storage, speech, networking and the app's lifecycle.
group: Platform features
order: 44
---

## Alerts and the clipboard

`cx.ShowAlert` shows the platform's alert or action sheet. Its `Actions` come back by index, and a role draws the cancel and destructive buttons the platform's way.

```go
cx.ShowAlert(core.AlertConfig{
	Title:   "Delete the file?",
	Message: "This cannot be undone.",
	Actions: []core.AlertAction{
		{Label: "Cancel", Role: core.AlertCancel},
		{Label: "Delete", Role: core.AlertDestructive},
	},
}, func(index int) {
	if index == 1 {
		remove()
	}
})
```

`SetClipboard` writes text and `SetClipboardImage` an image. `ReadClipboard`, `ReadClipboardImage` and `ReadClipboardFiles` answer asynchronously, and `HasClipboardText` and `HasClipboardImage` read what the clipboard holds without a paste notice.

```go
cx.ReadClipboard(func(text string) { pasted.Set(text) })
cx.SetClipboard("Hello")
```

## Opening links and files

`cx.OpenURL` opens a URL in the system's browser or app, and `core.CheckURL` says what it refuses: no scheme, over 16 KiB, spaces, a web address without a host, and `javascript:`, `data:`, `blob:`, `file:`, `about:` and `asset:`.

```go
if err := core.CheckURL(url); err == nil {
	cx.OpenURL(url, func(opened bool) {
		if !opened {
			status.Set("No app opens that link.")
		}
	})
}
```

`cx.OpenFile` opens a file the app has in the app associated with its type, and `cx.Share` shows the system's share sheet for files, text or both. A `core.SharedFile` is a path, with the name and type the other app sees.

```go
cx.Share(core.ShareOptions{
	Files: []core.SharedFile{{Path: path, Name: "Notes.txt"}},
	Text:  "A note from NSGo",
}, func(shown bool, err error) {})
```

## Photos and files

`cx.PickImages` shows the system's photo picker and answers with `core.PickedFile`s, whose bytes, name, type and pixel size the app reads. `cx.TakePhoto` shows the camera where the platform has one; `cx.Env().CanTakePhoto` says whether it does.

```go
cx.PickImages(core.PickOptions{Limit: 1, MaxDimension: 2048}, func(files []core.PickedFile) {
	if len(files) == 1 {
		avatar.Set(files[0].Data)
	}
})
```

`cx.PickFiles` picks files of any type, `cx.PickDirectories` picks folders as an `fs.FS`, and `cx.SaveFile` saves a file or bytes through the system's save dialog. `cx.PickScreenColor` picks a color anywhere on the screen where the platform allows it.

## Storage

`cx.Storage()` is the app's key-value storage, kept on the device and shared by every graph of the app. Reads are synchronous and read inside a computation subscribe it to changes; `SetItem` returns a `core.StorageFullError` past the quota (`core.StorageQuota`, 5 MiB by default).

```go
store := cx.Storage()
if token, ok := store.GetItem("token"); ok {
	login(token)
}
store.SetItem("token", token)
```

`store.Keys()` lists the keys, `RemoveItem` deletes one and `Clear` empties the storage. `cx.DataDir()` and `cx.CacheDir()` give the app's folders for files the key-value storage is too small for; a `Scope.ProvideStorage` gives a subtree a partition of its own.

## Networking

Go's own `net/http` makes requests on every target, with the system's TLS and proxy settings. Signals belong to the engine's goroutine, so a response from another goroutine reaches them through `cx.Post`.

```go
go func() {
	resp, err := http.Get("https://api.example.com/items")
	if err != nil {
		cx.Post(func() { errText.Set(err.Error()) })
		return
	}
	defer resp.Body.Close()
	var items []Item
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		cx.Post(func() { errText.Set(err.Error()) })
		return
	}
	cx.Post(func() { list.Set(items) })
}()
```

`cx.Post` runs its function on the graph's goroutine, where signals may be set. On Android the manifest asks for the internet permission, and image downloads cache themselves on disk.

`core.NewResource` does the same for a value loaded from a source. Its fetcher runs on a goroutine of its own and loads again when the source changes:

```go
items := core.NewResource(cx, func() (string, bool) { return query.Get(), query.Get() != "" },
	func(ctx context.Context, q string, _ core.ResourceInfo) ([]Item, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.example.com/items?q="+url.QueryEscape(q), nil)
		if err != nil {
			return nil, err
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		var list []Item
		if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
			return nil, err
		}
		return list, nil
	})
```

`items.Get()`, `items.Err()` and `items.Loading()` are signals that bindings follow. A new query cancels the request still running through `ctx`, so a late answer never replaces a newer one. `items.Refetch()` retries, keeping the last list while it loads, and `items.Mutate` sets a value at once for an optimistic update.

## Speech and the lifecycle

`cx.StartSpeech` recognizes speech with the system's recognizer where `cx.Env().CanRecognizeSpeech()` says it exists, and answers with the transcript as it changes.

```go
session, err := cx.StartSpeech(core.SpeechOptions{
	Lang:    "en-US",
	Interim: true,
	OnResult: func(text string, final bool) {
		transcript.Set(text)
	},
})
if err == nil {
	defer session.Stop()
}
```

`cx.Env().Lifecycle()` says whether the app is active, inactive or in the background, and `cx.OnQuit` cleans up as it quits, however it does, with 200 ms for its work. `cx.SetAppearance` forces light or dark, `cx.SetLocale` chooses the language, `cx.OnKey` binds a keyboard shortcut, `cx.RegisterHotKey` binds one while any app is frontmost, and `cx.OnOpenFiles` hears the documents a macOS app opens.

```go
cx.OnKey("mod+k", func() { palette.Set(true) })
cx.SetLocale("de")
cx.OnQuit(func(ctx context.Context) { saveDraft(ctx) })
```

## Biometrics

`cx.BiometricAvailability` reports whether Face ID, Touch ID or a device fingerprint is enrolled. `cx.AuthenticateBiometric` asks the system to confirm the user with a reason and reports success, failure, cancellation or unavailability. It does not choose a passcode fallback for the app.

```go
cx.BiometricAvailability(func(kind core.BiometricKind) {
	if kind != core.BiometricNone {
		cx.AuthenticateBiometric("Confirm deleting this account", func(result core.BiometricResult) {
			if result == core.BiometricSuccess {
				deleteAccount()
			}
		})
	}
})
```

Face ID apps declare `NSFaceIDUsageDescription` under `apple.info`; Android apps list `USE_BIOMETRIC` under `android.permissions`.

## Next

- [Notifications](notifications.md): permission, badges and presses.
- [Forms](forms.md): text fields and the keyboard.
- [Embedding](embedding.md): services a host app gives its embedded app.
- [Building and shipping](building.md): the permissions and files a package needs.
