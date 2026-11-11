package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"hash/crc32"
	"image"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"text/template"
)

// A store build of a Windows target is the Microsoft Store's package,
// <slug>-<version>-windows-<arch>.msix, written here as Windows SDK's
// makeappx would: a ZIP of the Windows package with its logos under
// Assets, AppxManifest.xml, AppxBlockMap.xml, which hashes every file in
// 64 KB blocks, and [Content_Types].xml. Its files are stored, so each
// block is its bytes as they are. The package is unsigned: the Store
// signs what it publishes, and Windows installs an unsigned package only
// with Add-AppxPackage -AllowUnsigned, for a publisher marked as
// unsigned's.

// msixBlock is how much of a file each of the block map's hashes covers.
const msixBlock = 64 << 10

// msixLogos are the package's logos: the Start menu's tile, the taskbar's
// and the Store's, each at the size Windows shows it at 100%.
var msixLogos = []struct {
	name string
	size int
}{
	{"Square150x150Logo.png", 150},
	{"Square44x44Logo.png", 44},
	{"StoreLogo.png", 50},
}

// buildMSIX writes the Store's package of the Windows package in dir,
// built for arch, Go's, beside it, and returns its path.
func buildMSIX(ctx context.Context, p *project, dir, arch string) (string, error) {
	stage(ctx, "writing the MSIX package")
	if p.Windows.Store.Identity == "" || p.Windows.Store.Publisher == "" {
		detailIn(ctx, "set windows.store's identity and publisher to Partner Center's to upload it to the Microsoft Store")
	}
	name := p.releaseName("windows_"+goArchName(arch), ".msix")
	var files []msixFile
	err := filepath.WalkDir(dir, func(file string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dir, file)
		if strings.HasSuffix(rel, ".msix") {
			return nil
		}
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		files = append(files, msixFile{filepath.ToSlash(rel), data})
		return nil
	})
	if err != nil {
		return "", err
	}
	img, err := p.iconImage()
	if err != nil {
		return "", err
	}
	if img == nil {
		// A package must have logos; a clear one stands in.
		img = image.NewNRGBA(image.Rect(0, 0, 1, 1))
	}
	for _, l := range msixLogos {
		files = append(files, msixFile{"Assets/" + l.name, scaledPNG(img, l.size)})
	}
	manifest, err := msixManifest(p, arch)
	if err != nil {
		return "", err
	}
	files = append(files, msixFile{"AppxManifest.xml", manifest})
	data, err := writeMSIX(files)
	if err != nil {
		return "", err
	}
	out := filepath.Join(dir, name)
	return out, os.WriteFile(out, data, 0o644)
}

// msixFile is a file of the package, by its path in it.
type msixFile struct {
	name string
	data []byte
}

