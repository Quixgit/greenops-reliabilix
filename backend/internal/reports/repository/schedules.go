package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/audit"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/database"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/methodology"
	"github.com/quixgit/greenops-reliabilix/backend/internal/reports/domain"
)

const scheduleCols = `id::text, project_id::text, kind, format, cadence, recipients, enabled, next_run_at, last_run_at, created_by, created_at`

func scanSchedule(row pgx.Row) (s domain.Schedule, err error) {
	var kind, format, cadence string
	err = row.Scan(&s.ID, &s.ProjectID, &kind, &format, &cadence, &s.Recipients, &s.Enabled, &s.NextRunAt, &s.LastRunAt, &s.CreatedBy, &s.CreatedAt)
	s.Kind, s.Format, s.Cadence = domain.Kind(kind), domain.Format(format), domain.Cadence(cadence)
	if s.Recipients == nil {
		s.Recipients = []string{}
	}
	return
}

// Postgres implements domain.ScheduleRepository next to domain.Repository.

func (r Postgres) CreateSchedule(ctx context.Context, tenantID string, s domain.Schedule) (out domain.Schedule, err error) {
	err = database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		out, err = scanSchedule(tx.QueryRow(ctx, `INSERT INTO reports.schedules (tenant_id, project_id, kind, format, cadence, recipients, next_run_at, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING `+scheduleCols,
			tenantID, s.ProjectID, string(s.Kind), string(s.Format), string(s.Cadence), s.Recipients, s.NextRunAt, s.CreatedBy))
		if err != nil {
			return err
		}
		return audit.Record(ctx, tx, "report.schedule_created", "report_schedule:"+out.ID, map[string]any{"kind": s.Kind, "cadence": s.Cadence, "recipients": len(s.Recipients)})
	})
	return
}

func (r Postgres) ListSchedules(ctx context.Context, tenantID string) ([]domain.Schedule, error) {
	out := []domain.Schedule{}
	err := database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT `+scheduleCols+` FROM reports.schedules ORDER BY created_at DESC LIMIT 100`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			s, err := scanSchedule(rows)
			if err != nil {
				return err
			}
			out = append(out, s)
		}
		return rows.Err()
	})
	return out, err
}

func (r Postgres) SetScheduleEnabled(ctx context.Context, tenantID, id string, enabled bool, next time.Time) (out domain.Schedule, err error) {
	err = database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		out, err = scanSchedule(tx.QueryRow(ctx, `UPDATE reports.schedules SET enabled = $2, next_run_at = CASE WHEN $2 THEN $3 ELSE next_run_at END
			WHERE id = $1 RETURNING `+scheduleCols, id, enabled, next))
		if database.IsNotFound(err) {
			return domain.ErrNotFound
		}
		if err != nil {
			return err
		}
		action := "report.schedule_paused"
		if enabled {
			action = "report.schedule_resumed"
		}
		return audit.Record(ctx, tx, action, "report_schedule:"+id, nil)
	})
	return
}

func (r Postgres) DeleteSchedule(ctx context.Context, tenantID, id string) error {
	return database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `DELETE FROM reports.schedules WHERE id = $1`, id)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrNotFound
		}
		return audit.Record(ctx, tx, "report.schedule_deleted", "report_schedule:"+id, nil)
	})
}

func (r Postgres) DueSchedules(ctx context.Context, now time.Time) ([][2]string, error) {
	rows, err := r.Pool.Query(ctx, `SELECT tenant_id::text, id::text FROM platform.due_report_schedules($1)`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out [][2]string
	for rows.Next() {
		var p [2]string
		if err := rows.Scan(&p[0], &p[1]); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r Postgres) ClaimSchedule(ctx context.Context, tenantID, id string, now time.Time) (s domain.Schedule, ok bool, err error) {
	err = database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		var cadence string
		if err := tx.QueryRow(ctx, `SELECT cadence FROM reports.schedules WHERE id = $1 AND enabled AND next_run_at <= $2 FOR UPDATE`, id, now).Scan(&cadence); err != nil {
			if database.IsNotFound(err) {
				return nil
			}
			return err
		}
		s, err = scanSchedule(tx.QueryRow(ctx, `UPDATE reports.schedules SET last_run_at = $2, next_run_at = $3 WHERE id = $1 RETURNING `+scheduleCols,
			id, now, domain.NextRun(domain.Cadence(cadence), now)))
		ok = err == nil
		return err
	})
	return
}

func (r Postgres) LinkSchedule(ctx context.Context, tenantID, reportID, scheduleID string) error {
	return database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO reports.schedule_runs (report_id, tenant_id, schedule_id) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`, reportID, tenantID, scheduleID)
		return err
	})
}

