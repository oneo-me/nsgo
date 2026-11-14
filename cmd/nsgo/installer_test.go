package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestPackageSlugs(t *testing.T) {
	for _, c := range []struct{ name, id, want string }{
		{"Gallery", "dev.oneo.gallery", "gallery"},
		{"My App 2", "dev.example.app", "my-app-2"},
		{"  Café--Tools ", "dev.example.cafe", "caf-tools"},
		{"画廊", "dev.oneo.Gallery", "gallery"},
		{"画廊", "画廊", "app"},
	} {
		p := &project{Name: c.name, ID: c.id}
		if got := p.packageSlug(); got != c.want {
			t.Errorf("packageSlug(%q, %q) = %q, want %q", c.name, c.id, got, c.want)
		}
	}
}

func TestLinuxVersions(t *testing.T) {
	for _, c := range []struct{ v, deb, arch string }{
		{"1.2.0", "1.2.0", "1.2.0"},
		{"v1.2.0-beta.1", "1.2.0~beta.1", "1.2.0_beta.1"},
		{"1.2.0+build.5", "1.2.0~build.5", "1.2.0_build.5"},
	} {
		if got := debVersion(c.v); got != c.deb {
			t.Errorf("debVersion(%q) = %q, want %q", c.v, got, c.deb)
		}
		if got := archVersion(c.v); got != c.arch {
			t.Errorf("archVersion(%q) = %q, want %q", c.v, got, c.arch)
		}
	}
}

// The script installs every file of the package but itself, its icon and
// the setup, again where the app already is, and its uninstaller removes
// those files alone.
func TestNSISScript(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"locales/en.yaml":      "appName: My App\n",
		"locales/ja.yaml":      "appName: アプリ\n",
		"locales/zh-Hans.yaml": "appName: 应用\n",
		"locales/zh-Hant.yaml": "appName: 應用\n",
		"locales/eo.yaml":      "appName: Apo\n",
	})
	pkg := filepath.Join(dir, "pkg")
	writeFiles(t, pkg, map[string]string{
		"MyApp.exe": "exe", "fonts/a$b.ttf": "font", "MyApp.nsi": "old",
		"myapp-0.9.0-windows-x64-setup.exe": "old",
	})
	p := &project{Dir: dir, Name: "MyApp", ID: "dev.example.app", Version: "1.2.0-beta.1"}
	p.I18n.Dir, p.I18n.Source = filepath.Join(dir, "locales"), "en"
	d, err := nsisScript(p, pkg, "arm64")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, l := range d.Languages {
		names = append(names, l.Name)
	}
	if got := strings.Join(names, ","); got != "English,Japanese,SimpChinese,TradChinese" {
		t.Errorf("languages %s", got)
	}
	var out bytes.Buffer
	if err := nsisTemplate.Execute(&out, d); err != nil {
		t.Fatal(err)
	}
	script := out.String()
	for _, want := range []string{
		`Name "My App" "My App"`,
		`OutFile "myapp-1.2.0-beta.1-windows-arm64-setup.exe"`,
		`InstallDir "$LOCALAPPDATA\Programs\My App"`,
		`InstallDirRegKey HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\dev.example.app" "InstallLocation"`,
		`WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\dev.example.app" "InstallLocation" "$INSTDIR"`,
		`File /r /x "MyApp.nsi" /x "*-setup.exe" "*"`,
		`LangString NSGO_SHORTCUT ${LANG_SimpChinese} "创建桌面快捷方式"`,
		`LangString NSGO_RUNNING ${LANG_Japanese} "My App は実行中です。終了して続行しますか？"`,
		`CreateShortCut "$SMPROGRAMS\My App.lnk" "$INSTDIR\MyApp.exe"`,
		`ExecWait '"$INSTDIR\MyApp.exe" --register-toast "$SMPROGRAMS\My App.lnk"'`,
		`ExecWait '"$INSTDIR\MyApp.exe" --unregister-toast'`,
		`Delete "$INSTDIR\fonts\a$$b.ttf"`,
		`RMDir "$INSTDIR\fonts"`,
		`${IfNot} ${IsNativeARM64}`,
		`${GetOptions} $0 "/RELAUNCH" $1`,
		`Exec '"$INSTDIR\MyApp.exe"'`,
		`VIProductVersion "1.2.0.0"`,
		`$$_.ExecutablePath.StartsWith($$d, 'OrdinalIgnoreCase')`,
	} {
		if !strings.Contains(script, want) {
			t.Errorf("script lacks %s", want)
		}
	}
	for _, unwanted := range []string{`Delete "$INSTDIR\MyApp.nsi"`, `myapp-0.9.0`, `RMDir /r`} {
		if strings.Contains(script, unwanted) {
			t.Errorf("script holds %s", unwanted)
		}
	}
	// A quote in the name stays inside the command lines the installer runs.
	d.Name = "Bob's `Notes`"
	out.Reset()
	if err := nsisTemplate.Execute(&out, d); err != nil {
		t.Fatal(err)
	}
	if want := "ExecWait '\"$INSTDIR\\MyApp.exe\" --register-toast \"$SMPROGRAMS\\Bob$\\'s `Notes`.lnk\"'"; !strings.Contains(out.String(), want) {
		t.Errorf("script lacks %s", want)
	}
	for v, want := range map[string]string{"1.2.3": "1.2.3.0", "v2": "2.0.0.0", "1.2.3.4.5": "1.2.3.4"} {
		if got := nsisVersion(v); got != want {
			t.Errorf("nsisVersion(%q) = %s, want %s", v, got, want)
		}
	}
}

