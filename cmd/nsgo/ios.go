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

// appleOS is iOS, tvOS or watchOS: what their builds, bundles,
// simulators and devices differ in. All build GOOS=ios; tvOS links with
// its own SDK, and the UIKit backend finds out at run time which it runs
// on; watchOS's apps are SwiftUI hosts of their own (watchos.go).
type appleOS struct {
	name    string // "iOS"
	target  string // the devices' target; the simulator's adds _simulator
	sdk     string // the devices' SDK; the simulator's ends in simulator
	minimum string // MinimumOSVersion
	device  string // what names its devices and simulators, "iPhone"
	// families are TARGETED_DEVICE_FAMILY and UIDeviceFamily; idioms are
	// actool's target devices.
	families []int
	idioms   []string
	// platform is CFBundleSupportedPlatforms' devices' name; the
	// simulator's ends in Simulator.
	platform string
	// deployment is the Xcode setting holding minimum.
	deployment string
	plist      *template.Template
}

var (
	iOS = &appleOS{name: "iOS", target: "ios", sdk: "iphoneos", minimum: "15.0", device: "iPhone",
		families: []int{1, 2}, idioms: []string{"iphone", "ipad"}, platform: "iPhone",
		deployment: "IPHONEOS_DEPLOYMENT_TARGET", plist: iosPlist}
	tvOS = &appleOS{name: "tvOS", target: "tvos", sdk: "appletvos", minimum: "17.0", device: "Apple TV",
		families: []int{3}, idioms: []string{"tv"}, platform: "AppleTV",
		deployment: "TVOS_DEPLOYMENT_TARGET", plist: tvosPlist}
)

// appleOSOf is the OS of a target, an Apple TV's or an iPhone's, and
// whether it is the simulator's.
func appleOSOf(target string) (sys *appleOS, simulator, ok bool) {
	name, sim := strings.CutSuffix(target, "_simulator")
	for _, sys := range []*appleOS{iOS, tvOS, watchOS} {
		if sys.target == name {
			return sys, sim, true
		}
	}
	return nil, false, false
}

// sdkName is the SDK the devices or the simulator build with.
func (sys *appleOS) sdkName(simulator bool) string {
	if simulator {
		return strings.TrimSuffix(sys.sdk, "os") + "simulator"
	}
	return sys.sdk
}

// triple is clang's target.
func (sys *appleOS) triple(simulator bool) string {
	t := "arm64-apple-" + sys.target + sys.minimum
	if simulator {
		t += "-simulator"
	}
	return t
}

// platformName is CFBundleSupportedPlatforms' entry.
func (sys *appleOS) platformName(simulator bool) string {
	if simulator {
		return sys.platform + "Simulator"
	}
	return sys.platform + "OS"
}

// familyList is TARGETED_DEVICE_FAMILY's value, "1,2".
func (sys *appleOS) familyList() string {
	parts := make([]string, len(sys.families))
	for i, f := range sys.families {
		parts[i] = strconv.Itoa(f)
	}
	return strings.Join(parts, ",")
}

// iosEnv is the environment go builds for sys's devices or simulator in:
// Go links iOS and tvOS only through Xcode's linker; NSGo itself has no C.
func iosEnv(ctx context.Context, sys *appleOS, simulator bool) ([]string, error) {
	sdkName := sys.sdkName(simulator)
	sdk, err := output(ctx, "xcrun", "--sdk", sdkName, "--show-sdk-path")
	if err != nil {
		return nil, fmt.Errorf("the %s SDK needs Xcode: %w", sys.name, err)
	}
	clang, err := output(ctx, "xcrun", "--sdk", sdkName, "-f", "clang")
	if err != nil {
		return nil, err
	}
	flags := "-isysroot " + sdk + " -target " + sys.triple(simulator)
	return []string{"GOOS=ios", "GOARCH=arm64", "CGO_ENABLED=1",
		"CC=" + clang + " " + flags, "CGO_CFLAGS=" + flags, "CGO_LDFLAGS=" + flags}, nil
}

