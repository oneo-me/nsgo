package main

import (
	"flag"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestLoadAndOverride(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "nsgo.yaml"), []byte("name: App\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := load(dir)
	if err != nil {
		t.Fatal(err)
	}

	fs := flag.NewFlagSet("dev", flag.ContinueOnError)
	f := bindDevFlags(fs)
	if err := fs.Parse([]string{"-name", "Other"}); err != nil {
		t.Fatal(err)
	}
	f.apply(p)

	if p.Name != "Other" || p.ID != "dev.nsgo.Other" {
		t.Errorf("app: got %q, %q", p.Name, p.ID)
	}
}

func TestLoadRefusesWindow(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "nsgo.yaml"), []byte("name: App\nwindow: {width: 420, height: 560}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := load(dir); err == nil || !strings.Contains(err.Error(), "nsgo.WindowSize") {
		t.Errorf("a window in nsgo.yaml: %v", err)
	}
}

func TestDevFlagsApply(t *testing.T) {
	fs := flag.NewFlagSet("dev", flag.ContinueOnError)
	f := bindDevFlags(fs)
	args := []string{"-target", "web", "-hot-reload", "-name", "Other"}
	if err := fs.Parse(args); err != nil {
		t.Fatal(err)
	}
	p := &project{
		Name:    "App",
		ID:      "com.example.app",
		Version: "1.0.0",
	}
	p.Dev.Target = "native"
	off := false
	p.Dev.HotReload = &off

	f.apply(p)

	if p.Dev.Target != "web" {
		t.Errorf("dev.target: got %q, want web", p.Dev.Target)
	}
	if !p.hotReload() {
		t.Error("dev.hot_reload: got false, want true")
	}
	if p.Name != "Other" {
		t.Errorf("name: got %q, want Other", p.Name)
	}
	if p.ID != "com.example.app" {
		t.Errorf("explicit id changed to %q", p.ID)
	}
	if p.Version != "1.0.0" {
		t.Errorf("version: got %q, want 1.0.0", p.Version)
	}
}

func TestDevFlagsKeepValues(t *testing.T) {
	fs := flag.NewFlagSet("dev", flag.ContinueOnError)
	f := bindDevFlags(fs)
	if err := fs.Parse(nil); err != nil {
		t.Fatal(err)
	}
	hot := true
	p := &project{Name: "App", ID: "com.example.app", Version: "1.0.0"}
	p.Dev.Target = "web"
	p.Dev.HotReload = &hot

	f.apply(p)

	if p.Dev.Target != "web" || !p.hotReload() {
		t.Errorf("dev: got target %q, hot reload %v", p.Dev.Target, p.hotReload())
	}
	if p.Name != "App" || p.ID != "com.example.app" || p.Version != "1.0.0" {
		t.Errorf("app: got %q, %q, %q", p.Name, p.ID, p.Version)
	}
}

func TestDevFlagsDisableHotReload(t *testing.T) {
	fs := flag.NewFlagSet("dev", flag.ContinueOnError)
	f := bindDevFlags(fs)
	if err := fs.Parse([]string{"-hot-reload=false"}); err != nil {
		t.Fatal(err)
	}
	hot := true
	p := &project{}
	p.Dev.HotReload = &hot

	f.apply(p)

	if p.hotReload() {
		t.Error("dev.hot_reload: got true, want false")
	}
}

func TestNameFollowsGeneratedID(t *testing.T) {
	fs := flag.NewFlagSet("dev", flag.ContinueOnError)
	f := bindDevFlags(fs)
	if err := fs.Parse([]string{"-name", "Other"}); err != nil {
		t.Fatal(err)
	}
	p := &project{Name: "App", ID: "dev.nsgo.App", idDefault: true}

	f.apply(p)

	if p.ID != "dev.nsgo.Other" {
		t.Errorf("id: got %q, want dev.nsgo.Other", p.ID)
	}
}

