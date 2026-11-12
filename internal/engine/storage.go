package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
)

// AppDirs returns app id's data and cache directories on this system:
// Application Support and Caches on Apple platforms, %APPDATA% and
// %LOCALAPPDATA% on Windows, the XDG directories elsewhere, each with a
// directory named by id, or "default" when id names no directory. Both are
// "" where only the platform knows them, on Android and the web.
func AppDirs(id string) (data, cache string) {
	return appDirs(runtime.GOOS, id, os.Getenv)
}

func appDirs(goos, id string, env func(string) string) (data, cache string) {
	home := env("HOME")
	under := func(dir, rest string) string {
		if dir == "" {
			return ""
		}
		return filepath.Join(dir, rest)
	}
	switch goos {
	case "android", "js":
		return "", ""
	case "darwin", "ios":
		data, cache = under(home, "Library/Application Support"), under(home, "Library/Caches")
	case "windows":
		data, cache = env("APPDATA"), env("LOCALAPPDATA")
	default:
		data, cache = env("XDG_DATA_HOME"), env("XDG_CACHE_HOME")
		if data == "" {
			data = under(home, ".local/share")
		}
		if cache == "" {
			cache = under(home, ".cache")
		}
	}
	name := appDirName(id)
	return under(data, name), under(cache, name)
}

// appDirName is the directory of app id: the id, or "default" for an id
// that is empty or would leave the directory it is joined to.
func appDirName(id string) string {
	if id == "" || id == "." || id == ".." || strings.ContainsAny(id, "/\\\x00") {
		return "default"
	}
	return id
}

// NewStorage returns a store kept by backend, for platforms that keep the
// app's entries themselves, such as in the browser's localStorage.
func NewStorage(backend driver.StorageBackend) *core.Store { return core.NewStore(backend) }

// OpenFileStorage returns the store kept in dir/storage.json: the one
// already open in this process, so every engine of the app shares it, or
// a new one. The directory is made on the first write.
func OpenFileStorage(dir string) *core.Store {
	path := filepath.Join(dir, "storage.json")
	openMu.Lock()
	defer openMu.Unlock()
	if s := open[path]; s != nil {
		return s
	}
	f := &fileStorage{path: path}
	f.idle = sync.NewCond(&f.mu)
	s := core.NewStore(f)
	open[path] = s
	files = append(files, f)
	return s
}

var (
	openMu sync.Mutex
	open   = map[string]*core.Store{}
	files  []*fileStorage
)

// SyncStorage waits until every change made so far to the stores
// OpenFileStorage returned is written. Platforms call it as the process
// ends, which would otherwise cut the last write short.
func SyncStorage() {
	openMu.Lock()
	all := slices.Clone(files)
	openMu.Unlock()
	for _, f := range all {
		f.mu.Lock()
		for f.writing {
			f.idle.Wait()
		}
		f.mu.Unlock()
	}
}

// fileStorage keeps the entries as one JSON object, rewritten whole on a
// goroutine of its own after changes: to a temporary file, then renamed
// over it, so a crash leaves the old file or the new one.
type fileStorage struct {
	path string

	mu      sync.Mutex
	entries map[string]string // as changed so far
	dirty   bool
	writing bool
	idle    *sync.Cond // signaled when writing stops
	// inMemory is set when the file could not be read: changes stay in
	// memory and never replace what it holds.
	inMemory bool
}

func (f *fileStorage) Load() (map[string]string, error) {
	entries, err := f.read()
	f.mu.Lock()
	defer f.mu.Unlock()
	f.entries = maps.Clone(entries)
	if f.entries == nil {
		f.entries = map[string]string{}
	}
	return entries, err
}

// read returns the file's entries. A file that reads but does not parse is
// set aside as storage.json.corrupt rather than left for the next write to
// replace; one that cannot be read keeps the changes in memory.
func (f *fileStorage) read() (map[string]string, error) {
	data, err := os.ReadFile(f.path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		f.inMemory = true
		return nil, fmt.Errorf("%w; changes are kept in memory", err)
	}
	var entries map[string]string
	if err := json.Unmarshal(data, &entries); err != nil {
		aside := f.path + ".corrupt"
		if rerr := os.Rename(f.path, aside); rerr != nil {
			f.inMemory = true
			return nil, fmt.Errorf("%s: %w; it could not be set aside (%v), so changes are kept in memory", f.path, err, rerr)
		}
		return nil, fmt.Errorf("%s: %w; it was set aside as %s", f.path, err, aside)
	}
	return entries, nil
}

func (f *fileStorage) Write(c driver.StorageChange) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch c.Op {
	case driver.StorageSet:
		f.entries[c.Key] = c.Value
	case driver.StorageRemove:
		delete(f.entries, c.Key)
	case driver.StorageClear:
		clear(f.entries)
	}
	if f.inMemory {
		return
	}
	f.dirty = true
	if !f.writing {
		f.writing = true
		go f.flush()
	}
}

// flush writes the entries until no change is left unwritten, so a burst
// of changes costs one or two writes.
func (f *fileStorage) flush() {
	for {
		f.mu.Lock()
		if !f.dirty {
			f.writing = false
			f.idle.Broadcast()
			f.mu.Unlock()
			return
		}
		f.dirty = false
		data, err := json.Marshal(f.entries)
		f.mu.Unlock()
		if err == nil {
			err = writeFile(f.path, data)
		}
		if err != nil {
			log.Printf("nsgo: the app's storage could not be written: %v", err)
		}
	}
}

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	file, err := os.Create(tmp)
	if err != nil {
		return err
	}
	_, err = file.Write(data)
	if err == nil {
		err = file.Sync()
	}
	if cerr := file.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

// SetDirs sets the app's directories and keeps its storage in a file in
// data, unless data is "".
func (o *Options) SetDirs(data, cache string) {
	o.DataDir, o.CacheDir = data, cache
	if data != "" {
		o.Storage = OpenFileStorage(data)
	}
}
