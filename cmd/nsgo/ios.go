package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"text/template"
)

// buildIOSApp writes <dir>/<name>.app for the simulator or for devices.
// Device builds are left unsigned for the developer's own identity.
func buildIOSApp(ctx context.Context, p *project, dir string, simulator bool) (string, error) {
	sdkName, triple := "iphoneos", "arm64-apple-ios15.0"
	if simulator {
		sdkName, triple = "iphonesimulator", "arm64-apple-ios15.0-simulator"
	}
	sdk, err := output(ctx, "xcrun", "--sdk", sdkName, "--show-sdk-path")
	if err != nil {
		return "", fmt.Errorf("the iOS SDK needs Xcode: %w", err)
	}
	clang, err := output(ctx, "xcrun", "--sdk", sdkName, "-f", "clang")
	if err != nil {
		return "", err
	}
	// Go links iOS only through Xcode's linker; NSGo itself has no C.
	flags := "-isysroot " + sdk + " -target " + triple
	app := filepath.Join(dir, p.Name+".app")
	env := []string{"GOOS=ios", "GOARCH=arm64", "CGO_ENABLED=1",
		"CC=" + clang + " " + flags, "CGO_CFLAGS=" + flags, "CGO_LDFLAGS=" + flags}
	// Go's runtime objects target a newer iOS than 15.0; the linker's
	// warning about it is noise.
	if err := goBuild(ctx, p, filepath.Join(app, p.Name), env, "-extldflags=-Wl,-w"); err != nil {
		return "", err
	}
	stage("bundling the app")
	// An iOS bundle's root is its resource directory; a resources
	// directory there would read as a macOS bundle's and fail signing.
	if err := copyResources(p, app); err != nil {
		return "", err
	}
	icon, err := writeIOSIcon(ctx, p, app, simulator)
	if err != nil {
		return "", err
	}
	if err := writePlist(filepath.Join(app, "Info.plist"), p, iosPlist, icon); err != nil {
		return "", err
	}
	if simulator {
		stage("signing")
		return app, run(ctx, "codesign", "--force", "--sign", "-", app)
	}
	return app, nil
}

// The SDK requires the scene life cycle; platform/ios registers
// NsgoSceneDelegate. An iPhone grants display links more than 60 Hz only
// to apps that set CADisableMinimumFrameDurationOnPhone.
var iosPlist = template.Must(template.New("ios").Parse(plistHead + `	<key>MinimumOSVersion</key><string>15.0</string>
	<key>CADisableMinimumFrameDurationOnPhone</key><true/>
	<key>UILaunchScreen</key><dict/>
	<key>UIRequiresFullScreen</key><true/>
	<key>UIDeviceFamily</key><array><integer>1</integer><integer>2</integer></array>
{{- if eq .Appearance "light"}}
	<key>UIUserInterfaceStyle</key><string>Light</string>
{{- else if eq .Appearance "dark"}}
	<key>UIUserInterfaceStyle</key><string>Dark</string>
{{- end}}
	<key>UIApplicationSceneManifest</key>
	<dict>
		<key>UIApplicationSupportsMultipleScenes</key><false/>
		<key>UISceneConfigurations</key>
		<dict>
			<key>UIWindowSceneSessionRoleApplication</key>
			<array><dict>
				<key>UISceneConfigurationName</key><string>Default</string>
				<key>UISceneDelegateClassName</key><string>NsgoSceneDelegate</string>
			</dict></array>
		</dict>
	</dict>
</dict>
</plist>
`))

