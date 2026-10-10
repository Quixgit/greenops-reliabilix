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
	return nil
}
