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

const roleCols = `r.id::text, r.name, r.description, r.permissions, r.created_by, r.created_at, r.updated_at,
	(SELECT count(*)::int FROM tenants.memberships m WHERE m.custom_role_id = r.id)`

func scanRole(row pgx.Row) (domain.CustomRole, error) {
	var r domain.CustomRole
	var perms []string
	err := row.Scan(&r.ID, &r.Name, &r.Description, &perms, &r.CreatedBy, &r.CreatedAt, &r.UpdatedAt, &r.Members)
	for _, p := range perms {
		r.Permissions = append(r.Permissions, auth.Permission(p))
	}
	if r.Permissions == nil {
		r.Permissions = []auth.Permission{}
	}
	return r, err
}

func strs(ps []auth.Permission) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = string(p)
	}
	return out
}

// uniqueName turns the unique-name violation into an input error the caller can show.
func uniqueName(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		return domain.ErrInvalidInput
	}
	return err
}

func (r Postgres) CustomRoles(ctx context.Context, tenantID string) (out []domain.CustomRole, err error) {
	out = []domain.CustomRole{}
	err = database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT `+roleCols+` FROM tenants.custom_roles r WHERE r.organization_id = $1 ORDER BY lower(r.name)`, tenantID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			x, err := scanRole(rows)
			if err != nil {
				return err
			}
			out = append(out, x)
		}
		return rows.Err()
	})
	return
}

func (r Postgres) CustomRole(ctx context.Context, tenantID, id string) (x domain.CustomRole, err error) {
	err = database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		var e error
		x, e = scanRole(tx.QueryRow(ctx, `SELECT `+roleCols+` FROM tenants.custom_roles r WHERE r.organization_id = $1 AND r.id = $2::uuid`, tenantID, id))
		if database.IsNotFound(e) {
			return domain.ErrNotFound
		}
		return e
	})
	return
}

func (r Postgres) CreateCustomRole(ctx context.Context, tenantID string, in domain.CustomRole, actor auth.Claims) (x domain.CustomRole, err error) {
	err = database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		var id string
		if e := tx.QueryRow(ctx, `INSERT INTO tenants.custom_roles (organization_id, name, description, permissions, created_by)
			VALUES ($1, $2, $3, $4, $5) RETURNING id::text`, tenantID, in.Name, in.Description, strs(in.Permissions), actor.Subject).Scan(&id); e != nil {
			return uniqueName(e)
		}
		var e error
		if x, e = scanRole(tx.QueryRow(ctx, `SELECT `+roleCols+` FROM tenants.custom_roles r WHERE r.id = $1::uuid`, id)); e != nil {
			return e
		}
		return audit.Record(ctx, tx, "role.created", "role:"+id, map[string]any{"name": in.Name, "permissions": strs(in.Permissions)})
	})
	return
}

func (r Postgres) UpdateCustomRole(ctx context.Context, tenantID, id string, in domain.CustomRole, actor auth.Claims) (x domain.CustomRole, err error) {
	err = database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		tag, e := tx.Exec(ctx, `UPDATE tenants.custom_roles SET name = $3, description = $4, permissions = $5, updated_at = now()
			WHERE organization_id = $1 AND id = $2::uuid`, tenantID, id, in.Name, in.Description, strs(in.Permissions))
		if e != nil {
			return uniqueName(e)
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrNotFound
		}
		if x, e = scanRole(tx.QueryRow(ctx, `SELECT `+roleCols+` FROM tenants.custom_roles r WHERE r.id = $1::uuid`, id)); e != nil {
			return e
		}
		return audit.Record(ctx, tx, "role.updated", "role:"+id, map[string]any{"name": in.Name, "permissions": strs(in.Permissions)})
	})
	return
}

func (r Postgres) DeleteCustomRole(ctx context.Context, tenantID, id string, actor auth.Claims) error {
	return database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		var n int
		if e := tx.QueryRow(ctx, `SELECT count(*)::int FROM tenants.memberships WHERE custom_role_id = $1::uuid`, id).Scan(&n); e != nil {
			return e
		}
		tag, e := tx.Exec(ctx, `DELETE FROM tenants.custom_roles WHERE organization_id = $1 AND id = $2::uuid`, tenantID, id)
		if e != nil {
			return e
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrNotFound
		}
		return audit.Record(ctx, tx, "role.deleted", "role:"+id, map[string]any{"members_reverted": n})
	})
}

// AssignCustomRole sets the member's custom role. The built-in role of the membership becomes "viewer" (the fallback if
// the custom role is deleted); owners are never given a custom role (that would be a demotion that must go through the
// role change rules and the last-owner protection).
func (r Postgres) AssignCustomRole(ctx context.Context, tenantID, memberID, roleID string, actor auth.Claims) (m domain.Member, err error) {
	err = database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		var email, role, sub, userID string
		if e := tx.QueryRow(ctx, `SELECT u.email, m.role, u.auth_sub, m.user_id::text FROM tenants.memberships m JOIN tenants.users u ON u.id = m.user_id
			WHERE m.organization_id = $1 AND m.id = $2::uuid FOR UPDATE OF m`, tenantID, memberID).Scan(&email, &role, &sub, &userID); e != nil {
			if database.IsNotFound(e) {
				return domain.ErrNotFound
			}
			return e
		}
		if auth.Role(role) == auth.RoleOwner {
			return domain.ErrForbiddenChange
		}
		var name string
		if e := tx.QueryRow(ctx, `SELECT name FROM tenants.custom_roles WHERE organization_id = $1 AND id = $2::uuid`, tenantID, roleID).Scan(&name); e != nil {
			if database.IsNotFound(e) {
				return domain.ErrNotFound
			}
			return e
		}
		if _, e := tx.Exec(ctx, `UPDATE tenants.memberships SET role = 'viewer', custom_role_id = $3::uuid WHERE organization_id = $1 AND id = $2::uuid`, tenantID, memberID, roleID); e != nil {
			return e
		}
		m = domain.Member{ID: memberID, UserID: userID, Subject: sub, Email: email, Role: auth.RoleViewer, CustomRole: &domain.RoleRef{ID: roleID, Name: name}}
		return audit.Record(ctx, tx, "member.role_changed", "member:"+memberID, map[string]any{"from": role, "to": "custom:" + name})
	})
	return
}

// MemberPermissions implements auth.PermissionResolver: the permission set of a member's custom role.
func (r Postgres) MemberPermissions(ctx context.Context, subject, tenantID string) ([]auth.Permission, error) {
	var ps []string
	err := r.Pool.QueryRow(ctx, `SELECT tenants.membership_permissions($1, $2::uuid)`, subject, tenantID).Scan(&ps)
	if err != nil {
		return nil, err
	}
	out := make([]auth.Permission, 0, len(ps))
	for _, p := range ps {
		if auth.CustomAssignable(auth.Permission(p)) { // a stored value outside the catalog grants nothing
			out = append(out, auth.Permission(p))
		}
	}
	return out, nil
}
