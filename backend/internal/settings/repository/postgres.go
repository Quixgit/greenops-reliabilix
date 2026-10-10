// Package repository stores setting values in PostgreSQL (migration 0016).
package repository

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/audit"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/database"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/settings"
	"github.com/quixgit/greenops-reliabilix/backend/internal/settings/domain"
)

type Postgres struct{ Pool *pgxpool.Pool }

var _ domain.Repository = Postgres{}

func scanValues(rows pgx.Rows, err error, platform bool) (map[string]domain.Stored, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]domain.Stored{}
	for rows.Next() {
		var key, source string
		var s domain.Stored
		if platform {
			if err := rows.Scan(&key, &s.Value, &s.UpdatedBy, &s.UpdatedAt); err != nil {
				return nil, err
			}
			s.Source = settings.FromPlatform
		} else {
			if err := rows.Scan(&key, &s.Value, &source, &s.UpdatedBy, &s.UpdatedAt); err != nil {
				return nil, err
			}
			s.Source = settings.Source(source)
		}
		out[key] = s
	}
	return out, rows.Err()
}

func (r Postgres) PlatformValues(ctx context.Context) (map[string]domain.Stored, error) {
	rows, err := r.Pool.Query(ctx, `SELECT key, value, updated_by, updated_at FROM settings.platform_values`)
	return scanValues(rows, err, true)
}

func (r Postgres) TenantValues(ctx context.Context, tenantID string) (out map[string]domain.Stored, err error) {
	err = database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		rows, qerr := tx.Query(ctx, `SELECT key, value, source, updated_by, updated_at FROM settings.tenant_values WHERE tenant_id = $1`, tenantID)
		out, qerr = scanValues(rows, qerr, false)
		return qerr
	})
	return
}

func nilIfEmpty(b json.RawMessage) any {
	if len(b) == 0 {
		return nil
	}
	return []byte(b)
}

func (r Postgres) ApplyTenant(ctx context.Context, tenantID, actor string, changes []domain.Change) error {
	return database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		for _, c := range changes {
			if c.New == nil {
				if _, err := tx.Exec(ctx, `DELETE FROM settings.tenant_values WHERE tenant_id = $1 AND key = $2`, tenantID, c.Key); err != nil {
					return err
				}
			} else if _, err := tx.Exec(ctx, `INSERT INTO settings.tenant_values (tenant_id, key, value, source, updated_by) VALUES ($1,$2,$3,'tenant',$4)
				ON CONFLICT (tenant_id, key) DO UPDATE SET value = EXCLUDED.value, updated_by = EXCLUDED.updated_by, updated_at = now()`,
				tenantID, c.Key, []byte(c.New), actor); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO settings.history (tenant_id, key, old_value, new_value, actor, via) VALUES ($1,$2,$3,$4,$5,'tenant')`,
				tenantID, c.Key, nilIfEmpty(c.Old), nilIfEmpty(c.New), actor); err != nil {
				return err
			}
			action := "setting.changed"
			if c.New == nil {
				action = "setting.reset"
			}
			if err := audit.Record(ctx, tx, action, "setting:"+c.Key, nil); err != nil {
				return err
			}
		}
		return nil
	})
}

func scanHistory(rows pgx.Rows, err error) ([]settings.HistoryEntry, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []settings.HistoryEntry{}
	for rows.Next() {
		var h settings.HistoryEntry
		if err := rows.Scan(&h.At, &h.TenantID, &h.Key, &h.Old, &h.New, &h.Actor, &h.Via); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func (r Postgres) TenantHistory(ctx context.Context, tenantID string, limit int) (out []settings.HistoryEntry, err error) {
	err = database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		rows, qerr := tx.Query(ctx, `SELECT at, tenant_id::text, key, old_value, new_value, actor, via FROM settings.history ORDER BY id DESC LIMIT $1`, limit)
		out, qerr = scanHistory(rows, qerr)
		return qerr
	})
	return
}

func (r Postgres) OperatorSetPlatform(ctx context.Context, actor, key string, value json.RawMessage, reset bool) error {
	_, err := r.Pool.Exec(ctx, `SELECT settings.operator_set_platform($1, $2, $3::jsonb, $4)`, actor, key, nilIfEmpty(value), reset)
	return err
}

func (r Postgres) OperatorTenantValues(ctx context.Context, tenantID string) (map[string]domain.Stored, error) {
	rows, err := r.Pool.Query(ctx, `SELECT key, value, source, updated_by, updated_at FROM settings.operator_tenant_values($1::uuid)`, tenantID)
	return scanValues(rows, err, false)
}

func (r Postgres) OperatorSetTenant(ctx context.Context, actor, tenantID, key string, value json.RawMessage, reset bool) (found bool, err error) {
	err = r.Pool.QueryRow(ctx, `SELECT settings.operator_set_tenant($1, $2::uuid, $3, $4::jsonb, $5)`, actor, tenantID, key, nilIfEmpty(value), reset).Scan(&found)
	return
}

func (r Postgres) OperatorHistory(ctx context.Context, tenantID *string, limit int) ([]settings.HistoryEntry, error) {
	rows, err := r.Pool.Query(ctx, `SELECT at, tenant_id::text, key, old_value, new_value, actor, via FROM settings.operator_history($1::uuid, $2)`, tenantID, limit)
	return scanHistory(rows, err)
}
