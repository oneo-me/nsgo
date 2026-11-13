package driver

import "sync"

// Live Updates are how Android shows package widget's Live Activities, as
// notifications the Android runtime posts. The package reaches the
// runtime outside any surface, as an activity outlives the app's views:
// its messages, JSON, wait here until the backend connects, and the
// runtime's reports wait for the package's handler.
var liveUpdates struct {
	sync.Mutex
	send    func([]byte)
	queue   [][]byte
	hear    func([]byte)
	reports [][]byte
}

// SendLiveUpdate sends package widget's msg to the Android runtime, once
// the backend has connected.
func SendLiveUpdate(msg []byte) {
	liveUpdates.Lock()
	send := liveUpdates.send
	if send == nil {
		liveUpdates.queue = append(liveUpdates.queue, msg)
	}
	liveUpdates.Unlock()
	if send != nil {
		send(msg)
	}
}

// ConnectLiveUpdates is the Android backend's: send carries package
// widget's messages to the runtime, those that waited first.
func ConnectLiveUpdates(send func([]byte)) {
	liveUpdates.Lock()
	queue := liveUpdates.queue
	liveUpdates.send, liveUpdates.queue = send, nil
	liveUpdates.Unlock()
	for _, msg := range queue {
		send(msg)
	}
}

// HearLiveUpdates is package widget's: fn hears the runtime's reports,
// those that waited first.
func HearLiveUpdates(fn func([]byte)) {
	liveUpdates.Lock()
	reports := liveUpdates.reports
	liveUpdates.hear, liveUpdates.reports = fn, nil
	liveUpdates.Unlock()
	for _, r := range reports {
		fn(r)
	}
}

// ReportLiveUpdate is the Android backend's: it hands package widget a
// report of the runtime.
func ReportLiveUpdate(report []byte) {
	liveUpdates.Lock()
	hear := liveUpdates.hear
	if hear == nil {
		liveUpdates.reports = append(liveUpdates.reports, report)
	}
	liveUpdates.Unlock()
	if hear != nil {
		hear(report)
	}
}
