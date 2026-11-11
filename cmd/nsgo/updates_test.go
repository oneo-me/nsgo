package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/xml"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/oneo-me/nsgo/internal/platform/updates"
	"github.com/oneo-me/nsgo/tsx/pkg"
)

// updatingProject loads an app whose nsgo.yaml adds yaml to updates with
// a key nsgo keygen would make, and returns its secret key.
func updatingProject(t *testing.T, yaml string) (*project, ed25519.PrivateKey) {
	t.Helper()
	dir := t.TempDir()
	key, err := pkg.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	writeFiles(t, dir, map[string]string{
		"updates.pub": pkg.EncodePublicKey(key.Public().(ed25519.PublicKey)) + "\n",
		"nsgo.yaml":   "name: Hello\nid: dev.example.hello\nversion: 1.2.0\nbuild_number: 12\nrelease: {github: example/hello}\nupdates: {key: updates.pub" + yaml + "}\n",
	})
	p, err := load(dir)
	if err != nil {
		t.Fatal(err)
	}
	return p, key
}

func TestUpdatesConfig(t *testing.T) {
	for _, c := range []struct {
		yaml, err string
	}{
		{"updates: {feed: https://example.com/hello}", "key names no public key"},
		{"updates: {automatic: false}", "key names no public key"},
		{"updates: {key: missing.pub}", "updates: key"},
	} {
		dir := t.TempDir()
		writeFiles(t, dir, map[string]string{"nsgo.yaml": "name: app\nid: dev.example.app\n" + c.yaml + "\n"})
		if _, err := load(dir); err == nil || !strings.Contains(err.Error(), c.err) {
			t.Errorf("%s: %v, want %q", c.yaml, err, c.err)
		}
	}
	p, _ := updatingProject(t, "")
	if got := p.appcastURL("macos_arm64"); got != "https://github.com/example/hello/releases/latest/download/appcast-macos_arm64.xml" {
		t.Errorf("appcast %s", got)
	}
	if got := p.appcastURL("ios"); got != "" {
		t.Errorf("iOS, which the App Store updates, reads appcast %s", got)
	}
	if got := p.downloadURL("hello-1.2.0-arm64.dmg"); got != "https://github.com/example/hello/releases/download/v1.2.0/hello-1.2.0-arm64.dmg" {
		t.Errorf("download %s", got)
	}
	if !p.updatesAutomatic() {
		t.Error("updates are not automatic by default")
	}
	p, _ = updatingProject(t, ", feed: 'https://example.com/hello', automatic: false")
	if got := p.appcastURL("windows_x64"); got != "https://example.com/hello/appcast-windows_x64.xml" {
		t.Errorf("appcast under a feed of its own %s", got)
	}
	if got := p.releasePage(); got != "" {
		t.Errorf("a feed of its own has the release page %s", got)
	}
	if p.updatesAutomatic() {
		t.Error("automatic: false checks on its own")
	}
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"k.pub": pkg.EncodePublicKey(make(ed25519.PublicKey, 32)) + "\n",
		"nsgo.yaml": "name: app\nid: dev.example.app\nupdates: {key: k.pub, feed: 'http://example.com'}\n"})
	if _, err := load(dir); err == nil || !strings.Contains(err.Error(), "not an https URL") {
		t.Errorf("an http feed: %v", err)
	}
}

