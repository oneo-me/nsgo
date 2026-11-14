package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestFindAdbInTheSDK checks the fallback that finds adb without it being
// on PATH, so nsgo dev works after installing the SDK in its usual place.
func TestFindAdbInTheSDK(t *testing.T) {
	home := t.TempDir()
	name := "adb"
	if runtime.GOOS == "windows" {
		name = "adb.exe"
	}
	want := filepath.Join(home, "Android", "platform-tools", name)
	if err := os.MkdirAll(filepath.Dir(want), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(want, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("PATH", "")
	t.Setenv("ANDROID_HOME", "")
	t.Setenv("ANDROID_SDK_ROOT", "")

	got, err := findAdb()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("findAdb = %q, want %q", got, want)
	}
}

// TestFindAdbWithoutTheSDK checks the error that names what to install.
func TestFindAdbWithoutTheSDK(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	t.Setenv("PATH", "")
	t.Setenv("ANDROID_HOME", "")
	t.Setenv("ANDROID_SDK_ROOT", "")

	if _, err := findAdb(); err == nil {
		t.Fatal("findAdb found adb in an empty machine")
	}
}

// TestAndroidArchitectures checks the two packages and the ABIs they run on.
func TestAndroidArchitectures(t *testing.T) {
	for goarch, abi := range map[string]string{"arm64": "arm64-v8a", "amd64": "x86_64"} {
		got, err := androidABI(goarch)
		if err != nil || got != abi {
			t.Errorf("androidABI(%q) = %q, %v; want %q", goarch, got, err, abi)
		}
		want, err := goarchForABI(abi)
		if err != nil || want != goarch {
			t.Errorf("goarchForABI(%q) = %q, %v; want %q", abi, want, err, goarch)
		}
	}
	if _, err := androidABI("386"); err == nil {
		t.Error("androidABI(386) succeeded")
	}
	// 32-bit ARM says why and where it is discussed; other ABIs do not claim to be 32-bit ARM.
	for abi, arm := range map[string]bool{"armeabi-v7a": true, "armeabi": true, "x86": false, "riscv64": false} {
		_, err := goarchForABI(abi)
		if err == nil {
			t.Errorf("goarchForABI(%q) succeeded", abi)
			continue
		}
		if says := strings.Contains(err.Error(), "/issues/41") && strings.Contains(err.Error(), "NDK"); says != arm {
			t.Errorf("goarchForABI(%q) error = %v; names 32-bit ARM's limit: %v, want %v", abi, err, says, arm)
		}
	}
}

// TestFindNDKInTheSDK checks that the emulator build finds the newest NDK
// in the SDK and the clang that targets the manifest's API.
func TestFindNDKInTheSDK(t *testing.T) {
	sdk := t.TempDir()
	name := "x86_64-linux-android28-clang"
	if runtime.GOOS == "windows" {
		name += ".cmd"
	}
	for _, version := range []string{"26.1.10909125", "27.2.12479018"} {
		bin := filepath.Join(sdk, "ndk", version, "toolchains", "llvm", "prebuilt", "linux-x86_64", "bin")
		if err := os.MkdirAll(bin, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("ANDROID_HOME", sdk)
	t.Setenv("ANDROID_SDK_ROOT", "")
	t.Setenv("ANDROID_NDK_HOME", "")
	t.Setenv("ANDROID_NDK_ROOT", "")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())

	got, err := findNDK()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(sdk, "ndk", "27.2.12479018"); got != want {
		t.Fatalf("findNDK = %q, want %q", got, want)
	}
	clang, err := androidClang("amd64")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(got, "toolchains", "llvm", "prebuilt", "linux-x86_64", "bin", name); clang != want {
		t.Fatalf("androidClang = %q, want %q", clang, want)
	}
}
