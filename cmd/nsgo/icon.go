package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// assetsDir is the directory beside nsgo.yaml for project files that
// packages do not carry, such as the icons they are made from.
const assetsDir = "assets"

// resolveIcons makes nsgo.yaml's icon paths absolute, or finds the
// defaults in assets: AppIcon.icon, AppIcon.icns and Icon.png, and for
// dev.icon the same names ending in -dev. A path nsgo.yaml names must
// exist; a missing default only leaves the app without that icon.
func (p *project) resolveIcons() error {
	if err := p.resolveIconSet(&p.Icon, "icon", ""); err != nil {
		return err
	}
	return p.resolveIconSet(&p.Dev.Icon, "dev.icon", "-dev")
}

func (p *project) resolveIconSet(set *icons, key, suffix string) error {
	var err error
	if set.Apple, err = p.iconPath(key, set.Apple, "AppIcon"+suffix+".icon", "AppIcon"+suffix+".icns"); err != nil {
		return err
	}
	if ext := filepath.Ext(set.Apple); set.Apple != "" && ext != ".icon" && ext != ".icns" {
		return fmt.Errorf("%s.apple: %s is neither an .icon nor an .icns", key, rel(set.Apple))
	}
	set.PNG, err = p.iconPath(key, set.PNG, "Icon"+suffix+".png")
	return err
}

func (p *project) iconPath(key, path string, defaults ...string) (string, error) {
	if path != "" {
		path = p.resolve(path)
		if _, err := os.Stat(path); err != nil {
			return "", fmt.Errorf("%s: %w", key, err)
		}
		return path, nil
	}
	for _, name := range defaults {
		path := filepath.Join(p.Dir, assetsDir, name)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", nil
}

// useDevIcons points icon at the files dev.icon has, for nsgo dev's
// builds.
func (p *project) useDevIcons() {
	if p.Dev.Icon.Apple != "" {
		p.Icon.Apple = p.Dev.Icon.Apple
	}
	if p.Dev.Icon.PNG != "" {
		p.Icon.PNG = p.Dev.Icon.PNG
	}
}

// iconImage decodes the project's PNG icon, nil when it has none.
func (p *project) iconImage() (image.Image, error) {
	if p.Icon.PNG == "" {
		return nil, nil
	}
	return readIcon(p.Icon.PNG)
}

// readIcon decodes a square PNG icon.
func readIcon(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("icon %s: %w", rel(path), err)
	}
	if b := img.Bounds(); b.Dx() != b.Dy() {
		return nil, fmt.Errorf("icon %s is %dx%d; use a square image", rel(path), b.Dx(), b.Dy())
	}
	return img, nil
}

// scaledPNG encodes img at size×size pixels, averaging the source pixels
// each one covers.
func scaledPNG(img image.Image, size int) []byte {
	b := img.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := range size {
		y0 := b.Min.Y + y*b.Dy()/size
		y1 := max(b.Min.Y+(y+1)*b.Dy()/size, y0+1)
		for x := range size {
			x0 := b.Min.X + x*b.Dx()/size
			x1 := max(b.Min.X+(x+1)*b.Dx()/size, x0+1)
			// Average premultiplied colors, so transparent pixels add
			// no color to the edges.
			var r, g, bl, a, n uint64
			for sy := y0; sy < y1; sy++ {
				for sx := x0; sx < x1; sx++ {
					cr, cg, cb, ca := img.At(sx, sy).RGBA()
					r, g, bl, a, n = r+uint64(cr), g+uint64(cg), bl+uint64(cb), a+uint64(ca), n+1
				}
			}
			dst.Set(x, y, color.RGBA64{uint16(r / n), uint16(g / n), uint16(bl / n), uint16(a / n)})
		}
	}
	var out bytes.Buffer
	png.Encode(&out, dst)
	return out.Bytes()
}

