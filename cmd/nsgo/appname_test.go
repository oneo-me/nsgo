package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAppNames names an app in its catalogs' languages in its Apple
// bundles, its web manifest and its desktop entry.
func TestAppNames(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"locales/en.yaml":      "appName: Gallery\nhello: Hi\n",
		"locales/zh-Hans.yaml": "appName: 画廊\n",
		"locales/pt-BR.yaml":   "appName: \"Galeria \\\"Nova\\\"\"\n",
	})
	p := &project{Dir: dir, Name: "gallery", ID: "dev.oneo.gallery"}
	p.I18n.Dir, p.I18n.Source = filepath.Join(dir, "locales"), "en"

	res := filepath.Join(dir, "Resources")
	extra, err := writeAppleNames(p, res)
	if err != nil {
		t.Fatal(err)
	}
	if want := "<key>CFBundleDisplayName</key><string>Gallery</string>"; !strings.Contains(extra, want) {
		t.Errorf("plist entries %q lack %q", extra, want)
	}
	if want := "<key>CFBundleLocalizations</key><array><string>en</string><string>pt-BR</string><string>zh-Hans</string></array>"; !strings.Contains(extra, want) {
		t.Errorf("plist entries %q lack %q", extra, want)
	}
	for lang, want := range map[string]string{"zh-Hans": `"CFBundleDisplayName" = "画廊";`, "pt-BR": `"CFBundleName" = "Galeria \"Nova\"";`} {
		data, _ := os.ReadFile(filepath.Join(res, lang+".lproj", "InfoPlist.strings"))
		if !strings.Contains(string(data), want) {
			t.Errorf("%s.lproj: %q lacks %q", lang, data, want)
		}
	}

	web := filepath.Join(dir, "web")
	os.MkdirAll(web, 0o755)
	if err := writeWebManifest(p, web, nil); err != nil {
		t.Fatal(err)
	}
	var m webManifest
	data, _ := os.ReadFile(filepath.Join(web, "manifest.webmanifest"))
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	if m.Name != "Gallery" || m.NameLocalized["zh-Hans"] != "画廊" || m.ShortNameLocalized["pt-BR"] != `Galeria "Nova"` {
		t.Errorf("manifest %s", data)
	}

	if got := desktopLocale("zh-Hant"); got != "zh_TW" {
		t.Errorf("desktop locale of zh-Hant: %q", got)
	}
	if got := desktopLocale("pt-BR"); got != "pt_BR" {
		t.Errorf("desktop locale of pt-BR: %q", got)
	}

	// An app named with an argument fails.
	writeFiles(t, dir, map[string]string{"locales/en.yaml": "appName: \"{n} apps\"\n"})
	if _, err := p.appNames(); err == nil || !strings.Contains(err.Error(), "takes no arguments") {
		t.Errorf("appName with an argument: %v", err)
	}
}
