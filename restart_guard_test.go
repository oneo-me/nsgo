package nsgo

import (
	"os"
	"testing"
)

func TestRestartGuardMarksRestartedProcess(t *testing.T) {
	was := restartGuarded.Load()
	t.Cleanup(func() { restartGuarded.Store(was) })
	restartGuarded.Store(false)
	t.Setenv(restartGuardEnv, "1")
	markRestarted()
	if !restartGuarded.Load() {
		t.Fatal("restart guard did not mark the restarted process")
	}
	if _, ok := os.LookupEnv(restartGuardEnv); ok {
		t.Fatal("restart guard environment leaked into the app")
	}
}

func TestRestartGuardRefusesAChainedRestart(t *testing.T) {
	wasGuarded := restartGuarded.Load()
	t.Cleanup(func() { restartGuarded.Store(wasGuarded) })
	restartGuarded.Store(true)
	if restartPermitted() {
		t.Fatal("a restarted app was allowed to launch another process")
	}
	if devBuild {
		t.Skip("development builds intentionally never relaunch")
	}
	restartGuarded.Store(false)
	if !restartPermitted() {
		t.Fatal("an ordinary release app was refused a restart")
	}
}
