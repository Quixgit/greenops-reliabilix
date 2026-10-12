package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewRejectsBadEndpointsAndEmptyKey(t *testing.T) {
	for _, e := range []string{"", "api.example.com", "ftp://x.io", "https://x.io/api"} {
		if _, err := New(e, "k"); err == nil {
			t.Errorf("%q accepted", e)
		}
	}
	if _, err := New("https://x.io", ""); err == nil {
		t.Error("empty key accepted")
	}
	if _, err := New("https://x.io/", "k"); err != nil {
		t.Errorf("trailing slash: %v", err)
	}
}

func TestPutPolicyKeepsWhatTerraformDoesNotManage(t *testing.T) {
	var put map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer k" {
			t.Errorf("auth header: %q", r.Header.Get("Authorization"))
		}
		switch r.Method {
		case http.MethodGet:
			_, _ = io.WriteString(w, `{"allowed_regions":["a"],"auto_apply_types":["rightsizing"]}`)
		case http.MethodPut:
			_ = json.NewDecoder(r.Body).Decode(&put)
			_, _ = io.WriteString(w, `{"allowed_regions":["b"],"auto_apply_types":["rightsizing"]}`)
		}
	}))
	defer srv.Close()
	c, _ := New(srv.URL, "k")
	if _, err := c.PutPolicy(context.Background(), "p1", Policy{AllowedRegions: []string{"b"}}); err != nil {
		t.Fatal(err)
	}
	if got := put["auto_apply_types"].([]any); len(got) != 1 || got[0] != "rightsizing" {
		t.Fatalf("automatic application was changed by a write: %v", put)
	}
}

func TestNotFoundAndProblemDetails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/projects/gone" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = io.WriteString(w, `{"title":"invalid request","detail":"name must be 1-120 characters"}`)
	}))
	defer srv.Close()
	c, _ := New(srv.URL, "k")
	if _, err := c.GetProject(context.Background(), "gone"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("404 must be ErrNotFound, got %v", err)
	}
	_, err := c.CreateProject(context.Background(), "", "u")
	var ae *APIError
	if !errors.As(err, &ae) || ae.Status != 422 || ae.Detail == "" {
		t.Fatalf("problem not carried: %v", err)
	}
}

func TestGetBudgetFindsByIDOrNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"items":[{"id":"b1","amount":5,"currency":"USD","period":"monthly","project_id":null}]}`)
	}))
	defer srv.Close()
	c, _ := New(srv.URL, "k")
	if b, err := c.GetBudget(context.Background(), "b1"); err != nil || b.Amount != 5 {
		t.Fatalf("%+v %v", b, err)
	}
	if _, err := c.GetBudget(context.Background(), "b2"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing budget: %v", err)
	}
}
