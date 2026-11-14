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

The first implementation covers APNs on iOS and macOS and FCM on Android. Windows, Linux, the web, tvOS and watchOS report that remote push is unavailable until they have a provider with the same lifecycle guarantees. Local notifications, badges and notification presses remain the existing service in [`services.md`](services.md#notifications-and-the-badge).

## Layers

The service has three layers:

1. `core` exposes a provider-neutral registration and delivery API. It owns scoped handlers, pending events and the engine-goroutine boundary. Its values are opaque tokens and payload JSON; it does not know whether a token came from APNs or FCM.
2. `internal/driver` carries commands to register or unregister and events for a token change or an incoming payload. Events include enough metadata to distinguish a foreground delivery from a launch delivery, but the payload is kept unchanged.
3. A platform backend translates the provider callback into the driver event. Apple calls APNs through the system frameworks. Android calls an adapter supplied by the runtime or an embedding host.

The Android adapter is a deliberate boundary. The core and the prebuilt runtime must not import Firebase classes. FCM's client library, `google-services.json` or an equivalent provider configuration, manifest entries, and background service are selected by the app's Android build or by its embedding host. The adapter forwards only a token, a payload and an acknowledgement result through the runtime protocol. This keeps the default NSGo build reproducible and does not require an Android SDK or a JDK on a user's machine. A future tool change may package a tested FCM adapter, but that is a separate decision and artifact.

An embedding host uses the same boundary: it owns provider setup and hands tokens and payloads to NSGo through an entry point of the embedding API that does not exist yet, beside `nsgo.NewView` and the iOS and Android faces. `core.Host`'s JSON channel ([embedding](embedding.md#the-host-channel)) carries the app's own names, not driver events, so it is not that entry point. The entry point feeds the same per-process queue as a backend, so a host cannot bypass `core`'s access checks or deliver a payload to a disposed scope.

## Registration and delivery

Registration is idempotent and belongs to the process, not an engine. Starting it again does not create another provider registration, and stopping it removes the provider callback when the last engine using it ends. A token may change at any time; every new value is delivered, including a value received before a handler is installed. Tokens are opaque strings and are not compared across providers. An app decides when to replace the token on its server.

A standalone app runs one engine, but an embedding host may show several views, each with an engine of its own ([embedding](embedding.md#views)), and on Android each in a process of its own. A new token goes to the handlers of every engine. A payload is kept in one queue per app process and delivered once, never once per engine: to the first handler installed among the engines, the queue entry removed only when that engine has accepted it. Handlers are scoped like `OnNotificationPress`: a handler is removed with its owner, and a payload received before any handler is installed stays queued until one is. Delivery runs on the receiving engine's goroutine. A handler must finish quickly and hand long work to the app's own queue.

Delivery cannot count on an engine running. An engine starts with its surface: on iOS when a scene connects (`sceneWillConnect` in `internal/platform/ios`), on Android when an `NsgoView` starts the app's Go process, and in a host when it shows a view. A background push launches an iOS app with no scene, as a Live Activity's intent does, which is why `widget.OnAction` is set before `nsgo.Run` ([tool](tool.md#widgets)); FCM starts Android's Java process with no `NsgoView`, so no Go code runs there at all. Without an engine the backend writes the payload to the queue on disk, so it outlives the process, and the next engine to start receives it as a launch delivery. Running app code for a background push with no view needs more: a process-wide handler set before `nsgo.Run`, as `widget.OnAction` is, which iOS can call but Android cannot without starting the Go process with no view, or a new viewless start. That is open decision 3.

The provider acknowledgement is separate from app handling. The backend acknowledges the platform callback after it has copied the payload into the queue, on disk when no engine runs; the app's handler is not allowed to hold APNs or FCM's callback thread. If a process cannot start, the provider or adapter keeps the platform's normal retry policy. A force-quit, revoked permission, expired token or provider-specific delivery drop is reported as unavailable or lost delivery, not retried forever by NSGo.

Neither platform hands the app the bytes the server sent. Apple gives the delegate the payload already parsed, as an `NSDictionary` (`userInfo`), and FCM gives `RemoteMessage.getData()`, a map of strings, with the notification block parsed into `RemoteMessage.Notification`. The backend therefore encodes what it receives as JSON, and that JSON, plus a small set of transport metadata, is the payload: on Apple the `userInfo` dictionary through `NSJSONSerialization`, which keeps neither the server's key order nor its spelling of numbers; on Android an object of the data's string values, with the notification fields beside it when FCM gives them. The two shapes differ because the providers' do. A server that needs exact bytes, such as for a signature, sends them as one string value. NSGo does not merge APNs' `aps` object with FCM's notification fields or turn arbitrary data into a `core.Notification`. An app may call `ShowNotification` after decoding its own data. This leaves notification presentation, deduplication and authentication in app code while keeping fields a provider adds later.

## Apple

The Apple backend calls `registerForRemoteNotifications`, on `UIApplication` on iOS and `NSApplication` on macOS, without waiting for notification authorization: APNs issues the token whatever the user answers, and a background push needs no authorization, so registration and permission are separate results, as on Android. The application delegate receives the binary device token (`application:didRegisterForRemoteNotificationsWithDeviceToken:`) and converts it to a stable lowercase hexadecimal string before emitting the driver event. This is the device's token; a Live Activity's push tokens, which `widget.OnPushToken` gives, are ActivityKit's and separate.

On iOS, `application:didReceiveRemoteNotification:fetchCompletionHandler:` forwards foreground and background payloads. The backend calls the completion handler once the payload is in the queue, on disk when no engine runs, with `UIBackgroundFetchResultNewData` when a handler accepted it and `UIBackgroundFetchResultNoData` when it is only queued, as iOS may suspend the process as soon as it is called. AppKit's `application:didReceiveRemoteNotification:` has no completion handler, and macOS has no background modes.

The tool must make the app's intent explicit in YAML, and adds nothing because local notifications are used. A package that uses remote push claims the push entitlement, `aps-environment` on iOS and `com.apple.developer.aps-environment` on macOS. An iOS package that needs background data delivery also lists `remote-notification` in its Info.plist's `UIBackgroundModes`, an array, which `apple.info`'s string entries cannot express. Only a provisioning profile grants the entitlement: on iOS the device's profile must include push, and on macOS the tool embeds a profile only in a Mac App Store package (`sign.macos_profile`), so a Developer ID disk image needs a Developer ID profile embedded as well, while `nsgo dev`'s and ad hoc builds claim no push entitlement, since macOS refuses to launch an app claiming it without a profile. The Apple backend reports unavailable when the package is not an app bundle or has no push entitlement.

The system may display a notification payload without launching the app. In that case NSGo receives no data event until Apple delivers one. A notification press continues through the existing notification-press path; remote push does not create a second press API.

## Android

The Android adapter owns `FirebaseMessagingService` (or a compatible provider implementation), token refresh, the notification-channel policy and the service's process lifetime. When Android starts the adapter in a process with no `NsgoView`, it persists the copied payload in the runtime's queue. The runtime cannot yet start the app's Go process without a view, so the payload waits for the next `NsgoView` unless open decision 3 adds such a start. Once a Go process is ready, the runtime emits the same driver event as a foreground delivery and removes the queue entry only after that process has accepted it.

The adapter must distinguish notification-only, data-only and mixed FCM messages. Notification-only messages may be displayed by FCM while the app is stopped and do not necessarily reach the app; mixed messages follow FCM's documented foreground/background rules. NSGo must document those provider rules instead of promising delivery that FCM cannot provide. A message that opens the app uses the existing notification press path when it has a notification ID or deep-link data.

Android 13 notification permission remains the existing `POST_NOTIFICATIONS` manifest and runtime permission. A provider token can exist while that permission is denied; registration and permission are therefore separate results. The adapter reports a token even when the app cannot show a notification, while `ShowNotification` keeps its current permission behavior.

## Configuration and security

Configuration is YAML and names intent, not secrets. The proposed shape is an `android.push` provider selection and an `apple.push`/background declaration; the exact keys remain open until the tool and runtime artifact are designed together. Provider credentials, server keys, `google-services.json`, APNs private keys and access tokens never enter `core`, generated Go, a committed runtime artifact or an issue report.

The capability check uses `AccessNotifications` for the user-visible notification and badge. Delivery of application data is a separate privileged operation, so the final API must decide whether it uses a new `AccessPush` capability or requires both `AccessNotifications` and an app-provided host grant. A package that lacks the grant must receive neither a token nor payload bytes.

## Verification

The implementation is complete only when these checks exist:

- headless `core` tests cover permission, idempotent registration, token replacement, pending delivery, scope cleanup, provider metadata and a payload reaching the handler as the backend passed it;
- `internal/driver` tests cover command and event encoding, including a payload larger than one bridge frame;
- Apple tests use a fake delegate to cover token conversion on iOS and macOS, the iOS completion handler's result with and without an engine, and a launch press;
- Android runtime tests use a fake adapter to cover a dead-process queue, acknowledgement, duplicate message IDs and token refresh without requiring Firebase or a device; and
- target tests cover a signed iOS app and a macOS app signed with a profile receiving a sandbox APNs payload, including an iOS app launched in the background, and an Android app receiving a test-provider payload, with the provider credentials kept outside the repository.

## Open decisions

These decisions need confirmation before an implementation chooses them:

1. Should the public API expose one `OnPush` callback with the payload's JSON, or separate token and message objects with a typed acknowledgement?
2. Is an Android FCM adapter part of `nsgo build`, supplied as a checked-in prebuilt artifact, or supplied by each app/embedding host?
3. Should a background data push run app code with no view, through a process-wide handler set before `nsgo.Run` on Apple and a viewless start of the Go process on Android, and if so by default, or only when an app declares a background policy and a bounded execution budget?
4. Does remote data delivery get a new capability distinct from `AccessNotifications`?
5. Which provider metadata is stable enough to expose (message ID, collapse key, delivery time), and which stays inside the adapter?
