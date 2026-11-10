package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"rsc.io/qr"
)

// An App Clip is a small part of an iOS app that runs without being
// installed, from a link, a code or an NFC tag. It is an app of its own,
// <id>.Clip by default, nested in the app's bundle under AppClips/, which
// the App Store hands out alone. nsgo builds it from nsgo.yaml's
// app_clip: a main package of its own, so it links only what it shows and
// stays within the size the system downloads clips at, or the app's own
// package, which tells the two apart with mobile.IsAppClip.

// appClip is nsgo.yaml's app_clip.
type appClip struct {
	// Name is what the clip is called, "<name> Clip" by default.
	Name string `yaml:"name"`
	// ID is its bundle ID, under the app's: <id>.Clip by default.
	ID string `yaml:"id"`
	// Main is its main package, relative to nsgo.yaml: the app's own by
	// default. Its resources/ are the clip's.
	Main string `yaml:"main"`
	// Minimum is the iOS version it needs, 16.0 by default: from 17.0 a
	// clip may take 100 MB rather than 15, when it is opened only from
	// links, as Safari's and Messages', never from codes or tags.
	Minimum string `yaml:"minimum"`
	// Domains are the web domains whose links open it, which its
	// associated domains list as appclips:<domain>.
	Domains []string `yaml:"domains"`
	// Notifications and Location ask for what the clip's card offers:
	// notifications for a day after it opens, and confirming where the
	// user opened it.
	Notifications bool `yaml:"notifications"`
	Location      bool `yaml:"location"`
}

const (
	appClipsDir = "AppClips"
	// clipSize and clipDigitalSize are how large, uncompressed, a clip
	// may be: one opened from codes and tags, and one needing iOS 17
	// opened only from links.
	clipSize        = 15_000_000
	clipDigitalSize = 100_000_000
)

var domainPattern = regexp.MustCompile(`^(\*\.)?[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?(\.[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?)+$`)

// validateAppClip checks app_clip and fills in its defaults; it runs once
// the app's ID is known.
func (p *project) validateAppClip() error {
	c := p.AppClip
	if c == nil {
		return nil
	}
	if c.Name == "" {
		c.Name = p.Name + " Clip"
	}
	if c.ID == "" {
		c.ID = p.ID + ".Clip"
	}
	if !strings.HasPrefix(c.ID, p.ID+".") || !utiPattern.MatchString(c.ID) {
		return fmt.Errorf("app_clip.id %q is not under the app's ID, as %s.Clip", c.ID, p.ID)
	}
	if c.Main == "" {
		c.Main = "."
	}
	if c.Minimum == "" {
		c.Minimum = "16.0"
	}
	if major := c.major(); major < 16 {
		return fmt.Errorf("app_clip.minimum %q is not an iOS version from 16.0", c.Minimum)
	} else if !strings.Contains(c.Minimum, ".") {
		c.Minimum += ".0"
	}
	for i, d := range c.Domains {
		if !domainPattern.MatchString(d) {
			return fmt.Errorf("app_clip.domains[%d] %q is not a domain; write example.com, not https://example.com", i, d)
		}
	}
	return nil
}

