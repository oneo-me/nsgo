// Command libghostty builds libghostty-vt, Ghostty's terminal emulator as
// a library, for every target base/terminal runs on, desktops and iOS, from
// Ghostty's sources at the commit it binds, and writes the themes that
// Ghostty ships. Run it on a Mac with Xcode, from the repository's root:
//
//	go run ./tools/libghostty
//
// It clones Ghostty into the user's cache, downloads the Zig its
// build.zig.zon asks for from ziglang.org and checks it against the
// index's SHA-256, and writes the libraries into
// base/terminal/internal/library/lib/<goos>-<goarch>/, which are
// committed, and base/terminal/ghostty_themes.go. With -themes it writes
// the themes alone, which needs no Zig.
package main

import (
	"bytes"
	"crypto/sha256"
	"debug/elf"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"flag"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// commit is the commit of Ghostty base/terminal binds; change it with the
// binding (base/terminal/internal/vt) and run the command again.
const commit = "befcdfd2c3a1cb24d9ec886e93c95b2b5daa7028"

// libDir is where the libraries go, by GOOS-GOARCH.
const libDir = "base/terminal/internal/library/lib"

var targets = []struct {
	goos, goarch, zig string
	// name is the library's file; built is where Zig puts it.
	name, built string
}{
	{"darwin", "arm64", "aarch64-macos", "libghostty-vt.dylib", "lib/libghostty-vt.dylib"},
	{"darwin", "amd64", "x86_64-macos", "libghostty-vt.dylib", "lib/libghostty-vt.dylib"},
	// The glibc of Debian 10 and Ubuntu 20.04, and later.
	{"linux", "amd64", "x86_64-linux-gnu.2.28", "libghostty-vt.so", "lib/libghostty-vt.so"},
	{"linux", "arm64", "aarch64-linux-gnu.2.28", "libghostty-vt.so", "lib/libghostty-vt.so"},
	{"windows", "amd64", "x86_64-windows-gnu", "ghostty-vt.dll", "bin/ghostty-vt.dll"},
	{"windows", "arm64", "aarch64-windows-gnu", "ghostty-vt.dll", "bin/ghostty-vt.dll"},
	// iOS's are not embedded: the nsgo tool signs them into apps as a
	// framework.
	{"ios", "arm64", "aarch64-ios", "libghostty-vt.dylib", "lib/libghostty-vt.dylib"},
	{"iossimulator", "arm64", "aarch64-ios-simulator", "libghostty-vt.dylib", "lib/libghostty-vt.dylib"},
}

func main() {
	log.SetFlags(0)
	src := flag.String("src", "", "a checkout of Ghostty at the commit (cloned into the user's cache when empty)")
	themes := flag.Bool("themes", false, "only write the themes")
	flag.Parse()
	if _, err := os.Stat("base/terminal"); err != nil {
		log.Fatal("run it from the repository's root")
	}
	if *themes {
		writeThemes(*src)
		return
	}
	if *src == "" {
		*src = checkout()
	}
	zig := zigFor(*src)
	for _, t := range targets {
		prefix, err := os.MkdirTemp("", "libghostty-vt-")
		if err != nil {
			log.Fatal(err)
		}
		defer os.RemoveAll(prefix)
		log.Printf("building %s/%s", t.goos, t.goarch)
		run(*src, zig, "build", "-Demit-lib-vt", "-Dtarget="+t.zig, "-Doptimize=ReleaseFast", "-Dstrip=true", "--prefix", prefix)
		built := filepath.Join(prefix, t.built)
		if t.goos == "linux" {
			// The build keeps the debug information of ELF libraries,
			// four fifths of them.
			built = stripELF(built)
		}
		dst := filepath.Join(libDir, t.goos+"-"+t.goarch, t.name)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			log.Fatal(err)
		}
		log.Printf("  %s %s", dst, copyFile(built, dst))
	}
	writeThemes(*src)
}

// cacheDir is NSGo's directory in the user's cache.
func cacheDir() string {
	cache, err := os.UserCacheDir()
	if err != nil {
		log.Fatal(err)
	}
	return filepath.Join(cache, "nsgo")
}

// checkout returns a checkout of Ghostty at the commit in the user's
// cache, cloning or fetching it.
func checkout() string {
	dir := filepath.Join(cacheDir(), "ghostty-src")
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		run("", "git", "clone", "--filter=blob:none", "https://github.com/ghostty-org/ghostty", dir)
	} else {
		run(dir, "git", "fetch", "origin")
	}
	run(dir, "git", "checkout", "--detach", commit)
	return dir
}

