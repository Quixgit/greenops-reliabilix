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
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/database"
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
		_, _ = owner.Exec(ctx, `DELETE FROM platform.operators WHERE auth_sub IN ($1,$2) OR email = $3`, viewerSub, adminSub, "alice-"+rid+"@example.com")
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
	// The operators' own log is readable and shows the reads above; the registry never invents counts.
	opLog := items(c.must("GET", "/api/v1/operator/audit?limit=50", viewerOp, "", 200))
	if first, _ := opLog[0].(map[string]any); len(opLog) == 0 || str(first, "operator_sub") == "" {
		t.Errorf("operator audit = %v", opLog)
	}
	srcs := items(c.must("GET", "/api/v1/operator/data-sources", viewerOp, "", 200))
	if len(srcs) < 5 {
		t.Errorf("data sources = %v", srcs)
	}
	for _, v := range srcs {
		sr, _ := v.(map[string]any)
		if sr["state"] == "planned" && sr["connections"] != nil {
			t.Errorf("a planned source must not carry counts: %v", sr)
		}
	}
	ops := items(c.must("GET", "/api/v1/operator/operators", viewerOp, "", 200))
	if len(ops) < 2 {
		t.Errorf("operators = %v", ops)
	}
	if code, _, _ := c.do("GET", "/api/v1/operator/jobs", viewerOp, ""); code != 200 && code != 503 {
		t.Errorf("jobs = %d, want 200 (queue reachable) or 503", code)
	}
	if pl := items(c.must("GET", "/api/v1/operator/plans", viewerOp, "", 200)); len(pl) < 4 {
		t.Errorf("plans = %v", pl)
	}
	// a plan is a preset of limits and modules, assigned with a reason, and it is in the tenant's own audit log
	if code, _, _ := c.do("PUT", "/api/v1/operator/tenants/"+tenant+"/plan", viewerOp, `{"plan":"team","reason":"viewer must not"}`); code != 403 {
		t.Errorf("viewer assigning a plan = %d, want 403", code)
	}
	if code, _, _ := c.do("PUT", "/api/v1/operator/tenants/"+tenant+"/plan", adminOp, `{"plan":"team","reason":""}`); code != 422 {
		t.Errorf("plan without a reason = %d, want 422", code)
	}
	c.must("PUT", "/api/v1/operator/tenants/"+tenant+"/plan", adminOp, `{"plan":"free","reason":"integration test"}`, 200)
	var gotPlan string
	if err := owner.QueryRow(ctx, `SELECT plan FROM tenants.organizations WHERE id = $1`, tenant).Scan(&gotPlan); err != nil || gotPlan != "free" {
		t.Errorf("plan = %q, %v", gotPlan, err)
	}
	var reason string
	if err := owner.QueryRow(ctx, `SELECT metadata->>'reason' FROM audit.audit_logs WHERE tenant_id = $1 AND action = 'operator.plan_changed'`, tenant).Scan(&reason); err != nil || reason != "integration test" {
		t.Errorf("plan audit reason = %q, %v", reason, err)
	}
	if code, _, _ := c.do("POST", "/api/v1/projects", alice, `{"name":"p1","functional_unit":"request"}`); code != 201 {
		t.Errorf("a first project under the free plan = %d, want 201", code)
	}
	// roles: assign and withdraw from the console, with the guard rails (own role, last admin, unknown person)
	aliceMail := "alice-" + rid + "@example.com"
	if code, _, _ := c.do("PUT", "/api/v1/operator/operators", viewerOp, `{"email":"`+aliceMail+`","role":"viewer"}`); code != 403 {
		t.Errorf("viewer assigning an operator = %d, want 403", code)
	}
	if code, _, _ := c.do("PUT", "/api/v1/operator/operators", adminOp, `{"email":"nobody-`+rid+`@example.com","role":"viewer"}`); code != 409 {
		t.Errorf("a person who never signed in = %d, want 409", code)
	}
	if code, _, _ := c.do("PUT", "/api/v1/operator/operators", adminOp, `{"email":"a@example.com","role":"viewer"}`); code != 409 {
		t.Errorf("changing your own role = %d, want 409", code)
	}
	c.must("PUT", "/api/v1/operator/operators", adminOp, `{"email":"`+aliceMail+`","role":"viewer"}`, 200)
	var aliceSub, aliceRole string
	if err := owner.QueryRow(ctx, `SELECT auth_sub, role FROM platform.operators WHERE email = $1`, aliceMail).Scan(&aliceSub, &aliceRole); err != nil || aliceRole != "viewer" {
		t.Fatalf("alice operator row = %q %q %v", aliceSub, aliceRole, err)
	}
	c.must("PUT", "/api/v1/operator/operators", adminOp, `{"email":"`+aliceMail+`","role":"admin"}`, 200)
	if code, _, _ := c.do("POST", "/api/v1/operator/operators/withdraw", adminOp, `{"subject":"`+adminSub+`"}`); code != 409 {
		t.Errorf("withdrawing yourself = %d, want 409", code)
	}
	c.must("POST", "/api/v1/operator/operators/withdraw", adminOp, `{"subject":"`+aliceSub+`"}`, 200)
	if code, _, _ := c.do("POST", "/api/v1/operator/operators/withdraw", adminOp, `{"subject":"`+aliceSub+`"}`); code != 404 {
		t.Errorf("withdrawing twice = %d, want 404", code)
	}
	sys := c.must("GET", "/api/v1/operator/system", viewerOp, "", 200)
	if sys["expected_migration"] != float64(database.ExpectedMigration) || len(sys["integrations"].([]any)) < 8 {
		t.Fatalf("system = %v", sys)
	}
	statuses := map[string]string{}
	for _, i := range sys["integrations"].([]any) {
		m := i.(map[string]any)
		statuses[str(m, "name")] = str(m, "status")
	}
	if statuses["E-mail (SMTP)"] != "stub" || statuses["Payments (Stripe, Paddle, LiqPay, WayForPay)"] != "missing" || statuses["Electricity Maps (grid carbon)"] != "stub" {
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
	if err := owner.QueryRow(ctx, `SELECT count(*) FROM platform.operator_access_log WHERE operator_sub = $1`, viewerSub).Scan(&n); err != nil || n != 12 { // the 12 successful reads (audit, data sources, operators, jobs, plans included); refused writes never reach the log
		t.Errorf("viewer access log rows = %d, %v", n, err)
	}
	if _, err := apiPool.Exec(ctx, `DELETE FROM platform.operator_access_log`); err == nil {
		t.Error("the access log must be unreachable for the API role")
	}
}

