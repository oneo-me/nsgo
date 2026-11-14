package updates

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/imageload"
)

func init() { driver.UpdateDownload = Download }

// Download fetches and verifies an appcast package before exposing it in dir.
func Download(ctx context.Context, update driver.Update, dir string, progress func(driver.UpdateDownloadProgress)) (string, error) {
	if update.Version == "" || update.Download == "" {
		return "", errors.New("updater: update has no downloadable package")
	}
	u, err := url.Parse(update.Download)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return "", errors.New("updater: update download is not an HTTP URL")
	}
	if imageload.HTTP == nil {
		return "", errors.New("updater: no platform HTTP client")
	}
	if dir == "" {
		dir = os.TempDir()
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("updater: create download directory: %w", err)
	}
	name := filepath.Base(u.Path)
	if name == "" || name == "." || name == string(filepath.Separator) {
		name = "update-" + strings.ReplaceAll(update.Version, "/", "-")
	}
	part, err := os.CreateTemp(dir, ".nsgo-update-*")
	if err != nil {
		return "", fmt.Errorf("updater: create staging file: %w", err)
	}
	partName := part.Name()
	defer os.Remove(partName)
	if err := part.Close(); err != nil {
		return "", err
	}
	resp, err := imageload.HTTP(ctx, update.Download, imageload.Header{"Cache-Control": {"no-cache"}}, func(loaded, total int64) {
		if progress != nil {
			progress(driver.UpdateDownloadProgress{Loaded: loaded, Total: total})
		}
	})
	if err != nil {
		return "", fmt.Errorf("updater: download: %w", err)
	}
	if resp.Status != 200 {
		return "", fmt.Errorf("updater: download answered %d", resp.Status)
	}
	if update.Size > 0 && int64(len(resp.Body)) != update.Size {
		return "", fmt.Errorf("updater: download size %d, want %d", len(resp.Body), update.Size)
	}
	if err := verify(resp.Body, update); err != nil {
		return "", err
	}
	if err := os.WriteFile(partName, resp.Body, 0o700); err != nil {
		return "", fmt.Errorf("updater: stage download: %w", err)
	}
	dst := filepath.Join(dir, name)
	if err := os.Remove(dst); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("updater: replace old download: %w", err)
	}
	if err := os.Rename(partName, dst); err != nil {
		return "", fmt.Errorf("updater: commit download: %w", err)
	}
	return dst, nil
}

func verify(data []byte, update driver.Update) error {
	if update.SHA256 != "" {
		got := fmt.Sprintf("%x", sha256.Sum256(data))
		if !strings.EqualFold(got, strings.TrimSpace(update.SHA256)) {
			return fmt.Errorf("updater: SHA-256 mismatch: got %s, want %s", got, update.SHA256)
		}
	}
	if update.Signature == "" {
		if update.Key != "" {
			return errors.New("updater: signed update has no signature")
		}
		return nil
	}
	key, err := base64.StdEncoding.DecodeString(update.Key)
	if err != nil || len(key) != ed25519.PublicKeySize {
		return errors.New("updater: update key is not an Ed25519 public key")
	}
	sig, err := base64.StdEncoding.DecodeString(update.Signature)
	if err != nil || len(sig) != ed25519.SignatureSize || !ed25519.Verify(ed25519.PublicKey(key), data, sig) {
		return errors.New("updater: update signature is invalid")
	}
	return nil
}
