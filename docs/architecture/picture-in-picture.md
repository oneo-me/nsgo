# Picture in Picture on iOS

This is the design for issue [#54](https://github.com/nsgo-dev/nsgo/issues/54). It is not an implemented API. iOS has two different Picture in Picture paths and they must not be hidden behind one operation with ambiguous ownership:

- video PiP uses the existing native video player and its timed playback;
- custom-content PiP supplies video frames to `AVPictureInPictureController` and needs a renderer, clock and playback delegate of its own.

The first target is iOS. Other Apple targets report unavailable until they have an equivalent system surface.

## Video PiP

The video path takes a `core.Video` or an explicit video handle, and what starts its PiP depends on how `internal/platform/apple/video.go` shows it:

- a video without `Controls` shows its `AVPlayer` in an `AVPlayerLayer`, so the backend makes an `AVPictureInPictureController` with `initWithPlayerLayer:` and starts and stops it from code with `startPictureInPicture` and `stopPictureInPicture`;
- a video with `Controls` shows an `AVPlayerViewController`, which has no start or stop of its own and does not expose its layer: the user starts PiP with its button, which `allowsPictureInPicturePlayback` shows, and the controller's delegate reports it. Starting it from code would need a second `AVPlayerLayer` on the same player, an open decision; until then a start from code reports it unsupported.

The player must be configured for PiP before the view appears, and the app must declare the playback/background intent that the system requires. The backend reports whether PiP is supported (`isPictureInPictureSupported`), possible (`pictureInPicturePossible`, which changes as the player becomes ready), started, stopped or failed with the delegate's error; calling start while it is already active is idempotent, and a start while it is not possible reports a failure rather than waiting.

The player remains the owner of timing, buffering, subtitles, controls and audio focus. PiP does not copy frames into Go. The PiP window's play, pause and skip act on the `AVPlayer` directly, as an inline video's controls do, while `core.Video` takes playback from the app's `Paused` and `Rate`, which `apple.Video.Configure` sends again on every config change today: an unrelated change such as `Muted` would play a video the user paused in the window. The backend therefore reports the player's own play and pause as an event the app follows with `Paused`, and sends `pause` or `setRate:` only when `Paused` or `Rate` changes; an inline video's controls have the same gap today. How long the player lives in PiP is under [Lifecycle and ownership](#lifecycle-and-ownership).

PiP starts on its own only where the app asks for it. An app setting maps to `canStartPictureInPictureAutomaticallyFromInline` (iOS 14.2, on both controllers), so a playing inline video enters PiP as the app goes to the background. A video with `Controls` also enters PiP by itself when the user leaves the app during full-screen playback while `allowsPictureInPicturePlayback` is set, so the backend sets that property from the app's choice rather than leaving AVKit's default.

When the user returns to the app from the PiP window, AVKit calls `restoreUserInterfaceForPictureInPictureStopWithCompletionHandler:` on the controller's delegate. The backend reports it as an event on the engine goroutine, so the app can show its player page again, and calls the completion handler with true once that event's handler and the commit it leads to are done; it never leaves the handler uncalled. NSGo does not persist a video URL or playback position for the restore.

## Custom content PiP

Custom content is a separate component. An app supplies a fixed-size render surface and a playback model that can produce a `CVPixelBuffer` or sample buffer at the requested cadence. The backend uses `AVPictureInPictureController` with a sample-buffer display layer and `AVPictureInPictureSampleBufferPlaybackDelegate`; it does not capture an arbitrary NSGo view tree or screenshot a window.

The source owns frame production and tells the backend when a frame is ready. The backend drops stale frames under back-pressure, retains at most one pending frame and reports the effective frame interval. The playback delegate maps play, pause, seek and skip commands to app callbacks. If the app cannot provide a frame while PiP is visible, the backend holds the last frame or ends PiP according to the app's chosen policy; it never blocks the main thread waiting for Go.

Custom content is therefore a product surface with its own size, color space, clock and control model. The first implementation may support a static or app-timed frame stream and leave arbitrary animated layouts for a later design.

## Lifecycle and ownership

Entering PiP is an explicit user action, an app action permitted by the current iOS policy, or the automatic start the app chose above. The backend never starts PiP merely because a video is playing. PiP callbacks are delivered on the engine goroutine, while AVKit delegate methods are acknowledged promptly on their required queue.

A session belongs to the video or the custom-content view that started it and has a stable ID. Apple requires the player, its layer or controller and the PiP controller to stay referenced while PiP is active, but in NSGo a view leaves the screen mostly by its scope's disposal (`Show`, a popped page, `Each`), and deleting a video's view runs `apple.Video.Close`, which pauses and releases the player and detaches and releases its layer or controller at once. So deleting a view while its PiP is active stops PiP, and `Close` keeps those objects until the stop callback and releases them then, which it does not do today. A view whose scope lives on keeps its PiP, the app's background included.

Apple's usual flow dismisses the player's page as PiP starts and shows it again on the restore, which in NSGo disposes the video. A session that outlives its view would be held by a scope the app names and handed the native player as the view is deleted, which needs a deferred deletion in the mount protocol; whether NSGo supports it is an open decision.

A backgrounded app can receive lifecycle callbacks, but it cannot assume the engine will run continuously. Custom content must keep enough state in the session adapter to answer AVKit while the engine is suspended, or declare that PiP ends when the app backgrounds.

PiP needs the shared `AVAudioSession` in the playback category and the `audio` background mode (Apple's "Configuring your app for media playback"). The existing video rules (`apple/audiosession_ios.go`) set that category only while a video whose `AudioSession` is not the default plays, and deactivate the session and restore the previous category once none plays, a pause included. A video with PiP enabled therefore takes the playback category as `AudioSessionPlayback` does even with the default `AudioSession`, from when PiP is enabled so that it can become possible, and keeps the session active, paused or not, until PiP's stop callback. Custom content needs the category too, with or without sound; whether a silent session can mix with other apps' audio, so that a countdown does not stop the user's music, is checked on a device before the implementation chooses. The PiP design does not add a second audio mixer or silently request microphone access.

## Configuration and privacy

The tool must make the app's intent explicit in YAML. PiP needs no entitlement but the Info.plist's `UIBackgroundModes` with `audio`, for custom content as for video, as PiP exists to show while the app is not in front. That key is an array, while `nsgo.yaml`'s `apple.info` holds string entries alone (`cmd/nsgo/project.go`, written as `<string>` by `cmd/nsgo/build.go`), so issue #54's route through `apple.info` cannot declare it: the tool needs a field that writes the array, an open decision. NSGo's iOS minimum, 15.0, has every API here: `AVPictureInPictureController` from iOS 9, automatic starts from 14.2 and content sources from 15.

PiP content is an app-owned frame or video URL. NSGo does not upload it, persist it or inspect its pixels. A network video continues to use the existing `AccessNetwork` check; a custom frame stream is app data and must follow the app's own access grant.

## Verification

The implementation is complete only when these checks exist:

- headless core tests cover support checks, session ownership, idempotent start/stop, lifecycle callbacks and owner cleanup;
- iOS tests use fake AVKit delegates for video and sample-buffer paths, including pause, resume, seek, stale-frame dropping and an app background transition;
- simulator tests verify a video enters and exits PiP, enters it on its own as the app goes to the background when the app chose so, and delivers the restore event as the user returns, and that deleting the view while PiP is active stops PiP and releases its player after the stop callback, leaking nothing; and
- a device test verifies custom content with a controlled frame source and confirms that no callback blocks an AVKit delegate queue.

## Open decisions

These decisions need confirmation before an implementation chooses them:

1. Should the first release expose video PiP only, or ship a separate custom-frame API at the same time?
2. Does custom content end when the engine is suspended, or must the adapter retain and present frames independently?
3. Is the public entry point a `mobile` service, a `core.Video` method, or a new `core` session type?
4. Which `nsgo.yaml` field declares the `audio` background mode: a PiP setting of its own, or `apple.info` taking arrays?
5. Should PiP control callbacks expose seek/skip commands, or only play and pause initially?
6. Can a session outlive the view that started it, through a deferred deletion in the mount protocol, or does deleting the view always end it?
7. Does a video with `Controls` start PiP from code through a second `AVPlayerLayer` on its player, or only by its button?
