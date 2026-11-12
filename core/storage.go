package core

import (
	"fmt"
	"log"
	"slices"
	"strings"
	"sync"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// StorageQuota is the most an app may keep, in UTF-8 bytes of its keys and
// values.
const StorageQuota = 5 << 20

// StorageFullError refuses a value that would grow the storage past its
// quota: StorageQuota, or a partition's (Scope.ProvideStorage).
type StorageFullError struct {
	// Bytes is what the storage would have held, and Quota the most it
	// may.
	Bytes, Quota int
}

func (e *StorageFullError) Error() string {
	return fmt.Sprintf("storage is full: %d bytes would exceed the quota of %d", e.Bytes, e.Quota)
}

// partitionMark starts the keys of partitions, which the app's own
// Storage does not list or clear.
const partitionMark = "\x00"

// Store holds an app's entries in memory, loaded from its backend on first
// use. Graphs given the same Store share its entries and quota, and each
// sees the others' changes through its signals. Hosts make one per app
// with NewStore and pass it in RootOptions.
type Store struct {
	mu       sync.Mutex
	backend  driver.StorageBackend
	entries  map[string]string
	bytes    int
	watchers []*storageWatch
}

// NewStore returns a store kept by backend, or only in memory when backend
// is nil.
func NewStore(backend driver.StorageBackend) *Store { return &Store{backend: backend} }

// load reads the backend's entries once. The caller holds mu.
func (s *Store) load() {
	if s.entries != nil {
		return
	}
	s.entries = map[string]string{}
	if s.backend == nil {
		return
	}
	entries, err := s.backend.Load()
	if err != nil {
		log.Printf("nsgo: the app's storage could not be read: %v", err)
		return
	}
	for k, v := range entries {
		s.entries[k] = v
		s.bytes += len(k) + len(v)
	}
}

func (s *Store) get(key string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	v, ok := s.entries[key]
	return v, ok
}

// keys are the keys of the partition prefix names, without it; the
// app's own ("") leaves out every partition's.
func (s *Store) keys(prefix string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	keys := make([]string, 0, len(s.entries))
	for k := range s.entries {
		if rest, ok := strings.CutPrefix(k, prefix); ok && (prefix != "" || !strings.HasPrefix(k, partitionMark)) {
			keys = append(keys, rest)
		}
	}
	slices.Sort(keys)
	return keys
}

// bytes is what the keys of partition prefix and their values hold. The
// caller holds mu.
func (s *Store) partitionBytes(prefix string) int {
	n := 0
	for k, v := range s.entries {
		if strings.HasPrefix(k, prefix) {
			n += len(k) - len(prefix) + len(v)
		}
	}
	return n
}

// change applies c, hands it to the backend and tells the graphs watching,
// all under mu so every graph and the backend see changes in one order. It
// reports whether anything changed.
func (s *Store) change(c StorageChange, from *runtime, part partition) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.load()
	if c.Op == driver.StorageClear && (part.prefix != "" || s.partitioned()) {
		return s.clearPart(part.prefix, from), nil
	}
	switch c.Op {
	case driver.StorageSet:
		old, had := s.entries[c.Key]
		if had && old == c.Value {
			return false, nil
		}
		n := s.bytes + len(c.Key) + len(c.Value)
		if had {
			n -= len(c.Key) + len(old)
		}
		if n > StorageQuota {
			return false, &StorageFullError{Bytes: n, Quota: StorageQuota}
		}
		if part.prefix != "" && part.quota > 0 {
			p := s.partitionBytes(part.prefix) + len(c.Key) - len(part.prefix) + len(c.Value)
			if had {
				p -= len(c.Key) - len(part.prefix) + len(old)
			}
			if p > part.quota {
				return false, &StorageFullError{Bytes: p, Quota: part.quota}
			}
		}
		s.entries[c.Key], s.bytes = c.Value, n
	case driver.StorageRemove:
		old, had := s.entries[c.Key]
		if !had {
			return false, nil
		}
		delete(s.entries, c.Key)
		s.bytes -= len(c.Key) + len(old)
	case driver.StorageClear:
		if len(s.entries) == 0 {
			return false, nil
		}
		clear(s.entries)
		s.bytes = 0
	}
	s.tell(c, from)
	return true, nil
}

// tell hands c to the backend and the graphs watching but from. The
// caller holds mu.
func (s *Store) tell(c StorageChange, from *runtime) {
	if s.backend != nil {
		s.backend.Write(c)
	}
	for _, w := range s.watchers {
		if w.rt != from {
			w.post(c)
		}
	}
}

// partitioned reports whether a partition holds entries. The caller
// holds mu.
func (s *Store) partitioned() bool {
	for k := range s.entries {
		if strings.HasPrefix(k, partitionMark) {
			return true
		}
	}
	return false
}

// clearPart removes the entries of the partition prefix names, the app's
// own when "", one by one, so the others stay. The caller holds mu.
func (s *Store) clearPart(prefix string, from *runtime) bool {
	var gone []string
	for k := range s.entries {
		if strings.HasPrefix(k, prefix) && (prefix != "" || !strings.HasPrefix(k, partitionMark)) {
			gone = append(gone, k)
		}
	}
	slices.Sort(gone)
	for _, k := range gone {
		s.bytes -= len(k) + len(s.entries[k])
		delete(s.entries, k)
		s.tell(StorageChange{Op: driver.StorageRemove, Key: k}, from)
	}
	return len(gone) > 0
}

