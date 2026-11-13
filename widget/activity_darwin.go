//go:build darwin

package widget

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// On iOS, ActivityKit starts, updates and ends activities through the
// functions of the bundle's NSGoWidgetKit (widgets/Activities.swift),
// whose attributes, NSGoActivityAttributes, the widget extension shares:
// a kind, an ID and a content-state of JSON, which the extension draws as
// it draws timelines, its images in the activity's directory of the app
// group's container (activityDir). The library hears presses of buttons
// as LiveActivityIntents, which iOS runs in the app's process, and push
// tokens, and calls back with them (listen).

var kitFns struct {
	once sync.Once
	err  error

	start   func(kind, id, state string, push int32) uintptr
	update  func(id, state, alert string) uintptr
	end     func(id, state string, final int32, dismissal int32, at float64) uintptr
	list    func() uintptr
	enabled func() int32
	listen  func(cb uintptr)
	free    func(p uintptr)

	listening sync.Once
}

// The events nsgo_activity_listen's callback reports.
const (
	eventPress = 1 + iota
	eventToken
	eventStartToken
)

func activityKit() error {
	kitFns.once.Do(func() {
		if runtime.GOOS != "ios" {
			kitFns.err = ErrActivitiesUnsupported
			return
		}
		load()
		if !native.activities {
			kitFns.err = ErrNoActivities
			return
		}
		lib, err := kit()
		if err != nil {
			kitFns.err = err
			return
		}
		defer func() {
			if r := recover(); r != nil {
				kitFns.err = errors.New("widget: the app's NSGoWidgetKit has no Live Activities: build it again")
			}
		}()
		purego.RegisterLibFunc(&kitFns.start, lib, "nsgo_activity_start")
		purego.RegisterLibFunc(&kitFns.update, lib, "nsgo_activity_update")
		purego.RegisterLibFunc(&kitFns.end, lib, "nsgo_activity_end")
		purego.RegisterLibFunc(&kitFns.list, lib, "nsgo_activity_list")
		purego.RegisterLibFunc(&kitFns.enabled, lib, "nsgo_activity_enabled")
		purego.RegisterLibFunc(&kitFns.listen, lib, "nsgo_activity_listen")
		purego.RegisterLibFunc(&kitFns.free, lib, "nsgo_free")
		if kitFns.enabled() < 0 {
			kitFns.err = ErrActivitiesUnsupported
		}
	})
	return kitFns.err
}

// answer copies and frees a string the library returned, "" for none.
func answer(p uintptr) string {
	if p == 0 {
		return ""
	}
	s := cString(p)
	kitFns.free(p)
	return s
}

func cString(p uintptr) string {
	if p == 0 {
		return ""
	}
	ptr := *(**byte)(unsafe.Pointer(&p))
	n := 0
	for *(*byte)(unsafe.Add(unsafe.Pointer(ptr), n)) != 0 {
		n++
	}
	return string(unsafe.Slice(ptr, n))
}

// failure turns the library's error, as it names it, into Go's.
func failure(msg string) error {
	switch msg {
	case "":
		return nil
	case "ended":
		return ErrActivityEnded
	}
	return errors.New("widget: " + msg)
}

// activityDir is where an activity's images are, in the app group's
// container beside the timelines; kinds of widgets cannot start with '.'.
func activityDir(id string) (string, error) {
	dir, err := containerDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ".activities", id), nil
}

// writeActivity encodes c and writes its images for the activity id.
func writeActivity(id string, c ActivityContent) (string, error) {
	st, images, err := encodeActivity(c)
	if err != nil {
		return "", err
	}
	dir, err := activityDir(id)
	if err != nil {
		return "", err
	}
	if err := writeImages(dir, images); err != nil {
		return "", err
	}
	// The state shown before stays until the new one shows; its images go
	// with the next update.
	return string(st), nil
}

func startActivity(a *Activity, c ActivityContent, o ActivityOptions) error {
	if err := activityKit(); err != nil {
		return err
	}
	if kitFns.enabled() == 0 {
		return ErrActivitiesDisabled
	}
	pruneActivities()
	st, err := writeActivity(a.id, c)
	if err != nil {
		return err
	}
	var push int32
	if o.Push {
		push = 1
		listen()
	}
	return failure(answer(kitFns.start(a.kind, a.id, st, push)))
}

func updateActivity(a *Activity, c ActivityContent, alert *activityAlert) error {
	if err := activityKit(); err != nil {
		return err
	}
	st, err := writeActivity(a.id, c)
	if err != nil {
		return err
	}
	var al string
	if alert != nil {
		b, _ := json.Marshal(alert)
		al = string(b)
	}
	return failure(answer(kitFns.update(a.id, st, al)))
}

func endActivity(a *Activity, final *ActivityContent, d Dismissal) error {
	if err := activityKit(); err != nil {
		return err
	}
	var st string
	var has int32
	if final != nil {
		var err error
		if st, err = writeActivity(a.id, *final); err != nil {
			return err
		}
		has = 1
	}
	var policy int32
	var at float64
	switch {
	case d.now:
		policy = 1
	case !d.at.IsZero():
		policy, at = 2, float64(d.at.UnixNano())/1e9
	}
	return failure(answer(kitFns.end(a.id, st, has, policy, at)))
}

// listed are the activities nsgo_activity_list reports.
type listed struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
}

func list() []listed {
	if activityKit() != nil {
		return nil
	}
	var out []listed
	json.Unmarshal([]byte(answer(kitFns.list())), &out)
	return out
}

func runningActivities() []*Activity {
	var out []*Activity
	for _, l := range list() {
		out = append(out, &Activity{id: l.ID, kind: l.Kind})
	}
	return out
}

// pruneActivities removes the directories of activities that ended.
func pruneActivities() {
	dir, err := activityDir("")
	if err != nil {
		return
	}
	files, _ := os.ReadDir(dir)
	running := list()
	for _, f := range files {
		if !slices.ContainsFunc(running, func(l listed) bool { return l.ID == f.Name() }) {
			os.RemoveAll(filepath.Join(dir, f.Name()))
		}
	}
}

func activitiesEnabled() bool { return activityKit() == nil && kitFns.enabled() > 0 }

// activityByID returns the running activity id, or one of an unknown kind.
func activityByID(id string) *Activity {
	for _, l := range list() {
		if l.ID == id {
			return &Activity{id: l.ID, kind: l.Kind}
		}
	}
	return &Activity{id: id}
}

// listen has the library report presses and push tokens, which it keeps
// until then: the press that started the app in the background waits for
// it.
func listen() {
	if activityKit() != nil {
		return
	}
	kitFns.listening.Do(func() {
		cb := purego.NewCallback(func(event int32, id, value uintptr) {
			a, v := cString(id), cString(value)
			switch event {
			case eventPress:
				pressed(ActivityPress{Activity: activityByID(a), Action: v})
			case eventToken:
				tokenChanged(PushToken{Activity: activityByID(a), Token: v})
			case eventStartToken:
				tokenChanged(PushToken{Token: v})
			}
		})
		kitFns.listen(cb)
	})
}
