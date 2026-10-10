//go:build integration

package tests

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/app"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/database"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/errlog"
	httpx "github.com/quixgit/greenops-reliabilix/backend/internal/platform/http"
)

// Errors recorded by the services reach the operator console grouped, sanitized and attributable to a tenant; a
// recurring resolved error reopens, an ignored one stays ignored, and the application roles cannot read the table.
func TestErrorLogEndToEnd(t *testing.T) {
	ctx := context.Background()
	owner, apiPool, workerPool := pool(t, "TEST_OWNER_DSN"), pool(t, "TEST_API_DSN"), pool(t, "TEST_WORKER_DSN")
	rid := fmt.Sprint(time.Now().UnixNano())
	svc := "it" + rid[len(rid)-8:] // unique service name per run: the table is shared
	tenant := org(t, owner, "ErrOrg "+rid)
	opSub := "operr-" + rid
	t.Cleanup(func() {
		_, _ = owner.Exec(ctx, `DELETE FROM platform.error_groups WHERE service = $1`, svc)
		_, _ = owner.Exec(ctx, `DELETE FROM platform.operator_access_log WHERE operator_sub = $1`, opSub)
		_, _ = owner.Exec(ctx, `DELETE FROM platform.operators WHERE auth_sub = $1`, opSub)
	})
	if _, err := owner.Exec(ctx, `INSERT INTO platform.operators (auth_sub, role) VALUES ($1, 'admin')`, opSub); err != nil {
		t.Fatal(err)
	}

	// ---- recording from the worker and the api role, aggregated ----
	w := errlog.NewPG(workerPool, quietLog(), svc)
	defer w.Close()
	for i := 0; i < 5; i++ {
		w.Record(errlog.Event{Component: "cloudaccounts:sync", Severity: errlog.Warning, Message: fmt.Sprintf("account %d denied: https://user:pw@host/p?token=SECRET%d", 100+i, i), TenantID: tenant, RequestID: "req-1"})
	}
	w.Record(errlog.Event{Component: "cloudaccounts:sync", Severity: errlog.Error, Message: "account 7 denied: https://user:pw@host/p?token=SECRET9"})
	w.Record(errlog.Event{Component: "reports:generate", Message: "render failed", TenantID: "not-a-uuid-and-not-a-tenant"})
	w.Flush(ctx)
	a := errlog.NewPG(apiPool, quietLog(), svc)
	defer a.Close()
	a.Record(errlog.Event{Component: "POST /api/v1/reports", Severity: errlog.Critical, Message: "panic: nil map"})
	a.Flush(ctx)

	rows := func(status string) []map[string]any {
		var out []map[string]any
		r, err := apiPool.Query(ctx, `SELECT id::text, component, severity, message, status, occurrences, tenant_name, last_request_id
			FROM platform.operator_errors($1, $2, 100)`, nilIfEmpty(status), svc)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Close()
		for r.Next() {
			var id, comp, sev, msg, st string
			var occ int64
			var tn, rq *string
			if err := r.Scan(&id, &comp, &sev, &msg, &st, &occ, &tn, &rq); err != nil {
				t.Fatal(err)
			}
			out = append(out, map[string]any{"id": id, "component": comp, "severity": sev, "message": msg, "status": st, "occurrences": occ, "tenant": tn, "req": rq})
		}
		return out
	}
	all := rows("")
	if len(all) != 3 {
		t.Fatalf("groups = %d (%v), want 3: sync (numbers masked), reports, api", len(all), all)
	}
	var sync map[string]any
	for _, g := range all {
		if g["component"] == "cloudaccounts:sync" {
			sync = g
		}
		for _, secret := range []string{"pw@", "SECRET", "token="} {
			if strings.Contains(g["message"].(string), secret) {
				t.Errorf("secret %q stored: %v", secret, g["message"])
			}
		}
	}
	if sync == nil || sync["occurrences"] != int64(6) || sync["severity"] != "error" || *(sync["tenant"].(*string)) != "ErrOrg "+rid || *(sync["req"].(*string)) != "req-1" {
		t.Fatalf("sync group = %v", sync)
	}
	for _, g := range all {
		if g["component"] == "reports:generate" && g["tenant"].(*string) != nil {
			t.Errorf("an unknown tenant id must not be attributed: %v", g)
		}
	}

	// ---- operator handling: resolve, recurrence reopens, ignore sticks ----
	syncID := sync["id"].(string)
	var ok bool
	if err := apiPool.QueryRow(ctx, `SELECT platform.operator_error_set_status($1, $2::uuid, 'resolved')`, opSub, syncID).Scan(&ok); err != nil || !ok {
		t.Fatalf("resolve: %v %v", ok, err)
	}
	if n := len(rows("open")); n != 2 {
		t.Errorf("open groups after resolve = %d, want 2", n)
	}
	w.Record(errlog.Event{Component: "cloudaccounts:sync", Severity: errlog.Warning, Message: "account 5 denied: https://user:pw@host/p?token=SECRET5"})
	w.Flush(ctx)
	var status string
	var occ int64
	_ = owner.QueryRow(ctx, `SELECT status, occurrences FROM platform.error_groups WHERE id = $1`, syncID).Scan(&status, &occ)
	if status != "open" || occ != 7 {
		t.Errorf("recurrence of a resolved error: status=%s occurrences=%d, want open/7", status, occ)
	}
	_ = apiPool.QueryRow(ctx, `SELECT platform.operator_error_set_status($1, $2::uuid, 'ignored')`, opSub, syncID).Scan(&ok)
	w.Record(errlog.Event{Component: "cloudaccounts:sync", Message: "account 6 denied: https://user:pw@host/p?token=SECRET6"})
	w.Flush(ctx)
	_ = owner.QueryRow(ctx, `SELECT status FROM platform.error_groups WHERE id = $1`, syncID).Scan(&status)
	if status != "ignored" {
		t.Errorf("an ignored error must stay ignored, got %s", status)
	}
	if err := apiPool.QueryRow(ctx, `SELECT platform.operator_error_set_status('somebody-else', $1::uuid, 'resolved')`, syncID).Scan(&ok); err == nil {
		t.Error("only an admin operator may change error status")
	}

	// ---- privileges ----
	if _, err := apiPool.Exec(ctx, `SELECT * FROM platform.error_groups`); err == nil {
		t.Error("the API role must not read the error table directly")
	}
	if _, err := workerPool.Exec(ctx, `SELECT * FROM platform.operator_errors(NULL, NULL, 10)`); err == nil {
		t.Error("the worker role must not call the operator listing")
	}
	if _, err := apiPool.Exec(ctx, `SELECT platform.purge_old_errors(interval '30 days')`); err == nil {
		t.Error("the API role must not purge")
	}
	if _, err := w.Purge(ctx, time.Hour); err == nil {
		t.Error("purging with under 7 days of retention must be refused")
	}
	if _, err := owner.Exec(ctx, `UPDATE platform.error_groups SET last_seen = now() - interval '200 days' WHERE id = $1`, syncID); err != nil {
		t.Fatal(err)
	}
	if n, err := w.Purge(ctx, 90*24*time.Hour); err != nil || n < 1 {
		t.Errorf("Purge = %d, %v (an old ignored group should go)", n, err)
	}
}

