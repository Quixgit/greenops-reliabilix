package app

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/database"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/entitlements"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/settings"
)

// limitsGate enforces the limits.* settings. It lives in the composition root because it counts rows owned by several
// domains (read-only, inside the organization's own tenant transaction, so RLS applies).
type limitsGate struct {
	pool *pgxpool.Pool
	cfg  settings.Reader
}

const platformDestinationCap = 200

var limitKeys = map[entitlements.Resource]string{
	entitlements.Projects:     settings.KeyLimitProjects,
	entitlements.Connections:  settings.KeyLimitConnections,
	entitlements.Members:      settings.KeyLimitMembers,
	entitlements.Destinations: settings.KeyLimitDestinations,
}

var countSQL = map[entitlements.Resource]string{
	entitlements.Projects:     `SELECT count(*) FROM projects.projects`,
	entitlements.Connections:  `SELECT count(*) FROM cloudaccounts.connections`,
	entitlements.Members:      `SELECT (SELECT count(*) FROM tenants.memberships) + (SELECT count(*) FROM tenants.invitations WHERE accepted_at IS NULL AND expires_at > now())`,
	entitlements.Destinations: `SELECT count(*) FROM notifications.destinations`,
}

func (g limitsGate) Allow(ctx context.Context, tenantID string, r entitlements.Resource) error {
	limit := g.cfg.Int(ctx, tenantID, limitKeys[r])
	if limit == 0 && r == entitlements.Destinations {
		limit = platformDestinationCap
	}
	if limit == 0 { // 0 = unlimited
		return nil
	}
	var used int64
	err := database.WithTenantTx(ctx, g.pool, tenantID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, countSQL[r]).Scan(&used)
	})
	if err != nil {
		return err
	}
	if used >= limit {
		return entitlements.LimitError{Resource: r, Limit: limit}
	}
	if r == entitlements.Connections {
		return g.allowSpend(ctx, tenantID)
	}
	return nil
}

// trackedSpendSQL is the cloud spend of the last 30 days in US dollars at each day's reference rate; an amount in a
// currency without a rate is left out (the same rule as every other total). Shared with the plan page.
const trackedSpendSQL = `SELECT COALESCE(sum(finops.convert_cost(billed_cost, currency::text, 'USD', recorded_at::date)), 0)::bigint
	FROM usage.usage_records WHERE recorded_at >= now() - interval '30 days'`

// spendRefusalPercent is how far above the plan's tracked spend a new connection is still allowed: the plan page warns at
// 100%, and only a customer well past it is asked to change the plan (ADR-0050). Data already tracked is never touched.
const spendRefusalPercent = 120

// allowSpend refuses one more cloud connection once the spend already under management is above the plan's limit by the margin.
func (g limitsGate) allowSpend(ctx context.Context, tenantID string) error {
	limit := g.cfg.Int(ctx, tenantID, settings.KeyLimitTrackedSpend)
	if limit == 0 {
		return nil
	}
	var spent int64
	if err := database.WithTenantTx(ctx, g.pool, tenantID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, trackedSpendSQL).Scan(&spent)
	}); err != nil {
		return err
	}
	if spent*100 > limit*spendRefusalPercent {
		return entitlements.LimitError{Resource: entitlements.TrackedSpend, Limit: limit}
	}
	return nil
}
