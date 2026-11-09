package main

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/oneo-me/nsgo/cmd/nsgo/apk"
)

// signing is nsgo.yaml's sign section: what release packages are signed
// with. Passwords never go in it: NSGO_KEYSTORE_PASSWORD and
// NSGO_KEY_PASSWORD give them.
type signing struct {
	// Identity is the codesign identity of the macOS and iOS packages,
	// its name or SHA-1. By default macOS packages are signed ad hoc and
	// iOS ones with the keychain's identity for one of Profile's
	// certificates.
	Identity string `yaml:"identity"`
	// Profile is the provisioning profile the iOS package carries and
	// takes its entitlements from; wildcard profiles sign the app's ID.
	Profile string `yaml:"profile"`
	// WidgetsProfile is the provisioning profile of the iOS package's
	// widget extension, <id>.widgets, granting the app group.
	WidgetsProfile string `yaml:"widgets_profile"`
	// Keystore signs Android packages: a PKCS#12 or JKS file, KeyAlias
	// naming its key when it holds several. Without it they carry the
	// project's debug key.
	Keystore string `yaml:"keystore"`
	KeyAlias string `yaml:"key_alias"`
}

// signFlags are nsgo build's flags that replace nsgo.yaml's sign values.
type signFlags struct {
	identity, profile, keystore, alias *override[string]
}

func (f *signFlags) bind(fs *flag.FlagSet) {
	f.identity = stringFlag(fs, "sign", "the macOS and iOS codesign `identity` (default: nsgo.yaml's sign.identity, else ad hoc on macOS and the profile's in the keychain on iOS)")
	f.profile = stringFlag(fs, "profile", "the iOS provisioning `profile` (default: nsgo.yaml's sign.profile)")
	f.keystore = stringFlag(fs, "keystore", "the Android `keystore`, PKCS#12 or JKS; NSGO_KEYSTORE_PASSWORD and NSGO_KEY_PASSWORD give its passwords (default: nsgo.yaml's sign.keystore)")
	f.alias = stringFlag(fs, "key-alias", "the `alias` of the keystore's key (default: nsgo.yaml's sign.key_alias, else its only key)")
}

func (f *signFlags) apply(p *project) {
	f.identity.replace(&p.Sign.Identity)
	if f.profile.set {
		p.Sign.Profile = p.resolve(f.profile.value)
	}
	if f.keystore.set {
		p.Sign.Keystore = p.resolve(f.keystore.value)
	}
	f.alias.replace(&p.Sign.KeyAlias)
}

// releaseKey is the key release APKs are signed with: the keystore's, or
// the project's debug key without one.
func releaseKey(p *project) (*apk.Key, error) {
	if p.Sign.Keystore == "" || !p.Release {
		if p.Release {
			detail("signed with the debug key: set sign.keystore for store releases")
		}
		return debugKey(p)
	}
	data, err := os.ReadFile(p.Sign.Keystore)
	if err != nil {
		return nil, err
	}
	key, err := apk.ReadKeystore(data, os.Getenv("NSGO_KEYSTORE_PASSWORD"), p.Sign.KeyAlias, os.Getenv("NSGO_KEY_PASSWORD"))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", rel(p.Sign.Keystore), err)
	}
	return key, nil
}

// signMacApp signs a macOS package with sign.identity, its libraries
// first so the app's signature seals them. macOS keeps the permissions it
// grants a program signed by an identity across builds, and loses them
// with each new ad hoc signature, which stays without an identity.
func signMacApp(ctx context.Context, p *project, app string) error {
	if p.Sign.Identity == "" {
		return nil
	}
	detail("sign with %s", p.Sign.Identity)
	stage("signing")
	libs, _ := filepath.Glob(filepath.Join(app, "Contents", "Frameworks", "*.dylib"))
	for _, lib := range libs {
		if err := run(ctx, "codesign", "--force", "--sign", p.Sign.Identity, lib); err != nil {
			return err
		}
	}
	if appex := macWidgetsAppex(app); len(p.Widgets) > 0 {
		if err := signMacWidgets(ctx, p, appex, p.Sign.Identity); err != nil {
			return err
		}
	}
	return run(ctx, "codesign", "--force", "--sign", p.Sign.Identity, app)
}