// buildIOSApp writes <dir>/<name>.app for sys's simulator or devices.
// Device builds are left unsigned for the developer's own identity.
func buildIOSApp(ctx context.Context, p *project, sys *appleOS, dir string, simulator bool) (string, error) {
	if sys == watchOS {
		return buildWatchApp(ctx, p, dir, simulator)
	}
	env, err := iosEnv(ctx, sys, simulator)
	if err != nil {
		return "", err
	}
	app := filepath.Join(dir, p.Name+".app")
	// Go's runtime objects target a newer OS than the minimum; the
	// linker's warning about it is noise.
	if err := goBuild(ctx, p, filepath.Join(app, p.Name), env, "-extldflags=-Wl,-w"); err != nil {
		return "", err
	}
	stage(ctx, "bundling the app")
	// An iOS bundle's root is its resource directory; a resources
	// directory there would read as a macOS bundle's and fail signing.
	if err := copyMobileResources(p, app); err != nil {
		return "", err
	}
	icon, err := writeIOSIcon(ctx, p, sys, app, simulator)
	if err != nil {
		return "", err
	}
	names, err := writeAppleNames(p, app)
	if err != nil {
		return "", err
	}
	// An Apple TV has no widgets.
	widgets := sys == iOS && len(p.Widgets) > 0
	if sys != iOS && len(p.Widgets) > 0 {
		detailIn(ctx, "no widgets on %s: they are left out", sys.name)
	}
	extra := icon + "\n" + names + "\n\t<key>CFBundleSupportedPlatforms</key><array><string>" + sys.platformName(simulator) + "</string></array>"
	if widgets {
		extra += "\n" + p.widgetPlist(true)
	}
	if clip := p.clipPlist(); clip != "" {
		extra += "\n" + clip
	}
	if err := writePlist(filepath.Join(app, "Info.plist"), p, sys.plist, extra); err != nil {
		return "", err
	}
	if err := addIOSFrameworks(ctx, p, sys, app, env, simulator); err != nil {
		return "", err
	}
	if widgets {
		if err := addIOSWidgets(ctx, p, app, simulator); err != nil {
			return "", err
		}
	}
	if sys == iOS {
		if err := addIOSAppClip(ctx, p, app, simulator); err != nil {
			return "", err
		}
	} else if p.AppClip != nil {
		detailIn(ctx, "no App Clip on %s: it is left out", sys.name)
	}
	if !simulator {
		return app, nil
	}
	stage(ctx, "signing")
	ents := p.simulatorEntitlements()
	if len(ents) == 0 {
		return app, run(ctx, "codesign", "--force", "--sign", "-", app)
	}
	// The simulator grants the app group, and ties an app to its clip,
	// from an ad hoc signature's entitlements.
	path, err := writeEntitlements(ents)
	if err != nil {
		return "", err
	}
	defer os.Remove(path)
	return app, run(ctx, "codesign", "--force", "--sign", "-", "--entitlements", path, app)
}

