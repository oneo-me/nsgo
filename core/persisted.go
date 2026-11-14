package core

import (
	"encoding/json"
	"fmt"
)

// Persisted is a typed value the app's storage keeps under a key as JSON, read as a signal is: it follows the entry, as the app sets it here or elsewhere and as other graphs of the app change it, and decodes it once per change.
//
// Like a signal's, its methods belong to the graph's goroutine.
type Persisted[T any] struct {
	st    Storage
	key   string
	def   T
	opts  PersistOptions
	value *Memo[persistedValue[T]]
}

// PersistOptions are how a Persisted reads entries an older version of the app wrote.
type PersistOptions struct {
	// Version is the version of the value's form the app writes; 0 for a form that never changed.
	Version int
	// Migrate turns the JSON of a value written at an older version into the JSON of the current form. Without it, or when it fails, an older value reads as the default.
	Migrate func(version int, data json.RawMessage) (json.RawMessage, error)
}

type persistedValue[T any] struct {
	v   T
	err error
}

// persistedEntry is the form an entry is stored in.
type persistedEntry struct {
	Version int             `json:"version"`
	Value   json.RawMessage `json:"value"`
}

// NewPersisted returns the value the current owner's storage keeps under key, def when it holds none or what it holds does not decode. It reads the storage Scope.Storage returns, so a partition Scope.ProvideStorage gives holds it.
func NewPersisted[T any](cx *Scope, key string, def T, opts PersistOptions) *Persisted[T] {
	p := &Persisted[T]{st: cx.Storage(), key: key, def: def, opts: opts}
	p.value = NewMemoFunc(cx, p.read, nil)
	return p
}

func (p *Persisted[T]) read() persistedValue[T] {
	raw, ok := p.st.GetItem(p.key)
	if !ok {
		return persistedValue[T]{v: p.def}
	}
	var e persistedEntry
	if err := json.Unmarshal([]byte(raw), &e); err != nil {
		return persistedValue[T]{v: p.def, err: fmt.Errorf("core: stored %q is not a value NewPersisted wrote: %w", p.key, err)}
	}
	data := e.Value
	if e.Version != p.opts.Version {
		if e.Version > p.opts.Version || p.opts.Migrate == nil {
			return persistedValue[T]{v: p.def, err: fmt.Errorf("core: stored %q is at version %d, the app reads %d", p.key, e.Version, p.opts.Version)}
		}
		migrated, err := p.opts.Migrate(e.Version, data)
		if err != nil {
			return persistedValue[T]{v: p.def, err: fmt.Errorf("core: migrating stored %q from version %d: %w", p.key, e.Version, err)}
		}
		data = migrated
	}
	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		return persistedValue[T]{v: p.def, err: fmt.Errorf("core: decoding stored %q: %w", p.key, err)}
	}
	return persistedValue[T]{v: v}
}

// Get returns the value, subscribing the running computation to its entry.
func (p *Persisted[T]) Get() T { return p.value.Get().v }

// Err returns why the entry the storage holds reads as the default: it does not decode, it is at a version the app cannot read, or its migration failed; nil when it reads as stored or there is none. It subscribes the running computation.
func (p *Persisted[T]) Err() error { return p.value.Get().err }

// Set stores v at the current version. It fails when v does not encode, and with a *StorageFullError when the storage would grow past its quota, keeping what it held.
func (p *Persisted[T]) Set(v T) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("core: encoding %q: %w", p.key, err)
	}
	raw, err := json.Marshal(persistedEntry{Version: p.opts.Version, Value: data})
	if err != nil {
		return err
	}
	return p.st.SetItem(p.key, string(raw))
}

// Update stores what fn returns for the current value.
func (p *Persisted[T]) Update(fn func(T) T) error { return p.Set(fn(p.value.GetUntracked().v)) }

// Reset removes the entry, so the value reads as the default.
func (p *Persisted[T]) Reset() { p.st.RemoveItem(p.key) }
