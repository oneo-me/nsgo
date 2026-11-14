# Android engine and view lifetime

This document is the design for issue #12. It is a dependency for Android
recording, floating windows and push delivery. The first stage now has a
process and pipe owner (`NsgoEngine`) plus one-surface attach and detach on
`NsgoView`; the protocol still supports only one surface at a time. A fresh
Activity view cannot yet resync the existing Go tree, so the complete
Activity-recreation guarantee remains a later protocol stage. Multiple
surfaces and process-level routing remain planned work described below.

## Ownership

The Android face will have two objects with different owners:

```text
Application or foreground service
                │
          NsgoEngine
       process, pipes, host
          │            │
     NsgoView A    NsgoView B
   Activity surface  another surface
```

`NsgoEngine` owns the Go process, its standard-input and standard-output
pipes, the reader and writer threads, the process data and cache directories,
the process-wide host channel, and the registry of attached surfaces. It is
created with an application context and keeps no `Activity`, `Window`, `View`
or other UI object. An application or service calls `destroy` explicitly; an
Activity being recreated or finished does not destroy the engine.

`NsgoView` owns one Android surface: its mounted nodes, text layouts, image
decoders, embedded native components, input listeners, insets and window
callbacks. It may attach to an engine while its Activity is visible and must
detach before the Activity releases it. A detached view releases its native
children and UI callbacks but leaves the engine and the app graph alive.

The first implementation allows one attached view at a time while the
protocol migration is in progress. The completed design supports several
attached views, each with its own surface tree and environment, sharing the
same Go process and application state.

## State machines

The engine has these transitions:

```text
New ──start──▶ Running ──last detach──▶ Background
                 │                         │
                 └────destroy──────────────┘
                 │                         │
                 └────process exit──────▶ Failed
```

`Background` means that no surface is attached. It does not promise that
Android will keep the process alive. A host that needs work while no Activity
is visible must own a foreground service, declare the matching service type
and keep its `NsgoEngine` there. The engine does not silently create a service
or hold an Activity context on the host's behalf.

Each view has an attach token and these transitions:

```text
Detached ──attach──▶ Attached ──detach──▶ Detached
```

Attach sends the surface's size, density, insets, appearance, locale and
font scale before the first batch. Detach sends a surface-detach command,
cancels pending UI work and destroys only that surface's native children.
Reattach sends a fresh environment and resize; the Go engine rebuilds or
reconciles that tree without resetting process-wide signals, storage or host
state.

`destroy` is idempotent. It marks the engine closed, rejects new writes,
detaches every surface, closes the pipes, waits for the process and releases
the reader and writer resources. A process that exits by itself marks the
engine failed and notifies each attached view; it is never restarted
implicitly, because a service must decide whether and when to recover it.

## Protocol changes

The current pipe protocol assumes one surface, so a lifecycle split cannot be
implemented by moving `Process` to another Java class alone. The protocol
needs:

- an engine handshake that assigns a protocol version and an engine ID;
- `SurfaceAttach`, carrying a new surface ID and its initial environment;
- `SurfaceDetach`, which lets Go stop committing that tree while retaining
  the app and its other trees;
- a surface ID on every batch, event, image, text reply, folder reply and
  host call that belongs to a surface;
- an explicit process-level channel for storage, network, host handlers and
  lifecycle notifications;
- a `SurfaceResync` request for a view that was detached while batches were
  queued, so reattachment cannot apply stale native nodes.

Messages from Java are serialized by the engine writer. A surface callback
never writes to the process directly while holding an Android view lock. Go
serializes commits by surface, and a detached surface's queued batches are
dropped after its attach token changes.

Process-wide host calls keep their current ordering and timeout. Surface host
events carry the surface ID; an application may still choose to route them to
one process-wide handler. A service can therefore receive a background event
without manufacturing a hidden `NsgoView`.

## Configuration and compatibility

The planned Java API is deliberately small:

```java
NsgoEngine engine = new NsgoEngine(applicationContext, options);
NsgoView view = new NsgoView(activity);
view.attachToEngine(engine);
// Activity recreation:
view.detachFromEngine();
// Later, from the new Activity:
newView.attachToEngine(engine);
// Application or service shutdown:
engine.destroy();
```

The existing `new NsgoView(activity)` behavior remains a compatibility
convenience: it lazily creates a private engine and owns it until `stop` is
called. An embedding host that wants a process to outlive an Activity must
construct and retain `NsgoEngine`, then use `attachToEngine` and
`detachFromEngine`; calling `NsgoView.stop` on that view is not the shutdown
operation for the shared engine.

The engine's options carry the native library path, launch flags, data and
cache directories, host parameters and test configuration. Paths are copied
as immutable strings before start. The engine always uses the application
context for those directories, while a view uses its current Activity context
only for windows, permission results and UI services.

## Foreground services and multiple Activities

An embedding app places the engine in an `Application` or a foreground
service, not in an Activity field. The service owns the notification and its
Android lifecycle. For API 34 and later, recording adds
`FOREGROUND_SERVICE_MICROPHONE` and `foregroundServiceType="microphone"` to
the host manifest; playback and other work use the service type their host
declares. The runtime does not request permissions or start a service as a
side effect of attaching a view.

Two Activities may attach different surface IDs to the same engine. Their
trees share Go signals and storage but have independent size, focus, insets,
navigation and embedded native children. If an app needs two Activities to
show one logical tree, it chooses one surface and hands the other Activity a
new view attached to that surface; the engine never guesses which Activity is
current.

## Delivery stages

1. Move process and pipe ownership into `NsgoEngine`, add attach and detach
   for one surface, and keep the old single-view API working. This stage is
   implemented; its protocol still has one surface, drops messages while no
   view is attached and cannot resync a fresh view yet.
2. Add surface IDs and attach/detach/resync messages to the Java and Go
   protocols; make the headless protocol tests cover stale batches and
   process death.
3. Allow several attached views and add an Android embedding example with an
   Activity that is recreated while a foreground service keeps the engine.
4. Build recording, floating windows and push delivery on the process-level
   runtime and verify permission, service and shutdown behavior on Android.

The acceptance gate for this design is a test that destroys and recreates an
Activity without losing a signal, a second test that attaches two surfaces to
one process, and a service test that calls `destroy` exactly once. Device
verification must also show that a detached view releases its native children
while the Go process remains alive.
