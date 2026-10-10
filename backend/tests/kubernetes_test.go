//go:build integration

package tests

import (
	"context"
	"testing"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/kubernetes/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/kubernetes/repository"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
)

func TestKubernetesEnergyUpsertAndAggregates(t *testing.T) {
	ctx := context.Background()
	owner, apiPool := pool(t, "TEST_OWNER_DSN"), pool(t, "TEST_API_DSN")
	tenant, project := seedOrg(t, wrap{owner}, "K8s Org")
	t.Cleanup(func() { _, _ = owner.Exec(ctx, `DELETE FROM tenants.organizations WHERE id = $1`, tenant) })
	repo := repository.Postgres{Pool: apiPool}

	h := time.Date(2026, 10, 9, 10, 0, 0, 0, time.UTC)
	f := func(v float64) *float64 { return &v }
	mk := func(w string, hour int, kwh float64, kg *float64) domain.Stored {
		return domain.Stored{Sample: domain.Sample{Cluster: "prod", Namespace: "shop", Workload: w, Kind: "deployment",
			PeriodStart: h.Add(time.Duration(hour) * time.Hour), EnergyKWh: kwh, Region: "eu-central-1"}, CarbonKg: kg, MethodologyVersion: "RLX-PROVISIONAL-1"}
	}
	withUse := func(s domain.Stored, cpu, req float64) domain.Stored {
		s.CPUCoreSeconds, s.CPURequestCores = &cpu, &req
		return s
	}
	rows := []domain.Stored{withUse(mk("api", 0, 1, f(0.5)), 1800, 2), withUse(mk("api", 1, 2, f(1)), 900, 2), mk("web", 0, 4, nil)}
	if err := repo.Upsert(ctx, tenant, project, rows); err != nil {
		t.Fatal(err)
	}
	// The same hour again replaces the value (idempotent), it does not add.
	if err := repo.Upsert(ctx, tenant, project, []domain.Stored{withUse(mk("api", 0, 1.5, f(0.75)), 1800, 2)}); err != nil {
		t.Fatal(err)
	}
	from, to := h.Add(-time.Hour), h.Add(24*time.Hour)
	ws, err := repo.Workloads(ctx, tenant, nil, "", from, to, 10)
	if err != nil || len(ws) != 2 {
		t.Fatalf("%+v %v", ws, err)
	}
	if ws[0].Workload != "web" || ws[0].EnergyKWh != 4 || ws[0].CarbonKg != nil || ws[0].NoGrid != 1 {
		t.Errorf("web has no grid data: carbon must be unknown, not 0: %+v", ws[0])
	}
	if ws[1].Workload != "api" || ws[1].EnergyKWh != 3.5 || ws[1].CarbonKg == nil || *ws[1].CarbonKg != 1.75 {
		t.Errorf("api must be 1.5 + 2 kWh and 0.75 + 1 kg: %+v", ws[1])
	}
	// A replaced hour carries its own usage: sending the hour again without usage would clear it (the row is replaced).
	// 1800 + 900 core-seconds of 2 requested cores over 2 hours = 2700 / 14400 = 18.75 %; web sent no usage.
	if ws[1].CPUUtilizationPct == nil || *ws[1].CPUUtilizationPct != 18.75 || ws[1].UsageHours != 2 {
		t.Errorf("api utilization: %+v", ws[1])
	}
	if ws[0].CPUUtilizationPct != nil || ws[0].UsageHours != 0 {
		t.Errorf("web has no usage: utilization must be unknown, not 0: %+v", ws[0])
	}
	cs, err := repo.Clusters(ctx, tenant, nil, from, to)
	if err != nil || len(cs) != 1 || cs[0].Workloads != 2 || cs[0].CarbonKg != nil {
		t.Fatalf("a cluster with a missing hour must not show a partial carbon total: %+v %v", cs, err)
	}
	// The agent key role may push and nothing else.
	if !auth.RoleAgent.Can(auth.PermKubernetesIngest) || auth.RoleAgent.Can(auth.PermCarbonRead) || auth.RoleAgent.Can(auth.PermProjectRead) {
		t.Error("agent role must only be allowed to ingest")
	}
}
