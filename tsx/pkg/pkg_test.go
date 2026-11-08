package pkg

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

// keys are a root, a release key it certifies for com.example.*, and the
// certificate.
type keys struct {
	root, release ed25519.PrivateKey
	cert          *Certificate
}

func newKeys(t *testing.T, apps ...string) keys {
	t.Helper()
	if len(apps) == 0 {
		apps = []string{"com.example.*"}
	}
	root, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	release, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	cert, err := Certify(root, release.Public().(ed25519.PublicKey), apps, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	return keys{root, release, cert}
}

func (k keys) trust() *Trust {
	t := NewTrust()
	t.AddRoot(k.root.Public().(ed25519.PublicKey))
	return t
}

func (k keys) build(t *testing.T, build uint64) []byte {
	t.Helper()
	data, err := Build(Contents{App: App{ID: "com.example.app", Name: "App", Version: "1.0.0", Build: build}, Source: "mount(1)"}, k.release, k.cert)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// craft signs a header and files of a test's own with the release key.
func (k keys) craft(t *testing.T, h map[string]any, body []byte) []byte {
	t.Helper()
	if _, ok := h["certificate"]; !ok {
		h["certificate"] = k.cert
	}
	if _, ok := h["createdAt"]; !ok {
		h["createdAt"] = time.Now().Unix()
	}
	if _, ok := h["app"]; !ok {
		h["app"] = App{ID: "com.example.app", Build: 1}
	}
	if _, ok := h["minRuntime"]; !ok {
		h["minRuntime"] = RuntimeVersion
	}
	if _, ok := h["files"]; !ok {
		h["files"] = []map[string]any{fileOf("bundle.js", "source", 0, body)}
	}
	j, err := json.Marshal(h)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	out.WriteString(magic)
	out.Write(binary.LittleEndian.AppendUint16(nil, format))
	out.Write(binary.LittleEndian.AppendUint16(nil, 0))
	out.Write(binary.LittleEndian.AppendUint32(nil, uint32(len(j))))
	out.Write(j)
	out.Write(body)
	out.Write(sign(k.release, contextPackage, out.Bytes()))
	return out.Bytes()
}

func fileOf(path, kind string, offset int, data []byte) map[string]any {
	sum := sha256.Sum256(data)
	return map[string]any{"path": path, "kind": kind, "offset": offset, "size": len(data), "sha256": hex.EncodeToString(sum[:])}
}

func TestVerify(t *testing.T) {
	k := newKeys(t)
	data := k.build(t, 3)
	p, err := k.trust().Verify(data)
	if err != nil {
		t.Fatal(err)
	}
	if p.Source() != "mount(1)" || p.App.ID != "com.example.app" || p.App.Build != 3 || p.MinRuntime != RuntimeVersion {
		t.Fatalf("got %+v", p)
	}
	if p.KeyID != KeyID(k.release.Public().(ed25519.PublicKey)) || len(p.Ignored) != 0 {
		t.Fatalf("got key %s, ignored %v", p.KeyID, p.Ignored)
	}
	sum := sha256.Sum256(data)
	if p.SHA256 != hex.EncodeToString(sum[:]) {
		t.Fatalf("got hash %s", p.SHA256)
	}
}

func TestRefuses(t *testing.T) {
	k := newKeys(t)
	other := newKeys(t)
	data := k.build(t, 1)
	tamper := func(at int) []byte {
		b := bytes.Clone(data)
		b[at] ^= 1
		return b
	}
	cases := []struct {
		name string
		data []byte
		want error
	}{
		{"a changed source", tamper(len(data) - 70), ErrSignature},
		{"a changed signature", tamper(len(data) - 1), ErrSignature},
		{"a changed header", tamper(bytes.Index(data, []byte(`"App"`)) + 1), ErrSignature},
		{"truncated", data[:40], ErrFormat},
		{"another root's", other.build(t, 1), ErrUntrusted},
		{"signed outside the certificate", k.craft(t, map[string]any{"createdAt": time.Now().Add(-2 * time.Hour).Unix()}, []byte("x")), ErrCertificate},
		{"of an app it does not cover", k.craft(t, map[string]any{"app": App{ID: "org.other.app"}}, []byte("x")), ErrCertificate},
		{"for a newer runtime", k.craft(t, map[string]any{"minRuntime": "0.2.0"}, []byte("x")), ErrRuntime},
		{"needing a field unknown", k.craft(t, map[string]any{"critical": []string{"quota"}, "quota": 3}, []byte("x")), ErrRuntime},
		{"a file not matching its hash", k.craft(t, map[string]any{"files": []map[string]any{fileOf("bundle.js", "source", 0, []byte("y"))}}, []byte("x")), ErrIntegrity},
		{"a file table with a gap", k.craft(t, map[string]any{"files": []map[string]any{fileOf("bundle.js", "source", 1, []byte("x"))}}, []byte("xx")), ErrFormat},
		{"data beyond the file table", k.craft(t, map[string]any{"files": []map[string]any{fileOf("bundle.js", "source", 0, []byte("x"))}}, []byte("xx")), ErrFormat},
		{"no source", k.craft(t, map[string]any{"files": []map[string]any{fileOf("a.png", "asset", 0, []byte("x"))}}, []byte("x")), ErrFormat},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := k.trust().Verify(c.data)
			if !errors.Is(err, c.want) {
				t.Fatalf("got %v, want %v", err, c.want)
			}
		})
	}
}

