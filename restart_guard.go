package nsgo

import (
	"os"
	"sync/atomic"
	"time"
)

const restartGuardEnv = "NSGO_RESTART_GUARD"

// restartLoopWindow is how soon after a restart another restart counts as a loop.
const restartLoopWindow = 10 * time.Second

// restartedAt is when markRestarted found this process started by Restart, in Unix nanoseconds; 0 when it was not.
var restartedAt atomic.Int64

// markRestarted records that this process was started by Restart, so a restart as it launches is refused in one log line instead of spawning forever. A restart the user asks for later, as after changing a setting, still runs.
func markRestarted() {
	if os.Getenv(restartGuardEnv) != "" {
		restartedAt.Store(time.Now().UnixNano())
		os.Unsetenv(restartGuardEnv)
	}
}

// restartLooping says whether this process, started by Restart, is restarting again within restartLoopWindow of its start.
func restartLooping() bool {
	at := restartedAt.Load()
	return at != 0 && time.Since(time.Unix(0, at)) < restartLoopWindow
}

// restartPermitted says whether a completed app may launch another process. Development runs deliberately end after a restart, and a release app restarting itself as it launches gets one visible refusal instead of chaining forever.
func restartPermitted() bool { return !devBuild && !restartLooping() }
