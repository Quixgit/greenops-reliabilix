// Package repository is the PostgreSQL adapter of the Kubernetes energy domain.
package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quixgit/greenops-reliabilix/backend/internal/kubernetes/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/audit"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/database"
)

type Postgres struct{ Pool *pgxpool.Pool }

func (r Postgres) ProjectExists(ctx context.Context, tenantID, projectID string) (ok bool, err error) {
	err = database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM projects.projects WHERE tenant_id = $1 AND id = $2)`, tenantID, projectID).Scan(&ok)
	})
	return
}

// Upsert stores the batch in one transaction; the same workload and hour replaces the earlier row.
func (r Postgres) Upsert(ctx context.Context, tenantID, projectID string, rows []domain.Stored) error {
	return database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		batch := &pgx.Batch{}
		for _, s := range rows {
			batch.Queue(`INSERT INTO kubernetes.workload_energy
				(tenant_id, project_id, cluster, namespace, workload, kind, period_start, energy_kwh, region_id, intensity_g_per_kwh, carbon_kg_co2e, methodology_version)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
				ON CONFLICT (tenant_id, project_id, cluster, namespace, workload, period_start) DO UPDATE SET
				  kind = EXCLUDED.kind, energy_kwh = EXCLUDED.energy_kwh, region_id = EXCLUDED.region_id,
				  intensity_g_per_kwh = EXCLUDED.intensity_g_per_kwh, carbon_kg_co2e = EXCLUDED.carbon_kg_co2e,
				  methodology_version = EXCLUDED.methodology_version, ingested_at = now()`,
				tenantID, projectID, s.Cluster, s.Namespace, s.Workload, s.Kind, s.PeriodStart.UTC(), s.EnergyKWh, s.Region, s.Intensity, s.CarbonKg, s.MethodologyVersion)
		}
		br := tx.SendBatch(ctx, batch)
		for range rows {
			if _, err := br.Exec(); err != nil {
				_ = br.Close()
				return err
			}
		}
		if err := br.Close(); err != nil {
			return err
		}
		return audit.Record(audit.WithTenant(ctx, tenantID), tx, "kubernetes.energy_ingested", "project:"+projectID, map[string]any{"samples": len(rows)})
	})
}

func (r Postgres) Workloads(ctx context.Context, tenantID string, project *string, cluster string, from, to time.Time, limit int) (out []domain.WorkloadRow, err error) {
	out = []domain.WorkloadRow{}
	err = database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT cluster, namespace, workload, max(kind), sum(energy_kwh)::float8,
				CASE WHEN count(*) FILTER (WHERE carbon_kg_co2e IS NULL) = 0 THEN sum(carbon_kg_co2e)::float8 END,
				count(*)::int, (count(*) FILTER (WHERE carbon_kg_co2e IS NULL))::int
			FROM kubernetes.workload_energy
			WHERE tenant_id = $1 AND ($2::uuid IS NULL OR project_id = $2) AND ($3 = '' OR cluster = $3)
			  AND period_start >= $4 AND period_start < $5
			GROUP BY cluster, namespace, workload ORDER BY sum(energy_kwh) DESC LIMIT $6`, tenantID, project, cluster, from, to, limit)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var w domain.WorkloadRow
			if err := rows.Scan(&w.Cluster, &w.Namespace, &w.Workload, &w.Kind, &w.EnergyKWh, &w.CarbonKg, &w.Hours, &w.NoGrid); err != nil {
				return err
			}
			out = append(out, w)
		}
		return rows.Err()
	})
	return
}

func (r Postgres) Clusters(ctx context.Context, tenantID string, project *string, from, to time.Time) (out []domain.ClusterRow, err error) {
	out = []domain.ClusterRow{}
	err = database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT cluster, count(DISTINCT (namespace, workload))::int, count(DISTINCT namespace)::int,
				sum(energy_kwh)::float8,
				CASE WHEN count(*) FILTER (WHERE carbon_kg_co2e IS NULL) = 0 THEN sum(carbon_kg_co2e)::float8 END,
				to_char(max(period_start) AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:00:00"Z"')
			FROM kubernetes.workload_energy
			WHERE tenant_id = $1 AND ($2::uuid IS NULL OR project_id = $2) AND period_start >= $3 AND period_start < $4
			GROUP BY cluster ORDER BY sum(energy_kwh) DESC`, tenantID, project, from, to)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var c domain.ClusterRow
			if err := rows.Scan(&c.Cluster, &c.Workloads, &c.Namespaces, &c.EnergyKWh, &c.CarbonKg, &c.LastHour); err != nil {
				return err
			}
			out = append(out, c)
		}
		return rows.Err()
	})
	return
}
