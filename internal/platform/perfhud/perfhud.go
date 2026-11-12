// Package perfhud holds what the backends' performance panels share: the
// process' CPU and memory use (engine.ProcessSample) and, on Apple
// platforms, drawing the trace.
package perfhud

import (
	"time"

	"github.com/nsgo-dev/nsgo/internal/engine"
)

// Sampler reads the process' resources. CPU use is a rate: each sample
// reports the CPU time since the previous one over the wall time between
// them, and the first reports none.
type Sampler struct {
	at  time.Time
	cpu time.Duration
	has bool
}

// Sample returns the process' use now.
func (p *Sampler) Sample() engine.ProcessSample {
	now := time.Now()
	var out engine.ProcessSample
	if used, ok := cpuTime(); ok {
		if p.has {
			if wall := now.Sub(p.at); wall > 0 {
				percent := float32(float64(used-p.cpu) / float64(wall) * 100)
				out.CPU = &percent
			}
		}
		p.at, p.cpu, p.has = now, used, true
	}
	if m, ok := memory(); ok {
		out.Memory = &m
	}
	return out
}
