# Foldable and dual-screen displays

This is the design for issue [#49](https://github.com/nsgo-dev/nsgo/issues/49). It is not an implemented API. Fold and hinge geometry belongs in the shared driver environment so an app can reason about the usable regions of a surface without importing Android or vendor types.

## Environment model

The proposed driver value adds a list of display regions to a surface environment. Each region has a frame in surface points, a safe area, and an optional reserved rectangle that the app must not place content across. A fold or hinge is an obstruction with a posture (`flat`, `half-open`, `separating` or `occluding`) and a spanning direction. The model describes geometry; it does not choose a two-pane layout.

Non-foldable targets report one region and no obstructions. A region list is ordered in reading direction only when the platform provides one; otherwise it is stable by frame position. Frames are in the same coordinate space as `Tree.Resize`, and the backend reports a new environment before the resize that follows a posture or display change.

The model must distinguish an occluding hinge, where pixels cannot be shown, from a separating fold, where two regions remain visible but a continuous layout is undesirable. It must also distinguish the hinge from a system bar or cutout, which continues to use `SafeArea`. Apps can therefore choose a split, a spanning layout with a gap, or a single-region layout explicitly.

## Layers

`internal/driver` owns the serializable geometry and posture enums. `core.Env` exposes the value as a signal and keeps its existing device-neutral behavior. `ui` and `base` do not switch forms merely because a device has a fold; an app or component author reads the regions and chooses its own navigation and tab-bar placement.

The backend translates platform geometry into the shared model. Android uses Jetpack WindowManager's `FoldingFeature` and window metrics, including display changes while an activity is resized. Other targets report the one-region form until they have an equivalent system concept. Vendor-specific labels and hinge sensors remain backend metadata unless they affect usable geometry.

An environment update is atomic: regions, obstructions, safe areas, posture and surface size describe one observation. A backend drops an observation that cannot be reconciled with the current surface, rather than sending a hinge frame in a previous coordinate space. A surface that detaches keeps its last observation but reports no new posture events until it attaches again.

## Layout and input

The core's layout remains continuous in the surface's coordinate space. A reserved rectangle clips or rejects content only when the app's own layout asks it to; core does not insert a gap, duplicate a navigation bar or move a tab bar implicitly. `SafeArea` continues to protect system bars, while the new reserved regions are available for an app's own constraints and hit testing.

Pointer and touch coordinates remain surface coordinates. The backend does not remap a touch from one region to another. An app can use a region's frame to decide which pane owns a press, and the core's existing hit testing then follows the resulting tree.

## Verification

The implementation is complete only when these checks exist:

- headless `core` tests cover one-region defaults, atomic environment updates, region ordering, reserved and occluding obstructions, surface detachment and signals that change only when geometry changes;
- `internal/driver` tests cover serialization, empty lists, posture transitions, points conversion and frames outside the surface;
- Android tests use recorded WindowManager observations for flat, half-open, separating, occluding, rotation and multi-window states; and
- a target test verifies an app can place two explicit panes and an edge-adaptable tab bar without any implicit device-specific behavior from core or base.

## Open decisions

These decisions need confirmation before an implementation chooses them:

1. Is the public value a list of usable regions plus obstructions, or a higher-level fold posture with derived regions?
2. Are reserved regions only geometry, or should core also offer an opt-in layout constraint that avoids them?
3. Which postures and hinge metadata are stable enough to expose across Android vendors?
4. Does the first release support only Android WindowManager, or also desktop multi-display arrangements with disjoint regions?
5. Should the adaptive tab bar be a `ui` component change, a gallery/app pattern, or a separate component after the environment model lands?
