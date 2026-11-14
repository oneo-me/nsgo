# System floating windows on Android

This is the design for issue [#53](https://github.com/nsgo-dev/nsgo/issues/53). It is not an implemented API. An Android system overlay is a window owned by `WindowManager`, outside the app's activity, and is distinct from a core overlay inside an NSGo surface.

The feature depends on the process/view lifetime split in [#12](https://github.com/nsgo-dev/nsgo/issues/12) and shares foreground-service decisions with long recording in [#38](https://github.com/nsgo-dev/nsgo/issues/38).

## Scope and ownership

The first scope is one app-owned overlay surface that can be shown, moved, resized within the system's bounds, focused when permitted and closed explicitly. It may host an NSGo tree, but it does not become a second independent app process. A future multi-overlay API must define ordering and resource budgets before it is added.

`core` owns the overlay's logical state and a stable surface ID. `desktop` or `mobile` owns the Android-specific entry point and options. `internal/driver` carries open, update, focus and close commands. The Android runtime owns the `WindowManager.LayoutParams`, the overlay `View`, touch policy and service connection. No raw Android `View` or window token crosses into core.

The overlay's owner is an app-level background component, not a view scope that disappears when an activity detaches. It may keep the engine alive after the last activity only under the lifecycle policy chosen for #12. Closing the owner removes the window and stops its service; a detached activity cannot accidentally leave an orphaned overlay.

## Permission and user consent

An app declares `SYSTEM_ALERT_WINDOW` and asks the user through the system's special-access settings. The API reports `unavailable`, `denied`, `granted` or `revoked`; it never opens the settings page without an explicit user action. A denied or revoked grant creates no window and exposes no overlay frame.

Android 13 notification permission is independent. A long-lived overlay service normally needs a visible notification, but the overlay API must report whether its chosen service policy can run without one. The tool adds only the permissions and service declarations the app names in YAML, and never adds `SYSTEM_ALERT_WINDOW` merely because a package imports the API.

Overlay content is subject to Android's touch and accessibility rules. The default is not touchable through the overlay and does not take focus; an app opts in to touch, focus or accessibility separately. The runtime must keep the overlay's touch region bounded and must not set flags that let it obscure security-sensitive system prompts without the app's explicit declaration.

## Service and lifecycle

The runtime starts a foreground service only after the app has permission and requested an overlay. The service owns the engine connection, window manager and persistent state needed to reattach after an activity is recreated. Starting from the background follows the Android version's restrictions and the app's declared foreground-service type; a rejected start is an explicit error.

The service may survive an activity rotation, task removal or a detached `NsgoView`, but not an app shutdown requested by the app. The background policy defines when a process may be restarted after it is killed, how much state is restored, and whether the overlay is shown before the main view attaches. Until #12 defines those guarantees, the backend reports the overlay as unavailable after process death rather than silently reconstructing a partial tree.

The app receives lifecycle events when the overlay becomes visible, hidden, focused or detached. A system revocation or user swipe that removes the service emits one final close event. The engine callback is bounded and never runs on the service's binder or window-manager thread.

## Layout and input

Overlay bounds are in physical pixels with the runtime's density exposed in the environment; the app's layout remains in NSGo points. The backend clamps a requested frame to the current display's safe region and reports the resulting frame. Rotation, display changes, cutouts and system bars trigger an environment update before the next commit.

The default window level is the platform overlay level. The API does not offer arbitrary z-order, transparent click-through regions or hidden system-window types. Dragging and resizing are explicit gestures in the overlay tree, and the runtime applies them through `WindowManager.updateViewLayout` rather than stepping coordinates from pointer events.

## Security and data

Overlay content is a high-risk capability because it can obscure other apps or capture touches. The final access model must decide whether it is a distinct `AccessOverlay` grant and whether a package can request it at all. The runtime records no screenshots, keystrokes or other apps' content. An overlay may receive only its own touch events and the app's declared host messages.

## Verification

The implementation is complete only when these checks exist:

- headless core tests cover permission state, owner disposal, surface attach/detach, frame clamping and lifecycle events;
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
