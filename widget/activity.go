package widget

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Live Activities show what the app follows as it happens, a delivery or
// a timer, on the Lock Screen and in the Dynamic Island of iPhones from
// iOS 16.1 (ActivityKit), and as Live Updates of Android 16, promoted
// notifications at the top of the shade and in a chip of the status bar,
// which earlier Androids show as ongoing notifications.
//
// The app turns them on in nsgo.yaml (live_activities) and starts one
// with StartActivity, which it updates and ends as what it follows
// changes. iOS draws an ActivityContent's views with the widget
// extension, and its fields where it has no views; Android shows the
// fields alone, as Live Updates take no views of the app.
//
//	a, err := widget.StartActivity("delivery", widget.ActivityContent{
//		Title: "Your order", Body: "On its way", Symbol: "box.truck.fill",
//		Timer: arrival, ShowProgress: true, Progress: 0.4,
//		Actions: []widget.ActivityAction{{ID: "call", Title: "Call"}},
//	})
//
// The system ends an activity after eight hours, and an app in the
// background updates its own only while it runs: one that follows what
// happens elsewhere for long updates it with push notifications
// (ActivityOptions.Push, OnPushToken, ActivityState).

// ActivityContent is what a Live Activity shows.
type ActivityContent struct {
	// Title and Body say what the activity follows; it needs a title.
	Title, Body string
	// Status is a few characters of where it stands, as "8 min": in
	// Android's status bar chip and the compact Dynamic Island.
	Status string
	// Timer counts the time from or to it, in Status's place, as the
	// system ticks it without updates.
	Timer time.Time
	// Symbol names one of Apple's SF Symbols that iOS shows before the
	// title and in the minimal Dynamic Island; Android shows the app's
	// icon.
	Symbol string
	// Progress, from 0 to 1, shows as a bar when ShowProgress is set.
	Progress     float64
	ShowProgress bool
	// Actions are buttons under the content, whose presses OnAction hears:
	// without opening the app on iOS 17 and later, and opening it on
	// Android. Three at most.
	Actions []ActivityAction
	// URL opens with the app when the activity is tapped; without it, a tap
	// only opens the app.
	URL string
	// Tint colors the symbol, the bar and the edge of the Dynamic Island,
	// and Android's icon; Background fills the activity on the Lock Screen.
	Tint, Background Color
	// Stale is when iOS shows the content as out of date, if no update
	// came by then.
	Stale time.Time

	// LockScreen is what iOS shows on the Lock Screen and in banners, and
	// the others are the parts of the Dynamic Island: expanded, as the user
	// presses it, Leading, Trailing and Center along its top and Bottom
	// under them; compact, as one activity shows, CompactLeading and
	// CompactTrailing either side of the camera; and Minimal, as several
	// share it. Each nil one shows what the fields above say.
	LockScreen                      View
	Leading, Trailing, Center       View
	Bottom                          View
	CompactLeading, CompactTrailing View
	Minimal                         View
}

// ActivityAction is a button of an activity: Title says what it does,
// with Symbol, an SF Symbol, before it on iOS, and ID tells OnAction.
type ActivityAction struct {
	ID, Title, Symbol string
}

// ActivityOptions are how StartActivity starts an activity.
type ActivityOptions struct {
	// Push asks APNs for a token to update the activity with, which
	// OnPushToken hears; iOS only, and the app needs the push
	// notifications entitlement.
	Push bool
}

// Activity is a Live Activity of the app.
type Activity struct {
	id, kind string
}

// ID names the activity among the app's, also after the app starts anew;
// a push that starts one names it in its attributes.
func (a *Activity) ID() string { return a.id }

// Kind is what StartActivity started the activity as.
func (a *Activity) Kind() string { return a.kind }

// Dismissal is when an ended activity leaves the Lock Screen.
type Dismissal struct {
	at  time.Time
	now bool
}

var (
	// DismissDefault leaves the ended activity on the Lock Screen for up
	// to four hours on iOS, and takes it away at once on Android.
	DismissDefault = Dismissal{}
	// DismissNow takes it away at once.
	DismissNow = Dismissal{now: true}
)

// DismissAt takes the ended activity away at t, within four hours.
func DismissAt(t time.Time) Dismissal { return Dismissal{at: t} }

// ActivityPress is a press of an ActivityAction or a Button.
type ActivityPress struct {
	// Activity is the pressed one, and Action its ActivityAction's ID or
	// its Button's Action.
	Activity *Activity
	Action   string
}

// PushToken is a token APNs takes to update an activity, or with
// Activity nil to start one (iOS 17.2 and later), in hexadecimal.
type PushToken struct {
	Activity *Activity
	Token    string
}

