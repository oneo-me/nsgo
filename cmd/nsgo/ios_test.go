package main

import (
	"strings"
	"testing"
)

const devicectlJSON = `{
	"result": {
		"devices": [
			{
				"identifier": "9C8D7EDE-05A6-555B-9C10-745C9528CD14",
				"deviceProperties": {"name": "ONEO", "developerModeStatus": "enabled"},
				"hardwareProperties": {"reality": "physical", "udid": "00008160-001204C424C1400A", "serialNumber": "GH9J3JXYD2", "ecid": 5071790057537546},
				"connectionProperties": {"pairingState": "paired", "tunnelState": "connected"}
			},
			{
				"identifier": "0A6ECD14-F679-40A9-A4A9-EF08BE8D0C39",
				"deviceProperties": {"name": "iPhone 18 Pro"},
				"hardwareProperties": {"reality": "simulated", "udid": "0A6ECD14-F679-40A9-A4A9-EF08BE8D0C39"},
				"connectionProperties": {"pairingState": "paired", "tunnelState": "connected"}
			}
		]
	}
}`

func TestParseIOSDevices(t *testing.T) {
	devices, err := parseIOSDevices([]byte(devicectlJSON))
	if err != nil {
		t.Fatal(err)
	}
	if len(devices) != 1 {
		t.Fatalf("got %d devices, want only the physical one: %+v", len(devices), devices)
	}
	d := devices[0]
	if d.Name != "ONEO" || d.UDID != "00008160-001204C424C1400A" || d.Serial != "GH9J3JXYD2" ||
		d.ECID != 5071790057537546 || !d.Paired || !d.Connected || !d.DeveloperMode {
		t.Fatalf("parsed %+v", d)
	}
}

func TestParseIOSDevicesProperties(t *testing.T) {
	data := `{"result": {"devices": [{
		"identifier": "X",
		"properties": {
			"hardware": {"reality": "physical", "udid": "AAA", "serialNumber": "S", "ecid": 1},
			"state": {"name": "ONEO", "developerModeStatus": {"enabled": {"mode": 1}}},
			"connection": {"pairingState": "paired", "state": "connected"}
		}
	}]}}`
	devices, err := parseIOSDevices([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	if len(devices) != 1 {
		t.Fatalf("got %d devices, want 1", len(devices))
	}
	if d := devices[0]; d.Name != "ONEO" || d.UDID != "AAA" || d.Serial != "S" || d.ECID != 1 ||
		!d.Paired || !d.Connected || !d.DeveloperMode {
		t.Fatalf("parsed %+v", d)
	}
}

func TestPickIOSDevice(t *testing.T) {
	oneo := iosDevice{Name: "ONEO", Platform: "iOS", UDID: "00008160-001204C424C1400A", Serial: "GH9J3JXYD2", ECID: 5071790057537546, Paired: true, Connected: true, DeveloperMode: true}
	other := iosDevice{Name: "Other", Platform: "iOS", UDID: "00008160-000000000000000", Paired: true, Connected: true, DeveloperMode: true}
	off := iosDevice{Name: "Off", Platform: "iOS", UDID: "00008160-111111111111111", Paired: true, Connected: true}
	for _, wanted := range []string{"", "ONEO", "oneo", "00008160-001204C424C1400A", "GH9J3JXYD2", "5071790057537546"} {
		got, err := pickIOSDevice(iOS, []iosDevice{oneo}, wanted)
		if err != nil {
			t.Errorf("pick %q: %v", wanted, err)
		} else if got != oneo {
			t.Errorf("pick %q = %+v, want ONEO", wanted, got)
		}
	}
	if _, err := pickIOSDevice(iOS, []iosDevice{oneo}, "missing"); err == nil || !strings.Contains(err.Error(), "ONEO") {
		t.Errorf("pick missing: %v, want an error listing ONEO", err)
	}
	if _, err := pickIOSDevice(iOS, []iosDevice{oneo, other}, ""); err == nil || !strings.Contains(err.Error(), "-device") {
		t.Errorf("pick ambiguous: %v, want an error asking for -device", err)
	}
	other.Connected = false
	if got, err := pickIOSDevice(iOS, []iosDevice{oneo, other}, ""); err != nil || got != oneo {
		t.Errorf("pick the connected device = %+v, %v", got, err)
	}
	if got, err := pickIOSDevice(iOS, []iosDevice{oneo, other}, "Other"); err != nil || got != other {
		t.Errorf("pick Other = %+v, %v", got, err)
	}
	if _, err := pickIOSDevice(iOS, []iosDevice{off}, ""); err == nil || !strings.Contains(err.Error(), "Developer Mode") {
		t.Errorf("pick developer mode off: %v, want an error about Developer Mode", err)
	}
	if _, err := pickIOSDevice(iOS, []iosDevice{{Name: "Unpaired", Platform: "iOS"}}, ""); err == nil || !strings.Contains(err.Error(), "no iPhone") {
		t.Errorf("pick unpaired: %v, want an error about no iPhone", err)
	}
}

func TestIOSProject(t *testing.T) {
	var b strings.Builder
	p := &project{Name: "Gallery", ID: "dev.oneo.nsgo.gallery"}
	err := iosProject.Execute(&b, iosProjectData(p, iOS, "TEAM123"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`PRODUCT_NAME = "Gallery";`,
		`PRODUCT_BUNDLE_IDENTIFIER = "dev.oneo.nsgo.gallery";`,
		`DEVELOPMENT_TEAM = "TEAM123";`,
		`path = "Gallery.app";`,
		`shellScript = "set -e\n`,
		`SDKROOT = iphoneos;`,
		`IPHONEOS_DEPLOYMENT_TARGET = 15.0;`,
		`TARGETED_DEVICE_FAMILY = "1,2";`,
	} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("project does not contain %q", want)
		}
	}
}

// TestIOSWidgetsProject gives an app with widgets or Live Activities an
// extension target that Xcode signs and embeds, where an App Clip and an
// Apple TV get none.
func TestIOSWidgetsProject(t *testing.T) {
	p := &project{Name: "Gallery", ID: "dev.oneo.nsgo.gallery", LiveActivities: liveActivities{Enabled: true}}
	var b strings.Builder
	if err := iosProject.Execute(&b, iosProjectData(p, iOS, "TEAM123")); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`productType = "com.apple.product-type.app-extension";`,
		`PRODUCT_BUNDLE_IDENTIFIER = "dev.oneo.nsgo.gallery.widgets";`,
		`CODE_SIGN_ENTITLEMENTS = Widgets.entitlements;`,
		`dstSubfolderSpec = 13;`,
		`AA0000000000000000000019 /* PBXTargetDependency */,`,
		`unsigned/PlugIns/Widgets.appex`,
	} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("project does not contain %q", want)
		}
	}
	for _, sys := range []*appleOS{tvOS} {
		b.Reset()
		if err := iosProject.Execute(&b, iosProjectData(p, sys, "TEAM123")); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(b.String(), "app-extension") {
			t.Errorf("%s's project has the extension", sys.name)
		}
	}
	b.Reset()
	if err := iosProject.Execute(&b, iosProjectData(&project{Name: "Plain", ID: "dev.example.plain"}, iOS, "TEAM123")); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(b.String(), "Widgets") {
		t.Error("an app without widgets has the extension")
	}
}

