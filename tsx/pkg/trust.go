package pkg

import (
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sync"
)

// A Trust verifies packages: it holds the roots an app trusts, the newest
// revocation list and the highest build accepted of each app. Its
// methods may be called from any goroutine; verifying hashes the whole
// package, so call Verify off the engine's.
type Trust struct {
	mu    sync.Mutex
	roots []ed25519.PublicKey
	state trustState
	dir   string
}

type trustState struct {
	Revocations *Revocations      `json:"revocations,omitempty"`
	Builds      map[string]uint64 `json:"builds,omitempty"`
}

// stateFile holds a Trust's state in its directory.
const stateFile = "trust.json"

// NewTrust returns a Trust whose state lasts as long as it does.
func NewTrust() *Trust { return &Trust{state: trustState{Builds: map[string]uint64{}}} }

// OpenTrust returns a Trust whose state is kept in dir, made when first
// written, so rollback protection and revocations survive restarts.
func OpenTrust(dir string) (*Trust, error) {
	t := NewTrust()
	t.dir = dir
	data, err := os.ReadFile(filepath.Join(dir, stateFile))
	if errors.Is(err, fs.ErrNotExist) {
		return t, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrState, err)
	}
	if err := json.Unmarshal(data, &t.state); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrState, err)
	}
	if t.state.Builds == nil {
		t.state.Builds = map[string]uint64{}
	}
	return t, nil
}

// AddRoot trusts the certificates root signs, and its revocation lists.
func (t *Trust) AddRoot(root ed25519.PublicKey) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !slices.ContainsFunc(t.roots, func(r ed25519.PublicKey) bool { return r.Equal(root) }) {
		t.roots = append(t.roots, root)
	}
}

// UpdateRevocations takes list when a root signed it and it is newer
// than the list the Trust holds.
func (t *Trust) UpdateRevocations(list *Revocations) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	info, err := list.open(t.roots)
	if err != nil {
		return err
	}
	if have := t.revocations(); info.Sequence <= have.Sequence {
		return fmt.Errorf("%w: list %d, where the Trust holds %d", ErrStale, info.Sequence, have.Sequence)
	}
	t.state.Revocations = list
	return t.save()
}

// revocations is the list held, if a root still trusted signed it.
func (t *Trust) revocations() RevocationsInfo {
	if t.state.Revocations == nil {
		return RevocationsInfo{}
	}
	info, err := t.state.Revocations.open(t.roots)
	if err != nil {
		return RevocationsInfo{}
	}
	return info
}

// Verify checks a package: its certificate's root, its release key and
// itself not revoked, its signature, its certificate covering its app
// and signing time, its minimum runtime, its files' hashes and its
// build no older than one accepted before.
func (t *Trust) Verify(data []byte) (*Package, error) {
	t.mu.Lock()
	roots := slices.Clone(t.roots)
	revoked := t.revocations()
	t.mu.Unlock()
	p, err := open(data, roots, revoked.Keys)
	if err != nil {
		return nil, err
	}
	if slices.Contains(revoked.Packages, p.SHA256) {
		return nil, fmt.Errorf("%w: package %s", ErrRevoked, p.SHA256)
	}
	if accepted, ok := t.AcceptedBuild(p.App.ID); ok && p.App.Build < accepted {
		return nil, fmt.Errorf("%w: %s build %d, where build %d was accepted", ErrRollback, p.App.ID, p.App.Build, accepted)
	}
	return p, nil
}

// Accept records a package's build as the oldest its app may run from
// now on. A host accepts a package once it proved healthy, so until then
// the app may go back to what it accepted before. A package or key
// revoked since it was verified is not accepted.
func (t *Trust) Accept(p *Package) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	revoked := t.revocations()
	if slices.Contains(revoked.Keys, p.KeyID) {
		return fmt.Errorf("%w: release key %s", ErrRevoked, p.KeyID)
	}
	if slices.Contains(revoked.Packages, p.SHA256) {
		return fmt.Errorf("%w: package %s", ErrRevoked, p.SHA256)
	}
	if have, ok := t.state.Builds[p.App.ID]; ok && p.App.Build <= have {
		return nil
	}
	t.state.Builds[p.App.ID] = p.App.Build
	return t.save()
}

// AcceptedBuild is the highest build of app accepted.
func (t *Trust) AcceptedBuild(app string) (uint64, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	b, ok := t.state.Builds[app]
	return b, ok
}

// save writes the state whole, then renames it into place, so a crash
// never leaves half a file.
func (t *Trust) save() error {
	if t.dir == "" {
		return nil
	}
	data, err := json.MarshalIndent(t.state, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(t.dir, 0o755); err != nil {
		return fmt.Errorf("%w: %v", ErrState, err)
	}
	tmp := filepath.Join(t.dir, stateFile+".tmp")
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("%w: %v", ErrState, err)
	}
	if err := os.Rename(tmp, filepath.Join(t.dir, stateFile)); err != nil {
		return fmt.Errorf("%w: %v", ErrState, err)
	}
	return nil
}
