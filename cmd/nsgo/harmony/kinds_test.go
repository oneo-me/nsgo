package harmony_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var (
	constBlock = regexp.MustCompile(`(?s)const \((.*?)\n\)`)
	kindName   = regexp.MustCompile(`(?m)^\s*(Kind[A-Z]\w*)\b`)
	hostKind   = regexp.MustCompile(`(?m)^\s*const (KIND_[A-Z_]+): number = ([0-9]+);`)
)

// TestHostKindsMatchDriver checks that every view kind the ArkTS host knows
// has the driver's number. A host constant that drifts draws the wrong view
// for a node, or none: KIND_CANVAS was 6 while KindCanvas was 7.
func TestHostKindsMatchDriver(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	driver := driverKindNumbers(t, filepath.Join(root, "internal", "driver", "mount.go"))
	data, err := os.ReadFile(filepath.Join(root, "cmd", "nsgo", "harmony", "templates", "entry", "src", "main", "ets", "pages", "Index.ets"))
	if err != nil {
		t.Fatal(err)
	}
	host := hostKind.FindAllStringSubmatch(string(data), -1)
	if len(host) == 0 {
		t.Fatal("the host declares no KIND_ constants: the pattern no longer matches")
	}
	for _, m := range host {
		name := driverKindName(m[1])
		got, ok := driver[name]
		if !ok {
			t.Errorf("%s has no driver kind %s", m[1], name)
			continue
		}
		if want := int(m[2][0] - '0'); got != want {
			t.Errorf("%s is %d, driver's %s is %d", m[1], want, name, got)
		}
	}
}

// driverKindNumbers reads the Kind constants in order, KindView first at 1.
func driverKindNumbers(t *testing.T, path string) map[string]int {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]int{}
	for _, b := range constBlock.FindAllStringSubmatch(string(data), -1) {
		names := kindName.FindAllStringSubmatch(b[1], -1)
		if len(names) == 0 || names[0][1] != "KindView" {
			continue
		}
		for i, n := range names {
			out[n[1]] = i + 1
		}
	}
	if len(out) == 0 {
		t.Fatal("no Kind constants found: the pattern no longer matches")
	}
	return out
}

// driverKindName turns KIND_CANVAS into KindCanvas.
func driverKindName(constName string) string {
	parts := strings.Split(strings.TrimPrefix(constName, "KIND_"), "_")
	for i, p := range parts {
		parts[i] = strings.ToUpper(p[:1]) + strings.ToLower(p[1:])
	}
	return "Kind" + strings.Join(parts, "")
}
