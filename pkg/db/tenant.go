// Package db holds database helpers shared by services.
package db

import (
	"context"
	"database/sql"
	"errors"
)

// WithTenantTx runs fn in a transaction whose PostgreSQL setting app.tenant_id
// is bound to tenantID (transaction-local). Row Level Security policies read
// that setting, so a missing WHERE tenant_id = ? in application code can never
// leak another tenant's rows. All tenant-scoped queries MUST go through this.
func WithTenantTx(ctx context.Context, d *sql.DB, tenantID string, fn func(*sql.Tx) error) (err error) {
	if tenantID == "" {
		return errors.New("db: empty tenant id")
	}
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	if _, err = tx.ExecContext(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenantID); err != nil {
		return err
	}
	if err = fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
