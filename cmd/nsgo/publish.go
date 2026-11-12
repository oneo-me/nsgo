package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/oneo-me/nsgo/tsx/bundler"
	"github.com/oneo-me/nsgo/tsx/pkg"
)

// Signed TSX packages (tsx/pkg): keygen prints keys, certify has a root
// certify a release key, publish compiles an app and signs it, or signs a
// package again, verify checks one and revoke has a root sign a
// revocation list. Secret keys come from the environment alone, never a
// file or a flag: NSGO_ROOT_KEY for a root's, NSGO_RELEASE_KEY for a
// release key's.

// packageExt is a package's file extension.
const packageExt = ".nsgopkg"

// keygen prints a key pair on standard output and writes nothing: the
// public key for nsgo.yaml, an app's code or a flag, the secret one for
// the user to keep and set in the environment.
func keygen() error {
	key, err := pkg.GenerateKey()
	if err != nil {
		return err
	}
	pub := key.Public().(ed25519.PublicKey)
	fmt.Printf("public: %s\nsecret: %s\n", pkg.EncodePublicKey(pub), pkg.EncodeSecretKey(key))
	done("made key %s: keep the secret one out of the repository and give it in NSGO_UPDATES_KEY, NSGO_ROOT_KEY or NSGO_RELEASE_KEY", pkg.KeyID(pub))
	return nil
}

type certifyOptions struct {
	key, apps, out string
	days           int
}

// certify has NSGO_ROOT_KEY's root certify a release key for apps, from
// now for days.
func certify(o certifyOptions) error {
	if o.key == "" || o.apps == "" {
		return errors.New("certify: -key and -apps are required")
	}
	root, err := envSecretKey("NSGO_ROOT_KEY")
	if err != nil {
		return err
	}
	release, err := pkg.DecodePublicKey(o.key)
	if err != nil {
		return fmt.Errorf("-key: %w", err)
	}
	if o.days <= 0 {
		return errors.New("certify: -days is 1 or more")
	}
	from := time.Now()
	cert, err := pkg.Certify(root, release, splitList(o.apps), from, from.AddDate(0, 0, o.days))
	if err != nil {
		return err
	}
	if o.out == "" {
		o.out = "release.cert"
	}
	if err := writeJSON(o.out, cert); err != nil {
		return err
	}
	done("certified key %s for %s until %s: %s", pkg.KeyID(release), o.apps, from.AddDate(0, 0, o.days).Format(time.DateOnly), rel(o.out))
	return nil
}

type publishOptions struct {
	cert, entry, minRuntime, out string
}

