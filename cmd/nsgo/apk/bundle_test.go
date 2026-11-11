package apk

import (
	"archive/zip"
	"bytes"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/asn1"
	"encoding/base64"
	"io"
	"strings"
	"testing"

	androidruntime "github.com/oneo-me/nsgo/internal/platform/android/runtime"
)

// A bundle holds the base module and its config, signed as a JAR whose
// digests and signature check out.
func TestWriteBundle(t *testing.T) {
	key, err := NewKey()
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	err = WriteBundle(&buf, androidruntime.Manifest, Manifest{
		Package:     "dev.oneo.nsgo.test",
		Label:       "Test",
		Names:       map[string]string{"": "Test", "zh-Hans": "测试"},
		VersionName: "1.0.0",
		VersionCode: 10000,
		Icon:        []byte("\x89PNG icon"),
		TargetSDK:   35,
	}, []File{
		{Name: "classes.dex", Data: []byte("dex")},
		{Name: "lib/arm64-v8a/libnsgo.so", Data: []byte("go")},
		{Name: "assets/a.txt", Data: []byte("a")},
	}, key)
	if err != nil {
		t.Fatal(err)
	}
	r, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	var names []string
	for _, f := range r.File {
		rc, _ := f.Open()
		files[f.Name], _ = io.ReadAll(rc)
		names = append(names, f.Name)
	}
	want := "META-INF/MANIFEST.MF,META-INF/NSGO.SF,META-INF/NSGO.RSA,base/manifest/AndroidManifest.xml,base/dex/classes.dex," +
		"base/lib/arm64-v8a/libnsgo.so,base/assets/a.txt,base/res/mipmap-xxxhdpi-v4/ic_launcher.png,base/resources.pb,BundleConfig.pb"
	if got := strings.Join(names, ","); got != want {
		t.Fatalf("the bundle holds %s", got)
	}

	// The manifest's strings, the target SDK among them, and the
	// resource table's names.
	manifest := string(files["base/manifest/AndroidManifest.xml"])
	for _, s := range []string{"dev.oneo.nsgo.test", "targetSdkVersion", "35", "dev.oneo.nsgo.test.nsgo.files", androidNS} {
		if !strings.Contains(manifest, s) {
			t.Errorf("the manifest lacks %q", s)
		}
	}
	table := string(files["base/resources.pb"])
	for _, s := range []string{"ic_launcher", "app_name", "测试", "zh-Hans", iconFile} {
		if !strings.Contains(table, s) {
			t.Errorf("the resource table lacks %q", s)
		}
	}
	if !bytes.Equal(files["BundleConfig.pb"], []byte("\x0a\x08\x12\x06"+BundletoolVersion+"\x12\x02\x12\x00")) {
		t.Errorf("BundleConfig.pb is %x", files["BundleConfig.pb"])
	}

	// Each file's digest is in the manifest, the manifest's in the
	// signature file, and the key signed that.
	mf := strings.ReplaceAll(string(files["META-INF/MANIFEST.MF"]), "\r\n ", "")
	for _, name := range names[3:] {
		sum := sha256.Sum256(files[name])
		if !strings.Contains(mf, "Name: "+name+"\r\nSHA-256-Digest: "+base64.StdEncoding.EncodeToString(sum[:])) {
			t.Errorf("the manifest lacks %s's digest", name)
		}
	}
	sf := files["META-INF/NSGO.SF"]
	sum := sha256.Sum256(files["META-INF/MANIFEST.MF"])
	if !bytes.Contains(sf, []byte("SHA-256-Digest-Manifest: "+base64.StdEncoding.EncodeToString(sum[:]))) {
		t.Error("the signature file lacks the manifest's digest")
	}
	var outer struct {
		Type    asn1.ObjectIdentifier
		Content asn1.RawValue
	}
	if _, err := asn1.Unmarshal(files["META-INF/NSGO.RSA"], &outer); err != nil || !outer.Type.Equal(oidSignedData) {
		t.Fatalf("NSGO.RSA is no signed data: %v", err)
	}
	var sd struct {
		Version      int
		Digests      asn1.RawValue
		Content      asn1.RawValue
		Certificates asn1.RawValue
		Signers      []struct {
			Version         int
			IssuerAndSerial asn1.RawValue
			Digest          asn1.RawValue
			Encryption      asn1.RawValue
			Signature       []byte
		} `asn1:"set"`
	}
	if _, err := asn1.Unmarshal(outer.Content.Bytes, &sd); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(sd.Certificates.Bytes, key.Cert.Raw) || len(sd.Signers) != 1 {
		t.Fatal("NSGO.RSA does not carry the key's certificate and one signer")
	}
	digest := sha256.Sum256(sf)
	if err := rsa.VerifyPKCS1v15(&key.Private.PublicKey, crypto.SHA256, digest[:], sd.Signers[0].Signature); err != nil {
		t.Errorf("the signature does not verify: %v", err)
	}
}

func TestManifestLinesWrap(t *testing.T) {
	line := manifestLine("Name: " + strings.Repeat("a", 100))
	for i, l := range strings.Split(strings.TrimSuffix(line, "\r\n"), "\r\n") {
		if len(l) > 72 || i > 0 && l[0] != ' ' {
			t.Errorf("line %d is %q", i, l)
		}
	}
	if strings.ReplaceAll(line, "\r\n ", "") != "Name: "+strings.Repeat("a", 100)+"\r\n" {
		t.Errorf("the line wraps into %q", line)
	}
}

func TestProtoWire(t *testing.T) {
	if got := pbVarint(nil, 5, 300); !bytes.Equal(got, []byte{0x28, 0xac, 0x02}) {
		t.Errorf("a varint field is %x", got)
	}
	if got := pbString(nil, 2, "hi"); !bytes.Equal(got, []byte{0x12, 2, 'h', 'i'}) {
		t.Errorf("a string field is %x", got)
	}
}