// makensis compiles the script, which reaches every file, for a name with
// quotes in it.
func TestNSISCompiles(t *testing.T) {
	compiler := nsisCompiler()
	if compiler == "" {
		t.Skip("no makensis")
	}
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"App.exe": "exe", "fonts/a.ttf": "font"})
	locales := t.TempDir()
	writeFiles(t, locales, map[string]string{"en.yaml": "appName: \"Bob's `Notes`\"\n"})
	p := &project{Dir: dir, Name: "App", ID: "dev.example.app", Version: "1.0.0", Release: true}
	p.I18n.Dir, p.I18n.Source = locales, "en"
	if _, err := buildNSIS(t.Context(), p, dir, "x64"); err != nil {
		t.Fatal(err)
	}
	// Icons of their own, a PNG and an .ico of one size, other than the
	// app's.
	icons := t.TempDir()
	img := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	var b bytes.Buffer
	png.Encode(&b, img)
	one := image.NewNRGBA(image.Rect(0, 0, 48, 48))
	var ico bytes.Buffer
	binary.Write(&ico, binary.LittleEndian, [3]uint16{0, 1, 1})
	var pngOne bytes.Buffer
	png.Encode(&pngOne, one)
	binary.Write(&ico, binary.LittleEndian, [4]uint8{48, 48, 0, 0})
	binary.Write(&ico, binary.LittleEndian, [2]uint16{1, 32})
	binary.Write(&ico, binary.LittleEndian, [2]uint32{uint32(pngOne.Len()), 22})
	ico.Write(pngOne.Bytes())
	writeFiles(t, icons, map[string]string{"Setup.png": b.String(), "Uninstall.ico": ico.String()})
	p.Windows.InstallerIcon = filepath.Join(icons, "Setup.png")
	p.Windows.UninstallerIcon = filepath.Join(icons, "Uninstall.ico")
	setup, err := buildNSIS(t.Context(), p, dir, "x64")
	if err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(setup); err != nil || info.Size() == 0 {
		t.Fatalf("no installer: %v", err)
	}
}