// iosSession installs the app on the simulator -device names, or the
// booted one, booting one when none is, and streams its output here.
func iosSession(opts devOptions) session {
	return session{
		build: func(ctx context.Context, p *project) error {
			dir := p.cache("dev", "ios_simulator")
			os.RemoveAll(dir)
			_, err := buildIOSApp(ctx, p, dir, true)
			return err
		},
		start: func(ctx context.Context, p *project) (app, error) {
			udid, err := bootSimulator(ctx, opts.device)
			if err != nil {
				return app{}, err
			}
			bundle := filepath.Join(p.cache("dev", "ios_simulator"), p.Name+".app")
			if err := run(ctx, "xcrun", "simctl", "install", udid, bundle); err != nil {
				return app{}, err
			}
			// The simulator shares this Mac's network.
			env := []string{"SIMCTL_CHILD_NSGO_DEV_SERVER=" + opts.server.url()}
			stop, wait, err := startProcessEnv(env, "xcrun", "simctl", "launch", "--console-pty", "--terminate-running-process", udid, p.ID)
			if err != nil {
				return app{}, err
			}
			return app{
				stop: func() {
					stop()
					exec.Command("xcrun", "simctl", "terminate", udid, p.ID).Run()
				},
				wait: wait,
			}, nil
		},
	}
}

// iosDeviceSession builds, signs and installs the app on a connected
// iPhone, and streams its output here until it exits.
func iosDeviceSession(opts devOptions) session {
	return session{
		build: func(ctx context.Context, p *project) error {
			dir := p.cache("dev", "ios")
			os.RemoveAll(dir)
			// The iPhone reaches the dev server over the LAN.
			if lan := opts.server.lanURLs(); len(lan) > 0 {
				p.devURL = lan[0]
			}
			_, err := buildIOSApp(ctx, p, filepath.Join(dir, "app"), false)
			return err
		},
		start: func(ctx context.Context, p *project) (app, error) {
			device, err := selectIOSDevice(ctx, opts.device)
			if err != nil {
				return app{}, err
			}
			team, err := iosTeam(ctx, opts.team)
			if err != nil {
				return app{}, err
			}
			dir := p.cache("dev", "ios")
			if err := installOnIOSDevice(ctx, p, filepath.Join(dir, "app", p.Name+".app"), filepath.Join(dir, "sign"), team, device); err != nil {
				return app{}, err
			}
			event("launch %s on %s", p.ID, device.Name)
			stop, wait, err := startProcess("xcrun", "devicectl", "device", "process", "launch",
				"--console", "--terminate-existing", "--device", device.UDID, p.ID)
			if err != nil {
				return app{}, err
			}
			return app{stop: stop, wait: wait}, nil
		},
	}
}

// installOnIOSDevice signs app for device with Xcode, working in work,
// and installs it there.
func installOnIOSDevice(ctx context.Context, p *project, app, work, team string, device iosDevice) error {
	signed, err := signIOSApp(ctx, p, app, work, team, device)
	if err != nil {
		return err
	}
	event("install on %s", device.Name)
	return run(ctx, "xcrun", "devicectl", "device", "install", "app", "--device", device.UDID, signed)
}

// signIOSApp signs a device build by handing it to Xcode, whose automatic
// signing makes the App ID, certificate and provisioning profile the app
// needs from the developer account in Xcode. The generated project holds
// a script that copies the app's build product into Xcode's, which Xcode
// then signs; work is Xcode's intermediate and output directory.
func signIOSApp(ctx context.Context, p *project, app, work, team string, device iosDevice) (string, error) {
	project := filepath.Join(work, "Nsgo.xcodeproj")
	if err := os.RemoveAll(work); err != nil {
		return "", err
	}
	if err := os.MkdirAll(project, 0o755); err != nil {
		return "", err
	}
	plist, err := os.ReadFile(filepath.Join(app, "Info.plist"))
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(work, "Info.plist"), plist, 0o644); err != nil {
		return "", err
	}
	if err := os.Symlink(app, filepath.Join(work, "unsigned")); err != nil {
		return "", err
	}
	f, err := os.Create(filepath.Join(project, "project.pbxproj"))
	if err != nil {
		return "", err
	}
	err = iosProject.Execute(f, map[string]string{
		"Name":   p.Name,
		"App":    p.Name + ".app",
		"ID":     p.ID,
		"Team":   team,
		"Script": iosSignScript,
	})
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return "", err
	}
	products := filepath.Join(work, "products")
	event("sign for %s with Xcode", device.Name)
	if err := runXcodebuild(ctx,
		"-project", project,
		"-target", "Nsgo",
		"-configuration", "Release",
		"-destination", "platform=iOS,id="+device.UDID,
		"-allowProvisioningUpdates",
		"-allowProvisioningDeviceRegistration",
		"CONFIGURATION_BUILD_DIR="+products,
		"SYMROOT="+filepath.Join(work, "build"),
		"OBJROOT="+filepath.Join(work, "obj"),
		"build"); err != nil {
		return "", err
	}
	signed := filepath.Join(products, p.Name+".app")
	if _, err := os.Stat(signed); err != nil {
		return "", fmt.Errorf("Xcode did not sign the app: %w", err)
	}
	return signed, nil
}

