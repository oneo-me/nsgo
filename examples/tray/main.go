// Command tray is a menu bar app run as an Avalonia application runs:
// nsgo.Run starts the app's function, which opens what it shows. It
// keeps out of the Dock, draws its tray icon with a view that counts,
// shows a panel under the icon on a click and a menu on a right click,
// opens a launcher panel with a global shortcut, a settings window from
// both, and a welcome window on its first launch. Every window shows the
// same signals.
package main

import (
	"strconv"

	"github.com/oneo-me/nsgo"
	"github.com/oneo-me/nsgo/base"
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/desktop"
	"github.com/oneo-me/nsgo/icons/lucide"
)

func main() { nsgo.Run(App, nsgo.HideDock()) }

// launcherKey shows and hides the launcher from any app.
const launcherKey = "ctrl+alt+space"

// App opens the app's tray icon, panels and first windows.
func App(app *core.Scope) {
	desktop.SetShutdownMode(app, desktop.ShutdownOnExplicit)
	count := core.NewSignal(app, 0)
	settings := func() {
		desktop.OpenWindow(app, desktop.WindowOptions{Title: "Settings", Width: 360, Height: 240, Key: "settings",
			Constraints: desktop.WindowConstraints{NoResize: true, NoMinimize: true}},
			func(cx *core.Scope) core.Node { return Settings(cx, count) })
	}

	panel := desktop.OpenPanel(app, desktop.PanelOptions{Width: 280, Height: 180, HideOnBlur: true},
		func(cx *core.Scope) core.Node { return Panel(cx, count, settings) })
	desktop.NewTray(app, desktop.TrayOptions{
		Tooltip: "NSGo tray",
		Width:   44,
		View:    func(cx *core.Scope) core.Node { return TrayIcon(cx, count) },
		Panel:   panel,
		Menu: func() []desktop.MenuItem {
			return []desktop.MenuItem{
				{Label: "Count", OnSelect: func() { count.Set(count.Get() + 1) }},
				{Label: "Settings…", OnSelect: settings},
				{Separator: true},
				{Label: "Quit Tray", OnSelect: app.Quit},
			}
		},
	})

	launcher := desktop.OpenPanel(app, desktop.PanelOptions{Width: 480, Height: 64, HideOnBlur: true},
		func(cx *core.Scope) core.Node { return Launcher(cx, count) })
	desktop.RegisterHotKey(app, launcherKey, launcher.Toggle)

	if _, seen := app.Storage().GetItem("tray.welcomed"); !seen {
		desktop.OpenWindow(app, desktop.WindowOptions{Title: "Welcome", Width: 360, Height: 200},
			func(cx *core.Scope) core.Node { return Welcome(cx) })
	}
}

// TrayIcon draws the bell and the count in the menu bar.
func TrayIcon(cx *core.Scope, count *core.Signal[int]) core.Node {
	return core.View{Class: "flex-row items-center justify-center gap-1 h-full", Children: []core.Node{
		core.Icon{SVG: lucide.Bell(), StyleFunc: func() core.Style {
			c := core.RGB(0x000000)
			if cx.Dark() {
				c = core.RGB(0xffffff)
			}
			return core.Style{Width: core.Pt(16), Height: core.Pt(16), Color: c}
		}},
		core.Text{Class: "text-xs font-semibold text-black dark:text-white",
			ContentFunc: func() string { return strconv.Itoa(count.Get()) }},
	}}
}

// Panel shows under the tray icon.
func Panel(cx *core.Scope, count *core.Signal[int], settings func()) core.Node {
	return core.View{Class: "flex-1 p-4 gap-3 bg-white dark:bg-neutral-900", Children: []core.Node{
		core.Text{Class: "text-sm font-semibold text-neutral-900 dark:text-white", Content: "NSGo tray"},
		core.Text{Class: "text-3xl font-bold text-neutral-900 dark:text-white",
			ContentFunc: func() string { return strconv.Itoa(count.Get()) }},
		core.View{Class: "flex-row gap-2", Children: []core.Node{
			button("Count", func() { count.Set(count.Get() + 1) }),
			button("Settings…", settings),
		}},
	}}
}

// Launcher is a field a global shortcut shows: Return counts and hides
// it.
func Launcher(cx *core.Scope, count *core.Signal[int]) core.Node {
	win := desktop.CurrentWindow(cx)
	return core.View{Class: "flex-1 flex-row items-center px-4 bg-white dark:bg-neutral-900", Children: []core.Node{
		base.Input{Class: "flex-1 text-lg text-neutral-900 dark:text-white", AutoFocus: true,
			Placeholder: "Press Return to count, " + launcherKey + " to hide",
			OnSubmit:    func(string) { count.Set(count.Get() + 1); win.Hide() }},
	}}
}

// Settings shows the count and whether the app starts at login.
func Settings(cx *core.Scope, count *core.Signal[int]) core.Node {
	login := core.NewSignal(cx, false)
	desktop.LaunchAtLogin(cx, login.Set)
	return core.View{Class: "flex-1 p-6 gap-4 bg-white dark:bg-neutral-900", Children: []core.Node{
		core.Text{Class: "text-base text-neutral-900 dark:text-white",
			ContentFunc: func() string { return "Counted " + strconv.Itoa(count.Get()) + " times" }},
		core.View{Class: "flex-row items-center gap-3", Children: []core.Node{
			base.Switch{Checked: login.Get, Label: "Launch at login",
				OnChange:   func(on bool) { login.Set(on); desktop.SetLaunchAtLogin(cx, on) },
				Class:      "w-10 h-6 rounded-full bg-neutral-300 checked:bg-blue-500",
				ThumbClass: "size-5 m-0.5 rounded-full bg-white"},
			core.Text{Class: "text-sm text-neutral-700 dark:text-neutral-300", Content: "Launch at login"},
		}},
		button("Reset", func() { count.Set(0) }),
	}}
}

// Welcome shows on the first launch.
func Welcome(cx *core.Scope) core.Node {
	win := desktop.CurrentWindow(cx)
	return core.View{Class: "flex-1 p-6 gap-4 bg-white dark:bg-neutral-900", Children: []core.Node{
		core.Text{Class: "text-lg font-semibold text-neutral-900 dark:text-white", Content: "Tray runs in the menu bar"},
		core.Text{Class: "text-sm text-neutral-600 dark:text-neutral-400",
			Content: "Click its icon for the panel, right-click it for the menu, or press " + launcherKey + "."},
		button("Got it", func() {
			cx.Storage().SetItem("tray.welcomed", "1")
			win.Close()
		}),
	}}
}

func button(label string, onPress func()) core.Node {
	return base.Button{Label: label, OnPress: func(core.PressEvent) { onPress() },
		Class: "px-3 py-1.5 rounded-md bg-blue-500 active:bg-blue-600", Children: []core.Node{
			core.Text{Class: "text-sm text-white", Content: label},
		}}
}
