package main

import (
	"archive/zip"
	"bytes"
	"compress/zlib"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Release files name their system and architecture, so a release page
// tells them apart.
func TestReleaseNames(t *testing.T) {
	p := &project{Name: "My App", Version: "1.2.0-beta.1"}
	for _, c := range []struct{ got, want string }{
		{dmgName(p, "arm64"), "my-app-1.2.0-beta.1-macos-arm64.dmg"},
		{nsisSetupName(p, "x64"), "my-app-1.2.0-beta.1-windows-x64-setup.exe"},
		{debName(p, "amd64"), "my-app-1.2.0-beta.1-linux-x64.deb"},
		{archPackageName(p, "arm64"), "my-app-1.2.0-beta.1-linux-arm64.pkg.tar.gz"},
		{p.releaseName("android_arm64", ".aab"), "my-app-1.2.0-beta.1-android-arm64.aab"},
		{p.releaseName("ios", ".ipa"), "my-app-1.2.0-beta.1-ios.ipa"},
	} {
		if c.got != c.want {
			t.Errorf("got %s, want %s", c.got, c.want)
		}
	}
}

// A store build's sandboxed app claims the network, the files the user
// picks, what apple.info explains and macos.entitlements.
func TestSandboxEntitlements(t *testing.T) {
	p := &project{}
	p.Apple.Info = map[string]string{"NSMicrophoneUsageDescription": "Calls"}
	p.MacOS.Entitlements = map[string]any{"com.apple.security.files.downloads.read-write": true}
	ents := p.sandboxEntitlements()
	for _, key := range []string{"com.apple.security.app-sandbox", "com.apple.security.network.client",
		"com.apple.security.files.user-selected.read-write", "com.apple.security.device.audio-input",
		"com.apple.security.files.downloads.read-write"} {
		if ents[key] != true {
			t.Errorf("the sandbox lacks %s", key)
		}
	}
	if _, ok := ents["com.apple.security.device.camera"]; ok {
		t.Error("the sandbox claims the camera unasked")
	}
}

func TestMSIXVersions(t *testing.T) {
	for v, want := range map[string]string{"1.2.3": "1.2.3.0", "v2.0": "2.0.0.0", "1.2.0-beta.1": "1.2.0.0", "70000.1.2+5": "65535.1.2.0", "dev": "0.0.0.0"} {
		if got := msixVersion(v); got != want {
			t.Errorf("msixVersion(%q) = %s, want %s", v, got, want)
		}
	}
}

// An MSIX hashes each file in 64 KB blocks, past its local header, and
// types every extension and every file without one.
func TestMSIXPackage(t *testing.T) {
	big := bytes.Repeat([]byte("nsgo"), 40000)
	files := []msixFile{{"App.exe", big}, {"fonts/My Font.ttf", []byte("font")}, {"LICENSE", []byte("MIT")}, {"AppxManifest.xml", []byte("<Package/>")}}
	data, err := writeMSIX(files)
	if err != nil {
		t.Fatal(err)
	}
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	read := map[string][]byte{}
	for _, f := range z.File {
		names = append(names, f.Name)
		if f.Method != zip.Store {
			t.Errorf("%s is compressed", f.Name)
		}
		r, _ := f.Open()
		read[f.Name], _ = io.ReadAll(r)
	}
	if got := strings.Join(names, ","); got != "App.exe,fonts/My%20Font.ttf,LICENSE,AppxManifest.xml,AppxBlockMap.xml,[Content_Types].xml" {
		t.Fatalf("the package holds %s", got)
	}
	var blockMap struct {
		Files []struct {
			Name    string `xml:"Name,attr"`
			Size    int    `xml:"Size,attr"`
			LfhSize int    `xml:"LfhSize,attr"`
			Blocks  []struct {
				Hash string `xml:"Hash,attr"`
			} `xml:"Block"`
		} `xml:"File"`
	}
	if err := xml.Unmarshal(read["AppxBlockMap.xml"], &blockMap); err != nil {
		t.Fatal(err)
	}
	if len(blockMap.Files) != 4 || blockMap.Files[1].Name != `fonts\My Font.ttf` {
		t.Fatalf("the block map lists %+v", blockMap.Files)
	}
	exe := blockMap.Files[0]
	if exe.Size != len(big) || len(exe.Blocks) != 3 {
		t.Fatalf("App.exe has %d bytes in %d blocks", exe.Size, len(exe.Blocks))
	}
	sum := sha256.Sum256(big[msixBlock : 2*msixBlock])
	if exe.Blocks[1].Hash != base64.StdEncoding.EncodeToString(sum[:]) {
		t.Error("the second block's hash is wrong")
	}
	// The data follows the local header the block map measures.
	off, err := z.File[0].DataOffset()
	if err != nil || int(off) != exe.LfhSize {
		t.Errorf("App.exe's data starts at %d, its LfhSize is %d", off, exe.LfhSize)
	}
	types := string(read["[Content_Types].xml"])
	for _, want := range []string{`Extension="exe" ContentType="application/x-msdownload"`, `Extension="ttf"`,
		`PartName="/LICENSE" ContentType="application/octet-stream"`, `PartName="/AppxManifest.xml"`} {
		if !strings.Contains(types, want) {
			t.Errorf("[Content_Types].xml lacks %s", want)
		}
	}
}

func TestMSIXManifest(t *testing.T) {
	p := &project{Name: "My App", ID: "dev.example.my_app", Version: "1.4.0"}
	p.FileTypes = []fileType{{Name: "Drawing", Extensions: []string{"draw", "DRW"}}}
	data, err := msixManifest(p, "arm64")
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Identity struct {
			Name      string `xml:"Name,attr"`
			Publisher string `xml:"Publisher,attr"`
			Version   string `xml:"Version,attr"`
			Arch      string `xml:"ProcessorArchitecture,attr"`
		}
		Application struct {
			Executable string   `xml:"Executable,attr"`
			FileTypes  []string `xml:"Extensions>Extension>FileTypeAssociation>SupportedFileTypes>FileType"`
		} `xml:"Applications>Application"`
	}
	if err := xml.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	if m.Identity.Name != "dev.example.my-app" || m.Identity.Publisher != "CN=My App" || m.Identity.Version != "1.4.0.0" || m.Identity.Arch != "arm64" {
		t.Errorf("the identity is %+v", m.Identity)
	}
	if m.Application.Executable != "My App.exe" || strings.Join(m.Application.FileTypes, ",") != ".draw,.drw" {
		t.Errorf("the application is %+v", m.Application)
	}
	p.Windows.Store.Identity, p.Windows.Store.Publisher = "12345Me.MyApp", "CN=0F1E2D3C"
	data, _ = msixManifest(p, "amd64")
	if !bytes.Contains(data, []byte(`Name="12345Me.MyApp" Publisher="CN=0F1E2D3C"`)) {
		t.Error("the manifest does not carry Partner Center's identity")
	}
}

