// Package domain defines the platform operator console: who may use it and the cross-tenant metadata it shows.
// Everything here is metadata about the platform's own operation (counts, statuses, error texts); the console never
// carries tenant payloads, destination addresses, tokens or usage data.
package domain

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/integrations"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/settings"
)

type Role string

const (
	Viewer Role = "viewer" // read only
	Admin  Role = "admin"  // may also retry deliveries and disable destinations
)

func (r Role) Valid() bool    { return r == Viewer || r == Admin }
func (r Role) CanWrite() bool { return r == Admin }

var (
	ErrNotOperator = errors.New("not a platform operator")
	ErrReadOnly    = errors.New("operator role is read-only")
	ErrNotFound    = errors.New("not found")
	ErrInvalid     = errors.New("invalid request")
)

type Operator struct {
	Subject string `json:"subject"`
	Email   string `json:"email"`
	Role    Role   `json:"role"`
}

type Tenant struct {
	ID                 string    `json:"id"`
	Name               string    `json:"name"`
	Plan               string    `json:"plan"`
	CreatedAt          time.Time `json:"created_at"`
	Members            int       `json:"members"`
	Projects           int       `json:"projects"`
	Connections        int       `json:"connections"`
	ConnectionsInError int       `json:"connections_in_error"`
	Destinations       int       `json:"destinations"`
	FailedDeliveries7d int       `json:"failed_deliveries_7d"`
}

type Delivery struct {
	TenantID        string     `json:"tenant_id"`
	TenantName      string     `json:"tenant_name"`
	ID              string     `json:"id"`
	DestinationID   *string    `json:"destination_id"`
	Kind            string     `json:"kind"`
	DestinationName string     `json:"destination_name"`
	EventType       string     `json:"event_type"`
	Severity        string     `json:"severity"`
	Status          string     `json:"status"`
	Attempts        int        `json:"attempts"`
	NextAttemptAt   time.Time  `json:"next_attempt_at"`
	LastError       *string    `json:"last_error"`
	CreatedAt       time.Time  `json:"created_at"`
	SentAt          *time.Time `json:"sent_at"`
}

type DeliveryStat struct {
	Kind   string `json:"kind"`
	Status string `json:"status"`
	Count  int64  `json:"count"`
}

type Connection struct {
	TenantID   string     `json:"tenant_id"`
	TenantName string     `json:"tenant_name"`
	Provider   string     `json:"provider"`
	AccountRef string     `json:"account_ref"`
	SyncStatus string     `json:"sync_status"`
	LastError  *string    `json:"last_error"`
	LastSyncAt *time.Time `json:"last_sync_at"`
}

// Integration describes one external dependency. Status "ready" means a real credential is configured,
// "stub" means a development stand-in is running, "missing" means the feature is unavailable until it is set up.
// ErrorGroup is one kind of error of a platform service, with how often and when it happened (ADR-0021).
type ErrorGroup struct {
	ID          string     `json:"id"`
	Service     string     `json:"service"`
	Component   string     `json:"component"`
	Severity    string     `json:"severity"`
	Message     string     `json:"message"`
	Status      string     `json:"status"`
	Occurrences int64      `json:"occurrences"`
	FirstSeen   time.Time  `json:"first_seen"`
	LastSeen    time.Time  `json:"last_seen"`
	TenantID    *string    `json:"tenant_id"`
	TenantName  *string    `json:"tenant_name"`
	RequestID   *string    `json:"last_request_id"`
	HandledBy   *string    `json:"handled_by"`
	HandledAt   *time.Time `json:"handled_at"`
}

var ErrorStatuses = map[string]bool{"open": true, "resolved": true, "ignored": true}

type Integration struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Note   string `json:"note"`
}

type System struct {
	Version           string        `json:"version"`
	Env               string        `json:"env"`
	ExpectedMigration int           `json:"expected_migration"`
	Integrations      []Integration `json:"integrations"`
}

var DeliveryStatuses = map[string]bool{"pending": true, "sent": true, "failed": true, "skipped": true}

// Repository reads across tenants through narrow SECURITY DEFINER functions.
type Repository interface {
	Role(ctx context.Context, subject string) (Role, bool, error)
	LogAccess(ctx context.Context, subject, action string) error
	Tenants(ctx context.Context, limit int) ([]Tenant, error)
	Deliveries(ctx context.Context, status string, limit int) ([]Delivery, error)
	DeliveryStats(ctx context.Context, hours int) ([]DeliveryStat, error)
	UnhealthyConnections(ctx context.Context, limit int) ([]Connection, error)
	Errors(ctx context.Context, status, service string, limit int) ([]ErrorGroup, error)
	SetErrorStatus(ctx context.Context, actor, id, status string) (bool, error)
	RetryDelivery(ctx context.Context, actor, tenantID, deliveryID string) (bool, error)
	SetDestinationEnabled(ctx context.Context, actor, tenantID, destinationID string, enabled bool) (bool, error)
}

// IntegrationsPort is what the console needs to manage external-service credentials (ADR-0029), wired in
// internal/app. Values of secret fields never come back through it.
type IntegrationsPort interface {
	Statuses(ctx context.Context) ([]integrations.Status, error)
	Set(ctx context.Context, actor, integration string, values map[string]*string) error
	// Test checks the integration with its current values and returns a short, safe message.
	Test(ctx context.Context, integration string) (string, error)
}

// SettingsPort is what the console needs from the settings engine (wired in internal/app).
type SettingsPort interface {
	Groups() []settings.Group
	PlatformItems(ctx context.Context) ([]settings.Item, error)
	SetPlatform(ctx context.Context, actor string, values map[string]json.RawMessage) error
	ResetPlatform(ctx context.Context, actor string, keys []string) error
	OperatorTenantItems(ctx context.Context, tenantID string) ([]settings.Item, error)
	SetTenantOverride(ctx context.Context, actor, tenantID string, values map[string]json.RawMessage) error
	ClearTenantOverride(ctx context.Context, actor, tenantID string, keys []string) error
	OperatorHistory(ctx context.Context, tenantID *string, limit int) ([]settings.HistoryEntry, error)
}
