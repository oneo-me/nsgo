# Foldable and dual-screen displays

This is the design for issue [#49](https://github.com/nsgo-dev/nsgo/issues/49). It is not an implemented API. Fold and hinge geometry belongs in the shared driver model, reported with each surface's size, so an app can reason about the usable regions of a surface without importing Android or vendor types.

## Environment model

The proposed driver value adds a list of display regions to what a surface reports. Each region has a frame in surface points, a safe area, and an optional reserved rectangle that the app must not place content across. A fold or hinge is an obstruction: a frame, an orientation (vertical or horizontal) and three properties that vary independently, as they do in both platforms' models. `Occluding` says the pixels under it are not shown or touched (Android's `occlusionType` `FULL`, UIKit's occlusion region kind). `Separating` says the regions on either side read as logically apart (Android's `isSeparating`, UIKit's division region kind). `State` is the hinge's `Unknown`, `Closed`, `HalfOpen` or `Open`, open as far as the device goes, which Android calls flat (Android's `FLAT` and `HALF_OPENED`, `UIHingeStatus`), with an angle where the platform reports one. A hinge between two panels lying open is occluding, separating and open at once; a flexible screen half folded is separating and half open but not occluding; the same screen opened flat is neither. The model describes geometry; it does not choose a two-pane layout.

Non-foldable targets report one region and no obstructions. A region list is ordered in reading direction only when the platform provides one; otherwise it is stable by frame position. Frames are in the same coordinate space as `Tree.Resize`, measured at the surface size the geometry travels with ([updates](#updates)).

Because the properties are separate, an app can tell an occluding hinge, where pixels cannot be shown, from a separating fold, where both sides remain visible but a continuous layout is undesirable, and a backend never has to drop one property to report another. The model must also distinguish the hinge from a system bar or cutout, which continues to use `SafeArea`. Apps can therefore choose a split, a spanning layout with a gap, or a single-region layout explicitly.

## Layers

`internal/driver` owns the serializable geometry and the state enum. `core.Env` exposes the value as a signal and keeps its existing device-neutral behavior. `ui` and `base` do not switch forms merely because a device has a fold; an app or component author reads the regions and chooses its own navigation and tab-bar placement.

## Platforms

The backend translates platform geometry into the shared model. Vendor-specific labels and hinge sensors remain backend metadata unless they affect usable geometry. Targets without a system concept of a fold report the one-region form.

Android has no fold API in `android.jar`, which is all the Java runtime compiles against (`internal/platform/android/runtime/build.sh`): `FoldingFeature` and `WindowLayoutInfo` come from Jetpack's `androidx.window:window`, a Kotlin library that pulls in the Kotlin standard library, coroutines and `androidx.core`. Bundling it would put those classes into the committed `classes.dex` and into the `classes.jar` that `nsgo build -embed` hosts dex themselves, where a host app that already uses `androidx.window` gets duplicate classes. The proposed route is the one Jetpack itself sits on: the device's `androidx.window.extensions` library, which the OEM ships, declared with `<uses-library android:name="androidx.window.extensions" android:required="false"/>` in the generated APK's manifest and the `-embed` AAR's, and reached by reflection from the runtime: `WindowExtensionsProvider.getWindowExtensions()`, its `WindowLayoutComponent` and `addWindowLayoutInfoListener`, whose `FoldingFeature` gives bounds, a type (`TYPE_FOLD` or `TYPE_HINGE`) and a state (`STATE_FLAT` or `STATE_HALF_OPENED`). The runtime derives the properties as Jetpack's `HardwareFoldingFeature` does: occluding when the bounds have area, separating for a hinge or a half-open fold. Where the library or the component is missing, the runtime reports the one-region form. The listener reports through the same path as the surface's size, including display changes while an activity is resized.

iOS has the concept from 27.1, in the SDK's UIKit headers: `-[UIView reservedRegionsOfKind:]` (and `reservedRegionsOfKind:options:`) returns `UIViewReservedRegion`s of `occlusionRegionKind` or `divisionRegionKind`, each with a `frame` in the view's coordinates that includes its `margins` for interactive content, and `isActive`; `UIHingeInteraction` reports a `UIHinge` whose `status` is `Unknown`, `Closed`, `PartiallyOpen` or `FullyOpen`, with its `angle`. The backend reads the surface view's active regions in its `layoutSubviews`, where it reports the size now, so they go with the resize: an occlusion region becomes an occluding obstruction, a division region a separating one, and the hinge interaction's updates set the state, a change of state alone being a resize to the same size. Whether a hinge comes as one region of each kind at the same frame, and whether the system's vertical bar comes as an occlusion region or only through the safe area, is to be checked on a fold simulator or device before the mapping is fixed. The trait collection's `verticalBarEdge` (`UIVerticalBarEdge`: unspecified, leading or trailing) is the edge the system chose for a vertical bar; it is not geometry, so it goes into `driver.Environment` and arrives through `SetEnvironment` from `traitCollectionDidChange:`. The backend reaches all of them through purego's Objective-C runtime as it does the rest of UIKit, checking `respondsToSelector:` first, so earlier systems report the one-region form.

The rest of #49 is packaging: the generated `Info.plist` hardcodes `UIRequiresFullScreen` true and `UIApplicationSupportsMultipleScenes` false (`cmd/nsgo/ios.go`), and `apple.info` takes strings only, so neither can be overridden; the [roadmap](../roadmap.md#foldable-displays) records it.

## Updates

The geometry and the surface size change together, so they travel together. `driver.Environment` keeps the size out (`Tree.Resize` reports it), and the engine merges a queued resize into the slot of an older one, which can move it ahead of an environment pushed before it; geometry sent through `SetEnvironment` could therefore be applied at a size it was not measured at. The geometry is instead part of the resize: the message `Engine.Resize` and `ResizeAndWait` push gains it, and a change of state at the same size, such as a fold half closing, is a resize to the same size with a new geometry. Merging queued resizes then keeps the newest size and geometry as one, and `Tree.Resize` sets `Env().window` and the geometry in one batch, where today it sets the size outside the batch `SetEnvironment` uses and ignores a size it already has, so what reads either sees both new and a new geometry at the same size still applies.

When the backend reports a size its latest platform observation was not measured at, it sends the one-region form for the new size until the platform reports for it, rather than frames in the previous coordinate space. Safe areas and the rest of the environment keep arriving through `SetEnvironment`. A surface that detaches keeps its last geometry but reports no new state until it attaches again.

## Layout and input

The core's layout remains continuous in the surface's coordinate space. A reserved rectangle clips or rejects content only when the app's own layout asks it to; core does not insert a gap, duplicate a navigation bar or move a tab bar implicitly. `SafeArea` continues to protect system bars, while the new reserved regions are available for an app's own constraints and hit testing.

Pointer and touch coordinates remain surface coordinates. The backend does not remap a touch from one region to another. An app can use a region's frame to decide which pane owns a press, and the core's existing hit testing then follows the resulting tree.

## Verification

The implementation is complete only when these checks exist:

- headless `core` tests cover one-region defaults, the size and geometry changing in one batch, region ordering, reserved and occluding obstructions, surface detachment and signals that change only when geometry changes;
- `internal/engine` tests cover queued resizes merging with their geometry, including a resize merged ahead of an environment pushed before it;
- `internal/driver` tests cover serialization, empty lists, posture transitions, points conversion and frames outside the surface;
- iOS tests use recorded reserved regions and hinge updates for an open, a half-open and a closed hinge, rotation and a vertical bar on either edge, and an older system without the selectors;
- Android tests use recorded WindowManager observations for an open fold, a half-open fold, an open hinge between two panels (occluding and separating together), rotation and multi-window states; and
- a target test verifies an app can place two explicit panes on either side of a separating obstruction without any implicit device-specific behavior from core or base.

An edge-adaptable tab bar, `mobile.TabBar` taking a leading or trailing edge where today it has only the system's bottom form (`core/tabbar.go`), follows once the model lands and decision 5 is answered; it is a separate [roadmap](../roadmap.md#foldable-displays) item.

## Open decisions

These decisions need confirmation before an implementation chooses them:

1. Is the public value a list of usable regions plus obstructions, or a higher-level fold posture with derived regions?
2. Are reserved regions only geometry, or should core also offer an opt-in layout constraint that avoids them?
3. Which hinge metadata, such as the state's values and the angle, is stable enough to expose across Android vendors and iOS?
4. Does the first release cover Android and iOS 27.1 alone, or also desktop multi-display arrangements with disjoint regions?
5. Should the adaptive tab bar be a `ui` component change, a gallery/app pattern, or a separate component after the environment model lands?
6. Does Android reach folds through the device's `androidx.window.extensions` by reflection, as proposed, or by bundling `androidx.window` and its Kotlin dependencies into the runtime?
