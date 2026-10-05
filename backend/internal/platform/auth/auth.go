package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"

	httpx "github.com/quixgit/greenops-reliabilix/backend/internal/platform/http"
)

// Claims is the verified identity of the caller.
type Claims struct {
	Subject  string
	TenantID string
	Role     Role
}

// Verifier validates a bearer token and returns its claims.
type Verifier interface {
	Verify(ctx context.Context, token string) (Claims, error)
}

// ErrUnauthenticated is returned for any invalid, expired or malformed token.
var ErrUnauthenticated = errors.New("unauthenticated")

type ctxKey struct{}

// FromContext returns the claims placed by Authenticate.
func FromContext(ctx context.Context) (Claims, bool) {
	c, ok := ctx.Value(ctxKey{}).(Claims)
	return c, ok
}

// Authenticate verifies the Bearer token and stores Claims in the request context.
func Authenticate(v Verifier) httpx.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := r.Header.Get("Authorization")
			tok, ok := strings.CutPrefix(h, "Bearer ")
			if !ok || tok == "" {
				httpx.WriteProblem(w, r, http.StatusUnauthorized, "unauthorized", "")
				return
			}
			c, err := v.Verify(r.Context(), tok)
			if err != nil || c.TenantID == "" || c.Subject == "" {
				httpx.WriteProblem(w, r, http.StatusUnauthorized, "unauthorized", "")
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, c)))
		})
	}
}

// Require enforces a permission. It must run after Authenticate.
func Require(p Permission) httpx.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c, ok := FromContext(r.Context())
			if !ok {
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

// WithClaims returns ctx carrying c. Intended for tests and background jobs
// that act on behalf of a verified identity.
func WithClaims(ctx context.Context, c Claims) context.Context {
	return context.WithValue(ctx, ctxKey{}, c)
}
