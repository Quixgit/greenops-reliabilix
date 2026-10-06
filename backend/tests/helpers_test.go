//go:build integration

// Integration tests run against a real PostgreSQL with all migrations applied:
//
//	TEST_OWNER_DSN   migration/owner role (creates fixtures)
//	TEST_API_DSN     greenops_api role
//	TEST_WORKER_DSN  greenops_worker role
//	TEST_REDIS_ADDR  optional: enables the real-Asynq test
package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/config"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/database"
)

func pool(t *testing.T, env string) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv(env)
	if dsn == "" {
		t.Skipf("%s not set", env)
	}
	p, err := database.Connect(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	return p
}

func org(t *testing.T, owner *pgxpool.Pool, name string) string {
	t.Helper()
	var id string
	if err := owner.QueryRow(context.Background(), `INSERT INTO tenants.organizations (name) VALUES ($1) RETURNING id`, name).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func quietLog() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// taskRec records enqueued jobs instead of sending them to Redis, so tests can drive handlers deterministically.
type taskRec struct {
	mu    sync.Mutex
	tasks []queued
}

type queued struct {
	Type    string
	Payload []byte
}

func (r *taskRec) Enqueue(_ context.Context, typ string, p any, _ ...asynq.Option) error {
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tasks = append(r.tasks, queued{typ, b})
	return nil
}

func (r *taskRec) pop() (queued, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.tasks) == 0 {
		return queued{}, false
	}
	q := r.tasks[0]
	r.tasks = r.tasks[1:]
	return q, true
}

func (r *taskRec) types() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, 0, len(r.tasks))
	for _, q := range r.tasks {
		out = append(out, q.Type)
	}
	return out
}

// pump feeds recorded jobs into the worker's handlers until no job is left (jobs may enqueue further jobs).
func pump(t *testing.T, mux *asynq.ServeMux, recs ...*taskRec) []string {
	t.Helper()
	var ran []string
	for i := 0; i < 200; i++ {
		var q queued
		found := false
		for _, r := range recs {
			if q, found = r.pop(); found {
				break
			}
		}
		if !found {
			return ran
		}
		ran = append(ran, q.Type)
		if err := mux.ProcessTask(context.Background(), asynq.NewTask(q.Type, q.Payload)); err != nil {
			t.Fatalf("job %s failed: %v", q.Type, err)
		}
	}
	t.Fatal("job loop did not terminate")
	return ran
}

func devConfig(storageDir string) config.Config {
	return config.Config{Env: "dev", RatePerSec: 1e6, RateBurst: 1_000_000, LocalStorageDir: storageDir, SyncBackfillDays: 10, MetricsAddr: "127.0.0.1:0"}
}

// api is a tiny JSON HTTP client for the router under test.
type api struct {
	t   *testing.T
	srv *httptest.Server
}

func (a api) do(method, path, token, body string) (int, map[string]any, []byte) {
	a.t.Helper()
	var rd io.Reader
	if body != "" {
		rd = bytes.NewBufferString(body)
	}
	req, err := http.NewRequestWithContext(context.Background(), method, a.srv.URL+path, rd)
	if err != nil {
		a.t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := a.srv.Client().Do(req)
	if err != nil {
		a.t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	return resp.StatusCode, m, raw
}

func (a api) must(method, path, token, body string, want int) map[string]any {
	a.t.Helper()
	code, m, raw := a.do(method, path, token, body)
	if code != want {
		a.t.Fatalf("%s %s = %d, want %d: %s", method, path, code, want, raw)
	}
	return m
}

func items(m map[string]any) []any {
	v, _ := m["items"].([]any)
	return v
}

func str(m map[string]any, k string) string { s, _ := m[k].(string); return s }

func sub(m map[string]any, k string) map[string]any { v, _ := m[k].(map[string]any); return v }
