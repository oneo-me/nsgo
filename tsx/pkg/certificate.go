package pkg

import (
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// A Certificate is a root's word that a release key may sign packages of
// some apps for a time.
type Certificate struct{ s signed }

type certificateBody struct {
	Format    int      `json:"format"`
	Key       string   `json:"key"`
	NotBefore int64    `json:"notBefore"`
	NotAfter  int64    `json:"notAfter"`
	Apps      []string `json:"apps"`
}

// CertificateInfo is what a certificate says.
type CertificateInfo struct {
	// Key is the release key, and Signer the ID of the root that signed.
	Key    ed25519.PublicKey
	Signer string
	// NotBefore and NotAfter bound when the key may sign.
	NotBefore, NotAfter time.Time
	// Apps are the IDs or patterns ("*", "com.example.*") it may sign.
	Apps []string
}

// Certify has root certify release to sign packages of apps between
// notBefore and notAfter.
func Certify(root ed25519.PrivateKey, release ed25519.PublicKey, apps []string, notBefore, notAfter time.Time) (*Certificate, error) {
	if len(release) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("%w: the release key is %d bytes", ErrKey, len(release))
	}
	if len(apps) == 0 {
		return nil, fmt.Errorf("%w: a certificate names the apps it covers", ErrFormat)
	}
	for _, a := range apps {
		if err := checkAppPattern(a); err != nil {
			return nil, err
		}
	}
	if !notAfter.After(notBefore) {
		return nil, fmt.Errorf("%w: a certificate ends after it starts", ErrFormat)
	}
	s, err := signBody(root, contextCertificate, certificateBody{
		Format: 1, Key: EncodePublicKey(release), NotBefore: notBefore.Unix(), NotAfter: notAfter.Unix(), Apps: apps,
	})
	if err != nil {
		return nil, err
	}
	return &Certificate{s}, nil
}

func checkAppPattern(p string) error {
	id, _ := strings.CutSuffix(p, ".*")
	if p == "*" || id != "" && !strings.ContainsAny(id, "* \t\n") {
		return nil
	}
	return fmt.Errorf("%w: %q is not an app ID or pattern (*, com.example.* or an ID)", ErrFormat, p)
}

// ParseCertificate reads a certificate MarshalJSON wrote.
func ParseCertificate(data []byte) (*Certificate, error) {
	c := new(Certificate)
	if err := c.UnmarshalJSON(data); err != nil {
		return nil, err
	}
	return c, nil
}

// MarshalJSON writes the certificate as JSON.
func (c *Certificate) MarshalJSON() ([]byte, error) { return json.Marshal(c.s) }

// UnmarshalJSON reads a certificate.
func (c *Certificate) UnmarshalJSON(data []byte) error {
	if err := decodeStrict(data, &c.s); err != nil {
		return fmt.Errorf("certificate: %w", err)
	}
	return nil
}

// Info is what the certificate says, unchecked: nothing has verified
// its root's signature.
func (c *Certificate) Info() (CertificateInfo, error) {
	var b certificateBody
	if err := c.s.unchecked(&b); err != nil {
		return CertificateInfo{}, err
	}
	return b.info(c.s.Signer)
}

// open is what the certificate says once one of roots verifies it.
func (c *Certificate) open(roots []ed25519.PublicKey) (CertificateInfo, error) {
	var b certificateBody
	if err := c.s.open(contextCertificate, roots, &b); err != nil {
		return CertificateInfo{}, err
	}
	return b.info(c.s.Signer)
}

func (b certificateBody) info(signer string) (CertificateInfo, error) {
	if b.Format != 1 {
		return CertificateInfo{}, fmt.Errorf("%w: certificate format %d", ErrFormat, b.Format)
	}
	key, err := DecodePublicKey(b.Key)
	if err != nil {
		return CertificateInfo{}, err
	}
	return CertificateInfo{Key: key, Signer: signer, NotBefore: time.Unix(b.NotBefore, 0), NotAfter: time.Unix(b.NotAfter, 0), Apps: b.Apps}, nil
}

// covers checks that the certificate lets its key sign app at signedAt.
func (i CertificateInfo) covers(app string, signedAt time.Time) error {
	matched := false
	for _, p := range i.Apps {
		matched = matched || appMatches(p, app)
	}
	if !matched {
		return fmt.Errorf("%w: it covers %s, not %s", ErrCertificate, strings.Join(i.Apps, ", "), app)
	}
	if signedAt.Before(i.NotBefore) || signedAt.After(i.NotAfter) {
		return fmt.Errorf("%w: signed at %s, outside %s to %s", ErrCertificate,
			signedAt.UTC().Format(time.RFC3339), i.NotBefore.UTC().Format(time.RFC3339), i.NotAfter.UTC().Format(time.RFC3339))
	}
	return nil
}