func TestICOFiles(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	img.Set(1, 1, color.White)
	ico := icoFile(img)
	var head [3]uint16
	binary.Read(bytes.NewReader(ico), binary.LittleEndian, &head)
	if head != [3]uint16{0, 1, uint16(len(windowsIconSizes))} {
		t.Fatalf("header %v", head)
	}
	for i, size := range windowsIconSizes {
		entry := ico[6+16*i:]
		length := binary.LittleEndian.Uint32(entry[8:])
		offset := binary.LittleEndian.Uint32(entry[12:])
		if int(entry[0]) != size%256 || !bytes.HasPrefix(ico[offset:offset+length], []byte("\x89PNG")) {
			t.Errorf("image %d (%d px) is not a PNG at its offset", i, size)
		}
	}
}

// Both Linux packages install the package's directory in /opt, a command,
// a desktop entry and the icon, and leave out what the tool wrote beside
// the app.
func TestLinuxPackages(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"App":                          "binary",
		"fonts/a.ttf":                  "font",
		"dev.example.app.png":          "icon",
		"dev.example.app.desktop":      "entry",
		"app-0.9.0-linux-x64.deb":      "old",
		"app-0.9.0-linux-x64.AppImage": "old",
	})
	os.Chmod(filepath.Join(dir, "App"), 0o755)
	p := &project{Dir: dir, Name: "App", ID: "dev.example.app", Version: "1.0.0-rc.1"}
	files, err := linuxPackageFiles(p, dir)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]linuxPackageEntry{}
	for _, f := range files {
		if _, ok := byName[f.name]; ok {
			t.Errorf("%s is listed twice", f.name)
		}
		byName[f.name] = f
	}
	for _, name := range []string{"opt", "opt/app", "opt/app/App", "opt/app/fonts/a.ttf", "usr/bin/app",
		"usr/share/applications/dev.example.app.desktop"} {
		if _, ok := byName[name]; !ok {
			t.Errorf("packages lack %s", name)
		}
	}
	for _, name := range []string{"opt/app/dev.example.app.png", "opt/app/dev.example.app.desktop", "opt/app/app-0.9.0-linux-x64.deb", "opt/app/app-0.9.0-linux-x64.AppImage"} {
		if _, ok := byName[name]; ok {
			t.Errorf("packages install %s", name)
		}
	}
	if f := byName["opt/app/App"]; f.mode != 0o755 {
		t.Errorf("the executable's mode is %o", f.mode)
	}
	if f := byName["usr/bin/app"]; f.link != "/opt/app/App" {
		t.Errorf("the command links to %q", f.link)
	}
	if entry := string(byName["usr/share/applications/dev.example.app.desktop"].data); !strings.Contains(entry, `Exec="/opt/app/App"`) ||
		!strings.Contains(entry, "StartupWMClass=App") {
		t.Errorf("desktop entry:\n%s", entry)
	}

	deb, err := debPackage(p, files, "amd64")
	if err != nil {
		t.Fatal(err)
	}
	members := arMembers(t, deb)
	if string(members["debian-binary"]) != "2.0\n" {
		t.Errorf("debian-binary is %q", members["debian-binary"])
	}
	control := tarFiles(t, members["control.tar.gz"])["./control"]
	for _, want := range []string{"Package: app\n", "Version: 1.0.0~rc.1\n", "Architecture: amd64\n", "Maintainer: App\n", "Depends: libgtk-4-1\n"} {
		if !strings.Contains(control, want) {
			t.Errorf("control lacks %q:\n%s", want, control)
		}
	}
	if got := tarFiles(t, members["data.tar.gz"])["./opt/app/fonts/a.ttf"]; got != "font" {
		t.Errorf("data.tar.gz holds %q for the font", got)
	}

	p.Linux.Maintainer = "Jane <jane@example.com>"
	pkg, err := archPackage(p, files, "arm64")
	if err != nil {
		t.Fatal(err)
	}
	gz, err := gzip.NewReader(bytes.NewReader(pkg))
	if err != nil {
		t.Fatal(err)
	}
	first, err := tar.NewReader(gz).Next()
	if err != nil || first.Name != ".PKGINFO" {
		t.Fatalf("the package starts with %v, %v", first, err)
	}
	info := tarFiles(t, pkg)[".PKGINFO"]
	for _, want := range []string{"pkgname = app\n", "pkgver = 1.0.0_rc.1-1\n", "arch = aarch64\n", "packager = Jane <jane@example.com>\n", "depend = gtk4\n"} {
		if !strings.Contains(info, want) {
			t.Errorf(".PKGINFO lacks %q:\n%s", want, info)
		}
	}
}

