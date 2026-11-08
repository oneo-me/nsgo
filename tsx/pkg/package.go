package pkg

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

// App is the app a package holds.
type App struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version string `json:"version"`
	// Build orders the app's packages: a Trust refuses one older than it
	// accepted before.
	Build uint64 `json:"build"`
}

// Contents is what Build signs.
type Contents struct {
	App App
	// Source is the bundle bundler.Compile wrote.
	Source string
	// MinRuntime is the oldest runtime the package runs on,
	// MAJOR.MINOR.PATCH, RuntimeVersion when empty and never newer.
	MinRuntime string
}

// Info is what a package says of itself.
type Info struct {
	App        App
	MinRuntime string
	// CreatedAt is when it was signed, by the release key KeyID, which
	// Certificate certifies.
	CreatedAt   time.Time
	KeyID       string
	Certificate *Certificate
	// SHA256 is the package's identity, the hash of the whole file in
	// hex, which revocation lists name.
	SHA256 string
	// Ignored names what this runtime does not know and left unused.
	Ignored []string
}

// A Package is one Trust.Verify verified, for tsx.View.
type Package struct {
	Info
	source string
}

// Source is the package's bundle.
func (p *Package) Source() string { return p.source }

const (
	magic     = "NSGP"
	format    = 1
	prefixLen = 12
	sourceKey = "source"
	// sourcePath names the bundle in the file table.
	sourcePath = "bundle.js"
)

type header struct {
	App         App               `json:"app"`
	MinRuntime  string            `json:"minRuntime"`
	CreatedAt   int64             `json:"createdAt"`
	Certificate *Certificate      `json:"certificate"`
	Files       []json.RawMessage `json:"files"`
	Critical    []string          `json:"critical,omitempty"`
}

var (
	headerFields = []string{"app", "minRuntime", "createdAt", "certificate", "files", "critical"}
	appFields    = []string{"id", "name", "version", "build"}
	fileFields   = []string{"path", "kind", "offset", "size", "sha256"}
)

type fileEntry struct {
	Path   string `json:"path"`
	Kind   string `json:"kind"`
	Offset uint64 `json:"offset"`
	Size   uint64 `json:"size"`
	SHA256 string `json:"sha256"`
}

// now is the clock packages and lists are signed by.
var now = time.Now

// Build signs contents with key, a release key cert certifies.
func Build(contents Contents, key ed25519.PrivateKey, cert *Certificate) ([]byte, error) {
	if err := checkApp(contents.App); err != nil {
		return nil, err
	}
	createdAt := now()
	if err := checkSigner(key, cert, contents.App.ID, createdAt); err != nil {
		return nil, err
	}
	min := contents.MinRuntime
	if min == "" {
		min = RuntimeVersion
	}
	if _, ok := versionParts(min); !ok {
		return nil, fmt.Errorf("%w: the minimum runtime %q is not MAJOR.MINOR.PATCH", ErrFormat, min)
	}
	if !versionAtLeast(RuntimeVersion, min) {
		return nil, fmt.Errorf("%w: the minimum runtime %s is newer than this one, %s", ErrFormat, min, RuntimeVersion)
	}
	if !utf8.ValidString(contents.Source) {
		return nil, fmt.Errorf("%w: the source is not UTF-8", ErrFormat)
	}
	return assemble(contents.App, min, []byte(contents.Source), key, cert, createdAt)
}

func checkApp(a App) error {
	if a.ID == "" || strings.ContainsAny(a.ID, "* \t\n") {
		return fmt.Errorf("%w: %q is not an app ID", ErrFormat, a.ID)
	}
	return nil
}

// checkSigner refuses to sign what devices would refuse.
func checkSigner(key ed25519.PrivateKey, cert *Certificate, app string, at time.Time) error {
	info, err := cert.Info()
	if err != nil {
		return err
	}
	if !info.Key.Equal(key.Public()) {
		return fmt.Errorf("%w: it certifies key %s, not %s", ErrCertificate, KeyID(info.Key), KeyID(key.Public().(ed25519.PublicKey)))
	}
	return info.covers(app, at)
}

