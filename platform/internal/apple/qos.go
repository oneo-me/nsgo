//go:build darwin

package apple

import "github.com/ebitengine/purego"

// QoS classes, from <pthread/qos.h>. A thread's class tells the scheduler
// which cores to put it on: user-interactive runs on the performance
// cores, utility and background on the efficiency ones.
const (
	QoSUtility    uint32 = 0x11
	QoSBackground uint32 = 0x09
)

var setQoS func(class uint32, relative int32) int32

func init() {
	lib, err := purego.Dlopen("/usr/lib/libSystem.B.dylib", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return
	}
	purego.RegisterLibFunc(&setQoS, lib, "pthread_set_qos_class_self_np")
}

// SetThreadQoS sets the calling thread's QoS class. A drawing engine that
// an app wakes at its own pace is utility work: measured on the quotes
// case, the same code at utility takes 26% less energy than at the default
// user-interactive class, because it runs on the efficiency cores.
//
// It sets the current thread only, so the caller must have locked itself
// to an OS thread for the effect to last.
func SetThreadQoS(class uint32) {
	if setQoS != nil {
		setQoS(class, 0)
	}
}
