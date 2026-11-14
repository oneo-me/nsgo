// Package harmony assembles the HAP the harmonyos target builds: an ArkTS
// host whose Web serves the app's web build from the HAP's rawfile
// directory, with module.json and pack.info named for the app.
// docs/architecture/harmonyos.md holds the plan and what is not built.
//
// A committed host (host/) is compiled with a fixed placeholder bundle, and
// Package replaces that placeholder byte for byte with an equally long name
// built from the app's id: the bytecode's length prefixes stay valid, so one
// host serves every app without an OpenHarmony SDK at build time. When the
// build machine has the SDK, CompileHost builds a host for the app's own
// bundle instead, and PackageWithHost assembles from it, keeping the id.
package harmony

import (
	"archive/zip"
	"bytes"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

//go:embed host
var host embed.FS

// templates are the host's sources, which CompileHost builds with the SDK
// when the machine has one.
//
//go:embed templates
var templates embed.FS

//go:embed boot.html
var bootPage []byte

// Boot returns the page that starts an app's module in the host's Web view:
// it loads wasm_exec.js and app.wasm from the same directory, and the ArkTS
// side it talks to is nsgoHarmony.
func Boot() []byte { return bootPage }

// BundleOf returns the bundle a HAP names for its app, as -run needs it to
// start: Package pads it without the SDK and keeps the id with it.
func BundleOf(hap string) (string, error) {
	zr, err := zip.OpenReader(hap)
	if err != nil {
		return "", err
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.Name != "module.json" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return "", err
		}
		if m := reAppBundle.FindSubmatch(data); len(m) == 2 {
			return string(m[1]), nil
		}
		return "", fmt.Errorf("%s names no bundle", hap)
	}
	return "", fmt.Errorf("%s has no module.json", hap)
}

var (
	reAppBundle = regexp.MustCompile("\"bundleName\":\"([^\"]*)\"")
	reName      = regexp.MustCompile("\"versionName\":\"[^\"]*\"")
	reCode      = regexp.MustCompile("\"versionCode\":[0-9]+")
	reLabel     = regexp.MustCompile("\"label\":\"[^\"]*\"")
	rePackVer   = regexp.MustCompile("\"version\":\\{\"code\":[0-9]+,\"name\":\"[^\"]*\"\\}")
)

// hostBundle is the fixed placeholder the committed ArkTS bytecode was
// compiled with (host/build.sh). It is long enough for any app id, and
// Package replaces it byte for byte so the bytecode's length prefixes and
// offsets stay valid.
const hostBundle = "com.nsgo.host.placeholder000000000000000000000000000000000000000"

// BundleName returns an equally long, legal bundle name for id: the id,
// a fixed marker that keeps different ids apart, then zero padding. The
// result is a valid dot-separated name that never ends in a dot.
func BundleName(id string) (string, error) {
	const marker = ".nsgohost"
	if len(id)+len(marker) > len(hostBundle) {
		return "", fmt.Errorf("bundle name %q is too long for the harmonyos host (max %d bytes)", id, len(hostBundle)-len(marker))
	}
	return id + marker + strings.Repeat("0", len(hostBundle)-len(id)-len(marker)), nil
}

// Package writes the HAP for an app into dir from the committed host and
// the web build in site, and returns its path. The committed host's
// bytecode names each module with the placeholder bundle; Package swaps it
// for an equally long name derived from the app's id, so one host serves
// every app without the SDK.
func Package(dir, site, name, bundle, version string, code int) (string, error) {
	padded, err := BundleName(bundle)
	if err != nil {
		return "", err
	}
	return packageFrom(dir, site, name, padded, version, code, host, "host", true)
}

// PackageWithHost writes the HAP for an app into dir from a host built for
// the app's own bundle, as CompileHost builds one with the SDK: its
// bytecode already names the modules with the app's bundle, so only the
// app's version and label change and the installed bundle keeps the id.
func PackageWithHost(dir, site, hostDir, name, bundle, version string, code int) (string, error) {
	return packageFrom(dir, site, name, bundle, version, code, os.DirFS(hostDir), ".", false)
}

// packageFrom writes the HAP from a host tree and the web build in site.
// When swap is set, the placeholder bundle is replaced by bundle in every
// host file, the bytecode included; otherwise the host already carries the
// app's bundle and only the app's values are patched.
func packageFrom(dir, site, name, bundle, version string, code int, hfs fs.FS, root string, swap bool) (string, error) {
	hostNames, err := walk(hfs, root)
	if err != nil {
		return "", err
	}
	siteNames, err := walk(os.DirFS(site), ".")
	if err != nil {
		return "", err
	}
	sortByEntry(hostNames)

	out := filepath.Join(dir, name+".hap")
	f, err := os.Create(out)
	if err != nil {
		return "", err
	}
	zw := zip.NewWriter(f)
	add := func(name string, data []byte) error {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store})
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	}
	for _, n := range hostNames {
		if n == "build.sh" {
			continue
		}
		data, err := fs.ReadFile(hfs, path.Join(root, n))
		if err != nil {
			return "", err
		}
		if swap {
			// The placeholder appears in the compiled bytecode too, so swap
			// it as bytes: both names are the same length, which keeps the
			// bytecode's string lengths and offsets valid.
			data = bytes.ReplaceAll(data, []byte(hostBundle), []byte(bundle))
		}
		switch n {
		case "module.json":
			s := string(data)
			s = reName.ReplaceAllString(s, "\"versionName\":\""+version+"\"")
			s = reCode.ReplaceAllString(s, "\"versionCode\":"+strconv.Itoa(code))
			s = reLabel.ReplaceAllString(s, "\"label\":\""+name+"\"")
			data = []byte(s)
		case "pack.info":
			s := string(data)
			s = rePackVer.ReplaceAllString(s,
				"\"version\":{\"code\":"+strconv.Itoa(code)+",\"name\":\""+version+"\"}")
			data = []byte(s)
		}
		if err := add(n, data); err != nil {
			return "", err
		}
	}
	for _, n := range siteNames {
		data, err := os.ReadFile(filepath.Join(site, filepath.FromSlash(n)))
		if err != nil {
			return "", err
		}
		if err := add("resources/rawfile/web/"+n, data); err != nil {
			return "", err
		}
	}
	if err := zw.Close(); err != nil {
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	return out, nil
}

// walk lists the files under root, with paths relative to it.
func walk(fsys fs.FS, root string) ([]string, error) {
	var out []string
	err := fs.WalkDir(fsys, root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		out = append(out, strings.TrimPrefix(strings.TrimPrefix(p, root), "/"))
		return nil
	})
	sort.Strings(out)
	return out, err
}

// sortByEntry puts module.json and resources.index first and pack.info
// last, as hvigor's packages have them.
func sortByEntry(names []string) {
	order := func(n string) int {
		switch n {
		case "module.json":
			return 0
		case "resources.index":
			return 1
		case "pack.info":
			return 3
		}
		return 2
	}
	sort.SliceStable(names, func(i, j int) bool {
		oi, oj := order(names[i]), order(names[j])
		if oi != oj {
			return oi < oj
		}
		return names[i] < names[j]
	})
}
