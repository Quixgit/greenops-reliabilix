// Package repository is the PostgreSQL adapter of the tenants domain. Identity resolution uses narrow
// SECURITY DEFINER functions (no tenant is known yet); everything else is tenant-scoped and RLS-bound.
package repository

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/audit"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/database"
	"github.com/quixgit/greenops-reliabilix/backend/internal/tenants/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/tenants/repository/db"
)

type Postgres struct{ Pool *pgxpool.Pool }

// Resolve implements auth.Resolver's lookup: the DB is authoritative for tenant and role.
func (r Postgres) Resolve(ctx context.Context, subject, wantedTenant string) (string, auth.Role, error) {
	var wanted *string
	if wantedTenant != "" {
		wanted = &wantedTenant
	}
	var tenant, role string
	err := r.Pool.QueryRow(ctx, `SELECT tenant_id::text, role FROM tenants.resolve_membership($1, $2::uuid)`, subject, wanted).Scan(&tenant, &role)
	if database.IsNotFound(err) {
		return "", "", auth.ErrNoMembership
	}
	if err != nil {
		// a malformed X-Tenant-ID is a client error, not a server failure
		var pg *pgconn.PgError
		if errors.As(err, &pg) && pg.Code == "22P02" {
			return "", "", auth.ErrNoMembership
		}
		return "", "", err
	}
	return tenant, auth.Role(role), nil
}

// LookupAPIKey authenticates a machine credential by hash; revoked/unknown keys return ErrUnauthenticated.
func (r Postgres) LookupAPIKey(ctx context.Context, hash string) (string, auth.Role, string, error) {
	var tenant, role, id string
	err := r.Pool.QueryRow(ctx, `SELECT tenant_id::text, role, key_id::text FROM tenants.resolve_api_key($1)`, hash).Scan(&tenant, &role, &id)
	if database.IsNotFound(err) {
		return "", "", "", auth.ErrUnauthenticated
	}
	return tenant, auth.Role(role), id, err
}

func (r Postgres) Onboard(ctx context.Context, subject, email, orgName string) (string, error) {
	return db.New(r.Pool).Onboard(ctx, db.OnboardParams{Sub: subject, Email: email, OrgName: orgName})
}

func (r Postgres) AcceptInvitation(ctx context.Context, tokenHash, subject, email string) (string, error) {
	t, err := db.New(r.Pool).AcceptInvitation(ctx, db.AcceptInvitationParams{TokenHash: tokenHash, Sub: subject, Email: email})
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "P0001" {
		return "", domain.ErrInvalidInvitation
	}
	return t, err
}

func (r Postgres) Organization(ctx context.Context, tenantID string) (o domain.Organization, err error) {
	err = database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		x, err := db.New(tx).GetOrganization(ctx, tenantID)
		if database.IsNotFound(err) {
			return domain.ErrNotFound
		}
		o = domain.Organization{ID: x.ID, Name: x.Name, Plan: x.Plan, CreatedAt: x.CreatedAt}
		return err
	})
	return
}

func (r Postgres) Members(ctx context.Context, tenantID string) ([]domain.Member, error) {
	out := []domain.Member{}
	err := database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		rows, err := db.New(tx).ListMembers(ctx, tenantID)
		for _, x := range rows {
			out = append(out, domain.Member{ID: x.ID, UserID: x.UserID, Subject: x.AuthSub, Email: x.Email, Role: auth.Role(x.Role), CreatedAt: x.CreatedAt})
		}
		return err
	})
	return out, err
}

func (r Postgres) ChangeRole(ctx context.Context, tenantID, memberID string, newRole auth.Role, actor auth.Claims) (m domain.Member, err error) {
	err = database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		q := db.New(tx)
		x, err := q.GetMember(ctx, db.GetMemberParams{TenantID: tenantID, ID: memberID}) // row lock: serializes concurrent role changes
		if database.IsNotFound(err) {
			return domain.ErrNotFound
		}
		if err != nil {
			return err
		}
		owners, err := q.CountOwners(ctx, tenantID)
		if err != nil {
			return err
		}
		if err := domain.CheckRoleChange(actor.Role, auth.Role(x.Role), newRole, int(owners)); err != nil {
			return err
		}
		if err := q.UpdateMemberRole(ctx, db.UpdateMemberRoleParams{TenantID: tenantID, ID: memberID, Role: string(newRole)}); err != nil {
			return err
		}
		m = domain.Member{ID: x.ID, UserID: x.UserID, Subject: x.AuthSub, Email: x.Email, Role: newRole, CreatedAt: x.CreatedAt}
		return audit.Record(ctx, tx, "member.role_changed", "member:"+memberID, map[string]any{"from": x.Role, "to": newRole})
	})
	return
}

