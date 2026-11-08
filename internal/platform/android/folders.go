//go:build android

package android

import (
	"errors"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/oneo-me/nsgo/internal/driver"
)

// A folder picked on Android is a document tree the runtime holds the
// grant to, so the Go process reads it by asking Java: msgFolder lists a
// folder or reads part of a file, and msgFolderReply answers. A document
// is its tree's URI and its ID.

// The requests a msgFolder makes.
const (
	folderList = 0
	folderRead = 1
)

// folderChunk is the most bytes one read asks for.
const folderChunk = 1 << 20

// folders are the folder reads waiting for Java's answers, by request.
type folders struct {
	mu      sync.Mutex
	last    uint32
	waiting map[uint32]chan *reader
	// picks are the folder picks Java has yet to answer, whose EventFiles
	// carry folders rather than files.
	picks map[uint32]bool
}

// ask sends a msgFolder of op for the document id in tree, with what
// write adds, and waits for Java's answer, its error read.
func (s *surface) ask(op byte, tree, id string, write func(w *writer)) (*reader, error) {
	f := &s.folders
	ch := make(chan *reader, 1)
	f.mu.Lock()
	f.last++
	n := f.last
	if f.waiting == nil {
		f.waiting = map[uint32]chan *reader{}
	}
	f.waiting[n] = ch
	f.mu.Unlock()
	s.send(msgFolder, func(w *writer) {
		w.u32(n)
		w.byte(op)
		w.str(tree)
		w.str(id)
		if write != nil {
			write(w)
		}
	})
	r := <-ch
	if msg := r.str(); msg != "" {
		return nil, errors.New(msg)
	}
	return r, nil
}

// folderReply hands Java's answer to the read waiting for it.
func (s *surface) folderReply(r *reader) {
	f := &s.folders
	n := r.u32()
	f.mu.Lock()
	ch := f.waiting[n]
	delete(f.waiting, n)
	f.mu.Unlock()
	if ch != nil {
		ch <- &reader{buf: append([]byte(nil), r.buf...)}
	}
}

// pickFolders records a folder pick for its answer.
func (s *surface) pickFolders(id uint32) {
	f := &s.folders
	f.mu.Lock()
	if f.picks == nil {
		f.picks = map[uint32]bool{}
	}
	f.picks[id] = true
	f.mu.Unlock()
}

// pickedFolders reports whether EventFiles id answers a folder pick, which
// it then forgets.
func (s *surface) pickedFolders(id uint32) bool {
	f := &s.folders
	f.mu.Lock()
	defer f.mu.Unlock()
	ok := f.picks[id]
	delete(f.picks, id)
	return ok
}

// readFolders reads the folders an EventFiles answering a folder pick
// carries, one per line: the name, the tree's URI and its root's ID.
func (s *surface) readFolders(data string) []driver.Directory {
	var dirs []driver.Directory
	for line := range strings.SplitSeq(data, "\n") {
		if f := strings.Split(line, "\t"); len(f) == 3 {
			src := tree{s: s, uri: f[1]}
			root := driver.FolderEntry{Handle: documentRef{id: f[2]}}
			dirs = append(dirs, driver.Directory{Name: f[0], FS: driver.FolderFS(src, root)})
		}
	}
	return dirs
}

// tree reads a document tree through Java, each entry's handle its
// documentRef.
type tree struct {
	s   *surface
	uri string
}

// documentRef is a document of a tree: its ID, and whether its provider
// said its size.
type documentRef struct {
	id    string
	sized bool
}

func (t tree) List(_ string, e driver.FolderEntry) ([]driver.FolderEntry, error) {
	r, err := t.s.ask(folderList, t.uri, e.Handle.(documentRef).id, nil)
	if err != nil {
		return nil, err
	}
	entries := make([]driver.FolderEntry, r.u32())
	for i := range entries {
		e := &entries[i]
		e.Name = r.str()
		ref := documentRef{id: r.str()}
		e.Dir = r.byte() != 0
		// -1 for a size the provider does not say.
		e.Size = int64(r.u64())
		ref.sized = e.Size >= 0
		e.Size = max(e.Size, 0)
		e.Handle = ref
		if ms := int64(r.u64()); ms > 0 {
			e.Modified = time.UnixMilli(ms)
		}
	}
	return entries, nil
}

func (t tree) Open(_ string, e driver.FolderEntry) (io.ReadSeekCloser, error) {
	ref := e.Handle.(documentRef)
	d := &document{tree: t, id: ref.id, size: -1}
	if ref.sized {
		d.size = e.Size
	}
	return d, nil
}

// document reads a file of a tree a chunk at a time; it seeks anywhere,
// and from its end once its size is known.
type document struct {
	tree
	id  string
	off int64
	// size is -1 until known.
	size int64
}

func (d *document) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if d.size >= 0 && d.off >= d.size {
		return 0, io.EOF
	}
	n := min(len(p), folderChunk)
	r, err := d.s.ask(folderRead, d.uri, d.id, func(w *writer) {
		w.u64(uint64(d.off))
		w.u32(uint32(n))
	})
	if err != nil {
		return 0, err
	}
	got := copy(p, r.str())
	if got == 0 {
		// The end, which the listing may not have said.
		d.size = d.off
		return 0, io.EOF
	}
	d.off += int64(got)
	return got, nil
}

func (d *document) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekCurrent:
		offset += d.off
	case io.SeekEnd:
		if d.size < 0 {
			return 0, errors.New("the document's size is unknown")
		}
		offset += d.size
	}
	if offset < 0 {
		return 0, errors.New("seek before the start")
	}
	d.off = offset
	return offset, nil
}

func (*document) Close() error { return nil }
