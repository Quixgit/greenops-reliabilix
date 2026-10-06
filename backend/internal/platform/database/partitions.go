package database

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// EnsurePartitions creates monthly partitions for the current and next two
// months of every partitioned table (usage.usage_records, carbon.calculations).
func EnsurePartitions(ctx context.Context, pool *pgxpool.Pool, now time.Time) error {
	for i := 0; i < 3; i++ {
		d := time.Date(now.Year(), now.Month()+time.Month(i), 1, 0, 0, 0, 0, time.UTC)
		if _, err := pool.Exec(ctx, `SELECT platform.ensure_month_partition('usage.usage_records', 'recorded_at', $1::date),
			platform.ensure_month_partition('carbon.calculations', 'period_start', $1::date)`, d); err != nil {
			return err
		}
	}
	return nil
}
