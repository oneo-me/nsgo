package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/oneo-me/nsgo/tsx/pkg"
)

// nsgo.yaml's updates have the app update the way its platform and the
// channel it is handed out through do (package updater). On the direct
// channel, for each macOS, Windows, Linux and Android target it releases,
// nsgo build writes appcast-<target>.xml into the releases directory: an
// RSS feed in Sparkle's format whose one item is this version's
// installer, signed with the updates key. macOS packages carry
// Sparkle.framework and Windows ones WinSparkle.dll, which read that feed
// and install; Linux and Android read it themselves. iOS asks the App
// Store on every channel but none.

// The channels updates.channel names: the app's own downloads, which
// update themselves; a store, or a package manager, that updates them;
// and none, which checks nothing.
const (
	channelDirect = "direct"
	channelStore  = "store"
	channelNone   = "none"
)

// updatesConfig is nsgo.yaml's updates.
type updatesConfig struct {
	// Key is the public key nsgo keygen made (updates.pub), whose secret
	// key signs each release's installers: NSGO_UPDATES_KEY's, else the
	// .key file beside it.
	Key string `yaml:"key"`
	// Feed is the URL the release's appcasts and installers are under; by
	// default the GitHub repository's latest release (release.github).
	Feed string `yaml:"feed"`
	// Automatic has the app check on its own, daily: on Sparkle's and
	// WinSparkle's schedule, and as it starts elsewhere; true by default.
	// The user's choice in Sparkle's and WinSparkle's settings wins.
	Automatic *bool `yaml:"automatic"`
	// Channel is what the builds are handed out through, which build's
	// -channel replaces: direct, store or none; by default direct with a
	// key and none without.
	Channel string `yaml:"channel"`

	// public is Key's key, and repo the GitHub repository the feed is in
	// without a Feed of its own.
	public ed25519.PublicKey
	repo   string
}

// validateUpdates reads updates' key and makes its feed whole.
func (p *project) validateUpdates() error {
	u := &p.Updates
	if err := checkChannel(u.Channel); err != nil {
		return err
	}
	if u.Key == "" {
		if u.Channel == "" && (u.Feed != "" || u.Automatic != nil) {
			return errors.New("updates: key names no public key; make one with nsgo keygen updates, or set channel to store")
		}
		return nil
	}
	u.Key = p.resolve(u.Key)
	key, err := readPublicKey(u.Key)
	if err != nil {
		return fmt.Errorf("updates: key: %w", err)
	}
	u.public = key
	if u.Feed == "" {
		u.repo = p.githubRepository()
		return nil
	}
	f, err := url.Parse(u.Feed)
	if err != nil || f.Scheme != "https" || f.Host == "" || strings.ContainsAny(u.Feed, "'\"") {
		return fmt.Errorf("updates: feed %q is not an https URL", u.Feed)
	}
	if !strings.HasSuffix(u.Feed, "/") {
		u.Feed += "/"
	}
	return nil
}

// checkChannel refuses a channel updates.channel or -channel cannot name.
func checkChannel(channel string) error {
	switch channel {
	case "", channelDirect, channelStore, channelNone:
		return nil
	}
	return fmt.Errorf("updates: channel %q is not direct, store or none", channel)
}

// channel is the channel the builds are handed out through.
func (p *project) channel() string {
	switch {
	case p.Updates.Channel != "":
		return p.Updates.Channel
	case p.Updates.Key != "":
		return channelDirect
	}
	return channelNone
}

// updatesOn reports whether the app's builds check for updates: on a
// channel other than none, and no development build, which never does.
func (p *project) updatesOn() bool { return p.channel() != channelNone && !p.Dev.Client }

// updates reports whether the app's builds update themselves, on the
// direct channel: Sparkle and WinSparkle, and the appcasts.
func (p *project) updates() bool { return p.updatesOn() && p.channel() == channelDirect }

