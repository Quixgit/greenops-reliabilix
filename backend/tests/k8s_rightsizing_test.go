//go:build integration

package tests

import (
	"context"
	"testing"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/recommendations/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/recommendations/repository"
)

func TestK8sRightsizingFromStoredUsage(t *testing.T) {
	ctx := context.Background()
	owner, workerPool := pool(t, "TEST_OWNER_DSN"), pool(t, "TEST_WORKER_DSN")
	tenant, project := seedOrg(t, wrap{owner}, "K8s Sizing Org")
	t.Cleanup(func() { _, _ = owner.Exec(ctx, `DELETE FROM tenants.organizations WHERE id = $1`, tenant) })

	start := time.Now().UTC().Truncate(time.Hour).AddDate(0, 0, -8)
	// 100 hours: uses ~0.2 cores of a 2-core request and ~500 MB of a 4 GiB request; one workload without requests.
	for i := 0; i < 100; i++ {
		at := start.Add(time.Duration(i) * time.Hour)
		for _, w := range []struct {
			name          string
			cpu, req, mem float64
			memReq        float64
			withReq       bool
		}{{"checkout", 720, 2, 5e8, 4 * 1024 * 1024 * 1024, true}, {"batch", 100, 0, 1e8, 0, false}} {
			var req, memReq any
			if w.withReq {
				req, memReq = w.req, w.memReq
			}
			if _, err := owner.Exec(ctx, `INSERT INTO kubernetes.workload_energy (tenant_id, project_id, cluster, namespace, workload, kind, period_start,
				energy_kwh, region_id, methodology_version, cpu_core_seconds, memory_avg_bytes, cpu_request_cores, memory_request_bytes)
				VALUES ($1,$2,'prod','shop',$3,'deployment',$4,1,'eu-central-1','RLX-PROVISIONAL-1',$5,$6,$7,$8)`,
				tenant, project, w.name, at, w.cpu, w.mem, req, memReq); err != nil {
				t.Fatal(err)
			}
		}
	}
	repo := repository.Postgres{Pool: workerPool}
	now := time.Now().UTC()
	usage, err := repo.K8sUsage(ctx, tenant, project, now.AddDate(0, 0, -14), now)
	if err != nil || len(usage) != 2 {
		t.Fatalf("%+v %v", usage, err)
	}
	recs := domain.GenerateK8sRightsizing(project, usage, now)
	if len(recs) != 1 || recs[0].ServiceName != "shop/checkout" {
		t.Fatalf("only the workload with requests and enough hours is recommended: %+v", recs)
	}
	r := recs[0]
	r.ProjectID, r.Fingerprint = project, "k8s|"+project+"|prod|shop/checkout"
	if ok, err := repo.Insert(ctx, tenant, r); err != nil || !ok {
		t.Fatalf("insert: %v %v", ok, err)
	}
	if ok, _ := repo.Insert(ctx, tenant, r); ok {
		t.Fatal("the same finding must not be stored twice")
	}
	got, err := repo.List(ctx, tenant, "open", nil, 10)
	if err != nil || len(got) != 1 || got[0].Type != domain.K8sRightsizing || got[0].Details["cpu_request_recommended_cores"] != "0.25" {
		t.Fatalf("%+v %v", got, err)
	}
}