func assemble(app App, min string, source []byte, key ed25519.PrivateKey, cert *Certificate, at time.Time) ([]byte, error) {
	sum := sha256.Sum256(source)
	file, err := json.Marshal(fileEntry{Path: sourcePath, Kind: sourceKey, Size: uint64(len(source)), SHA256: hex.EncodeToString(sum[:])})
	if err != nil {
		return nil, err
	}
	h, err := json.Marshal(header{App: app, MinRuntime: min, CreatedAt: at.Unix(), Certificate: cert, Files: []json.RawMessage{file}})
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	out.WriteString(magic)
	out.Write(binary.LittleEndian.AppendUint16(nil, format))
	out.Write(binary.LittleEndian.AppendUint16(nil, 0))
	out.Write(binary.LittleEndian.AppendUint32(nil, uint32(len(h))))
	out.Write(h)
	out.Write(source)
	out.Write(sign(key, contextPackage, out.Bytes()))
	return out.Bytes(), nil
}

// Resign signs a package again with key, a release key cert certifies:
// a platform signing with the key its apps trust what a build signed
// with keys of its own. Only the certificate, the signing time and the
// signature change; every other field is kept as written, those this
// runtime does not know included. The package must be intact, but its
// certificate is checked against no root.
func Resign(data []byte, key ed25519.PrivateKey, cert *Certificate) ([]byte, error) {
	p, err := selfSigned(data)
	if err != nil {
		return nil, err
	}
	if _, err := p.files(data); err != nil {
		return nil, err
	}
	createdAt := now()
	if err := checkSigner(key, cert, p.h.App.ID, createdAt); err != nil {
		return nil, err
	}
	// Rewritten as a map: header drops the fields this runtime does not
	// know, and dropping one critical names would lift its restriction.
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data[prefixLen:p.dataStart], &fields); err != nil {
		return nil, fmt.Errorf("%w: header: %v", ErrFormat, err)
	}
	if fields["certificate"], err = json.Marshal(cert); err != nil {
		return nil, err
	}
	fields["createdAt"] = json.RawMessage(fmt.Sprint(createdAt.Unix()))
	h, err := json.Marshal(fields)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	out.Write(data[:8])
	out.Write(binary.LittleEndian.AppendUint32(nil, uint32(len(h))))
	out.Write(h)
	out.Write(data[p.dataStart:p.signed])
	out.Write(sign(key, contextPackage, out.Bytes()))
	return out.Bytes(), nil
}

// Inspect reads what a package says, checking that its own certificate's
// key signed it and that its files are intact, but not its certificate
// against any root.
func Inspect(data []byte) (*Info, error) {
	p, err := selfSigned(data)
	if err != nil {
		return nil, err
	}
	pkg, err := p.contents(data)
	if err != nil {
		return nil, err
	}
	return &pkg.Info, nil
}

// parsed is a package's header, read but not yet believed.
type parsed struct {
	h         header
	ignored   []string
	dataStart int
	signed    int
}

func parse(data []byte) (*parsed, error) {
	if len(data) < prefixLen+ed25519.SignatureSize || string(data[:4]) != magic {
		return nil, fmt.Errorf("%w: no NSGo package header", ErrFormat)
	}
	if v := binary.LittleEndian.Uint16(data[4:]); v != format {
		return nil, fmt.Errorf("%w: format %d, where this runtime reads %d", ErrFormat, v, format)
	}
	n := uint64(binary.LittleEndian.Uint32(data[8:]))
	signed := len(data) - ed25519.SignatureSize
	if prefixLen+n > uint64(signed) {
		return nil, fmt.Errorf("%w: the header runs past the data", ErrFormat)
	}
	p := &parsed{dataStart: prefixLen + int(n), signed: signed}
	raw := data[prefixLen:p.dataStart]
	if err := json.Unmarshal(raw, &p.h); err != nil {
		return nil, fmt.Errorf("%w: header: %v", ErrFormat, err)
	}
	if p.h.Certificate == nil {
		return nil, fmt.Errorf("%w: no certificate", ErrFormat)
	}
	unknown, err := unknownFields(raw, headerFields)
	if err != nil {
		return nil, err
	}
	for _, f := range unknown {
		p.ignored = append(p.ignored, "header field "+f)
	}
	var app struct {
		App json.RawMessage `json:"app"`
	}
	_ = json.Unmarshal(raw, &app)
	if unknown, err = unknownFields(app.App, appFields); err != nil {
		return nil, err
	}
	for _, f := range unknown {
		p.ignored = append(p.ignored, "app field "+f)
	}
	return p, nil
}

// unknownFields lists the keys of the object raw that are not known.
func unknownFields(raw json.RawMessage, known []string) ([]string, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFormat, err)
	}
	var out []string
	for k := range m {
		if !slices.Contains(known, k) {
			out = append(out, k)
		}
	}
	slices.Sort(out)
	return out, nil
}

