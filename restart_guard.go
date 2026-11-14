package nsgo

import (
	"os"
	"sync/atomic"
)

const restartGuardEnv = "NSGO_RESTART_GUARD"

var restartGuarded atomic.Bool

// markRestarted records that this process was started by Restart. A second
// chained restart is refused, which makes an app that calls Restart during
// startup visible in one log line instead of spawning forever.
func markRestarted() {
	if os.Getenv(restartGuardEnv) != "" {
		restartGuarded.Store(true)
		os.Unsetenv(restartGuardEnv)
	}
}

// restartPermitted says whether a completed app may launch another process.
// Development runs deliberately end after a restart, while a release app
// launched by Restart gets one visible refusal instead of chaining forever.
func restartPermitted() bool { return !devBuild && !restartGuarded.Load() }
