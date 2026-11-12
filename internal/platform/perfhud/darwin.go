//go:build darwin

package perfhud

import "github.com/nsgo-dev/nsgo/internal/platform/apple"

// memory is the physical footprint, as Activity Monitor shows it.
func memory() (uint64, bool) { return apple.ProcessMemory() }
