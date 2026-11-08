//go:build !js

package pkg

import (
	"os"
	"path/filepath"
)

// stateFile holds a Trust's state in its directory.
const stateFile = "trust.json"

func readState(dir string) ([]byte, error) { return os.ReadFile(filepath.Join(dir, stateFile)) }

// writeState writes the state whole, then renames it into place, so a
// crash never leaves half a file.
func writeState(dir string, data []byte) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp := filepath.Join(dir, stateFile+".tmp")
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, stateFile))
}
