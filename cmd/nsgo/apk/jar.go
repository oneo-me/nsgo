package apk

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/base64"
	"math/big"
)

// A bundle is signed as jarsigner signs a JAR, which Play checks the
// upload key by: META-INF/MANIFEST.MF lists each file's SHA-256,
// META-INF/NSGO.SF the manifest's and each of its sections', and
// META-INF/NSGO.RSA is a PKCS #7 signature of NSGO.SF.

// The object identifiers of the signature beside the keystores' oidData.
var (
	oidSignedData = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 2}
	oidSHA256     = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 1}
	oidRSA        = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 1}
)

// signJar returns files signed: the signature's three files first, as
// jarsigner writes them, then files.
func signJar(files []File, key *Key) ([]File, error) {
	var manifest bytes.Buffer
	manifest.WriteString("Manifest-Version: 1.0\r\nCreated-By: nsgo\r\n\r\n")
	var sf bytes.Buffer
	var sections bytes.Buffer
	for _, f := range files {
		sum := sha256.Sum256(f.Data)
		var section bytes.Buffer
		section.WriteString(manifestLine("Name: " + f.Name))
		section.WriteString(manifestLine("SHA-256-Digest: " + base64.StdEncoding.EncodeToString(sum[:])))
		section.WriteString("\r\n")
		manifest.Write(section.Bytes())
		sectionSum := sha256.Sum256(section.Bytes())
		sections.WriteString(manifestLine("Name: " + f.Name))
		sections.WriteString(manifestLine("SHA-256-Digest: " + base64.StdEncoding.EncodeToString(sectionSum[:])))
		sections.WriteString("\r\n")
	}
	manifestSum := sha256.Sum256(manifest.Bytes())
	sf.WriteString("Signature-Version: 1.0\r\nCreated-By: nsgo\r\n")
	sf.WriteString(manifestLine("SHA-256-Digest-Manifest: " + base64.StdEncoding.EncodeToString(manifestSum[:])))
	sf.WriteString("\r\n")
	sf.Write(sections.Bytes())
	signature, err := pkcs7(sf.Bytes(), key)
	if err != nil {
		return nil, err
	}
	return append([]File{
		{Name: "META-INF/MANIFEST.MF", Data: manifest.Bytes()},
		{Name: "META-INF/NSGO.SF", Data: sf.Bytes()},
		{Name: "META-INF/NSGO.RSA", Data: signature},
	}, files...), nil
}

// manifestLine is a line of a JAR manifest: at most 72 bytes, the rest on
// lines that start with a space.
func manifestLine(s string) string {
	var b bytes.Buffer
	for first := true; ; first = false {
		n := 72
		if !first {
			b.WriteByte(' ')
			n = 71
		}
		if len(s) <= n {
			b.WriteString(s)
			b.WriteString("\r\n")
			return b.String()
		}
		b.WriteString(s[:n])
		b.WriteString("\r\n")
		s = s[n:]
	}
}

// pkcs7 is a detached PKCS #7 signature of data by key, carrying its
// certificate, without signed attributes, as jarsigner writes one.
func pkcs7(data []byte, key *Key) ([]byte, error) {
	sum := sha256.Sum256(data)
	signature, err := rsa.SignPKCS1v15(rand.Reader, key.Private, crypto.SHA256, sum[:])
	if err != nil {
		return nil, err
	}
	type issuerAndSerial struct {
		Issuer asn1.RawValue
		Serial *big.Int
	}
	type signerInfo struct {
		Version         int
		IssuerAndSerial issuerAndSerial
		Digest          pkix.AlgorithmIdentifier
		Encryption      pkix.AlgorithmIdentifier
		Signature       []byte
	}
	type contentInfo struct {
		Type asn1.ObjectIdentifier
	}
	type signedData struct {
		Version      int
		Digests      []pkix.AlgorithmIdentifier `asn1:"set"`
		Content      contentInfo
		Certificates asn1.RawValue
		Signers      []signerInfo `asn1:"set"`
	}
	sha := pkix.AlgorithmIdentifier{Algorithm: oidSHA256, Parameters: asn1.NullRawValue}
	sd := signedData{
		Version: 1,
		Digests: []pkix.AlgorithmIdentifier{sha},
		Content: contentInfo{oidData},
		// [0] IMPLICIT SET OF Certificate.
		Certificates: asn1.RawValue{Class: asn1.ClassContextSpecific, Tag: 0, IsCompound: true, Bytes: key.Cert.Raw},
		Signers: []signerInfo{{
			Version:         1,
			IssuerAndSerial: issuerAndSerial{asn1.RawValue{FullBytes: key.Cert.RawIssuer}, key.Cert.SerialNumber},
			Digest:          sha,
			Encryption:      pkix.AlgorithmIdentifier{Algorithm: oidRSA, Parameters: asn1.NullRawValue},
			Signature:       signature,
		}},
	}
	inner, err := asn1.Marshal(sd)
	if err != nil {
		return nil, err
	}
	return asn1.Marshal(struct {
		Type    asn1.ObjectIdentifier
		Content asn1.RawValue // [0] EXPLICIT SignedData
	}{oidSignedData, asn1.RawValue{Class: asn1.ClassContextSpecific, Tag: 0, IsCompound: true, Bytes: inner}})
}
