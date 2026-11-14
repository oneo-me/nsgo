//go:build windows

package windows

import (
	"runtime"
	"testing"
	"time"
	"unsafe"
)

func TestToastActivatorCLSIDStable(t *testing.T) {
	a := toastActivatorCLSID("com.example.notes")
	b := toastActivatorCLSID("com.example.notes")
	if a != b {
		t.Fatalf("same app ID produced different CLSIDs: %v != %v", a, b)
	}
	if got := formatGUID(a); len(got) != 38 || got[0] != '{' || got[len(got)-1] != '}' {
		t.Fatalf("invalid CLSID format %q", got)
	}
	if a.d[0]&0xc0 != 0x80 || a.c&0xf000 != 0x5000 {
		t.Fatalf("invalid UUID version/variant: %v", a)
	}
	if a == toastActivatorCLSID("com.example.other") {
		t.Fatal("different app IDs shared a CLSID")
	}
}

func TestParseToastActivationArgs(t *testing.T) {
	for _, test := range []struct {
		args, id, press string
	}{
		{"n=1a;id=invoice-42", "invoice-42", "n=1a;id=invoice-42"},
		{"n=1a;id=a;b&n=2;id=c", "a;b&n=2;id=c", "n=1a;id=a;b&n=2;id=c"},
		{"id=invoice-42", "invoice-42", "id=invoice-42"},
		{"id=a;id=b", "a;id=b", "id=a;id=b"},
		{"  id=hello world  ", "hello world", "id=hello world"},
		{"", "", ""},
		{"action=invoice-42", "", ""},
		{"n=1a;action=invoice-42", "", ""},
	} {
		if id, press := parseToastActivationArgs(test.args); id != test.id || press != test.press {
			t.Errorf("parseToastActivationArgs(%q) = %q, %q, want %q, %q", test.args, id, press, test.id, test.press)
		}
	}
}

// Each toast shown launches with a nonce of its own and parses back to its ID.
func TestToastLaunch(t *testing.T) {
	a, b := toastLaunch("chat:42"), toastLaunch("chat:42")
	if a == b {
		t.Fatalf("two toasts of one ID launch alike: %q", a)
	}
	for _, launch := range []string{a, b} {
		if id, press := parseToastActivationArgs(launch); id != "chat:42" || press != launch {
			t.Errorf("parseToastActivationArgs(%q) = %q, %q, want %q, %q", launch, id, press, "chat:42", launch)
		}
	}
}

// propVariant is as large as PROPVARIANT (propidlbase.h): 16 bytes on 386 and 24 on 64-bit Windows.
func TestPropVariantSize(t *testing.T) {
	want := uintptr(24)
	if runtime.GOARCH == "386" {
		want = 16
	}
	if got := unsafe.Sizeof(propVariant{}); got != want {
		t.Fatalf("propVariant is %d bytes, want %d", got, want)
	}
	if got := unsafe.Offsetof(propVariant{}.value); got != 8 {
		t.Fatalf("propVariant's value is at %d, want 8", got)
	}
}

// A press arriving through both the Activated event and the COM activator reaches the app once; a later press of the toast, another toast's, or one of a toast that replaced it under the same ID, again.
func TestToastPressDedup(t *testing.T) {
	var d toastPressDedup
	at := time.Unix(1000, 0)
	for _, step := range []struct {
		press string
		after time.Duration
		want  bool
	}{
		{"n=1;id=chat:42", 0, true},
		{"n=1;id=chat:42", 300 * time.Millisecond, false},
		{"n=2;id=chat:7", 300 * time.Millisecond, true},
		{"n=3;id=chat:42", 2 * time.Second, true},
		{"n=3;id=chat:42", 2*time.Second + 300*time.Millisecond, false},
		{"n=1;id=chat:42", toastPressWindow - time.Millisecond, false},
		{"n=1;id=chat:42", toastPressWindow, true},
		{"n=1;id=chat:42", toastPressWindow + time.Second, false},
	} {
		if got := d.first(step.press, at.Add(step.after)); got != step.want {
			t.Errorf("first(%q) at +%v = %v, want %v", step.press, step.after, got, step.want)
		}
	}
	if len(d.at) != 2 {
		t.Errorf("held %d presses, want 2 once the others passed the window", len(d.at))
	}
}