// updatesAutomatic reports whether the app checks on its own.
func (p *project) updatesAutomatic() bool { return p.Updates.Automatic == nil || *p.Updates.Automatic }

// feedTarget reports whether target's releases have an appcast.
func feedTarget(target string) bool {
	for _, prefix := range []string{"macos_", "windows_", "linux_", "android_"} {
		if strings.HasPrefix(target, prefix) {
			return true
		}
	}
	return false
}

// appcastName is target's appcast's file name.
func appcastName(target string) string { return "appcast-" + target + ".xml" }

// feedBase is the URL the release's files are under, "" when neither
// updates.feed nor a GitHub repository names one.
func (p *project) feedBase() string {
	if p.Updates.Feed != "" {
		return p.Updates.Feed
	}
	if p.Updates.repo != "" {
		return "https://github.com/" + p.Updates.repo + "/releases/latest/download/"
	}
	return ""
}

// appcastURL is target's appcast, which its builds read; "" for a target
// with none.
func (p *project) appcastURL(target string) string {
	if base := p.feedBase(); base != "" && feedTarget(target) {
		return base + appcastName(target)
	}
	return ""
}

// checkUpdateFeed refuses to build target's updating package without a
// channel nsgo knows, the key the direct one signs with, or a feed for it
// to read.
func (p *project) checkUpdateFeed(target string) error {
	if err := checkChannel(p.Updates.Channel); err != nil {
		return err
	}
	if p.updates() && p.Updates.public == nil {
		return errors.New("updates: the direct channel signs its installers: set updates.key to the public key nsgo keygen updates makes")
	}
	if p.updates() && feedTarget(target) && p.feedBase() == "" {
		return errors.New("updates: no feed for the appcasts: set updates.feed, or release.github to the repository whose releases hold them")
	}
	return nil
}

// downloadURL is where a release file of this version is: under the feed,
// or in this version's GitHub release.
func (p *project) downloadURL(file string) string {
	if p.Updates.Feed != "" {
		return p.Updates.Feed + url.PathEscape(file)
	}
	return "https://github.com/" + p.Updates.repo + "/releases/download/" + url.PathEscape(p.tagPrefix()+p.Version) + "/" + url.PathEscape(file)
}

// releasePage is this version's GitHub release, "" with a feed of its own.
func (p *project) releasePage() string {
	if p.Updates.Feed != "" || p.Updates.repo == "" {
		return ""
	}
	return "https://github.com/" + p.Updates.repo + "/releases/tag/" + url.PathEscape(p.tagPrefix()+p.Version)
}

// envTarget names the target a go build's environment builds for.
func envTarget(env []string) string {
	goos, goarch := runtime.GOOS, runtime.GOARCH
	for _, e := range env {
		if v, ok := strings.CutPrefix(e, "GOOS="); ok {
			goos = v
		}
		if v, ok := strings.CutPrefix(e, "GOARCH="); ok {
			goarch = v
		}
	}
	if goos == "darwin" {
		goos = "macos"
	}
	return goos + "_" + goArchName(goarch)
}

// updateLdflags give nsgo.Run the updates of a build for env's target:
// on the direct channel its appcast and key, where the backend reads one
// rather than Sparkle, and whether it checks on its own.
func (p *project) updateLdflags(env []string) []string {
	if !p.updatesOn() {
		return nil
	}
	const pkgPath = "github.com/oneo-me/nsgo"
	var flags []string
	if feed := p.appcastURL(envTarget(env)); feed != "" && p.updates() {
		flags = append(flags, fmt.Sprintf("-X '%s.updateFeed=%s'", pkgPath, feed),
			fmt.Sprintf("-X %s.updateKey=%s", pkgPath, pkg.EncodePublicKey(p.Updates.public)))
	}
	if p.updatesAutomatic() {
		flags = append(flags, fmt.Sprintf("-X %s.updateAutomatic=1", pkgPath))
	}
	return flags
}

