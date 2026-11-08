package pkg

import (
	"errors"
	"strconv"
	"strings"
)

// RuntimeVersion is this runtime's version. A package names the lowest
// it runs on, which defaults to the signing tool's.
const RuntimeVersion = "0.1.0"

// Why a package, certificate or revocation list is refused; errors wrap
// them with the details.
var (
	ErrFormat      = errors.New("pkg: not a valid package")
	ErrKey         = errors.New("pkg: not a key")
	ErrUntrusted   = errors.New("pkg: signed by a key that is not trusted")
	ErrSignature   = errors.New("pkg: the signature does not verify")
	ErrCertificate = errors.New("pkg: the release key's certificate does not cover the package")
	ErrRevoked     = errors.New("pkg: revoked")
	ErrRollback    = errors.New("pkg: older than a build accepted before")
	ErrRuntime     = errors.New("pkg: the package needs a newer runtime")
	ErrIntegrity   = errors.New("pkg: a file does not match its hash")
	ErrStale       = errors.New("pkg: the revocation list is not newer than the one held")
	ErrState       = errors.New("pkg: the trust state")
)

// appMatches reports whether pattern, "*", "prefix.*" or an ID, covers
// id.
func appMatches(pattern, id string) bool {
	if pattern == "*" {
		return true
	}
	if prefix, ok := strings.CutSuffix(pattern, ".*"); ok {
		return len(id) > len(prefix)+1 && strings.HasPrefix(id, prefix+".")
	}
	return pattern == id
}

// versionParts reads MAJOR.MINOR.PATCH, ignoring a pre-release or build
// suffix (0.2.0-beta.1 is 0.2.0).
func versionParts(v string) ([3]uint64, bool) {
	var parts [3]uint64
	core, _, _ := strings.Cut(v, "+")
	core, _, _ = strings.Cut(core, "-")
	fields := strings.Split(core, ".")
	if len(fields) != 3 {
		return parts, false
	}
	for i, f := range fields {
		n, err := strconv.ParseUint(f, 10, 64)
		if err != nil {
			return parts, false
		}
		parts[i] = n
	}
	return parts, true
}

// versionAtLeast reports whether have is needs or newer.
func versionAtLeast(have, needs string) bool {
	h, _ := versionParts(have)
	n, ok := versionParts(needs)
	if !ok {
		return false
	}
	for i := range h {
		if h[i] != n[i] {
			return h[i] > n[i]
		}
	}
	return true
}
