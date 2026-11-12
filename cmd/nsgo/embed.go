package main

import (
	"archive/zip"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"

	androidruntime "github.com/nsgo-dev/nsgo/internal/platform/android/runtime"
)

// nsgo build -embed packages the app for a host app of its own instead of
// as an app: for iOS an XCFramework of a static library per slice with
// NSGo.h, for Android a library (embedAAR). Desktop hosts are Go
// programs, which embed apps with nsgo.NewView.

// embedTargets are the targets -embed builds for.
var embedTargets = []string{"ios", "ios_simulator", "android_arm64", "android_x64"}

// embedExport is the file the tool adds to the app's main package for an
// iOS framework: the one C function the host calls. It is cgo's because
// only cgo exports Go functions; it waits for the Go runtime to start,
// which a static library does on a thread of its own.
const embedExport = `// Added by nsgo build -embed: the framework's NSGoStart.
package main

import "C"

import nsgoembed "github.com/nsgo-dev/nsgo"

//export NSGoStart
func NSGoStart() { nsgoembed.StartEmbedded(main) }
`

// embedHeader declares what Go registers with the Objective-C runtime:
// objc_runtime_visible classes have no symbols to link, so the compiler
// looks them up by name.
const embedHeader = `// NSGo.h: the NSGo app {{name}} in an iOS app's own views, made by nsgo build -embed.
#import <UIKit/UIKit.h>

NS_ASSUME_NONNULL_BEGIN

/// Starts the app's Go code. Call it once on the main thread, as the app
/// launches and before the first NSGoView: it returns once the app is
/// ready.
void NSGoStart(void);

/// A view of the host's own that the app shows with a Native element of
/// the name it was registered under (NSGoView.register(component:factory:)).
@protocol NSGoComponent <NSObject>
/// The view, which fills the element's box.
@property (nonatomic, readonly) UIView *view;
/// Sets one of the element's props: JSON values as Foundation's objects,
/// nil when the prop goes.
- (void)setProp:(NSString *)name value:(nullable id)value;
@optional
/// Sets a prop the app animates, on the frames it changes; without it
/// setProp receives the number.
- (void)setAnimatedNumber:(NSString *)name value:(double)value;
/// Sets a color prop the app animates, as 0xAARRGGBB, on the frames it
/// changes; without it setProp receives the number.
- (void)setAnimatedColor:(NSString *)name argb:(uint32_t)argb;
/// Runs a command the app sent (core.Ref.Command).
- (void)command:(NSString *)name args:(nullable id)args;
/// Releases what the component holds, once its element or app is gone.
- (void)destroy;
@end

/// How a component talks back to its element: events reach its OnEvent,
/// and a size it asks for is the box's where the app's style leaves it
/// open.
__attribute__((objc_runtime_visible))
@interface NSGoComponentContext : NSObject
- (void)emit:(NSString *)name data:(nullable id)data NS_SWIFT_NAME(emit(_:_:));
- (void)setSize:(CGSize)size;
@end

/// The app in a view. Its app starts once the view is in a window, from
/// the settings made by then, and stops when the view is deallocated or
/// stopped. Several views run at once, each an app of its own. Use it on
/// the main thread; its handlers and components run there.
__attribute__((objc_runtime_visible))
@interface NSGoView : UIView
/// The launch parameters the app reads (core.Scope.HostParams), set
/// before it starts.
@property (nonatomic, copy, nullable) NSDictionary<NSString *, id> *params;
/// Where the app keeps its data, its storage included, set before it
/// starts; Application Support/<the app's ID> when nil.
@property (nonatomic, copy, nullable) NSString *dataDirectory;
/// A BCP 47 locale the app formats for in place of the system's; nil
/// follows the system. overrideUserInterfaceStyle sets the appearance.
@property (nonatomic, copy, nullable) NSString *localeIdentifier;
/// Sets a text size multiplier in place of the system's; 0 follows the
/// system.
- (void)setFontScale:(CGFloat)scale;
/// The values the host themes the app with (core.Env.Token), CSS colors
/// and other strings: tokens in both appearances, darkTokens over them
/// while the app shows dark.
@property (nonatomic, copy, nullable) NSDictionary<NSString *, NSString *> *tokens;
@property (nonatomic, copy, nullable) NSDictionary<NSString *, NSString *> *darkTokens;
/// Hears each failure the app does not catch.
@property (nonatomic, copy, nullable) void (^onError)(NSString *message);
/// Sees the URLs the app opens before the system: YES takes one.
@property (nonatomic, copy, nullable) BOOL (^onOpenURL)(NSURL *url);
/// The status bar the app asks for, for the host's view controller to
/// return from its own.
@property (nonatomic, readonly) UIStatusBarStyle preferredStatusBarStyle;
@property (nonatomic, readonly) BOOL prefersStatusBarHidden;

/// Sends the app an event (core.Scope.OnHostEvent), kept until it has a
/// listener.
- (void)send:(NSString *)name data:(nullable id)data NS_SWIFT_NAME(send(_:_:));
/// Answers the app's calls and posts of name (core.Scope.CallHost):
/// reply once, from any thread, with a result or an error message.
- (void)handle:(NSString *)name
          with:(void (^)(id _Nullable data, void (^reply)(id _Nullable result, NSString *_Nullable error)))handler
    NS_SWIFT_NAME(handle(_:_:));
- (void)removeHandler:(NSString *)name;
/// Registers a component of the host's for this view's app, which shows
/// it with a Native element of name.
- (void)registerComponent:(NSString *)name
                  factory:(id<NSGoComponent> (^)(NSGoComponentContext *context))factory
    NS_SWIFT_NAME(register(component:factory:));
/// Stops the app now.
- (void)stop;
@end

NS_ASSUME_NONNULL_END
`

