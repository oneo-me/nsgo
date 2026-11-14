# Long-running Android recording

This is the design for issue [#38](https://github.com/nsgo-dev/nsgo/issues/38). It is not an implemented API. Android long recording is a runtime service, not a view feature: `AudioRecord`, a foreground service, app-owned storage and level callbacks must share one owner and one lifecycle.

The process/view split in [#12](https://github.com/nsgo-dev/nsgo/issues/12) is a prerequisite. An `NsgoView` detaching must not stop a recording, and an activity recreation must not create a second reader for the same microphone.

## API boundary

The existing `microphone` package remains the portable entry point. The proposed Android extension opens a recording with an explicit format and destination, then returns a handle with `Stop`, `Ended`, `File`, `Error` and level callbacks. The handle owns no view and belongs to the process, as the microphone does, not to an engine.

On Android the app's Go code is an executable the runtime starts (`NsgoView` with `ProcessBuilder`), with no audio API of its own, so `microphone` reaches the runtime the way `widget`'s Live Activities do: through a channel in `internal/driver` outside any surface, which the Android backend connects and whose messages wait until it does (`driver.SendLiveUpdate` and `ConnectLiveUpdates` are the model, carried as a message of the backend's protocol as `msgLiveUpdate` is). The implementation therefore changes `layering_test.go`'s `{"microphone", nil}` to allow `internal/driver`, and the paragraph of [layers](layers.md) that says `microphone` stands apart; `internal/driver` stays portable, so `microphone` still builds on every target.

The first API records to an app-owned file rather than streaming unbounded PCM through the Go/Java pipe, so `microphone.Capture` stays unsupported on Android until open question 1 is answered. The backend reports the actual format and rejects a format it cannot write. Raw PCM and a bounded frame-stream API may follow, but they are separate because they change memory, bridge and back-pressure guarantees.

Level callbacks carry peak and RMS values over a configurable interval, at least ten a second by default so a waveform can be drawn. They are metering data, not a second audio stream, and describe frames already written to the file. They run on a goroutine of `microphone`'s, never on an engine's, as `Capture`'s samples run on a thread of the system's on macOS and iOS; an app that updates its views from one hops with `Scope.Post`. A level the callback has not taken yet is replaced by the next, so a slow callback never holds up the recording.

## Android runtime

The Java runtime owns `AudioRecord`, its reader thread, the file writer and the foreground service, all in the app's process. Go sends start, stop and level-interval commands over the channel above; Java sends state, levels, file completion and stable errors. The reader never calls Go synchronously and never waits for Go while holding an `AudioRecord` buffer. One recording runs per app at a time: the service refuses a second start, from this process or another view's, as busy.

`RECORD_AUDIO` is asked for when a recording starts, through the visible activity as the runtime asks for `CAMERA`; an `-embed` host passes its `onRequestPermissionsResult` on as it does today ([embedding](embedding.md)). A refusal is `ErrDenied`, and a later start after the user grants it records without restarting the app. A start with no visible activity fails with a stable error instead of asking, since Android shows its permission dialog only over an activity.

The foreground-service rules span the whole supported range, API 28 to the target (the runtime's manifest has `minSdkVersion` 28 and `targetSdkVersion` 34):

- From Android 9 (API 28), an app capturing with no foreground UI and no foreground service receives silence, and a foreground service needs the `FOREGROUND_SERVICE` permission.
- From Android 11 (API 30), an app targeting it must declare the `microphone` foreground-service type to use the microphone in a foreground service, and a foreground service started while the app is in the background cannot use the microphone at all.
- From Android 14 (API 34), every foreground service declares its type, the app also declares `FOREGROUND_SERVICE_MICROPHONE`, `RECORD_AUDIO` must be granted before the service starts, and Google Play asks the app to declare the type in Play Console.

So the service starts only while an activity of the app is visible and after `RECORD_AUDIO` is granted, on every API level, not only on Android 14. The service shows the required ongoing notification; `POST_NOTIFICATIONS` controls that notification's visibility, not whether the microphone permission is granted.

Two processes can die. If the Go process ends while the app's process lives, the service finishes and closes the current file itself and reports the terminal record when Go connects again. If the app's process is killed (low memory, force stop, an OEM's swipe from recents), the service and the Go child die with it and no code runs, so the file on disk must already be usable: that is what the format below guarantees. A recording never resumes silently with a new file. The lifecycle design decides whether a stopped process may be restarted to deliver `Ended`; until then the app can inspect the durable file and the next start of the channel reports the terminal record.

## Files and formats

The tool and runtime must agree on where recordings live. A recording is written sequentially under a temporary name in the app's private storage and renamed to its final name when it stops. The API returns a file handle or app-owned path that `microphone` and `core` can read without copying the audio through the bridge. A host embedding the runtime supplies its storage root and cleanup policy.

The first format is 16-bit PCM WAV, as #38 asks, written by the Java reader itself. WAV is chosen over AAC in MPEG-4 because a killed recording stays playable: the writer rewrites the header's sizes about once a second and at stop, the writer holds no buffer of its own past one `AudioRecord` read, so what it wrote before a kill is in the system's page cache rather than the process's, and the next start repairs the header of a temporary file from its length and reports it as a partial recording with its duration. An MPEG-4 file `MediaMuxer` writes has no index until it stops (its `moov` box is written then), so a killed one is unplayable from its first second. The backend exposes the sample rate, channel count and duration actually written, reporting the rate the device gave rather than the one asked for. RIFF's 32-bit sizes cap a WAV at 4 GiB, about 12 hours of 16-bit mono at 48 kHz; the size limit and a free-space error are explicit terminal states before that, and the service never deletes an app's prior recording to make room.

## Permissions, interruption and privacy

The tool's `nsgo.yaml` gains a key, proposed as `android.recording: true`, that adds the runtime's recording service to the APK's and the bundle's manifest as a `<service>` with `android:foregroundServiceType="microphone"`, and the `RECORD_AUDIO`, `FOREGROUND_SERVICE` and `FOREGROUND_SERVICE_MICROPHONE` permissions. `android.permissions` cannot express this, as it adds only `<uses-permission>`s ([tool](tool.md)), and `axml.go`'s `androidAttrs` and `bundle.go` fail the build on `foregroundServiceType` today, so both encoders learn it. An app without the key declares no recording service, since Play asks the app to declare and describe every foreground-service type its manifest has. An `-embed` library leaves them to the host, as it leaves permissions: the host's manifest declares the runtime's service with that type and the three permissions.

The API reports denied, unavailable, busy, interrupted, stopped and completed separately. Interruptions are detected from the capture, not from missing frames, as an interrupted capture usually keeps delivering frames:

- From Android 10 (API 29), a voice call (`MODE_IN_CALL` or `MODE_IN_COMMUNICATION`) or a higher-priority capture leaves the recording running but silenced. The runtime registers `AudioRecord.registerAudioRecordingCallback` before `startRecording`, as Android requires, and reports interrupted while `AudioRecordingConfiguration.isClientSilenced()` is true.
- Before Android 10, `startRecording` fails while another app captures, which is reported as busy.
- A revoked permission or an `AudioRecord` read error ends the recording.

Whether an interruption pauses the file, inserts nothing and goes on, or ends the recording is an explicit option; the handle never claims to record while the capture is silenced. The recorder requests no audio focus and listens for none: focus arbitrates output, `AudioRecord` needs none, and requesting it would pause the app's own playback, so a recording and the app's playback, or another app's, run together as #38 asks.

The recording indicator and system privacy controls remain Android's. NSGo does not upload, transcribe or inspect the recording. Level values are not written into the audio file. Logs contain IDs and error codes only, never microphone data or file contents.

## Verification

The implementation is complete only when these checks exist:

- headless `microphone` tests cover format validation, state transitions, owner cleanup, level cadence and terminal errors;
- Android protocol tests cover start/stop, busy, writer failure, partial-file reporting and repair, a dead Go process and level back-pressure without an Android device;
- runtime tests use a fake `AudioRecord` to cover permission, buffer starvation, a silenced capture reported through the recording callback, rotation and the rename at stop; and
- device tests on an Android 9 or 10 device, one of Android 11 to 13 and one of Android 14 or later run a 30-minute recording across an activity recreation and a screen-off period, verify the foreground notification and file duration, play a sound while recording and record while playing with neither interrupted, report a phone call as an interruption, keep a playable WAV after the app's process is killed, and confirm that stopping leaves no open descriptor.

`nsgotest` recording what an app records, as it records speech, as #38 asks, depends on open question 5: a channel in `internal/driver` is one per process, while `nsgotest` runs many apps in one.

## Open decisions

These decisions need confirmation before an implementation chooses them; [ARCHITECTURE.md](../../ARCHITECTURE.md#open-questions) lists them too:

1. Is the first public recording API file-only, or must it expose bounded PCM frames as well, which `microphone.Capture` on Android would need?
2. Is an encoded format such as AAC in MPEG-4 added after WAV, accepting that a killed recording in it is lost, or only one that stays playable when killed?
3. Does an interruption pause and resume, finish the file, or fail the recording by default?
4. May a background service restart a stopped process to deliver completion, or is durable file inspection enough?
5. Does recording stay in `microphone` behind the `internal/driver` channel, with callbacks off every engine, or become a `core` service as `StartSpeech` is, with callbacks on the engine's goroutine of the scope that started it and records in `nsgotest`, at the cost of moving the API out of `microphone`? Is level metering then part of `microphone.Stream`, or a separate recorder option and callback type?
