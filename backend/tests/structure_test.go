//go:build integration

package tests

import (
	"context"
	"fmt"
	"math"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/app"
)

// Teams, environments and allocation rules over the real API and database: cost is moved between projects without
// being created or dropped, environments roll up the cost of their connections, and another organization's rows
// can neither be referenced nor read.
func TestOrganizationStructureAndAllocation(t *testing.T) {
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
	alice := fmt.Sprintf("dev:st-%s:-:-:st-%s@example.com", rid, rid)
	bob := fmt.Sprintf("dev:sb-%s:-:-:sb-%s@example.com", rid, rid)
	tenant := str(c.must("POST", "/api/v1/onboarding", alice, `{"organization_name":"Structure Org"}`, 201), "tenant_id")
	other := str(c.must("POST", "/api/v1/onboarding", bob, `{"organization_name":"Other Org"}`, 201), "tenant_id")

	mkProject := func(tn, name string) string {
		var id string
		if err := owner.QueryRow(ctx, `INSERT INTO projects.projects (tenant_id, name, functional_unit) VALUES ($1,$2,'req') RETURNING id::text`, tn, name).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	mkConn := func(tn, project, ref string) string {
		var id string
		if err := owner.QueryRow(ctx, `INSERT INTO cloudaccounts.connections (tenant_id,project_id,provider,account_ref,credential_ref,external_id)
			VALUES ($1,$2,'aws',$3,'arn:aws:iam::1:role/r','rlx-x') RETURNING id::text`, tn, project, ref).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	shared, web := mkProject(tenant, "shared-platform"), mkProject(tenant, "web")
	foreign := mkProject(other, "foreign")
	sharedConn, webProd, webDev := mkConn(tenant, shared, "111111111111"), mkConn(tenant, web, "222222222222"), mkConn(tenant, web, "333333333333")
	foreignConn := mkConn(other, foreign, "444444444444")

	day := time.Now().UTC().Add(-36 * time.Hour)
	ins := func(project, conn, service string, cost float64) {
		if _, err := owner.Exec(ctx, `INSERT INTO usage.usage_records (tenant_id, project_id, connection_id, resource_id, source, provider, service_name, service_category, region_id, billed_cost, effective_cost, currency, recorded_at, charge_period_end)
			VALUES ($1,$2,$3::uuid,$3::text,'cost_explorer','aws',$4,'compute','eu-central-1',$5,$5,'USD',$6::timestamptz,$6::timestamptz + interval '1 day')`, tenant, project, conn, service, cost, day); err != nil {
			t.Fatal(err)
		}
	}
	ins(shared, sharedConn, "Support", 100)
	ins(shared, sharedConn, "EC2", 400)
	ins(web, webProd, "EC2", 60)
	ins(web, webDev, "EC2", 20)

	// teams
	team := str(c.must("POST", "/api/v1/teams", alice, `{"name":"Platform"}`, 201), "id")
	c.must("POST", "/api/v1/teams", alice, `{"name":"Platform"}`, 409)
	c.must("PUT", "/api/v1/projects/"+shared+"/team", alice, fmt.Sprintf(`{"team_id":%q}`, team), 200)
	c.must("PUT", "/api/v1/projects/"+foreign+"/team", alice, fmt.Sprintf(`{"team_id":%q}`, team), 404) // another organization's project

	// environments: only the project's own connections can be assigned
	prod := str(c.must("POST", "/api/v1/environments", alice, fmt.Sprintf(`{"project_id":%q,"name":"prod","kind":"production"}`, web), 201), "id")
	c.must("POST", "/api/v1/environments", alice, fmt.Sprintf(`{"project_id":%q,"name":"x","kind":"moon"}`, web), 422)
	c.must("PUT", "/api/v1/environments/"+prod+"/connections", alice, fmt.Sprintf(`{"connection_ids":[%q]}`, sharedConn), 422) // wrong project
	c.must("PUT", "/api/v1/environments/"+prod+"/connections", alice, fmt.Sprintf(`{"connection_ids":[%q]}`, foreignConn), 422)  // another organization
	c.must("PUT", "/api/v1/environments/"+prod+"/connections", alice, fmt.Sprintf(`{"connection_ids":[%q]}`, webProd), 200)

	// allocation rule: Support of shared-platform is split 3:1 between web and the platform itself
	rule := fmt.Sprintf(`{"name":"Support","source_project_id":%q,"service_name":"Support","targets":[{"project_id":%q,"weight":3},{"project_id":%q,"weight":1}]}`, shared, web, shared)
	ruleID := str(c.must("POST", "/api/v1/allocation-rules", alice, rule, 201), "id")
	c.must("POST", "/api/v1/allocation-rules", alice, rule, 409) // one rule per source and service
	c.must("POST", "/api/v1/allocation-rules", alice, fmt.Sprintf(`{"name":"x","source_project_id":%q,"targets":[{"project_id":%q,"weight":1}]}`, shared, foreign), 404)
	c.must("POST", "/api/v1/allocation-rules", alice, fmt.Sprintf(`{"name":"x","source_project_id":%q,"targets":[{"project_id":%q,"weight":0}]}`, shared, web), 422)

	rep := c.must("GET", "/api/v1/allocation", alice, "", 200)
	if str(rep, "currency") != "USD" {
		t.Fatalf("currency = %q", str(rep, "currency"))
	}
	totals := map[string]map[string]any{}
	var sum float64
	for _, p := range rep["projects"].([]any) {
		m := p.(map[string]any)
		totals[str(m, "project_id")] = m
		sum += m["total"].(float64)
	}
	if math.Abs(sum-580) > 1e-6 {
		t.Errorf("totals add up to %v, want the direct 580", sum)
	}
	if m := totals[shared]; m["direct"].(float64) != 500 || m["allocated_out"].(float64) != 75 || m["total"].(float64) != 425 {
		t.Errorf("shared = %v", m)
	}
	if m := totals[web]; m["allocated_in"].(float64) != 75 || m["total"].(float64) != 155 {
		t.Errorf("web = %v", m)
	}
	envByID := map[string]float64{}
	for _, e := range rep["environments"].([]any) {
		m := e.(map[string]any)
		id, _ := m["environment_id"].(string)
		envByID[str(m, "project_id")+"|"+id] += m["cost"].(float64)
	}
	if envByID[web+"|"+prod] != 60 || envByID[web+"|"] != 20 {
		t.Errorf("environments = %v (web prod 60, unassigned 20)", envByID)
	}
	if len(rep["teams"].([]any)) != 2 {
		t.Errorf("teams = %v, want the Platform team and the projects without a team", rep["teams"])
	}

	// the other organization sees none of it, and cannot delete it
	if n := len(items(c.must("GET", "/api/v1/allocation-rules", bob, "", 200))); n != 0 {
		t.Errorf("another organization sees %d rules", n)
	}
	c.must("DELETE", "/api/v1/allocation-rules/"+ruleID, bob, "", 404)
	c.must("DELETE", "/api/v1/allocation-rules/"+ruleID, alice, "", 204)
	c.must("DELETE", "/api/v1/teams/"+team, alice, "", 204)
	var teamID *string
	if err := owner.QueryRow(ctx, `SELECT team_id::text FROM projects.projects WHERE id = $1`, shared).Scan(&teamID); err != nil || teamID != nil {
		t.Errorf("deleting a team must leave its projects without one: %v %v", teamID, err)
	}
}
