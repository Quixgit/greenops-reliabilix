// Package repository calls the operator SECURITY DEFINER functions (ADR-0020). There is no tenant context here on
// purpose: the functions are the only cross-tenant door, and each returns metadata only.
package repository

import (
	"context"
	"database/sql"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/quixgit/greenops-reliabilix/backend/internal/operator/domain"
)

type Postgres struct{ Pool *pgxpool.Pool }

var _ domain.Repository = Postgres{}

func (r Postgres) Role(ctx context.Context, subject string) (domain.Role, bool, error) {
	var role sql.NullString
	if err := r.Pool.QueryRow(ctx, `SELECT platform.operator_role($1)`, subject).Scan(&role); err != nil {
		return "", false, err
	}
	return domain.Role(role.String), role.Valid, nil
}

func (r Postgres) LogAccess(ctx context.Context, subject, action string) error {
	_, err := r.Pool.Exec(ctx, `SELECT platform.operator_log($1, $2)`, subject, action)
	return err
}

func collect[T any](rows pgx.Rows, err error, scan func(pgx.Rows) (T, error)) ([]T, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []T{}
	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r Postgres) Tenants(ctx context.Context, limit int) ([]domain.Tenant, error) {
	rows, err := r.Pool.Query(ctx, `SELECT tenant_id::text, name, plan, created_at, members, projects, connections, connections_in_error, destinations, failed_deliveries_7d
		FROM platform.operator_tenants($1)`, limit)
	return collect(rows, err, func(r pgx.Rows) (t domain.Tenant, err error) {
		err = r.Scan(&t.ID, &t.Name, &t.Plan, &t.CreatedAt, &t.Members, &t.Projects, &t.Connections, &t.ConnectionsInError, &t.Destinations, &t.FailedDeliveries7d)
		return
	})
}

func (r Postgres) Deliveries(ctx context.Context, status string, limit int) ([]domain.Delivery, error) {
	var st *string
	if status != "" {
		st = &status
	}
	rows, err := r.Pool.Query(ctx, `SELECT tenant_id::text, tenant_name, id::text, destination_id::text, kind, destination_name, event_type, severity,
		status, attempts, next_attempt_at, last_error, created_at, sent_at FROM notifications.operator_deliveries($1, $2)`, st, limit)
	return collect(rows, err, func(r pgx.Rows) (d domain.Delivery, err error) {
		err = r.Scan(&d.TenantID, &d.TenantName, &d.ID, &d.DestinationID, &d.Kind, &d.DestinationName, &d.EventType, &d.Severity,
			&d.Status, &d.Attempts, &d.NextAttemptAt, &d.LastError, &d.CreatedAt, &d.SentAt)
		return
	})
}

func (r Postgres) DeliveryStats(ctx context.Context, hours int) ([]domain.DeliveryStat, error) {
	rows, err := r.Pool.Query(ctx, `SELECT kind, status, n FROM notifications.operator_stats($1)`, hours)
	return collect(rows, err, func(r pgx.Rows) (s domain.DeliveryStat, err error) {
		err = r.Scan(&s.Kind, &s.Status, &s.Count)
		return
	})
}

func (r Postgres) UnhealthyConnections(ctx context.Context, limit int) ([]domain.Connection, error) {
	rows, err := r.Pool.Query(ctx, `SELECT tenant_id::text, tenant_name, provider, account_ref, sync_status, last_error, last_sync_at
		FROM cloudaccounts.operator_unhealthy($1)`, limit)
	return collect(rows, err, func(r pgx.Rows) (c domain.Connection, err error) {
		err = r.Scan(&c.TenantID, &c.TenantName, &c.Provider, &c.AccountRef, &c.SyncStatus, &c.LastError, &c.LastSyncAt)
		return
	})
}

func (r Postgres) RetryDelivery(ctx context.Context, actor, tenantID, deliveryID string) (done bool, err error) {
	err = r.Pool.QueryRow(ctx, `SELECT notifications.operator_retry($1, $2::uuid, $3::uuid)`, actor, tenantID, deliveryID).Scan(&done)
	return
}

func (r Postgres) SetDestinationEnabled(ctx context.Context, actor, tenantID, destinationID string, enabled bool) (done bool, err error) {
	err = r.Pool.QueryRow(ctx, `SELECT notifications.operator_set_enabled($1, $2::uuid, $3::uuid, $4)`, actor, tenantID, destinationID, enabled).Scan(&done)
	return
}

func (r Postgres) Errors(ctx context.Context, status, service string, limit int) ([]domain.ErrorGroup, error) {
	var st, sv *string
	if status != "" {
		st = &status
	}
	if service != "" {
		sv = &service
	}
	rows, err := r.Pool.Query(ctx, `SELECT id::text, service, component, severity, message, status, occurrences, first_seen, last_seen,
		tenant_id::text, tenant_name, last_request_id, handled_by, handled_at FROM platform.operator_errors($1, $2, $3)`, st, sv, limit)
	return collect(rows, err, func(r pgx.Rows) (e domain.ErrorGroup, err error) {
		err = r.Scan(&e.ID, &e.Service, &e.Component, &e.Severity, &e.Message, &e.Status, &e.Occurrences, &e.FirstSeen, &e.LastSeen,
			&e.TenantID, &e.TenantName, &e.RequestID, &e.HandledBy, &e.HandledAt)
		return
	})
}

func (r Postgres) SetErrorStatus(ctx context.Context, actor, id, status string) (done bool, err error) {
	err = r.Pool.QueryRow(ctx, `SELECT platform.operator_error_set_status($1, $2::uuid, $3)`, actor, id, status).Scan(&done)
	return
}
