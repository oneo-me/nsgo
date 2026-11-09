package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileTypesRegisterInThePlist(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"nsgo.yaml": `name: Flow
id: me.oneo.flow
file_types:
  - name: Flow File
    extensions: [flow]
  - name: Markdown
    id: net.daringfireball.markdown
    extensions: [md, markdown]
    role: viewer
    conforms: [public.plain-text]
`})
	p, err := load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := p.FileTypes[0].ID; got != "me.oneo.flow.flow" {
		t.Errorf("the default type is %q", got)
	}
	path := filepath.Join(t.TempDir(), "Info.plist")
	if err := writePlist(path, p, macPlist, p.fileTypesPlist()); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	plist := string(data)
	exported, imported, _ := strings.Cut(plist[strings.Index(plist, "UTExportedTypeDeclarations"):], "UTImportedTypeDeclarations")
	for _, want := range []string{
		"<key>CFBundleTypeRole</key><string>Editor</string>",
		"<key>LSHandlerRank</key><string>Owner</string>",
		"<key>CFBundleTypeRole</key><string>Viewer</string>",
		"<key>LSHandlerRank</key><string>Alternate</string>",
	} {
		if !strings.Contains(plist, want) {
			t.Errorf("Info.plist lacks %q:\n%s", want, plist)
		}
	}
	if !strings.Contains(exported, "<string>me.oneo.flow.flow</string>") || !strings.Contains(exported, "<string>public.data</string>") {
		t.Errorf("the app's own type is not exported:\n%s", exported)
	}
	if !strings.Contains(imported, "<array><string>md</string><string>markdown</string></array>") ||
		strings.Contains(imported, "me.oneo.flow.flow") {
		t.Errorf("another's type is not imported:\n%s", imported)
	}
	if lint, err := exec.LookPath("plutil"); err == nil {
		if out, err := exec.Command(lint, "-lint", path).CombinedOutput(); err != nil {
			t.Errorf("plutil: %s", out)
		}
	}
}

func TestFileTypesAreChecked(t *testing.T) {
	for yaml, want := range map[string]string{
		"- name: Flow\n  extensions: [.flow]":                              "not .flow",
		"- name: Flow":                                                     "extensions is required",
		"- extensions: [flow]":                                             "name is required",
		"- name: Flow\n  extensions: [flow]\n  role: owner":                "role is \"owner\"",
		"- name: Flow\n  extensions: [flow]\n  icon: flow.svg":             "neither an .icns nor a .png",
		"- name: A\n  extensions: [flow]\n- name: B\n  extensions: [flow]": "listed twice",
	} {
		dir := t.TempDir()
		writeFiles(t, dir, map[string]string{"nsgo.yaml": "name: Flow\nfile_types:\n" + yamlItems(yaml)})
		if _, err := load(dir); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v, want %q", yaml, err, want)
		}
	}
}

func yamlItems(s string) string {
	return "  " + strings.ReplaceAll(s, "\n", "\n  ") + "\n"
}
