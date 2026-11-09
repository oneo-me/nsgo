package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// fileType is a kind of document the app opens, which nsgo.yaml's
// file_types lists and the macOS package registers with Launch Services:
// files of it then open with the app from the Finder, and reach it through
// desktop.OnOpenFiles.
type fileType struct {
	// Name is what the Finder calls such a file.
	Name       string   `yaml:"name"`
	Extensions []string `yaml:"extensions"`
	// ID is its uniform type identifier: <app id>.<first extension> by
	// default. The app declares, and owns, a type under its own ID; any
	// other it imports, ranking as an alternative to its owner's app.
	ID string `yaml:"id"`
	// Icon is an .icns or a square PNG for its files, relative to
	// nsgo.yaml; without it, the Finder draws the app's icon on a page.
	Icon string `yaml:"icon"`
	// Role is editor, the default, or viewer.
	Role string `yaml:"role"`
	// Conforms are the types it is a kind of: public.data by default,
	// public.plain-text or public.json for text a text editor may open.
	Conforms []string `yaml:"conforms"`
}

var (
	extensionPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	utiPattern       = regexp.MustCompile(`^[A-Za-z0-9-]+(\.[A-Za-z0-9-]+)+$`)
)

// validateFileTypes checks file_types and fills in their defaults; it
// runs once the app's ID is known.
func (p *project) validateFileTypes() error {
	seen := map[string]bool{}
	for i := range p.FileTypes {
		f := &p.FileTypes[i]
		key := fmt.Sprintf("file_types[%d]", i)
		if f.Name == "" {
			return fmt.Errorf("%s.name is required", key)
		}
		if len(f.Extensions) == 0 {
			return fmt.Errorf("%s.extensions is required", key)
		}
		for _, ext := range f.Extensions {
			if !extensionPattern.MatchString(ext) {
				return fmt.Errorf("%s.extensions: %q is not an extension; write flow, not .flow", key, ext)
			}
		}
		if f.ID == "" {
			f.ID = p.ID + "." + strings.ToLower(f.Extensions[0])
		}
		if !utiPattern.MatchString(f.ID) {
			return fmt.Errorf("%s.id %q is not a type identifier, as com.example.app.doc", key, f.ID)
		}
		if seen[f.ID] {
			return fmt.Errorf("%s: the type %q is listed twice", key, f.ID)
		}
		seen[f.ID] = true
		switch f.Role {
		case "", "editor", "viewer":
		default:
			return fmt.Errorf("%s.role is %q; use editor, viewer or leave it out", key, f.Role)
		}
		for _, c := range f.Conforms {
			if !utiPattern.MatchString(c) {
				return fmt.Errorf("%s.conforms: %q is not a type identifier, as public.data", key, c)
			}
		}
		if f.Icon != "" {
			f.Icon = p.resolve(f.Icon)
			if ext := filepath.Ext(f.Icon); ext != ".icns" && ext != ".png" {
				return fmt.Errorf("%s.icon: %s is neither an .icns nor a .png", key, rel(f.Icon))
			}
			if _, err := os.Stat(f.Icon); err != nil {
				return fmt.Errorf("%s.icon: %w", key, err)
			}
		}
	}
	return nil
}

// owned reports whether the app declares f as its own type.
func (f fileType) owned(appID string) bool {
	return strings.HasPrefix(f.ID, appID+".")
}

// iconName is the name of f's icon in the package's resources.
func (f fileType) iconName() string {
	if f.Icon == "" {
		return ""
	}
	return f.ID + ".icns"
}

// writeFileTypeIcons writes the file types' icons into a macOS package's
// resources.
func (p *project) writeFileTypeIcons(resources string) error {
	for _, f := range p.FileTypes {
		if f.Icon == "" {
			continue
		}
		var data []byte
		if filepath.Ext(f.Icon) == ".icns" {
			var err error
			if data, err = os.ReadFile(f.Icon); err != nil {
				return err
			}
		} else {
			img, err := readIcon(f.Icon)
			if err != nil {
				return err
			}
			data = icns(img)
		}
		if err := os.WriteFile(filepath.Join(resources, f.iconName()), data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// fileTypesPlist is the Info.plist entries that register the file types:
// the documents the app opens, and the types it declares or imports.
func (p *project) fileTypesPlist() string {
	if len(p.FileTypes) == 0 {
		return ""
	}
	var docs, exported, imported strings.Builder
	for _, f := range p.FileTypes {
		role, rank := "Editor", "Owner"
		if f.Role == "viewer" {
			role = "Viewer"
		}
		decls := &exported
		if !f.owned(p.ID) {
			rank, decls = "Alternate", &imported
		}
		docs.WriteString("\t\t<dict>\n")
		fmt.Fprintf(&docs, "\t\t\t<key>CFBundleTypeName</key><string>%s</string>\n", xmlText(f.Name))
		fmt.Fprintf(&docs, "\t\t\t<key>CFBundleTypeRole</key><string>%s</string>\n", role)
		fmt.Fprintf(&docs, "\t\t\t<key>LSHandlerRank</key><string>%s</string>\n", rank)
		fmt.Fprintf(&docs, "\t\t\t<key>LSItemContentTypes</key><array><string>%s</string></array>\n", xmlText(f.ID))
		if name := f.iconName(); name != "" {
			fmt.Fprintf(&docs, "\t\t\t<key>CFBundleTypeIconFile</key><string>%s</string>\n", xmlText(name))
		}
		docs.WriteString("\t\t</dict>\n")

		conforms := f.Conforms
		if len(conforms) == 0 {
			conforms = []string{"public.data"}
		}
		decls.WriteString("\t\t<dict>\n")
		fmt.Fprintf(decls, "\t\t\t<key>UTTypeIdentifier</key><string>%s</string>\n", xmlText(f.ID))
		fmt.Fprintf(decls, "\t\t\t<key>UTTypeDescription</key><string>%s</string>\n", xmlText(f.Name))
		fmt.Fprintf(decls, "\t\t\t<key>UTTypeConformsTo</key>%s\n", plistStrings(conforms))
		if name := f.iconName(); name != "" {
			fmt.Fprintf(decls, "\t\t\t<key>UTTypeIconFile</key><string>%s</string>\n", xmlText(name))
		}
		fmt.Fprintf(decls, "\t\t\t<key>UTTypeTagSpecification</key><dict><key>public.filename-extension</key>%s</dict>\n",
			plistStrings(f.Extensions))
		decls.WriteString("\t\t</dict>\n")
	}
	out := "\t<key>CFBundleDocumentTypes</key>\n\t<array>\n" + docs.String() + "\t</array>"
	if exported.Len() > 0 {
		out += "\n\t<key>UTExportedTypeDeclarations</key>\n\t<array>\n" + exported.String() + "\t</array>"
	}
	if imported.Len() > 0 {
		out += "\n\t<key>UTImportedTypeDeclarations</key>\n\t<array>\n" + imported.String() + "\t</array>"
	}
	return out
}

func plistStrings(values []string) string {
	var b strings.Builder
	b.WriteString("<array>")
	for _, v := range values {
		fmt.Fprintf(&b, "<string>%s</string>", xmlText(v))
	}
	b.WriteString("</array>")
	return b.String()
}
