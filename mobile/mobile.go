// Package mobile holds what only iOS and Android have: the system's tab
// bar and a bar above the software keyboard, as NSH's nsh/mobile. It
// builds on every platform, so one app runs everywhere: elsewhere its
// components take no room, and IsAvailable says whether they show.
package mobile

import "github.com/nsgo-dev/nsgo/core"

type (
	// TabBar is the system's tab bar for an app's top-level sections:
	// UITabBar on iOS (Liquid Glass from iOS 26), a Material 3 navigation
	// bar on Android, each in the system's look. The app gives the tabs,
	// says which is selected, hears the user's choice with OnChange and
	// shows the selected tab's content; it places the bar with its style,
	// whose height is the system's where the style leaves it open.
	// Elsewhere it takes no room and shows nothing, and the app shows its
	// tabs with base.Tabs.
	TabBar = core.TabBar
	// TabBarItem is one tab of a TabBar.
	TabBarItem = core.TabBarItem
	// InputAccessoryView is a bar that rides above the software keyboard
	// while an Input naming its ID in InputAccessoryViewID has focus. It
	// is as wide as the surface and as tall as its content, and shows
	// nothing where it is written.
	InputAccessoryView = core.InputAccessoryView
)

// Haptic is feedback the device plays under the user's finger, from
// HapticSelection to HapticError.
type Haptic = core.Haptic

// The haptics PlayHaptic plays: a selection changing, an impact of a
// weight, and a task's outcome.
const (
	HapticSelection = core.HapticSelection
	HapticLight     = core.HapticLight
	HapticMedium    = core.HapticMedium
	HapticHeavy     = core.HapticHeavy
	HapticSoft      = core.HapticSoft
	HapticRigid     = core.HapticRigid
	HapticSuccess   = core.HapticSuccess
	HapticWarning   = core.HapticWarning
	HapticError     = core.HapticError
)

// PlayHaptic plays h: through UIKit's feedback generators on iOS, and
// on Android through the view's haptic feedback, its nearest constant,
// where the user's touch feedback setting allows it. Devices without
// haptics, and every other platform, play nothing.
func PlayHaptic(cx *core.Scope, h Haptic) { cx.PlayHaptic(h) }

// IsAvailable reports whether the app runs on iOS or Android, where this
// package's components show.
func IsAvailable(cx *core.Scope) bool {
	os := cx.Env().OS()
	return os == core.OSIOS || os == core.OSAndroid
}

// IsAppClip reports whether the app runs as an iOS App Clip, the part of
// an app nsgo.yaml's app_clip builds, which the system runs from a link,
// a code or an NFC tag without installing the app. An app whose clip is
// its own package shows the clip's screens when it does.
func IsAppClip(cx *core.Scope) bool { return cx.Env().AppClip() }

// OpenedURL returns the web link that last opened the app: on iOS an App
// Clip's invocation, such as https://example.com/order?table=12, which
// says what the clip shows; "" when no link opened it, and elsewhere. It
// changes as a link opens the running app again, so what reads it in a
// component follows.
func OpenedURL(cx *core.Scope) string { return cx.Env().OpenedURL() }

// ExitApp leaves the app on Android as its back button does when nothing
// in the app takes it: the task's root activity goes to the background,
// any other finishes. iOS apps never leave themselves, so it does nothing
// there, as elsewhere.
func ExitApp(cx *core.Scope) { cx.ExitApp() }

// InstallPackage hands the APK at path, in the app's own storage, to
// Android's package installer, which asks the user to confirm. done,
// when not nil, learns whether the installer started, with the error
// when it did not: a missing file, an app whose manifest does not ask
// for android.permission.REQUEST_INSTALL_PACKAGES (nsgo.yaml's
// android.permissions; an embedding host's own manifest), or a failed
// launch. That it started says nothing of whether the user installed
// the package. Where the user has not let the app install packages
// (CanInstallPackages), the installer sends them to the setting first.
// The installer reads the file through a content URI the runtime grants
// it, so the app keeps it until the installer is done and deletes it
// itself. Elsewhere it answers with an error. It never installs
// silently.
func InstallPackage(cx *core.Scope, path string, done func(started bool, err error)) {
	cx.InstallPackage(path, done)
}

// CanInstallPackages calls done with whether the user lets the app
// install packages on Android, as PackageManager.canRequestPackageInstalls
// says: false until they allow it in the setting OpenInstallSettings
// opens, and always for an app whose manifest does not ask for
// REQUEST_INSTALL_PACKAGES. Elsewhere it answers false.
func CanInstallPackages(cx *core.Scope, done func(allowed bool)) { cx.CanInstallPackages(done) }

// OpenInstallSettings opens Android's setting that lets the app install
// packages, for the user to allow it, and calls done with whether it
// opened. Elsewhere it answers false.
func OpenInstallSettings(cx *core.Scope, done func(opened bool)) { cx.OpenInstallSettings(done) }
