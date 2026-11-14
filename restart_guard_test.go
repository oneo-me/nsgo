package nsgo

import (
	"os"
	"testing"
	"time"
)

func TestRestartGuardMarksRestartedProcess(t *testing.T) {
	was := restartedAt.Load()
	t.Cleanup(func() { restartedAt.Store(was) })
	restartedAt.Store(0)
	t.Setenv(restartGuardEnv, "1")
	markRestarted()
	if !restartLooping() {
		t.Fatal("restart guard did not mark the restarted process")
	}
	if _, ok := os.LookupEnv(restartGuardEnv); ok {
		t.Fatal("restart guard environment leaked into the app")
	}
}

func TestRestartGuardRefusesOnlyARestartAtLaunch(t *testing.T) {
	was := restartedAt.Load()
	t.Cleanup(func() { restartedAt.Store(was) })
	restartedAt.Store(time.Now().UnixNano())
	if restartPermitted() {
		t.Fatal("an app restarting as it launched was allowed to launch another process")
	}
	if devBuild {
		t.Skip("development builds intentionally never relaunch")
	}
	restartedAt.Store(time.Now().Add(-restartLoopWindow - time.Second).UnixNano())
	if !restartPermitted() {
		t.Fatal("a restarted app was refused a restart the user asked for later")
	}
	restartedAt.Store(0)
	if !restartPermitted() {
		t.Fatal("an ordinary release app was refused a restart")
	}
}
