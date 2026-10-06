package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"

	httpx "github.com/quixgit/greenops-reliabilix/backend/internal/platform/http"
)

// Claims is the verified identity of the caller. TenantID and Role are NOT taken from the
// token: they are resolved from the database (memberships) or from the API key row.
type Claims struct {
	Subject       string
	Email         string
	EmailVerified bool
	TenantID      string
	Role          Role
	KeyID         string // set for API-key callers
}

// Verifier validates a bearer token and returns its identity (subject, email).
type Verifier interface {
	Verify(ctx context.Context, token string) (Claims, error)
}

// Resolver maps a verified subject to a tenant membership.
// wantedTenant (the X-Tenant-ID header) selects among several memberships and must belong to the subject.
type Resolver interface {
	Resolve(ctx context.Context, subject, wantedTenant string) (tenantID string, role Role, err error)
}

var (
	// ErrUnauthenticated is returned for any invalid, expired or malformed token.
	ErrUnauthenticated = errors.New("unauthenticated")
	// ErrNoMembership means the subject is valid but belongs to no (requested) tenant: onboarding is required.
	ErrNoMembership = errors.New("no tenant membership")
)

type ctxKey struct{}

// FromContext returns the claims placed by Authenticate.
func FromContext(ctx context.Context) (Claims, bool) {
	c, ok := ctx.Value(ctxKey{}).(Claims)
	return c, ok
}

// WithClaims returns ctx carrying c. Intended for tests and background jobs acting on behalf of an identity.
func WithClaims(ctx context.Context, c Claims) context.Context {
	return context.WithValue(ctx, ctxKey{}, c)
}

func bearer(r *http.Request) (string, bool) {
	tok, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	return tok, ok && tok != ""
}

// Authenticate verifies the Bearer token and establishes the tenant context. When the token carries no
// tenant (OIDC users) the Resolver supplies tenant and role; callers without membership get 403 "onboarding_required".
func Authenticate(v Verifier, res Resolver) httpx.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tok, ok := bearer(r)
			if !ok {
				httpx.WriteProblem(w, r, http.StatusUnauthorized, "unauthorized", "")
				return
			}
			c, err := v.Verify(r.Context(), tok)
			if err != nil || c.Subject == "" {
				httpx.WriteProblem(w, r, http.StatusUnauthorized, "unauthorized", "")
				return
			}
			if c.TenantID == "" {
				if res == nil {
					httpx.WriteProblem(w, r, http.StatusUnauthorized, "unauthorized", "")
					return
				}
				t, role, err := res.Resolve(r.Context(), c.Subject, r.Header.Get("X-Tenant-ID"))
				if err != nil {
					if errors.Is(err, ErrNoMembership) {
						httpx.WriteProblem(w, r, http.StatusForbidden, "no tenant membership", "onboarding_required")
						return
					}
					httpx.WriteProblem(w, r, http.StatusInternalServerError, "internal error", "")
					return
				}
				c.TenantID, c.Role = t, role
			}
			next.ServeHTTP(w, r.WithContext(WithClaims(r.Context(), c)))
		})
	}
}

// AuthenticateIdentity verifies the token only (no tenant). Used by onboarding and invitation acceptance,
// where the caller legitimately has no membership yet.
func AuthenticateIdentity(v Verifier) httpx.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tok, ok := bearer(r)
			if !ok {
				httpx.WriteProblem(w, r, http.StatusUnauthorized, "unauthorized", "")
				return
			}
			c, err := v.Verify(r.Context(), tok)
			if err != nil || c.Subject == "" || c.KeyID != "" {
				httpx.WriteProblem(w, r, http.StatusUnauthorized, "unauthorized", "")
				return
			}
			next.ServeHTTP(w, r.WithContext(WithClaims(r.Context(), c)))
		})
	}
}

// Require enforces a permission. It must run after Authenticate.
func Require(p Permission) httpx.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c, ok := FromContext(r.Context())
			if !ok || c.TenantID == "" {
				httpx.WriteProblem(w, r, http.StatusUnauthorized, "unauthorized", "")
				return
			}
			if !c.Role.Can(p) {
				httpx.WriteProblem(w, r, http.StatusForbidden, "forbidden", "")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
