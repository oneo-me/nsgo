package field

import (
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

func newState(events ...driver.EventKind) (*State, *[]driver.Event) {
	var got []driver.Event
	s := &State{ID: 7, Dispatch: func(ev driver.Event) { got = append(got, ev) }}
	for _, k := range events {
		s.Events |= 1 << k
	}
	return s, &got
}

func TestEditsAndStaleValues(t *testing.T) {
	s, got := newState()
	if !s.Edited("a") || s.Edited("a") {
		t.Fatal("an edit to the known text was reported")
	}
	if len(*got) != 1 || (*got)[0].Kind != driver.EventChangeText || (*got)[0].Count != 1 {
		t.Fatalf("events %+v", *got)
	}
	if s.Value("x", 0) {
		t.Fatal("a value older than the edit was applied")
	}
	if !s.Value("A", 1) || s.Known() != "A" || s.Edited("A") {
		t.Fatal("the app's value was not applied and known")
	}
}

func TestValueHeldDuringComposition(t *testing.T) {
	s, got := newState(driver.EventCompositionChange)
	s.Compose(true, "ni", "")
	if s.Value("X", 0) {
		t.Fatal("a value was applied during a composition")
	}
	s.Edited("你")
	// The committed text was reported after the value: it is dropped.
	if _, apply := s.Compose(false, "", "你"); apply {
		t.Fatal("a stale held value was applied")
	}
	s.Compose(true, "hao", "你")
	s.Value("你!", 1)
	if v, apply := s.Compose(false, "", "你"); !apply || v != "你!" {
		t.Fatalf("held value %q %v", v, apply)
	}
	n := 0
	for _, ev := range *got {
		if ev.Kind == driver.EventCompositionChange {
			n++
		}
	}
	if n != 4 {
		t.Fatalf("%d composition events, want 4", n)
	}
}

func TestInsertAndReturn(t *testing.T) {
	s, got := newState(driver.EventKeyPress, driver.EventSubmitEditing)
	s.Config.MaxLength = 3
	if kept, ok, whole := s.Insert("ab", 0, "c\nd"); kept != "c" || !ok || whole {
		t.Fatalf("kept %q %v %v", kept, ok, whole)
	}
	if _, ok, _ := s.Insert("abc", 0, "d"); ok {
		t.Fatal("a full field took more")
	}
	if insert, blur := s.Return("abc", false); insert || !blur {
		t.Fatal("a single line's return did not blur and submit")
	}
	s.Config.Multiline = true
	if insert, _ := s.Return("abc", false); !insert {
		t.Fatal("a multiline return did not break the line")
	}
	s.Config.SubmitBehavior = driver.SubmitBehaviorSubmit
	if insert, _ := s.Return("abc", true); !insert {
		t.Fatal("shift-return did not break the line")
	}
	if insert, blur := s.Return("abc", false); insert || blur {
		t.Fatal("submit blurred")
	}
	var keys []string
	for _, ev := range *got {
		if ev.Kind == driver.EventKeyPress {
			keys = append(keys, ev.Text)
		}
	}
	if len(keys) != 3 || keys[0] != "c" || keys[1] != "Enter" {
		t.Fatalf("keys %q", keys)
	}
}
