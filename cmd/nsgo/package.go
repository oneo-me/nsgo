package main

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/nsgo-dev/nsgo/cmd/nsgo/harmony"
)

// writeIPA packs app, a built .app, as the IPA at dst: its bundle under
// Payload/, as App Store Connect and installers take it.
func writeIPA(app, dst string) error {
	return writeZip(app, dst, "Payload/"+filepath.Base(app))
}

// writeZip packs dir as the zip archive at dst, its files under prefix, or
// at the top without one.
func writeZip(dir, dst, prefix string) error {
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	err = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		name := path.Join(prefix, filepath.ToSlash(rel))
		if name == "." {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		h, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		h.Name = name
		if d.IsDir() {
			h.Name += "/"
			_, err = zw.CreateHeader(h)
			return err
		}
		h.Method = zip.Deflate
		w, err := zw.CreateHeader(h)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	})
	if err != nil {
		return err
	}
	if err := zw.Close(); err != nil {
		return err
	}
	return f.Close()
}

// hashedName names data by its content, as a published site names every
// file but its page and manifest, so hosts cache them for good:
// app.wasm becomes app.<10 hex digits>.wasm.
func hashedName(name string, data []byte) string {
	sum := sha256.Sum256(data)
	ext := path.Ext(name)
	return strings.TrimSuffix(name, ext) + "." + hex.EncodeToString(sum[:])[:10] + ext
}

// hashed matches the names hashedName gives: a content hash before the
// extension, or at the end of a name without one.
var hashed = regexp.MustCompile(`\.[0-9a-f]{10}(\.[A-Za-z0-9]+)?(/|$)`)

// webManifest is a site's manifest.webmanifest, which lets browsers
// install it as an app.
type webManifest struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	ShortName string `json:"short_name"`
	// NameLocalized and ShortNameLocalized name the app in each of its
	// other languages, by BCP 47 tag.
	NameLocalized      map[string]string `json:"name_localized,omitempty"`
	ShortNameLocalized map[string]string `json:"short_name_localized,omitempty"`
	StartURL           string            `json:"start_url"`
	Display            string            `json:"display"`
	BackgroundColor    string            `json:"background_color,omitempty"`
	ThemeColor         string            `json:"theme_color,omitempty"`
	Icons              []webIcon         `json:"icons,omitempty"`
}

type webIcon struct {
	Src   string `json:"src"`
	Sizes string `json:"sizes"`
	Type  string `json:"type"`
}

// appearanceColor is the color a site's page and manifest start in.
func appearanceColor(appearance string) string {
	switch appearance {
	case "dark":
		return "#000000"
	case "light":
		return "#ffffff"
	}
	return ""
}

// writeWebManifest writes dir/manifest.webmanifest for p with its icons.
func writeWebManifest(p *project, dir string, icons []webIcon) error {
	color := appearanceColor(p.Appearance)
	m := webManifest{ID: p.ID, Name: p.Name, ShortName: p.Name, StartURL: ".", Display: "standalone",
		BackgroundColor: color, ThemeColor: color, Icons: icons}
	n, err := p.appNames()
	if err != nil {
		return err
	}
	if n != nil {
		m.Name, m.ShortName = n.names[n.source], n.names[n.source]
		for _, lang := range n.languages()[1:] {
			if m.NameLocalized == nil {
				m.NameLocalized, m.ShortNameLocalized = map[string]string{}, map[string]string{}
			}
			m.NameLocalized[lang], m.ShortNameLocalized[lang] = n.names[lang], n.names[lang]
		}
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "manifest.webmanifest"), append(data, '\n'), 0o644)
}

// copyHashedResources copies the project's resources beside the page in
// dir, each file named by its content, and returns the name each got by
// its path in the project.
func copyHashedResources(p *project, dir string) (map[string]string, error) {
	names := make(map[string]string)
	err := walkResources(p, func(rel string, data []byte) error {
		name := hashedName(rel, data)
		dst := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return err
		}
		names[rel] = name
		return nil
	})
	return names, err
}

// serveSite serves a published site from dir on this machine, with the
// cache headers a CDN should send: named files for good, the page and
// manifest checked each time.
func serveSite(ctx context.Context, dir string) error {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	files := http.FileServer(http.Dir(dir))
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hashed.MatchString(r.URL.Path) {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		files.ServeHTTP(w, r)
	})}
	go func() {
		<-ctx.Done()
		srv.Close()
	}()
	url := fmt.Sprintf("http://%s/", ln.Addr())
	note("%s (interrupt to stop)", link(url))
	name, args := openCommand(url)
	exec.Command(name, args...).Start()
	if err := srv.Serve(ln); err != http.ErrServerClosed {
		return err
	}
	return nil
}