// writeMSIX archives files, the payload and its manifest, with the block
// map and the content types after them, as makeappx orders them.
func writeMSIX(files []msixFile) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	var blockMap bytes.Buffer
	blockMap.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="no"?>` + "\n")
	blockMap.WriteString(`<BlockMap xmlns="http://schemas.microsoft.com/appx/2010/blockmap" HashMethod="http://www.w3.org/2001/04/xmlenc#sha256">` + "\n")
	add := func(name string, data []byte) (int, error) {
		part := msixPartName(name)
		h := &zip.FileHeader{Name: part, Method: zip.Store, CRC32: crc32.ChecksumIEEE(data),
			CompressedSize64: uint64(len(data)), UncompressedSize64: uint64(len(data)),
			// 1980-01-01, the earliest a ZIP's date holds, keeps builds alike.
			ModifiedDate: 0x0021}
		w, err := zw.CreateRaw(h)
		if err != nil {
			return 0, err
		}
		_, err = w.Write(data)
		// The local header: 30 bytes, the name and no extra field.
		return 30 + len(part), err
	}
	for _, f := range files {
		lfh, err := add(f.name, f.data)
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(&blockMap, "  <File Name=\"%s\" Size=\"%d\" LfhSize=\"%d\">\n", xmlText(strings.ReplaceAll(f.name, "/", `\`)), len(f.data), lfh)
		for data := f.data; len(data) > 0; {
			n := min(len(data), msixBlock)
			sum := sha256.Sum256(data[:n])
			fmt.Fprintf(&blockMap, "    <Block Hash=\"%s\"/>\n", base64.StdEncoding.EncodeToString(sum[:]))
			data = data[n:]
		}
		blockMap.WriteString("  </File>\n")
	}
	blockMap.WriteString("</BlockMap>\n")
	if _, err := add("AppxBlockMap.xml", blockMap.Bytes()); err != nil {
		return nil, err
	}
	if _, err := add("[Content_Types].xml", msixContentTypes(files)); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// msixPartName is a file's name in the ZIP: a part name of the Open
// Packaging Conventions, which escape what URIs do not keep.
func msixPartName(name string) string {
	var b strings.Builder
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.IndexByte("/-._~!$&'()*+,;=:@[]", c) >= 0 {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

// msixContentTypes is [Content_Types].xml: a type for each extension the
// payload has, and for each file without one and the manifests by name.
func msixContentTypes(files []msixFile) []byte {
	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="no"?>` + "\n")
	b.WriteString(`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` + "\n")
	var exts []string
	var bare []string
	for _, f := range files {
		if f.name == "AppxManifest.xml" {
			continue
		}
		ext := strings.ToLower(strings.TrimPrefix(path.Ext(path.Base(f.name)), "."))
		if ext == "" {
			bare = append(bare, f.name)
		} else if !slices.Contains(exts, ext) {
			exts = append(exts, ext)
		}
	}
	slices.Sort(exts)
	for _, ext := range exts {
		fmt.Fprintf(&b, "  <Default Extension=\"%s\" ContentType=\"%s\"/>\n", xmlText(msixPartName(ext)), msixContentType(ext))
	}
	for _, name := range bare {
		fmt.Fprintf(&b, "  <Override PartName=\"/%s\" ContentType=\"application/octet-stream\"/>\n", xmlText(msixPartName(name)))
	}
	b.WriteString("  <Override PartName=\"/AppxManifest.xml\" ContentType=\"application/vnd.ms-appx.manifest+xml\"/>\n")
	b.WriteString("  <Override PartName=\"/AppxBlockMap.xml\" ContentType=\"application/vnd.ms-appx.blockmap+xml\"/>\n")
	b.WriteString("</Types>\n")
	return b.Bytes()
}

// msixContentType is the MIME type makeappx gives files of ext.
func msixContentType(ext string) string {
	switch ext {
	case "exe", "dll":
		return "application/x-msdownload"
	case "png":
		return "image/png"
	case "jpg", "jpeg":
		return "image/jpeg"
	case "gif":
		return "image/gif"
	case "svg":
		return "image/svg+xml"
	case "ico":
		return "image/vnd.microsoft.icon"
	case "xml":
		return "application/xml"
	case "json":
		return "application/json"
	case "txt":
		return "text/plain"
	case "html", "htm":
		return "text/html"
	case "css":
		return "text/css"
	case "js":
		return "application/javascript"
	case "ttf":
		return "application/x-font-ttf"
	case "otf":
		return "application/x-font-otf"
	case "woff":
		return "application/font-woff"
	case "woff2":
		return "font/woff2"
	case "wasm":
		return "application/wasm"
	}
	return "application/octet-stream"
}

// msixArch is Windows's name of Go's architecture.
var msixArch = map[string]string{"amd64": "x64", "arm64": "arm64"}

// msixVersion is the package's version: four numbers up to 65535, the
// last 0 as the Store requires, from the version's leading numbers.
func msixVersion(v string) string {
	v, _, _ = strings.Cut(strings.TrimPrefix(v, "v"), "-")
	v, _, _ = strings.Cut(v, "+")
	parts := strings.Split(v, ".")
	out := []string{"0", "0", "0", "0"}
	for i := 0; i < 3 && i < len(parts); i++ {
		if n, err := strconv.Atoi(parts[i]); err == nil && n >= 0 {
			out[i] = strconv.Itoa(min(n, 65535))
		}
	}
	return strings.Join(out, ".")
}

// msixIdentityPattern is what a package's name may hold.
var msixIdentityPattern = regexp.MustCompile(`[^A-Za-z0-9.-]+`)

// msixData is what AppxManifest.xml says.
type msixData struct {
	Identity, Publisher, PublisherName, Version, Arch string
	Name, Description, Exe                            string
	Languages                                         []string
	FileTypes                                         []msixFileType
}

// msixFileType is a kind of document the package opens.
type msixFileType struct {
	Name, DisplayName string
	Extensions        []string
}

