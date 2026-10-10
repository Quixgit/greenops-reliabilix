//go:build integration

package tests

import (
	"context"
	"crypto/sha1" //nolint:gosec // LiqPay's own signature scheme
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/app"
)

// A payment provider's callback turns into a plan: verified, applied once, visible to the organization with its limits,
// and returned to Free when the period ends.
func TestPaymentCallbackAppliesAPlanOnceAndThePeriodExpires(t *testing.T) {
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
	alice := fmt.Sprintf("dev:bill-%s:-:-:bill-%s@example.com", rid, rid)
	opSub := "opbill-" + rid
	opTok := fmt.Sprintf("dev:%s:-:-:opbill-%s@example.com", opSub, rid)
	tenant := str(c.must("POST", "/api/v1/onboarding", alice, `{"organization_name":"Billing Test Org"}`, 201), "tenant_id")
	if _, err := owner.Exec(ctx, `INSERT INTO platform.operators (auth_sub, email, role) VALUES ($1,$2,'admin')`, opSub, "opbill-"+rid+"@example.com"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = owner.Exec(ctx, `DELETE FROM platform.operator_access_log WHERE operator_sub = $1`, opSub)
		_, _ = owner.Exec(ctx, `DELETE FROM platform.operators WHERE auth_sub = $1`, opSub)
		_, _ = owner.Exec(ctx, `DELETE FROM platform.integration_values WHERE integration = 'liqpay'`)
	})

	// the provider is not set up: nothing is offered and its callback is refused
	if p := c.must("GET", "/api/v1/plan", alice, "", 200); len(p["providers"].([]any)) != 0 {
		t.Fatalf("providers before setup = %v", p["providers"])
	}
	resp, err := http.Post(srv.URL+"/api/v1/billing/webhooks/liqpay", "application/x-www-form-urlencoded", strings.NewReader("data=x&signature=y"))
	if err != nil || resp.StatusCode != 503 {
		t.Fatalf("callback of a provider that is not set up = %v %v", resp, err)
	}

	// an operator sets the provider up in the console
	c.must("PUT", "/api/v1/operator/integrations/liqpay", opTok, `{"values":{"public_key":"sandbox_i123","private_key":"private-secret-key","amount_team":"1999"}}`, 200)
	deadline := time.Now().Add(15 * time.Second)
	for {
		p := c.must("GET", "/api/v1/plan", alice, "", 200)
		if len(p["providers"].([]any)) == 1 || time.Now().After(deadline) {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	pv := c.must("GET", "/api/v1/plan", alice, "", 200)
	provs := pv["providers"].([]any)
	if len(provs) != 1 || provs[0].(map[string]any)["id"] != "liqpay" {
		t.Fatalf("providers = %v", provs)
	}
	co := c.must("POST", "/api/v1/plan/checkout", alice, `{"provider":"liqpay","plan":"team"}`, 200)
	if !strings.HasPrefix(str(co, "url"), "https://www.liqpay.ua/api/3/checkout?") {
		t.Fatalf("checkout = %v", co)
	}
	c.must("POST", "/api/v1/plan/checkout", alice, `{"provider":"liqpay","plan":"business"}`, 422) // no amount set for it
	c.must("POST", "/api/v1/plan/checkout", alice, `{"provider":"liqpay","plan":"free"}`, 422)

	paymentID := time.Now().UnixNano() // a provider's payment id is unique; so is it per run
	callback := func(status, sig string) int {
		data := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(`{"status":%q,"order_id":"%s:team:abc","payment_id":%d}`, status, tenant, paymentID)))
		if sig == "" {
			h := sha1.Sum([]byte("private-secret-key" + data + "private-secret-key")) //nolint:gosec
			sig = base64.StdEncoding.EncodeToString(h[:])
		}
		r, err := http.Post(srv.URL+"/api/v1/billing/webhooks/liqpay", "application/x-www-form-urlencoded", strings.NewReader(url.Values{"data": {data}, "signature": {sig}}.Encode()))
		if err != nil {
			t.Fatal(err)
		}
		_ = r.Body.Close()
		return r.StatusCode
	}
	if code := callback("success", "forged"); code != 400 {
		t.Errorf("forged callback = %d, want 400", code)
	}
	if code := callback("success", ""); code != 200 {
		t.Fatalf("callback = %d", code)
	}
	if code := callback("success", ""); code != 200 { // delivered again
		t.Errorf("repeated callback = %d", code)
	}
	var plan, status string
	var n int
	if err := owner.QueryRow(ctx, `SELECT plan, status FROM tenants.subscriptions WHERE organization_id = $1`, tenant).Scan(&plan, &status); err != nil || plan != "team" || status != "active" {
		t.Fatalf("subscription = %q %q %v", plan, status, err)
	}
	if err := owner.QueryRow(ctx, `SELECT count(*) FROM platform.billing_events WHERE provider = 'liqpay' AND organization_id = $1`, tenant).Scan(&n); err != nil || n != 1 {
		t.Errorf("a repeated delivery must be applied once, events = %d %v", n, err)
	}
	var projects string
	if err := owner.QueryRow(ctx, `SELECT value::text FROM settings.tenant_values WHERE tenant_id = $1 AND key = 'limits.projects'`, tenant).Scan(&projects); err != nil || projects != "10" {
		t.Errorf("team plan must write its limits: %q %v", projects, err)
	}
	evs := items(c.must("GET", "/api/v1/operator/billing/events", opTok, "", 200))
	if len(evs) == 0 {
		t.Error("the operator must see the payment event")
	}
	sub := c.must("GET", "/api/v1/operator/tenants/"+tenant+"/subscription", opTok, "", 200)
	if sub["status"] != "active" || sub["plan"] != "team" {
		t.Errorf("operator view of the subscription = %v", sub)
	}

	// the period ends: the scheduler's function returns the organization to Free
	if _, err := owner.Exec(ctx, `UPDATE tenants.subscriptions SET period_end = now() - interval '1 day' WHERE organization_id = $1`, tenant); err != nil {
		t.Fatal(err)
	}
	var changed int
	if err := workerPool.QueryRow(ctx, `SELECT platform.expire_subscriptions('{"limits.projects":3,"modules.kubernetes":false}'::jsonb)`).Scan(&changed); err != nil || changed < 1 {
		t.Fatalf("expire = %d %v", changed, err)
	}
	if err := owner.QueryRow(ctx, `SELECT plan, status FROM tenants.subscriptions WHERE organization_id = $1`, tenant).Scan(&plan, &status); err != nil || plan != "free" || status != "canceled" {
		t.Errorf("after expiry = %q %q %v", plan, status, err)
	}
	if err := owner.QueryRow(ctx, `SELECT value::text FROM settings.tenant_values WHERE tenant_id = $1 AND key = 'limits.projects'`, tenant).Scan(&projects); err != nil || projects != "3" {
		t.Errorf("free limits after expiry: %q %v", projects, err)
	}
}
