package updater

import (
	"context"

	"github.com/nsgo-dev/nsgo/core"
)

// DownloadProgress reports bytes written while an update is downloaded.
type DownloadProgress struct {
	Loaded, Total int64
}

// Download fetches an update's signed package into dir. It verifies the
// appcast's SHA-256 and Ed25519 metadata before making the file visible, so
// an interrupted or tampered download cannot be installed.
func Download(ctx context.Context, update core.UpdateCheck, dir string, progress func(DownloadProgress)) (string, error) {
	return core.DownloadUpdate(ctx, update, dir, func(p core.UpdateDownloadProgress) {
		if progress != nil {
			progress(DownloadProgress(p))
		}
	})
}
