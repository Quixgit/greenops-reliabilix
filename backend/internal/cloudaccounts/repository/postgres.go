// Package repository is the PostgreSQL adapter for cloud connections.
package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/audit"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/database"
)

type Postgres struct{ Pool *pgxpool.Pool }

func (r Postgres) List(ctx context.Context, tenantID string) ([]domain.Connection, error) {
	out := []domain.Connection{}
	err := database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT id, tenant_id, project_id, provider, account_ref, credential_ref, last_sync_at, sync_status
			FROM cloudaccounts.connections WHERE tenant_id = $1 ORDER BY created_at DESC LIMIT 500`, tenantID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var c domain.Connection
			if err := rows.Scan(&c.ID, &c.TenantID, &c.ProjectID, &c.Provider, &c.AccountRef, &c.CredentialRef, &c.LastSyncAt, &c.SyncStatus); err != nil {
				return err
			}
			out = append(out, c)
		}
		return rows.Err()
	})
	return out, err
}

func (r Postgres) Create(ctx context.Context, c domain.Connection) (domain.Connection, error) {
	err := database.WithTenantTx(ctx, r.Pool, c.TenantID, func(tx pgx.Tx) error {
		c.SyncStatus = domain.StatusPending
		if err := tx.QueryRow(ctx, `INSERT INTO cloudaccounts.connections (tenant_id, project_id, provider, account_ref, credential_ref)
			VALUES ($1,$2,$3,$4,$5) RETURNING id`, c.TenantID, c.ProjectID, c.Provider, c.AccountRef, c.CredentialRef).Scan(&c.ID); err != nil {
			return err
		}
		return audit.Record(ctx, tx, "cloud_connection.created", "connection:"+c.ID, map[string]any{"provider": c.Provider})
	})
	return c, err
}