// embedOnly keeps the targets -embed builds for, the iOS framework once.
func embedOnly(targets []string) ([]string, error) {
	var out []string
	for _, t := range targets {
		if t == "ios_simulator" {
			t = "ios"
		}
		if slices.Contains(embedTargets, t) && !slices.Contains(out, t) {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("-embed builds for %s; desktop hosts embed apps with nsgo.NewView", strings.Join(embedTargets, ", "))
	}
	return out, nil
}

// embedModule names the framework's module, which Swift imports: the
// app's name in letters and digits.
func embedModule(p *project) string {
	name := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return -1
	}, p.Name)
	if name == "" || unicode.IsDigit(rune(name[0])) {
		name = "NSGoApp" + name
	}
	return name
}

// iosSlice is one platform of the iOS framework: its SDK, GOOS, the
// architectures it holds and their clang targets.
type iosSlice struct {
	sdk, goos string
	archs     []string // Go's
	target    string   // clang's, with %s for the architecture
	catalyst  bool
}

// iosSlices are the framework's platforms: devices, the simulator on
// Apple silicon and Intel Macs, and Mac Catalyst, which runs on macOS:
// its Go code is GOOS=darwin built with the ios tag, so NSGo's iOS
// backend serves it, as gomobile builds it.
var iosSlices = []iosSlice{
	{sdk: "iphoneos", goos: "ios", archs: []string{"arm64"}, target: "%s-apple-ios15.0"},
	{sdk: "iphonesimulator", goos: "ios", archs: []string{"arm64", "amd64"}, target: "%s-apple-ios15.0-simulator"},
	{sdk: "macosx", goos: "darwin", archs: []string{"arm64", "amd64"}, target: "%s-apple-ios15.0-macabi", catalyst: true},
}

// clangArch is clang's name of Go's architecture.
func clangArch(goarch string) string {
	if goarch == "amd64" {
		return "x86_64"
	}
	return goarch
}

// iosLinked are what Go's runtime and net link.
var iosLinked = []string{"UIKit", "Foundation", "CoreFoundation", "Security"}