// selfSigned parses data and checks the signature of the key its
// certificate names.
func selfSigned(data []byte) (*parsed, error) {
	p, err := parse(data)
	if err != nil {
		return nil, err
	}
	info, err := p.h.Certificate.Info()
	if err != nil {
		return nil, err
	}
	if !verify(info.Key, contextPackage, data[:p.signed], data[p.signed:]) {
		return nil, ErrSignature
	}
	return p, nil
}

// file is a file of the table, its bytes checked.
type file struct {
	entry fileEntry
	known bool
	data  []byte
}

// files checks that the file table tiles the data and every file matches
// its hash.
func (p *parsed) files(data []byte) ([]file, error) {
	body := data[p.dataStart:p.signed]
	var out []file
	var at uint64
	for _, raw := range p.h.Files {
		var e fileEntry
		if err := json.Unmarshal(raw, &e); err != nil {
			return nil, fmt.Errorf("%w: file table: %v", ErrFormat, err)
		}
		unknown, err := unknownFields(raw, fileFields)
		if err != nil {
			return nil, err
		}
		if e.Offset != at || e.Size > uint64(len(body))-at {
			return nil, fmt.Errorf("%w: the file table does not tile the data at %s", ErrFormat, e.Path)
		}
		b := body[at : at+e.Size]
		at += e.Size
		sum := sha256.Sum256(b)
		if hex.EncodeToString(sum[:]) != strings.ToLower(e.SHA256) {
			return nil, fmt.Errorf("%w: %s", ErrIntegrity, e.Path)
		}
		out = append(out, file{entry: e, known: len(unknown) == 0 && e.Kind == sourceKey, data: b})
	}
	if at != uint64(len(body)) {
		return nil, fmt.Errorf("%w: the file table does not tile the data", ErrFormat)
	}
	return out, nil
}

// contents checks the files and reads the package from them.
func (p *parsed) contents(data []byte) (*Package, error) {
	files, err := p.files(data)
	if err != nil {
		return nil, err
	}
	cert, _ := p.h.Certificate.Info()
	sum := sha256.Sum256(data)
	pkg := &Package{Info: Info{
		App: p.h.App, MinRuntime: p.h.MinRuntime, CreatedAt: time.Unix(p.h.CreatedAt, 0),
		KeyID: KeyID(cert.Key), Certificate: p.h.Certificate, SHA256: hex.EncodeToString(sum[:]),
		Ignored: slices.Clone(p.ignored),
	}}
	sources := 0
	for _, f := range files {
		if !f.known {
			pkg.Ignored = append(pkg.Ignored, "file "+f.entry.Path)
			continue
		}
		sources++
		if !utf8.Valid(f.data) {
			return nil, fmt.Errorf("%w: the source is not UTF-8", ErrFormat)
		}
		pkg.source = string(f.data)
	}
	if sources != 1 {
		return nil, fmt.Errorf("%w: %d sources, where a package holds one", ErrFormat, sources)
	}
	return pkg, nil
}

// open verifies data against roots, short of revoked packages and
// rollback, which the Trust checks.
func open(data []byte, roots []ed25519.PublicKey, revokedKeys []string) (*Package, error) {
	p, err := parse(data)
	if err != nil {
		return nil, err
	}
	// The certificate first: it says which key may have signed.
	cert, err := p.h.Certificate.open(roots)
	if err != nil {
		return nil, err
	}
	id := KeyID(cert.Key)
	if slices.Contains(revokedKeys, id) {
		return nil, fmt.Errorf("%w: release key %s", ErrRevoked, id)
	}
	if !verify(cert.Key, contextPackage, data[:p.signed], data[p.signed:]) {
		return nil, ErrSignature
	}
	// Signed: the header can be believed from here on.
	unknown, _ := unknownFields(data[prefixLen:p.dataStart], headerFields)
	for _, f := range p.h.Critical {
		if slices.Contains(unknown, f) {
			return nil, fmt.Errorf("%w: it needs %s, which a newer runtime knows", ErrRuntime, f)
		}
	}
	if err := checkApp(p.h.App); err != nil {
		return nil, err
	}
	if err := cert.covers(p.h.App.ID, time.Unix(p.h.CreatedAt, 0)); err != nil {
		return nil, err
	}
	if _, ok := versionParts(p.h.MinRuntime); !ok {
		return nil, fmt.Errorf("%w: the minimum runtime %q is not MAJOR.MINOR.PATCH", ErrFormat, p.h.MinRuntime)
	}
	if !versionAtLeast(RuntimeVersion, p.h.MinRuntime) {
		return nil, fmt.Errorf("%w: it needs %s, and this is %s", ErrRuntime, p.h.MinRuntime, RuntimeVersion)
	}
	return p.contents(data)
}