// iosSignScript replaces Xcode's empty app with the Go build's contents,
// except its Info.plist, which Xcode processes itself.
const iosSignScript = `set -e
app="$CODESIGNING_FOLDER_PATH"
mkdir -p "$app"
for file in "$SRCROOT/unsigned"/*; do
	name=$(basename "$file")
	case "$name" in
		Info.plist|PkgInfo|_CodeSignature|embedded.mobileprovision) continue ;;
	esac
	cp -R "$file" "$app/"
done
chmod +x "$app/$EXECUTABLE_NAME"
`

// iosTeam returns the Apple developer team to sign with: the -team flag,
// or the team Xcode last used.
func iosTeam(ctx context.Context, flag string) (string, error) {
	if flag != "" {
		return flag, nil
	}
	team, _ := output(ctx, "defaults", "read", "com.apple.dt.Xcode", "IDEProvisioningTeamManagerLastSelectedTeamID")
	if team != "" {
		return team, nil
	}
	return "", errors.New("no Apple developer team: pass -team with your team ID, or select one in Xcode's Settings > Accounts")
}

// iosDevice is a physical iPhone connected to this Mac.
type iosDevice struct {
	Name          string
	UDID          string
	Serial        string
	ECID          int64
	Paired        bool
	Connected     bool
	DeveloperMode bool
}

// devicectlList is the part of `xcrun devicectl list devices --json-output`
// this tool reads. Xcode marks the top-level device fields deprecated in
// favour of properties, which newer releases fill in, so both are read.
type devicectlList struct {
	Result struct {
		Devices []devicectlDevice `json:"devices"`
	} `json:"result"`
}

type devicectlDevice struct {
	DeviceProperties struct {
		Name                string `json:"name"`
		DeveloperModeStatus string `json:"developerModeStatus"`
	} `json:"deviceProperties"`
	HardwareProperties struct {
		Reality      string `json:"reality"`
		UDID         string `json:"udid"`
		SerialNumber string `json:"serialNumber"`
		ECID         int64  `json:"ecid"`
	} `json:"hardwareProperties"`
	ConnectionProperties struct {
		PairingState string `json:"pairingState"`
		TunnelState  string `json:"tunnelState"`
	} `json:"connectionProperties"`
	Properties struct {
		Hardware struct {
			Reality      string `json:"reality"`
			UDID         string `json:"udid"`
			SerialNumber string `json:"serialNumber"`
			ECID         int64  `json:"ecid"`
		} `json:"hardware"`
		State struct {
			Name                string          `json:"name"`
			DeveloperModeStatus json.RawMessage `json:"developerModeStatus"`
		} `json:"state"`
		Connection struct {
			PairingState string `json:"pairingState"`
			State        string `json:"state"`
		} `json:"connection"`
	} `json:"properties"`
}

func (d devicectlDevice) name() string {
	if d.DeviceProperties.Name != "" {
		return d.DeviceProperties.Name
	}
	return d.Properties.State.Name
}

func (d devicectlDevice) reality() string {
	if d.HardwareProperties.Reality != "" {
		return d.HardwareProperties.Reality
	}
	return d.Properties.Hardware.Reality
}

