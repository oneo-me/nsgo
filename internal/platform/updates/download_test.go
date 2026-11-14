package updates

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/imageload"
)

func TestDownloadVerifiesAndStagesPackage(t *testing.T) {
	public, private, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	body := []byte("signed update")
	sum := sha256.Sum256(body)
	was := imageload.HTTP
	imageload.HTTP = func(context.Context, string, imageload.Header, imageload.Progress) (*imageload.Response, error) {
		return &imageload.Response{Status: 200, Body: body}, nil
	}
	t.Cleanup(func() { imageload.HTTP = was })
	path, err := Download(context.Background(), driver.Update{
		Version: "2.0", Download: "https://example.com/hello-2.0.apk", Size: int64(len(body)),
		SHA256: fmt.Sprintf("%x", sum), Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(private, body)), Key: base64.StdEncoding.EncodeToString(public),
	}, t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "hello-2.0.apk" {
		t.Fatalf("path %q", path)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != string(body) {
		t.Fatalf("staged body %q, %v", got, err)
	}
}

func TestDownloadRejectsBadSignature(t *testing.T) {
	was := imageload.HTTP
	imageload.HTTP = func(context.Context, string, imageload.Header, imageload.Progress) (*imageload.Response, error) {
		return &imageload.Response{Status: 200, Body: []byte("actual")}, nil
	}
	t.Cleanup(func() { imageload.HTTP = was })
	public, _, _ := ed25519.GenerateKey(nil)
	_, err := Download(context.Background(), driver.Update{
		Version: "2.0", Download: "https://example.com/update.apk", Signature: base64.StdEncoding.EncodeToString(make([]byte, ed25519.SignatureSize)), Key: base64.StdEncoding.EncodeToString(public),
	}, t.TempDir(), nil)
	if err == nil {
		t.Fatal("bad signature accepted")
	}
}
