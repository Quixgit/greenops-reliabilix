package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/audit"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/database"
	"github.com/quixgit/greenops-reliabilix/backend/internal/usage/domain"
)

const importCols = `id::text, project_id::text, vendor, filename, currency::text, status, error, rows_read, rows_imported, rows_skipped, services,
	to_char(from_day, 'YYYY-MM-DD'), to_char(to_day, 'YYYY-MM-DD'), total_cost::float8, warnings, created_by, created_at, finished_at`

func scanImport(row pgx.Row) (domain.Import, error) {
	var i domain.Import
	err := row.Scan(&i.ID, &i.ProjectID, &i.Vendor, &i.Filename, &i.Currency, &i.Status, &i.Error, &i.RowsRead, &i.RowsImported, &i.RowsSkipped, &i.Services,
		&i.From, &i.To, &i.TotalCost, &i.Warnings, &i.CreatedBy, &i.CreatedAt, &i.FinishedAt)
	if database.IsNotFound(err) {
		return i, domain.ErrImportNotFound
	}
	if i.Warnings == nil {
		i.Warnings = []string{}
	}
	return i, err
}

func (r Postgres) CreateImport(ctx context.Context, tenantID string, i domain.Import) (out domain.Import, err error) {
	err = database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		out, err = scanImport(tx.QueryRow(ctx, `INSERT INTO usage.imports (tenant_id, project_id, vendor, filename, currency, created_by)
			VALUES ($1, $2, $3, $4, $5, $6) RETURNING `+importCols, tenantID, i.ProjectID, i.Vendor, i.Filename, i.Currency, i.CreatedBy))
		var pg *pgconn.PgError
		if errors.As(err, &pg) && pg.Code == "23503" { // the project is not this organization's
			return domain.ErrImportProject
		}
		if err != nil {
			return err
		}
		return audit.Record(ctx, tx, "import.created", "import:"+out.ID, map[string]any{"vendor": out.Vendor})
	})
	return
}

func (r Postgres) GetImport(ctx context.Context, tenantID, id string) (out domain.Import, err error) {
	err = database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		out, err = scanImport(tx.QueryRow(ctx, `SELECT `+importCols+` FROM usage.imports WHERE tenant_id = $1 AND id::text = $2`, tenantID, id))
		return err
	})
	return
}

func (r Postgres) ListImports(ctx context.Context, tenantID string, limit int) (out []domain.Import, err error) {
	out = []domain.Import{}
	err = database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT `+importCols+` FROM usage.imports WHERE tenant_id = $1 ORDER BY created_at DESC LIMIT $2`, tenantID, limit)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			i, err := scanImport(rows)
			if err != nil {
				return err
			}
			out = append(out, i)
		}
		return rows.Err()
	})
	return
}

func (r Postgres) FinishImport(ctx context.Context, tenantID, id string, res *domain.ImportResult, failure string) error {
	return database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		if res == nil {
			if len(failure) > 500 {
				failure = failure[:500]
			}
			_, err := tx.Exec(ctx, `UPDATE usage.imports SET status = 'failed', error = $3, finished_at = now() WHERE tenant_id = $1 AND id::text = $2 AND status = 'processing'`, tenantID, id, failure)
			return err
		}
		warnings := res.Warnings
		if warnings == nil {
			warnings = []string{} // the column is NOT NULL: a nil slice would be written as NULL
		}
		var from, to *time.Time
		if !res.From.IsZero() {
			from, to = &res.From, &res.To
		}
		_, err := tx.Exec(ctx, `UPDATE usage.imports SET status = 'done', error = '', rows_read = $3, rows_imported = $4, rows_skipped = $5, services = $6,
			from_day = $7::date, to_day = $8::date, currency = $9, total_cost = $10, warnings = $11, finished_at = now()
			WHERE tenant_id = $1 AND id::text = $2 AND status = 'processing'`,
			tenantID, id, res.RowsRead, res.RowsImported, res.RowsSkipped, res.Services, from, to, res.Currency, res.TotalCost, warnings)
		return err
	})
}

func (r Postgres) MarkImportDeleting(ctx context.Context, tenantID, id string) error {
	return database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE usage.imports SET status = 'deleting' WHERE tenant_id = $1 AND id::text = $2 AND status IN ('done', 'failed')`, tenantID, id)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrImportNotFound // unknown, or still being read, or already going
		}
		return audit.Record(ctx, tx, "import.deleted", "import:"+id, nil)
	})
}

func (r Postgres) DeleteImport(ctx context.Context, tenantID, id string) error {
	return database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		src := "import:" + id
		if _, err := tx.Exec(ctx, `DELETE FROM usage.usage_records WHERE tenant_id = $1 AND source = $2`, tenantID, src); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM usage.tagged_cost WHERE tenant_id = $1 AND source_key = $2`, tenantID, src); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `DELETE FROM usage.imports WHERE tenant_id = $1 AND id::text = $2`, tenantID, id)
		return err
	})
}
