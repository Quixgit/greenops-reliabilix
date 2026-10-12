package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/audit"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/database"
	"github.com/quixgit/greenops-reliabilix/backend/internal/tenants/domain"
)

const scimCols = `id::text, external_id, email, display_name, role, active, user_id IS NOT NULL, created_at, updated_at`

func scanSCIM(row pgx.Row) (domain.SCIMUser, error) {
	var u domain.SCIMUser
	var role string
	err := row.Scan(&u.ID, &u.ExternalID, &u.Email, &u.DisplayName, &role, &u.Active, &u.Joined, &u.CreatedAt, &u.UpdatedAt)
	u.Role = auth.Role(role)
	if database.IsNotFound(err) {
		return u, domain.ErrNotFound
	}
	return u, err
}

func (r Postgres) ListSCIMUsers(ctx context.Context, tenantID, email string, start, count int) (out []domain.SCIMUser, total int, err error) {
	out = []domain.SCIMUser{}
	err = database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM tenants.scim_users WHERE organization_id = $1 AND ($2 = '' OR lower(email) = lower($2))`, tenantID, email).Scan(&total); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT `+scimCols+` FROM tenants.scim_users WHERE organization_id = $1 AND ($2 = '' OR lower(email) = lower($2))
			ORDER BY created_at, id OFFSET $3 LIMIT $4`, tenantID, email, start-1, count)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			u, err := scanSCIM(rows)
			if err != nil {
				return err
			}
			out = append(out, u)
		}
		return rows.Err()
	})
	return
}

func (r Postgres) GetSCIMUser(ctx context.Context, tenantID, id string) (u domain.SCIMUser, err error) {
	err = database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		u, err = scanSCIM(tx.QueryRow(ctx, `SELECT `+scimCols+` FROM tenants.scim_users WHERE organization_id = $1 AND id::text = $2`, tenantID, id))
		return err
	})
	return
}

// CreateSCIMUser records the person. Someone who already is a member (invited earlier) is linked, and keeps their role.
func (r Postgres) CreateSCIMUser(ctx context.Context, tenantID string, u domain.SCIMUser) (out domain.SCIMUser, err error) {
	err = database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		var userID *string
		var memberRole string
		e := tx.QueryRow(ctx, `SELECT m.user_id::text, m.role FROM tenants.memberships m JOIN tenants.users x ON x.id = m.user_id
			WHERE m.organization_id = $1 AND lower(x.email) = lower($2) LIMIT 1`, tenantID, u.Email).Scan(&userID, &memberRole)
		if e != nil && !database.IsNotFound(e) {
			return e
		}
		role := u.Role
		if userID != nil {
			role = auth.Role(memberRole)
			if !domain.ValidSCIMRole(role) {
				role = auth.RoleAdmin // an owner or custom role: kept as it is in the membership, shown as admin here
			}
		}
		out, err = scanSCIM(tx.QueryRow(ctx, `INSERT INTO tenants.scim_users (organization_id, external_id, email, display_name, role, active, user_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7::uuid) RETURNING `+scimCols, tenantID, u.ExternalID, u.Email, u.DisplayName, string(role), u.Active, userID))
		var pg *pgconn.PgError
		if errors.As(err, &pg) && pg.Code == "23505" {
			return domain.ErrConflict
		}
		if err != nil {
			return err
		}
		return audit.Record(ctx, tx, "scim.user_provisioned", "scim_user:"+out.ID, map[string]any{"role": out.Role, "linked": userID != nil})
	})
	return
}

func (r Postgres) UpdateSCIMUser(ctx context.Context, tenantID string, u domain.SCIMUser) (out domain.SCIMUser, err error) {
	err = database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		cur, userID, err := lockSCIM(ctx, tx, tenantID, u.ID)
		if err != nil {
			return err
		}
		if userID != nil { // joined: the membership follows the record
			if !u.Active {
				if err := removeMembership(ctx, tx, tenantID, *userID); err != nil {
					return err
				}
				userID = nil
			} else if u.Role != cur.Role {
				if err := changeMembershipRole(ctx, tx, tenantID, *userID, u.Role); err != nil {
					return err
				}
			}
			u.Email = cur.Email // the sign-in identity does not change under a member
		}
		out, err = scanSCIM(tx.QueryRow(ctx, `UPDATE tenants.scim_users SET external_id = $3, email = $4, display_name = $5, role = $6, active = $7,
			user_id = $8::uuid, updated_at = now() WHERE organization_id = $1 AND id::text = $2 RETURNING `+scimCols,
			tenantID, u.ID, u.ExternalID, u.Email, u.DisplayName, string(u.Role), u.Active, userID))
		var pg *pgconn.PgError
		if errors.As(err, &pg) && pg.Code == "23505" {
			return domain.ErrConflict
		}
		if err != nil {
			return err
		}
		return audit.Record(ctx, tx, "scim.user_updated", "scim_user:"+out.ID, map[string]any{"role": out.Role, "active": out.Active})
	})
	return
}