// ActivityAttributesType is the attributes-type of an APNs push that
// starts a Live Activity, whose attributes are {"kind": ..., "id": ...}.
const ActivityAttributesType = "NSGoActivityAttributes"

var (
	// ErrNoActivities is what StartActivity reports in an app built
	// without live_activities in nsgo.yaml.
	ErrNoActivities = errors.New("widget: the app has no Live Activities: set live_activities.enabled in nsgo.yaml and build it with nsgo build or nsgo dev")
	// ErrActivitiesUnsupported is what it reports elsewhere than iOS 16.1
	// and later and Android.
	ErrActivitiesUnsupported = fmt.Errorf("widget: Live Activities need iOS 16.1 or Android: %w", errors.ErrUnsupported)
	// ErrActivitiesDisabled is what StartActivity reports when the user
	// turned the app's Live Activities off, or on Android its
	// notifications.
	ErrActivitiesDisabled = errors.New("widget: the user turned the app's Live Activities off")
	// ErrActivityEnded is what Update and End report for an activity that
	// ended, or that the user took away.
	ErrActivityEnded = errors.New("widget: the activity has ended")
	// ErrActivityTooLarge is what StartActivity, Update and ActivityState
	// report for content past the 4 KB ActivityKit and APNs take.
	ErrActivityTooLarge = errors.New("widget: the activity's content is larger than 4 KB")
)

// maxActivityState is how large ActivityKit takes the content of an
// activity, as APNs takes it.
const maxActivityState = 4096

// StartActivity starts a Live Activity of a kind, which names what it
// follows and is as widgets' kinds are, showing c; it needs a title.
func StartActivity(kind string, c ActivityContent, opts ...ActivityOptions) (*Activity, error) {
	if !kindPattern.MatchString(kind) {
		return nil, fmt.Errorf("widget: %q is not a kind of activity: letters, digits, '.', '_' and '-', up to 64", kind)
	}
	var o ActivityOptions
	if len(opts) > 0 {
		o = opts[0]
	}
	var b [8]byte
	rand.Read(b[:])
	a := &Activity{id: hex.EncodeToString(b[:]), kind: kind}
	return a, startActivity(a, c, o)
}

// Update shows c in place of what the activity shows.
func (a *Activity) Update(c ActivityContent) error { return updateActivity(a, c, nil) }

// Alert shows c and alerts the user with title and body: a banner on iOS,
// where the system lights the screen and the Dynamic Island expands, and
// the notification's sound and vibration on Android.
func (a *Activity) Alert(c ActivityContent, title, body string) error {
	return updateActivity(a, c, &activityAlert{Title: title, Body: body})
}

// End ends the activity, showing final when not nil, and leaves it as d
// says.
func (a *Activity) End(final *ActivityContent, d Dismissal) error { return endActivity(a, final, d) }

// Activities returns the app's activities that run, those of earlier runs
// of the app included.
func Activities() []*Activity { return runningActivities() }

// ActivitiesEnabled reports whether the app may start Live Activities:
// built with them, on iOS 16.1 or Android, and allowed by the user, who
// turns them off in the app's settings.
func ActivitiesEnabled() bool { return activitiesEnabled() }

var handlers struct {
	sync.Mutex
	press func(ActivityPress)
	token func(PushToken)
	// presses and tokens came before their handler.
	presses []ActivityPress
	tokens  []PushToken
}

// OnAction runs fn with each press of an activity's action or button. Set
// it in main, before nsgo.Run: on iOS the system starts the app in the
// background for a press, without its views, and the press that started
// it waits for fn. fn runs on a goroutine of the system's, not the app's
// views': reach them through a core.Scope's Post. A press holds the
// activity on iOS until fn returns, so update it there.
func OnAction(fn func(ActivityPress)) {
	handlers.Lock()
	handlers.press = fn
	pending := handlers.presses
	handlers.presses = nil
	handlers.Unlock()
	listen()
	for _, p := range pending {
		fn(p)
	}
}

// OnPushToken runs fn with each push token of an activity started with
// ActivityOptions.Push, as APNs gives and renews it, and from iOS 17.2
// with the app's token to start activities by push, with Activity nil.
// It runs as OnAction's handler does.
func OnPushToken(fn func(PushToken)) {
	handlers.Lock()
	handlers.token = fn
	pending := handlers.tokens
	handlers.tokens = nil
	handlers.Unlock()
	listen()
	for _, t := range pending {
		fn(t)
	}
}

func pressed(p ActivityPress) {
	handlers.Lock()
	fn := handlers.press
	if fn == nil {
		handlers.presses = append(handlers.presses, p)
	}
	handlers.Unlock()
	if fn != nil {
		fn(p)
	}
}

