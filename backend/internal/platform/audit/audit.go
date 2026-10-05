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
	c, _ := auth.FromContext(ctx)
	_, err := tx.Exec(ctx,
		`INSERT INTO audit.audit_logs (tenant_id, actor_user_id, action, target, request_id, metadata)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		c.TenantID, c.Subject, action, target, httpx.RequestID(ctx), metadata)
	return err
}
