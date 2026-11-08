package pkg

import (
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"
)

// Revocations is a root's list of the release keys and packages that
// must not load any more.
type Revocations struct{ s signed }

type revocationsBody struct {
	Format   int      `json:"format"`
	Sequence uint64   `json:"sequence"`
	IssuedAt int64    `json:"issuedAt"`
	Keys     []string `json:"keys"`
	Packages []string `json:"packages"`
}

// RevocationsInfo is what a revocation list says.
type RevocationsInfo struct {
	// Sequence orders the lists: a Trust takes only a newer one.
	Sequence uint64
	IssuedAt time.Time
	// Keys are release keys' IDs and Packages packages' SHA-256s, in hex.
	Keys, Packages []string
}

// Revoke has root sign list sequence, 1 or more, of the release keys
// and packages that must not load.
func Revoke(root ed25519.PrivateKey, sequence uint64, keys, packages []string) (*Revocations, error) {
	if sequence == 0 {
		return nil, fmt.Errorf("%w: revocation lists count from 1", ErrFormat)
	}
	b := revocationsBody{Format: 1, Sequence: sequence, IssuedAt: now().Unix(), Keys: []string{}, Packages: []string{}}
	for _, k := range keys {
		b.Keys = append(b.Keys, strings.ToLower(k))
	}
	for _, p := range packages {
		b.Packages = append(b.Packages, strings.ToLower(p))
	}
	slices.Sort(b.Keys)
	slices.Sort(b.Packages)
	b.Keys, b.Packages = slices.Compact(b.Keys), slices.Compact(b.Packages)
	s, err := signBody(root, contextRevocations, b)
	if err != nil {
		return nil, err
	}
	return &Revocations{s}, nil
}

// ParseRevocations reads a list MarshalJSON wrote.
func ParseRevocations(data []byte) (*Revocations, error) {
	r := new(Revocations)
	if err := r.UnmarshalJSON(data); err != nil {
		return nil, err
	}
	return r, nil
}

// MarshalJSON writes the list as JSON.
func (r *Revocations) MarshalJSON() ([]byte, error) { return json.Marshal(r.s) }

// UnmarshalJSON reads a list.
func (r *Revocations) UnmarshalJSON(data []byte) error {
	if err := decodeStrict(data, &r.s); err != nil {
		return fmt.Errorf("revocation list: %w", err)
	}
	return nil
}

// Info is what the list says, unchecked: nothing has verified its root's
// signature.
func (r *Revocations) Info() (RevocationsInfo, error) {
	var b revocationsBody
	if err := r.s.unchecked(&b); err != nil {
		return RevocationsInfo{}, err
	}
	return b.info()
}

func (r *Revocations) open(roots []ed25519.PublicKey) (RevocationsInfo, error) {
	var b revocationsBody
	if err := r.s.open(contextRevocations, roots, &b); err != nil {
		return RevocationsInfo{}, err
	}
	return b.info()
}

func (b revocationsBody) info() (RevocationsInfo, error) {
	if b.Format != 1 {
		return RevocationsInfo{}, fmt.Errorf("%w: revocation list format %d", ErrFormat, b.Format)
	}
	return RevocationsInfo{Sequence: b.Sequence, IssuedAt: time.Unix(b.IssuedAt, 0), Keys: b.Keys, Packages: b.Packages}, nil
}
