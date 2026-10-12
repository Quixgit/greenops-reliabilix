package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/audit"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/database"
	"github.com/quixgit/greenops-reliabilix/backend/internal/tenants/domain"
)

const ssoCols = `id::text, domain, connection, COALESCE(jit_role, ''), verified_at, created_at, token`

func scanSSO(row pgx.Row) (domain.SSODomain, error) {
	var d domain.SSODomain
	var token string
	if err := row.Scan(&d.ID, &d.Domain, &d.Connection, &d.JITRole, &d.VerifiedAt, &d.CreatedAt, &token); err != nil {
		if database.IsNotFound(err) {
			return d, domain.ErrNotFound
		}
		return d, err
	}
	d.Verified = d.VerifiedAt != nil
	if !d.Verified {
		d.RecordName, d.RecordValue = domain.ProofRecord(d.Domain, token)
	}
	return d, nil
}

func (r Postgres) ListSSODomains(ctx context.Context, tenantID string) (out []domain.SSODomain, err error) {
	out = []domain.SSODomain{}
	err = database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT `+ssoCols+` FROM tenants.sso_domains WHERE tenant_id = $1 ORDER BY created_at`, tenantID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			d, err := scanSSO(rows)
			if err != nil {
				return err
			}
			out = append(out, d)
		}
		return rows.Err()
	})
	return
}

func (r Postgres) CreateSSODomain(ctx context.Context, tenantID, dom, connection, jitRole, token, actor string) (out domain.SSODomain, err error) {
	var jit *string
	if jitRole != "" {
		jit = &jitRole
	}
	err = database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		out, err = scanSSO(tx.QueryRow(ctx, `INSERT INTO tenants.sso_domains (tenant_id, domain, connection, jit_role, token, created_by) VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (tenant_id, domain) DO UPDATE SET connection = EXCLUDED.connection, jit_role = EXCLUDED.jit_role
			RETURNING `+ssoCols, tenantID, dom, connection, jit, token, actor))
		if err != nil {
			return err
		}
		return audit.Record(ctx, tx, "sso.domain_added", "sso_domain:"+out.ID, map[string]any{"domain": dom, "jit": jitRole != ""})
	})
	return
}

func (r Postgres) SSODomainToken(ctx context.Context, tenantID, id string) (dom, token string, verified bool, err error) {
	err = database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		var at *string
		e := tx.QueryRow(ctx, `SELECT domain, token, verified_at::text FROM tenants.sso_domains WHERE tenant_id = $1 AND id::text = $2`, tenantID, id).Scan(&dom, &token, &at)
		if database.IsNotFound(e) {
			return domain.ErrNotFound
		}
		verified = at != nil
		return e
	})
	return
}

func (r Postgres) MarkSSODomainVerified(ctx context.Context, tenantID, id string) error {
	return database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE tenants.sso_domains SET verified_at = now() WHERE tenant_id = $1 AND id::text = $2 AND verified_at IS NULL`, tenantID, id)
		var pg *pgconn.PgError
		if errors.As(err, &pg) && pg.Code == "23505" { // another organization verified it first
			return domain.ErrDomainTaken
		}
		if err != nil {
			return err
		}
		return audit.Record(ctx, tx, "sso.domain_verified", "sso_domain:"+id, nil)
	})
}

func (r Postgres) DeleteSSODomain(ctx context.Context, tenantID, id string) error {
	return database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `DELETE FROM tenants.sso_domains WHERE tenant_id = $1 AND id::text = $2`, tenantID, id)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrNotFound
		}
		return audit.Record(ctx, tx, "sso.domain_removed", "sso_domain:"+id, nil)
	})
}

func (r Postgres) ConnectionFor(ctx context.Context, email string) (string, error) {
	var c *string
	if err := r.Pool.QueryRow(ctx, `SELECT tenants.sso_connection_for($1)`, email).Scan(&c); err != nil {
		return "", err
	}
	if c == nil {
		return "", nil
	}
	return *c, nil
}

func (r Postgres) ClaimDomain(ctx context.Context, subject, email string) (string, error) {
	var t *string
	if err := r.Pool.QueryRow(ctx, `SELECT tenants.claim_sso_domain($1, $2)::text`, subject, email).Scan(&t); err != nil {
		return "", err
	}
	if t == nil {
		return "", nil
	}
	return *t, nil
}
