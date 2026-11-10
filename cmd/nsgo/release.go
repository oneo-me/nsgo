package main

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
)

// nsgo build gathers everything it makes for others in build.output's
// releases directory, the one place to look: at its top the files a
// GitHub release uploads, each with <file>.sha256 (the installers, IPAs
// and APKs) and install.sh and install.ps1, which install the app from
// the repository's releases; the site in web/ and each mini program's
// project in miniprogram/<platform>/. Simulators' and watchOS's bundles
// install nowhere else, and stay in their packages. Machines building
// different targets each add theirs, so a release may gather them from
// several.

// releaseDir is where nsgo build gathers what a release uploads.
func (p *project) releaseDir() string {
	return filepath.Join(p.Build.Output, "releases")
}

// githubRepository is the repository ("owner/name") whose releases the
// install scripts install from: release.github, else the one the app's
// Git remote origin names on GitHub, else "".
func (p *project) githubRepository() string {
	if p.Releases.GitHub != "" {
		return p.Releases.GitHub
	}
	out, err := exec.Command("git", "-C", p.Dir, "remote", "get-url", "origin").Output()
	if err != nil {
		return ""
	}
	return githubFromRemote(strings.TrimSpace(string(out)))
}

// githubFromRemote reads owner/name from a GitHub remote's URL, in its
// HTTPS or SSH form.
func githubFromRemote(url string) string {
	for _, prefix := range []string{"https://github.com/", "http://github.com/", "ssh://git@github.com/", "git@github.com:"} {
		if rest, ok := strings.CutPrefix(url, prefix); ok {
			rest = strings.TrimSuffix(strings.TrimSuffix(rest, "/"), ".git")
			if owner, name, ok := strings.Cut(rest, "/"); ok && owner != "" && name != "" && !strings.Contains(name, "/") {
				return rest
			}
		}
	}
	return ""
}

// tagPrefix is what a release's tag adds before the version, v by default.
func (p *project) tagPrefix() string {
	if p.Releases.TagPrefix != nil {
		return *p.Releases.TagPrefix
	}
	return "v"
}

// releaseVersionFile records which version the releases directory holds.
const releaseVersionFile = ".version"

// cleanReleaseDir empties the releases directory when it holds another
// version, and keeps this version's, from other machines or earlier
// builds, which builds of the same targets replace.
func (p *project) cleanReleaseDir() error {
	dir := p.releaseDir()
	if v, err := os.ReadFile(filepath.Join(dir, releaseVersionFile)); err != nil || strings.TrimSpace(string(v)) != p.Version {
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, releaseVersionFile), []byte(p.Version+"\n"), 0o644)
}

// releaseCopy copies a package's file into the releases directory as
// name, with its checksum, leaving the package whole for -run.
func (p *project) releaseCopy(path, name string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	dst := filepath.Join(p.releaseDir(), name)
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		return err
	}
	return p.releaseAssets(dst)
}

// releaseTree copies a package's directory, a site or a mini program's
// project, into the releases directory at name, replacing what was there.
func (p *project) releaseTree(dir, name string) error {
	dst := filepath.Join(p.releaseDir(), filepath.FromSlash(name))
	if err := os.RemoveAll(dst); err != nil {
		return err
	}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), data, 0o644)
	})
	if err == nil {
		p.released = append(p.released, dst)
	}
	return err
}

// releaseAssets moves each file given, an installer, into the release
// directory, with its checksum beside it as <file>.sha256, which the
// install scripts and sha256sum -c check. Empty paths are skipped.
func (p *project) releaseAssets(paths ...string) error {
	for _, path := range paths {
		if path == "" {
			continue
		}
		if err := os.MkdirAll(p.releaseDir(), 0o755); err != nil {
			return err
		}
		dst := filepath.Join(p.releaseDir(), filepath.Base(path))
		if path != dst {
			if err := os.Rename(path, dst); err != nil {
				return err
			}
		}
		data, err := os.ReadFile(dst)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		line := hex.EncodeToString(sum[:]) + "  " + filepath.Base(dst) + "\n"
		if err := os.WriteFile(dst+".sha256", []byte(line), 0o644); err != nil {
			return err
		}
		p.released = append(p.released, dst)
	}
	return nil
}

//go:embed install/install.sh
var installSh string

//go:embed install/install.ps1
var installPs1 string

