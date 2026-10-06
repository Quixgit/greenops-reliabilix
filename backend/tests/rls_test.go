//go:build integration

package tests

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	dash "github.com/quixgit/greenops-reliabilix/backend/internal/dashboard/domain"
	dashrepo "github.com/quixgit/greenops-reliabilix/backend/internal/dashboard/repository"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/database"
	"github.com/quixgit/greenops-reliabilix/backend/internal/projects/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/projects/repository"
)

// Every table that carries tenant data must have RLS enabled AND forced. A new table without it fails here.
func TestRLSEnabledOnAllTenantTables(t *testing.T) {
	owner := pool(t, "TEST_OWNER_DSN")
	rows, err := owner.Query(context.Background(), `
		SELECT n.nspname || '.' || c.relname, c.relrowsecurity, c.relforcerowsecurity
		FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind IN ('r','p') AND c.relispartition = false
		  AND n.nspname IN ('tenants','projects','cloudaccounts','usage','carbon','finops','recommendations','reports','audit')
		  AND n.nspname || '.' || c.relname NOT IN ('carbon.grid_intensity', 'finops.region_price_index')`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var name string
		var rls, force bool
		if err := rows.Scan(&name, &rls, &force); err != nil {
			t.Fatal(err)
		}
		n++
		if !rls || !force {
			t.Errorf("%s: row level security not enabled+forced", name)
		}
	}
	if n < 15 {
		t.Fatalf("only %d tables inspected: schema changed?", n)
	}
}

