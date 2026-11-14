# Audio playback

This document is the design for issue #36. It describes the API and the
backend contract. The portable API and lifecycle wrappers exist, and macOS
and iOS now use AVAudioPlayer for local cues and players. Other targets still
return `ErrUnsupported`; shared audio-session coordination and the remaining
backends are still open work.

## The API

Audio is a top-level package, beside `microphone`, because it owns a native
resource and does not render a view. It never imports `core` or a platform
package. The package exposes two levels of use:

```go
sound, err := audio.PlayFile("assets/notify.wav")
if err == nil {
	sound.SetVolume(0.6)
	sound.OnEnd(func() { /* the cue finished */ })
}

player, err := audio.Open(path)
if err == nil {
	player.OnProgress(func(position, duration time.Duration) {})
	player.Play()
	player.Seek(12 * time.Second)
	player.SetRate(1.5)
	player.SetLoop(true)
}
```

`PlayBytes` and `OpenBytes` take encoded bytes and an explicit MIME type. A `Source` helper may be added if more than a path and bytes are needed, but the first API keeps the source forms explicit. Opening reads the source's header and prepares the output on the calling goroutine, so that these entry points can return its errors, and `Play` starts the output there too; an app opening a large file, or playing over a slow route such as Bluetooth, calls them off its engine goroutine. Until a target backend is linked, these entry points return `ErrUnsupported` after validating their source and options.

`Sound` is a short-lived cue. It has `SetVolume`, `Stop`, `OnEnd` and `Close`;
`Close` is idempotent and releases the native player. A cue that is played
again may use a new native player, so an old end callback cannot end the new
one.

`Player` is long-form playback. It has `Play`, `Pause`, `Stop`, `Seek`, `SetVolume`, `SetRate`, `SetLoop`, `Position`, `Duration`, `Playing`, `Ended`, `OnProgress`, `OnEnd`, `Err` and `Close`. `Open` returns after the source has been accepted, not after all bytes have decoded. `Duration` can be zero until metadata is ready; `OnProgress` reports the position, which moves forward while playing and goes back only when a loop restarts or a seek moves it, and the final duration once it is known. `Position` asks the backend, and after the end keeps the last position reported. Seeking before metadata is ready is queued and applied when the decoder can seek; until then the backend reports the queued position.

Callbacks are serialized on the package's callback queue. They never run on a platform audio thread and never hold a platform lock while calling app code. `Close` prevents later callbacks, waits only for the callback queue to forget the player, and does not block on a decoder or device teardown from the app's engine goroutine. On macOS and iOS, `AVAudioPlayer` reports the end of playback and decoding errors only to its delegate, on the main thread's run loop, which an app runs; the backend hands them off that thread at once.

## Errors and sessions

The package reports stable sentinel errors for an unsupported target, an
unreadable source, an unsupported format, a denied or unavailable output
device, and a closed player. A format failure is observable through `Open` or
`Err`; silence is never treated as success. `PlayFile` returns the same errors
as `Open` and invokes `OnEnd` only after a successfully opened cue finishes.

Playback has a `Session` option with the existing `AudioSessionDefault`,
`AudioSessionPlayback` and `AudioSessionMixWithOthers` values. The default
does not change another app's audio. Playback asks for the session only while
one of the package's players is audible, and restores the previous session
when the last player pauses, ends or closes. A cue defaults to mixing so it
does not interrupt a video or speech session; an app can request playback
explicitly for a long-form player.

The session is process-wide but its ownership is reference counted. Video, speech and `microphone` must share the same iOS audio-session coordinator rather than each setting `AVAudioSession` independently. Android maps playback to transient audio focus and mixing to no focus request. Desktops and the web use the system mixer and do not pause other applications.

Until that coordinator exists, iOS does not meet these goals: a cue or player plays in the app's current category, by default the solo ambient one, which stops other apps' audio and is muted by the silent switch, and a `Session` other than the default returns `ErrUnsupported` rather than being ignored. macOS accepts every `Session`, as its system mixer already plays alongside other apps.

## Backend contract

Every backend decodes and tears down off the engine's goroutine. Platform
callbacks are translated into the same state transitions: opened, metadata,
progress, paused, ended and error. A backend may choose a different native
buffer size, but progress is emitted at the requested interval with a minimum
of 50 ms and is coalesced while the app is not consuming it.

| Target | Native path | First implementation boundary |
| --- | --- | --- |
| macOS and iOS | `AVAudioPlayer` for cues and local players | Purego bindings; shared Apple audio-session coordination remains open |
| Windows | Media Foundation source reader and audio renderer | A worker-owned `IMFMediaEngine` or `IMFSourceReader`, with COM initialized on that worker |
| Linux | GStreamer through GTK's loaded libraries, or a small purego GStreamer binding | A worker-owned pipeline using `playbin`, with bus messages translated to the callback queue |
| Android | `MediaPlayer` for files and the existing Java runtime boundary | A Java `AudioPlayer` that reports state over the checked-in protocol; rebuild `classes.dex` and `classes.jar` |
| Web | `HTMLAudioElement` and `AudioContext` only where an app gesture has unlocked playback | A page host channel; autoplay refusal is an explicit error |
| Mini programs | The host audio API where it provides one, otherwise `ErrUnsupported` | A host protocol and a test fake before a platform binding |
| watchOS and tvOS | The platform's audio player where the target permits it | Separate target verification; unsupported targets still return an error |

The first desktop and Apple implementation should accept WAV and MP3, and
report a format error for a codec the native path cannot decode. FLAC and
other formats follow the codec matrix of each target; no Go decoder is added
until a target lacks a native codec that the API promises.

## Testing and delivery

`nsgotest` records `PlayFile`, `PlayBytes`, player commands, progress and end events. Its fake advances time only when a test asks it to, so tests can assert pause, seek, loop and close without sleeping. The portable package currently covers source and option validation, lifecycle behavior and explicit fallback errors, and on macOS plays silent WAVs through `AVAudioPlayer` to cover the end of playback, loops, seeks, stop and the error kinds, with its test's main thread running the run loop as an app's does; the `nsgotest` fake and the other native backends remain subsequent stages.

The implementation is staged so each change remains reviewable:

1. Add the portable API, errors, session values and `nsgotest` fake.
2. Add the Apple cue and player, with macOS and iOS target checks; share the
   audio session with video, speech and microphone in a follow-up.
3. Add Windows and Linux workers and their format/error tests.
4. Add the Android runtime protocol and rebuild the committed runtime.
5. Add the web and mini-program host paths, then a gallery cue and player
   demo that exercises both levels.

The issue closes only after WAV and MP3 play on macOS, Windows, Linux and the
web; progress, seek, loop and close are covered by tests; and a cue leaves a
video and speech session usable on iOS.
