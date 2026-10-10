//go:build integration

package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/app"
	"github.com/quixgit/greenops-reliabilix/backend/internal/notifications/application"
	"github.com/quixgit/greenops-reliabilix/backend/internal/notifications/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/notifications/repository"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/secretbox"
)

// The operator console over HTTP and over the real database functions: who may enter, what they see (metadata only),
// what they can change, and that every change shows up in the affected tenant's own audit log.
func TestOperatorConsole(t *testing.T) {
	ctx := context.Background()
	owner, apiPool, workerPool := pool(t, "TEST_OWNER_DSN"), pool(t, "TEST_API_DSN"), pool(t, "TEST_WORKER_DSN")

	cfg := devConfig(t.TempDir())
	apiC, err := app.Build(ctx, cfg, quietLog(), apiPool, &taskRec{})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(app.NewRouter(cfg, quietLog(), apiC.Verifier, apiC.Resolver, apiPool.Ping, apiC.Modules()...))
	defer srv.Close()
	c := api{t: t, srv: srv}

	rid := fmt.Sprint(time.Now().UnixNano())
	person := func(name string) string { return fmt.Sprintf("dev:%s-%s:-:-:%s-%s@example.com", name, rid, name, rid) }
	alice, viewerOp, adminOp, outsider := person("alice"), person("opview"), person("opadmin"), person("nobody")
	viewerSub, adminSub := "opview-"+rid, "opadmin-"+rid
	t.Cleanup(func() {
		_, _ = owner.Exec(ctx, `DELETE FROM platform.operator_access_log WHERE operator_sub IN ($1,$2)`, viewerSub, adminSub)
		_, _ = owner.Exec(ctx, `DELETE FROM platform.operators WHERE auth_sub IN ($1,$2)`, viewerSub, adminSub)
	})

	// A tenant with a webhook destination whose delivery failed.
	tenant := str(c.must("POST", "/api/v1/onboarding", alice, `{"organization_name":"Operator Test Org"}`, 201), "tenant_id")
	actor := auth.Claims{Subject: "auth0|a", TenantID: tenant, Role: auth.RoleOwner}
	box, err := secretbox.New([]byte(strings.Repeat("k", 32)))
	if err != nil {
		t.Fatal(err)
	}
	svc := application.Service{Repo: repository.Postgres{Pool: apiPool}, Channels: domain.NewRegistry(&recordingChannel{kind: domain.KindWebhook}), Box: box, Queue: &taskRec{}}
	dest, err := svc.CreateDestination(auth.WithClaims(ctx, actor), tenant, actor, application.CreateInput{Kind: domain.KindWebhook, Name: "SECRET-NAME-OK", Target: "https://hooks.example.com/SECRET-PATH-TOKEN"})
	if err != nil {
		t.Fatal(err)
	}
	delID, err := svc.Test(ctx, tenant, dest.Destination.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Exec(ctx, `UPDATE notifications.deliveries SET status='failed', attempts=8, last_error='endpoint answered HTTP 503' WHERE id=$1`, delID); err != nil {
		t.Fatal(err)
	}

	// ---- nobody is an operator until the database says so ----
	for name, tok := range map[string]string{"tenant owner": alice, "stranger": outsider, "no token": ""} {
		want := 403
		if tok == "" {
			want = 401
		}
		for _, p := range []string{"/api/v1/operator/me", "/api/v1/operator/tenants", "/api/v1/operator/system", "/api/v1/operator/notifications/deliveries"} {
			if code, _, _ := c.do("GET", p, tok, ""); code != want {
				t.Errorf("%s GET %s = %d, want %d", name, p, code, want)
			}
		}
	}
	ci := str(c.must("POST", "/api/v1/api-keys", alice, `{"name":"ci","role":"ci"}`, 201), "key")
	if code, _, _ := c.do("GET", "/api/v1/operator/me", ci, ""); code != 401 {
		t.Errorf("an API key must not reach the operator console, got %d", code)
	}
	// the application roles cannot appoint operators
	if _, err := apiPool.Exec(ctx, `INSERT INTO platform.operators (auth_sub, role) VALUES ('hacker','admin')`); err == nil {
		t.Fatal("the API role must not be able to add operators")
	}
	if _, err := workerPool.Exec(ctx, `SELECT platform.operator_tenants(10)`); err == nil {
		t.Fatal("the worker role must not call operator functions")
	}

	// ---- appoint operators (what `make operator-add` does, as the owner) ----
	if _, err := owner.Exec(ctx, `INSERT INTO platform.operators (auth_sub, email, role) VALUES ($1,'v@example.com','viewer'), ($2,'a@example.com','admin')`, viewerSub, adminSub); err != nil {
		t.Fatal(err)
	}
	if me := c.must("GET", "/api/v1/operator/me", viewerOp, "", 200); str(me, "role") != "viewer" || str(me, "subject") != viewerSub {
		t.Fatalf("me = %v", me)
	}

	// ---- what an operator sees: metadata, never secrets ----
	var found map[string]any
	for _, it := range items(c.must("GET", "/api/v1/operator/tenants", viewerOp, "", 200)) {
		if m := it.(map[string]any); str(m, "id") == tenant {
			found = m
		}
	}
	if found == nil || found["destinations"] != float64(1) || found["failed_deliveries_7d"] != float64(1) || found["members"] != float64(1) {
		t.Fatalf("tenant row = %v", found)
	}
	code, _, raw := c.do("GET", "/api/v1/operator/notifications/deliveries?status=failed&limit=200", viewerOp, "")
	if code != 200 {
		t.Fatalf("deliveries = %d %s", code, raw)
	}
	for _, secret := range []string{"SECRET-PATH-TOKEN", "hooks.example.com", "target_sealed", "payload", "v1:"} {
		if strings.Contains(string(raw), secret) {
			t.Errorf("operator deliveries leak %q", secret)
		}
	}
	var del map[string]any
	var list struct{ Items []map[string]any }
	_ = json.Unmarshal(raw, &list)
	for _, d := range list.Items {
		if str(d, "id") == delID {
			del = d
		}
	}
	if del == nil || str(del, "tenant_name") != "Operator Test Org" || str(del, "last_error") != "endpoint answered HTTP 503" || del["attempts"] != float64(8) {
		t.Fatalf("delivery row = %v", del)
	}
	if code, _, _ := c.do("GET", "/api/v1/operator/notifications/deliveries?status=nope", viewerOp, ""); code != 400 {
		t.Errorf("bad status filter = %d", code)
	}
	if stats := items(c.must("GET", "/api/v1/operator/notifications/stats?hours=24", viewerOp, "", 200)); len(stats) == 0 {
		t.Error("stats must include the failed delivery")
	}
	c.must("GET", "/api/v1/operator/connections/unhealthy", viewerOp, "", 200)
	sys := c.must("GET", "/api/v1/operator/system", viewerOp, "", 200)
	if sys["expected_migration"] != float64(18) || len(sys["integrations"].([]any)) < 8 {
		t.Fatalf("system = %v", sys)
	}
	statuses := map[string]string{}
	for _, i := range sys["integrations"].([]any) {
		m := i.(map[string]any)
		statuses[str(m, "name")] = str(m, "status")
	}
	if statuses["E-mail (SMTP)"] != "stub" || statuses["Stripe (billing)"] != "missing" || statuses["Electricity Maps (grid carbon)"] != "stub" {
		t.Errorf("stubs must be labelled as such: %v", statuses)
	}

	// ---- a viewer cannot act; an admin can, and the tenant sees it ----
	retry := "/api/v1/operator/tenants/" + tenant + "/deliveries/" + delID + "/retry"
	c.must("POST", retry, viewerOp, "", 403)
	c.must("POST", "/api/v1/operator/tenants/"+tenant+"/destinations/"+dest.Destination.ID+"/disable", viewerOp, "", 403)
	var status string
	_ = owner.QueryRow(ctx, `SELECT status FROM notifications.deliveries WHERE id=$1`, delID).Scan(&status)
	if status != "failed" {
		t.Fatalf("a read-only operator changed the delivery: %s", status)
	}
	c.must("POST", retry, adminOp, "", 202)
	var attempts int
	_ = owner.QueryRow(ctx, `SELECT status, attempts FROM notifications.deliveries WHERE id=$1`, delID).Scan(&status, &attempts)
	if status != "pending" || attempts != 0 {
		t.Errorf("after retry: %s / %d", status, attempts)
	}
	c.must("POST", retry, adminOp, "", 404) // no longer failed
	other := org(t, owner, "Other "+rid)
	c.must("POST", "/api/v1/operator/tenants/"+other+"/deliveries/"+delID+"/retry", adminOp, "", 404) // wrong tenant
	c.must("POST", "/api/v1/operator/tenants/"+tenant+"/destinations/"+dest.Destination.ID+"/disable", adminOp, "", 200)
	var enabled bool
	_ = owner.QueryRow(ctx, `SELECT enabled FROM notifications.destinations WHERE id=$1`, dest.Destination.ID).Scan(&enabled)
	if enabled {
		t.Error("the destination should be disabled")
	}
	c.must("POST", "/api/v1/operator/tenants/"+tenant+"/destinations/"+dest.Destination.ID+"/enable", adminOp, "", 200)

	// the tenant's own audit log shows what the operator did, attributed to the operator
	audit := items(c.must("GET", "/api/v1/audit-log?limit=100", alice, "", 200))
	seen := map[string]bool{}
	for _, a := range audit {
		m := a.(map[string]any)
		if strings.HasPrefix(str(m, "actor_user_id"), "operator:"+adminSub) {
			seen[str(m, "action")] = true
		}
	}
	for _, want := range []string{"operator.delivery_retried", "operator.destination_disabled", "operator.destination_enabled"} {
		if !seen[want] {
			t.Errorf("tenant audit log lacks %s (got %v)", want, seen)
		}
	}

	// every request, reads included, is in the operator access log
	var n int
	if err := owner.QueryRow(ctx, `SELECT count(*) FROM platform.operator_access_log WHERE operator_sub = $1`, viewerSub).Scan(&n); err != nil || n != 7 { // the 7 successful reads; refused writes never reach the log
		t.Errorf("viewer access log rows = %d, %v", n, err)
	}
	if _, err := apiPool.Exec(ctx, `DELETE FROM platform.operator_access_log`); err == nil {
		t.Error("the access log must be unreachable for the API role")
	}
}
