//go:build integration

package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/quixgit/greenops-reliabilix/backend/internal/app"
)

// The language configuration is public, and a problem comes back in the language asked for with a stable code.
func TestLanguagesArePublicAndProblemsAreTranslated(t *testing.T) {
	ctx := context.Background()
	apiPool := pool(t, "TEST_API_DSN")
	cfg := devConfig(t.TempDir())
	apiC, err := app.Build(ctx, cfg, quietLog(), apiPool, &taskRec{})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(app.NewRouter(cfg, quietLog(), apiC.Verifier, apiC.Resolver, apiPool.Ping, apiC.Modules()...))
	defer srv.Close()

	get := func(path, lang string) (int, map[string]any) {
		req, _ := http.NewRequest("GET", srv.URL+path, nil)
		if lang != "" {
			req.Header.Set("Accept-Language", lang)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var body map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&body)
		return resp.StatusCode, body
	}
	code, loc := get("/api/v1/locales", "")
	if code != 200 || loc["default"] != "en" || len(loc["enabled"].([]any)) < 2 {
		t.Errorf("locales = %d %v", code, loc)
	}
	code, p := get("/api/v1/operator/me", "ru-RU,ru;q=0.9")
	if code != 401 || p["code"] != "unauthorized" || p["title"] != "Требуется вход" {
		t.Errorf("russian problem = %d %v", code, p)
	}
	code, p = get("/api/v1/operator/me", "fr-FR")
	if code != 401 || p["code"] != "unauthorized" || p["title"] != "unauthorized" {
		t.Errorf("an unsupported language must get the English title: %d %v", code, p)
	}
}
