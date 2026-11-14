package harmonyos

import (
	"encoding/base64"
	"io"
	"io/fs"
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

func TestPickedFolderReadsThroughTheHost(t *testing.T) {
	body := "hello folder"
	src := &folderSource{uri: "file://docs/tree/1", read: func(method, uri, name string) string {
		if uri != "file://docs/tree/1" {
			t.Fatalf("uri = %q", uri)
		}
		switch method + ":" + name {
		case "listdir:.":
			return `[{"n":"a.txt","d":false,"s":12},{"n":"sub","d":true,"s":0}]`
		case "listdir:sub":
			return `[{"n":"b.txt","d":false,"s":3}]`
		case "readfile:a.txt":
			return base64.StdEncoding.EncodeToString([]byte(body))
		case "readfile:sub/b.txt":
			return base64.StdEncoding.EncodeToString([]byte("xyz"))
		}
		t.Fatalf("unexpected %s %q", method, name)
		return ""
	}}
	fsys := driver.FolderFS(src, driver.FolderEntry{})

	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].Name() != "a.txt" || entries[1].Name() != "sub" || !entries[1].IsDir() {
		t.Fatalf("entries = %+v", entries)
	}

	f, err := fsys.Open("a.txt")
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != body {
		t.Fatalf("a.txt = %q", got)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	info, err := fs.Stat(fsys, "sub")
	if err != nil || !info.IsDir() {
		t.Fatalf("stat sub = %v, %v", info, err)
	}
	r, err := fsys.Open("sub/b.txt")
	if err != nil {
		t.Fatal(err)
	}
	got, err = io.ReadAll(r)
	if err != nil || string(got) != "xyz" {
		t.Fatalf("sub/b.txt = %q, %v", got, err)
	}
}
