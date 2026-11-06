package bundler

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io/fs"
	"mime"
	"path"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/image/font/sfnt"
	_ "golang.org/x/image/webp"

	"github.com/oneo-me/nsgo/internal/driver"
	"github.com/oneo-me/nsgo/internal/woff"
)

// Assets a module imports (NSH's plugin.rs): an image is an ImageAsset,
// its size in points read as the bundle is built and its bytes inlined
// as a data: URL registered under its asset: URL as its module runs, so
// it ships in the bundle on every target; a font is
// its family name, its TrueType or OpenType data inlined and registered
// as its module runs, a WOFF or WOFF2 file unpacked here so no text
// engine meets one.

var imageExts = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".svg": true}

var fontExts = map[string]bool{".ttf": true, ".otf": true, ".woff": true, ".woff2": true}

// isAsset reports whether a module is an asset.
func isAsset(p string) bool {
	ext := strings.ToLower(path.Ext(p))
	return imageExts[ext] || fontExts[ext]
}

// density is the pixel density a file's name gives: logo@2x.png is 2.
var densityRE = regexp.MustCompile(`@([0-9.]+)x\.[A-Za-z]+$`)

// assetModule is the module of asset p, read from fsys: its URL, its
// size in points and, for a raster image with density variants
// beside it (logo@2x.png, logo@3x.png), their scales and data: URLs, of
// which the core mounts the one for the screen's pixel ratio.
func assetModule(fsys fs.FS, p string, data []byte) (string, error) {
	ext := strings.ToLower(path.Ext(p))
	if fontExts[ext] {
		data, err := woff.Unpack(data)
		if err != nil {
			return "", fmt.Errorf("%s: cannot read the font: %w", p, err)
		}
		family, err := fontFamily(data)
		if err != nil {
			return "", fmt.Errorf("%s: cannot read the font: %w", p, err)
		}
		// Registered as the module runs, before the app mounts.
		return "require(\"nsgo:rt\").registerFont(" + strconv.Quote(base64.StdEncoding.EncodeToString(data)) +
			");\nexport default " + strconv.Quote(family) + ";\n", nil
	}
	var w, h float64
	if ext == ".svg" {
		svg, err := driver.ParseSVG(data)
		if err != nil {
			return "", fmt.Errorf("%s: %w", p, err)
		}
		sw, sh := svg.Size()
		w, h = float64(sw), float64(sh)
	} else {
		cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			return "", fmt.Errorf("%s: cannot read the image: %w", p, err)
		}
		scale := 1.0
		if m := densityRE.FindStringSubmatch(p); m != nil {
			if s, err := strconv.ParseFloat(m[1], 64); err == nil && s > 0 {
				scale = s
			}
		}
		w, h = float64(cfg.Width)/scale, float64(cfg.Height)/scale
	}
	kind := mime.TypeByExtension(ext)
	if ext == ".svg" {
		kind = "image/svg+xml"
	}
	dataURL := func(b []byte) string { return "data:" + kind + ";base64," + base64.StdEncoding.EncodeToString(b) }
	m := map[string]any{"url": "asset:/" + p, "size": map[string]float64{"width": w, "height": h}}
	if ext != ".svg" && !densityRE.MatchString(p) {
		scales, variants := []float32{1}, map[string]string{}
		base := strings.TrimSuffix(p, path.Ext(p))
		for _, s := range driver.ImageScaleValues[1:] {
			name := base + "@" + strconv.FormatFloat(float64(s), 'g', -1, 32) + "x" + path.Ext(p)
			if b, err := fs.ReadFile(fsys, name); err == nil {
				scales = append(scales, s)
				variants[strconv.FormatFloat(float64(s), 'g', -1, 32)] = dataURL(b)
			}
		}
		if len(scales) > 1 {
			m["scales"], m["variants"] = scales, variants
		}
	}
	asset, _ := json.Marshal(m)
	// Its bytes come with the bundle: images and fetch read them by URL.
	return "require(\"nsgo:rt\").registerAsset(" + strconv.Quote("asset:/"+p) + ", " + strconv.Quote(dataURL(data)) +
		");\nexport default " + string(asset) + ";\n", nil
}

// fontFamily is the family a font registers under: its typographic
// family when it has one, else its family.
func fontFamily(data []byte) (string, error) {
	f, err := sfnt.Parse(data)
	if err != nil {
		return "", err
	}
	var buf sfnt.Buffer
	if name, err := f.Name(&buf, sfnt.NameIDTypographicFamily); err == nil && name != "" {
		return name, nil
	}
	return f.Name(&buf, sfnt.NameIDFamily)
}
