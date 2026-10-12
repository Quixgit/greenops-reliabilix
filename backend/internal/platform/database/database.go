// Package database provides the pgx pool and tenant-scoped transactions.
package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Connect opens a pool. The DSN's role decides privileges: api and worker use
// different least-privilege roles (see migrations/0001).
func Connect(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("database: parse dsn: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

type cloudScopeKey struct{}

// WithCloudScope marks a request as looking at one cloud only: every tenant transaction opened with the returned context also binds
// app.cloud, and restrictive row policies (migration 0080) then hide the rows of the other clouds, in every query of the request. The
// reporting currency of such a request is the cloud's own (finops.reporting_currency). Only reads are ever scoped (the middleware sets it
// on GET requests), so a write path never sees a partial table.
func WithCloudScope(ctx context.Context, cloud string) context.Context {
	return context.WithValue(ctx, cloudScopeKey{}, cloud)
}

// CloudScope is the cloud the context is scoped to, or "".
func CloudScope(ctx context.Context) string {
	c, _ := ctx.Value(cloudScopeKey{}).(string)
	return c
}

// WithTenantTx runs fn in a transaction with the transaction-local setting
// app.tenant_id bound to tenantID. Row Level Security policies read it, so a
// forgotten "WHERE tenant_id" can never return another tenant's rows. Every
// tenant-scoped query MUST run through this helper.
func WithTenantTx(ctx context.Context, pool *pgxpool.Pool, tenantID string, fn func(pgx.Tx) error) error {
	return WithTenantTxOpts(ctx, pool, tenantID, pgx.TxOptions{}, fn)
}

// WithTenantTxOpts is WithTenantTx with transaction options, for example a read-only repeatable-read snapshot.
func WithTenantTxOpts(ctx context.Context, pool *pgxpool.Pool, tenantID string, opts pgx.TxOptions, fn func(pgx.Tx) error) error {
	if tenantID == "" {
		return errors.New("database: empty tenant id")
	}
	tx, err := pool.BeginTx(ctx, opts)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op after Commit
	if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenantID); err != nil {
		return err
	}
	if cloud := CloudScope(ctx); cloud != "" {
		if _, err := tx.Exec(ctx, "SELECT set_config('app.cloud', $1, true)", cloud); err != nil {
			return err
		}
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// IsUniqueViolation reports a PostgreSQL unique-constraint violation (SQLSTATE 23505).
func IsUniqueViolation(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23505"
}

// IsNotFound reports pgx.ErrNoRows.
func IsNotFound(err error) bool { return errors.Is(err, pgx.ErrNoRows) }