// arMembers reads the members of an ar archive by name.
func arMembers(t *testing.T, data []byte) map[string][]byte {
	t.Helper()
	if !bytes.HasPrefix(data, []byte("!<arch>\n")) {
		t.Fatal("not an ar archive")
	}
	members := map[string][]byte{}
	for rest := data[8:]; len(rest) >= 60; {
		name := strings.TrimSpace(string(rest[:16]))
		var size int
		for _, c := range strings.TrimSpace(string(rest[48:58])) {
			size = size*10 + int(c-'0')
		}
		members[name] = rest[60 : 60+size]
		rest = rest[60+size+size%2:]
	}
	return members
}

// tarFiles reads the regular files of a gzip-compressed tar archive.
func tarFiles(t *testing.T, data []byte) map[string]string {
	t.Helper()
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return files
		}
		if err != nil {
			t.Fatal(err)
		}
		if h.Typeflag == tar.TypeReg {
			b, _ := io.ReadAll(tr)
			files[h.Name] = string(b)
		}
	}
}

// The disk image's layout is one .DS_Store page placing the app and the
// Applications link.
func TestDiskImageLayout(t *testing.T) {
	data, err := dsStore(dmgRecords("App.app", layoutFor(dmgDefaultWidth, dmgDefaultHeight)))
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 4+0x4000 || string(data[4:8]) != "Bud1" {
		t.Fatalf("%d bytes starting %q", len(data), data[:8])
	}
	for _, name := range []string{"App.app", "Applications"} {
		var utf16 []byte
		for _, r := range name {
			utf16 = append(utf16, 0, byte(r))
		}
		if !bytes.Contains(data, append(utf16, "Iloc"...)) {
			t.Errorf("%s has no location", name)
		}
	}
}

// The releases directory keeps one version: this version's files from
// other builds stay, another version's go; installers move in, other
// packages' files and directories are copied, with checksums for files.
func TestReleaseDirectory(t *testing.T) {
	dir := t.TempDir()
	p := &project{Dir: dir, Name: "App", ID: "dev.example.app", Version: "1.1.0"}
	p.Build.Output = filepath.Join(dir, "dist")
	writeFiles(t, p.releaseDir(), map[string]string{".version": "1.0.0\n", "app-1.0.0-macos-arm64.dmg": "old"})
	if err := p.cleanReleaseDir(); err != nil {
		t.Fatal(err)
	}
	writeFiles(t, p.releaseDir(), map[string]string{"app-1.1.0-linux-x64.deb": "from another machine"})
	if err := p.cleanReleaseDir(); err != nil {
		t.Fatal(err)
	}
	pkg := filepath.Join(dir, "pkg")
	writeFiles(t, pkg, map[string]string{"app-1.1.0-macos-arm64.dmg": "image", "App.apk": "apk", "site/index.html": "page"})
	if err := p.releaseAssets(filepath.Join(pkg, "app-1.1.0-macos-arm64.dmg"), ""); err != nil {
		t.Fatal(err)
	}
	if err := p.releaseCopy(filepath.Join(pkg, "App.apk"), "app-1.1.0-android-arm64.apk"); err != nil {
		t.Fatal(err)
	}
	if err := p.releaseZip(filepath.Join(pkg, "site"), "app-1.1.0-web.zip"); err != nil {
		t.Fatal(err)
	}
	if err := p.releaseTree(filepath.Join(pkg, "site"), "miniprogram/wechat"); err != nil {
		t.Fatal(err)
	}
	var got []string
	filepath.WalkDir(p.releaseDir(), func(path string, d fs.DirEntry, err error) error {
		if !d.IsDir() {
			rel, _ := filepath.Rel(p.releaseDir(), path)
			got = append(got, filepath.ToSlash(rel))
		}
		return nil
	})
	want := ".version,app-1.1.0-android-arm64.apk,app-1.1.0-android-arm64.apk.sha256,app-1.1.0-linux-x64.deb,app-1.1.0-macos-arm64.dmg,app-1.1.0-macos-arm64.dmg.sha256,app-1.1.0-web.zip,app-1.1.0-web.zip.sha256,miniprogram/wechat/index.html"
	if strings.Join(got, ",") != want {
		t.Fatalf("releases holds %v, want %s", got, want)
	}
	if _, err := os.Stat(filepath.Join(pkg, "App.apk")); err != nil {
		t.Error("the APK left its package")
	}
	sum, _ := os.ReadFile(filepath.Join(p.releaseDir(), "app-1.1.0-macos-arm64.dmg.sha256"))
	if want := "6105d6cc76af400325e94d588ce511be5bfdbb73b437dc51eca43917d7a43e3d  app-1.1.0-macos-arm64.dmg\n"; string(sum) != want {
		t.Errorf("checksum %q, want %q", sum, want)
	}
	zr, err := zip.OpenReader(filepath.Join(p.releaseDir(), "app-1.1.0-web.zip"))
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	if len(zr.File) != 1 || zr.File[0].Name != "index.html" {
		t.Errorf("the site's archive holds %v, want index.html at its top", zr.File)
	}
}

