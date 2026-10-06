package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
)

// APIKeyPrefix marks machine credentials (CI gates). Only the SHA-256 of a key is ever stored.
const APIKeyPrefix = "grk_"

// HashAPIKey returns the storage/lookup hash of a key.
func HashAPIKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

// GenerateAPIKey returns a new random key and its display prefix. The key is shown once to the caller.
func GenerateAPIKey() (key, prefix string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	key = APIKeyPrefix + base64.RawURLEncoding.EncodeToString(b)
	return key, key[:len(APIKeyPrefix)+6], nil
}

// APIKeyLookup resolves a key hash to its tenant and role (revoked or unknown keys return ErrUnauthenticated).
type APIKeyLookup func(ctx context.Context, hash string) (tenantID string, role Role, keyID string, err error)

// KeyedVerifier authenticates "grk_..." keys through Lookup and delegates every other token to Fallback.
type KeyedVerifier struct {
	Lookup   APIKeyLookup
	Fallback Verifier
}

func (k KeyedVerifier) Verify(ctx context.Context, token string) (Claims, error) {
	if !strings.HasPrefix(token, APIKeyPrefix) {
		return k.Fallback.Verify(ctx, token)
	}
	tenant, role, id, err := k.Lookup(ctx, HashAPIKey(token))
	if err != nil {
		return Claims{}, ErrUnauthenticated
	}
	return Claims{Subject: "apikey:" + id, TenantID: tenant, Role: role, KeyID: id}, nil
}
