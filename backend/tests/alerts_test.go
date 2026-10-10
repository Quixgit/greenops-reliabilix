//go:build integration

package tests

import (
	"context"
	"testing"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/alerts/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/alerts/repository"
)

func TestAlertLifecycleInTheDatabase(t *testing.T) {
	ctx := context.Background()
	owner, workerPool, apiPool := pool(t, "TEST_OWNER_DSN"), pool(t, "TEST_WORKER_DSN"), pool(t, "TEST_API_DSN")
	tenant := org(t, owner, "Alerts Org")
	t.Cleanup(func() { _, _ = owner.Exec(ctx, `DELETE FROM tenants.organizations WHERE id = $1`, tenant) })
	worker, api := repository.Postgres{Pool: workerPool}, repository.Postgres{Pool: apiPool}
	now := time.Now().UTC()

	if empty, err := api.List(ctx, tenant, "", 10); err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("an empty list must be a non-nil empty slice: %v %v", empty, err)
	}
	v := 120.0
	c := domain.Condition{DedupeKey: "budget:b1", Kind: domain.KindBudget, Severity: domain.Critical, Title: "over", Value: &v}
	a, created, err := worker.Open(ctx, tenant, c, now)
	if err != nil || !created || a.Status != domain.New {
		t.Fatalf("open: %+v %v %v", a, created, err)
	}
	// The same condition again must not create a second live alert.
	if _, again, err := worker.Open(ctx, tenant, c, now); err != nil || again {
		t.Fatalf("second open: created=%v err=%v", again, err)
	}
	// A person acknowledges (API role), then mutes; an invalid change is refused on the locked row.
	ack, err := api.SetStatus(ctx, tenant, a.ID, domain.Acknowledged, "someone")
	if err != nil || ack.Status != domain.Acknowledged || ack.AcknowledgedBy == nil {
		t.Fatalf("ack: %+v %v", ack, err)
	}
	if _, err := api.SetStatus(ctx, tenant, a.ID, domain.New, "someone"); err == nil {
		t.Fatal("acknowledged -> new must be refused")
	}
	// The evaluation resolves it; the same condition later opens a NEW alert (history is kept).
	if err := worker.Resolve(ctx, tenant, []string{a.ID}, now); err != nil {
		t.Fatal(err)
	}
	b, created, err := worker.Open(ctx, tenant, c, now)
	if err != nil || !created || b.ID == a.ID {
		t.Fatalf("reopen: %+v created=%v err=%v", b, created, err)
	}
	live, err := worker.Live(ctx, tenant)
	if err != nil || len(live) != 1 || live[0].ID != b.ID {
		t.Fatalf("live: %+v %v", live, err)
	}
	all, err := api.List(ctx, tenant, "", 10)
	if err != nil || len(all) != 2 || all[len(all)-1].Status != domain.Resolved {
		t.Fatalf("list must show history with resolved last: %+v %v", all, err)
	}
}
