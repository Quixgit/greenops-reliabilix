package auth

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
)

// DevVerifier accepts "dev:<subject>:<tenant>:<role>". Only constructed when ENV=dev.
type DevVerifier struct{}

func (DevVerifier) Verify(_ context.Context, token string) (Claims, error) {
	p := strings.Split(token, ":")
	if len(p) != 4 || p[0] != "dev" {
		return Claims{}, ErrUnauthenticated
	}
	return Claims{Subject: p[1], TenantID: p[2], Role: Role(p[3])}, nil
}

// OIDCVerifier validates RS256 JWTs issued by Auth0 (or any OIDC provider):
// signature via cached JWKS, iss, aud, exp, nbf. Tenant and role are read from
// namespaced custom claims set by an Auth0 Action. HS256/none are rejected.
type OIDCVerifier struct {
	Issuer   string // with trailing slash, e.g. https://tenant.auth0.com/
	Audience string
	ClaimNS  string
	JWKSURL  string
	Client   *http.Client
	Now      func() time.Time

	mu        sync.Mutex
	keys      map[string]*rsa.PublicKey
	fetchedAt time.Time
}

func NewOIDCVerifier(domain, audience, claimNS string) *OIDCVerifier {
	iss := "https://" + domain + "/"
	return &OIDCVerifier{Issuer: iss, Audience: audience, ClaimNS: claimNS,
		JWKSURL: iss + ".well-known/jwks.json", Client: &http.Client{Timeout: 5 * time.Second}, Now: time.Now}
}

func (v *OIDCVerifier) Verify(ctx context.Context, token string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, ErrUnauthenticated
	}
	var hdr struct{ Alg, Kid string }
	if err := decodePart(parts[0], &hdr); err != nil || hdr.Alg != "RS256" || hdr.Kid == "" {
		return Claims{}, ErrUnauthenticated
	}
	key, err := v.key(ctx, hdr.Kid)
	if err != nil {
		return Claims{}, ErrUnauthenticated
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return Claims{}, ErrUnauthenticated
	}
	sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if rsa.VerifyPKCS1v15(key, crypto.SHA256, sum[:], sig) != nil {
		return Claims{}, ErrUnauthenticated
	}
	var c map[string]any
	if err := decodePart(parts[1], &c); err != nil {
		return Claims{}, ErrUnauthenticated
	}
	now := v.Now()
	const leeway = 30 * time.Second
	if iss, _ := c["iss"].(string); iss != v.Issuer {
		return Claims{}, ErrUnauthenticated
	}
	if !audMatches(c["aud"], v.Audience) {
		return Claims{}, ErrUnauthenticated
	}
	exp, ok := c["exp"].(float64)
	if !ok || now.After(time.Unix(int64(exp), 0).Add(leeway)) {
		return Claims{}, ErrUnauthenticated
	}
	if nbf, ok := c["nbf"].(float64); ok && now.Add(leeway).Before(time.Unix(int64(nbf), 0)) {
		return Claims{}, ErrUnauthenticated
	}
	sub, _ := c["sub"].(string)
	tenant, _ := c[v.ClaimNS+"tenant_id"].(string)
	role, _ := c[v.ClaimNS+"role"].(string)
	if sub == "" || tenant == "" {
		return Claims{}, ErrUnauthenticated
	}
	return Claims{Subject: sub, TenantID: tenant, Role: Role(role)}, nil
}

func audMatches(aud any, want string) bool {
	switch a := aud.(type) {
	case string:
		return a == want
	case []any:
		for _, x := range a {
			if s, _ := x.(string); s == want {
				return true
			}
		}
	}
	return false
}

func decodePart(p string, dst any) error {
	b, err := base64.RawURLEncoding.DecodeString(p)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dst)
}

// key returns the signing key for kid, refreshing the JWKS at most once a
// minute when the kid is unknown (key rotation) and every 10 minutes otherwise.
func (v *OIDCVerifier) key(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	age := v.Now().Sub(v.fetchedAt)
	if k, ok := v.keys[kid]; ok && age < 10*time.Minute {
		return k, nil
	}
	if v.keys != nil && age < time.Minute {
		return nil, errors.New("unknown kid")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.JWKSURL, nil) //nolint:gosec // JWKSURL derives from operator config (AUTH0_DOMAIN), never from request input
	if err != nil {
		return nil, err
	}
	resp, err := v.Client.Do(req) //nolint:gosec // see above
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jwks: status %d", resp.StatusCode)
	}
	var set struct {
		Keys []struct{ Kid, Kty, N, E string } `json:"keys"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&set); err != nil {
		return nil, err
	}
	keys := map[string]*rsa.PublicKey{}
	for _, k := range set.Keys {
		if k.Kty != "RSA" {
			continue
		}
		n, err1 := base64.RawURLEncoding.DecodeString(k.N)
		e, err2 := base64.RawURLEncoding.DecodeString(k.E)
		if err1 != nil || err2 != nil {
			continue
		}
		keys[k.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}
	}
	v.keys, v.fetchedAt = keys, v.Now()
	if k, ok := keys[kid]; ok {
		return k, nil
	}
	return nil, errors.New("unknown kid")
}
