//go:build integration

// Integration tests against a real PostgreSQL with migrations applied.
//
//	TEST_OWNER_DSN  migration/owner role (creates fixtures)
//	TEST_API_DSN    greenops_api role
//	TEST_WORKER_DSN greenops_worker role
package tests

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	dash "github.com/quixgit/greenops-reliabilix/backend/internal/dashboard/domain"
	dashrepo "github.com/quixgit/greenops-reliabilix/backend/internal/dashboard/repository"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/database"
	"github.com/quixgit/greenops-reliabilix/backend/internal/projects/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/projects/repository"
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
	if err := owner.QueryRow(context.Background(),
		`INSERT INTO tenants.organizations (name) VALUES ($1) RETURNING id`, name).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

// Every table that carries tenant data must have RLS enabled AND forced.
func TestRLSEnabledOnAllTenantTables(t *testing.T) {
	owner := pool(t, "TEST_OWNER_DSN")
	rows, err := owner.Query(context.Background(), `
		SELECT n.nspname || '.' || c.relname, c.relrowsecurity, c.relforcerowsecurity
		FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind IN ('r','p') AND c.relispartition = false
		  AND n.nspname IN ('tenants','projects','cloudaccounts','usage','carbon','finops','recommendations','reports','audit')
		  AND (c.relname <> 'users' AND c.relname <> 'grid_intensity')`)
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
	if n == 0 {
		t.Fatal("no tables inspected")
	}
}

func TestProjectsTenantIsolation(t *testing.T) {
	ctx := context.Background()
	owner, api := pool(t, "TEST_OWNER_DSN"), pool(t, "TEST_API_DSN")
	a, b := org(t, owner, "A-"+t.Name()), org(t, owner, "B-"+t.Name())
	repo := repository.Postgres{Pool: api}

	cctx := auth.WithClaims(ctx, auth.Claims{Subject: "u1", TenantID: a, Role: auth.RoleAdmin})
	p, err := domain.New(a, "api", "request", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Create(cctx, p); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := repo.Create(cctx, p); !errors.Is(err, domain.ErrDuplicate) {
		t.Fatalf("duplicate: %v", err)
	}

	if got, _ := repo.List(ctx, b); len(got) != 0 {
		t.Fatalf("tenant B sees %d of tenant A's projects", len(got))
	}
	if got, _ := repo.List(ctx, a); len(got) != 1 {
		t.Fatalf("tenant A sees %d projects, want 1", len(got))
	}

	// Even a query that forgets the tenant filter is contained by RLS...
	err = database.WithTenantTx(ctx, api, b, func(tx pgxTx) error {
		var n int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM projects.projects`).Scan(&n); err != nil {
			return err
		}
		if n != 0 {
			t.Errorf("unfiltered query as tenant B returned %d rows", n)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// ...and tenant B cannot write a row belonging to tenant A (WITH CHECK).
	err = database.WithTenantTx(ctx, api, b, func(tx pgxTx) error {
		_, err := tx.Exec(ctx, `INSERT INTO projects.projects (tenant_id, name, functional_unit) VALUES ($1,'evil','x')`, a)
		return err
	})
	if err == nil {
		t.Error("tenant B inserted a row for tenant A")
	}

	// Without any tenant context the app role sees nothing (fail closed).
	var n int
	if err := api.QueryRow(ctx, `SELECT count(*) FROM projects.projects`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("no-tenant query: n=%d err=%v", n, err)
	}

	// The create was audited in the same transaction, and the trail is append-only.
	var audited int
	_ = database.WithTenantTx(ctx, api, a, func(tx pgxTx) error {
		return tx.QueryRow(ctx, `SELECT count(*) FROM audit.audit_logs WHERE action='project.created'`).Scan(&audited)
	})
	if audited != 1 {
		t.Errorf("audit entries = %d, want 1", audited)
	}
	err = database.WithTenantTx(ctx, api, a, func(tx pgxTx) error {
		_, err := tx.Exec(ctx, `DELETE FROM audit.audit_logs`)
		return err
	})
	if err == nil {
		t.Error("api role could delete audit logs")
	}
}

func TestWorkerRole(t *testing.T) {
	ctx := context.Background()
	owner, worker := pool(t, "TEST_OWNER_DSN"), pool(t, "TEST_WORKER_DSN")
	if err := database.EnsurePartitions(ctx, worker, time.Now().UTC()); err != nil {
		t.Fatalf("worker cannot ensure partitions: %v", err)
	}
	if _, err := worker.Exec(ctx, `SELECT platform.ensure_month_partition('tenants.users', 'created_at', current_date)`); err == nil {
		t.Error("ensure_month_partition accepted an unmanaged table")
	}
	a := org(t, owner, "fanout-"+t.Name())
	var proj string
	if err := owner.QueryRow(ctx, `INSERT INTO projects.projects (tenant_id,name,functional_unit) VALUES ($1,'p','u') RETURNING id`, a).Scan(&proj); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Exec(ctx, `INSERT INTO cloudaccounts.connections (tenant_id,project_id,provider,account_ref,credential_ref) VALUES ($1,$2,'aws','1','arn:aws:iam::1:role/r')`, a, proj); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := worker.QueryRow(ctx, `SELECT count(*) FROM cloudaccounts.list_connections_for_sync()`).Scan(&n); err != nil || n < 1 {
		t.Fatalf("fan-out function: n=%d err=%v", n, err)
	}
	if err := worker.QueryRow(ctx, `SELECT count(*) FROM cloudaccounts.connections`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("worker sees %d connections without tenant context (RLS must apply)", n)
	}
}

type pgxTx = pgx.Tx

func TestDashboardIsolationAndEmptyTenant(t *testing.T) {
	ctx := context.Background()
	owner, api := pool(t, "TEST_OWNER_DSN"), pool(t, "TEST_API_DSN")
	a, b := org(t, owner, "dash-A-"+t.Name()), org(t, owner, "dash-B-"+t.Name())
	var proj string
	if err := owner.QueryRow(ctx, `INSERT INTO projects.projects (tenant_id,name,functional_unit) VALUES ($1,'p','u') RETURNING id`, a).Scan(&proj); err != nil {
		t.Fatal(err)
	}
	day := time.Now().UTC().Truncate(24 * time.Hour).Add(-48 * time.Hour)
	if _, err := owner.Exec(ctx, `INSERT INTO usage.usage_records (tenant_id,project_id,source,provider,resource_id,resource_type,service_name,service_category,region,usage_amount,usage_unit,cost,currency,recorded_at)
		VALUES ($1,$2,'cost_explorer','aws','r1','instance','Amazon EC2','compute','eu-central-1',10,'vcpu_hours',42,'USD',$3)`, a, proj, day); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Exec(ctx, `INSERT INTO carbon.calculations (tenant_id,project_id,provider,region,service_name,service_category,period_start,period_end,energy_kwh,carbon_kg_co2e,methodology_version)
		VALUES ($1,$2,'aws','eu-central-1','Amazon EC2','compute',$3,$4,5,2,'CCF-2026.1')`, a, proj, day, day.Add(24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Exec(ctx, `INSERT INTO audit.audit_logs (tenant_id,actor_user_id,action,target) VALUES ($1,'u','project.created','project:p'),($1,'u','role.changed','user:x')`, a); err != nil {
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
	// Recent Activity shows only allowlisted user-facing actions, never e.g. role changes.
	if v, _ := repo.Activity(ctx, a, 10); len(v) != 1 || v[0].Action != "project.created" {
		t.Fatalf("activity must be allowlisted: %+v", v)
	}

	f.TenantID = b // empty tenant: no rows, no errors, never another tenant's data
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
