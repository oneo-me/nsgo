package main

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
)

// A store build of a macOS target is the Mac App Store's package,
// <slug>-<version>-macos-<arch>.pkg: the app in the App Sandbox, with
// sign.macos_profile embedded and its entitlements, signed with the
// profile's Apple Distribution identity, in an installer package
// productbuild, which ships with macOS, signs with the team's Mac
// Installer Distribution identity. Without a profile the app is signed ad
// hoc and the package unsigned, which runs sandboxed here but uploads
// nowhere.

// sandboxEntitlements are what a store build's app claims: the sandbox,
// with the network and the files the user picks, the microphone and the
// camera when apple.info explains why, and macos.entitlements on top.
func (p *project) sandboxEntitlements() map[string]any {
	ents := map[string]any{
		"com.apple.security.app-sandbox":                    true,
		"com.apple.security.network.client":                 true,
		"com.apple.security.files.user-selected.read-write": true,
	}
	maps.Copy(ents, p.hardenedEntitlements())
	maps.Copy(ents, p.MacOS.Entitlements)
	return ents
}

// buildMacStorePackage signs app for the Mac App Store and writes its
// installer package into dir, returning the package's path.
func buildMacStorePackage(ctx context.Context, p *project, app, dir, arch string) (string, error) {
	if len(p.Widgets) > 0 {
		return "", errors.New("the Mac App Store's sandboxed apps cannot share widgets' timelines yet; build without widgets")
	}
	if p.MacOS.Category == "" {
		detailIn(ctx, "set macos.category, which the Mac App Store requires, such as public.app-category.productivity")
	}
	ents := p.sandboxEntitlements()
	identity, installer := "-", ""
	if p.Sign.MacOSProfile == "" {
		detailIn(ctx, "unsigned: set sign.macos_profile to upload it to the Mac App Store")
	} else {
		data, err := os.ReadFile(p.Sign.MacOSProfile)
		if err != nil {
			return "", err
		}
		profile, err := readProfile(ctx, data)
		if err != nil {
			return "", fmt.Errorf("%s: %w", rel(p.Sign.MacOSProfile), err)
		}
		granted, _ := profile["Entitlements"].(map[string]any)
		appID, _ := granted["com.apple.application-identifier"].(string)
		if !profileAllows(appID, p.ID) {
			return "", fmt.Errorf("the provisioning profile is for %s, not %s", appID, p.ID)
		}
		team, _ := granted["com.apple.developer.team-identifier"].(string)
		ents["com.apple.application-identifier"] = signedAppID(appID, p.ID)
		ents["com.apple.developer.team-identifier"] = team
		if err := os.WriteFile(filepath.Join(app, "Contents", "embedded.provisionprofile"), data, 0o644); err != nil {
			return "", err
		}
		if identity, err = profileIdentity(ctx, profile); err != nil {
			return "", err
		}
		if installer, err = p.installerIdentity(ctx, team); err != nil {
			return "", err
		}
	}
	path, err := writeEntitlements(ents)
	if err != nil {
		return "", err
	}
	defer os.Remove(path)
	stage(ctx, "signing for the Mac App Store")
	libs, _ := filepath.Glob(filepath.Join(app, "Contents", "Frameworks", "*.dylib"))
	for _, lib := range libs {
		if err := run(ctx, "codesign", "--force", "--sign", identity, lib); err != nil {
			return "", err
		}
	}
	if err := run(ctx, "codesign", "--force", "--sign", identity, "--entitlements", path, app); err != nil {
		return "", err
	}
	stage(ctx, "packaging for the Mac App Store")
	pkg := filepath.Join(dir, p.releaseName("macos_"+arch, ".pkg"))
	args := []string{"--component", app, "/Applications"}
	if installer != "" {
		args = append(args, "--sign", installer)
	}
	return pkg, run(ctx, "productbuild", append(args, pkg)...)
}

// installerIdentity is the identity that signs the store's installer
// package: sign.installer, else the keychain's Mac Installer
// Distribution identity of team.
func (p *project) installerIdentity(ctx context.Context, team string) (string, error) {
	if p.Sign.Installer != "" {
		return p.Sign.Installer, nil
	}
	out, err := output(ctx, "security", "find-identity", "-v", "-p", "basic")
	if err != nil {
		return "", err
	}
	for line := range strings.SplitSeq(out, "\n") {
		if strings.Contains(line, "Installer") && strings.Contains(line, "("+team+")") {
			if _, rest, ok := strings.Cut(line, ") "); ok {
				if sha, _, ok := strings.Cut(strings.TrimSpace(rest), " "); ok {
					return sha, nil
				}
			}
		}
	}
	return "", fmt.Errorf("the keychain has no Mac Installer Distribution identity of team %s: install it, or set sign.installer", team)
}
