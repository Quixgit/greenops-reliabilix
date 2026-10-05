package transport

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/quixgit/greenops-reliabilix/pkg/auth"
	"github.com/quixgit/greenops-reliabilix/services/project/internal/infrastructure"
)

func server() http.Handler {
	mux := http.NewServeMux()
	Handler{Repo: infrastructure.NewMemoryRepo()}.Register(mux)
	return auth.Authenticate(auth.DevVerifier{})(mux)
}

func do(h http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestTenantIsolationAndRBAC(t *testing.T) {
	h := server()
	adminA := "dev:u1:tenantA:admin"
	viewerA := "dev:u2:tenantA:viewer"
	adminB := "dev:u3:tenantB:admin"

	if c := do(h, "POST", "/v1/projects", adminA, `{"name":"api"}`).Code; c != http.StatusCreated {
		t.Fatalf("create = %d", c)
	}
	if c := do(h, "POST", "/v1/projects", viewerA, `{"name":"x"}`).Code; c != http.StatusForbidden {
		t.Fatalf("viewer create = %d, want 403", c)
	}
	if c := do(h, "GET", "/v1/projects", "", "").Code; c != http.StatusUnauthorized {
		t.Fatalf("anonymous = %d, want 401", c)
	}
	if body := do(h, "GET", "/v1/projects", adminB, "").Body.String(); strings.Contains(body, "api") {
		t.Fatalf("tenant B sees tenant A data: %s", body)
	}
	if body := do(h, "GET", "/v1/projects", viewerA, "").Body.String(); !strings.Contains(body, "api") {
		t.Fatalf("tenant A cannot see own data: %s", body)
	}
}