func (d devicectlDevice) udid() string {
	if d.HardwareProperties.UDID != "" {
		return d.HardwareProperties.UDID
	}
	return d.Properties.Hardware.UDID
}

func (d devicectlDevice) serial() string {
	if d.HardwareProperties.SerialNumber != "" {
		return d.HardwareProperties.SerialNumber
	}
	return d.Properties.Hardware.SerialNumber
}

func (d devicectlDevice) ecid() int64 {
	if d.HardwareProperties.ECID != 0 {
		return d.HardwareProperties.ECID
	}
	return d.Properties.Hardware.ECID
}

func (d devicectlDevice) paired() bool {
	state := d.ConnectionProperties.PairingState
	if state == "" {
		state = d.Properties.Connection.PairingState
	}
	return state == "paired"
}

func (d devicectlDevice) connected() bool {
	if d.ConnectionProperties.TunnelState != "" {
		return d.ConnectionProperties.TunnelState == "connected"
	}
	return d.Properties.Connection.State == "connected"
}

func (d devicectlDevice) developerMode() bool {
	if d.DeviceProperties.DeveloperModeStatus != "" {
		return d.DeviceProperties.DeveloperModeStatus == "enabled"
	}
	return strings.Contains(string(d.Properties.State.DeveloperModeStatus), "enabled")
}

// iosDevices lists the physical iPhones this Mac knows.
func iosDevices(ctx context.Context) ([]iosDevice, error) {
	out, err := output(ctx, "xcrun", "devicectl", "list", "devices", "--json-output", "-")
	if err != nil {
		return nil, fmt.Errorf("listing iOS devices needs Xcode: %w", err)
	}
	return parseIOSDevices([]byte(out))
}

// parseIOSDevices reads the physical devices from devicectl's JSON.
func parseIOSDevices(data []byte) ([]iosDevice, error) {
	var list devicectlList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("xcrun devicectl list devices: %w", err)
	}
	var devices []iosDevice
	for _, d := range list.Result.Devices {
		if d.reality() != "physical" {
			continue
		}
		devices = append(devices, iosDevice{
			Name:          d.name(),
			UDID:          d.udid(),
			Serial:        d.serial(),
			ECID:          d.ecid(),
			Paired:        d.paired(),
			Connected:     d.connected(),
			DeveloperMode: d.developerMode(),
		})
	}
	return devices, nil
}

// selectIOSDevice picks the iPhone named by -device, or the only one
// connected.
func selectIOSDevice(ctx context.Context, wanted string) (iosDevice, error) {
	devices, err := iosDevices(ctx)
	if err != nil {
		return iosDevice{}, err
	}
	return pickIOSDevice(devices, wanted)
}

// pickIOSDevice chooses from the devices this Mac knows.
func pickIOSDevice(devices []iosDevice, wanted string) (iosDevice, error) {
	var paired, connected []iosDevice
	for _, d := range devices {
		if d.Paired {
			paired = append(paired, d)
			if d.Connected {
				connected = append(connected, d)
			}
		}
	}
	if wanted != "" {
		for _, d := range paired {
			if strings.EqualFold(d.Name, wanted) || strings.EqualFold(d.UDID, wanted) ||
				strings.EqualFold(d.Serial, wanted) || fmt.Sprint(d.ECID) == wanted {
				return withDeveloperMode(d)
			}
		}
		return iosDevice{}, fmt.Errorf("no iOS device %q is paired with this Mac; it lists %s", wanted, iosDeviceNames(paired))
	}
	switch {
	case len(paired) == 0:
		return iosDevice{}, errors.New("no iPhone is paired with this Mac: connect one, trust it, and turn on Developer Mode in Settings > Privacy & Security")
	case len(connected) == 1:
		return withDeveloperMode(connected[0])
	case len(connected) > 1:
		return iosDevice{}, fmt.Errorf("several iPhones are connected: pass -device with one of %s", iosDeviceNames(connected))
	case len(paired) == 1:
		return withDeveloperMode(paired[0])
	}
	return iosDevice{}, fmt.Errorf("several iPhones are paired with this Mac: pass -device with one of %s", iosDeviceNames(paired))
}

