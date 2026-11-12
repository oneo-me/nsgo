package core

import "testing"

func TestApp(t *testing.T) {
	if a := App(); !a.Development || a.Version != "" {
		t.Errorf("an app the tool did not build reads %+v", a)
	}
	defer func(id, name, version, build, release string) {
		appID, appName, appVersion, appBuild, appRelease = id, name, version, build, release
	}(appID, appName, appVersion, appBuild, appRelease)
	appID, appName, appVersion, appBuild, appRelease = "dev.example.app", "App", "1.2.0", "12", "1"
	if a, want := App(), (AppInfo{ID: "dev.example.app", Name: "App", Version: "1.2.0", Build: "12"}); a != want {
		t.Errorf("App() = %+v, want %+v", a, want)
	}
}
