package terminal

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/oneo-me/nsgo/core"
)

func TestParseGhosttyTheme(t *testing.T) {
	th, err := ParseGhosttyTheme([]byte(`# A theme
palette = 0=#123
palette = 0b1=#aaabbbccc
palette = 0x2 = 445566
palette = 200=#ffffff
background = "#fdf6e3"
foreground=#657b83
cursor-color = cell-foreground
cursor-text = #002b36
selection-background = #eee8d5
selection-foreground = #586e75
font-family = "Iosevka"
`))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][2]core.Color{
		"palette 0":     {th.Palette[0], core.RGB(0x112233)},
		"palette 1":     {th.Palette[1], core.RGB(0xaabbcc)},
		"palette 2":     {th.Palette[2], core.RGB(0x445566)},
		"background":    {th.Background, core.RGB(0xfdf6e3)},
		"foreground":    {th.Foreground, core.RGB(0x657b83)},
		"cursor":        {th.Cursor, 0},
		"cursor text":   {th.CursorText, core.RGB(0x002b36)},
		"selection":     {th.Selection, core.RGB(0xeee8d5)},
		"selected text": {th.SelectionText, core.RGB(0x586e75)},
		// What the theme leaves out comes from the light theme, as its
		// background is light.
		"palette 3": {th.Palette[3], LightTheme().Palette[3]},
	}
	for name, c := range want {
		if c[0] != c[1] {
			t.Errorf("%s = %v, want %v", name, c[0], c[1])
		}
	}
	empty, err := ParseGhosttyTheme(nil)
	if err != nil || *empty != *DarkTheme() {
		t.Errorf("an empty theme is %+v, %v; want the dark theme", empty, err)
	}
	for _, bad := range []string{"background = red", "palette = 300=#000000", "palette = #000000", "background"} {
		if _, err := ParseGhosttyTheme([]byte(bad)); err == nil || !strings.Contains(err.Error(), "line 1") {
			t.Errorf("%q: err = %v", bad, err)
		}
	}
}

func TestGhosttyTheme(t *testing.T) {
	config := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	if len(ghosttyThemes) < 500 || !slices.IsSortedFunc(ghosttyThemes[:], func(a, b ghosttyTheme) int { return strings.Compare(a.name, b.name) }) {
		t.Fatalf("%d themes, sorted: want hundreds", len(ghosttyThemes))
	}
	mocha, err := GhosttyTheme("Catppuccin Mocha")
	if err != nil {
		t.Fatal(err)
	}
	if mocha.Background != core.RGB(0x1e1e2e) || mocha.Foreground != core.RGB(0xcdd6f4) || mocha.Palette[1] != core.RGB(0xf38ba8) ||
		mocha.Cursor != core.RGB(0xf5e0dc) || mocha.SelectionText != core.RGB(0x1e1e2e) || !mocha.dark() {
		t.Errorf("Catppuccin Mocha is %+v", mocha)
	}
	if th, err := GhosttyTheme("catppuccin mocha"); err != nil || *th != *mocha {
		t.Errorf("names of another case: %+v, %v", th, err)
	}
	if th, _ := GhosttyTheme("Catppuccin Mocha"); th == mocha {
		t.Error("two calls returned the same theme")
	}
	for _, bad := range []string{"", "No Such Theme", "../Dracula", "."} {
		if _, err := GhosttyTheme(bad); err == nil {
			t.Errorf("%q found a theme", bad)
		}
	}

	// The user's themes come first, as in Ghostty.
	dir := filepath.Join(config, "ghostty", "themes")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "Mine"), []byte("background = #102030\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "Dracula"), []byte("background = #010203\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "Broken"), []byte("background = nope\n"), 0o644)
	for name, bg := range map[string]string{"Mine": "#102030", "Dracula": "#010203", filepath.Join(dir, "Mine"): "#102030"} {
		if th, err := GhosttyTheme(name); err != nil || th.Background != hexColor(bg) {
			t.Errorf("%s: %+v, %v", name, th, err)
		}
	}
	if _, err := GhosttyTheme("Broken"); err == nil || !strings.Contains(err.Error(), "Broken") {
		t.Errorf("a broken theme: %v", err)
	}
	names := GhosttyThemes()
	if !slices.Contains(names, "Mine") || !slices.Contains(names, "Catppuccin Latte") || len(slices.Compact(slices.Clone(names))) != len(names) {
		t.Errorf("GhosttyThemes misses the user's or Ghostty's themes, or repeats some: %d names", len(names))
	}
	if n := len(names); n != len(ghosttyThemes)+2 {
		t.Errorf("%d names, want %d", n, len(ghosttyThemes)+2)
	}
}

func hexColor(s string) core.Color {
	c, _ := parseHexColor(s)
	return c
}
