//go:build integration

package tests

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/quixgit/greenops-reliabilix/backend/internal/lifecycle/application"
	"github.com/quixgit/greenops-reliabilix/backend/internal/lifecycle/repository"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/storage"
)

// seedOrg creates an organization with a row in every kind of table the export and the erasure have to cover.
func seedOrg(t *testing.T, owner pgxQuerier, name string) (tenant, project string) {
	t.Helper()
	ctx := context.Background()
	tenant = org(t, owner.pool(), name)
	must := func(sql string, args ...any) {
		t.Helper()
		if _, err := owner.pool().Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	if err := owner.pool().QueryRow(ctx, `INSERT INTO projects.projects (tenant_id, name, functional_unit) VALUES ($1,'p','api_request') RETURNING id`, tenant).Scan(&project); err != nil {
		t.Fatal(err)
	}
	var user string
	if err := owner.pool().QueryRow(ctx, `INSERT INTO tenants.users (auth_sub, email) VALUES ($1,$2) RETURNING id`, "sub-"+tenant, tenant+"@erase.example").Scan(&user); err != nil {
		t.Fatal(err)
	}
	must(`INSERT INTO tenants.memberships (organization_id, user_id, role) VALUES ($1,$2,'owner')`, tenant, user)
	must(`INSERT INTO tenants.api_keys (organization_id, name, prefix, key_hash, role, created_by) VALUES ($1,'k','grk_ab',$2,'ci',$3)`, tenant, "hash-"+tenant, user)
	must(`INSERT INTO projects.policies (tenant_id, project_id) VALUES ($1,$2)`, tenant, project)
	must(`INSERT INTO usage.usage_records (tenant_id, project_id, source, provider, service_name, service_category, region_id, billed_cost, effective_cost, currency, recorded_at, charge_period_end)
		VALUES ($1,$2,'cost_explorer','aws','EC2','compute','eu-central-1',10,10,'USD',now(), now() + interval '1 day')`, tenant, project)
	must(`INSERT INTO carbon.calculations (tenant_id, project_id, provider, region_id, service_name, service_category, method, period_start, period_end, energy_kwh, intensity_g_per_kwh, carbon_kg_co2e, methodology_version)
		VALUES ($1,$2,'aws','eu-central-1','EC2','compute','usage_based',now(), now() + interval '1 day',1,300,1,'RLX-PROVISIONAL-1')`, tenant, project)
	must(`INSERT INTO finops.budgets (tenant_id, project_id, amount, currency, period) VALUES ($1,$2,100,'USD','monthly')`, tenant, project)
	must(`INSERT INTO audit.audit_logs (tenant_id, actor_user_id, action, target) VALUES ($1,'x','test.seed','t')`, tenant)
	return tenant, project
}

type pgxQuerier interface{ pool() poolT }

func countAll(t *testing.T, owner poolT, tenant string) int {
	t.Helper()
	ctx := context.Background()
	tx, err := owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	_, _ = tx.Exec(ctx, `SELECT set_config('app.tenant_id', $1, true)`, tenant)
	rows, err := tx.Query(ctx, `SELECT n.nspname, c.relname,
		(SELECT a.attname FROM pg_attribute a WHERE a.attrelid=c.oid AND a.attname IN ('tenant_id','organization_id') AND NOT a.attisdropped LIMIT 1)
		FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
		WHERE c.relkind IN ('r','p') AND NOT c.relispartition AND c.relrowsecurity AND n.nspname NOT IN ('pg_catalog','information_schema','platform')
		AND EXISTS (SELECT 1 FROM pg_attribute a WHERE a.attrelid=c.oid AND a.attname IN ('tenant_id','organization_id') AND NOT a.attisdropped)`)
	if err != nil {
		t.Fatal(err)
	}
	type tc struct{ s, r, c string }
	var tabs []tc
	for rows.Next() {
		var x tc
		if err := rows.Scan(&x.s, &x.r, &x.c); err != nil {
			t.Fatal(err)
		}
		tabs = append(tabs, x)
	}
	rows.Close()
	total := 0
	for _, x := range tabs {
		var n int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM `+pgx.Identifier{x.s, x.r}.Sanitize()+` WHERE `+x.c+` = $1`, tenant).Scan(&n); err != nil {
			t.Fatal(err)
		}
		total += n
	}
	return total
}

func TestOrganizationErasureIsCompleteAndLeavesOnlyATombstone(t *testing.T) {
	ctx := context.Background()
	ownerDB, workerPool := pool(t, "TEST_OWNER_DSN"), pool(t, "TEST_WORKER_DSN")
	tenant, _ := seedOrg(t, wrap{ownerDB}, "Erase Me")
	other, _ := seedOrg(t, wrap{ownerDB}, "Keep Me")
	t.Cleanup(func() { _, _ = ownerDB.Exec(ctx, `DELETE FROM platform.erasure_log WHERE tenant_id = $1`, tenant) })

	if before := countAll(t, ownerDB, tenant); before < 8 {
		t.Fatalf("seed too small to prove anything: %d rows", before)
	}
	st := storage.NewMemory()
	key, _ := storage.Key(tenant, "reports", "r.pdf")
	otherKey, _ := storage.Key(other, "reports", "r.pdf")
	_ = st.Put(ctx, key, jsonReader("x"))
	_ = st.Put(ctx, otherKey, jsonReader("y"))

	repo := repository.Postgres{Pool: workerPool}
	// Not due yet: nothing happens.
	svc := application.Service{Repo: repo, Store: st, Log: quietLog()}
	if n, err := svc.EraseDue(ctx); err != nil || n != 0 {
		t.Fatalf("nothing is due: n=%d err=%v", n, err)
	}
	// Make it due.
	if _, err := ownerDB.Exec(ctx, `INSERT INTO tenants.deletion_requests (tenant_id, requested_by, requested_at, execute_after) VALUES ($1,'me', now() - interval '2 days', now() - interval '1 minute')`, tenant); err != nil {
		t.Fatal(err)
	}
	n, err := svc.EraseDue(ctx)
	if err != nil || n != 1 {
		t.Fatalf("erased %d, err %v", n, err)
	}
	if left := countAll(t, ownerDB, tenant); left != 0 {
		t.Fatalf("%d rows of the organization remain", left)
	}
	var orgs int
	_ = ownerDB.QueryRow(ctx, `SELECT count(*) FROM tenants.organizations WHERE id = $1`, tenant).Scan(&orgs)
	if orgs != 0 {
		t.Fatal("the organization row must be gone")
	}
	var users int
	_ = ownerDB.QueryRow(ctx, `SELECT count(*) FROM tenants.users WHERE email = $1`, tenant+"@erase.example").Scan(&users)
	if users != 0 {
		t.Fatal("a user who belonged only to this organization must be gone")
	}
	if _, ok := st.M[key]; ok {
		t.Fatal("the organization's files must be gone")
	}
	if _, ok := st.M[otherKey]; !ok {
		t.Fatal("another organization's files must be untouched")
	}
	if countAll(t, ownerDB, other) < 8 {
		t.Fatal("another organization's data must be untouched")
	}
	var logged int
	var counts []byte
	if err := ownerDB.QueryRow(ctx, `SELECT count(*), max(row_counts::text)::bytea FROM platform.erasure_log WHERE tenant_id = $1`, tenant).Scan(&logged, &counts); err != nil || logged != 1 {
		t.Fatalf("tombstone: %d %v", logged, err)
	}
	if string(counts) == "" || !json.Valid(counts) {
		t.Fatalf("row counts not recorded: %q", counts)
	}
	t.Cleanup(func() {
		_, _ = ownerDB.Exec(ctx, `DELETE FROM tenants.organizations WHERE id = $1`, other)
	})
}

func TestExportContainsTheOrganizationAndNoSecrets(t *testing.T) {
	ctx := context.Background()
	ownerDB, apiPool := pool(t, "TEST_OWNER_DSN"), pool(t, "TEST_API_DSN")
	tenant, _ := seedOrg(t, wrap{ownerDB}, "Export Me")
	t.Cleanup(func() { _, _ = ownerDB.Exec(ctx, `DELETE FROM tenants.organizations WHERE id = $1`, tenant) })

	got := map[string]int{}
	var all []byte
	err := repository.Postgres{Pool: apiPool}.Export(ctx, tenant, func(table string, row json.RawMessage) error {
		got[table]++
		all = append(all, row...)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"organization", "members", "projects", "usage_records", "carbon_calculations", "budgets", "audit_log", "api_keys"} {
		if got[want] == 0 {
			t.Errorf("export has no rows of %s: %v", want, got)
		}
	}
	for _, secret := range []string{"hash-" + tenant, "key_hash", "sub-" + tenant, "token_hash"} {
		if contains(all, secret) {
			t.Errorf("the export contains %q", secret)
		}
	}
}

func TestRetentionDropsOnlyOldWholeMonths(t *testing.T) {
	ctx := context.Background()
	ownerDB, workerPool := pool(t, "TEST_OWNER_DSN"), pool(t, "TEST_WORKER_DSN")
	tenant, project := seedOrg(t, wrap{ownerDB}, "Retention Org")
	t.Cleanup(func() { _, _ = ownerDB.Exec(ctx, `DELETE FROM tenants.organizations WHERE id = $1`, tenant) })

	old := time.Date(2023, 3, 15, 0, 0, 0, 0, time.UTC)
	if _, err := workerPool.Exec(ctx, `SELECT platform.ensure_month_partition('usage.usage_records', 'recorded_at', $1::date)`, old); err != nil {
		t.Fatal(err)
	}
	if _, err := ownerDB.Exec(ctx, `INSERT INTO usage.usage_records (tenant_id, project_id, source, provider, service_name, service_category, region_id, billed_cost, effective_cost, currency, recorded_at, charge_period_end)
		VALUES ($1,$2,'cost_explorer','aws','EC2','compute','eu-central-1',1,1,'USD',$3,$3::timestamptz + interval '1 day')`, tenant, project, old); err != nil {
		t.Fatal(err)
	}
	n, err := repository.Postgres{Pool: workerPool}.DropOldPartitions(ctx, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil || n < 1 {
		t.Fatalf("dropped %d, err %v", n, err)
	}
	var old2, cur int
	_ = ownerDB.QueryRow(ctx, `SELECT count(*) FROM usage.usage_records WHERE tenant_id=$1 AND recorded_at < '2024-01-01'`, tenant).Scan(&old2)
	_ = ownerDB.QueryRow(ctx, `SELECT count(*) FROM usage.usage_records WHERE tenant_id=$1 AND recorded_at > now() - interval '1 day'`, tenant).Scan(&cur)
	if old2 != 0 || cur != 1 {
		t.Fatalf("old rows %d (want 0), current rows %d (want 1)", old2, cur)
	}
}
