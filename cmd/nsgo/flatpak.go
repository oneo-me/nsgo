package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"text/template"
	"time"
)

// A store build of a Linux target is what Flathub builds the app's
// Flatpak from, since a .flatpak bundle is an OSTree commit no Mac can
// write: <slug>-<version>-linux-<arch>.tar.gz, the Linux package with
// the app's AppStream metadata, <id>.metainfo.xml, and once every target
// is built, flatpak/<id>.yml in the releases directory, the manifest
// that installs the archives of the release on GNOME's runtime, which
// has GTK 4 and WebKitGTK. Submit the manifest to Flathub, or build it
// with flatpak-builder.

// flatpakRuntime is the GNOME runtime's version the manifest names,
// unless linux.flatpak_runtime names another.
const flatpakRuntime = "51"

// flatpakArch is Flatpak's name of each architecture of the targets.
var flatpakArch = map[string]string{"x64": "x86_64", "arm64": "aarch64"}

// buildLinuxArchive writes the store's archive of the Linux package in
// dir, built for arch, Go's, beside it, with the AppStream metadata
// added, and returns its path.
func buildLinuxArchive(ctx context.Context, p *project, dir, arch string) (string, error) {
	stage(ctx, "writing the Flatpak's archive")
	metainfo, err := p.metainfo()
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir, p.ID+".metainfo.xml"), metainfo, 0o644); err != nil {
		return "", err
	}
	entries, err := linuxDirEntries(dir)
	if err != nil {
		return "", err
	}
	data, err := tarGz(entries, p.packageSlug()+"/")
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, p.releaseName("linux_"+goArchName(arch), ".tar.gz"))
	return path, os.WriteFile(path, data, 0o644)
}

// metainfo is the app's AppStream metadata, which software centers and
// Flathub list it by.
func (p *project) metainfo() ([]byte, error) {
	name, err := p.displayName()
	if err != nil {
		return nil, err
	}
	n, err := p.appNames()
	if err != nil {
		return nil, err
	}
	var names [][2]string
	if n != nil {
		for _, lang := range n.languages()[1:] {
			names = append(names, [2]string{desktopLocale(lang), n.names[lang]})
		}
	}
	summary := p.Linux.Summary
	if summary == "" {
		summary = name
	}
	license := p.Linux.License
	if license == "" {
		license = "LicenseRef-proprietary"
	}
	developer := p.ID
	if i := strings.LastIndexByte(developer, '.'); i > 0 {
		developer = developer[:i]
	}
	homepage := ""
	if repo := p.githubRepository(); repo != "" {
		homepage = "https://github.com/" + repo
	}
	var out bytes.Buffer
	err = metainfoTemplate.Execute(&out, map[string]any{
		"ID": p.ID, "Name": name, "Names": names, "Summary": summary, "License": license,
		"Developer": developer, "DeveloperName": p.maintainer(), "Homepage": homepage,
		"Version": p.Version, "Date": time.Now().Format(time.DateOnly),
	})
	return out.Bytes(), err
}

var metainfoTemplate = template.Must(template.New("metainfo").Funcs(template.FuncMap{"x": xmlText}).Parse(`<?xml version="1.0" encoding="UTF-8"?>
<component type="desktop-application">
  <id>{{x .ID}}</id>
  <name>{{x .Name}}</name>
{{- range .Names}}
  <name xml:lang="{{x (index . 0)}}">{{x (index . 1)}}</name>
{{- end}}
  <summary>{{x .Summary}}</summary>
  <metadata_license>CC0-1.0</metadata_license>
  <project_license>{{x .License}}</project_license>
  <developer id="{{x .Developer}}">
    <name>{{x .DeveloperName}}</name>
  </developer>
  <description>
    <p>{{x .Summary}}</p>
  </description>
  <launchable type="desktop-id">{{x .ID}}.desktop</launchable>
{{- if .Homepage}}
  <url type="homepage">{{x .Homepage}}</url>
{{- end}}
  <content_rating type="oars-1.1"/>
  <releases>
    <release version="{{x .Version}}" date="{{.Date}}"/>
  </releases>
</component>
`))

