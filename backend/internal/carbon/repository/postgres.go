// Package repository reads and writes persisted carbon data.
package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/database"
)

// Totals aggregates calculations over a window.
type Totals struct {
	EnergyKWh float64 `json:"energy_kwh"`
	CO2eKg    float64 `json:"carbon_kg_co2e"`
	SCI       float64 `json:"sci_score"`
	// IntensityGPerKWh is the effective grid intensity: CO2e / energy.
	IntensityGPerKWh float64 `json:"carbon_intensity_g_per_kwh"`
	Rows             int64   `json:"-"`
}

type Summary struct {
	Totals
	HasData  bool   `json:"has_data"`
	Previous Totals `json:"previous"`
}

type TrendPoint struct {
	Day    time.Time `json:"day"`
	CO2eKg float64   `json:"carbon_kg_co2e"`
}

type Postgres struct{ Pool *pgxpool.Pool }

func totals(ctx context.Context, tx pgx.Tx, tenantID string, project *string, from, to time.Time) (t Totals, err error) {
	err = tx.QueryRow(ctx, `SELECT COALESCE(sum(energy_kwh),0), COALESCE(sum(carbon_kg_co2e),0), COALESCE(avg(sci_score),0), count(*)
		FROM carbon.calculations
		WHERE tenant_id = $1 AND ($4::uuid IS NULL OR project_id = $4) AND period_start >= $2 AND period_start < $3`,
		tenantID, from, to, project).Scan(&t.EnergyKWh, &t.CO2eKg, &t.SCI, &t.Rows)
	if t.EnergyKWh > 0 {
		t.IntensityGPerKWh = t.CO2eKg * 1000 / t.EnergyKWh
	}
	return
}

// Summary returns the window and the immediately preceding window of equal length.
func (r Postgres) Summary(ctx context.Context, tenantID string, project *string, from, to time.Time) (s Summary, err error) {
	err = database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		var err error
		if s.Totals, err = totals(ctx, tx, tenantID, project, from, to); err != nil {
			return err
		}
		s.Previous, err = totals(ctx, tx, tenantID, project, from.Add(-to.Sub(from)), from)
		return err
	})
	s.HasData = s.Rows > 0
	return
}

func (r Postgres) Trend(ctx context.Context, tenantID string, project *string, from, to time.Time) ([]TrendPoint, error) {
	out := []TrendPoint{}
	err := database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT date_trunc('day', period_start) d, sum(carbon_kg_co2e)
			FROM carbon.calculations WHERE tenant_id = $1 AND ($4::uuid IS NULL OR project_id = $4)
			AND period_start >= $2 AND period_start < $3 GROUP BY d ORDER BY d`, tenantID, from, to, project)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var p TrendPoint
			if err := rows.Scan(&p.Day, &p.CO2eKg); err != nil {
				return err
			}
			out = append(out, p)
		}
		return rows.Err()
	})
	return out, err
}

// SaveGridIntensity stores a reading in the global reference table (idempotent).
func (r Postgres) SaveGridIntensity(ctx context.Context, provider, region string, at time.Time, gPerKWh float64, forecast bool) error {
	_, err := r.Pool.Exec(ctx, `INSERT INTO carbon.grid_intensity (provider, region, ts, g_co2e_per_kwh, is_forecast)
		VALUES ($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`, provider, region, at, gPerKWh, forecast)
	return err
}
