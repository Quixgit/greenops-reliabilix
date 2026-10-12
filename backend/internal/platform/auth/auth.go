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
	// Permissions is the permission set of a member with a custom role. nil: the built-in Role decides.
	Permissions map[Permission]struct{}
}

// Can reports whether the caller holds a permission: a custom role's set when there is one, else the built-in role's.
func (c Claims) Can(p Permission) bool {
	if c.Permissions != nil {
		_, ok := c.Permissions[p]
		return ok
	}
	return c.Role.Can(p)
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

// Claimer is implemented by a Resolver that can attach a first-time sign-in to an organization whose identity provider
// provisioned the person (SCIM). It is asked only for a verified e-mail and returns "" when nothing was provisioned.
type Claimer interface {
	ClaimProvisioned(ctx context.Context, subject, email string) (tenantID string, err error)
}

// PermissionResolver is implemented by a Resolver that also knows custom roles: it returns the permissions of a member
// whose Resolve answered RoleCustom.
type PermissionResolver interface {
	MemberPermissions(ctx context.Context, subject, tenantID string) ([]Permission, error)
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
				if errors.Is(err, ErrNoMembership) && c.KeyID == "" && c.EmailVerified && c.Email != "" {
					if cl, ok := res.(Claimer); ok {
						if joined, cerr := cl.ClaimProvisioned(r.Context(), c.Subject, c.Email); cerr != nil {
							httpx.Internal(w, r, cerr)
							return
						} else if joined != "" {
							t, role, err = res.Resolve(r.Context(), c.Subject, r.Header.Get("X-Tenant-ID"))
						}
					}
				}
				if err != nil {
					if errors.Is(err, ErrNoMembership) {
						httpx.WriteProblem(w, r, http.StatusForbidden, "no tenant membership", "onboarding_required")
						return
					}
					httpx.Internal(w, r, err)
					return
				}
				c.TenantID, c.Role = t, role
				if role == RoleCustom {
					c.Permissions = map[Permission]struct{}{} // fail closed: no set loaded, no permission
					if pr, ok := res.(PermissionResolver); ok {
						ps, err := pr.MemberPermissions(r.Context(), c.Subject, t)
						if err != nil {
							httpx.Internal(w, r, err)
							return
						}
						for _, p := range ps {
							c.Permissions[p] = struct{}{}
						}
					}
				}
			}
			httpx.SetTenant(r, c.TenantID) // lets the error log attribute a 5xx to the organization
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
			if !c.Can(p) {
				httpx.WriteProblem(w, r, http.StatusForbidden, "forbidden", "")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