// writeFlatpakManifest writes flatpak/<id>.yml into the releases
// directory for a store build of Linux targets: the manifest installing
// the archives there, each by its download in the release, else by its
// path beside the manifest, for flatpak-builder to build here.
func (p *project) writeFlatpakManifest(targets []string) error {
	if p.channel() != channelStore || !p.installers() || !slices.ContainsFunc(targets, func(t string) bool { return strings.HasPrefix(t, "linux_") }) {
		return nil
	}
	base := p.releaseFilesURL()
	type source struct{ URL, Path, SHA256, Arch string }
	var sources []source
	// Archives other machines built join this build's.
	for _, arch := range []string{"x64", "arm64"} {
		file := p.releaseName("linux_"+arch, ".tar.gz")
		data, err := os.ReadFile(filepath.Join(p.releaseDir(), file))
		if err != nil {
			continue
		}
		sum := sha256.Sum256(data)
		s := source{SHA256: hex.EncodeToString(sum[:]), Arch: flatpakArch[arch]}
		if base != "" {
			s.URL = base + url.PathEscape(file)
		} else {
			s.Path = "../" + file
		}
		sources = append(sources, s)
	}
	if len(sources) == 0 {
		return nil
	}
	runtime := p.Linux.FlatpakRuntime
	if runtime == "" {
		runtime = flatpakRuntime
	}
	var out bytes.Buffer
	err := flatpakManifest.Execute(&out, map[string]any{
		"ID": p.ID, "Slug": p.packageSlug(), "Command": p.Name, "Runtime": runtime, "Sources": sources,
	})
	if err != nil {
		return err
	}
	dir := filepath.Join(p.releaseDir(), "flatpak")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, p.ID+".yml")
	if err := os.WriteFile(path, out.Bytes(), 0o644); err != nil {
		return err
	}
	detail("→ %s", rel(path))
	if base == "" {
		warn("%s builds from the archives beside it: set release.github or updates.feed to download them from the release", rel(path))
	}
	return nil
}

// releaseFilesURL is where this version's release files are downloaded
// from: updates.feed, else its GitHub release; "" without either.
func (p *project) releaseFilesURL() string {
	if p.Updates.Feed != "" {
		return p.Updates.Feed
	}
	if repo := p.githubRepository(); repo != "" {
		return "https://github.com/" + repo + "/releases/download/" + url.PathEscape(p.tagPrefix()+p.Version) + "/"
	}
	return ""
}

// flatpakManifest installs the app in /app/lib/<slug>, its command, desktop
// entry, icon and metadata where Flatpak exports them, with the
// permissions a desktop app on Wayland or X11 needs: its display, the GPU,
// the network and sound.
var flatpakManifest = template.Must(template.New("flatpak").Funcs(template.FuncMap{"q": yamlQuote}).Parse(`id: {{.ID}}
runtime: org.gnome.Platform
runtime-version: {{q .Runtime}}
sdk: org.gnome.Sdk
command: {{q .Command}}
finish-args:
  - --share=ipc
  - --socket=wayland
  - --socket=fallback-x11
  - --device=dri
  - --share=network
  - --socket=pulseaudio
modules:
  - name: {{.Slug}}
    buildsystem: simple
    build-commands:
      - mkdir -p /app/lib/{{.Slug}} /app/bin
      - cp -a . /app/lib/{{.Slug}}/
      - ln -s {{q (printf "/app/lib/%s/%s" .Slug .Command)}} {{q (printf "/app/bin/%s" .Command)}}
      - install -Dm644 {{.ID}}.desktop /app/share/applications/{{.ID}}.desktop
      - install -Dm644 {{.ID}}.png /app/share/icons/hicolor/512x512/apps/{{.ID}}.png
      - install -Dm644 {{.ID}}.metainfo.xml /app/share/metainfo/{{.ID}}.metainfo.xml
    sources:
{{- range .Sources}}
      - type: archive
{{- if .URL}}
        url: {{q .URL}}
{{- else}}
        path: {{q .Path}}
{{- end}}
        sha256: {{.SHA256}}
        only-arches: [{{.Arch}}]
{{- end}}
`))

// yamlQuote is s as a YAML string in double quotes.
func yamlQuote(s string) string {
	return fmt.Sprintf("%q", s)
}
