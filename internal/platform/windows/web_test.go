//go:build windows

package windows

import (
	"testing"
	"unsafe"
)

func TestWebEnvironmentOptionsRegisterSchemes(t *testing.T) {
	o := newWebEnvironmentOptions([]string{"app", "gallery"})
	opts := (*comObject)(unsafe.Pointer(o))
	if v := opts.getString(7); v != webTargetCompatibleVersion { // get_TargetCompatibleBrowserVersion
		t.Errorf("target version %q", v)
	}
	if options5 := parseGUID("0ae35d64-c47f-4464-814e-259c345d1501"); opts.query(&options5) != nil {
		t.Error("options claim ICoreWebView2EnvironmentOptions5")
	}
	opts4 := opts.query(&iidEnvironmentOptions4)
	if opts4 == nil {
		t.Fatal("no ICoreWebView2EnvironmentOptions4")
	}
	var count uint32
	var list *[2]*comObject
	if hr := opts4.call(3, uintptr(unsafe.Pointer(&count)), uintptr(unsafe.Pointer(&list))); failed(hr) || count != 2 {
		t.Fatalf("GetCustomSchemeRegistrations = %#x, %d", hr, count)
	}
	for i, want := range []string{"app", "gallery"} {
		r := list[i]
		if name := r.getString(3); name != want {
			t.Errorf("scheme %d = %q, want %q", i, name, want)
		}
		if !r.getBool(4) || !r.getBool(8) {
			t.Errorf("scheme %q is not secure with an authority", want)
		}
		r.release()
	}
	coTaskMemFree.Call(uintptr(unsafe.Pointer(list)))
	opts4.release()
	o.release()
	webEnvironmentObjectsMu.Lock()
	defer webEnvironmentObjectsMu.Unlock()
	if len(webEnvironmentObjects) != 0 {
		t.Errorf("%d objects still held", len(webEnvironmentObjects))
	}
}

func TestWebRequestHeadersAreLines(t *testing.T) {
	got := webRequestHeaders(map[string]string{"X-B": "2", "X-A": "1", "Bad\r\nName": "x", "X-C": "a\r\nInjected: y", "": "z"})
	if want := "X-A: 1\r\nX-B: 2"; got != want {
		t.Errorf("webRequestHeaders = %q, want %q", got, want)
	}
	if got := webRequestHeaders(nil); got != "" {
		t.Errorf("webRequestHeaders(nil) = %q", got)
	}
}

func TestWebDownloadFilenameStaysInDownloadDirectory(t *testing.T) {
	for _, test := range []struct {
		path, want string
	}{
		{`C:\Users\me\Downloads\report.pdf`, "report.pdf"},
		{`..\..\secret.txt`, "secret.txt"},
		{`..`, "download"},
		{`C:\`, "download"},
	} {
		if got := webDownloadFilename(test.path); got != test.want {
			t.Errorf("webDownloadFilename(%q) = %q, want %q", test.path, got, test.want)
		}
	}
}
