package nsgotest

import (
	"time"

	"github.com/oneo-me/nsgo/internal/driver"
)

// traceBatch prints batch b, which the step running produced in a commit
// that took took, as `nsgo test -trace` shows it: a line naming the step,
// the items and the commit's time, then a line per item.
func (a *App) traceBatch(b *driver.MountBatch, took time.Duration) {
	if a.trace == nil {
		return
	}
	driver.WriteTrace(a.trace, a.stepName, b, took)
}
