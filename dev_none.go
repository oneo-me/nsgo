//go:build !nsgodev

package nsgo

// devApp is app: only nsgo dev's builds connect to its server.
func devApp(app App) App { return app }

// startDevClient does nothing: only nsgo dev's builds trace and serve DevTools.
func startDevClient() {}

// devBuild is whether nsgo dev built the app.
const devBuild = false