// A suspended organization is left out of the scheduler fan-out and comes back when access is restored.
func TestSuspendedOrganizationIsSkippedByJobFanOut(t *testing.T) {
	ctx := context.Background()
	owner := pool(t, "TEST_OWNER_DSN")
	var tenant string
	if err := owner.QueryRow(ctx, `INSERT INTO tenants.organizations (name) VALUES ('Fanout Suspend Test') RETURNING id::text`).Scan(&tenant); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = owner.Exec(ctx, `DELETE FROM projects.projects WHERE tenant_id = $1`, tenant)
		_, _ = owner.Exec(ctx, `DELETE FROM settings.tenant_values WHERE tenant_id = $1`, tenant)
		_, _ = owner.Exec(ctx, `DELETE FROM tenants.organizations WHERE id = $1`, tenant)
	})
	if _, err := owner.Exec(ctx, `INSERT INTO projects.projects (tenant_id, name, functional_unit) VALUES ($1, 'p', 'request')`, tenant); err != nil {
		t.Fatal(err)
	}
	count := func() (n int) {
		if err := owner.QueryRow(ctx, `SELECT count(*) FROM projects.list_projects_for_jobs() WHERE tenant_id = $1`, tenant).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return
	}
	if count() != 1 {
		t.Fatal("an active organization must be in the fan-out")
	}
	if _, err := owner.Exec(ctx, `INSERT INTO settings.tenant_values (tenant_id, key, value, source, updated_by) VALUES ($1, 'organization.suspended', 'true', 'operator', 'test')`, tenant); err != nil {
		t.Fatal(err)
	}
	if count() != 0 {
		t.Error("a suspended organization must not be in the fan-out")
	}
	if _, err := owner.Exec(ctx, `UPDATE settings.tenant_values SET value = 'false' WHERE tenant_id = $1`, tenant); err != nil {
		t.Fatal(err)
	}
	if count() != 1 {
		t.Error("a restored organization must be back in the fan-out")
	}
}
