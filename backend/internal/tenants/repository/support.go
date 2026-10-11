package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/audit"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/database"
	"github.com/quixgit/greenops-reliabilix/backend/internal/tenants/domain"
)

// Grant opens support access. A grant still open is closed first, so there is never more than one.
func (r Postgres) Grant(ctx context.Context, tenantID, by string, hours int, note string) (g domain.SupportGrant, err error) {
	if hours < domain.MinSupportHours || hours > domain.MaxSupportHours || len(note) > 300 {
		return g, domain.ErrInvalidInput
	}
	err = database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE tenants.support_grants SET revoked_at = now() WHERE tenant_id = $1 AND revoked_at IS NULL AND expires_at > now()`, tenantID); err != nil {
			return err
		}
		exp := time.Now().UTC().Add(time.Duration(hours) * time.Hour)
		err := tx.QueryRow(ctx, `INSERT INTO tenants.support_grants (tenant_id, granted_by, expires_at, note) VALUES ($1, $2, $3, $4)
			RETURNING id::text, granted_by, granted_at, expires_at, revoked_at, note`, tenantID, by, exp, note).
			Scan(&g.ID, &g.GrantedBy, &g.GrantedAt, &g.ExpiresAt, &g.RevokedAt, &g.Note)
		if err != nil {
			return err
		}
		g.Active = true
		return audit.Record(ctx, tx, "support.access_granted", "organization:"+tenantID, map[string]any{"hours": hours, "grant": g.ID})
	})
	return g, err
}

// Revoke closes the open grant now.
func (r Postgres) Revoke(ctx context.Context, tenantID string) (closed bool, err error) {
	err = database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE tenants.support_grants SET revoked_at = now() WHERE tenant_id = $1 AND revoked_at IS NULL AND expires_at > now()`, tenantID)
		if err != nil {
			return err
		}
		closed = tag.RowsAffected() > 0
		if !closed {
			return nil
		}
		return audit.Record(ctx, tx, "support.access_revoked", "organization:"+tenantID, nil)
	})
	return closed, err
}

// History returns the newest grants and the newest looks of an organization.
func (r Postgres) History(ctx context.Context, tenantID string, grants, views int) (gs []domain.SupportGrant, vs []domain.SupportView, err error) {
	gs, vs = []domain.SupportGrant{}, []domain.SupportView{}
	err = database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT id::text, granted_by, granted_at, expires_at, revoked_at, note, (revoked_at IS NULL AND expires_at > now())
			FROM tenants.support_grants WHERE tenant_id = $1 ORDER BY granted_at DESC LIMIT $2`, tenantID, grants)
		if err != nil {
			return err
		}
		for rows.Next() {
			var g domain.SupportGrant
			if err := rows.Scan(&g.ID, &g.GrantedBy, &g.GrantedAt, &g.ExpiresAt, &g.RevokedAt, &g.Note, &g.Active); err != nil {
				rows.Close()
				return err
			}
			gs = append(gs, g)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		vr, err := tx.Query(ctx, `SELECT operator, what, at FROM tenants.support_views WHERE tenant_id = $1 ORDER BY at DESC, id DESC LIMIT $2`, tenantID, views)
		if err != nil {
			return err
		}
		defer vr.Close()
		for vr.Next() {
			var v domain.SupportView
			if err := vr.Scan(&v.Operator, &v.What, &v.At); err != nil {
				return err
			}
			vs = append(vs, v)
		}
		return vr.Err()
	})
	return gs, vs, err
}
