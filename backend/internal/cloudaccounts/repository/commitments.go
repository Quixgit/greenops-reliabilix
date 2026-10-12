package repository

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/database"
)

// SaveCommitments replaces what is known about the connection: the kinds read now are stored, a kind not read now is removed (the account no
// longer holds any, or the reading found none).
func (r Postgres) SaveCommitments(ctx context.Context, tenantID, connectionID string, snaps []domain.CommitmentSnapshot) error {
	return database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		kinds := make([]string, 0, len(snaps))
		for _, s := range snaps {
			kinds = append(kinds, s.Kind)
			if _, err := tx.Exec(ctx, `INSERT INTO cloudaccounts.commitment_snapshots
				(connection_id, tenant_id, kind, period_from, period_to, currency, utilization_pct, coverage_pct, commitment, unused, net_savings, fetched_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, now())
				ON CONFLICT (connection_id, kind) DO UPDATE SET period_from = EXCLUDED.period_from, period_to = EXCLUDED.period_to, currency = EXCLUDED.currency,
				  utilization_pct = EXCLUDED.utilization_pct, coverage_pct = EXCLUDED.coverage_pct, commitment = EXCLUDED.commitment,
				  unused = EXCLUDED.unused, net_savings = EXCLUDED.net_savings, fetched_at = now()`,
				connectionID, tenantID, s.Kind, s.PeriodFrom, s.PeriodTo, s.Currency, s.UtilizationPct, s.CoveragePct, s.Commitment, s.Unused, s.NetSavings); err != nil {
				return err
			}
		}
		_, err := tx.Exec(ctx, `DELETE FROM cloudaccounts.commitment_snapshots WHERE connection_id = $1 AND NOT (kind = ANY($2::text[]))`, connectionID, kinds)
		return err
	})
}

// ListCommitments returns the snapshots of every connection of the organization, with the connection's provider and account.
func (r Postgres) ListCommitments(ctx context.Context, tenantID string) (out []domain.CommitmentSnapshot, err error) {
	out = []domain.CommitmentSnapshot{}
	err = database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT s.connection_id::text, c.provider, c.account_ref, s.kind, s.period_from, s.period_to, s.currency::text,
				s.utilization_pct::float8, s.coverage_pct::float8, s.commitment::float8, s.unused::float8, s.net_savings::float8, s.fetched_at
			FROM cloudaccounts.commitment_snapshots s JOIN cloudaccounts.connections c ON c.id = s.connection_id
			ORDER BY c.provider, c.account_ref, s.kind`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var x domain.CommitmentSnapshot
			if err := rows.Scan(&x.ConnectionID, &x.Provider, &x.Account, &x.Kind, &x.PeriodFrom, &x.PeriodTo, &x.Currency,
				&x.UtilizationPct, &x.CoveragePct, &x.Commitment, &x.Unused, &x.NetSavings, &x.FetchedAt); err != nil {
				return err
			}
			out = append(out, x)
		}
		return rows.Err()
	})
	return
}
