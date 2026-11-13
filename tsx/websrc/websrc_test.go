package websrc

import (
	"strings"
	"testing"
)

// TestBundleKeepsSourceMapLines checks that a shipped script starts on
// its file's first line, so its inline source map's lines hold, and that
// the map stays the file's last line, where a browser reads it.
func TestBundleKeepsSourceMapLines(t *testing.T) {
	const mapLine = "//# sourceMappingURL=data:application/json;base64,e30="
	src := "(() => {\n  first();\n  second();\n})();\n" + mapLine + "\n"
	lines := strings.Split(strings.TrimRight(Bundle("App.tsx", src), "\n"), "\n")
	for i, want := range strings.Split("(() => {\n  first();\n  second();\n})();", "\n") {
		if !strings.HasSuffix(lines[i], want) {
			t.Errorf("line %d = %q, want it to end in %q", i+1, lines[i], want)
		}
	}
	if got := lines[len(lines)-1]; got != mapLine {
		t.Errorf("last line = %q, want the source map's", got)
	}
}
