// Package audit records who did what. Entries are written inside the same
// tenant transaction as the change they describe, so they cannot diverge.
package audit

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
	httpx "github.com/quixgit/greenops-reliabilix/backend/internal/platform/http"
)

// Record inserts an audit entry for the caller found in ctx. metadata must not
// contain secrets or personal data beyond ids.
func Record(ctx context.Context, tx pgx.Tx, action, target string, metadata map[string]any) error {
	c, ok := auth.FromContext(ctx)
	if !ok || c.Subject == "" {
		c.Subject = "system" // background jobs act without a user
	}
	if metadata == nil {
		metadata = map[string]any{} // the column is NOT NULL: a nil map would be written as NULL
	}
	_, err := tx.Exec(ctx,
		`INSERT INTO audit.audit_logs (tenant_id, actor_user_id, action, target, request_id, metadata)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		tenantFrom(ctx, c), c.Subject, action, target, httpx.RequestID(ctx), metadata)
	return err
}

// tenantFrom prefers the tenant of the identity; background jobs set it with WithTenant.
func tenantFrom(ctx context.Context, c auth.Claims) string {
	if c.TenantID != "" {
		return c.TenantID
	}
	t, _ := ctx.Value(tenantKey{}).(string)
	return t
}

type tenantKey struct{}

// WithTenant tags ctx with the tenant a background job works for, so Record can attribute entries.
func WithTenant(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantKey{}, tenantID)
}
