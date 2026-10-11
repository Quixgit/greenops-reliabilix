package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/quixgit/greenops-reliabilix/backend/internal/notifications/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/database"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/notify"
)

// Add keeps one event in the organization's inbox. created is false when the same dedupe key was kept before.
// Items older than 90 days are dropped by the same call, so the table stays small without a separate job.
func (r Postgres) InboxAdd(ctx context.Context, tenantID string, e notify.Event) (created bool, err error) {
	err = database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		var key *string
		if e.DedupeKey != "" {
			key = &e.DedupeKey
		}
		tag, err := tx.Exec(ctx, `INSERT INTO notifications.inbox (tenant_id, type, severity, title, body, path, dedupe_key)
			VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT DO NOTHING`, tenantID, e.Type, string(e.Severity), e.Title, e.Body, e.Path, key)
		if err != nil {
			return err
		}
		created = tag.RowsAffected() == 1
		_, err = tx.Exec(ctx, `DELETE FROM notifications.inbox WHERE tenant_id = $1 AND created_at < now() - interval '90 days'`, tenantID)
		return err
	})
	return
}

// InboxList returns the newest items and the moment the person last marked everything as read.
func (r Postgres) InboxList(ctx context.Context, tenantID, userID string, limit int) (items []domain.InboxItem, readUntil time.Time, err error) {
	items = []domain.InboxItem{}
	err = database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT COALESCE((SELECT read_until FROM notifications.inbox_state WHERE tenant_id = $1 AND user_id = $2), 'epoch'::timestamptz)`,
			tenantID, userID).Scan(&readUntil); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT id::text, type, severity, title, body, path, created_at FROM notifications.inbox
			WHERE tenant_id = $1 ORDER BY created_at DESC LIMIT $2`, tenantID, limit)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var it domain.InboxItem
			var sev string
			if err := rows.Scan(&it.ID, &it.Type, &sev, &it.Title, &it.Body, &it.Path, &it.CreatedAt); err != nil {
				return err
			}
			it.Severity = notify.Severity(sev)
			items = append(items, it)
		}
		return rows.Err()
	})
	return
}

// InboxMarkRead records that the person has seen everything up to now.
func (r Postgres) InboxMarkRead(ctx context.Context, tenantID, userID string) error {
	return database.WithTenantTx(ctx, r.Pool, tenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO notifications.inbox_state (tenant_id, user_id, read_until) VALUES ($1,$2,now())
			ON CONFLICT (tenant_id, user_id) DO UPDATE SET read_until = now()`, tenantID, userID)
		return err
	})
}
