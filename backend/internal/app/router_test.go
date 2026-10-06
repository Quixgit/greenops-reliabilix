package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/config"
	"github.com/quixgit/greenops-reliabilix/backend/internal/users"
)

type fake struct{}

func (fake) Name() string        { return "fake" }
func (fake) Routes(r chi.Router) {}

func TestRouter(t *testing.T) {
	h := NewRouter(config.Config{RatePerSec: 1000, RateBurst: 1000}, nil2(), devVerifier(), nil, func(context.Context) error { return nil }, users.New(), fake{})
	get := func(path, token string) *httptest.ResponseRecorder {
		req := httptest.NewRequestWithContext(context.Background(), "GET", path, nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	if c := get("/healthz", "").Code; c != http.StatusOK {
		t.Errorf("healthz = %d", c)
	}
	if c := get("/api/v1/me", "").Code; c != http.StatusUnauthorized {
		t.Errorf("anonymous /me = %d, want 401", c)
	}
	rec := get("/api/v1/me", "dev:alice:t1:viewer")
	if rec.Code != http.StatusOK {
		t.Fatalf("/me = %d", rec.Code)
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" || rec.Header().Get("X-Request-Id") == "" {
		t.Error("security/request-id headers missing")
	}
}