func TestTVOSProject(t *testing.T) {
	var b strings.Builder
	p := &project{Name: "Gallery", ID: "dev.oneo.nsgo.gallery"}
	if err := iosProject.Execute(&b, iosProjectData(p, tvOS, "TEAM123")); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`SDKROOT = appletvos;`, `TVOS_DEPLOYMENT_TARGET = 17.0;`, `TARGETED_DEVICE_FAMILY = "3";`} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("project does not contain %q", want)
		}
	}
	if strings.Contains(b.String(), "iphoneos") {
		t.Error("the Apple TV's project names the iPhone's SDK")
	}
}

func TestAppleOSOf(t *testing.T) {
	for target, want := range map[string]struct {
		sys       *appleOS
		simulator bool
		sdk       string
		triple    string
	}{
		"ios":            {iOS, false, "iphoneos", "arm64-apple-ios15.0"},
		"ios_simulator":  {iOS, true, "iphonesimulator", "arm64-apple-ios15.0-simulator"},
		"tvos":           {tvOS, false, "appletvos", "arm64-apple-tvos17.0"},
		"tvos_simulator": {tvOS, true, "appletvsimulator", "arm64-apple-tvos17.0-simulator"},
	} {
		sys, simulator, ok := appleOSOf(target)
		if !ok || sys != want.sys || simulator != want.simulator {
			t.Errorf("appleOSOf(%q) = %v, %v, %v", target, sys, simulator, ok)
			continue
		}
		if got := sys.sdkName(simulator); got != want.sdk {
			t.Errorf("%s's SDK = %s, want %s", target, got, want.sdk)
		}
		if got := sys.triple(simulator); got != want.triple {
			t.Errorf("%s's triple = %s, want %s", target, got, want.triple)
		}
	}
	if _, _, ok := appleOSOf("macos_arm64"); ok {
		t.Error("macos_arm64 is an iOS or tvOS target")
	}
}

func TestPickDeviceOfItsOS(t *testing.T) {
	devices, err := parseIOSDevices([]byte(`{"result": {"devices": [
		{"deviceProperties": {"name": "ONEO", "developerModeStatus": "enabled"},
		 "hardwareProperties": {"reality": "physical", "platform": "iOS", "udid": "PHONE"},
		 "connectionProperties": {"pairingState": "paired", "tunnelState": "connected"}},
		{"deviceProperties": {"name": "Living Room", "developerModeStatus": "enabled"},
		 "hardwareProperties": {"reality": "physical", "platform": "tvOS", "udid": "TV"},
		 "connectionProperties": {"pairingState": "paired", "tunnelState": "connected"}}]}}`))
	if err != nil {
		t.Fatal(err)
	}
	if d, err := pickIOSDevice(iOS, devices, ""); err != nil || d.UDID != "PHONE" {
		t.Errorf("pick an iPhone = %+v, %v", d, err)
	}
	if d, err := pickIOSDevice(tvOS, devices, ""); err != nil || d.UDID != "TV" {
		t.Errorf("pick an Apple TV = %+v, %v", d, err)
	}
	if _, err := pickIOSDevice(tvOS, devices, "ONEO"); err == nil {
		t.Error("an iPhone was picked as an Apple TV")
	}
	if _, err := pickIOSDevice(tvOS, devices[:1], ""); err == nil || !strings.Contains(err.Error(), "no Apple TV") {
		t.Errorf("pick with no Apple TV: %v, want an error about no Apple TV", err)
	}
}

func TestPBXString(t *testing.T) {
	if got, want := pbxString("a\"b\\c\nd"), `"a\"b\\c\nd"`; got != want {
		t.Errorf("pbxString = %s, want %s", got, want)
	}
}