// sparklePlist is what a macOS app's Info.plist tells Sparkle: its
// appcast, the key its downloads are signed with and whether it checks on
// its own. A user's choice in Sparkle's settings wins.
func (p *project) sparklePlist(target string) string {
	if !p.updates() {
		return ""
	}
	automatic := "<false/>"
	if p.updatesAutomatic() {
		automatic = "<true/>"
	}
	return fmt.Sprintf("\t<key>SUFeedURL</key><string>%s</string>\n\t<key>SUPublicEDKey</key><string>%s</string>\n\t<key>SUEnableAutomaticChecks</key>%s",
		xmlText(p.appcastURL(target)), pkg.EncodePublicKey(p.Updates.public), automatic)
}

// pinned is a release of an updater nsgo bundles, by its SHA-256.
type pinned struct{ name, url, sha256 string }

// The updaters nsgo bundles: Sparkle in macOS packages and WinSparkle in
// Windows ones, under their MIT licenses.
var (
	sparkleRelease = pinned{"Sparkle-2.10.1.tar.xz",
		"https://github.com/sparkle-project/Sparkle/releases/download/2.10.1/Sparkle-2.10.1.tar.xz",
		"1a856d2a9edb8e628de88121d7be04cf0e2167f8641650b78fca00b14244cf27"}
	winSparkleRelease = pinned{"WinSparkle-0.9.4.zip",
		"https://github.com/vslavik/winsparkle/releases/download/v0.9.4/WinSparkle-0.9.4.zip",
		"6037df37fc263bd1650a1c4949681a9d40ffe991d01f35892a406cb5d103c976"}
)

// The targets build at once (buildPackages), so one at a time downloads
// the updaters and unpacks Sparkle into the user's cache.
var fetching, unpacking sync.Mutex

// fetch returns the release's path in the user's cache, downloading it
// the first time.
func (r pinned) fetch(ctx context.Context) (string, error) {
	fetching.Lock()
	defer fetching.Unlock()
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(cache, "nsgo", "downloads")
	path := filepath.Join(dir, r.name)
	if data, err := os.ReadFile(path); err == nil && r.matches(data) {
		return path, nil
	}
	stage(ctx, "downloading %s", r.name)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.url, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s answered %s", r.url, resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if !r.matches(data) {
		return "", fmt.Errorf("%s does not have the SHA-256 nsgo pins", r.url)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(dir, r.name+".*")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	return path, os.Rename(tmp.Name(), path)
}

func (r pinned) matches(data []byte) bool {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]) == r.sha256
}

// sparkleFramework returns Sparkle.framework, unpacked once from its
// release in the user's cache. Unpacking takes macOS's tar, as the
// framework's links and signatures must stay whole.
func sparkleFramework(ctx context.Context) (string, error) {
	archive, err := sparkleRelease.fetch(ctx)
	if err != nil {
		return "", err
	}
	unpacking.Lock()
	defer unpacking.Unlock()
	dir := strings.TrimSuffix(archive, ".tar.xz")
	framework := filepath.Join(dir, "Sparkle.framework")
	if _, err := os.Stat(filepath.Join(framework, "Sparkle")); err == nil {
		return framework, nil
	}
	tmp, err := os.MkdirTemp(filepath.Dir(archive), "sparkle-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	if err := run(ctx, "tar", "-xJf", archive, "-C", tmp, "./Sparkle.framework"); err != nil {
		return "", err
	}
	if err := os.RemoveAll(dir); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, dir); err != nil {
		// Another nsgo unpacked it meanwhile.
		if _, statErr := os.Stat(filepath.Join(framework, "Sparkle")); statErr == nil {
			return framework, nil
		}
		return "", err
	}
	return framework, nil
}

