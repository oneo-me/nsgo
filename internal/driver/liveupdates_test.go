package driver

import (
	"slices"
	"testing"
)

func TestLiveUpdates(t *testing.T) {
	SendLiveUpdate([]byte("a"))
	ReportLiveUpdate([]byte("x"))
	var sent, heard []string
	ConnectLiveUpdates(func(m []byte) { sent = append(sent, string(m)) })
	SendLiveUpdate([]byte("b"))
	HearLiveUpdates(func(r []byte) { heard = append(heard, string(r)) })
	ReportLiveUpdate([]byte("y"))
	if !slices.Equal(sent, []string{"a", "b"}) || !slices.Equal(heard, []string{"x", "y"}) {
		t.Errorf("sent %v and heard %v, want what waited first", sent, heard)
	}
}