func (r Postgres) RunRecipients(ctx context.Context, tenantID, reportID string) (rec []string, found bool, err error) {
	err = database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `SELECT s.recipients FROM reports.schedule_runs x JOIN reports.schedules s ON s.id = x.schedule_id WHERE x.report_id = $1`, reportID).Scan(&rec)
		if database.IsNotFound(err) {
			return nil
		}
		found = err == nil
		return err
	})
	return
}

func (r Postgres) MemberEmails(ctx context.Context, tenantID string, emails []string) (out []string, err error) {
	if len(emails) == 0 {
		return []string{}, nil
	}
	err = database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT platform.member_emails($1::uuid, $2::text[])`, tenantID, emails).Scan(&out)
	})
	return
}

// Headline sums the window and the equally long one before it: spend in the currency that carries most of it, carbon of the current
// methodology, and the service that cost most in the window.
func (r Postgres) Headline(ctx context.Context, tenantID string, project *string, from, to time.Time) (h domain.Headline, err error) {
	prev := from.Add(-to.Sub(from))
	err = database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		// the currency with the most spend in the window is the one reported; mixed currencies are never added together
		e := tx.QueryRow(ctx, `SELECT currency::text, coalesce(sum(effective_cost) FILTER (WHERE recorded_at >= $3), 0)::float8,
			coalesce(sum(effective_cost) FILTER (WHERE recorded_at < $3), 0)::float8
			FROM usage.usage_records WHERE tenant_id = $1 AND ($2::uuid IS NULL OR project_id = $2) AND recorded_at >= $4 AND recorded_at < $5
			GROUP BY currency ORDER BY sum(effective_cost) FILTER (WHERE recorded_at >= $3) DESC NULLS LAST LIMIT 1`,
			tenantID, project, from, prev, to).Scan(&h.Currency, &h.Cost, &h.PrevCost)
		if e != nil && !database.IsNotFound(e) {
			return e
		}
		if e = tx.QueryRow(ctx, `SELECT coalesce(sum(carbon_kg_co2e) FILTER (WHERE period_start >= $3), 0)::float8,
			coalesce(sum(carbon_kg_co2e) FILTER (WHERE period_start < $3), 0)::float8
			FROM carbon.calculations WHERE tenant_id = $1 AND ($2::uuid IS NULL OR project_id = $2) AND methodology_version = $6
			AND period_start >= $4 AND period_start < $5`, tenantID, project, from, prev, to, methodology.Version).Scan(&h.CarbonKg, &h.PrevCarbonKg); e != nil {
			return e
		}
		e = tx.QueryRow(ctx, `SELECT service_name, sum(effective_cost)::float8 FROM usage.usage_records
			WHERE tenant_id = $1 AND ($2::uuid IS NULL OR project_id = $2) AND recorded_at >= $3 AND recorded_at < $4 AND currency::text = $5
			GROUP BY service_name ORDER BY sum(effective_cost) DESC LIMIT 1`, tenantID, project, from, to, h.Currency).Scan(&h.TopService, &h.TopServiceCost)
		if e != nil && !database.IsNotFound(e) {
			return e
		}
		return nil
	})
	return
}