// writeMacIcon puts the app's icon into a macOS bundle's Resources and
// returns the Info.plist entries naming it. An Icon Composer .icon
// compiles with actool, an .icns is copied, and a PNG becomes an .icns.
func writeMacIcon(ctx context.Context, p *project, resources string) (string, error) {
	switch filepath.Ext(p.Icon.Apple) {
	case ".icon":
		return compileIcon(ctx, p.Icon.Apple, resources, "macosx", "11.0")
	case ".icns":
		data, err := os.ReadFile(p.Icon.Apple)
		if err != nil {
			return "", err
		}
		return iconFilePlist, os.WriteFile(filepath.Join(resources, "AppIcon.icns"), data, 0o644)
	}
	img, err := p.iconImage()
	if img == nil || err != nil {
		return "", err
	}
	return iconFilePlist, os.WriteFile(filepath.Join(resources, "AppIcon.icns"), icns(img), 0o644)
}

const iconFilePlist = "\t<key>CFBundleIconFile</key><string>AppIcon</string>\n"

// icnsTypes are the PNG entries of an .icns by their pixel size; each
// size above 16 also serves the half size at 2x.
var icnsTypes = []struct {
	kind string
	size int
}{
	{"icp4", 16}, {"icp5", 32}, {"ic11", 32}, {"ic12", 64}, {"ic07", 128},
	{"ic08", 256}, {"ic13", 256}, {"ic09", 512}, {"ic14", 512}, {"ic10", 1024},
}

// icns writes img as an .icns of PNG entries, so no Apple tool is needed.
func icns(img image.Image) []byte {
	pngs := map[int][]byte{}
	var body bytes.Buffer
	for _, t := range icnsTypes {
		if pngs[t.size] == nil {
			pngs[t.size] = scaledPNG(img, t.size)
		}
		body.WriteString(t.kind)
		binary.Write(&body, binary.BigEndian, uint32(8+len(pngs[t.size])))
		body.Write(pngs[t.size])
	}
	var out bytes.Buffer
	out.WriteString("icns")
	binary.Write(&out, binary.BigEndian, uint32(8+body.Len()))
	out.Write(body.Bytes())
	return out.Bytes()
}

