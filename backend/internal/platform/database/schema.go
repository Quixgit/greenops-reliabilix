package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ExpectedMigration is the newest goose migration this build needs. A unit test ties it to the files in
// backend/migrations, so adding a migration without bumping it fails the build.
const ExpectedMigration = 11

// SchemaStatus is what `admin doctor` learns about the database the process is connected to.
type SchemaStatus struct {
	Role              string
	Superuser         bool
	BypassRLS         bool
	MigrationVersion  int64 // 0 when the goose table does not exist
	TenantTables      int
	UnprotectedTables []string // tenant tables without ENABLE + FORCE row level security
	MonthPartition    bool     // the current month's usage partition exists
}

// CheckSchema inspects role flags, migration level, row-level-security coverage and partitions.
func CheckSchema(ctx context.Context, pool *pgxpool.Pool, now time.Time) (SchemaStatus, error) {
	var s SchemaStatus
	if err := pool.QueryRow(ctx, `SELECT current_user, r.rolsuper, r.rolbypassrls FROM pg_roles r WHERE r.rolname = current_user`).
		Scan(&s.Role, &s.Superuser, &s.BypassRLS); err != nil {
		return s, err
	}
	var exists bool
	if err := pool.QueryRow(ctx, `SELECT to_regclass('public.goose_db_version') IS NOT NULL`).Scan(&exists); err != nil {
		return s, err
	}
	if exists {
		if err := pool.QueryRow(ctx, `SELECT COALESCE(max(version_id), 0) FROM public.goose_db_version WHERE is_applied`).Scan(&s.MigrationVersion); err != nil {
			return s, err
		}
	}
	rows, err := pool.Query(ctx, `
		SELECT n.nspname || '.' || c.relname, c.relrowsecurity AND c.relforcerowsecurity
		FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind IN ('r','p') AND NOT c.relispartition
		  AND n.nspname IN ('tenants','projects','cloudaccounts','usage','carbon','finops','recommendations','reports','audit','automation')
		  AND n.nspname || '.' || c.relname NOT IN ('carbon.grid_intensity', 'finops.region_price_index')`)
	if err != nil {
		return s, err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		var ok bool
		if err := rows.Scan(&name, &ok); err != nil {
			return s, err
		}
		s.TenantTables++
		if !ok {
			s.UnprotectedTables = append(s.UnprotectedTables, name)
		}
	}
	if err := rows.Err(); err != nil {
		return s, err
	}
	part := fmt.Sprintf("usage.usage_records_%s", now.UTC().Format("2006_01"))
	err = pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, part).Scan(&s.MonthPartition)
	return s, err
}
