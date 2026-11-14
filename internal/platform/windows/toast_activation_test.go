//go:build windows

package windows

import (
	"runtime"
	"testing"
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
		args, want string
	}{
		{"id=invoice-42", "invoice-42"},
		{"  id=hello world  ", "hello world"},
		{"", ""},
		{"action=invoice-42", ""},
	} {
		if got := parseToastActivationArgs(test.args); got != test.want {
			t.Errorf("parseToastActivationArgs(%q) = %q, want %q", test.args, got, test.want)
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
