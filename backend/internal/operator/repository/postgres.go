// Package repository calls the operator SECURITY DEFINER functions (ADR-0020). There is no tenant context here on
// purpose: the functions are the only cross-tenant door, and each returns metadata only.
package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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

func (r Postgres) Audit(ctx context.Context, limit int) ([]domain.AuditEntry, error) {
	rows, err := r.Pool.Query(ctx, `SELECT id, at, operator_sub, operator_email, role, action FROM platform.operator_audit($1)`, limit)
	return collect(rows, err, func(r pgx.Rows) (e domain.AuditEntry, err error) {
		err = r.Scan(&e.ID, &e.At, &e.OperatorSub, &e.OperatorEmail, &e.Role, &e.Action)
		return
	})
}

func (r Postgres) ProviderUsage(ctx context.Context) ([]domain.ProviderUsage, error) {
	rows, err := r.Pool.Query(ctx, `SELECT provider, connections, organizations, in_error, last_sync_at FROM cloudaccounts.operator_provider_usage()`)
	return collect(rows, err, func(r pgx.Rows) (u domain.ProviderUsage, err error) {
		err = r.Scan(&u.Provider, &u.Connections, &u.Organizations, &u.InError, &u.LastSyncAt)
		return
	})
}

func (r Postgres) Operators(ctx context.Context) ([]domain.OperatorRecord, error) {
	rows, err := r.Pool.Query(ctx, `SELECT auth_sub, email, role, created_at, last_seen_at, actions_30d FROM platform.operator_list()`)
	return collect(rows, err, func(r pgx.Rows) (o domain.OperatorRecord, err error) {
		err = r.Scan(&o.Subject, &o.Email, &o.Role, &o.CreatedAt, &o.LastSeenAt, &o.Reads30d)
		return
	})
}

func (r Postgres) SetSubscription(ctx context.Context, actor, tenantID, plan, status string, trialEnd, periodEnd *time.Time, reason string) (done bool, err error) {
	err = r.Pool.QueryRow(ctx, `SELECT platform.operator_set_subscription($1, $2::uuid, $3, $4, $5, $6, $7)`, actor, tenantID, plan, status, trialEnd, periodEnd, reason).Scan(&done)
	return done, rule(err)
}

func (r Postgres) Subscription(ctx context.Context, tenantID string) (domain.Subscription, error) {
	s := domain.Subscription{Status: "none", Source: "manual"}
	var updated time.Time
	err := r.Pool.QueryRow(ctx, `SELECT plan, status, source, trial_ends_at, period_end, updated_by, updated_at FROM platform.operator_subscription($1::uuid)`, tenantID).
		Scan(&s.Plan, &s.Status, &s.Source, &s.TrialEndsAt, &s.PeriodEnd, &s.UpdatedBy, &updated)
	if errors.Is(err, pgx.ErrNoRows) {
		return s, nil
	}
	if err == nil {
		s.UpdatedAt = &updated
	}
	return s, err
}

func (r Postgres) BillingEvents(ctx context.Context, limit int) ([]domain.BillingEvent, error) {
	rows, err := r.Pool.Query(ctx, `SELECT at, provider, kind, organization_id::text, organization_name, plan, detail FROM platform.operator_billing_events($1)`, limit)
	return collect(rows, err, func(r pgx.Rows) (e domain.BillingEvent, err error) {
		err = r.Scan(&e.At, &e.Provider, &e.Kind, &e.OrganizationID, &e.OrganizationName, &e.Plan, &e.Detail)
		return
	})
}

// rule turns a function's own refusal (SQLSTATE P0001 with a message written for people) into domain.ErrRule.
func rule(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "P0001" {
		return fmt.Errorf("%w: %s", domain.ErrRule, pg.Message)
	}
	return err
}

func (r Postgres) AssignOperator(ctx context.Context, actor, email string, role domain.Role) (sub string, err error) {
	err = r.Pool.QueryRow(ctx, `SELECT platform.operator_assign($1, $2, $3)`, actor, email, string(role)).Scan(&sub)
	return sub, rule(err)
}

func (r Postgres) WithdrawOperator(ctx context.Context, actor, subject string) (done bool, err error) {
	err = r.Pool.QueryRow(ctx, `SELECT platform.operator_withdraw($1, $2)`, actor, subject).Scan(&done)
	return done, rule(err)
}

func (r Postgres) CarbonRegions(ctx context.Context) ([]string, error) {
	rows, err := r.Pool.Query(ctx, `SELECT region_id FROM carbon.operator_regions()`)
	return collect(rows, err, func(r pgx.Rows) (v string, err error) { err = r.Scan(&v); return })
}

func (r Postgres) APIUsage(ctx context.Context, days int) ([]domain.APIUsage, error) {
	rows, err := r.Pool.Query(ctx, `SELECT provider, day::timestamptz, requests, errors FROM platform.operator_api_usage($1)`, days)
	return collect(rows, err, func(r pgx.Rows) (u domain.APIUsage, err error) {
		err = r.Scan(&u.Provider, &u.Day, &u.Requests, &u.Errors)
		return
	})
}

func (r Postgres) CarbonRegionSources(ctx context.Context) ([]domain.RegionSourceRow, error) {
	rows, err := r.Pool.Query(ctx, `SELECT region_id, provider, hourly_30d, months, newest FROM carbon.operator_region_sources()`)
	return collect(rows, err, func(r pgx.Rows) (x domain.RegionSourceRow, err error) {
		err = r.Scan(&x.Region, &x.Provider, &x.Hourly, &x.Months, &x.Newest)
		return
	})
}

func (r Postgres) OrgUsage(ctx context.Context, days int) ([]domain.OrgUsage, error) {
	rows, err := r.Pool.Query(ctx, `SELECT tenant_id::text, name, plan, usage_rows, calculation_rows, sync_runs, deliveries, reports, k8s_samples, members, connections FROM platform.operator_org_usage($1)`, days)
	return collect(rows, err, func(r pgx.Rows) (u domain.OrgUsage, err error) {
		err = r.Scan(&u.TenantID, &u.Name, &u.Plan, &u.UsageRows, &u.CalculationRows, &u.SyncRuns, &u.Deliveries, &u.Reports, &u.K8sSamples, &u.Members, &u.Connections)
		return
	})
}