// The SDK requires the scene life cycle; platform/ios registers
// NsgoSceneDelegate. An iPhone grants display links more than 60 Hz only
// to apps that set CADisableMinimumFrameDurationOnPhone.
var iosPlist = template.Must(template.New("ios").Parse(plistHead + `	<key>MinimumOSVersion</key><string>{{.IOSMinimum}}</string>
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

// tvosPlist is iosPlist for an Apple TV, which keeps an app light unless
// it says it follows the system's appearance.
var tvosPlist = template.Must(template.New("tvos").Parse(plistHead + `	<key>MinimumOSVersion</key><string>17.0</string>
	<key>UILaunchScreen</key><dict/>
	<key>UIDeviceFamily</key><array><integer>3</integer></array>
	<key>UIUserInterfaceStyle</key><string>{{if eq .Appearance "light"}}Light{{else if eq .Appearance "dark"}}Dark{{else}}Automatic{{end}}</string>
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

// iosSession installs the app on sys's simulator -device names, or the
// booted one, booting one when none is, and streams its output here.
func iosSession(sys *appleOS, opts devOptions) session {
	cache := sys.target + "_simulator"
	return session{
		build: func(ctx context.Context, p *project) error {
			dir := p.cache("dev", cache)
			os.RemoveAll(dir)
			_, err := buildIOSApp(ctx, p, sys, dir, true)
			return err
		},
		start: func(ctx context.Context, p *project) (app, error) {
			udid, err := bootSimulator(ctx, sys, opts.device)
			if err != nil {
				return app{}, err
			}
			bundle := filepath.Join(p.cache("dev", cache), p.Name+".app")
			if opts.clip != "" {
				stop, wait, err := startIOSClip(ctx, p, udid, bundle, opts.clip, opts.server.url())
				if err != nil {
					return app{}, err
				}
				return app{
					stop: func() {
						stop()
						exec.Command("xcrun", "simctl", "terminate", udid, p.AppClip.ID).Run()
					},
					wait: wait,
				}, nil
			}
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
// iPhone or Apple TV, and streams its output here until it exits.
func iosDeviceSession(sys *appleOS, opts devOptions) session {
	return session{
		build: func(ctx context.Context, p *project) error {
			dir := p.cache("dev", sys.target)
			os.RemoveAll(dir)
			// The device reaches the dev server over the LAN.
			if lan := opts.server.lanURLs(); len(lan) > 0 {
				p.devURL = lan[0]
			}
			_, err := buildIOSApp(ctx, p, sys, filepath.Join(dir, "app"), false)
			return err
		},
		start: func(ctx context.Context, p *project) (app, error) {
			device, err := selectIOSDevice(ctx, sys, opts.device)
			if err != nil {
				return app{}, err
			}
			team, err := iosTeam(ctx, opts.team)
			if err != nil {
				return app{}, err
			}
			dir := p.cache("dev", sys.target)
			if opts.clip != "" {
				return startIOSDeviceClip(ctx, p, filepath.Join(dir, "app", p.Name+".app"), filepath.Join(dir, "sign-clip"), team, device, opts.clip)
			}
			if err := installOnIOSDevice(ctx, p, sys, filepath.Join(dir, "app", p.Name+".app"), filepath.Join(dir, "sign"), team, device); err != nil {
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
func installOnIOSDevice(ctx context.Context, p *project, sys *appleOS, app, work, team string, device iosDevice) error {
	signed, err := signIOSApp(ctx, p, sys, app, work, team, device)
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
func signIOSApp(ctx context.Context, p *project, sys *appleOS, app, work, team string, device iosDevice) (string, error) {
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
	data := iosProjectData(p, sys, team)
	if p.clipParent != "" {
		// Xcode registers an App Clip's App ID under its parent's from
		// these entitlements, and fills in the team's prefix.
		ents := map[string]any{
			"com.apple.developer.on-demand-install-capable":      true,
			"com.apple.developer.parent-application-identifiers": []any{"$(AppIdentifierPrefix)" + p.clipParent},
		}
		if domains := p.parentClip.Domains; len(domains) > 0 {
			ents["com.apple.developer.associated-domains"] = clipDomains(domains)
		}
		if err := os.WriteFile(filepath.Join(work, "Clip.entitlements"), encodePlist(ents), 0o644); err != nil {
			return "", err
		}
		data["Entitlements"] = "Clip.entitlements"
	}
	f, err := os.Create(filepath.Join(project, "project.pbxproj"))
	if err != nil {
		return "", err
	}
	err = iosProject.Execute(f, data)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return "", err
	}
	products := filepath.Join(work, "products")
	if len(p.Widgets) > 0 {
		warnIn(ctx, "the widgets are left out: Xcode signs the app alone for devices; nsgo build -target ios with sign.profile and sign.widgets_profile keeps them")
	}
	if p.AppClip != nil {
		warnIn(ctx, "the App Clip is left out: Xcode signs the app alone for devices; nsgo dev -clip runs the clip alone, and nsgo build -target ios with sign.profile and sign.clip_profile keeps it")
	}
	event("sign for %s with Xcode", device.Name)
	if err := runXcodebuild(ctx,
		"-project", project,
		"-target", "Nsgo",
		"-configuration", "Release",
		"-destination", "platform="+sys.name+",id="+device.UDID,
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

// iosProjectData fills iosProject in for p on sys, signed by team: an
// app's, or an App Clip's, whose product type Xcode signs as one.
func iosProjectData(p *project, sys *appleOS, team string) map[string]string {
	data := map[string]string{
		"Name":        p.Name,
		"App":         p.Name + ".app",
		"ID":          p.ID,
		"Team":        team,
		"Script":      iosSignScript,
		"SDK":         sys.sdk,
		"Deployment":  sys.deployment,
		"Minimum":     sys.minimum,
		"Families":    sys.familyList(),
		"ProductType": "com.apple.product-type.application",
	}
	if p.clipParent != "" {
		data["Minimum"] = p.IOSMinimum()
		data["ProductType"] = "com.apple.product-type.application.on-demand-install-capable"
	}
	return data
}

// iosSignScript replaces Xcode's empty app with the Go build's contents,
// except its Info.plist, which Xcode processes itself, and the widget
// extension and the App Clip, which Xcode's project has no target to sign.
const iosSignScript = `set -e
app="$CODESIGNING_FOLDER_PATH"
mkdir -p "$app"
for file in "$SRCROOT/unsigned"/*; do
	name=$(basename "$file")
	case "$name" in
		Info.plist|PkgInfo|_CodeSignature|embedded.mobileprovision|PlugIns|AppClips) continue ;;
	esac
	cp -R "$file" "$app/"
done
chmod +x "$app/$EXECUTABLE_NAME"
for fw in "$app"/Frameworks/*.framework; do
	[ -d "$fw" ] && codesign --force --sign "$EXPANDED_CODE_SIGN_IDENTITY" "$fw"
done
true
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

// iosDevice is a physical iPhone or Apple TV this Mac knows.
type iosDevice struct {
	Name          string
	Platform      string // "iOS", "tvOS"
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
		Platform     string `json:"platform"`
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
			Platform     string `json:"platform"`
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

// platform is the device's OS; older Xcodes say none, only of iPhones.
func (d devicectlDevice) platform() string {
	if d.HardwareProperties.Platform != "" {
		return d.HardwareProperties.Platform
	}
	if d.Properties.Hardware.Platform != "" {
		return d.Properties.Hardware.Platform
	}
	return "iOS"
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
			Platform:      d.platform(),
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

// selectIOSDevice picks sys's device named by -device, or the only one
// connected.
func selectIOSDevice(ctx context.Context, sys *appleOS, wanted string) (iosDevice, error) {
	devices, err := iosDevices(ctx)
	if err != nil {
		return iosDevice{}, err
	}
	return pickIOSDevice(sys, devices, wanted)
}

// pickIOSDevice chooses from sys's devices this Mac knows.
func pickIOSDevice(sys *appleOS, devices []iosDevice, wanted string) (iosDevice, error) {
	var paired, connected []iosDevice
	for _, d := range devices {
		if d.Platform == sys.name && d.Paired {
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
		return iosDevice{}, fmt.Errorf("no %s device %q is paired with this Mac; it lists %s", sys.name, wanted, iosDeviceNames(paired))
	}
	switch {
	case len(paired) == 0 && sys == tvOS:
		return iosDevice{}, errors.New("no Apple TV is paired with this Mac: pair one in Xcode's Devices window from the Apple TV's Settings > Remotes and Devices > Remote App and Devices")
	case len(paired) == 0:
		return iosDevice{}, errors.New("no iPhone is paired with this Mac: connect one, trust it, and turn on Developer Mode in Settings > Privacy & Security")
	case len(connected) == 1:
		return withDeveloperMode(connected[0])
	case len(connected) > 1:
		return iosDevice{}, fmt.Errorf("several %ss are connected: pass -device with one of %s", sys.device, iosDeviceNames(connected))
	case len(paired) == 1:
		return withDeveloperMode(paired[0])
	}
	return iosDevice{}, fmt.Errorf("several %ss are paired with this Mac: pass -device with one of %s", sys.device, iosDeviceNames(paired))
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

// simulator is an iOS or tvOS simulator this Mac has.
type simulator struct {
	UDID, Name, State, Runtime string
}

// simulators lists sys's available simulators, newest runtime first.
func simulators(ctx context.Context, sys *appleOS) ([]simulator, error) {
	out, err := output(ctx, "xcrun", "simctl", "list", "devices", "available", "-j")
	if err != nil {
		return nil, fmt.Errorf("the %s simulator needs Xcode: %w", sys.name, err)
	}
	return parseSimulators(sys, []byte(out))
}

func parseSimulators(sys *appleOS, data []byte) ([]simulator, error) {
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
		if !strings.Contains(rt, ".SimRuntime."+sys.name+"-") {
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
	_, v, _ := strings.Cut(rt, "OS-")
	major, rest, _ := strings.Cut(v, "-")
	minor, _, _ := strings.Cut(rest, "-")
	a, _ := strconv.Atoi(major)
	b, _ := strconv.Atoi(minor)
	return a*100 + b
}

// pickSimulator chooses the simulator wanted names, by name or UDID (a
// booted one of that name first, then the newest runtime's), or without a
// name the booted one, else the newest of sys's device, an iPhone or an
// Apple TV.
func pickSimulator(sys *appleOS, sims []simulator, wanted string) (simulator, error) {
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
		if wanted != "" || strings.Contains(s.Name, sys.device) {
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
	return simulator{}, fmt.Errorf("no %s simulator is installed; add one in Xcode", sys.device)
}

// bootSimulator boots the simulator wanted names (or picks one) unless it
// runs, shows the Simulator app and returns its UDID.
func bootSimulator(ctx context.Context, sys *appleOS, wanted string) (string, error) {
	sims, err := simulators(ctx, sys)
	if err != nil {
		return "", err
	}
	sim, err := pickSimulator(sys, sims, wanted)
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
			productType = {{pbx .ProductType}};
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
				{{.Deployment}} = {{.Minimum}};
				SDKROOT = {{.SDK}};
			};
			name = Debug;
		};
		AA000000000000000000000A /* Release */ = {
			isa = XCBuildConfiguration;
			buildSettings = {
				ALWAYS_SEARCH_USER_PATHS = NO;
				CLANG_ENABLE_MODULES = YES;
				ENABLE_USER_SCRIPT_SANDBOXING = NO;
				{{.Deployment}} = {{.Minimum}};
				SDKROOT = {{.SDK}};
			};
			name = Release;
		};
		AA000000000000000000000B /* Debug */ = {
			isa = XCBuildConfiguration;
			buildSettings = {
{{- with .Entitlements}}
				CODE_SIGN_ENTITLEMENTS = {{pbx .}};
{{- end}}
				CODE_SIGN_STYLE = Automatic;
				DEVELOPMENT_TEAM = {{pbx .Team}};
				ENABLE_USER_SCRIPT_SANDBOXING = NO;
				INFOPLIST_FILE = Info.plist;
				{{.Deployment}} = {{.Minimum}};
				PRODUCT_BUNDLE_IDENTIFIER = {{pbx .ID}};
				PRODUCT_NAME = {{pbx .Name}};
				SDKROOT = {{.SDK}};
				TARGETED_DEVICE_FAMILY = {{pbx .Families}};
			};
			name = Debug;
		};
		AA000000000000000000000C /* Release */ = {
			isa = XCBuildConfiguration;
			buildSettings = {
{{- with .Entitlements}}
				CODE_SIGN_ENTITLEMENTS = {{pbx .}};
{{- end}}
				CODE_SIGN_STYLE = Automatic;
				DEVELOPMENT_TEAM = {{pbx .Team}};
				ENABLE_USER_SCRIPT_SANDBOXING = NO;
				INFOPLIST_FILE = Info.plist;
				{{.Deployment}} = {{.Minimum}};
				PRODUCT_BUNDLE_IDENTIFIER = {{pbx .ID}};
				PRODUCT_NAME = {{pbx .Name}};
				SDKROOT = {{.SDK}};
				TARGETED_DEVICE_FAMILY = {{pbx .Families}};
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