func tokenChanged(t PushToken) {
	handlers.Lock()
	fn := handlers.token
	if fn == nil {
		handlers.tokens = append(handlers.tokens, t)
	}
	handlers.Unlock()
	if fn != nil {
		fn(t)
	}
}

// ActivityState returns c as the content-state of an APNs push that
// updates, ends or starts an activity, for a server that sends them. Its
// views may hold no images, which a push cannot carry.
func ActivityState(c ActivityContent) (json.RawMessage, error) {
	st, images, err := encodeActivity(c)
	if err != nil {
		return nil, err
	}
	if len(images) > 0 {
		return nil, errors.New("widget: a pushed activity's views hold no images")
	}
	return st, nil
}

// activityAlert is what Alert alerts with.
type activityAlert struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

// activityStateVersion changes with what the extension cannot read of
// earlier states.
const activityStateVersion = 1

// activityState is an activity's content as the extension reads it, its
// content-state: the fields, which the extension lays out where the app
// gave no view, and the views of the places the app gave.
type activityState struct {
	V        int           `json:"v"`
	Title    string        `json:"title"`
	Body     string        `json:"body,omitempty"`
	Status   string        `json:"status,omitempty"`
	Timer    float64       `json:"timer,omitempty"` // in seconds since 1970
	Symbol   string        `json:"symbol,omitempty"`
	Progress *float64      `json:"progress,omitempty"`
	Actions  []stateAction `json:"actions,omitempty"`
	URL      string        `json:"url,omitempty"`
	Tint     *color        `json:"tint,omitempty"`
	BG       *color        `json:"bg,omitempty"`
	// Stale is the content's staleDate, in seconds since 1970.
	Stale float64 `json:"stale,omitempty"`

	Lock     *node `json:"lock,omitempty"`
	Leading  *node `json:"leading,omitempty"`
	Trailing *node `json:"trailing,omitempty"`
	Center   *node `json:"center,omitempty"`
	Bottom   *node `json:"bottom,omitempty"`
	CL       *node `json:"cl,omitempty"`
	CT       *node `json:"ct,omitempty"`
	Min      *node `json:"min,omitempty"`
}

type stateAction struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Symbol string `json:"symbol,omitempty"`
}

// checkActivity reports what keeps c from showing.
func checkActivity(c ActivityContent) error {
	if c.Title == "" {
		return errors.New("widget: an activity needs a title")
	}
	if len(c.Actions) > 3 {
		return fmt.Errorf("widget: an activity has %d actions, more than 3", len(c.Actions))
	}
	for _, act := range c.Actions {
		if act.ID == "" || act.Title == "" {
			return errors.New("widget: an activity's action needs an ID and a title")
		}
	}
	return nil
}

// encodeActivity returns c's content-state and the images its views hold,
// by file name.
func encodeActivity(c ActivityContent) (json.RawMessage, map[string][]byte, error) {
	if err := checkActivity(c); err != nil {
		return nil, nil, err
	}
	e := &encoder{images: map[string][]byte{}}
	st := activityState{V: activityStateVersion, Title: c.Title, Body: c.Body, Status: c.Status, Symbol: c.Symbol,
		URL: c.URL, Tint: encodeColor(c.Tint), BG: encodeColor(c.Background)}
	if !c.Timer.IsZero() {
		st.Timer = float64(c.Timer.UnixNano()) / 1e9
	}
	if c.ShowProgress {
		st.Progress = ptr(min(max(c.Progress, 0), 1))
	}
	for _, act := range c.Actions {
		st.Actions = append(st.Actions, stateAction(act))
	}
	if !c.Stale.IsZero() {
		st.Stale = float64(c.Stale.UnixNano()) / 1e9
	}
	for _, p := range []struct {
		out **node
		v   View
	}{
		{&st.Lock, c.LockScreen}, {&st.Leading, c.Leading}, {&st.Trailing, c.Trailing}, {&st.Center, c.Center},
		{&st.Bottom, c.Bottom}, {&st.CL, c.CompactLeading}, {&st.CT, c.CompactTrailing}, {&st.Min, c.Minimal},
	} {
		n, err := e.node(p.v, nil)
		if err != nil {
			return nil, nil, fmt.Errorf("widget: %w", err)
		}
		*p.out = n
	}
	data, err := json.Marshal(st)
	if err != nil {
		return nil, nil, err
	}
	if len(data) > maxActivityState {
		return nil, nil, fmt.Errorf("%w: %d bytes", ErrActivityTooLarge, len(data))
	}
	return data, e.images, nil
}
