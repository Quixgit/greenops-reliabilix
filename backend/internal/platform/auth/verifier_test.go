package auth

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func sign(t *testing.T, k *rsa.PrivateKey, alg string, claims map[string]any) string {
	t.Helper()
	h, _ := json.Marshal(map[string]string{"alg": alg, "kid": "k1", "typ": "JWT"})
	c, _ := json.Marshal(claims)
	in := b64(h) + "." + b64(c)
	sum := sha256.Sum256([]byte(in))
	sig, err := rsa.SignPKCS1v15(rand.Reader, k, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatal(err)
	}
	return in + "." + b64(sig)
}

func TestOIDCVerifier(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	jwks := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kid": "k1", "kty": "RSA", "n": b64(key.N.Bytes()), "e": b64(big.NewInt(int64(key.E)).Bytes())}}})
	}))
	defer jwks.Close()

	v := NewOIDCVerifier("example.auth0.com", "https://api.reliabilix.com", "https://reliabilix.com/")
	v.JWKSURL = jwks.URL
	good := func() map[string]any {
		return map[string]any{"iss": v.Issuer, "aud": "https://api.reliabilix.com", "sub": "auth0|1",
			"exp": time.Now().Add(time.Hour).Unix(), v.ClaimNS + "email": "a@b.co", v.ClaimNS + "email_verified": true}
	}
	ctx := context.Background()

	c, err := v.Verify(ctx, sign(t, key, "RS256", good()))
	if err != nil || c.Subject != "auth0|1" || c.Email != "a@b.co" || !c.EmailVerified || c.TenantID != "" {
		t.Fatalf("valid token rejected: %v %+v", err, c)
	}
	mut := func(f func(m map[string]any)) string { m := good(); f(m); return sign(t, key, "RS256", m) }
	bad := map[string]string{
		"expired":   mut(func(m map[string]any) { m["exp"] = time.Now().Add(-time.Hour).Unix() }),
		"wrong iss": mut(func(m map[string]any) { m["iss"] = "https://evil/" }),
		"wrong aud": mut(func(m map[string]any) { m["aud"] = "other" }),
		"no sub":    mut(func(m map[string]any) { delete(m, "sub") }),
		"hs256":     sign(t, key, "HS256", good()),
	}
	other, _ := rsa.GenerateKey(rand.Reader, 2048)
	bad["foreign key"] = sign(t, other, "RS256", good())
	for name, tok := range bad {
		if _, err := v.Verify(ctx, tok); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