// major is the major version of Minimum, 0 when it is none.
func (c *appClip) major() int {
	s, _, _ := strings.Cut(c.Minimum, ".")
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

// limit is how large the clip may be.
func (c *appClip) limit() int64 {
	if c.major() >= 17 {
		return clipDigitalSize
	}
	return clipSize
}

// clipProject is the project the App Clip builds as: the app's, under the
// clip's name, ID and main package, without what only the app has.
func (p *project) clipProject() *project {
	c := *p
	c.clipParent, c.parentClip = p.ID, p.AppClip
	c.Name, c.ID = p.AppClip.Name, p.AppClip.ID
	c.Dir = p.resolve(p.AppClip.Main)
	c.AppClip, c.Widgets, c.FileTypes = nil, nil, nil
	return &c
}

// IOSMinimum is MinimumOSVersion of p's iOS package.
func (p *project) IOSMinimum() string {
	if p.clipParent != "" {
		return p.parentClip.Minimum
	}
	return iOS.minimum
}

// clipPlist returns the entries of an App Clip's Info.plist, "" for an
// app.
func (p *project) clipPlist() string {
	if p.clipParent == "" {
		return ""
	}
	c := p.parentClip
	return "<key>NSAppClip</key><dict>" +
		"<key>NSAppClipRequestEphemeralUserNotification</key>" + plistBool(c.Notifications) +
		"<key>NSAppClipRequestLocationConfirmation</key>" + plistBool(c.Location) + "</dict>"
}

func plistBool(v bool) string {
	if v {
		return "<true/>"
	}
	return "<false/>"
}

// iosClipApp is the App Clip in an iOS bundle.
func iosClipApp(p *project, app string) string {
	return filepath.Join(app, appClipsDir, p.AppClip.Name+".app")
}

// addIOSAppClip builds the App Clip into an iOS bundle, signed ad hoc for
// the simulator as the app is; device builds sign it with the app
// (signIOSClip). A device build larger than the system downloads clips at
// fails, as the App Store would refuse it.
func addIOSAppClip(ctx context.Context, p *project, app string, simulator bool) error {
	if p.AppClip == nil {
		return nil
	}
	clip, err := buildIOSApp(ctx, p.clipProject(), iOS, filepath.Join(app, appClipsDir), simulator)
	if err != nil {
		return fmt.Errorf("the App Clip: %w", err)
	}
	size, limit := dirSize(clip), p.AppClip.limit()
	switch {
	case size > limit && simulator:
		warnIn(ctx, "the App Clip takes %s, more than the %s a device downloads it at", formatSize(size), formatSize(limit))
	case size > limit && limit == clipSize:
		return fmt.Errorf("the App Clip takes %s, more than the %s iOS 16 downloads it at: link less into %s, or set app_clip.minimum to 17.0 for %s when only links open it",
			formatSize(size), formatSize(limit), p.AppClip.Main, formatSize(clipDigitalSize))
	case size > limit:
		return fmt.Errorf("the App Clip takes %s, more than the %s iOS downloads it at", formatSize(size), formatSize(limit))
	case size > clipSize:
		detailIn(ctx, "the App Clip takes %s: only links open it, never codes or NFC tags", formatSize(size))
	}
	return nil
}

// simulatorEntitlements are what an iOS bundle's ad hoc signature claims
// for the simulator, which grants them from it: the app group the app
// shares with its widgets. The simulator refuses to launch an app whose ad
// hoc signature claims a developer entitlement, so the tie between an app
// and its clip is left to device builds' profiles.
func (p *project) simulatorEntitlements() map[string]any {
	if len(p.Widgets) == 0 {
		return nil
	}
	return p.groupEntitlements()
}

// clipDomains are the associated domains of a clip opened from domains.
func clipDomains(domains []string) []any {
	out := make([]any, len(domains))
	for i, d := range domains {
		out[i] = "appclips:" + d
	}
	return out
}

// signIOSClip signs an iOS device build's App Clip with
// sign.clip_profile, which it embeds and takes the entitlements from,
// with its frameworks, before the app is signed.
func signIOSClip(ctx context.Context, p *project, app, identity string) error {
	if p.AppClip == nil {
		return nil
	}
	clip := iosClipApp(p, app)
	id := p.AppClip.ID
	if p.Sign.ClipProfile == "" {
		return fmt.Errorf("the App Clip %s needs sign.clip_profile, its own provisioning profile", id)
	}
	data, err := os.ReadFile(p.Sign.ClipProfile)
	if err != nil {
		return err
	}
	profile, err := readProfile(ctx, data)
	if err != nil {
		return fmt.Errorf("%s: %w", rel(p.Sign.ClipProfile), err)
	}
	entitlements, ok := profile["Entitlements"].(map[string]any)
	if !ok {
		return fmt.Errorf("%s has no entitlements", rel(p.Sign.ClipProfile))
	}
	appID, _ := entitlements["application-identifier"].(string)
	if !profileAllows(appID, id) {
		return fmt.Errorf("sign.clip_profile is for %s, not %s", appID, id)
	}
	entitlements["application-identifier"] = signedAppID(appID, id)
	if _, ok := entitlements["com.apple.developer.parent-application-identifiers"]; !ok {
		return fmt.Errorf("sign.clip_profile is not an App Clip's: its App ID must be the clip of %s", p.ID)
	}
	if domains := p.AppClip.Domains; len(domains) > 0 {
		if _, ok := entitlements["com.apple.developer.associated-domains"]; !ok {
			return fmt.Errorf("sign.clip_profile does not grant Associated Domains, which app_clip.domains needs")
		}
		entitlements["com.apple.developer.associated-domains"] = clipDomains(domains)
	} else {
		delete(entitlements, "com.apple.developer.associated-domains")
	}
	if err := os.WriteFile(filepath.Join(clip, "embedded.mobileprovision"), data, 0o644); err != nil {
		return err
	}
	ents, err := writeEntitlements(entitlements)
	if err != nil {
		return err
	}
	defer os.Remove(ents)
	if err := signFrameworks(ctx, clip, identity); err != nil {
		return err
	}
	return run(ctx, "codesign", "--force", "--sign", identity, "--entitlements", ents, "--generate-entitlement-der", clip)
}

// startIOSClip installs the App Clip of the app built at app on the
// simulator udid alone, as the system installs it from a link, and
// launches it opened from url, which Xcode's _XCAppClipURL passes it, and
// connected to the dev server at server.
func startIOSClip(ctx context.Context, p *project, udid, app, url, server string) (stop func(), wait func(context.Context) error, err error) {
	if err := run(ctx, "xcrun", "simctl", "install", udid, iosClipApp(p, app)); err != nil {
		return nil, nil, err
	}
	event("open the App Clip from %s", url)
	return startProcessEnv([]string{"SIMCTL_CHILD__XCAppClipURL=" + url, "SIMCTL_CHILD_NSGO_DEV_SERVER=" + server},
		"xcrun", "simctl", "launch", "--console-pty", "--terminate-running-process", udid, p.AppClip.ID)
}

// startIOSDeviceClip signs the App Clip of the app built at bundle for
// device with Xcode, working in work, installs it alone, as Xcode runs a
// clip's scheme, and launches it opened from url. Its link then opens it
// from codes too, once the device's Local Experiences list it.
func startIOSDeviceClip(ctx context.Context, p *project, bundle, work, team string, device iosDevice, url string) (app, error) {
	if freeTeam(ctx, team) {
		return app{}, fmt.Errorf("the team %s is a personal team, which Apple lets sign no App Clip: pass -team with a team of the Apple Developer Program", team)
	}
	signed, err := signIOSApp(ctx, p.clipProject(), iOS, iosClipApp(p, bundle), work, team, device)
	if err != nil {
		return app{}, err
	}
	event("install the App Clip on %s", device.Name)
	if err := run(ctx, "xcrun", "devicectl", "device", "install", "app", "--device", device.UDID, signed); err != nil {
		return app{}, err
	}
	env, _ := json.Marshal(map[string]string{"_XCAppClipURL": url})
	event("open the App Clip from %s", url)
	if code := qrLines(url); code != nil && terminalOf().color {
		detail("with the link in Settings > Developer > Local Experiences, the camera opens the clip from this code:")
		for _, l := range code {
			line(l)
		}
	}
	stop, wait, err := startProcess("xcrun", "devicectl", "device", "process", "launch", "--console", "--terminate-existing",
		"--environment-variables", string(env), "--device", device.UDID, p.AppClip.ID)
	if err != nil {
		return app{}, err
	}
	return app{stop: stop, wait: wait}, nil
}

// freeTeam reports whether Xcode knows team as a personal team, which
// signs no App Clip or associated domains.
func freeTeam(ctx context.Context, team string) bool {
	out, err := exec.CommandContext(ctx, "defaults", "export", "com.apple.dt.Xcode", "-").Output()
	if err != nil {
		return false
	}
	return xcodeFreeTeam(out, team)
}

// xcodeFreeTeam reports whether Xcode's preferences, as defaults exports
// them, list team as a personal team.
func xcodeFreeTeam(prefs []byte, team string) bool {
	v, err := decodePlist(prefs)
	if err != nil {
		return false
	}
	root, _ := v.(map[string]any)
	accounts, _ := root["IDEProvisioningTeamByIdentifier"].(map[string]any)
	for _, teams := range accounts {
		list, _ := teams.([]any)
		for _, t := range list {
			d, _ := t.(map[string]any)
			if d["teamID"] == team {
				free, _ := d["isFreeProvisioningTeam"].(bool)
				return free
			}
		}
	}
	return false
}

// qrLines draws a QR code of text for the terminal, two of its rows to a
// line of half blocks in black and white, whatever the terminal's colors,
// within the quiet zone scanners need.
func qrLines(text string) []string {
	code, err := qr.Encode(text, qr.M)
	if err != nil {
		return nil
	}
	const quiet = 4
	var lines []string
	for y := -quiet; y < code.Size+quiet; y += 2 {
		var b strings.Builder
		for x := -quiet; x < code.Size+quiet; x++ {
			// The upper half block takes the top module's color, its
			// background the bottom's.
			fg, bg := "97", "107"
			if code.Black(x, y) {
				fg = "30"
			}
			if code.Black(x, y+1) {
				bg = "40"
			}
			fmt.Fprintf(&b, "\x1b[%s;%sm▀", fg, bg)
		}
		b.WriteString("\x1b[0m")
		lines = append(lines, b.String())
	}
	return lines
}
