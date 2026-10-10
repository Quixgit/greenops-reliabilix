package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/settings"
)

// suspendedReader answers true for the suspension switch of one organization only.
type suspendedReader struct {
	settings.Defaults
	tenant string
}

func (r suspendedReader) Bool(ctx context.Context, tenantID, key string) bool {
	if key == settings.KeyOrgSuspended {
		return tenantID == r.tenant
	}
	return r.Defaults.Bool(ctx, tenantID, key)
}

func TestSuspendedOrganizationCanReadButNotChange(t *testing.T) {
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			tenant := req.Header.Get("X-Tenant")
			next.ServeHTTP(w, req.WithContext(auth.WithClaims(req.Context(), auth.Claims{Subject: "u", TenantID: tenant, Role: auth.RoleOwner})))
		})
	})
	gatesModule{cfg: suspendedReader{tenant: "t-suspended"}}.Routes(r)
	ok := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }
	r.Get("/api/v1/projects", ok)
	r.Post("/api/v1/projects", ok)

	do := func(method, tenant string) int {
		req := httptest.NewRequest(method, "/api/v1/projects", nil)
		req.Header.Set("X-Tenant", tenant)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec.Code
	}
	for _, c := range []struct {
		method, tenant string
		want           int
	}{
		{"GET", "t-suspended", 200}, {"POST", "t-suspended", 403}, {"POST", "t-other", 200}, {"GET", "t-other", 200},
	} {
		if got := do(c.method, c.tenant); got != c.want {
			t.Errorf("%s as %s = %d, want %d", c.method, c.tenant, got, c.want)
		}
	}
}
