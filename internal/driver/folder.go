package driver

import (
	"errors"
	"io"
	"io/fs"
	"path"
	"slices"
	"strings"
	"sync"
	"time"
)

// A folder picked on Android or the web has no path: the platform lists
// it and opens its files, and FolderFS makes that an fs.FS. Each call
// waits for the platform, so an app reads folders off the engine's
// goroutine, as it does files.

// FolderEntry is a file or folder a folder holds, as its platform lists
// it.
type FolderEntry struct {
	Name     string
	Dir      bool
	Size     int64
	Modified time.Time
	// Handle is what the platform reads it by: a document's ID on
	// Android, a handle on the web.
	Handle any
}

// FolderSource is how a platform reads a folder without a path. Names
// are slash-separated inside the folder, "." for itself.
type FolderSource interface {
	// List lists the folder e at name.
	List(name string, e FolderEntry) ([]FolderEntry, error)
	// Open opens the file e at name.
	Open(name string, e FolderEntry) (io.ReadSeekCloser, error)
}

// FolderFS reads the folder root through src. It remembers each listing
// to find what a name opens, and lists a folder afresh whenever it is
// read.
func FolderFS(src FolderSource, root FolderEntry) fs.FS {
	root.Name, root.Dir = ".", true
	return &folderFS{src: src, root: root, listed: map[string][]FolderEntry{}}
}

type folderFS struct {
	src  FolderSource
	root FolderEntry

	mu     sync.Mutex
	listed map[string][]FolderEntry // by folder, sorted by name
}

func (f *folderFS) Open(name string) (fs.File, error) {
	e, err := f.entry("open", name)
	if err != nil {
		return nil, err
	}
	if e.Dir {
		return &folderDir{fs: f, name: name, e: e}, nil
	}
	r, err := f.src.Open(name, e)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: err}
	}
	return &folderFile{ReadSeekCloser: r, e: e}, nil
}

func (f *folderFS) ReadDir(name string) ([]fs.DirEntry, error) {
	e, err := f.entry("readdir", name)
	if err != nil {
		return nil, err
	}
	if !e.Dir {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: errNotDir}
	}
	entries, err := f.list(name, e)
	if err != nil {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: err}
	}
	return dirEntries(entries), nil
}

func (f *folderFS) Stat(name string) (fs.FileInfo, error) {
	e, err := f.entry("stat", name)
	if err != nil {
		return nil, err
	}
	return folderInfo{e}, nil
}

var errNotDir = errors.New("not a directory")

// entry finds what name is, listing the folders on its way that were
// not listed yet.
func (f *folderFS) entry(op, name string) (FolderEntry, error) {
	if !fs.ValidPath(name) {
		return FolderEntry{}, &fs.PathError{Op: op, Path: name, Err: fs.ErrInvalid}
	}
	e := f.root
	dir := "."
	for part := range strings.SplitSeq(name, "/") {
		if part == "." {
			break // only "." itself
		}
		if !e.Dir {
			return FolderEntry{}, &fs.PathError{Op: op, Path: name, Err: fs.ErrNotExist}
		}
		f.mu.Lock()
		entries, ok := f.listed[dir]
		f.mu.Unlock()
		if !ok {
			var err error
			if entries, err = f.list(dir, e); err != nil {
				return FolderEntry{}, &fs.PathError{Op: op, Path: name, Err: err}
			}
		}
		i, found := slices.BinarySearchFunc(entries, part, func(e FolderEntry, name string) int { return strings.Compare(e.Name, name) })
		if !found {
			return FolderEntry{}, &fs.PathError{Op: op, Path: name, Err: fs.ErrNotExist}
		}
		e, dir = entries[i], path.Join(dir, part)
	}
	return e, nil
}

// list lists the folder e at name afresh, sorted by name, without the
// names fs.FS cannot open.
func (f *folderFS) list(name string, e FolderEntry) ([]FolderEntry, error) {
	entries, err := f.src.List(name, e)
	if err != nil {
		return nil, err
	}
	entries = slices.DeleteFunc(slices.Clone(entries), func(e FolderEntry) bool {
		return e.Name == "" || e.Name == "." || e.Name == ".." || strings.ContainsRune(e.Name, '/')
	})
	slices.SortStableFunc(entries, func(a, b FolderEntry) int { return strings.Compare(a.Name, b.Name) })
	entries = slices.CompactFunc(entries, func(a, b FolderEntry) bool { return a.Name == b.Name })
	f.mu.Lock()
	f.listed[name] = entries
	f.mu.Unlock()
	return entries, nil
}

