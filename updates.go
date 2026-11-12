package nsgo

import (
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/internal/platform/updates"
	"github.com/oneo-me/nsgo/updater"
)

// updateConfig is how the app updates, as the nsgo tool set it from
// nsgo.yaml's updates; macOS's Sparkle reads its own from Info.plist.
func updateConfig() updates.Config {
	info := core.App()
	return updates.Config{App: name, ID: id, Version: info.Version, Build: info.Build,
		Feed: updateFeed, Key: updateKey, Automatic: updateAutomatic == "1"}
}

// withUpdateChecks returns what Run runs, checking for updates in the
// background at most once a day when nsgo.yaml's updates.automatic is on
// and c has anything to check (Config.Checks), for the platforms whose
// updater has no schedule of its own: Linux, Android and iOS. It checks as
// the app starts and whenever the user comes back to it, as a phone's app
// stays in memory for days, and never while the app is out of sight.
func withUpdateChecks(root App, app func(*core.Scope), c updates.Config, appStore bool) (App, func(*core.Scope)) {
	if !c.Automatic || !c.Checks(appStore) {
		return root, app
	}
	inner := appOf(root, app)
	return nil, func(cx *core.Scope) {
		active := false
		cx.Effect(func() {
			now := cx.Env().Lifecycle() == core.LifecycleActive
			if now && !active {
				// Under the app's scope, which outlives this run of the effect.
				cx.Run(func() { updater.CheckDaily(cx) })
			}
			active = now
		})
		inner(cx)
	}
}
