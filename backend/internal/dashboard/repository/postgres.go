// Package repository implements the dashboard read models on PostgreSQL.
package repository

import (
	"context"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quixgit/greenops-reliabilix/backend/internal/dashboard/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/database"
)

type Postgres struct{ Pool *pgxpool.Pool }

func (r Postgres) Trend(ctx context.Context, f domain.Filter) ([]domain.TrendPoint, error) {
	out := []domain.TrendPoint{}
	err := database.WithTenantTx(ctx, r.Pool, f.TenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			WITH c AS (SELECT date_trunc('day', recorded_at) d, sum(cost) cost FROM usage.usage_records
			             WHERE tenant_id = $1 AND ($4::uuid IS NULL OR project_id = $4) AND recorded_at >= $2 AND recorded_at < $3 GROUP BY 1),
			     k AS (SELECT date_trunc('day', period_start) d, sum(carbon_kg_co2e) co2e FROM carbon.calculations
			             WHERE tenant_id = $1 AND ($4::uuid IS NULL OR project_id = $4) AND period_start >= $2 AND period_start < $3 GROUP BY 1)
			SELECT COALESCE(c.d, k.d), COALESCE(c.cost, 0), COALESCE(k.co2e, 0)
			FROM c FULL JOIN k ON c.d = k.d ORDER BY 1`, f.TenantID, f.From, f.To, f.Project)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var p domain.TrendPoint
			if err := rows.Scan(&p.Day, &p.Cost, &p.CO2eKg); err != nil {
				return err
			}
			out = append(out, p)
		}
		return rows.Err()
	})
	return out, err
}

func (r Postgres) Providers(ctx context.Context, f domain.Filter) ([]domain.ProviderShare, error) {
	out := []domain.ProviderShare{}
	err := database.WithTenantTx(ctx, r.Pool, f.TenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT provider, sum(carbon_kg_co2e) FROM carbon.calculations
			WHERE tenant_id = $1 AND ($4::uuid IS NULL OR project_id = $4) AND period_start >= $2 AND period_start < $3
			GROUP BY provider HAVING sum(carbon_kg_co2e) > 0 ORDER BY 2 DESC`, f.TenantID, f.From, f.To, f.Project)
		if err != nil {
			return err
		}
		defer rows.Close()
		var total float64
		for rows.Next() {
			var p domain.ProviderShare
			if err := rows.Scan(&p.Provider, &p.CO2eKg); err != nil {
				return err
			}
			total += p.CO2eKg
			out = append(out, p)
		}
		for i := range out {
			out[i].SharePct = out[i].CO2eKg / total * 100
		}
		return rows.Err()
	})
	return out, err
}

func (r Postgres) Services(ctx context.Context, f domain.Filter, category string) ([]domain.ServiceRow, error) {
	byName := map[string]*domain.ServiceRow{}
	err := database.WithTenantTx(ctx, r.Pool, f.TenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT service_name, service_category, sum(cost) FROM usage.usage_records
			WHERE tenant_id = $1 AND ($4::uuid IS NULL OR project_id = $4) AND recorded_at >= $2 AND recorded_at < $3
			AND ($5 = '' OR service_category = $5) GROUP BY 1, 2`, f.TenantID, f.From, f.To, f.Project, category)
		if err != nil {
			return err
		}
		for rows.Next() {
			s := &domain.ServiceRow{Spark: []float64{}}
			if err := rows.Scan(&s.Service, &s.Category, &s.Cost); err != nil {
				rows.Close()
				return err
			}
			byName[s.Service] = s
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		crows, err := tx.Query(ctx, `SELECT service_name, service_category, date_trunc('day', period_start) d, sum(carbon_kg_co2e)
			FROM carbon.calculations
			WHERE tenant_id = $1 AND ($4::uuid IS NULL OR project_id = $4) AND period_start >= $2 AND period_start < $3
			AND ($5 = '' OR service_category = $5) GROUP BY 1, 2, 3 ORDER BY 3`, f.TenantID, f.From, f.To, f.Project, category)
		if err != nil {
			return err
		}
		defer crows.Close()
		for crows.Next() {
			var name, cat string
			var d interface{}
			var co2e float64
			if err := crows.Scan(&name, &cat, &d, &co2e); err != nil {
				return err
			}
			s, ok := byName[name]
			if !ok {
				s = &domain.ServiceRow{Service: name, Category: cat, Spark: []float64{}}
				byName[name] = s
			}
			s.CO2eKg += co2e
			s.Spark = append(s.Spark, co2e)
		}
		return crows.Err()
	})
	out := make([]domain.ServiceRow, 0, len(byName))
	for _, s := range byName {
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Cost > out[j].Cost })
	return out, err
}

// Regions reads global reference data (no tenant): the latest stored reading per
// region and the one at least 24h older, if any.
func (r Postgres) Regions(ctx context.Context) ([]domain.RegionIntensity, error) {
	rows, err := r.Pool.Query(ctx, `
		WITH latest AS (SELECT DISTINCT ON (region) region, g_co2e_per_kwh g, ts FROM carbon.grid_intensity
		                 WHERE provider = 'electricitymaps' AND NOT is_forecast ORDER BY region, ts DESC),
		     prev AS (SELECT DISTINCT ON (gi.region) gi.region, gi.g_co2e_per_kwh g FROM carbon.grid_intensity gi
		                JOIN latest l ON l.region = gi.region AND gi.ts <= l.ts - interval '24 hours'
		               WHERE gi.provider = 'electricitymaps' AND NOT gi.is_forecast ORDER BY gi.region, gi.ts DESC)
		SELECT l.region, l.g, l.ts, p.g FROM latest l LEFT JOIN prev p USING (region) ORDER BY l.g`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.RegionIntensity{}
	for rows.Next() {
		var x domain.RegionIntensity
		var prev *float64
		if err := rows.Scan(&x.Region, &x.GPerKWh, &x.At, &prev); err != nil {
			return nil, err
		}
		if prev != nil && *prev > 0 {
			c := (x.GPerKWh - *prev) / *prev * 100
			x.ChangePct = &c
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (r Postgres) Activity(ctx context.Context, tenantID string, limit int) ([]domain.Activity, error) {
	out := []domain.Activity{}
	err := database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT id, action, target, created_at FROM audit.audit_logs
			WHERE tenant_id = $1 AND action = ANY($2) ORDER BY created_at DESC LIMIT $3`, tenantID, domain.UserFacingActions, limit)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var a domain.Activity
			if err := rows.Scan(&a.ID, &a.Action, &a.Target, &a.At); err != nil {
				return err
			}
			out = append(out, a)
		}
		return rows.Err()
	})
	return out, err
}
