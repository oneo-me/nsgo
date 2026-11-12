package apk

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	androidruntime "github.com/nsgo-dev/nsgo/internal/platform/android/runtime"
)

// TestNames writes an APK named in several languages and, when the
// Android SDK's aapt2 is installed, checks the labels it reads.
func TestNames(t *testing.T) {
	key, err := NewKey()
	if err != nil {
		t.Fatal(err)
	}
	for _, icon := range []bool{false, true} {
		m := Manifest{Package: "dev.oneo.nsgo.test", Label: "Test", VersionName: "1.0.0", VersionCode: 10000,
			Names: map[string]string{"": "Gallery", "en": "Gallery", "zh-Hans": "画廊", "pt-BR": "Galeria", "ar": "المعرض", "es-419": "Galería"}}
		if icon {
			m.Icon = []byte("\x89PNG icon")
		}
		var buf bytes.Buffer
		if err := Write(&buf, androidruntime.Manifest, m, nil, key); err != nil {
			t.Fatal(err)
		}
		aapt2, err := tool("aapt2")
		if err != nil {
			t.Skip("aapt2 is not installed")
		}
		path := filepath.Join(t.TempDir(), "test.apk")
		if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		out, err := exec.CommandContext(ctx, aapt2, "dump", "badging", path).CombinedOutput()
		if err != nil {
			t.Fatalf("aapt2 dump badging: %v\n%s", err, out)
		}
		for _, want := range []string{"application-label:'Gallery'", "application-label-zh-Hans:'画廊'",
			"application-label-pt-BR:'Galeria'", "application-label-ar:'المعرض'", "application-label-es-419:'Galería'"} {
			if !strings.Contains(string(out), want) {
				t.Errorf("icon %v: no %s in\n%s", icon, want, out)
			}
		}
		if icon && !strings.Contains(string(out), "application-icon-640:'"+iconFile+"'") {
			t.Errorf("no icon in\n%s", out)
		}
	}
}
