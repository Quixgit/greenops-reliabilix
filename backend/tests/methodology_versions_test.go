//go:build integration

package tests

import (
	"context"
	"testing"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/carbon/repository"
)

// Rows of one methodology version are replaced and read without touching another version's rows: that is what
// keeps old results reproducible and lets two versions be compared over the same window.
func TestMethodologyVersionsStayIsolated(t *testing.T) {
	ctx := context.Background()
	owner, workerPool := pool(t, "TEST_OWNER_DSN"), pool(t, "TEST_WORKER_DSN")
	tenant := org(t, owner, "Versions Org")
	t.Cleanup(func() { _, _ = owner.Exec(ctx, `DELETE FROM tenants.organizations WHERE id = $1`, tenant) })
	var project string
	if err := owner.QueryRow(ctx, `INSERT INTO projects.projects (tenant_id, name, functional_unit) VALUES ($1,'p','api_request') RETURNING id`, tenant).Scan(&project); err != nil {
		t.Fatal(err)
	}
	repo := repository.Postgres{Pool: workerPool}
	day := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	mk := func(version string, kg float64) []domain.Calculation {
		return []domain.Calculation{{Provider: "aws", ServiceName: "EC2", ServiceCategory: "compute", RegionID: "eu-central-1", Method: domain.UsageBased,
			PeriodStart: day, PeriodEnd: day.Add(24 * time.Hour), EnergyKWh: 10, IntensityGPerKWh: 300, CO2eKg: kg, MethodologyVersion: version}}
	}
	to := day.AddDate(0, 0, 1)
	if err := repo.ReplaceCalculations(ctx, tenant, project, day, to, "V-A", mk("V-A", 3)); err != nil {
		t.Fatal(err)
	}
	if err := repo.ReplaceCalculations(ctx, tenant, project, day, to, "V-B", mk("V-B", 5)); err != nil {
		t.Fatal(err)
	}
	// Recomputing B again must not remove A.
	if err := repo.ReplaceCalculations(ctx, tenant, project, day, to, "V-B", mk("V-B", 6)); err != nil {
		t.Fatal(err)
	}
	a, err := repo.VersionTotals(ctx, tenant, nil, day, to, "V-A")
	if err != nil || a.CO2eKg != 3 || a.Rows != 1 {
		t.Fatalf("A = %+v err %v, want 3 kg in 1 row", a, err)
	}
	b, err := repo.VersionTotals(ctx, tenant, nil, day, to, "V-B")
	if err != nil || b.CO2eKg != 6 || b.Rows != 1 {
		t.Fatalf("B = %+v err %v, want 6 kg in 1 row", b, err)
	}
	if none, _ := repo.VersionTotals(ctx, tenant, nil, day, to, "V-C"); none.Rows != 0 {
		t.Fatalf("an unknown version has no rows: %+v", none)
	}
}
