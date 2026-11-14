# Audio playback

This is the design for issue #36, a port of NSH 0.5.0's audio design (`nsh/docs/dev/audio-design.md`, approved and implemented there): `Sound` for short sounds and a player for long audio. None of it exists yet, and the [roadmap](../roadmap.md#services-and-components-since-040) keeps its row until the stages below land. Read this, and NSH's design, before building any part of it; [NSH's design and this one](#nshs-design-and-this-one) lists where they part.

## The API

The API's home is the `audio` package, which imports `core` alone, as `mobile` does: `core` implements its engine part and names `audio` in that part's comments. Its functions take the `*core.Scope` asking, so a sound or a player belongs to that scope's engine and owner, as NSH's belong to their engine and JS owner.

```go
// A short sound: decoded once, played any number of times, overlapping.
audio.LoadSound(cx, audio.Source{URI: "asset:sounds/click.wav"}, func(click *audio.Sound, err error) {
	if err != nil {
		return
	}
	click.Play(audio.Voice{})                       // fire and forget
	click.Play(audio.Voice{Volume: 0.5, Rate: 1.2}) // overlaps the one above
})

// Long audio: streamed and decoded as it plays.
player := audio.NewPlayer(cx, audio.Source{URI: "https://example.com/voice.m4a"}, audio.PlayerOptions{
	OnProgress: func(position, duration time.Duration) { elapsed.Set(position) },
	OnEnd:      func() {},
	OnError:    func(err error) {},
})
player.Play(func(err error) {}) // runs once it plays, or with why it cannot
player.Seek(12500 * time.Millisecond)
player.SetRate(1.5)
player.Close()
```

A `Source` is what an image's is: an `asset:` path in the app's resources, a file path or `file:` URL, a `data:` URL, an `http(s)` URL, or `Data`, bytes such as a file embedded with `go:embed`, named by the format their first bytes say. Backends resolve it as they resolve images, so an `asset:` sound is found in a macOS bundle's resources, beside the executable on Windows and Linux, inside the APK on Android and beside the page on the web.

`Sound` is NSH's `Sound`:

- `LoadSound` has the platform read and decode the source into memory once, and calls back with the sound or why not. Loading a source this engine already holds (the same resolved URI, or bytes of the same digest) shares the decoded buffer, which goes with the last `Sound` over it.
- `Play(Voice)` starts a playback within a frame: it is a command in the batch of the turn that calls it, so nothing is opened, fetched or decoded then. Playbacks overlap. `Volume` runs from 0 to 1 and `Rate` from 0.5 to 2; 0 is 1 for both, as `Video`'s, and other values are clamped, where NSH throws a `RangeError`. A playback that cannot be heard, the device muted or a voice dropped past the limit, is no error.
- `Stop` stops every playback of the sound, `Duration` is the decoded length, and `Close` frees the buffer. The owner that loaded the sound closes it as it goes, and the engine as it closes.

`Player` is NSH's `AudioPlayer`, `Video` without a picture:

