//go:build !nsgo_bundled

package compiler

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"mime"
	"path"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/image/font/sfnt"
	_ "golang.org/x/image/webp"

	"github.com/oneo-me/nsgo/core"
)

// Assets a module imports (NSH's plugin.rs): an image is an ImageAsset,
// its size in points read as the bundle is built and its bytes inlined
// as a data: URL, so it ships in the bundle on every target; a font is
// its family name, its bytes inlined and registered as its module runs.

var imageExts = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".svg": true}

var fontExts = map[string]bool{".ttf": true, ".otf": true}

// isAsset reports whether a module is an asset.
func isAsset(p string) bool {
	ext := strings.ToLower(path.Ext(p))
	return imageExts[ext] || fontExts[ext]
}

// density is the pixel density a file's name gives: logo@2x.png is 2.
var densityRE = regexp.MustCompile(`@([0-9.]+)x\.[A-Za-z]+$`)

// assetModule is the module of asset p.
func assetModule(p string, data []byte) (string, error) {
	ext := strings.ToLower(path.Ext(p))
	if fontExts[ext] {
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
		svg, err := core.ParseSVG(data)
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
	asset, _ := json.Marshal(map[string]any{
		"url":  "data:" + kind + ";base64," + base64.StdEncoding.EncodeToString(data),
		"size": map[string]float64{"width": w, "height": h},
	})
	return "export default " + string(asset) + ";\n", nil
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
