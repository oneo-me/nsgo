package main

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

// An Apple Watch app is a SwiftUI app: watchOS has no public UIKit, so
// its views are SwiftUI's, which the host in watch/Host.swift shows as
// internal/platform/watchos tells it. The tool compiles the host with
// Xcode's swiftc and links it into the app's Go executable, whose main
// runs the host's app as an iPhone app's runs UIKit's. Go builds arm64
// alone, not arm64_32, so the app runs on the watches watchOS 26 runs as
// arm64: Series 9, Ultra 2 and later (decided by the user, 2026-11-09).

//go:embed watch/Host.swift
var watchHost []byte

var watchOS = &appleOS{name: "watchOS", target: "watchos", sdk: "watchos", minimum: "26.0", device: "Apple Watch",
	families: []int{4}, idioms: []string{"watch"}, platform: "Watch",
	deployment: "WATCHOS_DEPLOYMENT_TARGET", plist: watchosPlist}

// buildWatchApp writes <dir>/<name>.app for watchOS's simulator or
// devices: the app's Go code with the host linked in, its resources, icon
// and Info.plist. Device builds are left unsigned for the developer's own
// identity, as iOS's are.
func buildWatchApp(ctx context.Context, p *project, dir string, simulator bool) (string, error) {
	sys := watchOS
	env, err := iosEnv(ctx, sys, simulator)
	if err != nil {
		return "", err
	}
	host, err := compileWatchHost(ctx, simulator)
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(filepath.Dir(host))
	sdkName := sys.sdkName(simulator)
	sdk, err := output(ctx, "xcrun", "--sdk", sdkName, "--show-sdk-path")
	if err != nil {
		return "", err
	}
	app := filepath.Join(dir, p.Name+".app")
	if err := os.RemoveAll(app); err != nil {
		return "", err
	}
	// Go's runtime objects target a newer OS than the minimum; the
	// linker's warning about it is noise. The host's object names the
	// Swift libraries it needs, which the system holds.
	link := fmt.Sprintf("-extldflags '-Wl,-w %s -L%s'", host, filepath.Join(sdk, "usr/lib/swift"))
	if err := goBuildWith(ctx, p, filepath.Join(app, p.Name), env, buildExtra{tags: []string{"nsgowatch"}}, link); err != nil {
		return "", err
	}
	stage(ctx, "bundling the app")
	if err := copyMobileResources(p, app); err != nil {
		return "", err
	}
	icon, err := writeWatchIcon(ctx, p, app, simulator)
	if err != nil {
		return "", err
	}
	names, err := writeAppleNames(p, app)
	if err != nil {
		return "", err
	}
	if p.iosExtension() {
		detailIn(ctx, "no widgets or Live Activities on watchOS: they are left out")
	}
	extra := icon + "\n" + names + "\n\t<key>CFBundleSupportedPlatforms</key><array><string>" + sys.platformName(simulator) + "</string></array>"
	if err := writePlist(filepath.Join(app, "Info.plist"), p, sys.plist, extra); err != nil {
		return "", err
	}
	if !simulator {
		return app, nil
	}
	stage(ctx, "signing")
	return app, run(ctx, "codesign", "--force", "--sign", "-", app)
}

// compileWatchHost compiles the host into an object in a directory of its
// own, which the caller removes, and returns the object's path.
func compileWatchHost(ctx context.Context, simulator bool) (string, error) {
	stage(ctx, "compiling the watch host")
	tmp, err := os.MkdirTemp("", "nsgo-watch-")
	if err != nil {
		return "", err
	}
	src, obj := filepath.Join(tmp, "Host.swift"), filepath.Join(tmp, "Host.o")
	if err := os.WriteFile(src, watchHost, 0o644); err != nil {
		os.RemoveAll(tmp)
		return "", err
	}
	sdkName := watchOS.sdkName(simulator)
	sdk, err := output(ctx, "xcrun", "--sdk", sdkName, "--show-sdk-path")
	if err != nil {
		os.RemoveAll(tmp)
		return "", fmt.Errorf("the watchOS SDK needs Xcode: %w", err)
	}
	if err := run(ctx, "xcrun", "--sdk", sdkName, "swiftc", "-c", "-target", watchOS.triple(simulator), "-sdk", sdk,
		"-O", "-parse-as-library", "-module-name", "NSGoWatch", src, "-o", obj); err != nil {
		os.RemoveAll(tmp)
		return "", err
	}
	return obj, nil
}

// writeWatchIcon compiles the app's icon into a watch bundle: Icon.WatchOS
// when the project has one, else the Icon Composer .icon macOS and iOS
// use, else the PNG through an asset catalog, which watchOS masks to a
// circle.
func writeWatchIcon(ctx context.Context, p *project, app string, simulator bool) (string, error) {
	sys := watchOS
	sdk := sys.sdkName(simulator)
	icon := p.Icon.WatchOS
	if icon == "" {
		if filepath.Ext(p.Icon.Apple) == ".icon" {
			icon = p.Icon.Apple
		} else {
			icon = p.Icon.PNG
		}
	}
	switch filepath.Ext(icon) {
	case "":
		return "", nil
	case ".icon":
		return compileIcon(ctx, icon, app, sdk, sys.minimum, sys.idioms)
	}
	img, err := readSquareIcon(icon)
	if err != nil {
		return "", err
	}
	tmp, err := os.MkdirTemp("", "nsgo-icon-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	catalog := filepath.Join(tmp, "Assets.xcassets")
	if err := writeDir(filepath.Join(catalog, "AppIcon.appiconset"), map[string][]byte{
		"Icon.png":      scaledPNG(img, 1024),
		"Contents.json": []byte(`{"images":[{"filename":"Icon.png","idiom":"universal","platform":"watchos","size":"1024x1024"}],` + catalogInfo + `}`),
	}); err != nil {
		return "", err
	}
	return compileIcon(ctx, catalog, app, sdk, sys.minimum, sys.idioms)
}

// watchosPlist is a watch-only app's: watchOS shows it dark whatever the
// app asks for.
var watchosPlist = template.Must(template.New("watchos").Parse(plistHead + `	<key>MinimumOSVersion</key><string>26.0</string>
	<key>UIDeviceFamily</key><array><integer>4</integer></array>
	<key>WKApplication</key><true/>
	<key>WKWatchOnly</key><true/>
</dict>
</plist>
`))
