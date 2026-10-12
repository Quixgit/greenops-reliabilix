//go:build integration

package tests

import (
	"context"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/app"
)

// One cloud at a time (ADR-0065): ?cloud= narrows every read of the request to that cloud, in its own currency, and the automatic reporting
// currency does not depend on which window a query happens to cover.
func TestCloudScopeIsOwnCurrencyAndOwnRows(t *testing.T) {
	ctx := context.Background()
	owner, apiPool := pool(t, "TEST_OWNER_DSN"), pool(t, "TEST_API_DSN")
	cfg := devConfig(t.TempDir())
	apiC, err := app.Build(ctx, cfg, quietLog(), apiPool, &taskRec{})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(app.NewRouter(cfg, quietLog(), apiC.Verifier, apiC.Resolver, apiPool.Ping, apiC.Modules()...))
	defer srv.Close()
	c := api{t: t, srv: srv}

	rid := fmt.Sprint(time.Now().UnixNano())
	alice := fmt.Sprintf("dev:scope-%s:-:-:scope-%s@example.com", rid, rid)
	tenant := str(c.must("POST", "/api/v1/onboarding", alice, `{"organization_name":"Cloud Scope Org"}`, 201), "tenant_id")
	var project string
	if err := owner.QueryRow(ctx, `INSERT INTO projects.projects (tenant_id, name, functional_unit) VALUES ($1,'p','req') RETURNING id::text`, tenant).Scan(&project); err != nil {
		t.Fatal(err)
	}
	// reference rates for the window (the same for every day: 1 EUR = 1.10 USD = 1.50 CAD)
	if _, err := owner.Exec(ctx, `INSERT INTO finops.fx_rates (day, currency, per_eur)
		SELECT d::date, c, v FROM generate_series(current_date - 20, current_date, interval '1 day') d, (VALUES ('USD', 1.10), ('CAD', 1.50)) x(c, v)
		ON CONFLICT (day, currency) DO NOTHING`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { // other tests rely on there being no USD rate in the database
		_, _ = owner.Exec(ctx, `DELETE FROM finops.fx_rates WHERE currency IN ('USD', 'CAD') AND day >= current_date - 20`)
	})
	at := time.Now().UTC().Add(-36 * time.Hour)
	ins := func(provider, service, currency string, cost float64) {
		if _, err := owner.Exec(ctx, `INSERT INTO usage.usage_records (tenant_id, project_id, source, provider, service_name, service_category, region_id, billed_cost, effective_cost, currency, recorded_at, charge_period_end)
			VALUES ($1,$2,'x',$3,$4,'compute','eu-central-1',$5,$5,$6,$7::timestamptz, $7::timestamptz + interval '1 day')`, tenant, project, provider, service, cost, currency, at); err != nil {
			t.Fatal(err)
		}
	}
	ins("aws", "EC2", "USD", 100)
	ins("azure", "VM", "CAD", 200)

	aws := c.must("GET", "/api/v1/finops/summary?cloud=aws", alice, "", 200)
	if str(aws, "currency") != "USD" || aws["total_cost"].(float64) != 100 || len(aws["by_provider"].([]any)) != 1 {
		t.Errorf("aws scope = %v, want USD 100 with one cloud", aws)
	}
	az := c.must("GET", "/api/v1/finops/summary?cloud=azure", alice, "", 200)
	if str(az, "currency") != "CAD" || az["total_cost"].(float64) != 200 {
		t.Errorf("azure scope = %v, want CAD 200", az)
	}
	all := c.must("GET", "/api/v1/finops/summary", alice, "", 200)
	if len(all["by_provider"].([]any)) != 2 {
		t.Errorf("unscoped summary = %v, want both clouds", all)
	}
	// the same currency for a narrow window and a wide one
	narrow := c.must("GET", "/api/v1/finops/summary?from="+at.Format(time.DateOnly)+"&to="+at.Format(time.DateOnly), alice, "", 200)
	if str(narrow, "currency") != str(all, "currency") {
		t.Errorf("reporting currency changed with the window: %v vs %v", str(narrow, "currency"), str(all, "currency"))
	}
	// the other cloud's rows never reach a scoped list of services
	for _, s := range items(c.must("GET", "/api/v1/dashboard/services?cloud=azure", alice, "", 200)) {
		if p := s.(map[string]any)["provider"]; p != "azure" {
			t.Errorf("azure scope lists a %v service", p)
		}
	}
	// a scope on a write is ignored: nothing is partially written
	c.must("POST", "/api/v1/projects?cloud=aws", alice, `{"name":"scoped write","functional_unit":"req"}`, 201)
}