func TestBuildFlagsApply(t *testing.T) {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	f := bindBuildFlags(fs)
	args := []string{"-target", "web,ios", "-output", "out", "-open=false", "-version", "2.0.0", "-jobs", "3"}
	if err := fs.Parse(args); err != nil {
		t.Fatal(err)
	}
	p := &project{Dir: t.TempDir()}
	p.Build.Output = p.resolve("dist")
	p.Build.Targets = []string{"macos_arm64"}

	f.apply(p)

	if got := p.Build.Targets; len(got) != 2 || got[0] != "web" || got[1] != "ios" {
		t.Errorf("build.targets: got %v, want [web ios]", got)
	}
	if want := filepath.Join(p.Dir, "out"); p.Build.Output != want {
		t.Errorf("build.output: got %q, want %q", p.Build.Output, want)
	}
	if p.openOutput() {
		t.Error("build.open: got true, want false")
	}
	if p.Version != "2.0.0" {
		t.Errorf("version: got %q, want 2.0.0", p.Version)
	}
	if p.Build.Jobs != 3 {
		t.Errorf("build.jobs: got %d, want 3", p.Build.Jobs)
	}
}

func TestBuildFlagsKeepValues(t *testing.T) {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	f := bindBuildFlags(fs)
	if err := fs.Parse(nil); err != nil {
		t.Fatal(err)
	}
	open := false
	p := &project{Dir: t.TempDir()}
	p.Build.Output = p.resolve("dist")
	p.Build.Targets = []string{"macos_arm64"}
	p.Build.Open = &open

	f.apply(p)

	if p.Build.Output != filepath.Join(p.Dir, "dist") {
		t.Errorf("build.output: got %q", p.Build.Output)
	}
	if len(p.Build.Targets) != 1 || p.Build.Targets[0] != "macos_arm64" {
		t.Errorf("build.targets: got %v", p.Build.Targets)
	}
	if p.openOutput() {
		t.Error("build.open: got true, want false")
	}
}

func TestBuildFlagsOpen(t *testing.T) {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	f := bindBuildFlags(fs)
	if err := fs.Parse([]string{"-open"}); err != nil {
		t.Fatal(err)
	}
	open := false
	p := &project{}
	p.Build.Open = &open

	f.apply(p)

	if !p.openOutput() {
		t.Error("build.open: got false, want true")
	}
}

func TestBuildFlagsAbsoluteOutput(t *testing.T) {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	f := bindBuildFlags(fs)
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	if err := fs.Parse([]string{"-output", out}); err != nil {
		t.Fatal(err)
	}
	p := &project{Dir: t.TempDir()}

	f.apply(p)

	if p.Build.Output != out {
		t.Errorf("build.output: got %q, want %q", p.Build.Output, out)
	}
}

func TestAppFlags(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "nsgo.yaml"), []byte("name: App\ndev: { fps: true }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := load(dir); err == nil || !strings.Contains(err.Error(), "--nsgo-fps") {
		t.Errorf("dev.fps: %v", err)
	}

	fs := flag.NewFlagSet("dev", flag.ContinueOnError)
	f := bindDevFlags(fs)
	if err := fs.Parse([]string{"-fps", "-lang", "zh-Hans"}); err != nil {
		t.Fatal(err)
	}
	opts := devOptions{fps: f.fps, lang: f.lang}
	if got := opts.appFlags(); !slices.Equal(got, []string{"--nsgo-fps", "--nsgo-lang=zh-Hans"}) {
		t.Errorf("appFlags = %q", got)
	}
	if got := androidExtras(opts.appFlags()); !slices.Equal(got, []string{"--ez", "nsgo-fps", "true", "--es", "nsgo-lang", "zh-Hans"}) {
		t.Errorf("androidExtras = %q", got)
	}
	if got := webQuery(opts.appFlags()); got != "?nsgo-fps&nsgo-lang=zh-Hans" {
		t.Errorf("webQuery = %q", got)
	}
	if got := (devOptions{}).appFlags(); got != nil {
		t.Errorf("appFlags without -fps or -lang = %q", got)
	}
	p := &project{Name: "App", ID: "com.example.app"}
	if strings.Contains(p.ldflags(), "fps") {
		t.Errorf("ldflags = %q", p.ldflags())
	}
}

func TestIdentityLdflags(t *testing.T) {
	p := &project{Name: "App", ID: "com.example.app", Version: "1.2.0", BuildNumber: 12}
	flags := p.ldflags()
	for _, want := range []string{"core.appName=App'", "core.appID=com.example.app'", "core.appVersion=1.2.0'", "core.appBuild=12'"} {
		if !strings.Contains(flags, want) {
			t.Errorf("ldflags %s lack %s", flags, want)
		}
	}
	if strings.Contains(flags, "appRelease") {
		t.Errorf("a development build's ldflags %s say release", flags)
	}
	p.Release = true
	if !strings.Contains(p.ldflags(), "core.appRelease=1") {
		t.Errorf("a release's ldflags %s say development", p.ldflags())
	}
}
