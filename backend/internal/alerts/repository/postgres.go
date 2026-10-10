// Package repository is the PostgreSQL adapter of the alerts domain.
package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quixgit/greenops-reliabilix/backend/internal/alerts/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/audit"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/database"
)

type Postgres struct{ Pool *pgxpool.Pool }

const cols = `id, kind, severity, status, title, detail, project_id::text, object_ref, value::float8, threshold::float8,
	first_seen_at, last_seen_at, acknowledged_by, acknowledged_at, resolved_at, dedupe_key`

func scan(row pgx.Row) (a domain.Alert, err error) {
	var kind, sev, status string
	err = row.Scan(&a.ID, &kind, &sev, &status, &a.Title, &a.Detail, &a.ProjectID, &a.ObjectRef, &a.Value, &a.Threshold,
		&a.FirstSeenAt, &a.LastSeenAt, &a.AcknowledgedBy, &a.AcknowledgedAt, &a.ResolvedAt, &a.DedupeKey)
	a.Kind, a.Severity, a.Status = domain.Kind(kind), domain.Severity(sev), domain.Status(status)
	return
}

func (r Postgres) query(ctx context.Context, tenantID, sql string, args ...any) (out []domain.Alert, err error) {
	out = []domain.Alert{} // an empty list is [], never null: clients parse it as an array
	err = database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, sql, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			a, err := scan(rows)
			if err != nil {
				return err
			}
			out = append(out, a)
		}
		return rows.Err()
	})
	return
}

func (r Postgres) List(ctx context.Context, tenantID, status string, limit int) ([]domain.Alert, error) {
	return r.query(ctx, tenantID, `SELECT `+cols+` FROM alerts.alerts
		WHERE tenant_id = $1 AND ($2 = '' OR status = $2)
		ORDER BY (status = 'resolved'), CASE severity WHEN 'critical' THEN 0 WHEN 'warning' THEN 1 ELSE 2 END, last_seen_at DESC LIMIT $3`,
		tenantID, status, limit)
}

func (r Postgres) Live(ctx context.Context, tenantID string) ([]domain.Alert, error) {
	return r.query(ctx, tenantID, `SELECT `+cols+` FROM alerts.alerts WHERE tenant_id = $1 AND status <> 'resolved'`, tenantID)
}

// Open inserts a new live alert; created is false when one for the same condition appeared concurrently.
func (r Postgres) Open(ctx context.Context, tenantID string, c domain.Condition, at time.Time) (a domain.Alert, created bool, err error) {
	err = database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `INSERT INTO alerts.alerts (tenant_id, kind, severity, dedupe_key, title, detail, project_id, object_ref, value, threshold, first_seen_at, last_seen_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7::uuid,$8,$9,$10,$11,$11)
			ON CONFLICT (tenant_id, dedupe_key) WHERE status <> 'resolved' DO NOTHING
			RETURNING `+cols, tenantID, string(c.Kind), string(c.Severity), c.DedupeKey, c.Title, c.Detail, c.ProjectID, c.ObjectRef, c.Value, c.Threshold, at)
		got, err := scan(row)
		if database.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		a, created = got, true
		return audit.Record(audit.WithTenant(ctx, tenantID), tx, "alert.opened", "alert:"+got.ID, map[string]any{"kind": c.Kind, "severity": c.Severity})
	})
	return
}

func (r Postgres) Refresh(ctx context.Context, tenantID string, c domain.Condition, at time.Time) error {
	return database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE alerts.alerts SET severity = $3, title = $4, detail = $5, value = $6, threshold = $7, last_seen_at = $8
			WHERE tenant_id = $1 AND dedupe_key = $2 AND status <> 'resolved'`,
			tenantID, c.DedupeKey, string(c.Severity), c.Title, c.Detail, c.Value, c.Threshold, at)
		return err
	})
}

func (r Postgres) Resolve(ctx context.Context, tenantID string, ids []string, at time.Time) error {
	if len(ids) == 0 {
		return nil
	}
	return database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE alerts.alerts SET status = 'resolved', resolved_at = $3
			WHERE tenant_id = $1 AND id = ANY($2::uuid[]) AND status <> 'resolved'`, tenantID, ids, at); err != nil {
			return err
		}
		return audit.Record(audit.WithTenant(ctx, tenantID), tx, "alert.resolved", "alerts", map[string]any{"count": len(ids)})
	})
}

// SetStatus applies a person's change atomically; the transition rules are checked on the locked row.
func (r Postgres) SetStatus(ctx context.Context, tenantID, id string, to domain.Status, actor string) (a domain.Alert, err error) {
	err = database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		cur, err := scan(tx.QueryRow(ctx, `SELECT `+cols+` FROM alerts.alerts WHERE tenant_id = $1 AND id = $2 FOR UPDATE`, tenantID, id))
		if database.IsNotFound(err) {
			return domain.ErrNotFound
		}
		if err != nil {
			return err
		}
		if err := domain.Transition(cur.Status, to); err != nil {
			return err
		}
		a, err = scan(tx.QueryRow(ctx, `UPDATE alerts.alerts SET status = $3,
			acknowledged_by = CASE WHEN $3 = 'acknowledged' THEN $4 ELSE acknowledged_by END,
			acknowledged_at = CASE WHEN $3 = 'acknowledged' THEN now() ELSE acknowledged_at END
			WHERE tenant_id = $1 AND id = $2 RETURNING `+cols, tenantID, id, string(to), actor))
		if err != nil {
			return err
		}
		return audit.Record(audit.WithTenant(ctx, tenantID), tx, "alert."+string(to), "alert:"+id, nil)
	})
	return
}
