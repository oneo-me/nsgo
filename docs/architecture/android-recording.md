# Long-running Android recording

This is the design for issue [#38](https://github.com/nsgo-dev/nsgo/issues/38). It is not an implemented API. Android long recording is a runtime service, not a view feature: `AudioRecord`, a foreground service, app-owned storage and level callbacks must share one owner and one lifecycle.

The process/view split in [#12](https://github.com/nsgo-dev/nsgo/issues/12) is a prerequisite. An `NsgoView` detaching must not stop a recording, and an activity recreation must not create a second reader for the same microphone.

## API boundary

The existing `microphone` package remains the portable entry point. The proposed Android extension opens a recording with an explicit format and destination, then returns a handle with `Stop`, `Ended`, `File`, `Error` and level callbacks. The handle owns no view and is closed with the app-level recording owner.

The first API records encoded audio to an app-owned file rather than streaming unbounded PCM through the Go/Java pipe. A recording option chooses a container and codec supported by the target; the backend reports the actual format and rejects a codec it cannot encode. Raw PCM and a bounded frame-stream API may follow, but they are separate because they change memory, bridge and back-pressure guarantees.

Level callbacks carry peak and RMS values over a configurable interval. They are metering data, not a second audio stream. The Android worker computes them from the frames it writes and coalesces callbacks when the engine is busy. A callback runs on the engine goroutine after the worker has acknowledged the audio buffer.

## Android runtime

The Java runtime owns `AudioRecord`, its reader thread, the encoder and the foreground service. Go sends start, pause/resume if supported, stop and level-interval commands over a protocol channel; Java sends state, levels, file completion and stable errors. The reader never calls Go synchronously and never waits for the engine while holding an `AudioRecord` buffer.

The service acquires the microphone only after `RECORD_AUDIO` is granted and the app has explicitly started a recording. On Android 14 and later the manifest declares the microphone foreground-service type and starts it only from a permitted foreground context. The service shows the required ongoing notification; `POST_NOTIFICATIONS` controls that notification's visibility, not whether the microphone permission is granted.

If the process is killed, the service either finishes and atomically closes the current file or reports a terminal error with the partial file's status. It never resumes silently with a new file. The lifecycle design decides whether a stopped process may be restarted to deliver `Ended`; until then the app can inspect the durable file and the next engine start reports the terminal record.

## Files and formats

The tool and runtime must agree on where recordings live. A temporary file is created in the app's private storage, written sequentially, flushed and renamed atomically to its final name only after the encoder closes. The API returns a file handle or app-owned path that `microphone` and `core` can read without copying the audio through the bridge. A host embedding the runtime supplies its storage root and cleanup policy.

The implementation starts with one documented format that Android can encode on the supported API range, likely AAC in an MPEG-4 container, and exposes the sample rate, channel count and duration actually written. WAV/PCM and codec choices that vary by device stay out of the first contract until the acceptance matrix is tested. A size limit and free-space error are explicit; the service never deletes an app's prior recording to make room.

## Permissions, interruption and privacy

The app declares `RECORD_AUDIO` and the foreground-service permission/type in YAML. The API reports denied, unavailable, interrupted, stopped and completed separately. A phone call, another exclusive recorder, audio focus loss or a revoked permission ends or pauses according to an explicit option; it never leaves a handle that claims to record while no frames arrive.

The recording indicator and system privacy controls remain Android's. NSGo does not upload, transcribe or inspect the recording. Level values are not written into the audio file. Logs contain IDs and error codes only, never microphone data or file contents.

## Verification

The implementation is complete only when these checks exist:

- headless `microphone` tests cover format validation, state transitions, owner cleanup, level cadence and terminal errors;
- Android protocol tests cover start/stop, encoder failure, partial-file reporting, a dead-process service and level back-pressure without an Android device;
- runtime tests use a fake `AudioRecord` and encoder to cover permission, buffer starvation, interruption, rotation and atomic rename; and
- device tests run a long recording across an activity recreation and screen-off period, verify the foreground notification and file duration, and confirm that stopping leaves no open descriptor.

## Open decisions

These decisions need confirmation before an implementation chooses them:

1. Is the first public recording API file-only, or must it expose bounded PCM frames as well?
2. Which encoded format, sample rates and channel counts are guaranteed across the supported Android API range?
3. Does an interruption pause and resume, finish the file, or fail the recording by default?
4. May a background service restart a stopped process to deliver completion, or is durable file inspection enough?
5. Is level metering part of `microphone.Stream`, or a separate recorder option and callback type?