func TestUpdateBuildSettings(t *testing.T) {
	p, key := updatingProject(t, ", automatic: false")
	pub := pkg.EncodePublicKey(key.Public().(ed25519.PublicKey))
	flags := strings.Join(p.updateLdflags([]string{"GOOS=windows", "GOARCH=amd64"}), " ")
	for _, want := range []string{"nsgo.version=1.2.0'", "nsgo.build=12'",
		"nsgo.updateFeed=https://github.com/example/hello/releases/latest/download/appcast-windows_x64.xml'", "nsgo.updateKey=" + pub} {
		if !strings.Contains(flags, want) {
			t.Errorf("ldflags %s lack %s", flags, want)
		}
	}
	if strings.Contains(flags, "updateAutomatic") {
		t.Errorf("automatic: false sets %s", flags)
	}
	if flags := strings.Join(p.updateLdflags([]string{"GOOS=ios", "GOARCH=arm64"}), " "); strings.Contains(flags, "updateFeed") {
		t.Errorf("iOS's flags %s name an appcast", flags)
	}
	plist := p.sparklePlist("macos_arm64")
	for _, want := range []string{"<key>SUFeedURL</key><string>https://github.com/example/hello/releases/latest/download/appcast-macos_arm64.xml</string>",
		"<key>SUPublicEDKey</key><string>" + pub + "</string>", "<key>SUEnableAutomaticChecks</key><false/>"} {
		if !strings.Contains(plist, want) {
			t.Errorf("Info.plist's %s lacks %s", plist, want)
		}
	}
	// Development builds never update.
	p.Dev.Client = true
	if p.updateLdflags(nil) != nil || p.sparklePlist("macos_arm64") != "" {
		t.Error("a development build updates")
	}
}

func TestWriteAppcast(t *testing.T) {
	t.Setenv("NSGO_UPDATES_KEY", "")
	p, key := updatingProject(t, "")
	if err := p.cleanReleaseDir(); err != nil {
		t.Fatal(err)
	}
	setup := filepath.Join(t.TempDir(), "hello-1.2.0-x64-setup.exe")
	writeFiles(t, filepath.Dir(setup), map[string]string{filepath.Base(setup): "installer"})
	// Without the secret key, Windows gets no appcast it would refuse.
	if err := p.releaseInstaller("windows_x64", setup); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(p.releaseDir(), "appcast-windows_x64.xml")); err == nil {
		t.Fatal("an appcast of an unsigned installer")
	}
	t.Setenv("NSGO_UPDATES_KEY", pkg.EncodeSecretKey(key))
	if err := p.writeAppcast("windows_x64", filepath.Join(p.releaseDir(), filepath.Base(setup))); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(p.releaseDir(), "appcast-windows_x64.xml"))
	if err != nil {
		t.Fatal(err)
	}
	// The backends' reader takes it.
	item, err := updates.ParseAppcast(data)
	want := updates.Item{Version: "1.2.0", Build: "12", Link: "https://github.com/example/hello/releases/tag/v1.2.0"}
	if err != nil || item != want {
		t.Fatalf("ParseAppcast = %+v, %v; want %+v", item, err, want)
	}
	var feed struct {
		Enclosure struct {
			URL       string `xml:"url,attr"`
			Length    int    `xml:"length,attr"`
			OS        string `xml:"http://www.andymatuschak.org/xml-namespaces/sparkle os,attr"`
			Signature string `xml:"http://www.andymatuschak.org/xml-namespaces/sparkle edSignature,attr"`
			Arguments string `xml:"http://www.andymatuschak.org/xml-namespaces/sparkle installerArguments,attr"`
		} `xml:"channel>item>enclosure"`
	}
	if err := xml.Unmarshal(data, &feed); err != nil {
		t.Fatal(err)
	}
	e := feed.Enclosure
	if e.URL != "https://github.com/example/hello/releases/download/v1.2.0/hello-1.2.0-x64-setup.exe" || e.Length != len("installer") ||
		e.OS != "windows" || e.Arguments != "/S /RELAUNCH" {
		t.Errorf("enclosure %+v", e)
	}
	sig, err := base64.StdEncoding.DecodeString(e.Signature)
	if err != nil || !ed25519.Verify(key.Public().(ed25519.PublicKey), []byte("installer"), sig) {
		t.Errorf("the signature %q does not verify the installer", e.Signature)
	}
	if !slices.Contains(p.released, filepath.Join(p.releaseDir(), "appcast-windows_x64.xml")) {
		t.Errorf("released %v", p.released)
	}
	// Another key's secret is refused.
	other, _ := pkg.GenerateKey()
	t.Setenv("NSGO_UPDATES_KEY", pkg.EncodeSecretKey(other))
	if err := p.writeAppcast("windows_x64", filepath.Join(p.releaseDir(), filepath.Base(setup))); err == nil {
		t.Error("another key signed the installer")
	}
}
