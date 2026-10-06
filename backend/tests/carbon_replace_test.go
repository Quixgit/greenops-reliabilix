//go:build integration

package tests

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon/domain"
	carbonrepo "github.com/quixgit/greenops-reliabilix/backend/internal/carbon/repository"
)

// A recomputed window must replace the old rows of the current methodology version, and only those: a
// cost-based row that measured usage now supersedes must disappear, other projects and other methodology
// versions must be untouched.
func TestReplaceCalculationsRewritesOnlyTheWindowOfThisProject(t *testing.T) {
	ctx := context.Background()
	owner, worker := pool(t, "TEST_OWNER_DSN"), pool(t, "TEST_WORKER_DSN")
	tenant := org(t, owner, "carbon-replace")
	projA, projB := uuid.NewString(), uuid.NewString()
	repo := carbonrepo.Postgres{Pool: worker}

	day := time.Now().UTC().Truncate(24 * time.Hour).AddDate(0, 0, -5)
	calc := func(method domain.Method, d time.Time, kg float64) domain.Calculation {
		return domain.Calculation{Provider: "aws", ServiceName: "EC2", ServiceCategory: "compute", RegionID: "eu-central-1", Method: method,
			PeriodStart: d, PeriodEnd: d.Add(24 * time.Hour), EnergyKWh: 1, IntensityGPerKWh: 300, CO2eKg: kg, MethodologyVersion: domain.MethodologyVersion}
	}
	from, to := day, day.AddDate(0, 0, 2)
	if err := repo.ReplaceCalculations(ctx, tenant, projA, from, to, []domain.Calculation{calc(domain.CostBased, day, 10), calc(domain.CostBased, day.AddDate(0, 0, 1), 11)}); err != nil {
		t.Fatal(err)
	}
	if err := repo.ReplaceCalculations(ctx, tenant, projB, from, to, []domain.Calculation{calc(domain.CostBased, day, 99)}); err != nil {
		t.Fatal(err)
	}
	// a row of an older methodology version in the same window is history and must survive
	if _, err := owner.Exec(ctx, `INSERT INTO carbon.calculations (tenant_id, project_id, provider, region_id, service_name, service_category, method, period_start, period_end,
		energy_kwh, intensity_g_per_kwh, carbon_kg_co2e, methodology_version) VALUES ($1,$2,'aws','eu-central-1','EC2','compute','cost_based',$3,$4,1,300,7,'OLD-1')`,
		tenant, projA, day, day.Add(24*time.Hour)); err != nil {
		t.Fatal(err)
	}

	// the engine now measures day 0 and no longer produces the cost-based rows
	if err := repo.ReplaceCalculations(ctx, tenant, projA, from, to, []domain.Calculation{calc(domain.UsageBased, day, 4)}); err != nil {
		t.Fatal(err)
	}

	rows := func(project string) map[string]int {
		r, err := owner.Query(ctx, `SELECT method || '/' || methodology_version, count(*) FROM carbon.calculations WHERE tenant_id=$1 AND project_id=$2 GROUP BY 1`, tenant, project)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Close()
		out := map[string]int{}
		for r.Next() {
			var k string
			var n int
			if err := r.Scan(&k, &n); err != nil {
				t.Fatal(err)
			}
			out[k] = n
		}
		return out
	}
	a := rows(projA)
	if a["usage_based/"+domain.MethodologyVersion] != 1 || a["cost_based/"+domain.MethodologyVersion] != 0 || a["cost_based/OLD-1"] != 1 {
		t.Errorf("project A rows = %v: stale cost rows must go, the old methodology must stay", a)
	}
	if b := rows(projB); b["cost_based/"+domain.MethodologyVersion] != 1 {
		t.Errorf("project B must be untouched, got %v", b)
	}

	// an empty result still clears the window (nothing measurable any more)
	if err := repo.ReplaceCalculations(ctx, tenant, projA, from, to, nil); err != nil {
		t.Fatal(err)
	}
	if a := rows(projA); a["usage_based/"+domain.MethodologyVersion] != 0 {
		t.Errorf("empty replace must clear the window: %v", a)
	}
}