// withDeveloperMode rejects a device that cannot run development builds.
func withDeveloperMode(d iosDevice) (iosDevice, error) {
	if !d.DeveloperMode {
		return iosDevice{}, fmt.Errorf("developer mode is off on %s: turn it on in Settings > Privacy & Security > Developer Mode", d.Name)
	}
	return d, nil
}

// iosDeviceNames lists devices for a message, each with its UDID.
func iosDeviceNames(devices []iosDevice) string {
	if len(devices) == 0 {
		return "none"
	}
	names := make([]string, len(devices))
	for i, d := range devices {
		names[i] = d.Name + " (" + d.UDID + ")"
	}
	return strings.Join(names, ", ")
}

// runXcodebuild runs xcodebuild, reporting only the end of its output when
// it fails, since a successful build's log buries the error.
func runXcodebuild(ctx context.Context, args ...string) error {
	out, err := exec.CommandContext(ctx, "xcodebuild", args...).CombinedOutput()
	if err == nil {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) > 30 {
		lines = lines[len(lines)-30:]
	}
	return fmt.Errorf("xcodebuild %s: %w\n%s", strings.Join(args, " "), err, strings.Join(lines, "\n"))
}

// simulator is an iOS simulator this Mac has.
type simulator struct {
	UDID, Name, State, Runtime string
}

// simulators lists the available simulators, newest runtime first.
func simulators(ctx context.Context) ([]simulator, error) {
	out, err := output(ctx, "xcrun", "simctl", "list", "devices", "available", "-j")
	if err != nil {
		return nil, fmt.Errorf("the iOS simulator needs Xcode: %w", err)
	}
	return parseSimulators([]byte(out))
}