func dirEntries(entries []FolderEntry) []fs.DirEntry {
	out := make([]fs.DirEntry, len(entries))
	for i, e := range entries {
		out[i] = fs.FileInfoToDirEntry(folderInfo{e})
	}
	return out
}

// folderInfo describes an entry; the folder itself is named ".".
type folderInfo struct{ e FolderEntry }

func (i folderInfo) Name() string       { return i.e.Name }
func (i folderInfo) Size() int64        { return i.e.Size }
func (i folderInfo) ModTime() time.Time { return i.e.Modified }
func (i folderInfo) IsDir() bool        { return i.e.Dir }
func (i folderInfo) Sys() any           { return nil }

func (i folderInfo) Mode() fs.FileMode {
	if i.e.Dir {
		return fs.ModeDir | 0o555
	}
	return 0o444
}

type folderFile struct {
	io.ReadSeekCloser
	e FolderEntry
}

func (f *folderFile) Stat() (fs.FileInfo, error) { return folderInfo{f.e}, nil }

type folderDir struct {
	fs   *folderFS
	name string
	e    FolderEntry
	// entries are what ReadDir has yet to return, listed on its first
	// call.
	entries []fs.DirEntry
	listed  bool
}

func (d *folderDir) Stat() (fs.FileInfo, error) { return folderInfo{d.e}, nil }
func (d *folderDir) Close() error               { return nil }

func (d *folderDir) Read([]byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: d.name, Err: errors.New("is a directory")}
}

func (d *folderDir) ReadDir(n int) ([]fs.DirEntry, error) {
	if !d.listed {
		entries, err := d.fs.list(d.name, d.e)
		if err != nil {
			return nil, &fs.PathError{Op: "readdir", Path: d.name, Err: err}
		}
		d.entries, d.listed = dirEntries(entries), true
	}
	if n <= 0 {
		out := d.entries
		d.entries = nil
		return out, nil
	}
	if len(d.entries) == 0 {
		return nil, io.EOF
	}
	n = min(n, len(d.entries))
	out := d.entries[:n:n]
	d.entries = d.entries[n:]
	return out, nil
}

// TreeFile is a file of a folder the platform lists whole, as the web's
// <input webkitdirectory> does: its path, slash-separated inside the
// folder, and how it opens.
type TreeFile struct {
	Path     string
	Size     int64
	Modified time.Time
	Open     func() (io.ReadSeekCloser, error)
}

// TreeFS is the folder holding files, and the folders their paths name.
func TreeFS(files []TreeFile) fs.FS {
	t := treeSource{}
	seen := map[string]bool{".": true}
	for i, f := range files {
		p := path.Clean(strings.TrimPrefix(f.Path, "/"))
		if !fs.ValidPath(p) || p == "." || seen[p] {
			continue
		}
		seen[p] = true
		dir, name := path.Split(p)
		dir = path.Clean(dir)
		t[dir] = append(t[dir], FolderEntry{Name: name, Size: f.Size, Modified: f.Modified, Handle: files[i].Open})
		// The folders it is in, each once.
		for dir != "." && !seen[dir] {
			seen[dir] = true
			parent, name := path.Split(dir)
			parent = path.Clean(parent)
			t[parent] = append(t[parent], FolderEntry{Name: name, Dir: true})
			dir = parent
		}
	}
	return FolderFS(t, FolderEntry{})
}

// treeSource is what a TreeFS holds, by folder.
type treeSource map[string][]FolderEntry

func (t treeSource) List(name string, _ FolderEntry) ([]FolderEntry, error) { return t[name], nil }

func (treeSource) Open(_ string, e FolderEntry) (io.ReadSeekCloser, error) {
	open, _ := e.Handle.(func() (io.ReadSeekCloser, error))
	if open == nil {
		return nil, fs.ErrNotExist
	}
	return open()
}
