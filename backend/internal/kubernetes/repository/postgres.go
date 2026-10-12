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
				(tenant_id, project_id, cluster, namespace, workload, kind, period_start, energy_kwh, region_id, intensity_g_per_kwh, carbon_kg_co2e, methodology_version,
				 cpu_core_seconds, memory_avg_bytes, cpu_request_cores, memory_request_bytes)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
				ON CONFLICT (tenant_id, project_id, cluster, namespace, workload, period_start) DO UPDATE SET
				  kind = EXCLUDED.kind, energy_kwh = EXCLUDED.energy_kwh, region_id = EXCLUDED.region_id,
				  intensity_g_per_kwh = EXCLUDED.intensity_g_per_kwh, carbon_kg_co2e = EXCLUDED.carbon_kg_co2e,
				  methodology_version = EXCLUDED.methodology_version, cpu_core_seconds = EXCLUDED.cpu_core_seconds,
				  memory_avg_bytes = EXCLUDED.memory_avg_bytes, cpu_request_cores = EXCLUDED.cpu_request_cores,
				  memory_request_bytes = EXCLUDED.memory_request_bytes, ingested_at = now()`,
				tenantID, projectID, s.Cluster, s.Namespace, s.Workload, s.Kind, s.PeriodStart.UTC(), s.EnergyKWh, s.Region, s.Intensity, s.CarbonKg, s.MethodologyVersion,
				s.CPUCoreSeconds, s.MemoryAvgBytes, s.CPURequestCores, s.MemoryRequestBytes)
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
				count(*)::int, (count(*) FILTER (WHERE carbon_kg_co2e IS NULL))::int,
				-- utilization over the hours that have both the usage and the request
				(sum(cpu_core_seconds) FILTER (WHERE cpu_core_seconds IS NOT NULL AND cpu_request_cores > 0)
				  / NULLIF(sum(cpu_request_cores * 3600) FILTER (WHERE cpu_core_seconds IS NOT NULL AND cpu_request_cores > 0), 0) * 100)::float8,
				(sum(memory_avg_bytes) FILTER (WHERE memory_avg_bytes IS NOT NULL AND memory_request_bytes > 0)
				  / NULLIF(sum(memory_request_bytes) FILTER (WHERE memory_avg_bytes IS NOT NULL AND memory_request_bytes > 0), 0) * 100)::float8,
				(count(*) FILTER (WHERE cpu_core_seconds IS NOT NULL AND cpu_request_cores > 0))::int
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
			if err := rows.Scan(&w.Cluster, &w.Namespace, &w.Workload, &w.Kind, &w.EnergyKWh, &w.CarbonKg, &w.Hours, &w.NoGrid, &w.CPUUtilizationPct, &w.MemoryUtilizationPct, &w.UsageHours); err != nil {
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

func (r Postgres) UpsertCapacity(ctx context.Context, tenantID, projectID string, rows []domain.Capacity) error {
	return database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		batch := &pgx.Batch{}
		for _, c := range rows {
			batch.Queue(`INSERT INTO kubernetes.cluster_capacity (tenant_id, project_id, cluster, period_start, cpu_cores, memory_bytes) VALUES ($1,$2,$3,$4,$5,$6)
				ON CONFLICT (tenant_id, project_id, cluster, period_start) DO UPDATE SET cpu_cores = EXCLUDED.cpu_cores, memory_bytes = EXCLUDED.memory_bytes, ingested_at = now()`,
				tenantID, projectID, c.Cluster, c.PeriodStart.UTC(), c.CPUCores, c.MemoryBytes)
		}
		br := tx.SendBatch(ctx, batch)
		for range rows {
			if _, err := br.Exec(); err != nil {
				_ = br.Close()
				return err
			}
		}
		return br.Close()
	})
}

func (r Postgres) CostClusters(ctx context.Context, tenantID string, project *string, cluster string, from, to time.Time) (out []domain.ClusterRef, err error) {
	err = database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT DISTINCT project_id::text, cluster FROM kubernetes.workload_energy
			WHERE tenant_id = $1 AND ($2::uuid IS NULL OR project_id = $2) AND ($3 = '' OR cluster = $3) AND period_start >= $4 AND period_start < $5
			ORDER BY 1, 2 LIMIT 50`, tenantID, project, cluster, from, to)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var c domain.ClusterRef
			if err := rows.Scan(&c.ProjectID, &c.Cluster); err != nil {
				return err
			}
			out = append(out, c)
		}
		return rows.Err()
	})
	return
}

const gib = 1073741824.0