func (s *Store) watch(w *storageWatch) {
	s.mu.Lock()
	s.watchers = append(s.watchers, w)
	s.mu.Unlock()
}

func (s *Store) unwatch(w *storageWatch) {
	s.mu.Lock()
	s.watchers = slices.DeleteFunc(s.watchers, func(x *storageWatch) bool { return x == w })
	s.mu.Unlock()
}

// storageWatch is one graph's signals over a store: a version per key read
// and one for the set of keys.
type storageWatch struct {
	rt   *runtime
	post func(StorageChange)
	keys map[string]*Signal[uint64]
	all  *Signal[uint64]
}

func (w *storageWatch) key(k string) *Signal[uint64] {
	s := w.keys[k]
	if s == nil {
		s = &Signal[uint64]{rt: w.rt, equal: func(a, b uint64) bool { return a == b }}
		w.keys[k] = s
	}
	return s
}

// changed bumps the signals c affects; it runs on the watching graph.
func (w *storageWatch) changed(c StorageChange) {
	w.rt.root.Batch(func() {
		w.all.Update(func(n uint64) uint64 { return n + 1 })
		if c.Op == driver.StorageClear {
			for _, s := range w.keys {
				s.Update(func(n uint64) uint64 { return n + 1 })
			}
			return
		}
		w.key(c.Key).Update(func(n uint64) uint64 { return n + 1 })
	})
}

// Storage is the app's key-value storage: strings by string key, kept on
// the device and shared by every graph of the app. Reads are synchronous,
// as the entries are in memory, and reading in a computation subscribes it
// to changes, the app's own and other graphs'. Use it where the app's code
// runs: components, handlers, effects and posted functions.
type Storage struct {
	store *Store
	watch *storageWatch
	part  partition
}

// partition is a part of a store: the keys starting with prefix, which
// hold at most quota bytes.
type partition struct {
	prefix string
	quota  int
}

// storageKey is where ProvideStorage keeps an owner's partition.
type storageKey struct{}

// ProvideStorage gives the current owner and what it owns a partition of
// the app's storage, name's, in place of the whole: its keys are only
// theirs, the app's own Storage neither lists nor clears them, and they
// hold at most quota bytes, 0 for no quota of their own; they count
// against the app's StorageQuota too. A host app gives code it did not
// write a partition of its own this way (tsx.View).
func (cx *Scope) ProvideStorage(name string, quota int) {
	o := cx.current()
	if o.values == nil {
		o.values = map[any]any{}
	}
	o.values[storageKey{}] = partition{partitionMark + name + partitionMark, quota}
	cx.rt.storageScoped = true
}

// Storage returns the app's storage: the host's Store, or one in memory
// for the graph's life when the host gave none.
func (cx *Scope) Storage() Storage {
	rt := cx.rt
	if rt.storage == nil {
		if rt.store == nil {
			rt.store = NewStore(nil)
		}
		w := &storageWatch{rt: rt, keys: map[string]*Signal[uint64]{}}
		w.all = &Signal[uint64]{rt: rt, equal: func(a, b uint64) bool { return a == b }}
		if rt.post != nil {
			w.post = func(c StorageChange) { rt.root.Post(func() { w.changed(c) }) }
		} else {
			w.post = func(StorageChange) {}
		}
		rt.store.watch(w)
		store := rt.store
		rt.root.addCleanup(func() { store.unwatch(w) })
		rt.storage = w
	}
	st := Storage{store: rt.store, watch: rt.storage}
	if rt.storageScoped {
		for o := cx.current(); o != nil; o = o.parent {
			if p, ok := o.values[storageKey{}]; ok {
				st.part = p.(partition)
				break
			}
		}
	}
	return st
}

// GetItem returns the value of key and whether it has one.
func (s Storage) GetItem(key string) (string, bool) {
	key = s.part.prefix + key
	s.watch.key(key).Get()
	return s.store.get(key)
}

// SetItem sets key to value. It fails with a *StorageFullError when the
// storage would grow past StorageQuota, keeping what it held.
func (s Storage) SetItem(key, value string) error {
	return s.apply(StorageChange{Op: driver.StorageSet, Key: s.part.prefix + key, Value: value})
}

// RemoveItem removes key.
func (s Storage) RemoveItem(key string) {
	s.apply(StorageChange{Op: driver.StorageRemove, Key: s.part.prefix + key})
}

// Keys returns every key, sorted.
func (s Storage) Keys() []string {
	s.watch.all.Get()
	return s.store.keys(s.part.prefix)
}

// Clear removes every entry.
func (s Storage) Clear() {
	s.apply(StorageChange{Op: driver.StorageClear})
}

func (s Storage) apply(c StorageChange) error {
	changed, err := s.store.change(c, s.watch.rt, s.part)
	if changed {
		if c.Op == driver.StorageClear {
			// A part cleared bumps every key's signal, as a clear does.
			c = StorageChange{Op: driver.StorageClear}
		}
		s.watch.changed(c)
	}
	return err
}

// DataDir returns the directory the app keeps its files in: Application
// Support on Apple platforms, %APPDATA% on Windows, the files directory on
// Android, each with a directory of the app's own. It is "" on the web,
// which has none.
func (cx *Scope) DataDir() string { return cx.rt.dataDir }

// CacheDir returns the directory for files the app can make again, which
// the system may delete when space runs low; "" on the web.
func (cx *Scope) CacheDir() string { return cx.rt.cacheDir }
