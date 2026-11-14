package nsgo

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// devClientSymbols are functions only the dev client has, which nsgo dev's builds link with the nsgodev tag; small ones the compiler inlines are left out, as they leave no symbol.
var devClientSymbols = []string{"nsgo.startDevTools", "nsgo.runDevCommand", "nsgo.listenDevTools", "nsgo.traceTo"}

// TestReleaseBuildsLinkNoDevClient builds an app as nsgo build does and as nsgo dev does and reads their symbols: the dev client, which serves DevTools and traces, links only into dev builds. Core's recording for DevTools links into both, behind devtools.Enabled.
func TestReleaseBuildsLinkNoDevClient(t *testing.T) {
	if testing.Short() {
		t.Skip("builds an app twice")
	}
	dir := t.TempDir()
	symbols := func(tags string) string {
		t.Helper()
		bin := filepath.Join(dir, "hello"+tags)
		args := []string{"build", "-o", bin}
		if tags != "" {
			args = append(args, "-tags", tags)
		}
		build := exec.Command("go", append(args, "./examples/hello")...)
		build.Env = os.Environ()
		if out, err := build.CombinedOutput(); err != nil {
			t.Fatalf("go build %s: %v\n%s", tags, err, out)
		}
		out, err := exec.Command("go", "tool", "nm", bin).Output()
		if err != nil {
			t.Fatalf("go tool nm: %v", err)
		}
		return string(out)
	}
	release, dev := symbols(""), symbols("nsgodev")
	for _, s := range devClientSymbols {
		if strings.Contains(release, s) {
			t.Errorf("a release build links %s", s)
		}
		if !strings.Contains(dev, s) {
			t.Errorf("a dev build lacks %s, so this test reads the wrong names", s)
		}
	}
}