func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// A failing database makes a request fail with 500: the cause must reach the error log through the HTTP hook, grouped
// by route pattern, without leaking into the response.
func TestHTTPFailuresAreRecorded(t *testing.T) {
	ctx := context.Background()
	apiPool := pool(t, "TEST_API_DSN")
	// a second pool that we close: every query through it fails
	broken, err := database.Connect(ctx, os.Getenv("TEST_API_DSN"))
	if err != nil {
		t.Fatal(err)
	}
	sink := &errlog.Sink{}
	old := httpx.ErrorHook
	httpx.ErrorHook = app.HTTPErrorHook(sink, "api")
	t.Cleanup(func() { httpx.ErrorHook = old })

	cfg := devConfig(t.TempDir())
	apiC, err := app.Build(ctx, cfg, quietLog(), broken, &taskRec{})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(app.NewRouter(cfg, quietLog(), apiC.Verifier, apiC.Resolver, apiPool.Ping, apiC.Modules()...))
	defer srv.Close()
	c := api{t: t, srv: srv}
	broken.Close()

	code, body, raw := c.do("GET", "/api/v1/projects", "dev:someone:-:-:someone@example.com", "")
	if code != 500 || str(body, "title") != "internal error" || strings.Contains(string(raw), "closed") || strings.Contains(string(raw), "pool") {
		t.Fatalf("response = %d %s (must be a plain 500 without internals)", code, raw)
	}
	if len(sink.Events) != 1 {
		t.Fatalf("recorded %d events, want 1: %+v", len(sink.Events), sink.Events)
	}
	e := sink.Events[0]
	if e.Service != "api" || e.Component != "GET /api/v1/projects" || e.Severity != errlog.Error || !strings.HasPrefix(e.Message, "HTTP 500: ") || e.RequestID == "" {
		t.Errorf("event = %+v", e)
	}
}