// The SquashFS image's superblock finds its tables, and its first data
// block is the first file's bytes, compressed.
func TestSquashFS(t *testing.T) {
	data := bytes.Repeat([]byte("abc"), 100000)
	root, err := sqfsTree([]linuxPackageEntry{
		{name: "App", data: data, mode: 0o755},
		{name: "fonts/a.ttf", data: []byte("font"), mode: 0o644},
		{name: "AppRun", link: "App", mode: 0o777},
	})
	if err != nil {
		t.Fatal(err)
	}
	img, err := squashfs(root, 0)
	if err != nil {
		t.Fatal(err)
	}
	le := binary.LittleEndian
	if le.Uint32(img) != sqfsMagic || le.Uint32(img[4:]) != 5 || le.Uint16(img[20:]) != sqfsGzip || le.Uint16(img[28:]) != 4 {
		t.Fatalf("the superblock is %x", img[:32])
	}
	used := le.Uint64(img[40:])
	ids, inodes, dirs := le.Uint64(img[48:]), le.Uint64(img[64:]), le.Uint64(img[72:])
	if len(img)%4096 != 0 || used > uint64(len(img)) || !(96 < inodes && inodes < dirs && dirs < ids && ids+8 == used) {
		t.Fatalf("the tables are at %d, %d and %d of %d, used %d", inodes, dirs, ids, len(img), used)
	}
	z, err := zlib.NewReader(bytes.NewReader(img[96:]))
	if err != nil {
		t.Fatal(err)
	}
	block, _ := io.ReadAll(z)
	if !bytes.Equal(block, data[:sqfsBlockSize]) {
		t.Error("the first block is not App's first 128 KB")
	}
}

