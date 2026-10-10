// Package repository is the PostgreSQL adapter of the lifecycle domain.
package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quixgit/greenops-reliabilix/backend/internal/lifecycle/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/audit"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/database"
)

type Postgres struct{ Pool *pgxpool.Pool }

func (r Postgres) OrganizationName(ctx context.Context, tenantID string) (name string, err error) {
	err = database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT name FROM tenants.organizations WHERE id = $1`, tenantID).Scan(&name)
	})
	return
}

func (r Postgres) Deletion(ctx context.Context, tenantID string) (d domain.Deletion, err error) {
	err = database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		e := tx.QueryRow(ctx, `SELECT requested_by, requested_at, execute_after FROM tenants.deletion_requests WHERE tenant_id = $1`, tenantID).
			Scan(&d.RequestedBy, &d.RequestedAt, &d.ExecuteAfter)
		if database.IsNotFound(e) {
			return nil
		}
		d.Scheduled = e == nil
		return e
	})
	return
}

// Schedule records the request; a second request while one exists is ErrAlreadyScheduled (the primary key).
func (r Postgres) Schedule(ctx context.Context, tenantID, by string, executeAfter time.Time) error {
	return database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `INSERT INTO tenants.deletion_requests (tenant_id, requested_by, execute_after) VALUES ($1,$2,$3)
			ON CONFLICT (tenant_id) DO NOTHING`, tenantID, by, executeAfter)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrAlreadyScheduled
		}
		return audit.Record(audit.WithTenant(ctx, tenantID), tx, "tenant.deletion_scheduled", "tenant:"+tenantID,
			map[string]any{"execute_after": executeAfter.Format(time.RFC3339)})
	})
}

func (r Postgres) Cancel(ctx context.Context, tenantID string) error {
	return database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `DELETE FROM tenants.deletion_requests WHERE tenant_id = $1`, tenantID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrNothingScheduled
		}
		return audit.Record(audit.WithTenant(ctx, tenantID), tx, "tenant.deletion_cancelled", "tenant:"+tenantID, nil)
	})
}

// Export streams every allow-listed table as NDJSON lines {"table":..., "row":{...}} from one consistent snapshot.
// The callback receives each line; a returned error stops the export.
func (r Postgres) Export(ctx context.Context, tenantID string, emit func(table string, row json.RawMessage) error) error {
	return database.WithTenantTxOpts(ctx, r.Pool, tenantID, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
		for _, t := range domain.ExportTables {
			rows, err := tx.Query(ctx, t.SQL)
			if err != nil {
				return err
			}
			for rows.Next() {
				var raw json.RawMessage
				if err := rows.Scan(&raw); err != nil {
					rows.Close()
					return err
				}
				if err := emit(t.Name, raw); err != nil {
					rows.Close()
					return err
				}
			}
			rows.Close()
			if err := rows.Err(); err != nil {
				return err
			}
		}
		return nil
	})
}

// DueDeletions lists organizations whose grace period is over (worker role).
func (r Postgres) DueDeletions(ctx context.Context) ([]string, error) {
	rows, err := r.Pool.Query(ctx, `SELECT tenant_id::text FROM platform.due_deletions()`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// Erase removes the organization from the database atomically and returns the per-table row counts.
func (r Postgres) Erase(ctx context.Context, tenantID string) (map[string]int64, error) {
	var raw []byte
	if err := r.Pool.QueryRow(ctx, `SELECT platform.erase_tenant($1::uuid)`, tenantID).Scan(&raw); err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, nil // already gone
	}
	var counts map[string]int64
	if err := json.Unmarshal(raw, &counts); err != nil {
		return nil, err
	}
	return counts, nil
}

var errBadParent = errors.New("lifecycle: unknown partitioned table")

// DropOldPartitions drops whole monthly partitions that end on or before cutoff, for the two managed tables.
func (r Postgres) DropOldPartitions(ctx context.Context, cutoff time.Time) (int, error) {
	total := 0
	for _, parent := range []string{"usage.usage_records", "carbon.calculations"} {
		var n int
		if err := r.Pool.QueryRow(ctx, `SELECT platform.drop_old_partitions($1::regclass, $2::date)`, parent, cutoff).Scan(&n); err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

var _ = errBadParent