func TestMiniProgramTargetGroup(t *testing.T) {
	got, err := expandTargets([]string{"web", "miniprogram", "miniprogram_qq"})
	if err != nil {
		t.Fatal(err)
	}
	if want := "web,miniprogram_wechat,miniprogram_qq,miniprogram_alipay,miniprogram_douyin"; strings.Join(got, ",") != want {
		t.Errorf("targets %v, want %s", got, want)
	}
	all, _ := expandTargets([]string{"all"})
	if slices.ContainsFunc(all, func(t string) bool { return strings.HasPrefix(t, "miniprogram_") }) {
		t.Error("all holds mini programs")
	}
}

// The install scripts name the app's repository, files and version, and
// the shell one parses; builds with no desktop package write none.
func TestInstallScripts(t *testing.T) {
	dir := t.TempDir()
	p := &project{Dir: dir, Name: "My App", ID: "dev.example.app", Version: "1.2.0-beta.1"}
	p.Build.Output = filepath.Join(dir, "dist")
	if err := p.writeInstallScripts([]string{"macos_arm64"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(p.releaseDir(), "install.sh")); err == nil {
		t.Fatal("scripts written without release.github")
	}
	p.Releases.GitHub = "me/my-app"
	if err := p.writeInstallScripts([]string{"web", "android_arm64"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(p.releaseDir(), "install.sh")); err == nil {
		t.Fatal("scripts written without a desktop target")
	}
	if err := p.writeInstallScripts([]string{"web", "linux_x64"}); err != nil {
		t.Fatal(err)
	}
	sh, _ := os.ReadFile(filepath.Join(p.releaseDir(), "install.sh"))
	ps, _ := os.ReadFile(filepath.Join(p.releaseDir(), "install.ps1"))
	for _, want := range []string{"repository='me/my-app'\n", "slug='my-app'\n", "app='My App.app'\n", "version=\"${MY_APP_VERSION:-}\"",
		"version='1.2.0-beta.1'\n", "tag='v'\"$version\"", "https://github.com/me/my-app/releases/latest/download/install.sh"} {
		if !strings.Contains(string(sh), want) {
			t.Errorf("install.sh lacks %q", want)
		}
	}
	for _, want := range []string{"$repository = 'me/my-app'\r\n", "$env:MY_APP_VERSION", "\"$slug-$version-windows-$arch-setup.exe\""} {
		if !strings.Contains(string(ps), want) {
			t.Errorf("install.ps1 lacks %q", want)
		}
	}
	if strings.Contains(string(sh)+string(ps), "@@") {
		t.Error("a script keeps a placeholder")
	}
	if out, err := exec.Command("sh", "-n", filepath.Join(p.releaseDir(), "install.sh")).CombinedOutput(); err != nil {
		t.Errorf("sh -n: %v\n%s", err, out)
	}
	// The shell's version conversions agree with the packages' names.
	for _, v := range []string{"1.2.0", "1.2.0-beta.1", "1.2.0-rc-2+build.5"} {
		deb, _ := exec.Command("sh", "-c", `echo "$1" | sed -e 's/-/~/' -e 's/-/./g' -e 's/+/~/g'`, "sh", v).Output()
		arch, _ := exec.Command("sh", "-c", `echo "$1" | sed 's/[-+]/_/g'`, "sh", v).Output()
		if got := strings.TrimSpace(string(deb)); got != debVersion(v) {
			t.Errorf("install.sh's Debian version of %s is %s, want %s", v, got, debVersion(v))
		}
		if got := strings.TrimSpace(string(arch)); got != archVersion(v) {
			t.Errorf("install.sh's Arch version of %s is %s, want %s", v, got, archVersion(v))
		}
	}
}

func TestPowerShellQuotes(t *testing.T) {
	for s, want := range map[string]string{
		"":       "''",
		"it's":   "'it''s'",
		"画廊 App": "([string]'' + [char]0x753B + [char]0x5ECA + ' App')",
		"a😀":     "([string]'' + 'a' + [char]0xD83D + [char]0xDE00)",
	} {
		if got := psQuote(s); got != want {
			t.Errorf("psQuote(%q) = %s, want %s", s, got, want)
		}
	}
}

func TestNotarization(t *testing.T) {
	r, err := parseNotaryResult([]byte("Conducting pre-submission checks...\n{\"id\":\"abc\",\"status\":\"Invalid\",\"message\":\"Processing complete\"}\n"))
	if err != nil || r.ID != "abc" || r.Status != "Invalid" {
		t.Fatalf("parsed %+v, %v", r, err)
	}
	for _, k := range notaryKeyEnv {
		t.Setenv(k, "")
	}
	p := &project{Release: true}
	if p.notarizing() {
		t.Fatal("notarizing without credentials")
	}
	p.Sign.Notarize = "nsgo"
	if err := p.checkNotarizing(); err == nil {
		t.Error("notarizing without a Developer ID identity")
	}
	p.Sign.Identity = "Developer ID Application: Jane (ABCDE12345)"
	if args, _ := p.notaryCredentials(); strings.Join(args, " ") != "--keychain-profile nsgo" {
		t.Errorf("credentials %v", args)
	}
	if args := p.macSignArgs("A.app"); strings.Join(args, " ") != "--force --sign "+p.Sign.Identity+" --options runtime --timestamp A.app" {
		t.Errorf("codesign %v", args)
	}
	t.Setenv("NSGO_NOTARY_KEY", "key.p8")
	if _, err := p.notaryCredentials(); err == nil {
		t.Error("an API key without its ID and issuer")
	}
	t.Setenv("NSGO_NOTARY_KEY_ID", "KEY")
	t.Setenv("NSGO_NOTARY_ISSUER", "ISSUER")
	if args, _ := p.notaryCredentials(); strings.Join(args, " ") != "--key key.p8 --key-id KEY --issuer ISSUER" {
		t.Errorf("credentials %v", args)
	}
	p.Apple.Info = map[string]string{"NSMicrophoneUsageDescription": "Records"}
	if ents := p.hardenedEntitlements(); ents["com.apple.security.device.audio-input"] != true || len(ents) != 1 {
		t.Errorf("entitlements %v", ents)
	}
}

// A background's alias holds the volume's and the file's names, IDs and
// path, and the layout names it as a picture.
func TestDiskImageBackground(t *testing.T) {
	alias := macAlias(macAliasInfo{volume: "画廊 Gallery", volumeCreated: 1, parentCNID: 17, parentName: ".background",
		fileCNID: 18, fileCreated: 2, fileName: "background.tiff", path: "/.background/background.tiff"})
	if got := int(binary.BigEndian.Uint16(alias[4:])); got != len(alias) || binary.BigEndian.Uint16(alias[6:]) != 2 {
		t.Fatalf("alias of %d bytes says %d, version %d", len(alias), got, binary.BigEndian.Uint16(alias[6:]))
	}
	if binary.BigEndian.Uint32(alias[46:]) != 17 || binary.BigEndian.Uint32(alias[114:]) != 18 || string(alias[51:51+alias[50]]) != "background.tiff" {
		t.Error("the alias's IDs or name are misplaced")
	}
	for _, want := range []string{"/.background/background.tiff", "/Volumes/画廊 Gallery"} {
		if !bytes.Contains(alias, []byte(want)) {
			t.Errorf("alias lacks %s", want)
		}
	}
	if !bytes.HasSuffix(alias, []byte{0xff, 0xff, 0, 0}) {
		t.Error("alias does not end")
	}
	l := layoutFor(dmgDefaultWidth, dmgDefaultHeight)
	l.BackgroundAlias = alias
	data, err := dsStore(dmgRecords("App.app", l))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, alias) || !bytes.Contains(data, []byte("backgroundImageAlias")) {
		t.Error("the layout lacks the background's alias")
	}

	// The built-in picture: the gradient at the corners, the arrow's ink
	// between the icons, at both resolutions.
	for _, scale := range []int{1, 2} {
		img := arrowBackground(l, scale)
		if b := img.Bounds(); b.Dx() != l.Width*scale || b.Dy() != l.Height*scale {
			t.Fatalf("@%dx is %v", scale, b)
		}
		mid := img.NRGBAAt((l.AppX+l.ApplicationsX)/2*scale, l.AppY*scale)
		corner := img.NRGBAAt(0, 0)
		if mid.R >= corner.R-0x30 {
			t.Errorf("@%dx: no arrow between the icons (%v against %v)", scale, mid, corner)
		}
		if above := img.NRGBAAt((l.AppX+l.ApplicationsX)/2*scale, (l.AppY-40)*scale); above.R < corner.R-0x10 {
			t.Errorf("@%dx: ink above the arrow", scale)
		}
	}
}

// Installers' pictures and icons resolve against the app, must exist and
// take their formats, and an icon is the app's unless one is given.
func TestInstallerFiles(t *testing.T) {
	dir := t.TempDir()
	icon := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	var b bytes.Buffer
	png.Encode(&b, icon)
	writeFiles(t, dir, map[string]string{"assets/Icon.png": b.String(), "assets/Setup.ico": "ico", "assets/bg.jpg": "jpg"})
	p := &project{Dir: dir}
	p.Icon.PNG = filepath.Join(dir, "assets/Icon.png")
	p.Windows.InstallerIcon = "assets/Setup.ico"
	if err := p.resolveInstallerFiles(); err != nil {
		t.Fatal(err)
	}
	if data, _ := p.iconFile(p.Windows.InstallerIcon, ".ico"); string(data) != "ico" {
		t.Errorf("an .ico is not taken as it is: %q", data)
	}
	if data, _ := p.iconFile("", ".ico"); !bytes.HasPrefix(data, []byte{0, 0, 1, 0}) {
		t.Error("no .ico of the app's icon")
	}
	if data, _ := p.iconFile(p.Icon.PNG, ".icns"); !bytes.HasPrefix(data, []byte("icns")) {
		t.Error("no .icns of a PNG")
	}
	p.MacOS.DMGBackground = "assets/bg.jpg"
	if err := p.resolveInstallerFiles(); err == nil || !strings.Contains(err.Error(), "macos.dmg_background") {
		t.Errorf("a JPEG background: %v", err)
	}
	p.MacOS.DMGBackground = "assets/missing.png"
	if err := p.resolveInstallerFiles(); err == nil {
		t.Error("a missing background")
	}
}