// signForStore signs an iOS device build with the project's provisioning
// profile, which it embeds and takes the entitlements from. Without a
// profile the package stays unsigned, for the developer to sign.
func signForStore(ctx context.Context, p *project, app string) error {
	if p.Sign.Profile == "" {
		detail("unsigned: set sign.profile to install it on devices or upload it")
		return nil
	}
	data, err := os.ReadFile(p.Sign.Profile)
	if err != nil {
		return err
	}
	profile, err := readProfile(ctx, data)
	if err != nil {
		return fmt.Errorf("%s: %w", rel(p.Sign.Profile), err)
	}
	entitlements, ok := profile["Entitlements"].(map[string]any)
	if !ok {
		return fmt.Errorf("%s has no entitlements", rel(p.Sign.Profile))
	}
	appID, _ := entitlements["application-identifier"].(string)
	if !profileAllows(appID, p.ID) {
		return fmt.Errorf("the provisioning profile is for %s, not %s", appID, p.ID)
	}
	// Devices refuse an app signed with the wildcard itself, as Xcode
	// knows: a wildcard profile signs the app's own ID.
	entitlements["application-identifier"] = signedAppID(appID, p.ID)
	if groups, _ := entitlements["com.apple.security.application-groups"].([]any); len(p.Widgets) > 0 && !slices.Contains(groups, any(p.appGroup())) {
		return fmt.Errorf("the provisioning profile does not grant the app group %s, which the app shares with its widgets", p.appGroup())
	}
	identity := p.Sign.Identity
	if identity == "" {
		if identity, err = profileIdentity(ctx, profile); err != nil {
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(app, "embedded.mobileprovision"), data, 0o644); err != nil {
		return err
	}
	ents := filepath.Join(filepath.Dir(app), p.Name+".entitlements")
	if err := os.WriteFile(ents, encodePlist(entitlements), 0o644); err != nil {
		return err
	}
	defer os.Remove(ents)
	detail("sign with %s", identity)
	stage("signing")
	if err := signFrameworks(ctx, app, identity); err != nil {
		return err
	}
	if err := signIOSWidgets(ctx, p, app, identity); err != nil {
		return err
	}
	return run(ctx, "codesign", "--force", "--sign", identity, "--entitlements", ents, "--generate-entitlement-der", app)
}

// readProfile decodes a provisioning profile: CMS-signed XML, which
// security verifies and opens. A plain plist reads as it is.
func readProfile(ctx context.Context, data []byte) (map[string]any, error) {
	if !bytes.HasPrefix(bytes.TrimSpace(data), []byte("<?xml")) {
		cmd := exec.CommandContext(ctx, "security", "cms", "-D")
		cmd.Stdin = bytes.NewReader(data)
		out, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("security cms cannot read the provisioning profile: %w", err)
		}
		data = out
	}
	v, err := decodePlist(data)
	if err != nil {
		return nil, err
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, errors.New("the provisioning profile is not a dictionary")
	}
	return m, nil
}

// profileIdentity finds the keychain's codesign identity for one of the
// profile's certificates, by its SHA-1.
func profileIdentity(ctx context.Context, profile map[string]any) (string, error) {
	certs, _ := profile["DeveloperCertificates"].([]any)
	var want []string
	for _, c := range certs {
		if der, ok := c.([]byte); ok {
			sum := sha1.Sum(der)
			want = append(want, strings.ToUpper(hex.EncodeToString(sum[:])))
		}
	}
	out, err := output(ctx, "security", "find-identity", "-v", "-p", "codesigning")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(out, "\n") {
		for _, sha := range want {
			if strings.Contains(line, sha) {
				return sha, nil
			}
		}
	}
	return "", errors.New("the keychain has no identity for the provisioning profile's certificates: install the certificate, or pass -sign")
}

// profileAllows reports whether a profile's application-identifier
// (TEAMID.com.example.app, or a wildcard such as TEAMID.* or
// TEAMID.com.example.*) covers the bundle ID id.
func profileAllows(appID, id string) bool {
	_, pattern, ok := strings.Cut(appID, ".")
	if !ok {
		return false
	}
	if prefix, wild := strings.CutSuffix(pattern, "*"); wild {
		return strings.HasPrefix(id, prefix)
	}
	return pattern == id
}

