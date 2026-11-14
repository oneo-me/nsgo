# Remote push notifications

This is the design for issue [#59](https://github.com/nsgo-dev/nsgo/issues/59). It is not an implemented API. Remote push must be designed as one service across APNs and FCM without making the core depend on either vendor's SDK.

## Scope

Remote push is a server-to-device channel. An app needs to:

- ask for the system permission that allows a push to show a notification;
- register for a provider token and send that opaque token to its own server;
- receive data from a push while its process is running;
- receive the data that the platform gives it when a push starts or resumes the app; and
- let the platform display a notification payload when the app is not in the foreground.

The server, provider credentials, token storage and message encryption remain the app's responsibility. NSGo does not send a token to a service owned by NSGo and does not interpret application data.

The first implementation covers APNs on Apple targets and FCM on Android. Desktop and web backends report that remote push is unavailable until they have a provider with the same lifecycle guarantees. Local notifications, badges and notification presses remain the existing service in [`services.md`](services.md#notifications-and-the-badge).

## Layers

The service has three layers:

1. `core` exposes a provider-neutral registration and delivery API. It owns scoped handlers, pending events and the engine-goroutine boundary. Its values are opaque tokens and payload JSON; it does not know whether a token came from APNs or FCM.
2. `internal/driver` carries commands to register or unregister and events for a token change or an incoming payload. Events include enough metadata to distinguish a foreground delivery from a launch delivery, but the payload is kept unchanged.
3. A platform backend translates the provider callback into the driver event. Apple calls APNs through the system frameworks. Android calls an adapter supplied by the runtime or an embedding host.

The Android adapter is a deliberate boundary. The core and the prebuilt runtime must not import Firebase classes. FCM's client library, `google-services.json` or an equivalent provider configuration, manifest entries, and background service are selected by the app's Android build or by its embedding host. The adapter forwards only a token, a payload and an acknowledgement result through the runtime protocol. This keeps the default NSGo build reproducible and does not require an Android SDK or a JDK on a user's machine. A future tool change may package a tested FCM adapter, but that is a separate decision and artifact.

An embedding host uses the same boundary: it owns provider setup and injects the provider callbacks into the view's driver channel. A host cannot bypass `core`'s access checks or deliver a payload to a disposed scope.

## Registration and delivery

Registration is idempotent for an engine. Starting it again does not create another provider registration, and stopping it removes the provider callback when the last engine using it ends. A token may change at any time; every new value is delivered, including a value received before a handler is installed. Tokens are opaque strings and are not compared across providers. An app decides when to replace the token on its server.

Handlers are scoped like `OnNotificationPress`: a handler is removed with its owner, and a payload received before the app installs its first handler is retained until that first handler can receive it. Delivery runs on the engine goroutine. A handler must finish quickly and hand long work to the app's own queue.

The provider acknowledgement is separate from app handling. The backend acknowledges the platform callback after it has copied the payload into the engine's queue; the app's handler is not allowed to hold APNs or FCM's callback thread. If a process cannot start, the provider or adapter keeps the platform's normal retry policy. A force-quit, revoked permission, expired token or provider-specific delivery drop is reported as unavailable or lost delivery, not retried forever by NSGo.

Neither platform hands the app the bytes the server sent. Apple gives the delegate the payload already parsed, as an `NSDictionary` (`userInfo`), and FCM gives `RemoteMessage.getData()`, a map of strings, with the notification block parsed into `RemoteMessage.Notification`. The backend therefore encodes what it receives as JSON, and that JSON, plus a small set of transport metadata, is the payload: on Apple the `userInfo` dictionary through `NSJSONSerialization`, which keeps neither the server's key order nor its spelling of numbers; on Android an object of the data's string values, with the notification fields beside it when FCM gives them. The two shapes differ because the providers' do. A server that needs exact bytes, such as for a signature, sends them as one string value. NSGo does not merge APNs' `aps` object with FCM's notification fields or turn arbitrary data into a `core.Notification`. An app may call `ShowNotification` after decoding its own data. This leaves notification presentation, deduplication and authentication in app code while keeping fields a provider adds later.

## Apple

The Apple backend registers with `UIApplication`'s remote-notification API after the app has requested notification authorization. The application delegate receives the binary device token and converts it to a stable lowercase hexadecimal string before emitting the driver event. The delegate also forwards foreground and background payloads, and always calls Apple's completion handler after the payload has been copied.

The tool must make the app's intent explicit in YAML. A package that uses remote push opts into the push entitlement (`aps-environment`) and, when it needs background data delivery, the `remote-notification` background mode. The tool does not add either entitlement merely because local notifications are used. The Apple backend reports unavailable when the package is not an app bundle, has no push entitlement, or is running on a target without APNs.

The system may display a notification payload without launching the app. In that case NSGo receives no data event until Apple delivers one. A notification press continues through the existing notification-press path; remote push does not create a second press API.

## Android

The Android adapter owns `FirebaseMessagingService` (or a compatible provider implementation), token refresh, the notification-channel policy and the service's process lifetime. When Android starts the adapter in a dead process, it persists the copied payload in the app's runtime queue and starts the NSGo engine only under the app's declared background policy. Once the engine is ready, the adapter emits the same driver event as a foreground delivery and removes the queue entry only after the driver has accepted it.

The adapter must distinguish notification-only, data-only and mixed FCM messages. Notification-only messages may be displayed by FCM while the app is stopped and do not necessarily reach the app; mixed messages follow FCM's documented foreground/background rules. NSGo must document those provider rules instead of promising delivery that FCM cannot provide. A message that opens the app uses the existing notification press path when it has a notification ID or deep-link data.

Android 13 notification permission remains the existing `POST_NOTIFICATIONS` manifest and runtime permission. A provider token can exist while that permission is denied; registration and permission are therefore separate results. The adapter reports a token even when the app cannot show a notification, while `ShowNotification` keeps its current permission behavior.

## Configuration and security

Configuration is YAML and names intent, not secrets. The proposed shape is an `android.push` provider selection and an `apple.push`/background declaration; the exact keys remain open until the tool and runtime artifact are designed together. Provider credentials, server keys, `google-services.json`, APNs private keys and access tokens never enter `core`, generated Go, a committed runtime artifact or an issue report.

The capability check uses `AccessNotifications` for the user-visible notification and badge. Delivery of application data is a separate privileged operation, so the final API must decide whether it uses a new `AccessPush` capability or requires both `AccessNotifications` and an app-provided host grant. A package that lacks the grant must receive neither a token nor payload bytes.

## Verification

The implementation is complete only when these checks exist:

- headless `core` tests cover permission, idempotent registration, token replacement, pending delivery, scope cleanup, provider metadata and a payload reaching the handler as the backend passed it;
- `internal/driver` tests cover command and event encoding, including a payload larger than one bridge frame;
- Apple tests use a fake delegate to cover token conversion, foreground/background completion and a launch press;
- Android runtime tests use a fake adapter to cover a dead-process queue, acknowledgement, duplicate message IDs and token refresh without requiring Firebase or a device; and
- target tests cover a signed Apple app receiving a sandbox APNs payload and an Android app receiving a test-provider payload, with the provider credentials kept outside the repository.

## Open decisions

These decisions need confirmation before an implementation chooses them:

1. Should the public API expose one `OnPush` callback with the payload's JSON, or separate token and message objects with a typed acknowledgement?
2. Is an Android FCM adapter part of `nsgo build`, supplied as a checked-in prebuilt artifact, or supplied by each app/embedding host?
3. Should a background data push be allowed to start a stopped NSGo process by default, or only when an app declares a background policy and a bounded execution budget?
4. Does remote data delivery get a new capability distinct from `AccessNotifications`?
5. Which provider metadata is stable enough to expose (message ID, collapse key, delivery time), and which stays inside the adapter?
