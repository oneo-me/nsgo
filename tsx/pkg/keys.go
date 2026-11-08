package pkg

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// GenerateKey makes a root or release key.
func GenerateKey() (ed25519.PrivateKey, error) {
	_, key, err := ed25519.GenerateKey(rand.Reader)
	return key, err
}

// KeyID is the ID of a public key: the first 8 bytes of its SHA-256, in
// hex.
func KeyID(key ed25519.PublicKey) string {
	sum := sha256.Sum256(key)
	return hex.EncodeToString(sum[:8])
}

// EncodeSecretKey writes a secret key as the base64 of its seed.
func EncodeSecretKey(key ed25519.PrivateKey) string {
	return base64.StdEncoding.EncodeToString(key.Seed())
}

// DecodeSecretKey reads a secret key EncodeSecretKey wrote.
func DecodeSecretKey(text string) (ed25519.PrivateKey, error) {
	seed, err := decode32(text, "a secret key")
	if err != nil {
		return nil, err
	}
	return ed25519.NewKeyFromSeed(seed), nil
}

// EncodePublicKey writes a public key as base64.
func EncodePublicKey(key ed25519.PublicKey) string {
	return base64.StdEncoding.EncodeToString(key)
}

// DecodePublicKey reads a public key EncodePublicKey wrote.
func DecodePublicKey(text string) (ed25519.PublicKey, error) {
	b, err := decode32(text, "a public key")
	if err != nil {
		return nil, err
	}
	return ed25519.PublicKey(b), nil
}

func decode32(text, what string) ([]byte, error) {
	b, err := base64.StdEncoding.DecodeString(strings.TrimSpace(text))
	if err != nil {
		return nil, fmt.Errorf("%w: %s is not base64: %v", ErrKey, what, err)
	}
	if len(b) != 32 {
		return nil, fmt.Errorf("%w: %s is 32 bytes, not %d", ErrKey, what, len(b))
	}
	return b, nil
}

// The contexts each kind of signature is made in.
const (
	contextPackage     = "nsgo-package-v1"
	contextCertificate = "nsgo-certificate-v1"
	contextRevocations = "nsgo-revocations-v1"
)

// framed is what a signature in context signs of message.
func framed(context string, message []byte) []byte {
	sum := sha256.Sum256(message)
	return append(append([]byte(context), 0), sum[:]...)
}

func sign(key ed25519.PrivateKey, context string, message []byte) []byte {
	return ed25519.Sign(key, framed(context, message))
}

func verify(key ed25519.PublicKey, context string, message, signature []byte) bool {
	return len(signature) == ed25519.SignatureSize && ed25519.Verify(key, framed(context, message), signature)
}

// signed is a JSON body a root signed: a certificate or a revocation
// list.
type signed struct {
	Body      string `json:"body"`
	Signer    string `json:"signer"`
	Signature string `json:"signature"`
}

func signBody(key ed25519.PrivateKey, context string, body any) (signed, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return signed{}, err
	}
	return signed{
		Body:      base64.StdEncoding.EncodeToString(data),
		Signer:    KeyID(key.Public().(ed25519.PublicKey)),
		Signature: base64.StdEncoding.EncodeToString(sign(key, context, data)),
	}, nil
}

// unchecked reads the body without checking who signed it.
func (s signed) unchecked(v any) error {
	data, err := base64.StdEncoding.DecodeString(s.Body)
	if err != nil {
		return fmt.Errorf("%w: the signed body is not base64: %v", ErrFormat, err)
	}
	return decodeStrict(data, v)
}

// open reads the body once one of roots verifies its signature.
func (s signed) open(context string, roots []ed25519.PublicKey, v any) error {
	var root ed25519.PublicKey
	for _, r := range roots {
		if KeyID(r) == s.Signer {
			root = r
			break
		}
	}
	if root == nil {
		return fmt.Errorf("%w: key %s", ErrUntrusted, s.Signer)
	}
	data, err := base64.StdEncoding.DecodeString(s.Body)
	if err != nil {
		return fmt.Errorf("%w: the signed body is not base64: %v", ErrFormat, err)
	}
	signature, err := base64.StdEncoding.DecodeString(s.Signature)
	if err != nil || !verify(root, context, data, signature) {
		return ErrSignature
	}
	return decodeStrict(data, v)
}

// decodeStrict decodes JSON that may hold no field v lacks.
func decodeStrict(data []byte, v any) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return fmt.Errorf("%w: %v", ErrFormat, err)
	}
	return nil
}
