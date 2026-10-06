package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
	"github.com/quixgit/greenops-reliabilix/backend/internal/projects/application"
	"github.com/quixgit/greenops-reliabilix/backend/internal/projects/domain"
)

type memRepo struct {
	domain.Repository // unimplemented methods panic: the test only exercises list/create/get
	mu                sync.Mutex
	m                 map[string][]domain.Project
}

func (r *memRepo) List(_ context.Context, t string) ([]domain.Project, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]domain.Project{}, r.m[t]...), nil
}
func (r *memRepo) Create(_ context.Context, p domain.Project) (domain.Project, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range r.m[p.TenantID] {
		if e.Name == p.Name {
			return p, domain.ErrDuplicate
		}
	}
	p.ID = "id-" + p.Name
	r.m[p.TenantID] = append(r.m[p.TenantID], p)
	return p, nil
}

func (r *memRepo) Get(_ context.Context, t, id string) (domain.Project, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range r.m[t] {
		if p.ID == id {
			return p, nil
		}
	}
	return domain.Project{}, domain.ErrNotFound
}

func srv() http.Handler {
	r := chi.NewRouter()
	r.Use(auth.Authenticate(auth.DevVerifier{}, nil))
	Handlers{Svc: application.Service{Repo: &memRepo{m: map[string][]domain.Project{}}}}.Routes(r)
	return r
}

func do(h http.Handler, method, token, body string) *httptest.ResponseRecorder {
	return doPath(h, method, "/projects", token, body)
}

func doPath(h http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(context.Background(), method, path, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestTenantIsolationAndRBAC(t *testing.T) {
	h := srv()
	if c := do(h, "POST", "dev:u1:A:admin", `{"name":"api","functional_unit":"request"}`).Code; c != 201 {
		t.Fatalf("create = %d", c)
	}
	if c := do(h, "POST", "dev:u2:A:viewer", `{"name":"x","functional_unit":"r"}`).Code; c != 403 {
		t.Fatalf("viewer create = %d, want 403", c)
	}
	if c := do(h, "GET", "", "").Code; c != 401 {
		t.Fatalf("anonymous = %d, want 401", c)
	}
	if b := do(h, "GET", "dev:u3:B:admin", "").Body.String(); strings.Contains(b, "api") {
		t.Fatalf("tenant B sees tenant A data: %s", b)
	}
	if b := do(h, "GET", "dev:u2:A:viewer", "").Body.String(); !strings.Contains(b, "api") {
		t.Fatalf("tenant A cannot see own data: %s", b)
	}
	if c := do(h, "POST", "dev:u1:A:admin", `{"name":"api","functional_unit":"request"}`).Code; c != 409 {
		t.Fatalf("duplicate = %d, want 409", c)
	}
	// Cross-tenant read by id is a 404, never the other tenant's project.
	id := "id-api"
	if c := doPath(h, "GET", "/projects/"+id, "dev:u3:B:admin", "").Code; c != 404 && c != 400 {
		t.Fatalf("cross-tenant get = %d, want 404", c)
	}
	if c := doPath(h, "GET", "/projects/not-a-uuid", "dev:u1:A:admin", "").Code; c != 404 {
		t.Fatalf("malformed id = %d, want 404", c)
	}
}
