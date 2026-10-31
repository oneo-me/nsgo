// Package mobile holds what only iOS and Android have: the system's tab
// bar and a bar above the software keyboard, as NSH's nsh/mobile. It
// builds on every platform, so one app runs everywhere: elsewhere its
// components take no room, and IsAvailable says whether they show.
package mobile

import "github.com/oneo-me/nsgo/core"

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

// IsAvailable reports whether the app runs on iOS or Android, where this
// package's components show.
func IsAvailable(cx *core.Scope) bool {
	os := cx.Env().OS()
	return os == core.OSIOS || os == core.OSAndroid
}

// ExitApp leaves the app on Android as its back button does when nothing
// in the app takes it: the task's root activity goes to the background,
// any other finishes. iOS apps never leave themselves, so it does nothing
// there, as elsewhere.
func ExitApp(cx *core.Scope) { cx.ExitApp() }