func (r Postgres) DeleteSCIMUser(ctx context.Context, tenantID, id string) error {
	return database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		_, userID, err := lockSCIM(ctx, tx, tenantID, id)
		if err != nil {
			return err
		}
		if userID != nil {
			if err := removeMembership(ctx, tx, tenantID, *userID); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `DELETE FROM tenants.scim_users WHERE organization_id = $1 AND id::text = $2`, tenantID, id); err != nil {
			return err
		}
		return audit.Record(ctx, tx, "scim.user_deprovisioned", "scim_user:"+id, nil)
	})
}

func (r Postgres) ClaimProvisioned(ctx context.Context, subject, email string) (string, error) {
	var t *string
	if err := r.Pool.QueryRow(ctx, `SELECT tenants.claim_provisioned($1, $2)::text`, subject, email).Scan(&t); err != nil {
		return "", err
	}
	if t == nil {
		return "", nil
	}
	return *t, nil
}

// lockSCIM reads a record with a row lock and the linked user id.
func lockSCIM(ctx context.Context, tx pgx.Tx, tenantID, id string) (domain.SCIMUser, *string, error) {
	var userID *string
	u, err := scanSCIM(tx.QueryRow(ctx, `SELECT `+scimCols+` FROM tenants.scim_users WHERE organization_id = $1 AND id::text = $2 FOR UPDATE`, tenantID, id))
	if err != nil {
		return u, nil, err
	}
	if u.Joined {
		if err := tx.QueryRow(ctx, `SELECT user_id::text FROM tenants.scim_users WHERE organization_id = $1 AND id::text = $2`, tenantID, id).Scan(&userID); err != nil {
			return u, nil, err
		}
	}
	return u, userID, nil
}

func memberRole(ctx context.Context, tx pgx.Tx, tenantID, userID string) (auth.Role, int, error) {
	var role string
	err := tx.QueryRow(ctx, `SELECT role FROM tenants.memberships WHERE organization_id = $1 AND user_id::text = $2 FOR UPDATE`, tenantID, userID).Scan(&role)
	if database.IsNotFound(err) {
		return "", 0, domain.ErrNotFound
	}
	if err != nil {
		return "", 0, err
	}
	var owners int
	err = tx.QueryRow(ctx, `SELECT count(*) FROM tenants.memberships WHERE organization_id = $1 AND role = 'owner'`, tenantID).Scan(&owners)
	return auth.Role(role), owners, err
}

// removeMembership ends a membership the identity provider took away. Owners are never touched by it.
func removeMembership(ctx context.Context, tx pgx.Tx, tenantID, userID string) error {
	role, owners, err := memberRole(ctx, tx, tenantID, userID)
	if errors.Is(err, domain.ErrNotFound) {
		return nil // already gone
	}
	if err != nil {
		return err
	}
	if err := domain.CheckRemoval(auth.RoleAdmin, role, owners); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM tenants.memberships WHERE organization_id = $1 AND user_id::text = $2`, tenantID, userID); err != nil {
		return err
	}
	return audit.Record(ctx, tx, "member.removed", "user:"+userID, map[string]any{"role": role, "by": "scim"})
}

func changeMembershipRole(ctx context.Context, tx pgx.Tx, tenantID, userID string, to auth.Role) error {
	role, owners, err := memberRole(ctx, tx, tenantID, userID)
	if errors.Is(err, domain.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if role == auth.RoleOwner {
		return nil // provisioning never changes an owner's role
	}
	if err := domain.CheckRoleChange(auth.RoleAdmin, role, to, owners); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE tenants.memberships SET role = $3, custom_role_id = NULL WHERE organization_id = $1 AND user_id::text = $2`, tenantID, userID, string(to)); err != nil {
		return err
	}
	return audit.Record(ctx, tx, "member.role_changed", "user:"+userID, map[string]any{"from": role, "to": to, "by": "scim"})
}