// addSparkle puts Sparkle.framework into an updating macOS app's
// Frameworks, signed ad hoc until signMacApp signs it with the app's
// identity.
func addSparkle(ctx context.Context, p *project, app string) error {
	if !p.updates() {
		return nil
	}
	framework, err := sparkleFramework(ctx)
	if err != nil {
		return fmt.Errorf("updates: Sparkle: %w", err)
	}
	dst := filepath.Join(app, "Contents", "Frameworks", "Sparkle.framework")
	if err := os.RemoveAll(dst); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := run(ctx, "ditto", framework, dst); err != nil {
		return err
	}
	return signSparkle(ctx, app, func(path string) []string { return []string{"--force", "--sign", "-", path} })
}

// signSparkle signs the Sparkle.framework in app, if any, with args'
// codesign arguments for each path: its helpers inside out, then the
// framework, as Sparkle's documentation orders them, the downloader
// keeping its entitlements.
func signSparkle(ctx context.Context, app string, args func(path string) []string) error {
	framework := filepath.Join(app, "Contents", "Frameworks", "Sparkle.framework")
	if _, err := os.Stat(framework); err != nil {
		return nil
	}
	version, err := filepath.EvalSymlinks(filepath.Join(framework, "Versions", "Current"))
	if err != nil {
		return err
	}
	for _, part := range []string{"XPCServices/Installer.xpc", "XPCServices/Downloader.xpc", "Autoupdate", "Updater.app"} {
		path := filepath.Join(version, filepath.FromSlash(part))
		if _, err := os.Stat(path); err != nil {
			continue
		}
		a := args(path)
		if part == "XPCServices/Downloader.xpc" {
			a = append(a[:len(a)-1:len(a)-1], "--preserve-metadata=entitlements", path)
		}
		if err := run(ctx, "codesign", a...); err != nil {
			return err
		}
	}
	return run(ctx, "codesign", args(framework)...)
}

// addWinSparkle puts WinSparkle.dll for arch, Go's, beside an updating
// Windows app's executable, from its release in the user's cache.
func addWinSparkle(ctx context.Context, p *project, dir, arch string) error {
	if !p.updates() {
		return nil
	}
	archive, err := winSparkleRelease.fetch(ctx)
	if err != nil {
		return fmt.Errorf("updates: WinSparkle: %w", err)
	}
	name := strings.TrimSuffix(winSparkleRelease.name, ".zip") + "/" + map[string]string{"amd64": "x64", "arm64": "ARM64"}[arch] + "/Release/WinSparkle.dll"
	z, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer z.Close()
	f, err := z.Open(name)
	if err != nil {
		return fmt.Errorf("updates: %s holds no %s", winSparkleRelease.name, name)
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "WinSparkle.dll"), data, 0o644)
}

// updatesSecretKey is the secret key a release's installers are signed
// with: NSGO_UPDATES_KEY's, else the .key file beside updates.key's
// public key; nil without either. One of another public key is refused.
func (p *project) updatesSecretKey() (ed25519.PrivateKey, error) {
	var key ed25519.PrivateKey
	if text := os.Getenv("NSGO_UPDATES_KEY"); text != "" {
		k, err := pkg.DecodeSecretKey(text)
		if err != nil {
			return nil, fmt.Errorf("NSGO_UPDATES_KEY: %w", err)
		}
		key = k
	} else {
		path := strings.TrimSuffix(p.Updates.Key, filepath.Ext(p.Updates.Key)) + ".key"
		if _, err := os.Stat(path); err != nil {
			return nil, nil
		}
		k, err := readSecretKey(path)
		if err != nil {
			return nil, err
		}
		key = k
	}
	if !key.Public().(ed25519.PublicKey).Equal(p.Updates.public) {
		return nil, fmt.Errorf("updates: the secret key is not the one of %s", rel(p.Updates.Key))
	}
	return key, nil
}

// releaseInstaller moves target's installer, if any, into the releases
// directory with its checksum, and writes the appcast offering it.
func (p *project) releaseInstaller(target, path string) error {
	if err := p.releaseAssets(path); err != nil || path == "" {
		return err
	}
	return p.writeAppcast(target, filepath.Join(p.releaseDir(), filepath.Base(path)))
}

