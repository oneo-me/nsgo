# Location and motion sensors

This is the design for issue [#56](https://github.com/nsgo-dev/nsgo/issues/56). It is not an implemented API. Location and motion are separate providers with different permission, accuracy and power behavior, but share a lifecycle-aware sensor service in the engine.

## Scope

The first scope covers:

- one-shot and continuous location fixes;
- authorization and availability state;
- accelerometer, gyroscope, magnetometer and device-motion samples where a target provides them; and
- explicit start, stop, rate and accuracy choices with timestamps and a documented coordinate frame.

Geofencing, significant-location-change monitoring, step counting, activity recognition, barometer data and background navigation are separate features. A target that lacks a sensor reports unavailable; it does not synthesize values from another sensor.

## Layers and lifecycle

`core` owns provider-neutral requests, copied sample values and scoped callbacks. `internal/driver` carries request IDs, timestamps, coordinates and numeric sample arrays. Apple uses Core Location and Core Motion; Android uses `LocationManager`/the fused provider boundary and `SensorManager`. Platform objects and callback queues stay in backends.

| Target | Location | Motion |
| --- | --- | --- |
| iOS, watchOS | Core Location: one-shot fixes and streams | Core Motion, the sensors `CMMotionManager` says are available |
| macOS | Core Location: one-shot fixes and streams | Unavailable: `CMMotionManager` is not on macOS |
| tvOS | One-shot fixes (`requestLocation`); streams report unavailable, as `startUpdatingLocation` is not on tvOS | Unavailable: Core Motion's updates are not on tvOS |
| Android | `LocationManager` | `SensorManager` |
| The web, Windows, Linux, mini programs | Unavailable in the first release; the web's Geolocation API, Windows' `Windows.Devices.Geolocation` and Linux's GeoClue are the later backends, recorded in the [roadmap](../roadmap.md) | Unavailable; the web's `DeviceMotionEvent` is the later backend |

Sensor callbacks never run user code on a platform thread. The backend copies a sample, acknowledges the provider callback promptly and posts the sample to the engine goroutine. A scope stopping a stream removes it before later samples are delivered. A pending one-shot location is canceled when its owner ends.

The default policy follows `Env.Lifecycle` ([the app's lifecycle](services.md#the-apps-lifecycle)): streams keep running while the app is `inactive`, which still shows it (another app frontmost on a Mac, the other half of Android's split screen, Control Center over an iPhone app), and pause when it turns `background`. A paused stream stays the app's until its scope ends or the app stops it; the backend restarts the provider when the app shows again, and the first sample after the pause says it follows a gap, so the app can tell a cold fix from a continuous track. A one-shot request waits through the pause rather than failing. A separate background policy may opt in to location updates or a bounded motion session after the Android engine/view lifetime design is complete. The core must not keep a process or a radio alive merely because a handler remains registered.

## Location

A location sample has WGS 84 latitude and longitude, altitude, horizontal and vertical accuracy, course, speed and times. The API reports unavailable fields explicitly rather than using zero as a value. Altitude is above the WGS 84 ellipsoid, as Android's `getAltitude` and Apple's `ellipsoidalAltitude` give it; height above mean sea level is a separate field, Apple's `altitude` and Android's `getMslAltitudeMeters` (API 34), unavailable where the provider has none, since the two differ by the geoid's tens of meters. Each fix carries the wall-clock time the provider gives (`CLLocation.timestamp`, `Location.getTime`), which can jump when the clock is set, and a monotonic time since boot only where the provider has one (`getElapsedRealtimeNanos` on Android; none on Apple). The core delivers fixes in the order the provider sends them and does not compare location times with motion timestamps.

A request chooses desired accuracy, minimum distance and whether a one-shot or stream is wanted. The backend may deliver a less accurate fix while the provider warms up, but each sample says its accuracy. The core does not reverse-geocode, snap to roads or infer a place name.

Apple uses `CLLocationManager`'s authorization and accuracy state. `WhenInUse` and `Always` are distinct app declarations. Continuous background delivery needs the `location` background mode and `allowsBackgroundLocationUpdates` set before the stream starts, under either grant: with When In Use the system shows its background location indicator while it runs. `Always` is needed only for services that relaunch the app, such as region monitoring, significant changes and visits, which this scope leaves out; the system may still reduce updates. Android separates fine and coarse location and its background location grant. The tool adds only the usage descriptions and manifest permissions named by the app, and code never starts a prompt implicitly.

`nsgo build` needs work for this. `apple.info` writes only string entries, so it cannot declare the `UIBackgroundModes` array that background delivery needs, or the `NSLocationTemporaryUsageDescriptionDictionary` that `requestTemporaryFullAccuracyAuthorizationWithPurposeKey` needs to ask for a precise fix after the user chose an approximate one; `nsgo.yaml` gains keys for both. On macOS, `hardenedEntitlements` (`cmd/nsgo/sign.go`) claims `com.apple.security.personal-information.location` when `apple.info` gives a location usage description, as it claims the microphone and the camera, so a notarized build under the hardened runtime, and a Mac App Store build whose sandbox entitlements copy that map, may use Location Services.

The final API must decide whether a one-shot request can use the last cached fix. If it can, the result must identify its age and accuracy; otherwise the backend waits for a new provider fix. A cached fix is never treated as proof that the user is currently at a place.

## Motion

Motion samples use SI units in the right-handed device frame Android and Core Motion share: x toward the screen's right, y toward its top and z out of its face, in the device's natural orientation. Accelerometer, gravity and user-acceleration vectors are in m/s² with Android's sign, so a device lying face up at rest reads +9.81 on z; rotation rates are in rad/s, positive by the right-hand rule, and magnetic fields in µT. A fused attitude rotates the device frame into an east-north-up world frame, as Android's rotation vector does. The core distinguishes raw accelerometer and gyroscope readings from a fused device-motion estimate. Every stream carries the provider's monotonic timestamps (`CMLogItem.timestamp`, `SensorEvent.timestamp`), its sampling interval and an accuracy or calibration state where the platform supplies one.

The app requests a bounded sample interval and a set of sensors. The backend clamps an interval the platform cannot provide and reports the effective interval once. It may drop samples under back-pressure, but it must preserve the latest sample and report whether a stream is throttled; it must not silently allocate an unbounded queue.

Apple keeps Core Motion's axes but not its unit or sign for acceleration: `CMAccelerometerData`, `CMDeviceMotion.gravity` and `userAcceleration` are in g and point the other way (a face-up device reads −1 on z), so the backend multiplies them by −9.80665. Rotation rates and magnetic fields map as they are. Core Motion's north-referenced attitude frames (`CMAttitudeReferenceFrameXMagneticNorthZVertical`, `XTrueNorthZVertical`) put x toward north, so the backend turns them a quarter turn about the vertical into east-north-up; an attitude from an arbitrary-x frame says its heading has no reference. Android maps `SensorManager` events as they are and documents its event timestamps. Device orientation is metadata, not a hidden transform. The API does not promise identical noise, calibration or rate across platforms.

Most motion sensors need no user prompt, while activity recognition, body sensors and target-specific sensors may. The backend reports each required authorization separately. An app can stop a high-rate stream without changing location or notification permission.

## Power, privacy and capabilities

High accuracy and high sample rates cost battery. The API requires an explicit stream and rate, uses provider batching where the target supports it, and exposes a stop operation. Background policies are opt-in and bounded by a declared purpose and budget. The service never uploads samples or writes them to storage on the app's behalf.

Location and motion data use a dedicated capability, distinct from network, contacts and calendar. The final access model must decide whether location and motion are separate grants. A denied scope receives no coordinates or sensor arrays, and diagnostics contain only stable error codes, never a location.

## Verification

The implementation is complete only when these checks exist:

- headless `core` tests cover authorization, one-shot versus stream ownership, cancellation, pausing in the background and resuming with a gap, sample ordering, missing fields, rate clamping and bounded back-pressure;
- `internal/driver` tests cover coordinate encoding, timestamps, empty sensor sets and large batches;
- Apple tests use fake location and motion managers for permission changes, reduced accuracy, unavailable sensors, a face-up device reading +9.81 m/s² on z and stop-on-disposal;
- Android tests use fake location and sensor providers for coarse/fine permission, provider changes, sensor accuracy and API-level differences; and
- target tests use recorded fixtures or simulator providers, never a user's live location, and verify no sample reaches a disposed scope.

## Open decisions

These decisions need confirmation before an implementation chooses them:

1. Can a one-shot location use a cached fix, and what maximum age is acceptable?
2. Does the first release offer background location at all, through When In Use with the background mode on Apple and a location foreground service or `ACCESS_BACKGROUND_LOCATION` on Android, or only foreground fixes?
3. Is the public motion API raw sensors only, or does it include fused device motion, with its east-north-up attitude and gravity split, from the start?
4. Does the capability model separate location from motion and foreground from background access?
5. Are dropped high-rate samples reported as an explicit event, a counter, or only through effective-rate metadata?
