//go:build android

package widget

import (
	"encoding/json"
	"math"
	"slices"
	"sync"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// On Android, the runtime shows activities as notifications
// (LiveUpdates.java): Live Updates from Android 16, promoted to the top of
// the shade and a chip of the status bar, and ongoing notifications
// before. Package widget sends it each start, update and end as JSON
// through the backend (driver.SendLiveUpdate), and hears what it reports:
// whether the app may show them and which show, as it connects and as
// they change, and the presses of their actions, which open the app.

// liveUpdate is a message to the runtime.
type liveUpdate struct {
	Op       string       `json:"op"` // start, update or end
	ID       string       `json:"id"`
	Kind     string       `json:"kind"`
	Title    string       `json:"title,omitempty"`
	Body     string       `json:"body,omitempty"`
	Status   string       `json:"status,omitempty"`
	Timer    int64        `json:"timer,omitempty"` // in milliseconds since 1970
	Progress float64      `json:"progress"`        // -1 for none
	Actions  []liveAction `json:"actions,omitempty"`
	URL      string       `json:"url,omitempty"`
	Tint     int64        `json:"tint,omitempty"` // ARGB, 0 for the system's
	Alert    bool         `json:"alert,omitempty"`
	// Dismiss is when an ended one goes, in milliseconds since 1970: 0 at
	// once.
	Dismiss int64 `json:"dismiss,omitempty"`
}

type liveAction struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// liveReport is what the runtime reports.
type liveReport struct {
	Type    string   `json:"type"` // state or press
	Enabled bool     `json:"enabled"`
	Running []listed `json:"running"`
	ID      string   `json:"id"`
	Action  string   `json:"action"`
}

type listed struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
}

var live struct {
	once    sync.Once
	mu      sync.Mutex
	enabled bool
	// reported is whether the runtime has reported its state, which it
	// does as it connects.
	reported bool
	running  []listed
}

func hear() {
	live.once.Do(func() {
		driver.HearLiveUpdates(func(data []byte) {
			var r liveReport
			if json.Unmarshal(data, &r) != nil {
				return
			}
			switch r.Type {
			case "state":
				live.mu.Lock()
				live.enabled, live.running, live.reported = r.Enabled, r.Running, true
				live.mu.Unlock()
			case "press":
				pressed(ActivityPress{Activity: activityByID(r.ID), Action: r.Action})
			}
		})
	})
}

func activityByID(id string) *Activity {
	live.mu.Lock()
	defer live.mu.Unlock()
	if i := slices.IndexFunc(live.running, func(l listed) bool { return l.ID == id }); i >= 0 {
		return &Activity{id: id, kind: live.running[i].Kind}
	}
	return &Activity{id: id}
}

func running(id string) bool {
	live.mu.Lock()
	defer live.mu.Unlock()
	return slices.ContainsFunc(live.running, func(l listed) bool { return l.ID == id })
}

func liveMessage(op string, a *Activity, c ActivityContent) liveUpdate {
	m := liveUpdate{Op: op, ID: a.id, Kind: a.kind, Title: c.Title, Body: c.Body, Status: c.Status, Progress: -1, URL: c.URL, Tint: argb(c.Tint)}
	if !c.Timer.IsZero() {
		m.Timer = c.Timer.UnixMilli()
	}
	if c.ShowProgress {
		m.Progress = min(max(c.Progress, 0), 1)
	}
	for _, act := range c.Actions {
		m.Actions = append(m.Actions, liveAction{ID: act.ID, Title: act.Title})
	}
	return m
}

// argb is c's light color as Android's, 0 for none or a system color.
func argb(c Color) int64 {
	if c.light == 0 {
		return 0
	}
	r, g, b, a := c.light.Components()
	ch := func(f float64) int64 { return int64(math.Round(min(max(f, 0), 1) * 255)) }
	return ch(a)<<24 | ch(r)<<16 | ch(g)<<8 | ch(b)
}

func post(m liveUpdate) {
	data, _ := json.Marshal(m)
	driver.SendLiveUpdate(data)
}

func startActivity(a *Activity, c ActivityContent, _ ActivityOptions) error {
	hear()
	if err := checkActivity(c); err != nil {
		return err
	}
	live.mu.Lock()
	if live.reported && !live.enabled {
		live.mu.Unlock()
		return ErrActivitiesDisabled
	}
	live.running = append(live.running, listed{ID: a.id, Kind: a.kind})
	live.mu.Unlock()
	post(liveMessage("start", a, c))
	return nil
}

func updateActivity(a *Activity, c ActivityContent, alert *activityAlert) error {
	hear()
	if err := checkActivity(c); err != nil {
		return err
	}
	if !running(a.id) {
		return ErrActivityEnded
	}
	m := liveMessage("update", a, c)
	if alert != nil {
		// Android alerts with the notification itself.
		m.Alert = true
		if alert.Title != "" {
			m.Title = alert.Title
		}
		if alert.Body != "" {
			m.Body = alert.Body
		}
	}
	post(m)
	return nil
}

func endActivity(a *Activity, final *ActivityContent, d Dismissal) error {
	hear()
	if !running(a.id) {
		return ErrActivityEnded
	}
	live.mu.Lock()
	live.running = slices.DeleteFunc(live.running, func(l listed) bool { return l.ID == a.id })
	live.mu.Unlock()
	m := liveUpdate{Op: "end", ID: a.id, Kind: a.kind, Progress: -1}
	if final != nil {
		m = liveMessage("end", a, *final)
	}
	if !d.now && !d.at.IsZero() {
		m.Dismiss = d.at.UnixMilli()
	}
	post(m)
	return nil
}

func runningActivities() []*Activity {
	hear()
	live.mu.Lock()
	defer live.mu.Unlock()
	var out []*Activity
	for _, l := range live.running {
		out = append(out, &Activity{id: l.ID, kind: l.Kind})
	}
	return out
}

func activitiesEnabled() bool {
	hear()
	live.mu.Lock()
	defer live.mu.Unlock()
	return live.enabled
}

func listen() { hear() }