// writeIOSIcon compiles the app's icon into an iOS bundle and returns the
// Info.plist entries naming it: the Icon Composer .icon when there is
// one, else the PNG through an asset catalog.
func writeIOSIcon(ctx context.Context, p *project, app string, simulator bool) (string, error) {
	sdk := "iphoneos"
	if simulator {
		sdk = "iphonesimulator"
	}
	if filepath.Ext(p.Icon.Apple) == ".icon" {
		return compileIcon(ctx, p.Icon.Apple, app, sdk, "15.0")
	}
	img, err := p.iconImage()
	if img == nil || err != nil {
		return "", err
	}
	tmp, err := os.MkdirTemp("", "nsgo-icon-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	set := filepath.Join(tmp, "Assets.xcassets", "AppIcon.appiconset")
	if err := os.MkdirAll(set, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(set, "Icon.png"), scaledPNG(img, 1024), 0o644); err != nil {
		return "", err
	}
	contents := `{"images":[{"filename":"Icon.png","idiom":"universal","platform":"ios","size":"1024x1024"}],"info":{"author":"nsgo","version":1}}`
	if err := os.WriteFile(filepath.Join(set, "Contents.json"), []byte(contents), 0o644); err != nil {
		return "", err
	}
	return compileIcon(ctx, filepath.Join(tmp, "Assets.xcassets"), app, sdk, "15.0")
}

// compileIcon runs actool on an Icon Composer .icon or an asset catalog
// holding AppIcon, writing Assets.car and the platform's icon files into
// out, and returns the Info.plist entries actool says name them. actool
// takes seconds, so the user's cache keeps its output by actool's
// version, the arguments and the input's files, and builds after the
// first copy it.
func compileIcon(ctx context.Context, input, out, platform, minimum string) (string, error) {
	name := "AppIcon"
	if filepath.Ext(input) == ".icon" {
		name = strings.TrimSuffix(filepath.Base(input), ".icon")
	}
	args := []string{"--platform", platform, "--minimum-deployment-target", minimum, "--app-icon", name}
	if platform != "macosx" {
		args = append(args, "--target-device", "iphone", "--target-device", "ipad")
	}
	version, err := output(ctx, "xcrun", "actool", "--version")
	if err != nil {
		return "", fmt.Errorf("the icon needs Xcode's actool: %w", err)
	}
	sum := sha256.New()
	fmt.Fprintf(sum, "%s\n%q\n", version, args)
	err = filepath.WalkDir(input, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		fmt.Fprintf(sum, "%q %d\n", strings.TrimPrefix(path, input), len(data))
		sum.Write(data)
		return err
	})
	if err != nil {
		return "", err
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(cache, "nsgo", "icons", hex.EncodeToString(sum.Sum(nil)[:16]))
	if _, err := os.Stat(dir); err != nil {
		if err := runActool(ctx, input, dir, args); err != nil {
			return "", err
		}
	}
	data, err := os.ReadFile(filepath.Join(dir, "Info.plist"))
	if err != nil {
		return "", err
	}
	files := filepath.Join(dir, "files")
	return plistEntries(string(data)), filepath.WalkDir(files, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		target := filepath.Join(out, strings.TrimPrefix(path, files))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

// runActool compiles input into dir: the icon files in files, and the
// partial Info.plist naming them.
func runActool(ctx context.Context, input, dir string, args []string) error {
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(filepath.Dir(dir), "tmp-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	partial := filepath.Join(tmp, "Info.plist")
	args = append([]string{"actool", input, "--compile", filepath.Join(tmp, "files"),
		"--output-partial-info-plist", partial}, args...)
	if err := os.MkdirAll(filepath.Join(tmp, "files"), 0o755); err != nil {
		return err
	}
	if err := run(ctx, "xcrun", args...); err != nil {
		return fmt.Errorf("the icon needs Xcode's actool: %w", err)
	}
	data, err := os.ReadFile(partial)
	if err != nil {
		return err
	}
	// actool reports problems in its output and still succeeds; an icon
	// it could not compile leaves the plist empty.
	if plistEntries(string(data)) == "" {
		return fmt.Errorf("actool compiled no icon from %s", rel(input))
	}
	// Another build may have filled dir meanwhile, with the same files.
	if err := os.Rename(tmp, dir); err != nil {
		if _, statErr := os.Stat(dir); statErr != nil {
			return err
		}
	}
	return nil
}

// plistEntries returns the keys and values of a plist's top dictionary.
func plistEntries(plist string) string {
	start := strings.Index(plist, "<dict>")
	end := strings.LastIndex(plist, "</dict>")
	if start < 0 || end < start {
		return ""
	}
	return strings.TrimSpace(plist[start+len("<dict>") : end])
}

// writeLinuxIcon puts the icon beside a Linux executable as <id>.png,
// with a desktop entry that names it.
func writeLinuxIcon(p *project, dir string) error {
	img, err := p.iconImage()
	if img == nil || err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, p.ID+".png"), scaledPNG(img, 512), 0o644); err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(dir, p.ID+".desktop"))
	if err != nil {
		return err
	}
	defer f.Close()
	return desktopEntry.Execute(f, p)
}

var desktopEntry = template.Must(template.New("desktop").Parse(`[Desktop Entry]
Type=Application
Name={{.Name}}
Exec="{{.Name}}"
Icon={{.ID}}
Terminal=false
`))

// writeWindowsIcon writes the icon as a resource object into the app's
// package directory, so go build links it into the executable, and
// returns the function that removes it again. Go only links .syso files
// from the package's own directory.
func writeWindowsIcon(p *project, goarch string) (func(), error) {
	img, err := p.iconImage()
	if img == nil || err != nil {
		return func() {}, err
	}
	var images [][]byte
	for _, size := range windowsIconSizes {
		images = append(images, scaledPNG(img, size))
	}
	syso, err := iconResource(images, windowsIconSizes, goarch)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(p.Dir, "nsgo_icon_windows_"+goarch+".syso")
	if err := os.WriteFile(path, syso, 0o644); err != nil {
		return nil, err
	}
	return func() { os.Remove(path) }, nil
}

// windowsIconSizes are the sizes Explorer, the taskbar and title bars
// pick from at common scales.
var windowsIconSizes = []int{16, 20, 24, 32, 40, 48, 64, 256}