// zigFor returns the zig command of the version Ghostty's build.zig.zon
// asks for: one on the PATH of that version, else the release from
// ziglang.org, downloaded once into the user's cache.
func zigFor(src string) string {
	zon, err := os.ReadFile(filepath.Join(src, "build.zig.zon"))
	if err != nil {
		log.Fatal(err)
	}
	m := regexp.MustCompile(`\.minimum_zig_version\s*=\s*"([^"]+)"`).FindSubmatch(zon)
	if m == nil {
		log.Fatal("build.zig.zon names no minimum_zig_version")
	}
	version := string(m[1])
	if out, err := exec.Command("zig", "version").Output(); err == nil && strings.TrimSpace(string(out)) == version {
		return "zig"
	}
	host := map[string]string{"arm64": "aarch64", "amd64": "x86_64"}[runtime.GOARCH] + "-" + map[string]string{"darwin": "macos"}[runtime.GOOS]
	dir := filepath.Join(cacheDir(), "zig-"+host+"-"+version)
	zig := filepath.Join(dir, "zig")
	if _, err := os.Stat(zig); err == nil {
		return zig
	}
	var index map[string]map[string]json.RawMessage
	if err := json.Unmarshal(fetch("https://ziglang.org/download/index.json"), &index); err != nil {
		log.Fatal(err)
	}
	var release struct {
		Tarball, Shasum string
	}
	if err := json.Unmarshal(index[version][host], &release); err != nil || release.Tarball == "" {
		log.Fatalf("ziglang.org has no Zig %s for %s", version, host)
	}
	log.Printf("downloading %s", release.Tarball)
	data := fetch(release.Tarball)
	if sum := sha256.Sum256(data); hex.EncodeToString(sum[:]) != release.Shasum {
		log.Fatalf("%s does not have the SHA-256 the index gives", release.Tarball)
	}
	tmp, err := os.MkdirTemp(cacheDir(), "zig-")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(tmp)
	archive := filepath.Join(tmp, "zig.tar.xz")
	if err := os.WriteFile(archive, data, 0o644); err != nil {
		log.Fatal(err)
	}
	run(tmp, "tar", "-xJf", archive, "--strip-components=1")
	os.Remove(archive)
	if err := os.Rename(tmp, dir); err != nil {
		log.Fatal(err)
	}
	return zig
}

func run(dir, name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatalf("%s %s: %v", name, strings.Join(args, " "), err)
	}
}

// copyFile copies src, following links, to dst and returns its SHA-256.
func copyFile(src, dst string) string {
	in, err := os.Open(src)
	if err != nil {
		log.Fatal(err)
	}
	defer in.Close()
	f, err := os.Create(dst)
	if err != nil {
		log.Fatal(err)
	}
	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(f, h), in); err != nil {
		log.Fatal(err)
	}
	if err := f.Close(); err != nil {
		log.Fatal(err)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// stripELF writes a copy of the ELF library at path without the contents
// of its debug sections, as `strip --strip-debug` would, and returns its
// path. What loading uses (the segments, and the sections in them) stays
// in place; the other sections after the segments move down, and the
// debug sections keep their headers, empty, so that no index changes.
func stripELF(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}
	f, err := elf.NewFile(bytes.NewReader(data))
	if err != nil {
		log.Fatal(err)
	}
	if f.Class != elf.ELFCLASS64 || f.Data != elf.ELFDATA2LSB {
		log.Fatalf("%s: not a 64-bit little-endian ELF file", path)
	}
	end := uint64(0)
	for _, p := range f.Progs {
		end = max(end, p.Off+p.Filesz)
	}
	out := append([]byte(nil), data[:end]...)
	le := binary.LittleEndian
	shoff := le.Uint64(data[0x28:])
	shentsize, shnum := uint64(le.Uint16(data[0x3a:])), uint64(le.Uint16(data[0x3c:]))
	headers := append([]byte(nil), data[shoff:shoff+shentsize*shnum]...)
	for i, s := range f.Sections {
		h := headers[uint64(i)*shentsize:]
		if s.Type == elf.SHT_NULL || s.Type == elf.SHT_NOBITS || s.Flags&elf.SHF_ALLOC != 0 || s.Offset < end {
			continue
		}
		size := s.FileSize
		if strings.HasPrefix(s.Name, ".debug") || strings.HasPrefix(s.Name, ".zdebug") {
			size = 0
		}
		align := max(s.Addralign, 1)
		for uint64(len(out))%align != 0 {
			out = append(out, 0)
		}
		le.PutUint64(h[0x18:], uint64(len(out))) // sh_offset
		le.PutUint64(h[0x20:], size)             // sh_size
		out = append(out, data[s.Offset:s.Offset+size]...)
	}
	for len(out)%8 != 0 {
		out = append(out, 0)
	}
	le.PutUint64(out[0x28:], uint64(len(out))) // e_shoff
	out = append(out, headers...)
	stripped := path + ".stripped"
	if err := os.WriteFile(stripped, out, 0o755); err != nil {
		log.Fatal(err)
	}
	return stripped
}
