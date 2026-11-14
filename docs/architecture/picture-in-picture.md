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

The player remains the owner of timing, buffering, subtitles, controls and audio focus. PiP does not copy frames into Go. When the app's view is detached, the player and its audio session remain alive while the PiP controller owns them; closing the video or its scope ends PiP and releases the player. A notification press or app relaunch may restore the app's own model, but NSGo does not persist a video URL or playback position for it.

## Custom content PiP

Custom content is a separate component. An app supplies a fixed-size render surface and a playback model that can produce a `CVPixelBuffer` or sample buffer at the requested cadence. The backend uses `AVPictureInPictureController` with a sample-buffer display layer and `AVPictureInPictureSampleBufferPlaybackDelegate`; it does not capture an arbitrary NSGo view tree or screenshot a window.

The source owns frame production and tells the backend when a frame is ready. The backend drops stale frames under back-pressure, retains at most one pending frame and reports the effective frame interval. The playback delegate maps play, pause, seek and skip commands to app callbacks. If the app cannot provide a frame while PiP is visible, the backend holds the last frame or ends PiP according to the app's chosen policy; it never blocks the main thread waiting for Go.

Custom content is therefore a product surface with its own size, color space, clock and control model. The first implementation may support a static or app-timed frame stream and leave arbitrary animated layouts for a later design.

## Lifecycle and ownership

Entering PiP is an explicit user action or an app action permitted by the current iOS policy. The backend never starts PiP merely because a video is playing. PiP callbacks are delivered on the engine goroutine, while AVKit delegate methods are acknowledged promptly on their required queue.

The PiP session owns the native resources after the originating view leaves the screen. A session has one owner scope and a stable ID; owner disposal ends it after the final stop callback. A backgrounded app can receive lifecycle callbacks, but it cannot assume the engine will run continuously. Custom content must keep enough state in the session adapter to answer AVKit while the engine is suspended, or declare that PiP ends when the app backgrounds.

Audio follows the existing video audio-session rules. The PiP design does not add a second audio mixer or silently request microphone access. A session that has no audio must say so so iOS does not keep an unnecessary playback category active.

## Configuration and privacy

The tool must make the app's intent explicit in YAML and generated entitlements. Video PiP needs the appropriate playback/background declaration; custom content needs the same only if its session continues while the app is not foreground. The exact key and minimum iOS version remain open until the tool and runtime are implemented together.

PiP content is an app-owned frame or video URL. NSGo does not upload it, persist it or inspect its pixels. A network video continues to use the existing `AccessNetwork` check; a custom frame stream is app data and must follow the app's own access grant.

## Verification

The implementation is complete only when these checks exist:

- headless core tests cover support checks, session ownership, idempotent start/stop, lifecycle callbacks and owner cleanup;
- iOS tests use fake AVKit delegates for video and sample-buffer paths, including pause, resume, seek, stale-frame dropping and an app background transition;
- simulator tests verify a video enters and exits PiP and that the originating view may disappear without leaking its player; and
- a device test verifies custom content with a controlled frame source and confirms that no callback blocks an AVKit delegate queue.

## Open decisions

These decisions need confirmation before an implementation chooses them:

1. Should the first release expose video PiP only, or ship a separate custom-frame API at the same time?
2. Does custom content end when the engine is suspended, or must the adapter retain and present frames independently?
3. Is the public entry point a `mobile` service, a `core.Video` method, or a new `core` session type?
4. Which iOS versions and background entitlements are supported for each path?
5. Should PiP control callbacks expose seek/skip commands, or only play and pause initially?
6. Does a video with `Controls` start PiP from code through a second `AVPlayerLayer` on its player, or only by its button?
