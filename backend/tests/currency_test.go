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

// Amounts in different currencies are never added together: every cost view uses the dominant currency of the window.
func TestCostViewsNeverMixCurrencies(t *testing.T) {
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
	alice := fmt.Sprintf("dev:cur-%s:-:-:cur-%s@example.com", rid, rid)
	tenant := str(c.must("POST", "/api/v1/onboarding", alice, `{"organization_name":"Currency Test Org"}`, 201), "tenant_id")
	var project string
	if err := owner.QueryRow(ctx, `INSERT INTO projects.projects (tenant_id, name, functional_unit) VALUES ($1,'p','req') RETURNING id::text`, tenant).Scan(&project); err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC().Add(-36 * time.Hour)
	ins := func(service, currency string, cost float64) {
		if _, err := owner.Exec(ctx, `INSERT INTO usage.usage_records (tenant_id, project_id, source, provider, service_name, service_category, region_id, billed_cost, effective_cost, currency, recorded_at, charge_period_end)
			VALUES ($1,$2,'cost_explorer','aws',$3,'compute','eu-central-1',$4,$4,$5,$6::timestamptz, $6::timestamptz + interval '1 day')`, tenant, project, service, cost, currency, at); err != nil {
			t.Fatal(err)
		}
	}
	ins("EC2", "EUR", 500)
	ins("S3", "USD", 100) // a smaller amount in another currency must not be added to the EUR total

	fin := c.must("GET", "/api/v1/finops/summary", alice, "", 200)
	if str(fin, "currency") != "EUR" || fin["total_cost"].(float64) != 500 {
		t.Errorf("summary = %v, want EUR 500 (USD 100 excluded)", fin)
	}
	var cost float64
	for _, p := range items(c.must("GET", "/api/v1/dashboard/trend", alice, "", 200)) {
		cost += p.(map[string]any)["cost"].(float64)
	}
	if cost != 500 {
		t.Errorf("dashboard trend cost = %v, want 500", cost)
	}
	svcs := items(c.must("GET", "/api/v1/dashboard/services", alice, "", 200))
	if len(svcs) != 1 {
		t.Errorf("dashboard services = %v, want only the EUR service", svcs)
	}
}
