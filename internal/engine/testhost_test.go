package engine

import (
	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/testhost"
)

func testhostOptions(observe func(*driver.MountBatch)) testhost.Options {
	return testhost.Options{Observe: observe}
}
