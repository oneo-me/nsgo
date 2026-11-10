package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/nsgotest"
)

// TestMain hands the main thread to the platform under nsgo test -target.
func TestMain(m *testing.M) { nsgotest.Main(m) }

// start runs the demo id as the website mounts it.
func start(t *testing.T, id string) *nsgotest.App {
	h := core.NewHost()
	h.SetParams(map[string]any{"demo": id})
	return nsgotest.Start(t, Root, nsgotest.Host(h), nsgotest.Size(480, 320))
}

func TestEveryDemoShows(t *testing.T) {
	for id := range demos {
		t.Run(id, func(t *testing.T) {
			app := start(t, id)
			if app.Exists("text:No preview " + id) {
				t.Fatal("the demo did not show")
			}
		})
	}
}

func TestButtonSays(t *testing.T) {
	app := start(t, "button/basic")
	app.Press("text:Save")
	app.WaitFor("text:Saved", time.Second)
}

func TestPickerWithoutHost(t *testing.T) {
	app := nsgotest.Start(t, Root, nsgotest.Size(480, 320))
	app.Find("text:button/basic")
	app.Find("text:Save")
}

// previewMark is a code block's preview mark in the docs.
var previewMark = regexp.MustCompile("(?m)^```go\\b.*\\bpreview=(\\S+)")

// TestDocsNameDemos checks that every preview mark in the docs names a
// demo and every demo has an English page showing it.
func TestDocsNameDemos(t *testing.T) {
	root := filepath.Join("..", "..", "docs", "public")
	used := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".md") {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		english := strings.HasPrefix(filepath.ToSlash(path), filepath.ToSlash(root)+"/en/")
		for _, m := range previewMark.FindAllStringSubmatch(string(data), -1) {
			if _, ok := demos[m[1]]; !ok {
				t.Errorf("%s marks preview=%s, which no demo is", path, m[1])
			}
			if english {
				used[m[1]] = true
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var unused []string
	for id := range demos {
		if !used[id] {
			unused = append(unused, id)
		}
	}
	slices.Sort(unused)
	if len(unused) > 0 {
		t.Errorf("no English page shows %v", unused)
	}
}
