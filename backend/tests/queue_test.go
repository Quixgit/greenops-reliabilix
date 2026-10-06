//go:build integration

package tests

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/hibiken/asynq"

	"github.com/quixgit/greenops-reliabilix/backend/internal/app"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/queue"
)

// TestAsynqWorkerOnRealRedis runs the real worker server against Redis: a carbon:calculate job enqueued
// through the production queue client must be decoded, executed by the module handler and persist results.
func TestAsynqWorkerOnRealRedis(t *testing.T) {
	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("TEST_REDIS_ADDR not set")
	}
	ctx := context.Background()
	owner, workerPool := pool(t, "TEST_OWNER_DSN"), pool(t, "TEST_WORKER_DSN")

	tenant := org(t, owner, "queue-"+t.Name()+time.Now().Format("150405.000"))
	var proj string
	if err := owner.QueryRow(ctx, `INSERT INTO projects.projects (tenant_id,name,functional_unit) VALUES ($1,'p','u') RETURNING id`, tenant).Scan(&proj); err != nil {
		t.Fatal(err)
	}
	day := time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, -2)
	if _, err := owner.Exec(ctx, `INSERT INTO usage.usage_records (tenant_id,project_id,source,provider,service_name,service_category,region_id,billed_cost,effective_cost,currency,recorded_at,charge_period_end)
		VALUES ($1,$2,'cost_explorer','aws','Amazon EC2','compute','us-east-1',50,50,'USD',$3,$4)`, tenant, proj, day, day.Add(24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Exec(ctx, `INSERT INTO carbon.grid_intensity (provider, region_id, ts, g_co2e_per_kwh) VALUES ('electricitymaps','us-east-1',now() - interval '1 hour',380) ON CONFLICT DO NOTHING`); err != nil {
		t.Fatal(err)
	}

	cfg := devConfig(t.TempDir())
	qc := queue.NewClient(addr)
	defer func() { _ = qc.Close() }()
	c, err := app.Build(ctx, cfg, quietLog(), workerPool, qc)
	if err != nil {
		t.Fatal(err)
	}
	mux := asynq.NewServeMux()
	for _, j := range c.JobRegistrars() {
		j.RegisterJobs(mux)
	}
	failed := make(chan error, 4)
	srv := queue.NewServer(addr, 2, func(_ context.Context, typ string, err error) { failed <- err })
	if err := srv.Start(mux); err != nil {
		t.Fatal(err)
	}
	defer srv.Shutdown()

	payload := queue.WindowPayload{TenantID: tenant, ProjectID: proj, From: day.Format(time.DateOnly), To: day.AddDate(0, 0, 1).Format(time.DateOnly)}
	if err := qc.Enqueue(ctx, queue.TaskCalculateCarbon, payload); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(20 * time.Second)
	for {
		var n int
		if err := owner.QueryRow(ctx, `SELECT count(*) FROM carbon.calculations WHERE tenant_id = $1`, tenant).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n == 1 {
			break
		}
		select {
		case err := <-failed:
			t.Fatalf("job failed: %v", err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("carbon calculation did not complete through Asynq within 20s")
		}
		time.Sleep(200 * time.Millisecond)
	}

	// unique jobs: queuing the same sync twice must not error (the second is deduplicated)
	p := queue.TenantPayload{TenantID: tenant, ProjectID: proj, RefID: "00000000-0000-4000-8000-0000000000aa"}
	for i := 0; i < 2; i++ {
		if err := qc.Enqueue(ctx, queue.TaskSyncAWSAccount, p, asynq.Unique(time.Minute), asynq.ProcessIn(time.Hour)); err != nil {
			t.Fatalf("enqueue #%d: %v", i, err)
		}
	}
	// a malformed payload is rejected without retries (SkipRetry), not looped forever
	if err := qc.Enqueue(ctx, queue.TaskCalculateCarbon, map[string]string{"tenant_id": "x", "from": "bad", "to": "bad"}, asynq.MaxRetry(0)); err != nil {
		t.Fatal(err)
	}
	select {
	case <-failed:
	case <-time.After(10 * time.Second):
		t.Error("malformed job was not reported as failed")
	}
}
