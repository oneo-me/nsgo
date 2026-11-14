package harmonyos

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// pickedFolder is what the host reports for a folder the user picked: the
// name it showed and the URI its files are read through.
type pickedFolder struct {
	Name string `json:"n"`
	URI  string `json:"u"`
}

// folderSource reads a picked folder through the host's synchronous
// bridge, as Android's tree reads one through Java. read is a field so a
// test can answer it without the WebAssembly runtime.
type folderSource struct {
	uri  string
	read func(method, uri, name string) string
}

// listedFolder is one entry the host lists in a folder.
type listedFolder struct {
	Name string `json:"n"`
	Dir  bool   `json:"d"`
	Size int64  `json:"s"`
}

func (f *folderSource) List(name string, e driver.FolderEntry) ([]driver.FolderEntry, error) {
	var in []listedFolder
	if raw := f.read("listdir", f.uri, name); raw != "" {
		if err := json.Unmarshal([]byte(raw), &in); err != nil {
			return nil, err
		}
	}
	out := make([]driver.FolderEntry, 0, len(in))
	for _, x := range in {
		out = append(out, driver.FolderEntry{Name: x.Name, Dir: x.Dir, Size: x.Size})
	}
	return out, nil
}

func (f *folderSource) Open(name string, e driver.FolderEntry) (io.ReadSeekCloser, error) {
	data, err := base64.StdEncoding.DecodeString(f.read("readfile", f.uri, name))
	if err != nil {
		return nil, err
	}
	return readSeekCloser{Reader: bytes.NewReader(data)}, nil
}

// readSeekCloser is a file's bytes as an fs.File the app reads.
type readSeekCloser struct {
	*bytes.Reader
}

func (readSeekCloser) Close() error { return nil }
