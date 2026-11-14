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

Sensor callbacks never run user code on a platform thread. The backend copies a sample, acknowledges the provider callback promptly and posts the sample to the engine goroutine. A scope stopping a stream removes it before later samples are delivered. A pending one-shot location is canceled when its owner ends.

The default policy stops continuous streams when the app becomes inactive or backgrounded. A separate background policy may opt in to location updates or a bounded motion session after the Android engine/view lifetime design is complete. The core must not keep a process or a radio alive merely because a handler remains registered.

## Location

A location sample has latitude, longitude, altitude, horizontal and vertical accuracy, course, speed, timestamp and the source's coordinate reference. The API reports unavailable fields explicitly rather than using zero as a value. Timestamps preserve the provider's monotonic ordering; wall-clock conversion is an app decision.

A request chooses desired accuracy, minimum distance and whether a one-shot or stream is wanted. The backend may deliver a less accurate fix while the provider warms up, but each sample says its accuracy. The core does not reverse-geocode, snap to roads or infer a place name.

Apple uses `CLLocationManager`'s authorization and accuracy state. `WhenInUse` and `Always` are distinct app declarations; background delivery requires the latter plus the appropriate background mode, and the system may reduce updates. Android separates fine and coarse location and its background location grant. The tool adds only the usage descriptions and manifest permissions named by the app, and code never starts a prompt implicitly.

The final API must decide whether a one-shot request can use the last cached fix. If it can, the result must identify its age and accuracy; otherwise the backend waits for a new provider fix. A cached fix is never treated as proof that the user is currently at a place.

## Motion

Motion samples use SI units and a documented right-handed coordinate frame relative to the device. The core distinguishes raw accelerometer and gyroscope readings from a fused device-motion estimate. Every stream carries a provider timestamp, sampling interval and an accuracy or calibration state where the platform supplies one.

The app requests a bounded sample interval and a set of sensors. The backend clamps an interval the platform cannot provide and reports the effective interval once. It may drop samples under back-pressure, but it must preserve the latest sample and report whether a stream is throttled; it must not silently allocate an unbounded queue.

Apple maps `CMMotionManager` and `CMDeviceMotion` data without changing axes. Android maps `SensorManager` events and documents its sensor coordinate system and event timestamps. Device orientation is metadata, not a hidden transform. The API does not promise identical noise, calibration or rate across platforms.

Most motion sensors need no user prompt, while activity recognition, body sensors and target-specific sensors may. The backend reports each required authorization separately. An app can stop a high-rate stream without changing location or notification permission.

## Power, privacy and capabilities

High accuracy and high sample rates cost battery. The API requires an explicit stream and rate, uses provider batching where the target supports it, and exposes a stop operation. Background policies are opt-in and bounded by a declared purpose and budget. The service never uploads samples or writes them to storage on the app's behalf.

Location and motion data use a dedicated capability, distinct from network, contacts and calendar. The final access model must decide whether location and motion are separate grants. A denied scope receives no coordinates or sensor arrays, and diagnostics contain only stable error codes, never a location.

## Verification

The implementation is complete only when these checks exist:

- headless `core` tests cover authorization, one-shot versus stream ownership, cancellation, sample ordering, missing fields, rate clamping and bounded back-pressure;
- `internal/driver` tests cover coordinate encoding, timestamps, empty sensor sets and large batches;
- Apple tests use fake location and motion managers for permission changes, reduced accuracy, unavailable sensors and stop-on-disposal;
- Android tests use fake location and sensor providers for coarse/fine permission, provider changes, sensor accuracy and API-level differences; and
- target tests use recorded fixtures or simulator providers, never a user's live location, and verify no sample reaches a disposed scope.

## Open decisions

These decisions need confirmation before an implementation chooses them:

1. Can a one-shot location use a cached fix, and what maximum age is acceptable?
2. Which location authorization modes and background guarantees are in the first release?
3. Is the public motion API raw sensors only, or does it include fused device motion from the start?
4. Does the capability model separate location from motion and foreground from background access?
5. Are dropped high-rate samples reported as an explicit event, a counter, or only through effective-rate metadata?
