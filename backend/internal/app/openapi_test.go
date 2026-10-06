package app

import (
	"context"
	"net/http"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"gopkg.in/yaml.v3"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/config"
)

func registeredRoutes(t *testing.T) map[string]bool {
	t.Helper()
	cfg := config.Config{Env: "dev", RatePerSec: 10, RateBurst: 10, LocalStorageDir: t.TempDir()}
	c, err := Build(context.Background(), cfg, nil2(), nil, &noopQueue{}) // no database needed to enumerate routes
	if err != nil {
		t.Fatal(err)
	}
	h := NewRouter(cfg, nil2(), c.Verifier, c.Resolver, func(context.Context) error { return nil }, c.Modules()...)
	routes := map[string]bool{}
	err = chi.Walk(h.(chi.Routes), func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		routes[strings.ToUpper(method)+" "+strings.TrimSuffix(route, "/")] = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return routes
}

func specOperations(t *testing.T) map[string]bool {
	t.Helper()
	raw, err := os.ReadFile("../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	ops := map[string]bool{}
	for path, item := range doc.Paths {
		for method := range item {
			switch m := strings.ToUpper(method); m {
			case "GET", "POST", "PUT", "PATCH", "DELETE":
				ops[m+" "+path] = true
			}
		}
	}
	return ops
}

// The API contract must describe exactly the routes the server registers: a new endpoint without a spec
// entry (or a stale spec entry) fails the build, so the generated TypeScript client can be trusted.
func TestOpenAPIMatchesRoutes(t *testing.T) {
	routes, spec := registeredRoutes(t), specOperations(t)
	var missing, stale []string
	for r := range routes {
		if !spec[r] {
			missing = append(missing, r)
		}
	}
	for s := range spec {
		if !routes[s] {
			stale = append(stale, s)
		}
	}
	sort.Strings(missing)
	sort.Strings(stale)
	if len(missing) > 0 {
		t.Errorf("routes missing from backend/api/openapi.yaml:\n  %s", strings.Join(missing, "\n  "))
	}
	if len(stale) > 0 {
		t.Errorf("openapi.yaml documents routes that do not exist:\n  %s", strings.Join(stale, "\n  "))
	}
}