// writeInstallScripts writes install.sh, for macOS and Linux, and
// install.ps1, for Windows, into the release directory. Uploaded with each
// release, the latest's are at releases/latest/download/, and install that
// release's version, or <SLUG>_VERSION's. They install only the desktop
// packages, so a build of none of targets' macOS, Windows or Linux writes
// none. Without a GitHub repository it writes none and says how to name one.
func (p *project) writeInstallScripts(targets []string) error {
	if !p.installers() || !slices.ContainsFunc(targets, desktopTarget) {
		return nil
	}
	repo := p.githubRepository()
	if repo == "" {
		warn("no install scripts: set release.github in nsgo.yaml to the repository whose releases they install from")
		return nil
	}
	name, err := p.displayName()
	if err != nil {
		return err
	}
	slug := p.packageSlug()
	values := map[string][2]string{
		"REPOSITORY": {shQuote(repo), psQuote(repo)},
		"NAME":       {shQuote(name), psQuote(name)},
		"SLUG":       {shQuote(slug), psQuote(slug)},
		"APP":        {shQuote(p.Name + ".app"), ""},
		"VERSION":    {shQuote(p.Version), psQuote(p.Version)},
		"TAG_PREFIX": {shQuote(p.tagPrefix()), psQuote(p.tagPrefix())},
		"ENV":        {envName(slug), envName(slug)},
		"GITHUB":     {repo, repo},
	}
	var sh, ps []string
	for key, v := range values {
		sh = append(sh, "@@"+key+"@@", v[0])
		ps = append(ps, "@@"+key+"@@", v[1])
	}
	if err := os.MkdirAll(p.releaseDir(), 0o755); err != nil {
		return err
	}
	shPath := filepath.Join(p.releaseDir(), "install.sh")
	if err := os.WriteFile(shPath, []byte(strings.NewReplacer(sh...).Replace(installSh)), 0o755); err != nil {
		return err
	}
	// Windows PowerShell reads a script without a byte order mark as the
	// system's code page; irm, which downloads it, decodes GitHub's
	// untyped bytes as Latin-1, so psQuote keeps the script ASCII.
	psPath := filepath.Join(p.releaseDir(), "install.ps1")
	script := strings.ReplaceAll(strings.NewReplacer(ps...).Replace(installPs1), "\n", "\r\n")
	if err := os.WriteFile(psPath, []byte(script), 0o644); err != nil {
		return err
	}
	base := "https://github.com/" + repo + "/releases/latest/download/"
	detail("→ curl -fsSL %sinstall.sh | sh", base)
	detail("→ irm %sinstall.ps1 | iex", base)
	return nil
}

// desktopTarget reports whether target is macOS, Windows or Linux, whose
// packages the install scripts install.
func desktopTarget(target string) bool {
	sys, _, _ := strings.Cut(target, "_")
	return sys == "macos" || sys == "windows" || sys == "linux"
}

// envName is the prefix of an install script's environment variables:
// MY_APP for my-app.
func envName(slug string) string {
	name := strings.Map(func(r rune) rune {
		if r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)) {
			return unicode.ToUpper(r)
		}
		return '_'
	}, slug)
	if name == "" || name[0] >= '0' && name[0] <= '9' {
		name = "APP_" + name
	}
	return name
}

// shQuote quotes s for a POSIX shell.
func shQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// psQuote is a PowerShell expression of s in ASCII alone: a quoted string,
// with what is not ASCII as [char] codes joined to it.
func psQuote(s string) string {
	var parts []string
	var b strings.Builder
	flush := func() {
		if b.Len() > 0 {
			parts = append(parts, "'"+strings.ReplaceAll(b.String(), "'", "''")+"'")
			b.Reset()
		}
	}
	for _, r := range s {
		if r < 0x80 {
			b.WriteRune(r)
			continue
		}
		flush()
		if r > 0xffff {
			// A pair of UTF-16 surrogates.
			r -= 0x10000
			parts = append(parts, fmt.Sprintf("[char]0x%04X + [char]0x%04X", 0xd800+(r>>10), 0xdc00+(r&0x3ff)))
			continue
		}
		parts = append(parts, fmt.Sprintf("[char]0x%04X", r))
	}
	flush()
	if len(parts) == 0 {
		return "''"
	}
	if len(parts) == 1 && strings.HasPrefix(parts[0], "'") {
		return parts[0]
	}
	return "(" + "[string]''" + " + " + strings.Join(parts, " + ") + ")"
}