// The AppImage's directory is the Linux package with AppRun and .DirIcon,
// without the packages written beside it.
func TestAppDirEntries(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"App": "binary", "dev.example.app.png": "icon", "dev.example.app.desktop": "entry",
		"res/a.txt": "a", "app-1.0.0-linux-x64.deb": "deb", "app-1.0.0-linux-x64.AppImage": "image"})
	p := &project{Name: "App", ID: "dev.example.app"}
	entries, err := appDirEntries(p, dir)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, e := range entries {
		got[e.name] = e.link
	}
	if _, ok := got["app-1.0.0-linux-x64.deb"]; ok {
		t.Error("the AppImage holds the Debian package")
	}
	if _, ok := got["app-1.0.0-linux-x64.AppImage"]; ok {
		t.Error("the AppImage holds itself")
	}
	if got["AppRun"] != "App" || got[".DirIcon"] != "dev.example.app.png" {
		t.Errorf("AppRun is %q and .DirIcon %q", got["AppRun"], got[".DirIcon"])
	}
	for _, name := range []string{"App", "res/a.txt", "dev.example.app.desktop"} {
		if _, ok := got[name]; !ok {
			t.Errorf("the AppImage lacks %s", name)
		}
	}
}

// A store build of Linux targets writes the manifest that installs each
// archive of the release on its architecture.
func TestFlatpakManifest(t *testing.T) {
	dir := t.TempDir()
	p := &project{Dir: dir, Name: "My App", ID: "dev.example.myapp", Version: "1.0.0"}
	p.Build.Output = dir
	p.Releases.GitHub = "me/my-app"
	p.Updates.Channel = channelStore
	os.MkdirAll(p.releaseDir(), 0o755)
	os.WriteFile(filepath.Join(p.releaseDir(), "my-app-1.0.0-linux-x64.tar.gz"), []byte("x64"), 0o644)
	if err := p.writeFlatpakManifest([]string{"linux_x64"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(p.releaseDir(), "flatpak", "dev.example.myapp.yml"))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte("x64"))
	for _, want := range []string{"id: dev.example.myapp\n", "command: \"My App\"\n",
		`url: "https://github.com/me/my-app/releases/download/v1.0.0/my-app-1.0.0-linux-x64.tar.gz"`,
		"sha256: " + hex.EncodeToString(sum[:]), "only-arches: [x86_64]", `ln -s "/app/lib/my-app/My App" "/app/bin/My App"`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("the manifest lacks %s:\n%s", want, data)
		}
	}
	if strings.Contains(string(data), "aarch64") {
		t.Error("the manifest names an archive the release lacks")
	}
	meta, err := p.metainfo()
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		ID         string `xml:"id"`
		Launchable string `xml:"launchable"`
		License    string `xml:"project_license"`
	}
	if err := xml.Unmarshal(meta, &m); err != nil {
		t.Fatal(err)
	}
	if m.ID != p.ID || m.Launchable != p.ID+".desktop" || m.License != "LicenseRef-proprietary" {
		t.Errorf("the metadata says %+v", m)
	}
}
