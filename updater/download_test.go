package updater_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/updater"
)

func TestDownloadDelegatesToPlatform(t *testing.T) {
	was := driver.UpdateDownload
	driver.UpdateDownload = func(context.Context, driver.Update, string, func(driver.UpdateDownloadProgress)) (string, error) {
		t.Cleanup(func() { driver.UpdateDownload = was })
		return filepath.Join("/tmp", "hello-2.0.apk"), nil
	}
	path, err := updater.Download(context.Background(), core.UpdateCheck{Version: "2.0"}, t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "hello-2.0.apk" {
		t.Fatalf("path %q", path)
	}
}

func TestDownloadPropagatesPlatformError(t *testing.T) {
	was := driver.UpdateDownload
	driver.UpdateDownload = func(context.Context, driver.Update, string, func(driver.UpdateDownloadProgress)) (string, error) {
		t.Cleanup(func() { driver.UpdateDownload = was })
		return "", fmt.Errorf("signature rejected")
	}
	_, err := updater.Download(context.Background(), core.UpdateCheck{Version: "2.0", Download: "https://example.com/update.apk"}, t.TempDir(), nil)
	if err == nil {
		t.Fatal("bad signature accepted")
	}
}