// signedAppID is the application-identifier an app signed with a profile
// for appID carries: its team and the app's own ID.
func signedAppID(appID, id string) string {
	if team, pattern, ok := strings.Cut(appID, "."); ok && strings.HasSuffix(pattern, "*") {
		return team + "." + id
	}
	return appID
}

// decodePlist reads an XML property list into Go values: maps, slices,
// strings, bools, int64s, float64s and []byte for data.
func decodePlist(data []byte) (any, error) {
	d := xml.NewDecoder(bytes.NewReader(data))
	d.Strict = false
	var value func(start xml.StartElement) (any, error)
	text := func() (string, error) {
		var b strings.Builder
		for {
			t, err := d.Token()
			if err != nil {
				return "", err
			}
			switch t := t.(type) {
			case xml.CharData:
				b.Write(t)
			case xml.EndElement:
				return b.String(), nil
			}
		}
	}
	next := func() (xml.StartElement, bool, error) {
		for {
			t, err := d.Token()
			if err != nil {
				return xml.StartElement{}, false, err
			}
			switch t := t.(type) {
			case xml.StartElement:
				return t, true, nil
			case xml.EndElement:
				return xml.StartElement{}, false, nil
			}
		}
	}
	value = func(start xml.StartElement) (any, error) {
		switch start.Name.Local {
		case "dict":
			m := map[string]any{}
			for {
				_, ok, err := next()
				if err != nil || !ok {
					return m, err
				}
				key, err := text()
				if err != nil {
					return nil, err
				}
				v, _, err := next()
				if err != nil {
					return nil, err
				}
				if m[key], err = value(v); err != nil {
					return nil, err
				}
			}
		case "array":
			var a []any
			for {
				e, ok, err := next()
				if err != nil || !ok {
					return a, err
				}
				v, err := value(e)
				if err != nil {
					return nil, err
				}
				a = append(a, v)
			}
		case "true", "false":
			_, err := text()
			return start.Name.Local == "true", err
		case "integer":
			s, err := text()
			if err != nil {
				return nil, err
			}
			return strconv.ParseInt(strings.TrimSpace(s), 10, 64)
		case "real":
			s, err := text()
			if err != nil {
				return nil, err
			}
			return strconv.ParseFloat(strings.TrimSpace(s), 64)
		case "data":
			s, err := text()
			if err != nil {
				return nil, err
			}
			return base64.StdEncoding.DecodeString(strings.Join(strings.Fields(s), ""))
		default: // string, date
			return text()
		}
	}
	for {
		start, ok, err := next()
		if err != nil {
			if err == io.EOF {
				return nil, errors.New("an empty property list")
			}
			return nil, err
		}
		if ok && start.Name.Local != "plist" {
			return value(start)
		}
	}
}

// encodePlist writes v, as decodePlist reads it, as an XML property list.
func encodePlist(v any) []byte {
	var b bytes.Buffer
	b.WriteString(xml.Header + `<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">` + "\n<plist version=\"1.0\">\n")
	esc := func(s string) string {
		var e bytes.Buffer
		xml.EscapeText(&e, []byte(s))
		return e.String()
	}
	var write func(v any, indent string)
	write = func(v any, indent string) {
		switch v := v.(type) {
		case map[string]any:
			keys := make([]string, 0, len(v))
			for k := range v {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			b.WriteString(indent + "<dict>\n")
			for _, k := range keys {
				b.WriteString(indent + "\t<key>" + esc(k) + "</key>\n")
				write(v[k], indent+"\t")
			}
			b.WriteString(indent + "</dict>\n")
		case []any:
			b.WriteString(indent + "<array>\n")
			for _, e := range v {
				write(e, indent+"\t")
			}
			b.WriteString(indent + "</array>\n")
		case bool:
			if v {
				b.WriteString(indent + "<true/>\n")
			} else {
				b.WriteString(indent + "<false/>\n")
			}
		case int64:
			fmt.Fprintf(&b, "%s<integer>%d</integer>\n", indent, v)
		case float64:
			fmt.Fprintf(&b, "%s<real>%v</real>\n", indent, v)
		case []byte:
			fmt.Fprintf(&b, "%s<data>%s</data>\n", indent, base64.StdEncoding.EncodeToString(v))
		case string:
			b.WriteString(indent + "<string>" + esc(v) + "</string>\n")
		}
	}
	write(v, "")
	b.WriteString("</plist>\n")
	return b.Bytes()
}
