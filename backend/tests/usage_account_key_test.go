//go:build integration

package tests

import (
	"context"
	"testing"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/focus"
	usagerepo "github.com/quixgit/greenops-reliabilix/backend/internal/usage/repository"
)

// Two accounts of one provider in one project must keep their own lines, and connecting the same account again must
// update its lines instead of doubling them.
func TestUsageLinesAreKeyedByAccount(t *testing.T) {
	ctx := context.Background()
	owner, worker := pool(t, "TEST_OWNER_DSN"), pool(t, "TEST_WORKER_DSN")
	tenant := org(t, owner, "Account Key Org")
	var project string
	if err := owner.QueryRow(ctx, `INSERT INTO projects.projects (tenant_id, name, functional_unit) VALUES ($1,'p','req') RETURNING id::text`, tenant).Scan(&project); err != nil {
		t.Fatal(err)
	}
	conn := func(account string) string {
		var id string
		if err := owner.QueryRow(ctx, `INSERT INTO cloudaccounts.connections (tenant_id, project_id, provider, account_ref, credential_ref, external_id)
			VALUES ($1,$2,'aws',$3,'arn:aws:iam::'||$3||':role/ReliabilixR','rlx-'||$3) RETURNING id::text`, tenant, project, account).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	a, b := conn("111111111111"), conn("222222222222")
	day := time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC)
	line := func(cost float64) []focus.Record {
		return []focus.Record{{Provider: "aws", Source: "cost_explorer", ServiceName: "Amazon S3", ServiceCategory: "storage", RegionID: "eu-central-1",
			BilledCost: cost, EffectiveCost: cost, Currency: "USD", ChargePeriodStart: day, ChargePeriodEnd: day.AddDate(0, 0, 1)}}
	}
	repo := usagerepo.Postgres{Pool: worker}
	if err := repo.Upsert(ctx, tenant, project, a, line(10)); err != nil {
		t.Fatal(err)
	}
	if err := repo.Upsert(ctx, tenant, project, b, line(7)); err != nil {
		t.Fatal(err)
	}
	total := func() (n int, sum float64) {
		if err := owner.QueryRow(ctx, `SELECT count(*), COALESCE(sum(billed_cost),0)::float8 FROM usage.usage_records WHERE tenant_id=$1`, tenant).Scan(&n, &sum); err != nil {
			t.Fatal(err)
		}
		return
	}
	if n, sum := total(); n != 2 || sum != 17 {
		t.Fatalf("two accounts must keep their own lines: %d lines, %v", n, sum)
	}
	// the same account again (a re-sync, or the account connected anew under another connection id) updates its line
	if err := repo.Upsert(ctx, tenant, project, a, line(12)); err != nil {
		t.Fatal(err)
	}
	if n, sum := total(); n != 2 || sum != 19 {
		t.Fatalf("a re-sync must update, not duplicate: %d lines, %v", n, sum)
	}
	if _, err := owner.Exec(ctx, `DELETE FROM cloudaccounts.connections WHERE id = $1`, a); err != nil {
		t.Fatal(err)
	}
	a2 := conn("111111111111")
	if err := repo.Upsert(ctx, tenant, project, a2, line(13)); err != nil {
		t.Fatal(err)
	}
	if n, sum := total(); n != 2 || sum != 20 {
		t.Fatalf("a reconnected account must update its old lines: %d lines, %v", n, sum)
	}
}
