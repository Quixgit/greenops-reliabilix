// Package repository is the PostgreSQL adapter of the project domain.
package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/audit"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/database"
	"github.com/quixgit/greenops-reliabilix/backend/internal/projects/domain"
)

type Postgres struct{ Pool *pgxpool.Pool }

func (r Postgres) List(ctx context.Context, tenantID string) ([]domain.Project, error) {
	out := []domain.Project{}
	err := database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT id, tenant_id, name, functional_unit, created_at
			FROM projects.projects WHERE tenant_id = $1 ORDER BY created_at DESC LIMIT 500`, tenantID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var p domain.Project
			if err := rows.Scan(&p.ID, &p.TenantID, &p.Name, &p.FunctionalUnit, &p.CreatedAt); err != nil {
				return err
			}
			out = append(out, p)
		}
		return rows.Err()
	})
	return out, err
}

func (r Postgres) Create(ctx context.Context, p domain.Project) (domain.Project, error) {
	err := database.WithTenantTx(ctx, r.Pool, p.TenantID, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `INSERT INTO projects.projects (tenant_id, name, functional_unit)
			VALUES ($1, $2, $3) RETURNING id, created_at`, p.TenantID, p.Name, p.FunctionalUnit).Scan(&p.ID, &p.CreatedAt); err != nil {
			return err
		}
		return audit.Record(ctx, tx, "project.created", "project:"+p.ID, map[string]any{"name": p.Name})
	})
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		return domain.Project{}, domain.ErrDuplicate
	}
	return p, err
}