// publish compiles the app in dir and signs it with NSGO_RELEASE_KEY's
// key, or signs the package at path again.
func publish(o publishOptions, path string) error {
	if o.cert == "" {
		return errors.New("publish: -cert is required")
	}
	key, err := envSecretKey("NSGO_RELEASE_KEY")
	if err != nil {
		return err
	}
	certData, err := os.ReadFile(o.cert)
	if err != nil {
		return err
	}
	cert, err := pkg.ParseCertificate(certData)
	if err != nil {
		return fmt.Errorf("%s: %w", rel(o.cert), err)
	}
	if strings.HasSuffix(path, packageExt) {
		if o.out == "" {
			return errors.New("publish: signing a package again needs -out")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if data, err = pkg.Resign(data, key, cert); err != nil {
			return err
		}
		return writePackage(o.out, data, "signed again")
	}
	p, err := load(path)
	if err != nil {
		return err
	}
	entry, err := publishEntry(p.Dir, o.entry)
	if err != nil {
		return err
	}
	header(p.Name, p.Version, "package")
	source, err := bundler.Compile(os.DirFS(p.Dir), entry)
	if err != nil {
		return err
	}
	app := pkg.App{ID: p.ID, Name: p.Name, Version: p.Version, Build: uint64(p.versionCode())}
	contents := pkg.Contents{App: app, Source: source, MinRuntime: o.minRuntime}
	caps, err := p.capabilities()
	if err != nil {
		return err
	}
	if caps != nil {
		contents.Capabilities = *caps
	}
	data, err := pkg.Build(contents, key, cert)
	if err != nil {
		return err
	}
	if o.out == "" {
		o.out = filepath.Join(p.Build.Output, fmt.Sprintf("%s-%s%s", p.Name, p.Version, packageExt))
	}
	return writePackage(o.out, data, fmt.Sprintf("signed %s %s build %d", app.ID, app.Version, app.Build))
}

// publishEntry is the TSX entry to compile: the one given, else the one
// the main package runs, else App.tsx.
func publishEntry(dir, entry string) (string, error) {
	if entry != "" {
		return entry, nil
	}
	entries, err := tsxEntries(dir)
	if err != nil {
		return "", err
	}
	switch len(entries) {
	case 0:
		if _, err := os.Stat(filepath.Join(dir, "App.tsx")); err != nil {
			return "", errors.New("publish: no TSX entry; name one with -entry")
		}
		return "App.tsx", nil
	case 1:
		return entries[0], nil
	}
	return "", fmt.Errorf("publish: the app runs %s; name one with -entry", strings.Join(entries, ", "))
}

func writePackage(out string, data []byte, what string) error {
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(out, data, 0o644); err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	done("%s: %s (%s)", what, rel(out), formatSize(int64(len(data))))
	detail("sha256 %s", hex.EncodeToString(sum[:]))
	return nil
}

type verifyOptions struct {
	roots, revocations string
}

// verifyPackage checks the package at path against roots, or only
// against itself without them, and prints what it says.
func verifyPackage(o verifyOptions, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var info *pkg.Info
	if o.roots == "" {
		if info, err = pkg.Inspect(data); err != nil {
			return err
		}
	} else {
		trust := pkg.NewTrust()
		for _, r := range splitList(o.roots) {
			root, err := pkg.DecodePublicKey(r)
			if err != nil {
				return fmt.Errorf("-root: %w", err)
			}
			trust.AddRoot(root)
		}
		if o.revocations != "" {
			list, err := readRevocations(o.revocations)
			if err != nil {
				return err
			}
			if err := trust.UpdateRevocations(list); err != nil {
				return err
			}
		}
		p, err := trust.Verify(data)
		if err != nil {
			return err
		}
		info = &p.Info
	}
	cert, err := info.Certificate.Info()
	if err != nil {
		return err
	}
	if o.roots == "" {
		done("intact, signed by key %s; its certificate is unchecked without -root", info.KeyID)
	} else {
		done("verified, signed by key %s", info.KeyID)
	}
	detail("app %s %s build %d (%s)", info.App.ID, info.App.Version, info.App.Build, info.App.Name)
	detail("signed %s, for runtime %s or newer", info.CreatedAt.Format(time.DateTime), info.MinRuntime)
	detail("certified by root %s for %s until %s", cert.Signer, strings.Join(cert.Apps, ", "), cert.NotAfter.Format(time.DateOnly))
	detail("sha256 %s", info.SHA256)
	if lines := describeCapabilities(info.Capabilities); len(lines) == 0 {
		detail("capabilities: none")
	} else {
		for _, l := range lines {
			detail("may reach %s", l)
		}
	}
	for _, i := range info.Ignored {
		warn("this runtime leaves out %s", i)
	}
	return nil
}

type revokeOptions struct {
	keys, packages, out string
	sequence            uint64
}

// revoke has NSGO_ROOT_KEY's root sign a revocation list of release keys
// and packages, named by SHA-256 or by file.
func revoke(o revokeOptions) error {
	if o.out == "" || o.sequence == 0 {
		return errors.New("revoke: -sequence and -out are required")
	}
	root, err := envSecretKey("NSGO_ROOT_KEY")
	if err != nil {
		return err
	}
	var packages []string
	for _, p := range splitList(o.packages) {
		if strings.HasSuffix(p, packageExt) {
			data, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			sum := sha256.Sum256(data)
			p = hex.EncodeToString(sum[:])
		}
		packages = append(packages, p)
	}
	keys := splitList(o.keys)
	list, err := pkg.Revoke(root, o.sequence, keys, packages)
	if err != nil {
		return err
	}
	if err := writeJSON(o.out, list); err != nil {
		return err
	}
	done("revocation list %d of %d keys and %d packages: %s", o.sequence, len(keys), len(packages), rel(o.out))
	return nil
}

// envSecretKey is the secret key nsgo keygen printed in the environment
// variable name.
func envSecretKey(name string) (ed25519.PrivateKey, error) {
	text := os.Getenv(name)
	if text == "" {
		return nil, fmt.Errorf("%s holds no secret key; set it to the secret key nsgo keygen prints", name)
	}
	key, err := pkg.DecodeSecretKey(text)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return key, nil
}

func readRevocations(path string) (*pkg.Revocations, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	list, err := pkg.ParseRevocations(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", rel(path), err)
	}
	return list, nil
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		warn("replacing %s", rel(path))
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// splitList splits a comma-separated flag.
func splitList(s string) []string {
	var out []string
	for _, f := range strings.Split(s, ",") {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	return out
}