// runnable reports whether this machine runs target's package with -run.
func runnable(target string) bool {
	arch := goArchName(runtime.GOARCH)
	switch target {
	case "web", "android_arm64", "android_x64":
		return true
	case "harmonyos":
		_, err := exec.LookPath("hdc")
		return err == nil
	case "ios", "ios_simulator", "tvos", "tvos_simulator", "watchos", "watchos_simulator":
		return runtime.GOOS == "darwin"
	case "macos_" + arch:
		return runtime.GOOS == "darwin"
	case "linux_" + arch:
		return runtime.GOOS == "linux"
	case "windows_" + arch:
		return runtime.GOOS == "windows"
	}
	if mp, ok := miniProgramPlatformOf(target); ok {
		return canOpenMiniProgram(mp)
	}
	return false
}

// runHarmony installs the HAP on the device hdc reaches, -device or the only
// one connected, and starts its ability, the bundle the HAP names. flags are
// the engine's launch flags, passed as the ability's Want parameters so a
// second launch can change them without rebuilding the HAP.
func runHarmony(ctx context.Context, p *project, dir string, flags []string) error {
	hap := filepath.Join(dir, p.Name+".hap")
	bundle, err := harmony.BundleOf(hap)
	if err != nil {
		return err
	}
	device := []string(nil)
	if p.Build.device != "" {
		device = []string{"-t", p.Build.device}
	}
	if err := run(ctx, "hdc", append(append([]string{}, device...), "install", "-r", hap)...); err != nil {
		return err
	}
	start := []string{"shell", "aa", "start", "-a", "EntryAbility", "-b", bundle}
	for _, f := range flags {
		name, value, hasValue := strings.Cut(strings.TrimPrefix(f, "--"), "=")
		if hasValue {
			start = append(start, "--ps", name, value)
		} else {
			start = append(start, "--pb", name, "true")
		}
	}
	return run(ctx, "hdc", append(append([]string{}, device...), start...)...)
}

// runPackage starts the package built for target in dir, as -run asks:
// the app opens here, in the simulator or on the device -device names
// or the only one connected, or the site is served. An iPhone's package
// is signed again by Xcode, as nsgo dev signs it, since devices run only
// development builds.
func runPackage(ctx context.Context, p *project, target, dir string) error {
	event("run %s", target)
	switch {
	case target == "harmonyos":
		return runHarmony(ctx, p, dir, harmonyFlags())
	case target == "web":
		return serveSite(ctx, dir)
	case strings.HasPrefix(target, "miniprogram_"):
		mp, _ := miniProgramPlatformOf(target)
		return openMiniProgram(ctx, mp, dir)
	case strings.HasPrefix(target, "macos_"):
		return run(ctx, "open", "-n", filepath.Join(dir, p.Name+".app"))
	case target == "ios" || target == "tvos" || target == "watchos":
		sys, _, _ := appleOSOf(target)
		device, err := selectIOSDevice(ctx, sys, p.Build.device)
		if err != nil {
			return err
		}
		team, err := iosTeam(ctx, p.Build.team)
		if err != nil {
			return err
		}
		if err := installOnIOSDevice(ctx, p, sys, filepath.Join(dir, p.Name+".app"), p.cache("run", target), team, device); err != nil {
			return err
		}
		return run(ctx, "xcrun", "devicectl", "device", "process", "launch", "--terminate-existing", "--device", device.UDID, p.ID)
	case target == "ios_simulator" || target == "tvos_simulator" || target == "watchos_simulator":
		sys, _, _ := appleOSOf(target)
		udid, err := bootSimulator(ctx, sys, p.Build.device)
		if err != nil {
			return err
		}
		if err := run(ctx, "xcrun", "simctl", "install", udid, filepath.Join(dir, p.Name+".app")); err != nil {
			return err
		}
		return run(ctx, "xcrun", "simctl", "launch", "--terminate-running-process", udid, p.ID)
	case strings.HasPrefix(target, "android_"):
		d, err := adbDevice(ctx, p.Build.device)
		if err != nil {
			return err
		}
		if err := d.run(ctx, "install", "-r", filepath.Join(dir, p.Name+".apk")); err != nil {
			return err
		}
		return d.run(ctx, "shell", "am", "start", "-S", "-n", p.ID+androidComponent)
	case strings.HasPrefix(target, "linux_"):
		return startDetached(filepath.Join(dir, p.Name))
	case strings.HasPrefix(target, "windows_"):
		return startDetached(filepath.Join(dir, p.Name+".exe"))
	}
	return fmt.Errorf("-run cannot run %s here", target)
}

// startDetached starts bin, which keeps running after nsgo exits.
func startDetached(bin string) error {
	cmd := exec.Command(bin)
	cmd.Dir = filepath.Dir(bin)
	return cmd.Start()
}