// writeAppcast writes target's appcast into the releases directory, its
// one item installer, a release file. macOS and Windows install only a
// signed installer, so without the secret key their appcast is left out.
func (p *project) writeAppcast(target, installer string) error {
	if !p.updates() || installer == "" || !feedTarget(target) {
		return nil
	}
	data, err := os.ReadFile(installer)
	if err != nil {
		return err
	}
	key, err := p.updatesSecretKey()
	if err != nil {
		return err
	}
	system, _, _ := strings.Cut(target, "_")
	if key == nil && (system == "macos" || system == "windows") {
		warn("no %s: sign the installers with updates.key's secret key, in NSGO_UPDATES_KEY or beside it", appcastName(target))
		return nil
	}
	name, err := p.displayName()
	if err != nil {
		return err
	}
	item := appcastItem{name: name, version: p.Version, build: p.BundleVersion(), page: p.releasePage(),
		url: p.downloadURL(filepath.Base(installer)), os: system, length: len(data), published: time.Now()}
	if key != nil {
		item.signature = base64.StdEncoding.EncodeToString(ed25519.Sign(key, data))
	}
	if system == "windows" {
		// The installer quits the app, installs quietly and starts the new
		// version (nsis.go).
		item.arguments = "/S /RELAUNCH"
	}
	path := filepath.Join(p.releaseDir(), appcastName(target))
	if err := writeFile(path, item.feed()); err != nil {
		return err
	}
	return p.releaseAssets(path)
}

// appcastItem is the one release an appcast lists.
type appcastItem struct {
	name, version, build string
	// page is the release's page, url its installer's download and os
	// Sparkle's name of its system.
	page, url, os string
	length        int
	// signature is the installer's EdDSA signature, in base64; "" unsigned.
	signature string
	// arguments are what WinSparkle runs the installer with.
	arguments string
	published time.Time
}

// feed is the appcast listing it. Its versions are elements and the
// enclosure's attributes both, which Sparkle's older readers take.
func (it appcastItem) feed() []byte {
	var b bytes.Buffer
	b.WriteString("<?xml version=\"1.0\" encoding=\"utf-8\"?>\n")
	b.WriteString("<rss version=\"2.0\" xmlns:sparkle=\"http://www.andymatuschak.org/xml-namespaces/sparkle\">\n  <channel>\n")
	fmt.Fprintf(&b, "    <title>%s</title>\n", xmlText(it.name))
	b.WriteString("    <item>\n")
	fmt.Fprintf(&b, "      <title>%s</title>\n", xmlText(it.name+" "+it.version))
	if it.page != "" {
		fmt.Fprintf(&b, "      <link>%s</link>\n", xmlText(it.page))
	}
	fmt.Fprintf(&b, "      <pubDate>%s</pubDate>\n", it.published.UTC().Format(time.RFC1123Z))
	fmt.Fprintf(&b, "      <sparkle:version>%s</sparkle:version>\n", xmlText(it.build))
	fmt.Fprintf(&b, "      <sparkle:shortVersionString>%s</sparkle:shortVersionString>\n", xmlText(it.version))
	fmt.Fprintf(&b, "      <enclosure url=\"%s\" length=\"%d\" type=\"application/octet-stream\" sparkle:os=\"%s\" sparkle:version=\"%s\" sparkle:shortVersionString=\"%s\"",
		xmlText(it.url), it.length, xmlText(it.os), xmlText(it.build), xmlText(it.version))
	if it.signature != "" {
		fmt.Fprintf(&b, " sparkle:edSignature=\"%s\"", it.signature)
	}
	if it.arguments != "" {
		fmt.Fprintf(&b, " sparkle:installerArguments=\"%s\"", xmlText(it.arguments))
	}
	b.WriteString("/>\n    </item>\n  </channel>\n</rss>\n")
	return b.Bytes()
}
