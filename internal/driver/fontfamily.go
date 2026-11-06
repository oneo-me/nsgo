package driver

import "strings"

// FamilyKind is what a family of a fontFamily list names.
type FamilyKind uint8

const (
	// FamilyNamed is a family installed or registered by the app.
	FamilyNamed FamilyKind = iota
	// FamilySystem is the system font: system-ui, sans-serif and their
	// kind.
	FamilySystem
	FamilySerif
	FamilyMonospace
	// FamilyRounded is the system's rounded font, where it has one.
	FamilyRounded
)

// Family is a family of a fontFamily list.
type Family struct {
	Kind FamilyKind
	Name string
}

// FontFamilies reads a fontFamily value, most preferred first, as CSS
// lists families: names separated by commas, each trimmed and unquoted
// (`Inter, "Noto Sans SC", sans-serif`). A quoted name runs to its closing
// quote, commas and all, and is always a family's name. Text is set in
// the first family a platform has, and a character that family lacks in
// the next that has it.
func FontFamilies(value string) []Family {
	var out []Family
	for rest := value; ; {
		rest = strings.TrimSpace(rest)
		if rest == "" {
			return out
		}
		var name string
		quoted := rest[0] == '"' || rest[0] == '\''
		if quoted {
			body := rest[1:]
			end := strings.IndexByte(body, rest[0])
			if end < 0 {
				end = len(body)
			}
			name = strings.TrimSpace(body[:end])
			rest = ""
			if end < len(body) {
				if _, after, ok := strings.Cut(body[end+1:], ","); ok {
					rest = after
				}
			}
		} else {
			name, rest, _ = strings.Cut(rest, ",")
			name = strings.TrimSpace(name)
		}
		if name == "" {
			continue
		}
		f := Family{Name: name}
		if !quoted {
			switch name {
			case "system-ui", "sans-serif", "ui-sans-serif", "-apple-system", "BlinkMacSystemFont":
				f = Family{Kind: FamilySystem}
			case "serif", "ui-serif":
				f = Family{Kind: FamilySerif}
			case "monospace", "ui-monospace":
				f = Family{Kind: FamilyMonospace}
			case "ui-rounded":
				f = Family{Kind: FamilyRounded}
			}
		}
		out = append(out, f)
	}
}
