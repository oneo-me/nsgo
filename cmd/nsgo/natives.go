package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Native libraries the app's packages ship: base/terminal's
// libghostty-vt, which its library package holds by GOOS-GOARCH and the
// tool puts into the apps that link it, signed with them.

// ghosttyPackage is the package holding libghostty-vt's builds.
const ghosttyPackage = "github.com/oneo-me/nsgo/base/terminal/internal/library"

// ghosttyFramework is libghostty-vt as an iOS app's framework, where the
// terminal looks for it.
const ghosttyFramework = "GhosttyVT.framework"

// linkedDir returns the directory of package pkg when the app's build for
// env links it, else "".
func linkedDir(ctx context.Context, p *project, env []string, pkg string) (string, error) {
	args := []string{"list", "-deps", "-f", fmt.Sprintf(`{{if eq .ImportPath %q}}{{.Dir}}{{end}}`, pkg)}
	if p.testBinary {
		args = append(args, "-test", "-tags", "nsgotest")
	}
	cmd := exec.CommandContext(ctx, "go", append(args, ".")...)
	cmd.Dir = p.Dir
	cmd.Env = append(os.Environ(), env...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("go list: %w\n%s", err, stderr.Bytes())
	}
	for _, line := range strings.Split(string(out), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line, nil
		}
	}
	return "", nil
}

// addMacLibraries puts the native libraries the app links for arch into
// its Contents/Frameworks, signed ad hoc as the app is; the app's own
// signature then seals them.
func addMacLibraries(ctx context.Context, p *project, app, arch string) error {
	dir, err := linkedDir(ctx, p, []string{"GOOS=darwin", "GOARCH=" + arch, "CGO_ENABLED=0"}, ghosttyPackage)
	if err != nil || dir == "" {
		return err
	}
	dst := filepath.Join(app, "Contents", "Frameworks", "libghostty-vt.dylib")
	if err := copyLibrary(filepath.Join(dir, "lib", "darwin-"+arch, "libghostty-vt.dylib"), dst); err != nil {
		return err
	}
	return run(ctx, "codesign", "--force", "--sign", "-", dst)
}

// addIOSFrameworks puts the native libraries the app links for sys into its
// Frameworks as frameworks: signed ad hoc for the simulator, left for the
// app's signing on devices (signFrameworks).
func addIOSFrameworks(ctx context.Context, p *project, sys *appleOS, app string, env []string, simulator bool) error {
	dir, err := linkedDir(ctx, p, env, ghosttyPackage)
	if err != nil || dir == "" {
		return err
	}
	build := sys.target + "-arm64"
	if simulator {
		build = sys.target + "simulator-arm64"
	}
	lib := filepath.Join(dir, "lib", build, "libghostty-vt.dylib")
	if _, err := os.Stat(lib); err != nil && sys != iOS {
		// tools/libghostty builds no tvOS library yet; the terminal then
		// reports it missing.
		detailIn(ctx, "no libghostty-vt for %s: the terminal will not start", sys.name)
		return nil
	}
	fw := filepath.Join(app, "Frameworks", ghosttyFramework)
	binary := filepath.Join(fw, "GhosttyVT")
	if err := copyLibrary(lib, binary); err != nil {
		return err
	}
	if err := run(ctx, "install_name_tool", "-id", "@rpath/"+ghosttyFramework+"/GhosttyVT", binary); err != nil {
		return err
	}
	plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleExecutable</key><string>GhosttyVT</string>
	<key>CFBundleIdentifier</key><string>` + p.ID + `.ghosttyvt</string>
	<key>CFBundleName</key><string>GhosttyVT</string>
	<key>CFBundlePackageType</key><string>FMWK</string>
	<key>CFBundleShortVersionString</key><string>1.0</string>
	<key>CFBundleVersion</key><string>1</string>
	<key>CFBundleSupportedPlatforms</key><array><string>` + sys.platformName(simulator) + `</string></array>
	<key>MinimumOSVersion</key><string>` + sys.minimum + `</string>
</dict>
</plist>
`
	if err := os.WriteFile(filepath.Join(fw, "Info.plist"), []byte(plist), 0o644); err != nil {
		return err
	}
	if simulator {
		return run(ctx, "codesign", "--force", "--sign", "-", fw)
	}
	return nil
}

// signFrameworks signs the frameworks of an iOS app with identity, before
// the app is, which seals them.
func signFrameworks(ctx context.Context, app, identity string) error {
	fws, _ := filepath.Glob(filepath.Join(app, "Frameworks", "*.framework"))
	for _, fw := range fws {
		if err := run(ctx, "codesign", "--force", "--sign", identity, fw); err != nil {
			return err
		}
	}
	return nil
}

// copyLibrary copies a native library to dst, making its directory.
func copyLibrary(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o755)
}