func TestIgnoresWhatItDoesNotKnow(t *testing.T) {
	k := newKeys(t)
	body := []byte("mount(1)extra")
	data := k.craft(t, map[string]any{
		"future": true,
		"app":    map[string]any{"id": "com.example.app", "build": 1, "icon": "x"},
		"files": []map[string]any{
			fileOf("bundle.js", "source", 0, body[:8]),
			fileOf("extra.bin", "bytecode", 8, body[8:]),
		},
	}, body)
	p, err := k.trust().Verify(data)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"header field future", "app field icon", "file extra.bin"}
	if p.Source() != "mount(1)" || strings.Join(p.Ignored, "|") != strings.Join(want, "|") {
		t.Fatalf("got %q, ignored %q", p.Source(), p.Ignored)
	}
	// A source with a field unknown may mean other bytes: it is not run.
	sourceWith := fileOf("bundle.js", "source", 0, body[:8])
	sourceWith["compression"] = "zstd"
	data = k.craft(t, map[string]any{"files": []map[string]any{sourceWith}}, body[:8])
	if _, err := k.trust().Verify(data); !errors.Is(err, ErrFormat) {
		t.Fatalf("got %v", err)
	}
}

func TestBuildRefuses(t *testing.T) {
	k := newKeys(t)
	other := newKeys(t)
	cases := []struct {
		name string
		c    Contents
		key  ed25519.PrivateKey
	}{
		{"an app it does not cover", Contents{App: App{ID: "org.other"}}, k.release},
		{"another key's certificate", Contents{App: App{ID: "com.example.app"}}, other.release},
		{"a newer runtime", Contents{App: App{ID: "com.example.app"}, MinRuntime: "9.0.0"}, k.release},
		{"no version", Contents{App: App{ID: "com.example.app"}, MinRuntime: "1"}, k.release},
		{"no ID", Contents{}, k.release},
	}
	for _, c := range cases {
		if _, err := Build(c.c, c.key, k.cert); err == nil {
			t.Errorf("%s: built", c.name)
		}
	}
	old, err := Certify(k.root, k.release.Public().(ed25519.PublicKey), []string{"*"}, time.Now().Add(-2*time.Hour), time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Build(Contents{App: App{ID: "com.example.app"}}, k.release, old); !errors.Is(err, ErrCertificate) {
		t.Errorf("an expired certificate: got %v", err)
	}
}

func TestRevocations(t *testing.T) {
	k := newKeys(t)
	trust := k.trust()
	data := k.build(t, 1)
	p, err := trust.Verify(data)
	if err != nil {
		t.Fatal(err)
	}
	list, err := Revoke(k.root, 1, nil, []string{strings.ToUpper(p.SHA256)})
	if err != nil {
		t.Fatal(err)
	}
	if err := trust.UpdateRevocations(list); err != nil {
		t.Fatal(err)
	}
	if _, err := trust.Verify(data); !errors.Is(err, ErrRevoked) {
		t.Fatalf("a revoked package: got %v", err)
	}
	if err := trust.Accept(p); !errors.Is(err, ErrRevoked) {
		t.Fatalf("accepting a package revoked since: got %v", err)
	}
	if _, err := trust.Verify(k.build(t, 2)); err != nil {
		t.Fatalf("another package: %v", err)
	}
	if err := trust.UpdateRevocations(list); !errors.Is(err, ErrStale) {
		t.Fatalf("the same list again: got %v", err)
	}
	keyList, err := Revoke(k.root, 2, []string{p.KeyID}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := trust.UpdateRevocations(keyList); err != nil {
		t.Fatal(err)
	}
	if _, err := trust.Verify(k.build(t, 2)); !errors.Is(err, ErrRevoked) {
		t.Fatalf("a revoked key: got %v", err)
	}
	foreign, err := Revoke(newKeys(t).root, 3, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := trust.UpdateRevocations(foreign); !errors.Is(err, ErrUntrusted) {
		t.Fatalf("another root's list: got %v", err)
	}
	if _, err := Revoke(k.root, 0, nil, nil); err == nil {
		t.Fatal("made list 0")
	}
}

func TestRollback(t *testing.T) {
	k := newKeys(t)
	dir := t.TempDir()
	trust, err := OpenTrust(dir)
	if err != nil {
		t.Fatal(err)
	}
	trust.AddRoot(k.root.Public().(ed25519.PublicKey))
	p, err := trust.Verify(k.build(t, 5))
	if err != nil {
		t.Fatal(err)
	}
	// Until it accepts, the app may go back.
	if _, err := trust.Verify(k.build(t, 4)); err != nil {
		t.Fatal(err)
	}
	if err := trust.Accept(p); err != nil {
		t.Fatal(err)
	}
	list, err := Revoke(k.root, 7, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := trust.UpdateRevocations(list); err != nil {
		t.Fatal(err)
	}

	again, err := OpenTrust(dir)
	if err != nil {
		t.Fatal(err)
	}
	again.AddRoot(k.root.Public().(ed25519.PublicKey))
	if b, ok := again.AcceptedBuild("com.example.app"); !ok || b != 5 {
		t.Fatalf("accepted build %d, %v", b, ok)
	}
	if _, err := again.Verify(k.build(t, 4)); !errors.Is(err, ErrRollback) {
		t.Fatalf("an older build: got %v", err)
	}
	if _, err := again.Verify(k.build(t, 5)); err != nil {
		t.Fatalf("the same build: %v", err)
	}
	if err := again.UpdateRevocations(list); !errors.Is(err, ErrStale) {
		t.Fatalf("a replayed list: got %v", err)
	}
}

func TestResign(t *testing.T) {
	ci := newKeys(t, "*")
	platform := newKeys(t)
	data := ci.craft(t, map[string]any{"critical": []string{"app"}, "future": "kept"}, []byte("mount(1)"))
	again, err := Resign(data, platform.release, platform.cert)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ci.trust().Verify(again); !errors.Is(err, ErrUntrusted) {
		t.Fatalf("CI's root: got %v", err)
	}
	p, err := platform.trust().Verify(again)
	if err != nil {
		t.Fatal(err)
	}
	if p.Source() != "mount(1)" || len(p.Ignored) != 1 || p.KeyID != KeyID(platform.release.Public().(ed25519.PublicKey)) {
		t.Fatalf("got %q, ignored %v, key %s", p.Source(), p.Ignored, p.KeyID)
	}
	b := bytes.Clone(data)
	b[len(b)-70] ^= 1
	if _, err := Resign(b, platform.release, platform.cert); !errors.Is(err, ErrSignature) {
		t.Fatalf("a changed package: got %v", err)
	}
	if _, err := Resign(data, ci.release, platform.cert); !errors.Is(err, ErrCertificate) {
		t.Fatalf("another key's certificate: got %v", err)
	}
}

func TestInspect(t *testing.T) {
	k := newKeys(t)
	info, err := Inspect(k.build(t, 2))
	if err != nil {
		t.Fatal(err)
	}
	cert, err := info.Certificate.Info()
	if err != nil {
		t.Fatal(err)
	}
	if info.App.Build != 2 || cert.Signer != KeyID(k.root.Public().(ed25519.PublicKey)) || cert.Apps[0] != "com.example.*" {
		t.Fatalf("got %+v, %+v", info, cert)
	}
}

func TestKeys(t *testing.T) {
	k, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	back, err := DecodeSecretKey(EncodeSecretKey(k) + "\n")
	if err != nil || !back.Equal(k) {
		t.Fatalf("secret key: %v", err)
	}
	pub := k.Public().(ed25519.PublicKey)
	if p, err := DecodePublicKey(EncodePublicKey(pub)); err != nil || !p.Equal(pub) {
		t.Fatalf("public key: %v", err)
	}
	if _, err := DecodePublicKey("AAAA"); !errors.Is(err, ErrKey) {
		t.Fatalf("a short key: got %v", err)
	}
	if len(KeyID(pub)) != 16 {
		t.Fatalf("id %s", KeyID(pub))
	}
	certJSON, err := json.Marshal(newKeys(t).cert)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseCertificate(certJSON); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseCertificate([]byte(`{"body":"","signer":"","signature":"","extra":1}`)); !errors.Is(err, ErrFormat) {
		t.Fatalf("a field unknown: got %v", err)
	}
}

func TestAppMatches(t *testing.T) {
	for _, c := range []struct {
		pattern, id string
		want        bool
	}{
		{"*", "a", true},
		{"com.example.*", "com.example.app", true},
		{"com.example.*", "com.example", false},
		{"com.example.*", "com.examples.app", false},
		{"com.example.app", "com.example.app", true},
	} {
		if appMatches(c.pattern, c.id) != c.want {
			t.Errorf("%s %s", c.pattern, c.id)
		}
	}
	if !versionAtLeast("0.1.0", "0.1.0-beta") || versionAtLeast("0.1.0", "0.1.1") || !versionAtLeast("1.0.0", "0.9.9") {
		t.Error("versions")
	}
}
