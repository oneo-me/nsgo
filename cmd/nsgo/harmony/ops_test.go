package harmony_test

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var (
	goWriteOp = regexp.MustCompile(`\.op\("([a-z]+)"`)
	hostReads = regexp.MustCompile(`code === '([a-z]+)'`)
)

// TestOpsMatchHost checks that every operation the Go renderer writes has a
// branch the ArkTS host reads. A name the host does not know is dropped
// silently, so a new element's view goes missing with no error anywhere.
func TestOpsMatchHost(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	written := names(t, filepath.Join(root, "internal", "platform", "harmonyos", "mount.go"), goWriteOp)
	read := names(t, filepath.Join(root, "cmd", "nsgo", "harmony", "templates", "entry", "src", "main", "ets", "pages", "Index.ets"), hostReads)
	if len(written) == 0 || len(read) == 0 {
		t.Fatalf("found %d operations and %d branches: the patterns no longer match", len(written), len(read))
	}
	var missing []string
	for op := range written {
		if !read[op] {
			missing = append(missing, op)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Errorf("operations the host does not read: %s", strings.Join(missing, ", "))
	}
}

// TestHostReadsNothingTheRendererNeverWrites checks the other way: a branch
// for an operation Go never writes is dead code in the host.
func TestHostReadsNothingTheRendererNeverWrites(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	written := names(t, filepath.Join(root, "internal", "platform", "harmonyos", "mount.go"), goWriteOp)
	read := names(t, filepath.Join(root, "cmd", "nsgo", "harmony", "templates", "entry", "src", "main", "ets", "pages", "Index.ets"), hostReads)
	var dead []string
	for op := range read {
		if !written[op] {
			dead = append(dead, op)
		}
	}
	if len(dead) > 0 {
		sort.Strings(dead)
		t.Errorf("branches for operations the renderer never writes: %s", strings.Join(dead, ", "))
	}
}

func names(t *testing.T, path string, re *regexp.Regexp) map[string]bool {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, m := range re.FindAllStringSubmatch(string(data), -1) {
		out[m[1]] = true
	}
	return out
}
