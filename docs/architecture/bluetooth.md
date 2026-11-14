# Bluetooth Low Energy

This is the design for issue [#58](https://github.com/nsgo-dev/nsgo/issues/58). It is not an implemented API. The first scope is the central role: discover peripherals, connect to one, discover its GATT database, read and write characteristics, and subscribe to notifications. Peripheral advertising and acting as a GATT server are separate work.

Background scanning and connections depend on the Android engine/view lifetime work in [#12](https://github.com/nsgo-dev/nsgo/issues/12); this document does not promise a background process before that dependency is decided.

## Layers

The service has three layers:

1. `core` owns a provider-neutral session and the app-visible state machine. It exposes stable handles for a scan, connection, service, characteristic and subscription, runs callbacks on the engine goroutine and closes handles with their scope.
2. `internal/driver` carries commands and events with numeric handles, UUIDs and byte payloads. It does not expose CoreBluetooth objects, Android `BluetoothGatt` instances or platform addresses.
3. A backend owns the platform queue and translates state and errors into driver events. `internal/platform/apple`, which macOS and iOS share, uses `CBCentralManager` and `CBPeripheral`; Android uses `BluetoothLeScanner` and `BluetoothGatt`.

Every operation is asynchronous. A backend never blocks the engine goroutine while scanning, connecting, discovering, reading, writing or subscribing. Bytes are copied at the driver boundary, so an app may reuse its input after a write returns. Notifications preserve their arrival order per characteristic; events from different characteristics may interleave.

## Targets

| Target | First scope |
| --- | --- |
| iOS, macOS | `internal/platform/apple`; the Info.plist declares `NSBluetoothAlwaysUsageDescription` (`apple.info`) |
| Android | The Java runtime, with `classes.dex` rebuilt |
| tvOS, watchOS, Windows, Linux, the web, mini programs | Unavailable, recorded in the [roadmap](../roadmap.md#finish-the-existing-platforms) |

An `Env` flag says the platform has the service, as `Env.CanRecognizeSpeech` does for speech; elsewhere `core` answers availability as unsupported and fails a scan or connection at once, without asking the platform. Windows (`Windows.Devices.Bluetooth`) and Linux (BlueZ over D-Bus) can follow. The web's Web Bluetooth picks one device in the browser's own chooser after a user gesture rather than scanning with the app's filters, so it would need an API of its own; mini programs' Bluetooth is each host's.

A Mac App Store build is sandboxed and needs `com.apple.security.device.bluetooth`. `sandboxEntitlements` (`cmd/nsgo/macstore.go`) adds only the microphone and camera entitlements from `apple.info`, so the tool should add Bluetooth's where the Info.plist declares `NSBluetoothAlwaysUsageDescription`; until then an app adds it through `macos.entitlements`.

## Identity and handles

The API cannot use a MAC address as its public identity: iOS does not expose one, and Android addresses can change or be restricted. A scan result therefore has a backend-generated device ID valid for the app's process and a display name, advertised service UUIDs, RSSI and connectability. The app may keep its own identifier in manufacturer data or a characteristic and rediscover it later.

Handles are opaque integers owned by one engine. A scan handle ends when stopped or its owner is disposed. A connection handle owns its discovered services and characteristics; closing it cancels pending operations, removes notification subscriptions and disconnects the peripheral. A stale event is ignored after the handle is closed. The backend may report a disconnect and a final error once, then all child handles become invalid.

The final API must choose whether a write or read callback identifies the operation explicitly or whether ordering on a characteristic is sufficient. The driver protocol should carry an operation ID even if the first public API does not expose it, so a late platform callback cannot answer a newer request by mistake.

## Scanning

Scanning takes filters that the app supplies: service UUIDs, manufacturer data and an optional name predicate. The backend does not scan continuously by default. Starting a second scan either replaces the first or returns an error; the final API must choose one behavior and document its power cost.

Results are coalesced by the backend's device ID and carry the latest RSSI and advertisement data. The core does not infer proximity, deduplicate by name or keep a device alive merely because it was seen. A scan can finish with a normal stop, an unavailable radio, a denied permission or a provider error.

On Apple, the radio's state exists only on a `CBCentralManager`, and creating the first one while the authorization is not determined shows the system's prompt. The backend therefore reads the permission from the class property `CBManager.authorization` without a manager, and creates its manager, with `CBCentralManagerOptionShowPowerAlertKey` false, only once the permission is granted or a scan, connection or permission request asks for it. A new manager starts in `unknown`, a temporary state in which CoreBluetooth ignores a scan, so the backend holds commands until the first state update; it then maps `poweredOff`, `unauthorized` and `unsupported` to stable core errors, and `resetting` fails the pending operations and closes the connections, as the system drops them. On Android, the backend checks the adapter and the runtime permissions before `BluetoothLeScanner.startScan`; it must not request a permission from a background callback.

## GATT operations

After connecting, the app asks for service and characteristic discovery. The backend reports the UUID, service type and characteristic properties without exposing platform objects. Reads and writes carry the characteristic handle and bytes. A write says whether it requests a response; the backend rejects a request the characteristic's properties do not allow before calling the platform.

Subscriptions are explicit, and each backend enables them the platform's way. Android calls `setCharacteristicNotification` and writes the client characteristic configuration descriptor (0x2902), choosing indications or notifications as the characteristic allows. Apple cannot write that descriptor: `writeValue:forDescriptor:` refuses it, so the backend calls `setNotifyValue:forCharacteristic:`, which enables notifications whenever the characteristic allows both. The app therefore never requests indications: the core only reports which were enabled. Each value event carries the characteristic handle and a copied byte slice. Unsubscribing is idempotent and is also performed when the connection closes.

Provider-specific limits stay in the backend: maximum write length, prepare-write support, MTU negotiation, retry behavior and descriptor ordering. The core may expose a negotiated maximum later, but it must not silently split an application message into writes without an API that reports boundaries and failure.

## Permissions and lifecycle

The API reports Bluetooth availability and authorization separately from a scan or connection error, and reading them never asks the user, as `NotificationPermission` reads the notification permission. As the other services do, the first scan or connection asks with the system's prompt when the permission is not determined yet, as `TakePhoto` asks for the camera and a speech session for the microphone, and an explicit request asks without scanning, as `RequestNotificationPermission` does. A denied permission fails the scan or connection with a permission error and asks no more.

Apple packages that use the service declare `NSBluetoothAlwaysUsageDescription` in their Info.plist; without it a request fails, since the system would end the app as it asks. Background central activity needs the `bluetooth-central` background mode, an array entry in `UIBackgroundModes` that `apple.info`, which holds strings only, cannot write yet, and remains subject to Apple's restoration and scheduling rules. The backend may restore a connection, but it cannot promise a scan or notification at an exact time while the app is suspended.

Android 12 and later require the `BLUETOOTH_SCAN` and `BLUETOOTH_CONNECT` runtime permissions; advertising would additionally require `BLUETOOTH_ADVERTISE`. The runtime's `minSdkVersion` is 28, and on API 28 to 30 BLE needs the install-time `BLUETOOTH` and `BLUETOOTH_ADMIN`, declared with `android:maxSdkVersion="30"`, and scans need `ACCESS_FINE_LOCATION` granted and find nothing while location is off. An app whose scans never derive location marks `BLUETOOTH_SCAN` with `android:usesPermissionFlags="neverForLocation"` and caps `ACCESS_FINE_LOCATION` at `maxSdkVersion` 30; that is a package policy choice, not something the core can add silently. The declarations belong in `nsgo.yaml` and the generated manifest, while the user prompt belongs to the Android host and its answer comes back through `permissionResult`, as the camera's does. Today `android.permissions` (`cmd/nsgo/project.go`) takes names only and `cmd/nsgo/apk/axml.go` writes only `android:name`, with neither `maxSdkVersion` nor `usesPermissionFlags` in its attribute table, so the tool must take those attributes before an app can declare these.

The Android backend must use the process/view split from #12. A visible view may own a connection without keeping a detached view alive. A future background owner may continue a connection only after the lifecycle design defines its execution budget, storage for pending events and app restart behavior. Until then the target reports background operation as unavailable rather than starting an unowned process.

## Security and data

Bluetooth bytes are untrusted input. The API does not decode a characteristic as text, JSON or an image, and it does not treat a device name as authentication. Apps verify their own protocol, pair or bond as the platform requires, enforce message size limits and clear sensitive buffers when their own policy needs it. A package capability for Bluetooth data is distinct from network access; the final access model must decide whether it is one `AccessBluetooth` capability or separate scan and connect grants.

## Verification

The implementation is complete only when these checks exist:

- headless `core` tests cover handle ownership, scan filters, coalesced results, connection state, operation cancellation, stale events, subscription cleanup and byte preservation;
- `internal/driver` tests cover protocol encoding, operation IDs, UUIDs, empty values and payloads larger than one bridge frame;
- Apple tests use fake central and peripheral delegates to cover authorization read without a manager, the `unknown` state before the first update, powered-off state, discovery, read/write ordering, subscriptions through `setNotifyValue:forCharacteristic:` and disconnects;
- Android tests use a fake scanner and GATT client to cover API 31 permissions, the install-time permissions and location-gated scans before it, adapter state, reconnect and cleanup without a device; and
- target tests use a controlled peripheral or provider simulator, never a user's nearby device, and verify that no callback runs after its owning scope is disposed.

## Open decisions

These decisions need confirmation before an implementation chooses them:

1. Is the first public API central-only, or should advertising and the peripheral role be part of the initial contract?
2. Does a second scan replace the first scan or return an error?
3. Should the public API expose operation IDs and negotiated MTU/write limits, or keep them backend details initially?
4. Does Bluetooth use one `AccessBluetooth` capability or separate scan/connect grants?
5. Which background guarantees are allowed after #12: restored connections, scans, notification delivery, or all three with separate policies?