// buildIOSFramework writes <dir>/<module>.xcframework, the app for
// devices, the simulator (arm64 and x86_64) and Mac Catalyst (arm64 and
// x86_64) with NSGo.h, and <dir>/<name>Resources, the app's resources,
// which the host adds to its bundle as a folder. Each platform is a
// static library, or with -dynamic a dynamic framework the host embeds:
// iOS forbids Go's c-shared, so the static library is linked into one,
// which exports only NSGoStart.
func buildIOSFramework(ctx context.Context, p *project, dir string) error {
	tmp, err := os.MkdirTemp("", "nsgo-embed-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	export := filepath.Join(tmp, "nsgo_embed.go")
	if err := os.WriteFile(export, []byte(embedExport), 0o644); err != nil {
		return err
	}
	module := embedModule(p)
	header := strings.ReplaceAll(embedHeader, "{{name}}", p.Name)
	args := []string{"-create-xcframework"}
	for _, sl := range iosSlices {
		sdk, err := output(ctx, "xcrun", "--sdk", sl.sdk, "--show-sdk-path")
		if err != nil {
			return fmt.Errorf("the iOS SDK needs Xcode: %w", err)
		}
		clang, err := output(ctx, "xcrun", "--sdk", sl.sdk, "-f", "clang")
		if err != nil {
			return err
		}
		slice := filepath.Join(tmp, sl.sdk)
		var libs, dylibs []string
		for _, arch := range sl.archs {
			flags := "-isysroot " + sdk + " -target " + fmt.Sprintf(sl.target, clangArch(arch))
			if sl.catalyst {
				// UIKit for Mac Catalyst lives in the macOS SDK's iOSSupport.
				support := filepath.Join(sdk, "System/iOSSupport")
				flags += " -iframework " + filepath.Join(support, "System/Library/Frameworks") +
					" -isystem " + filepath.Join(support, "usr/include")
			}
			lib := filepath.Join(slice, arch, "lib"+module+".a")
			env := []string{"GOOS=" + sl.goos, "GOARCH=" + arch, "CGO_ENABLED=1",
				"CC=" + clang + " " + flags, "CGO_CFLAGS=" + flags, "CGO_LDFLAGS=" + flags}
			tags, ldflags := []string{"nsgoembed"}, []string(nil)
			if sl.catalyst {
				// AppKit's threads drop the signal stack Go's preemption
				// signal needs, so goroutines are preempted only where
				// they yield, as GODEBUG=asyncpreemptoff=1 has it.
				tags, ldflags = []string{"ios", "nsgoembed"}, []string{"-X=runtime.godebugDefault=asyncpreemptoff=1"}
			}
			extra := buildExtra{flags: []string{"-buildmode=c-archive"}, tags: tags,
				files: map[string]string{"nsgo_embed_export.go": export}}
			if err := goBuildWith(ctx, p, lib, env, extra, ldflags...); err != nil {
				return err
			}
			// cgo's header declares Go's types too; the host needs only ours.
			os.Remove(strings.TrimSuffix(lib, ".a") + ".h")
			libs = append(libs, lib)
			if p.Build.dynamic {
				dylib := filepath.Join(slice, arch, module)
				install := "@rpath/" + module + ".framework/" + module
				if sl.catalyst {
					install = "@rpath/" + module + ".framework/Versions/A/" + module
				}
				link := append(strings.Fields(flags), "-dynamiclib", "-o", dylib, "-Wl,-all_load", lib,
					"-lresolv", "-install_name", install, "-Wl,-exported_symbol,_NSGoStart", "-Wl,-dead_strip")
				if sl.catalyst {
					link = append(link, "-F", filepath.Join(sdk, "System/iOSSupport/System/Library/Frameworks"))
				}
				for _, f := range iosLinked {
					link = append(link, "-framework", f)
				}
				if err := run(ctx, clang, link...); err != nil {
					return err
				}
				dylibs = append(dylibs, dylib)
			}
		}
		if !p.Build.dynamic {
			lib := filepath.Join(slice, "lib"+module+".a")
			if err := run(ctx, "lipo", append([]string{"-create", "-output", lib}, libs...)...); err != nil {
				return err
			}
			headers := filepath.Join(slice, "Headers")
			if err := writeIOSHeaders(headers, header, fmt.Sprintf("module %s {\n\theader \"NSGo.h\"\n%s\texport *\n}\n", module, iosLinks())); err != nil {
				return err
			}
			args = append(args, "-library", lib, "-headers", headers)
			continue
		}
		fw, err := writeIOSFramework(slice, module, p.ID, header, sl.catalyst)
		if err != nil {
			return err
		}
		binary := filepath.Join(fw, module)
		if sl.catalyst {
			binary = filepath.Join(fw, "Versions/A", module)
		}
		if err := run(ctx, "lipo", append([]string{"-create", "-output", binary}, dylibs...)...); err != nil {
			return err
		}
		args = append(args, "-framework", fw)
	}
	stage(ctx, "making the XCFramework")
	framework := filepath.Join(dir, module+".xcframework")
	os.RemoveAll(framework)
	if err := run(ctx, "xcodebuild", append(args, "-output", framework)...); err != nil {
		return err
	}
	resources := filepath.Join(dir, p.Name+"Resources")
	if err := os.MkdirAll(resources, 0o755); err != nil {
		return err
	}
	return copyResources(p, resources)
}

// iosLinks are a static library's module map's link lines: what Go's
// runtime and net link, autolinked by the module.
func iosLinks() string {
	var b strings.Builder
	for _, f := range iosLinked {
		fmt.Fprintf(&b, "\tlink framework %q\n", f)
	}
	b.WriteString("\tlink \"resolv\"\n")
	return b.String()
}

// writeIOSHeaders writes NSGo.h and modulemap into dir.
func writeIOSHeaders(dir, header, modulemap string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "NSGo.h"), []byte(header), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "module.modulemap"), []byte(modulemap), 0o644)
}

