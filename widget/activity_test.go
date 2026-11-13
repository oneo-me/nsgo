package widget

import (
	"encoding/json"
	"errors"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestActivityState(t *testing.T) {
	timer := time.Unix(1_800_000_000, 0)
	c := ActivityContent{
		Title: "Delivery", Body: "On its way", Status: "8 min", Timer: timer, Symbol: "box.truck.fill",
		ShowProgress: true, Progress: 1.5, Tint: RGB(0x30d158), URL: "https://example.com/order",
		Actions:  []ActivityAction{{ID: "call", Title: "Call", Symbol: "phone"}},
		Trailing: Button{Action: "more", Children: []View{Text{Content: "More"}}},
		Minimal:  Image{Data: []byte("\x89PNG")},
	}
	data, images, err := encodeActivity(c)
	if err != nil {
		t.Fatal(err)
	}
	var st activityState
	if err := json.Unmarshal(data, &st); err != nil {
		t.Fatal(err)
	}
	if st.V != activityStateVersion || st.Title != "Delivery" || st.Body != "On its way" || st.Status != "8 min" || st.Timer != 1_800_000_000 ||
		st.Symbol != "box.truck.fill" || st.URL != "https://example.com/order" || st.Tint == nil {
		t.Errorf("fields = %+v", st)
	}
	if st.Progress == nil || *st.Progress != 1 {
		t.Errorf("progress = %v, want 1", st.Progress)
	}
	if len(st.Actions) != 1 || st.Actions[0] != (stateAction{ID: "call", Title: "Call", Symbol: "phone"}) {
		t.Errorf("actions = %+v", st.Actions)
	}
	if n := st.Trailing; n == nil || n.T != "button" || n.Action != "more" || len(n.Children) != 1 {
		t.Errorf("trailing = %+v", n)
	}
	if st.Lock != nil || st.Leading != nil || st.CL != nil {
		t.Errorf("places without views hold nodes: %+v", st)
	}
	if len(images) != 1 || st.Min == nil || images[st.Min.Image] == nil {
		t.Errorf("images = %v for %+v", images, st.Min)
	}

	// Without a bar, there is no progress, and pushes carry no images.
	c.ShowProgress = false
	if data, _, _ := encodeActivity(c); strings.Contains(string(data), "progress") {
		t.Errorf("state without a bar = %s", data)
	}
	if _, err := ActivityState(c); err == nil {
		t.Error("ActivityState took views with images")
	}
	c.Minimal = nil
	if _, err := ActivityState(c); err != nil {
		t.Error(err)
	}
}

func TestActivityChecks(t *testing.T) {
	for _, c := range []ActivityContent{
		{},
		{Title: "a", Actions: make([]ActivityAction, 4)},
		{Title: "a", Actions: []ActivityAction{{ID: "x"}}},
	} {
		if _, _, err := encodeActivity(c); err == nil {
			t.Errorf("%+v was taken", c)
		}
	}
	large := ActivityContent{Title: "a", Body: strings.Repeat("x", maxActivityState)}
	if _, _, err := encodeActivity(large); !errors.Is(err, ErrActivityTooLarge) {
		t.Errorf("a large activity = %v", err)
	}
	if _, err := StartActivity("no/slash", ActivityContent{Title: "a"}); err == nil {
		t.Error("StartActivity took a kind with a slash")
	}
}

func TestStartActivity(t *testing.T) {
	_, err := StartActivity("delivery", ActivityContent{Title: "a"})
	switch runtime.GOOS {
	case "darwin":
		// Live Activities are iOS's alone.
		if !errors.Is(err, ErrActivitiesUnsupported) {
			t.Errorf("StartActivity on macOS = %v", err)
		}
	case "ios":
		// A test is no app built with Live Activities.
		if !errors.Is(err, ErrNoActivities) {
			t.Errorf("StartActivity outside an app = %v", err)
		}
	case "android":
		return
	default:
		if !errors.Is(err, errors.ErrUnsupported) {
			t.Errorf("StartActivity on %s = %v", runtime.GOOS, err)
		}
	}
	if ActivitiesEnabled() || len(Activities()) != 0 {
		t.Error("Live Activities run where they cannot")
	}
}

func TestActivityHandlers(t *testing.T) {
	a := &Activity{id: "1", kind: "k"}
	pressed(ActivityPress{Activity: a, Action: "early"})
	tokenChanged(PushToken{Token: "ab"})
	var presses []string
	OnAction(func(p ActivityPress) { presses = append(presses, p.Action) })
	pressed(ActivityPress{Activity: a, Action: "late"})
	if strings.Join(presses, ",") != "early,late" {
		t.Errorf("presses = %v, want the early one first", presses)
	}
	var tokens []string
	OnPushToken(func(p PushToken) { tokens = append(tokens, p.Token) })
	if strings.Join(tokens, ",") != "ab" {
		t.Errorf("tokens = %v", tokens)
	}
}
