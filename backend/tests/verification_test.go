//go:build integration

package tests

import (
	"context"
	"testing"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/methodology"
	"github.com/quixgit/greenops-reliabilix/backend/internal/recommendations/application"
	"github.com/quixgit/greenops-reliabilix/backend/internal/recommendations/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/recommendations/repository"
)

// A change applied on 1 Sept with data around it is measured by the worker, stored, and read back through the API role.
func TestImpactVerification(t *testing.T) {
	ctx := context.Background()
	owner, apiPool, workerPool := pool(t, "TEST_OWNER_DSN"), pool(t, "TEST_API_DSN"), pool(t, "TEST_WORKER_DSN")
	tenant := org(t, owner, "Verification Org")
	t.Cleanup(func() { _, _ = owner.Exec(ctx, `DELETE FROM tenants.organizations WHERE id = $1`, tenant) })

	var project, rec string
	if err := owner.QueryRow(ctx, `INSERT INTO projects.projects (tenant_id, name, functional_unit) VALUES ($1,'p','api_request') RETURNING id`, tenant).Scan(&project); err != nil {
		t.Fatal(err)
	}
	applied := time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)
	if err := owner.QueryRow(ctx, `INSERT INTO recommendations.recommendations (tenant_id, project_id, type, title, provider, service_name,
		estimated_carbon_reduction_pct, carbon_reduction_kg_month, estimated_cost_impact, cost_basis, confidence, compliance_check, status,
		fingerprint, applied_at) VALUES ($1,$2,'rightsizing','t','aws','Amazon EC2',10,30,-100,'modelled',0.8,'{"residency":"passed"}','applied','fp-verify',$3) RETURNING id`,
		tenant, project, applied).Scan(&rec); err != nil {
		t.Fatal(err)
	}
	// 14 days before at 10 cost / 5 kg per day, 14 days after (from the 5th) at 10 cost / 4 kg per day.
	for i := 0; i < 36; i++ {
		d := time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC).AddDate(0, 0, i)
		kg := 5.0
		if !d.Before(time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)) {
			kg = 4
		}
		if _, err := owner.Exec(ctx, `INSERT INTO usage.usage_records (tenant_id, project_id, source, provider, service_name, service_category,
			region_id, billed_cost, effective_cost, currency, recorded_at, charge_period_end) VALUES ($1,$2,'cost_explorer','aws','Amazon EC2','compute',
			'eu-central-1',10,10,'USD',$3,$3::timestamptz + interval '1 day')`, tenant, project, d); err != nil {
			t.Fatal(err)
		}
		if _, err := owner.Exec(ctx, `INSERT INTO carbon.calculations (tenant_id, project_id, provider, region_id, service_name, service_category,
			method, period_start, period_end, energy_kwh, intensity_g_per_kwh, carbon_kg_co2e, methodology_version) VALUES ($1,$2,'aws','eu-central-1',
			'Amazon EC2','compute','usage_based',$3,$3::timestamptz + interval '1 day',1,300,$4,$5)`, tenant, project, d, kg, methodology.Version); err != nil {
			t.Fatal(err)
		}
	}

	svc := application.Service{Repo: repository.Postgres{Pool: workerPool}, Now: func() time.Time { return time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC) }}
	n, err := svc.VerifyApplied(ctx, tenant, project)
	if err != nil || n != 1 {
		t.Fatalf("verified %d, err %v", n, err)
	}
	if again, _ := svc.VerifyApplied(ctx, tenant, project); again != 0 {
		t.Fatalf("a verified recommendation must not be measured twice, got %d", again)
	}

	got, err := repository.Postgres{Pool: apiPool}.Get(ctx, tenant, rec)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != domain.Verified || got.Verification == nil {
		t.Fatalf("status %s verification %v", got.Status, got.Verification)
	}
	v := got.Verification
	if v.Outcome != domain.Realized || v.MeasuredCarbonKgMonth == nil || *v.MeasuredCarbonKgMonth != 30 {
		t.Errorf("outcome %s measured %v, want realized 30 kg/month", v.Outcome, v.MeasuredCarbonKgMonth)
	}
	if v.MeasuredCostMonth == nil || *v.MeasuredCostMonth != 0 || v.ExpectedCostMonth == nil || *v.ExpectedCostMonth != 100 {
		t.Errorf("cost must be reported separately: measured %v expected %v", v.MeasuredCostMonth, v.ExpectedCostMonth)
	}
}
