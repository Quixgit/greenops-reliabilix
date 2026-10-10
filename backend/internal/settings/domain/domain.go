// Package domain defines how setting values are stored and who may change them. The catalog itself (what settings
// exist, their types and bounds) is platform/settings; this domain only holds values.
package domain

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/settings"
)

var (
	ErrLocked   = errors.New("this setting is fixed by the platform for your organization")
	ErrNotFound = settings.ErrNotFound
)

// Stored is a persisted value.
type Stored struct {
	Value     json.RawMessage
	Source    settings.Source // tenant | operator for organization values, platform for platform values
	UpdatedBy string
	UpdatedAt time.Time
}

// Change is one write inside a transaction. New == nil means "remove the organization's own value".
type Change struct {
	Key string
	Old json.RawMessage // previous stored value or nil, for the history
	New json.RawMessage
}

type Repository interface {
	PlatformValues(ctx context.Context) (map[string]Stored, error)
	TenantValues(ctx context.Context, tenantID string) (map[string]Stored, error)
	// ApplyTenant writes all changes and their history entries (and an audit record) atomically.
	ApplyTenant(ctx context.Context, tenantID, actor string, changes []Change) error
	TenantHistory(ctx context.Context, tenantID string, limit int) ([]settings.HistoryEntry, error)

	// Operator side: SECURITY DEFINER functions, no tenant context.
	OperatorSetPlatform(ctx context.Context, actor, key string, value json.RawMessage, reset bool) error
	OperatorTenantValues(ctx context.Context, tenantID string) (map[string]Stored, error)
	OperatorSetTenant(ctx context.Context, actor, tenantID, key string, value json.RawMessage, reset bool) (bool, error)
	OperatorHistory(ctx context.Context, tenantID *string, limit int) ([]settings.HistoryEntry, error)
}
