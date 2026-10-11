//go:build integration

package tests

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// Rows of a month that arrived before its partition exists sit in the DEFAULT partition and used to block the creation
// of that month's partition for good, failing every sync that touched the month.
func TestPartitionCreationMovesRowsHeldByTheDefaultPartition(t *testing.T) {
	ctx := context.Background()
	owner, worker := pool(t, "TEST_OWNER_DSN"), pool(t, "TEST_WORKER_DSN")
	tenant := org(t, owner, "Partition Org")
	var project string
	if err := owner.QueryRow(ctx, `INSERT INTO projects.projects (tenant_id, name, functional_unit) VALUES ($1,'p','req') RETURNING id::text`, tenant).Scan(&project); err != nil {
		t.Fatal(err)
	}
	// a month far enough back that no partition was created for it by migrations or by other tests
	month := time.Date(2019, 3, 1, 0, 0, 0, 0, time.UTC)
	part := "usage.usage_records_" + month.Format("2006_01")
	var existing *string
	_ = owner.QueryRow(ctx, `SELECT to_regclass($1)::text`, part).Scan(&existing)
	if existing != nil {
		t.Skipf("%s already exists in this database", part)
	}
	for i := 0; i < 5; i++ {
		at := month.AddDate(0, 0, i)
		if _, err := owner.Exec(ctx, `INSERT INTO usage.usage_records (tenant_id, project_id, source, provider, service_name, service_category, region_id, billed_cost, effective_cost, currency, recorded_at, charge_period_end)
			VALUES ($1,$2,'cost_explorer','aws','EC2','compute','eu-central-1',$3,$3,'USD',$4::timestamptz,$4::timestamptz + interval '1 day')`, tenant, project, 10+i, at); err != nil {
			t.Fatal(err)
		}
	}
	var inDefault int
	_ = owner.QueryRow(ctx, `SELECT count(*) FROM usage.usage_records_default WHERE tenant_id=$1`, tenant).Scan(&inDefault)
	if inDefault != 5 {
		t.Fatalf("the rows must start in the default partition: %d", inDefault)
	}

	// the sync's path: the worker role asks for the month's partition
	if _, err := worker.Exec(ctx, `SELECT platform.ensure_month_partition('usage.usage_records', 'recorded_at', $1::date)`, month); err != nil {
		t.Fatalf("creating the partition must work although the default partition holds the month: %v", err)
	}
	var inPart, left int
	var sum float64
	if err := owner.QueryRow(ctx, fmt.Sprintf(`SELECT count(*), COALESCE(sum(billed_cost),0)::float8 FROM %s WHERE tenant_id=$1`, part), tenant).Scan(&inPart, &sum); err != nil {
		t.Fatal(err)
	}
	_ = owner.QueryRow(ctx, `SELECT count(*) FROM usage.usage_records_default WHERE tenant_id=$1`, tenant).Scan(&left)
	if inPart != 5 || sum != 10+11+12+13+14 || left != 0 {
		t.Errorf("rows must move, none lost: partition=%d sum=%v default=%d", inPart, sum, left)
	}
	// and the table still answers as a whole
	var total int
	_ = owner.QueryRow(ctx, `SELECT count(*) FROM usage.usage_records WHERE tenant_id=$1`, tenant).Scan(&total)
	if total != 5 {
		t.Errorf("total = %d", total)
	}
	// calling again is harmless
	if _, err := worker.Exec(ctx, `SELECT platform.ensure_month_partition('usage.usage_records', 'recorded_at', $1::date)`, month); err != nil {
		t.Errorf("second call: %v", err)
	}
}
