// Package secretbox protects the few customer-supplied secrets the platform has to be able to use again
// (a Slack webhook URL, a webhook endpoint that carries a token). Values are sealed with AES-256-GCM under a key
// that comes from the environment, so the database only ever holds ciphertext (ADR-0010, ADR-0019).
//
// The additional data binds a ciphertext to its row: copying a sealed value to another destination or tenant
// makes Open fail. The sealed form is "v1:" + base64(nonce || ciphertext); the version prefix leaves room for key
// rotation.
package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

const prefix = "v1:"

var ErrInvalid = errors.New("secretbox: cannot open value")

// Box seals and opens values under one key. The zero value (and a nil *Box) is unusable: check Configured.
type Box struct{ key []byte }

// New builds a Box from a 32-byte key.
func New(key []byte) (*Box, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("secretbox: key must be 32 bytes, got %d", len(key))
	}
	return &Box{key: append([]byte(nil), key...)}, nil
}

// ParseKey decodes a base64 (standard or URL, padded or not) 32-byte key, e.g. from `openssl rand -base64 32`.
func ParseKey(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		if b, err := enc.DecodeString(s); err == nil && len(b) == 32 {
			return b, nil
		}
	}
	return nil, errors.New("secretbox: key must be 32 random bytes encoded as base64 (openssl rand -base64 32)")
}

// Configured reports whether the box can be used.
func (b *Box) Configured() bool { return b != nil && len(b.key) == 32 }

func (b *Box) aead() (cipher.AEAD, error) {
	blk, err := aes.NewCipher(b.key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(blk)
}

// Seal encrypts plaintext; aad must be the same at Open time (use "<tenant>:<row id>").
func (b *Box) Seal(plaintext, aad string) (string, error) {
	if !b.Configured() {
		return "", errors.New("secretbox: no key configured")
	}
	g, err := b.aead()
	if err != nil {
		return "", err
	}
	nonce := make([]byte, g.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	return prefix + base64.RawStdEncoding.EncodeToString(g.Seal(nonce, nonce, []byte(plaintext), []byte(aad))), nil
}

// Open reverses Seal. Any tampering, wrong key or wrong aad yields ErrInvalid.
func (b *Box) Open(sealed, aad string) (string, error) {
	if !b.Configured() {
		return "", errors.New("secretbox: no key configured")
	}
	raw, ok := strings.CutPrefix(sealed, prefix)
	if !ok {
		return "", ErrInvalid
	}
	data, err := base64.RawStdEncoding.DecodeString(raw)
	if err != nil {
		return "", ErrInvalid
	}
	g, err := b.aead()
	if err != nil {
		return "", err
	}
	if len(data) < g.NonceSize() {
		return "", ErrInvalid
	}
	pt, err := g.Open(nil, data[:g.NonceSize()], data[g.NonceSize():], []byte(aad))
	if err != nil {
		return "", ErrInvalid
	}
	return string(pt), nil
}

// Derive returns a stable per-purpose secret (hex) from the key, e.g. the signing secret of one webhook
// destination. Nothing is stored: it is recomputed on demand, and it differs for every (purpose, id).
func (b *Box) Derive(purpose, id string) string {
	if !b.Configured() {
		return ""
	}
	m := hmac.New(sha256.New, b.key)
	m.Write([]byte("derive:" + purpose + ":" + id))
	return hex.EncodeToString(m.Sum(nil))
}
