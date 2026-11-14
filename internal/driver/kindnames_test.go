package driver

import "testing"

// TestKindNames checks the names go generate writes: a log says ItemModal,
// and a kind the driver does not know keeps its number.
func TestKindNames(t *testing.T) {
	cases := []struct {
		got  string
		want string
	}{
		{ItemModal.String(), "ItemModal"},
		{ItemCreate.String(), "ItemCreate"},
		{EventPress.String(), "EventPress"},
		{KindCanvas.String(), "canvas"}, // trace.go names a view kind for traces
		{DrawSave.String(), "DrawSave"},
		{ItemKind(200).String(), "ItemKind(200)"},
		{EventKind(0).String(), "EventKind(0)"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("got %q, want %q", c.got, c.want)
		}
	}
}