func TestProjectsTenantIsolationAndRolePrivileges(t *testing.T) {
	ctx := context.Background()
	owner, api := pool(t, "TEST_OWNER_DSN"), pool(t, "TEST_API_DSN")
	a, b := org(t, owner, "A-"+t.Name()), org(t, owner, "B-"+t.Name())
	repo := repository.Postgres{Pool: api}

	cctx := auth.WithClaims(ctx, auth.Claims{Subject: "u1", TenantID: a, Role: auth.RoleAdmin})
	p, err := domain.New(a, "api", "request", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	created, err := repo.Create(cctx, p)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := repo.Create(cctx, p); err != domain.ErrDuplicate {
		t.Fatalf("duplicate: %v", err)
	}
	if got, _ := repo.List(ctx, b); len(got) != 0 {
		t.Fatalf("tenant B sees %d of tenant A's projects", len(got))
	}
	if _, err := repo.Get(ctx, b, created.ID); err != domain.ErrNotFound {
		t.Fatalf("tenant B reads A's project by id: %v", err)
	}
	if got, _ := repo.List(ctx, a); len(got) != 1 {
		t.Fatalf("tenant A sees %d projects, want 1", len(got))
	}

	// A query that forgets the tenant filter is contained by RLS, and cross-tenant writes are refused (WITH CHECK).
	_ = database.WithTenantTx(ctx, api, b, func(tx pgx.Tx) error {
		var n int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM projects.projects`).Scan(&n); err != nil || n != 0 {
			t.Errorf("unfiltered query as tenant B: n=%d err=%v", n, err)
		}
		return nil
	})
	if err := database.WithTenantTx(ctx, api, b, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO projects.projects (tenant_id, name, functional_unit) VALUES ($1,'evil','x')`, a)
		return err
	}); err == nil {
		t.Error("tenant B inserted a row for tenant A")
	}
	var n int
	if err := api.QueryRow(ctx, `SELECT count(*) FROM projects.projects`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("no-tenant query must see nothing (fail closed): n=%d err=%v", n, err)
	}

	// audit entry written in the same transaction; the trail is append-only for the runtime roles
	var audited int
	_ = database.WithTenantTx(ctx, api, a, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT count(*) FROM audit.audit_logs WHERE action='project.created'`).Scan(&audited)
	})
	if audited != 1 {
		t.Errorf("audit entries = %d, want 1", audited)
	}
	for _, stmt := range []string{`DELETE FROM audit.audit_logs`, `UPDATE audit.audit_logs SET action='x'`} {
		if err := database.WithTenantTx(ctx, api, a, func(tx pgx.Tx) error { _, err := tx.Exec(ctx, stmt); return err }); err == nil {
			t.Errorf("api role could run %q", stmt)
		}
	}
	// least privilege: the API reads usage and carbon but never writes them (only the worker does)
	for _, stmt := range []string{
		`INSERT INTO usage.usage_records (tenant_id,project_id,source,provider,service_name,service_category,region_id,billed_cost,effective_cost,currency,recorded_at,charge_period_end)
		 VALUES ('` + a + `','` + created.ID + `','x','aws','s','other','r',1,1,'USD',now(),now()+interval '1 day')`,
		`DELETE FROM carbon.calculations`,
	} {
		if err := database.WithTenantTx(ctx, api, a, func(tx pgx.Tx) error { _, err := tx.Exec(ctx, stmt); return err }); err == nil {
			t.Errorf("api role could write usage/carbon: %.40s", stmt)
		}
	}
}

func TestWorkerRoleAndCrossTenantFunctions(t *testing.T) {
	ctx := context.Background()
	owner, worker, api := pool(t, "TEST_OWNER_DSN"), pool(t, "TEST_WORKER_DSN"), pool(t, "TEST_API_DSN")
	if err := database.EnsurePartitions(ctx, worker, time.Now().UTC()); err != nil {
		t.Fatalf("worker cannot ensure partitions: %v", err)
	}
	if _, err := worker.Exec(ctx, `SELECT platform.ensure_month_partition('tenants.users', 'created_at', current_date)`); err == nil {
		t.Error("ensure_month_partition accepted an unmanaged table")
	}
	if _, err := api.Exec(ctx, `SELECT platform.ensure_month_partition('usage.usage_records', 'recorded_at', current_date)`); err == nil {
		t.Error("api role may create partitions")
	}
	a := org(t, owner, "fanout-"+t.Name())
	var proj string
	if err := owner.QueryRow(ctx, `INSERT INTO projects.projects (tenant_id,name,functional_unit) VALUES ($1,'p','u') RETURNING id`, a).Scan(&proj); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Exec(ctx, `INSERT INTO cloudaccounts.connections (tenant_id,project_id,provider,account_ref,credential_ref,external_id) VALUES ($1,$2,'aws','1','arn:aws:iam::1:role/r','rlx-x')`, a, proj); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := worker.QueryRow(ctx, `SELECT count(*) FROM cloudaccounts.list_connections_for_sync()`).Scan(&n); err != nil || n < 1 {
		t.Fatalf("fan-out function: n=%d err=%v", n, err)
	}
	if err := worker.QueryRow(ctx, `SELECT count(*) FROM projects.list_projects_for_jobs()`).Scan(&n); err != nil || n < 1 {
		t.Fatalf("project fan-out: n=%d err=%v", n, err)
	}
	if err := worker.QueryRow(ctx, `SELECT count(*) FROM cloudaccounts.connections`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("worker sees %d connections without tenant context (RLS must apply)", n)
	}
	if _, err := api.Exec(ctx, `SELECT * FROM cloudaccounts.list_connections_for_sync()`); err == nil {
		t.Error("api role may call the cross-tenant fan-out")
	}
	if _, err := worker.Exec(ctx, `SELECT * FROM tenants.resolve_membership('x', NULL)`); err == nil {
		t.Error("worker role may resolve memberships")
	}
}

func TestDashboardIsolationAndEmptyTenant(t *testing.T) {
	ctx := context.Background()
	owner, api := pool(t, "TEST_OWNER_DSN"), pool(t, "TEST_API_DSN")
	a, b := org(t, owner, "dash-A-"+t.Name()), org(t, owner, "dash-B-"+t.Name())
	var proj string
	if err := owner.QueryRow(ctx, `INSERT INTO projects.projects (tenant_id,name,functional_unit) VALUES ($1,'p','u') RETURNING id`, a).Scan(&proj); err != nil {
		t.Fatal(err)
	}
	day := time.Now().UTC().Truncate(24 * time.Hour).Add(-48 * time.Hour)
	if _, err := owner.Exec(ctx, `INSERT INTO usage.usage_records (tenant_id,project_id,source,provider,service_name,service_category,region_id,billed_cost,effective_cost,currency,recorded_at,charge_period_end)
		VALUES ($1,$2,'cost_explorer','aws','Amazon EC2','compute','eu-central-1',42,42,'USD',$3,$4)`, a, proj, day, day.Add(24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Exec(ctx, `INSERT INTO carbon.calculations (tenant_id,project_id,provider,region_id,service_name,service_category,method,period_start,period_end,energy_kwh,intensity_g_per_kwh,carbon_kg_co2e,methodology_version)
		VALUES ($1,$2,'aws','eu-central-1','Amazon EC2','compute','cost_based',$3,$4,5,400,2,'CCF-2026.1')`, a, proj, day, day.Add(24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Exec(ctx, `INSERT INTO audit.audit_logs (tenant_id,actor_user_id,action,target) VALUES ($1,'u','project.created','project:p'),($1,'u','member.role_changed','member:x')`, a); err != nil {
		t.Fatal(err)
	}

	repo := dashrepo.Postgres{Pool: api}
	f := dash.Filter{From: day.Add(-24 * time.Hour), To: time.Now().UTC().Add(24 * time.Hour)}
	f.TenantID = a
	if v, err := repo.Trend(ctx, f); err != nil || len(v) != 1 || v[0].Cost != 42 || v[0].CO2eKg != 2 {
		t.Fatalf("tenant A trend: %+v %v", v, err)
	}
	if v, _ := repo.Providers(ctx, f); len(v) != 1 || v[0].SharePct != 100 {
		t.Fatalf("tenant A providers: %+v", v)
	}
	if v, _ := repo.Services(ctx, f, "compute"); len(v) != 1 || v[0].Service != "Amazon EC2" {
		t.Fatalf("tenant A services: %+v", v)
	}
	if v, _ := repo.Services(ctx, f, "storage"); len(v) != 0 {
		t.Fatalf("category filter ignored: %+v", v)
	}
	if v, _ := repo.Activity(ctx, a, 10); len(v) != 1 || v[0].Action != "project.created" {
		t.Fatalf("activity must be allowlisted: %+v", v)
	}
	f.TenantID = b // an empty tenant gets empty answers, never another tenant's data
	if v, err := repo.Trend(ctx, f); err != nil || len(v) != 0 {
		t.Fatalf("tenant B trend: %+v %v", v, err)
	}
	if v, _ := repo.Providers(ctx, f); len(v) != 0 {
		t.Fatalf("tenant B providers: %+v", v)
	}
	if v, _ := repo.Services(ctx, f, ""); len(v) != 0 {
		t.Fatalf("tenant B services: %+v", v)
	}
	if v, _ := repo.Activity(ctx, b, 10); len(v) != 0 {
		t.Fatalf("tenant B activity: %+v", v)
	}
}