// CostInputs reads the day-level sums of one cluster: per workload (what it held, requested and used, in core-hours and GiB-hours), and per day the hours the
// agent reported and the nodes' capacity over them.
func (r Postgres) CostInputs(ctx context.Context, tenantID, projectID, cluster string, from, to time.Time) (in domain.CostInputs, err error) {
	err = database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT date_trunc('day', period_start)::date, namespace, workload,
				sum(GREATEST(COALESCE(cpu_request_cores, 0), COALESCE(cpu_core_seconds, 0) / 3600.0))::float8,
				sum(GREATEST(COALESCE(memory_request_bytes, 0), COALESCE(memory_avg_bytes, 0)) / $6::float8)::float8,
				COALESCE(sum(cpu_request_cores) FILTER (WHERE cpu_request_cores > 0), 0)::float8,
				COALESCE(sum(COALESCE(cpu_core_seconds, 0) / 3600.0) FILTER (WHERE cpu_request_cores > 0), 0)::float8,
				COALESCE(sum(memory_request_bytes / $6::float8) FILTER (WHERE memory_request_bytes > 0), 0)::float8,
				COALESCE(sum(COALESCE(memory_avg_bytes, 0) / $6::float8) FILTER (WHERE memory_request_bytes > 0), 0)::float8,
				count(*) FILTER (WHERE cpu_request_cores > 0) > 0, count(*) FILTER (WHERE memory_request_bytes > 0) > 0, count(*)::int
			FROM kubernetes.workload_energy
			WHERE tenant_id = $1 AND project_id = $2 AND cluster = $3 AND period_start >= $4 AND period_start < $5
			GROUP BY 1, 2, 3`, tenantID, projectID, cluster, from, to, gib)
		if err != nil {
			return err
		}
		for rows.Next() {
			var w domain.DayWorkload
			if err := rows.Scan(&w.Day, &w.Namespace, &w.Workload, &w.CPUHold, &w.MemHold, &w.CPURequested, &w.CPUUsed, &w.MemRequested, &w.MemUsed, &w.CPUKnown, &w.MemKnown, &w.Hours); err != nil {
				rows.Close()
				return err
			}
			if w.CPUHold == 0 && w.MemHold == 0 {
				in.Unmeasured++
				continue
			}
			in.Workloads = append(in.Workloads, w)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		hours := map[time.Time]*domain.DayHours{}
		hr, err := tx.Query(ctx, `SELECT date_trunc('day', period_start)::date, count(DISTINCT period_start)::int FROM kubernetes.workload_energy
			WHERE tenant_id = $1 AND project_id = $2 AND cluster = $3 AND period_start >= $4 AND period_start < $5 GROUP BY 1`, tenantID, projectID, cluster, from, to)
		if err != nil {
			return err
		}
		for hr.Next() {
			var d domain.DayHours
			if err := hr.Scan(&d.Day, &d.HoursSeen); err != nil {
				hr.Close()
				return err
			}
			dd := d
			hours[d.Day] = &dd
		}
		hr.Close()
		cr, err := tx.Query(ctx, `SELECT date_trunc('day', period_start)::date, sum(cpu_cores)::float8, sum(memory_bytes / $6::float8)::float8, count(*)::int FROM kubernetes.cluster_capacity
			WHERE tenant_id = $1 AND project_id = $2 AND cluster = $3 AND period_start >= $4 AND period_start < $5 GROUP BY 1`, tenantID, projectID, cluster, from, to, gib)
		if err != nil {
			return err
		}
		for cr.Next() {
			var day time.Time
			var cpu, mem float64
			var n int
			if err := cr.Scan(&day, &cpu, &mem, &n); err != nil {
				cr.Close()
				return err
			}
			if d := hours[day]; d != nil {
				d.CapCPU, d.CapMem, d.CapHours = cpu, mem, n
			}
		}
		cr.Close()
		for _, d := range hours {
			in.Days = append(in.Days, *d)
		}
		return cr.Err()
	})
	return
}

func (r Postgres) GetPool(ctx context.Context, tenantID, projectID, cluster string) (services []string, err error) {
	services = []string{}
	err = database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		e := tx.QueryRow(ctx, `SELECT services FROM kubernetes.cluster_pools WHERE tenant_id = $1 AND project_id = $2 AND cluster = $3`, tenantID, projectID, cluster).Scan(&services)
		if database.IsNotFound(e) {
			return nil
		}
		return e
	})
	return
}

func (r Postgres) SetPool(ctx context.Context, tenantID, projectID, cluster string, services []string) error {
	if services == nil {
		services = []string{}
	}
	return database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO kubernetes.cluster_pools (tenant_id, project_id, cluster, services) VALUES ($1, $2, $3, $4)
			ON CONFLICT (tenant_id, project_id, cluster) DO UPDATE SET services = EXCLUDED.services, updated_at = now()`, tenantID, projectID, cluster, services); err != nil {
			return err
		}
		return audit.Record(audit.WithTenant(ctx, tenantID), tx, "kubernetes.pool_set", "cluster:"+cluster, map[string]any{"services": len(services)})
	})
}