// msixManifest is the package's AppxManifest.xml: the app, which runs
// with full trust as desktop apps do, its logos, languages and file types.
func msixManifest(p *project, arch string) ([]byte, error) {
	name, err := p.displayName()
	if err != nil {
		return nil, err
	}
	d := msixData{
		Identity:      p.Windows.Store.Identity,
		Publisher:     p.Windows.Store.Publisher,
		PublisherName: p.Windows.Store.PublisherName,
		Version:       msixVersion(p.Version),
		Arch:          msixArch[arch],
		Name:          name,
		Description:   name,
		Exe:           p.Name + ".exe",
	}
	if d.Identity == "" {
		d.Identity = strings.Trim(msixIdentityPattern.ReplaceAllString(p.ID, "-"), ".-")
		if len(d.Identity) > 50 {
			d.Identity = d.Identity[:50]
		}
	}
	if d.PublisherName == "" {
		d.PublisherName = p.maintainer()
	}
	if d.Publisher == "" {
		d.Publisher = "CN=" + strings.NewReplacer(",", " ", "=", " ", "\"", " ").Replace(d.PublisherName)
	}
	n, err := p.appNames()
	if err != nil {
		return nil, err
	}
	d.Languages = []string{"en-us"}
	if n != nil {
		d.Languages = nil
		for _, lang := range n.languages() {
			d.Languages = append(d.Languages, strings.ToLower(lang))
		}
	}
	for _, t := range p.FileTypes {
		ft := msixFileType{Name: slug(t.Extensions[0]), DisplayName: t.Name}
		if ft.Name == "" {
			ft.Name = "file"
		}
		for _, ext := range t.Extensions {
			ft.Extensions = append(ft.Extensions, "."+strings.ToLower(ext))
		}
		if ft.DisplayName == "" {
			ft.DisplayName = ft.Extensions[0]
		}
		d.FileTypes = append(d.FileTypes, ft)
	}
	var out bytes.Buffer
	err = msixManifestTemplate.Execute(&out, d)
	return out.Bytes(), err
}

var msixManifestTemplate = template.Must(template.New("msix").Funcs(template.FuncMap{"x": xmlText}).Parse(`<?xml version="1.0" encoding="utf-8"?>
<Package xmlns="http://schemas.microsoft.com/appx/manifest/foundation/windows10"
  xmlns:uap="http://schemas.microsoft.com/appx/manifest/uap/windows10"
  xmlns:rescap="http://schemas.microsoft.com/appx/manifest/foundation/windows10/restrictedcapabilities"
  IgnorableNamespaces="uap rescap">
  <Identity Name="{{x .Identity}}" Publisher="{{x .Publisher}}" Version="{{.Version}}" ProcessorArchitecture="{{.Arch}}"/>
  <Properties>
    <DisplayName>{{x .Name}}</DisplayName>
    <PublisherDisplayName>{{x .PublisherName}}</PublisherDisplayName>
    <Logo>Assets\StoreLogo.png</Logo>
  </Properties>
  <Dependencies>
    <TargetDeviceFamily Name="Windows.Desktop" MinVersion="10.0.17763.0" MaxVersionTested="10.0.26100.0"/>
  </Dependencies>
  <Resources>
{{- range .Languages}}
    <Resource Language="{{x .}}"/>
{{- end}}
  </Resources>
  <Applications>
    <Application Id="App" Executable="{{x .Exe}}" EntryPoint="Windows.FullTrustApplication">
      <uap:VisualElements DisplayName="{{x .Name}}" Description="{{x .Description}}" BackgroundColor="transparent"
        Square150x150Logo="Assets\Square150x150Logo.png" Square44x44Logo="Assets\Square44x44Logo.png"/>
{{- if .FileTypes}}
      <Extensions>
{{- range .FileTypes}}
        <uap:Extension Category="windows.fileTypeAssociation">
          <uap:FileTypeAssociation Name="{{x .Name}}">
            <uap:DisplayName>{{x .DisplayName}}</uap:DisplayName>
            <uap:SupportedFileTypes>
{{- range .Extensions}}
              <uap:FileType>{{x .}}</uap:FileType>
{{- end}}
            </uap:SupportedFileTypes>
          </uap:FileTypeAssociation>
        </uap:Extension>
{{- end}}
      </Extensions>
{{- end}}
    </Application>
  </Applications>
  <Capabilities>
    <Capability Name="internetClient"/>
    <rescap:Capability Name="runFullTrust"/>
  </Capabilities>
</Package>
`))
