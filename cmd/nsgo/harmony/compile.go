package harmony

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// ErrNoSDK reports that the machine has no OpenHarmony SDK to build a host
// with. A caller that only wants the exact bundle name falls back to the
// committed host.
var ErrNoSDK = errors.New("no OpenHarmony SDK")

// compileMu keeps two builds of the same host from writing one cache
// directory at once.
var compileMu sync.Mutex

// sdkAvailable reports whether hvigorw and an SDK are here, as host/build.sh
// needs them.
func sdkAvailable() error {
	if _, err := exec.LookPath("hvigorw"); err != nil {
		return fmt.Errorf("%w: hvigorw is not on PATH", ErrNoSDK)
	}
	root := os.Getenv("OHOS_BASE_SDK_HOME")
	if root == "" {
		return fmt.Errorf("%w: OHOS_BASE_SDK_HOME is not set", ErrNoSDK)
	}
	if fi, err := os.Stat(root); err != nil || !fi.IsDir() {
		return fmt.Errorf("%w: OHOS_BASE_SDK_HOME %q is not a directory", ErrNoSDK, root)
	}
	return nil
}

// runHvigor builds the project in dir, and is a variable so a test can
// stand in for the SDK.
var runHvigor = func(ctx context.Context, dir string) error {
	cmd := exec.CommandContext(ctx, "hvigorw",
		"--mode", "module", "-p", "product=default", "-p", "module=entry@default",
		"-p", "buildMode=debug", "assembleHap", "--no-daemon")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("hvigorw assembleHap: %w\n%s\n(the OpenHarmony SDK's license may need accepting in DevEco Studio)", err, out)
	}
	return nil
}

// CompileHost builds a host for bundle with the SDK and returns the directory
// holding its files, from the user's cache once it is there. It returns
// ErrNoSDK when the machine cannot build one.
func CompileHost(ctx context.Context, bundle string) (string, error) {
	if err := sdkAvailable(); err != nil {
		return "", err
	}
	key, err := hostKey(bundle)
	if err != nil {
		return "", err
	}
	root, err := cacheRoot()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(root, key)
	if hostComplete(dir) {
		return dir, nil
	}
	compileMu.Lock()
	defer compileMu.Unlock()
	if hostComplete(dir) {
		return dir, nil
	}
	if err := buildHost(ctx, bundle, dir); err != nil {
		return "", err
	}
	return dir, nil
}

// cacheRoot is where compiled hosts wait between builds.
func cacheRoot() (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cache, "nsgo", "harmony-host"), nil
}

// hostComplete reports whether dir holds a host PackageWithHost can use.
func hostComplete(dir string) bool {
	for _, name := range []string{"module.json", "pack.info", "ets/modules.abc"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(name))); err != nil {
			return false
		}
	}
	return true
}

// hostKey names a cached host by the bundle, the templates and the SDK, so a
// change to any of them builds afresh.
func hostKey(bundle string) (string, error) {
	h := sha256.New()
	io.WriteString(h, "harmony-host-v1\x00")
	io.WriteString(h, bundle)
	io.WriteString(h, "\x00")
	err := fs.WalkDir(templates, "templates", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := templates.ReadFile(p)
		if err != nil {
			return err
		}
		io.WriteString(h, p)
		io.WriteString(h, "\x00")
		h.Write(data)
		io.WriteString(h, "\x00")
		return nil
	})
	if err != nil {
		return "", err
	}
	io.WriteString(h, sdkStamp())
	return hex.EncodeToString(h.Sum(nil)[:16]), nil
}

// sdkStamp identifies the SDK a host was built with, so an upgrade does not
// reuse an older host.
func sdkStamp() string {
	root := os.Getenv("OHOS_BASE_SDK_HOME")
	matches, _ := filepath.Glob(filepath.Join(root, "*", "ets", "oh-uni-package.json"))
	sort.Strings(matches)
	for _, m := range matches {
		if data, err := os.ReadFile(m); err == nil {
			return string(data)
		}
	}
	return root
}

// buildHost writes a host for bundle into dir through a temporary tree, so a
// failed build leaves no half-written cache entry.
func buildHost(ctx context.Context, bundle, dir string) error {
	parent := filepath.Dir(dir)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(parent, "build-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	work := filepath.Join(tmp, "project")
	if err := materialize(work, bundle); err != nil {
		return err
	}
	if err := runHvigor(ctx, work); err != nil {
		return err
	}
	hap := filepath.Join(work, "entry", "build", "default", "outputs", "default", "entry-default-unsigned.hap")
	out := filepath.Join(tmp, "host")
	if err := unzipHAP(hap, out); err != nil {
		return err
	}
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	return os.Rename(out, dir)
}

// materialize writes the templates to dst with the host's placeholders filled
// in, as host/build.sh does.
func materialize(dst, bundle string) error {
	err := fs.WalkDir(templates, "templates", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := templates.ReadFile(p)
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(strings.TrimPrefix(p, "templates"), "/")
		switch filepath.Ext(rel) {
		case ".json5", ".json", ".ts", ".ets":
			data = []byte(strings.NewReplacer(
				"{{name}}", "NSGo",
				"{{bundle}}", bundle,
				"{{version}}", "0.0.0",
				"{{code}}", "1",
			).Replace(string(data)))
		}
		out := filepath.Join(dst, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		return os.WriteFile(out, data, 0o644)
	})
	if err != nil {
		return err
	}
	web := filepath.Join(dst, "entry", "src", "main", "resources", "rawfile", "web")
	if err := os.MkdirAll(web, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(web, "index.html"), []byte("<!doctype html>\n"), 0o644)
}

// unzipHAP writes the host files of a HAP to dst, leaving out what the
// committed host leaves out.
func unzipHAP(src, dst string) error {
	zr, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer zr.Close()
	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, "/") || f.Name == "ets/sourceMaps.map" ||
			strings.HasPrefix(f.Name, "resources/rawfile/") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return err
		}
		out := filepath.Join(dst, filepath.FromSlash(f.Name))
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(out, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}
