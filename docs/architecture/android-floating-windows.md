# System floating windows on Android

This is the design for issue [#53](https://github.com/nsgo-dev/nsgo/issues/53). It is not an implemented API. An Android system overlay is a window owned by `WindowManager`, outside the app's activity, and is distinct from a core overlay inside an NSGo surface.

The feature depends on the process/view lifetime split in [#12](https://github.com/nsgo-dev/nsgo/issues/12) and shares foreground-service decisions with long recording in [#38](https://github.com/nsgo-dev/nsgo/issues/38).

## The surface it is

The overlay is a surface the app opens, as `desktop.OpenPanel` opens one on macOS ([application and windows](application.md#surfaces)), with no commands or IDs of its own. The first scope is one overlay that can be shown, moved, resized within the system's bounds, focused when permitted and closed explicitly; it hosts an NSGo tree of the app's one process, and a future multi-overlay API must define ordering and resource budgets before it is added.

- `mobile` is the API's home, as only phones have it; `desktop` gets no entry point. `mobile.OpenOverlay` makes a tree and sends `ItemOpenSurface` with a `SurfaceConfig` whose `ID` is the tree's, of a kind of its own (`SurfaceOverlay`), since its touch, focus and permission are not a panel's. The runtime attaches the overlay's view with `engine.Engine.Attach`, and the tree has its own `Env`, as every opened surface's.
- Moving, resizing, showing, hiding and closing are the `ItemWindow` commands every surface takes: `WindowSetFrame`, `WindowShow`, `WindowHide` and `WindowDestroy`. A removal by the system, such as a revoked grant, ends the tree as a closed window's ends, reported once with its reason.
- Its owner is the opener's, as for every opened surface: it closes as that owner goes. Opened from the app's own scope (`nsgo.Run(App)`), it lives as long as the engine, which after #12 outlives the activity; opened from a view's scope, it closes with that view, so a detached activity leaves no orphaned overlay.
- The Android runtime owns the `WindowManager.LayoutParams`, the overlay's view, its touch flags and the service. No raw Android `View` or window token crosses into core.

## What Android needs first

The Android backend has none of this model yet. Phones open no windows, so the first tree is the one surface and `OpenWindow` and `OpenPanel` return nil; only macOS handles `ItemOpenSurface`; and `NsgoView` starts and owns the Go process, so one view talks to one process over its pipe. Before an overlay shows:

1. The Go process and its pipe belong to a host apart from any view (#12), to which the activity's view and the overlay's attach and from which they detach.
2. The pipe protocol names the surface each message is for, so one process drives several views, each with the `Engine` that `Attach` returns.
3. The Android backend takes the app's own surface, as macOS's `own`: `Run` starts the engine on a surface in no view, which applies `ItemOpenSurface`; the activity's view attaches as the main view's surface, and an overlay as another. `OpenWindow` and `OpenPanel` still return nil on phones.

## Permission and user consent

An app declares `SYSTEM_ALERT_WINDOW` and asks the user through the system's special-access settings. The API reports `unavailable`, `denied`, `granted` or `revoked`; it never opens the settings page without an explicit user action. A denied or revoked grant creates no window and exposes no overlay frame.

Android 13 notification permission is independent. A long-lived overlay service normally needs a visible notification, but the overlay API must report whether its chosen service policy can run without one. The tool adds only the permissions and service declarations the app names in YAML, and never adds `SYSTEM_ALERT_WINDOW` merely because a package imports the API.

Overlay content is subject to Android's touch and accessibility rules. The default is not touchable through the overlay and does not take focus; an app opts in to touch, focus or accessibility separately. The runtime must keep the overlay's touch region bounded and must not set flags that let it obscure security-sensitive system prompts without the app's explicit declaration.

## Service and lifecycle

The runtime starts a foreground service only after the app has permission and requested an overlay. The service owns the engine connection, window manager and persistent state needed to reattach after an activity is recreated. Starting from the background follows the Android version's restrictions and the app's declared foreground-service type; a rejected start is an explicit error.

The service may survive an activity rotation, task removal or a detached `NsgoView`, but not an app shutdown requested by the app. The background policy defines when a process may be restarted after it is killed, how much state is restored, and whether the overlay is shown before the main view attaches. Until #12 defines those guarantees, the backend reports the overlay as unavailable after process death rather than silently reconstructing a partial tree.

The app hears the overlay shown, hidden, focused and closed. A system revocation or user swipe that removes the service emits one final close. The engine callback is bounded and never runs on the service's binder or window-manager thread.

## Layout and input

An overlay's frame is in points, as every window's (`SurfaceConfig.Width` and `Height`, `WindowFrame`), and the runtime converts it to pixels with the display's density at the Java boundary, as `NsgoView` converts sizes and safe areas. The backend keeps a requested frame on the display's usable area, as `WindowFrame.OnScreen` keeps a window, and reports the frame it applied. Rotation, display changes, cutouts and system bars trigger an environment update before the next commit.

The default window level is the platform overlay level. The API does not offer arbitrary z-order, transparent click-through regions or hidden system-window types. Dragging and resizing are explicit gestures in the overlay tree, and the runtime applies them through `WindowManager.updateViewLayout` rather than stepping coordinates from pointer events.

## Security and data

Overlay content is a high-risk capability because it can obscure other apps or capture touches. The final access model must decide whether it is a distinct `AccessOverlay` grant and whether a package can request it at all. The runtime records no screenshots, keystrokes or other apps' content. An overlay may receive only its own touch events and the app's declared host messages.

## Other targets and tests

- iOS has no window over other apps: `OverlayPermission` answers unavailable and `OpenOverlay` returns nil, whose methods do nothing, as `OpenPanel` does on phones. Picture in Picture (#54) and Live Activities keep their own APIs.
- macOS, Windows, Linux and the web answer unavailable too. A desktop's floating window is `desktop.OpenPanel`, which macOS shows at the status level on every space.
- `nsgotest` keeps the overlay permission, which a test sets and revokes, as it keeps notifications', and models an opened overlay as a surface with its frame, shown, hidden and closed, so an app's tests assert that it opened or that the permission was missing.

## Verification

The implementation is complete only when these checks exist:

- headless core tests cover permission state, owner disposal, surface attach/detach, frame clamping and lifecycle events;
- `nsgotest` tests open, move, hide and close an overlay, and find it refused without the permission and closed after a revocation;
- Android runtime tests use a fake `WindowManager` and service to cover permission revocation, activity recreation, process death, duplicate close and binder-thread isolation;
- API-level tests cover the special-access settings flow and foreground-service restrictions without relying on a real user's overlay permission; and
- a device test checks rotation, cutouts, touch routing, accessibility and removal from system settings, with no unrelated app content captured.

## Open decisions

These decisions need confirmation before an implementation chooses them:

1. Is the first release one overlay surface per app, or a collection of independently owned surfaces?
2. Which foreground-service type and notification policy keep the overlay valid on Android 14 and later?
3. Does process death restore the overlay automatically, or only after the app explicitly reopens it?
4. Is system overlay a separate `AccessOverlay` capability, and can sandboxed packages use it?
5. Which input and accessibility modes are supported initially: passive, touchable, focusable and screen-reader visible?
6. Does iOS's unavailable answer point apps to Picture in Picture and Live Activities, and do desktops share this API with `desktop.OpenPanel` rather than answer unavailable?