// writeIOSFramework makes the bundle of a dynamic framework in slice,
// its binary left to the caller: iOS's flat, Mac Catalyst's in macOS's
// versioned layout. It returns the bundle's path.
func writeIOSFramework(slice, module, id, header string, catalyst bool) (string, error) {
	fw := filepath.Join(slice, module+".framework")
	root := fw
	if catalyst {
		root = filepath.Join(fw, "Versions/A")
	}
	modulemap := fmt.Sprintf("framework module %s {\n\tumbrella header \"NSGo.h\"\n\texport *\n}\n", module)
	if err := writeIOSHeaders(filepath.Join(root, "Headers"), header, ""); err != nil {
		return "", err
	}
	os.Remove(filepath.Join(root, "Headers", "module.modulemap"))
	if err := os.MkdirAll(filepath.Join(root, "Modules"), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(root, "Modules", "module.modulemap"), []byte(modulemap), 0o644); err != nil {
		return "", err
	}
	plistDir := root
	if catalyst {
		plistDir = filepath.Join(root, "Resources")
	}
	if err := os.MkdirAll(plistDir, 0o755); err != nil {
		return "", err
	}
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleExecutable</key><string>%[1]s</string>
	<key>CFBundleIdentifier</key><string>%[2]s</string>
	<key>CFBundleName</key><string>%[1]s</string>
	<key>CFBundlePackageType</key><string>FMWK</string>
	<key>CFBundleShortVersionString</key><string>1.0</string>
	<key>CFBundleVersion</key><string>1</string>
	<key>MinimumOSVersion</key><string>15.0</string>
</dict>
</plist>
`, module, id+".framework")
	if err := os.WriteFile(filepath.Join(plistDir, "Info.plist"), []byte(plist), 0o644); err != nil {
		return "", err
	}
	if catalyst {
		for link, to := range map[string]string{"Versions/Current": "A", module: "Versions/Current/" + module,
			"Headers": "Versions/Current/Headers", "Modules": "Versions/Current/Modules", "Resources": "Versions/Current/Resources"} {
			if err := os.Symlink(to, filepath.Join(fw, link)); err != nil {
				return "", err
			}
		}
	}
	return fw, nil
}

// aarManifest is the library's manifest, which a host's build merges into
// its own: the runtime starts the app from the native library directory,
// so native libraries must be extracted, and the provider of the files
// the app hands other apps is named after the host's package.
const aarManifest = `<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="dev.oneo.nsgo">
    <uses-sdk android:minSdkVersion="28" />
    <uses-permission android:name="android.permission.INTERNET" />
    <uses-permission android:name="android.permission.ACCESS_NETWORK_STATE" />
    <application android:extractNativeLibs="true">
        <provider
            android:name="dev.oneo.nsgo.FileProvider"
            android:authorities="${applicationId}.nsgo.files"
            android:exported="false"
            android:grantUriPermissions="true" />
    </application>
</manifest>
`

// buildAndroidAAR writes <dir>/<name>.aar for a host app's Gradle build:
// the Java runtime's classes as a jar, the app's executable for goarch as
// libnsgo.so and its resources as assets.
func buildAndroidAAR(ctx context.Context, p *project, dir, goarch string) error {
	tmp, err := os.MkdirTemp("", "nsgo-aar-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	app, abi, err := buildAndroidBinary(ctx, p, tmp, goarch)
	if err != nil {
		return err
	}
	stage(ctx, "packaging the AAR")
	f, err := os.Create(filepath.Join(dir, p.Name+".aar"))
	if err != nil {
		return err
	}
	defer f.Close()
	z := zip.NewWriter(f)
	add := func(name string, data []byte) error {
		w, err := z.Create(name)
		if err == nil {
			_, err = w.Write(data)
		}
		return err
	}
	for _, e := range []struct {
		name string
		data []byte
	}{
		{"AndroidManifest.xml", []byte(aarManifest)},
		{"classes.jar", androidruntime.ClassesJar},
		{"R.txt", nil},
		{"jni/" + abi + "/libnsgo.so", app},
	} {
		if err := add(e.name, e.data); err != nil {
			return err
		}
	}
	err = walkResources(p, func(rel string, data []byte) error { return add("assets/"+rel, data) })
	if err != nil {
		return err
	}
	return z.Close()
}