func parseSimulators(data []byte) ([]simulator, error) {
	var list struct {
		Devices map[string][]struct {
			UDID, Name, State string
		} `json:"devices"`
	}
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("xcrun simctl list devices: %w", err)
	}
	var out []simulator
	for rt, ds := range list.Devices {
		if !strings.Contains(rt, "iOS") {
			continue
		}
		for _, d := range ds {
			out = append(out, simulator{UDID: d.UDID, Name: d.Name, State: d.State, Runtime: rt})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if a, b := runtimeVersion(out[i].Runtime), runtimeVersion(out[j].Runtime); a != b {
			return a > b
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

// runtimeVersion orders runtimes such as ...SimRuntime.iOS-18-2.
func runtimeVersion(rt string) int {
	_, v, _ := strings.Cut(rt, "iOS-")
	major, rest, _ := strings.Cut(v, "-")
	minor, _, _ := strings.Cut(rest, "-")
	a, _ := strconv.Atoi(major)
	b, _ := strconv.Atoi(minor)
	return a*100 + b
}

// pickSimulator chooses the simulator wanted names, by name or UDID (a
// booted one of that name first, then the newest runtime's), or without a
// name the booted one, else the newest iPhone.
func pickSimulator(sims []simulator, wanted string) (simulator, error) {
	var match []simulator
	for _, s := range sims {
		if wanted == "" || s.UDID == wanted || strings.EqualFold(s.Name, wanted) {
			match = append(match, s)
		}
	}
	for _, s := range match {
		if s.State == "Booted" {
			return s, nil
		}
	}
	for _, s := range match {
		if wanted != "" || strings.Contains(s.Name, "iPhone") {
			return s, nil
		}
	}
	if wanted != "" {
		var names []string
		for _, s := range sims {
			if !slices.Contains(names, s.Name) {
				names = append(names, s.Name)
			}
		}
		return simulator{}, fmt.Errorf("no simulator named %q; Xcode has %s", wanted, strings.Join(names, ", "))
	}
	return simulator{}, fmt.Errorf("no iPhone simulator is installed; add one in Xcode")
}

// bootSimulator boots the simulator wanted names (or picks one) unless it
// runs, shows the Simulator app and returns its UDID.
func bootSimulator(ctx context.Context, wanted string) (string, error) {
	sims, err := simulators(ctx)
	if err != nil {
		return "", err
	}
	sim, err := pickSimulator(sims, wanted)
	if err != nil {
		return "", err
	}
	if sim.State != "Booted" {
		event("boot simulator %s", sim.Name)
		if err := run(ctx, "xcrun", "simctl", "boot", sim.UDID); err != nil {
			return "", err
		}
	}
	// Show the simulator's window; the app runs without it.
	if xcode, err := output(ctx, "xcode-select", "-p"); err == nil {
		exec.CommandContext(ctx, "open", filepath.Join(xcode, "Applications", "Simulator.app")).Run()
	}
	return sim.UDID, nil
}

// iosProject is an Xcode project whose app target signs the build product
// this tool copies into it. It has no sources: Xcode makes the bundle,
// runs the script and signs the result.
var iosProject = template.Must(template.New("ios").Funcs(template.FuncMap{"pbx": pbxString}).Parse(`// !$*UTF8*$!
{
	archiveVersion = 1;
	classes = {
	};
	objectVersion = 56;
	objects = {

/* Begin PBXFileReference section */
		AA0000000000000000000001 /* Nsgo.app */ = {isa = PBXFileReference; explicitFileType = wrapper.application; includeInIndex = 0; path = {{pbx .App}}; sourceTree = BUILT_PRODUCTS_DIR; };
/* End PBXFileReference section */

/* Begin PBXFrameworksBuildPhase section */
		AA0000000000000000000002 /* Frameworks */ = {
			isa = PBXFrameworksBuildPhase;
			buildActionMask = 2147483647;
			files = (
			);
			runOnlyForDeploymentPostprocessing = 0;
		};
/* End PBXFrameworksBuildPhase section */

/* Begin PBXGroup section */
		AA0000000000000000000003 = {
			isa = PBXGroup;
			children = (
				AA0000000000000000000001 /* Nsgo.app */,
			);
			sourceTree = "<group>";
		};
/* End PBXGroup section */

/* Begin PBXNativeTarget section */
		AA0000000000000000000004 /* Nsgo */ = {
			isa = PBXNativeTarget;
			buildConfigurationList = AA0000000000000000000005 /* Build configuration list for PBXNativeTarget "Nsgo" */;
			buildPhases = (
				AA0000000000000000000006 /* Copy the app */,
				AA0000000000000000000002 /* Frameworks */,
			);
			buildRules = (
			);
			dependencies = (
			);
			name = Nsgo;
			productName = Nsgo;
			productReference = AA0000000000000000000001 /* Nsgo.app */;
			productType = "com.apple.product-type.application";
		};
/* End PBXNativeTarget section */

/* Begin PBXProject section */
		AA0000000000000000000007 /* Project object */ = {
			isa = PBXProject;
			attributes = {
				BuildIndependentTargetsInParallel = 1;
				LastUpgradeCheck = 1500;
				TargetAttributes = {
					AA0000000000000000000004 = {
						CreatedOnToolsVersion = 15.0;
					};
				};
			};
			buildConfigurationList = AA0000000000000000000008 /* Build configuration list for PBXProject "Nsgo" */;
			compatibilityVersion = "Xcode 14.0";
			developmentRegion = en;
			hasScannedForEncodings = 0;
			knownRegions = (
				en,
				Base,
			);
			mainGroup = AA0000000000000000000003;
			productRefGroup = AA0000000000000000000003 /* Products */;
			projectDirPath = "";
			projectRoot = "";
			targets = (
				AA0000000000000000000004 /* Nsgo */,
			);
		};
/* End PBXProject section */

/* Begin PBXShellScriptBuildPhase section */
		AA0000000000000000000006 /* Copy the app */ = {
			isa = PBXShellScriptBuildPhase;
			alwaysOutOfDate = 1;
			buildActionMask = 2147483647;
			files = (
			);
			inputPaths = (
			);
			name = "Copy the app";
			outputPaths = (
			);
			runOnlyForDeploymentPostprocessing = 0;
			shellPath = /bin/sh;
			shellScript = {{pbx .Script}};
		};
/* End PBXShellScriptBuildPhase section */

/* Begin XCBuildConfiguration section */
		AA0000000000000000000009 /* Debug */ = {
			isa = XCBuildConfiguration;
			buildSettings = {
				ALWAYS_SEARCH_USER_PATHS = NO;
				CLANG_ENABLE_MODULES = YES;
				ENABLE_USER_SCRIPT_SANDBOXING = NO;
				IPHONEOS_DEPLOYMENT_TARGET = 15.0;
				SDKROOT = iphoneos;
			};
			name = Debug;
		};
		AA000000000000000000000A /* Release */ = {
			isa = XCBuildConfiguration;
			buildSettings = {
				ALWAYS_SEARCH_USER_PATHS = NO;
				CLANG_ENABLE_MODULES = YES;
				ENABLE_USER_SCRIPT_SANDBOXING = NO;
				IPHONEOS_DEPLOYMENT_TARGET = 15.0;
				SDKROOT = iphoneos;
			};
			name = Release;
		};
		AA000000000000000000000B /* Debug */ = {
			isa = XCBuildConfiguration;
			buildSettings = {
				CODE_SIGN_STYLE = Automatic;
				DEVELOPMENT_TEAM = {{pbx .Team}};
				ENABLE_USER_SCRIPT_SANDBOXING = NO;
				INFOPLIST_FILE = Info.plist;
				IPHONEOS_DEPLOYMENT_TARGET = 15.0;
				PRODUCT_BUNDLE_IDENTIFIER = {{pbx .ID}};
				PRODUCT_NAME = {{pbx .Name}};
				SDKROOT = iphoneos;
				TARGETED_DEVICE_FAMILY = "1,2";
			};
			name = Debug;
		};
		AA000000000000000000000C /* Release */ = {
			isa = XCBuildConfiguration;
			buildSettings = {
				CODE_SIGN_STYLE = Automatic;
				DEVELOPMENT_TEAM = {{pbx .Team}};
				ENABLE_USER_SCRIPT_SANDBOXING = NO;
				INFOPLIST_FILE = Info.plist;
				IPHONEOS_DEPLOYMENT_TARGET = 15.0;
				PRODUCT_BUNDLE_IDENTIFIER = {{pbx .ID}};
				PRODUCT_NAME = {{pbx .Name}};
				SDKROOT = iphoneos;
				TARGETED_DEVICE_FAMILY = "1,2";
			};
			name = Release;
		};
/* End XCBuildConfiguration section */

/* Begin XCConfigurationList section */
		AA0000000000000000000005 /* Build configuration list for PBXNativeTarget "Nsgo" */ = {
			isa = XCConfigurationList;
			buildConfigurations = (
				AA000000000000000000000B /* Debug */,
				AA000000000000000000000C /* Release */,
			);
			defaultConfigurationIsVisible = 0;
			defaultConfigurationName = Release;
		};
		AA0000000000000000000008 /* Build configuration list for PBXProject "Nsgo" */ = {
			isa = XCConfigurationList;
			buildConfigurations = (
				AA0000000000000000000009 /* Debug */,
				AA000000000000000000000A /* Release */,
			);
			defaultConfigurationIsVisible = 0;
			defaultConfigurationName = Release;
		};
/* End XCConfigurationList section */
	};
	rootObject = AA0000000000000000000007 /* Project object */;
}
`))

// pbxString quotes a value for an Xcode project.
func pbxString(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\t", `\t`).Replace(s) + `"`
}