func (r Postgres) RemoveMember(ctx context.Context, tenantID, memberID string, actor auth.Claims) error {
	return database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		q := db.New(tx)
		x, err := q.GetMember(ctx, db.GetMemberParams{TenantID: tenantID, ID: memberID})
		if database.IsNotFound(err) {
			return domain.ErrNotFound
		}
		if err != nil {
			return err
		}
		owners, err := q.CountOwners(ctx, tenantID)
		if err != nil {
			return err
		}
		if err := domain.CheckRemoval(actor.Role, auth.Role(x.Role), int(owners)); err != nil {
			return err
		}
		if err := q.DeleteMember(ctx, db.DeleteMemberParams{TenantID: tenantID, ID: memberID}); err != nil {
			return err
		}
		return audit.Record(ctx, tx, "member.removed", "member:"+memberID, map[string]any{"role": x.Role})
	})
}

func (r Postgres) CreateInvitation(ctx context.Context, tenantID string, inv domain.Invitation, tokenHash string) (out domain.Invitation, err error) {
	err = database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		x, err := db.New(tx).CreateInvitation(ctx, db.CreateInvitationParams{TenantID: tenantID, Email: inv.Email, Role: string(inv.Role),
			TokenHash: tokenHash, CreatedBy: inv.CreatedBy, ExpiresAt: inv.ExpiresAt})
		if err != nil {
			return err
		}
		out = domain.Invitation{ID: x.ID, Email: x.Email, Role: auth.Role(x.Role), CreatedBy: inv.CreatedBy, ExpiresAt: x.ExpiresAt, CreatedAt: x.CreatedAt}
		return audit.Record(ctx, tx, "invitation.created", "invitation:"+x.ID, map[string]any{"role": inv.Role})
	})
	return
}

func (r Postgres) Invitations(ctx context.Context, tenantID string) ([]domain.Invitation, error) {
	out := []domain.Invitation{}
	err := database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		rows, err := db.New(tx).ListInvitations(ctx, tenantID)
		for _, x := range rows {
			out = append(out, domain.Invitation{ID: x.ID, Email: x.Email, Role: auth.Role(x.Role), CreatedBy: x.CreatedBy, ExpiresAt: x.ExpiresAt, AcceptedAt: x.AcceptedAt, CreatedAt: x.CreatedAt})
		}
		return err
	})
	return out, err
}

func (r Postgres) CreateAPIKey(ctx context.Context, tenantID string, k domain.APIKey, hash string) (out domain.APIKey, err error) {
	err = database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		x, err := db.New(tx).CreateAPIKey(ctx, db.CreateAPIKeyParams{TenantID: tenantID, Name: k.Name, Prefix: k.Prefix, KeyHash: hash, Role: string(k.Role), CreatedBy: k.CreatedBy})
		if err != nil {
			return err
		}
		out = domain.APIKey{ID: x.ID, Name: x.Name, Prefix: x.Prefix, Role: auth.Role(x.Role), CreatedBy: k.CreatedBy, CreatedAt: x.CreatedAt}
		return audit.Record(ctx, tx, "api_key.created", "api_key:"+x.ID, map[string]any{"name": x.Name, "role": x.Role})
	})
	return
}

func (r Postgres) APIKeys(ctx context.Context, tenantID string) ([]domain.APIKey, error) {
	out := []domain.APIKey{}
	err := database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		rows, err := db.New(tx).ListAPIKeys(ctx, tenantID)
		for _, x := range rows {
			out = append(out, domain.APIKey{ID: x.ID, Name: x.Name, Prefix: x.Prefix, Role: auth.Role(x.Role), CreatedBy: x.CreatedBy, CreatedAt: x.CreatedAt, LastUsedAt: x.LastUsedAt, RevokedAt: x.RevokedAt})
		}
		return err
	})
	return out, err
}

func (r Postgres) RevokeAPIKey(ctx context.Context, tenantID, id string) error {
	return database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		n, err := db.New(tx).RevokeAPIKey(ctx, db.RevokeAPIKeyParams{TenantID: tenantID, ID: id})
		if err != nil {
			return err
		}
		if n == 0 {
			return domain.ErrNotFound
		}
		return audit.Record(ctx, tx, "api_key.revoked", "api_key:"+id, nil)
	})
}

func (r Postgres) AuditLog(ctx context.Context, tenantID, action string, limit int) ([]domain.AuditEntry, error) {
	out := []domain.AuditEntry{}
	err := database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		rows, err := db.New(tx).ListAuditLog(ctx, db.ListAuditLogParams{TenantID: tenantID, Action: action, RowLimit: int32(limit)}) //nolint:gosec // limit clamped to <= 200 by the handler
		for _, x := range rows {
			meta := x.Metadata
			if len(meta) == 0 {
				meta = json.RawMessage("{}")
			}
			out = append(out, domain.AuditEntry{ID: x.ID, Actor: x.ActorUserID, Action: x.Action, Target: x.Target, RequestID: x.RequestID, Metadata: meta, CreatedAt: x.CreatedAt})
		}
		return err
	})
	return out, err
}