- `NewPlayer` loads nothing until `Play` or `Preload`, so making one is free. `Play` loads if needed and calls back once the platform plays, or with the error.
- `Pause`, `Seek` (clamped to the duration), `SetVolume`, `SetRate` and `SetLoop` are commands in the batch. A seek while paused or before the duration is known applies when the platform can seek, and `Play` starts from there.
- `Position`, `Duration` (0 until loaded) and `State` (idle, loading, playing, paused, ended or failed) are kept by `core` from the platform's events, so reading them asks no platform.
- `PlayerOptions` has `OnLoad` with the duration, `OnProgress` with a position that never goes back and the duration, every `ProgressInterval` while playing (250 milliseconds when 0, at least 16, as `Video`'s) and once more as it pauses or ends; `OnBuffer`, `OnEnd` once per play-through (never while looping) and `OnError`. It also has `AudioSession` ([sessions](#sessions)).
- `Close` stops it and releases the platform's player. A player is not tied to a view, but it is to the owner that made it and to the engine: they close it as they go, so a window or an embedded view (`nsgo.NewView`) that closes stops its audio.

Both are called on the engine's goroutine, as every scope service is; other goroutines reach them through `Scope.Post`. Their callbacks run on the engine's goroutine too, in a later turn, as an image's or a speech session's do, so they may set signals. A closed sound's or player's callbacks never run: `core` numbers sounds and players from one counter of the process, so a late event finds no one.

`Env.CanPlayAudio` says the platform plays audio. Elsewhere, `LoadSound` and `Play` fail at once with `Unavailable`, without asking the platform.

## Errors and limits

Failures are an `audio.Error`, named as NSH's codes, which follow the web's `MediaError`: `NotAllowed` (refused by a capability, or the web's autoplay policy before a user gesture), `Network`, `Decode` (a format the platform cannot read: an error, never silence), `NotSupported`, `QuotaExceeded`, `Unavailable` and `Failed`. A player's error arrives through `OnError` and `Play`'s callback, and it goes idle.

An output that fails makes silence, not an error: a device unplugged or changed, an engine reconfigured or a system that refuses the output. The platform reopens on the default device when it can and keeps the sounds loaded; a player that loses its output reports `Failed` once and goes idle.

NSH's limits hold per engine:

- A sound's encoded bytes are at most 8 MiB, whatever the source, checked by the backend's reader before decoding. Its decoded length is at most 5 seconds: the load carries that cap, the platform decodes no more and reports a cut, and a cut sound is freed and fails with `QuotaExceeded`. Longer audio is the player's.
- At most 64 sounds and 16 MiB of decoded audio, counted by `core` from what each load reports; the next load fails with `QuotaExceeded`.
- At most 32 playbacks at once; the platform drops the oldest past that.

Go code is trusted ([ARCHITECTURE.md](../../ARCHITECTURE.md#boundaries)), so these limits are what an app meets. TSX packages are also held to capabilities through `core.Access`: an `http(s)` source needs `network`, as an image's does, and every sound and player needs a new `audio` capability (`AccessAudio`, NSH's), refused with `NotAllowed`.

## Sessions

The engines' audio objects are per engine, but iOS has one `AVAudioSession` per process, which videos, speech, `microphone` and now audio all set. A new shared package, `internal/audiosession`, holds it for all of them: it imports nothing of the module, so `microphone` and the Apple backend both import it (a rule of its own in `layering_test.go`, with `microphone`'s widened to it and the package added to `platformShared`), it guards its holders with a mutex, so the main thread, the audio workers and `microphone`'s caller may all call it, and it does nothing outside iOS. It replaces `apple/audiosession_ios.go`'s list, `speech_ios.go`'s and `microphone_ios.go`'s own copies, which is the roadmap's *Recording beside video on iOS* row.

Each holder asks for a level, and the highest held sets the session:

1. A recording, `microphone.Capture` or `StartSpeech`: `PlayAndRecord` with `AllowBluetooth` and `DefaultToSpeaker`, in speech's `Measurement` mode while a speech session listens, else `Default`. Sounds and players play through it, so a cue never stops a recording; the Ring/Silent switch does not silence this category.
2. Playback, a video or player with `AudioSessionPlayback`: `Playback` in the `SpokenAudio` mode, pausing other apps' audio, as videos have it now.
3. Mixing, `AudioSessionMixWithOthers`: the same with `MixWithOthers`.
4. Ambient, every sound and a player with `AudioSessionDefault`: `Ambient`, as NSH sets it, which mixes with other apps' audio and which the Ring/Silent switch and the screen lock silence.
5. None held: the category from before the first holder, then deactivation telling other apps they may play.

A video's `AudioSessionDefault` stays no holder at all, as it is now. Audio's is ambient instead because leaving the session alone would play under `SoloAmbient`, iOS's default category, which stops other apps' audio.

A holder holds only while it plays, so a paused or ended player and a sound with no playback hold nothing. On Android, sounds ask for no audio focus, and a player takes a video's: `AUDIOFOCUS_GAIN_TRANSIENT` while it plays with `AudioSessionPlayback`, pausing for another app's focus. Desktops and the web play through the system mixer, under the app's own volume there, and pause no other application. Mobile players pause when the app leaves the foreground, as NSH's do; background playback and lock-screen controls are not part of this design.

## The platforms

Each engine starts its audio objects with its first load or player, so an app that makes no sound never opens the output, and releases them as it closes. Decoding runs off the main thread and off the engine's goroutine.

| Target | `Sound` | `Player` |
| --- | --- | --- |
| macOS, iOS | One `AVAudioEngine` per engine, an `AVAudioPlayerNode` per playback over its mixer; decoded once into an `AVAudioPCMBuffer` at load | `AVPlayer` with no layer, the class `Video` uses |
| Windows | One `XAudio2` engine per engine, a source voice per playback over its mastering voice; Media Foundation's source reader decodes at load, on one decoding thread for the process | Media Foundation's media engine, as `Video`'s, with no video output |
| Linux | GStreamer through purego: decoded once (`decodebin` into 48 kHz stereo floats, cut at the cap), played by one live pipeline per engine with an `appsrc` per playback into an `audiomixer` | `GtkMediaFile`, as `Video`'s; GTK's streams have no rate, so `SetRate` does nothing there |
| Android | One `SoundPool` per engine with 32 streams, its duration read with `MediaExtractor` | `MediaPlayer`, as `Video`'s, with its focus handling; both in the Java runtime, with `classes.dex` and `classes.jar` rebuilt |
| Web | One `AudioContext` per engine, made and resumed on the first user gesture the engine sees; `decodeAudioData` at load, an `AudioBufferSourceNode` per playback | An `HTMLAudioElement`, as `Video`'s `<video>` |
| Mini programs, tvOS, watchOS | Not in the first stages: `Env.CanPlayAudio` is false and calls fail with `Unavailable` until each gets a roadmap row of its own | The same |

A sound is read by the backend's image reader (`imageload`, with its cache and limits; the browser on the web) and only decoded natively; a player's source goes to the native player as a video's does, which streams it. Formats are the platform decoders': WAV, AAC (`.m4a`) and MP3 decode everywhere and are the ones to ship; FLAC, Opus and Vorbis decode only where the platform does, and fail with `Decode` elsewhere. No Go decoder is added, as NSH adds none.

## NSH's design and this one

- **The package.** NSH's `Sound` and `AudioPlayer` are modules of its core runtime. Here the API's home is `audio`, as issue #36 asked, so `core`'s API does not grow, while `core` does the engine's part as it does `mobile`'s. A package of its own beside `microphone`, importing nothing, cannot: it would reach no backend, `asset:` resource, iOS session or Android runtime, and its callbacks would run off the engine.
- **`Player`, not `AudioPlayer`.** Go names drop their package's (`dock.Area`); TSX keeps `AudioPlayer`.
- **Go's shapes.** Callbacks where NSH returns promises and fires events, `Close` where it has `dispose`, and clamping where it throws a `RangeError`, since a fire-and-forget call has no error to return.
- **No `PlayFile` or `PlayBytes`.** Issue #36 proposed a call that opens and decodes a file each time; NSH rules that out, since it starts too late to be feedback and hides a read in a tap handler. A cue is a sound loaded once and played many times.
- **A player's `AudioSession`.** NSH plays everything ambient and reserves a `session` option for later. NSGo's videos have it already, and an audiobook or a voice message needs to play in silent mode, so a player takes it, ambient by default as NSH's.
- **The shared session.** NSH sets `Ambient` once. NSGo also holds the session for videos, speech and `microphone`, so the levels above settle who wins.
- **Reading.** NSH reads a sound's bytes in its core through its image loader, so its limits are checked there. NSGo's image loader lives in the backends, so they read and check the encoded size, and `core` counts the rest.

## Delivery

Each stage is one reviewable change, in NSH's order:

1. The `audio` package with its rule in `layering_test.go`, `core`'s part, the protocol's requests and events, `Env.CanPlayAudio`, `AccessAudio` and the `audio` capability, and `nsgotest`'s fake: it records the sounds loaded and played and each player's commands, answers a load with a duration the test sets, advances progress only on the test's clock, and can fail a load with `Decode` and a play with `NotAllowed`, so tests assert on what an action played without sleeping.
2. `internal/audiosession`, with videos, speech and `microphone` moved onto it.
3. macOS and iOS, then the web, Windows, Android and Linux, each with its format and error tests.
4. The gallery: a sound for each kind of moment and a player demo of both levels.

Issue #36 closes, and the roadmap's row with it, when a WAV and an MP3 play as sounds and through a player on macOS, iOS, Windows, Linux, Android and the web, and a FLAC either plays or fails with `Decode`; a sound plays overlapping and starts within a frame; `OnProgress` never goes back, and a seek while paused shows the new position on `Play`; `Close` on a playing player stops it, and ten open and close cycles leave no handle behind (on macOS, no audio device stays open); on an iPhone, a sound during a playing video neither pauses nor stops it, a recording and `StartSpeech` keep working through a sound, and the Ring/Silent switch silences sounds; and the gallery's demos are in.
