package core

// Set by the nsgo tool with -ldflags -X from nsgo.yaml; appRelease is "1"
// in builds of nsgo build.
var appID, appName, appVersion, appBuild, appRelease string

// AppInfo is the app's identity as nsgo.yaml gives it.
type AppInfo struct {
	// ID and Name are nsgo.yaml's id and name.
	ID, Name string
	// Version is nsgo.yaml's version, and Build the build's number:
	// build_number, else Version. Both are "" where nsgo.yaml has none.
	Version, Build string
	// Development is true unless nsgo build built the app: under nsgo dev,
	// nsgo test and a plain go build.
	Development bool
}

// App returns the app's identity, which the nsgo tool builds into it.
func App() AppInfo {
	return AppInfo{ID: appID, Name: appName, Version: appVersion, Build: appBuild, Development: appRelease != "1"}
}
