// Package domain defines cloud connections and the provider plugin contract.
package domain

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

type ProviderType string

const (
	AWS   ProviderType = "aws"
	Azure ProviderType = "azure"
	GCP   ProviderType = "gcp"
)

func (p ProviderType) Valid() bool { return p == AWS || p == Azure || p == GCP }

type SyncStatus string

const (
	StatusPending SyncStatus = "pending"
	StatusHealthy SyncStatus = "healthy"
	StatusError   SyncStatus = "error"
)

// Connection links a project to a cloud account. CredentialRef is a *reference*
// (IAM role ARN + External ID path / secret-manager path), never a secret.
type Connection struct {
	ID            string       `json:"id"`
	TenantID      string       `json:"tenant_id"`
	ProjectID     string       `json:"project_id"`
	Provider      ProviderType `json:"provider"`
	AccountRef    string       `json:"account_ref"` // AWS account id / subscription id / GCP project id
	CredentialRef string       `json:"credential_ref"`
	LastSyncAt    *time.Time   `json:"last_sync_at"`
	SyncStatus    SyncStatus   `json:"sync_status"`
}

var ErrInvalidConnection = errors.New("invalid cloud connection")

// secretLike catches values that look like raw credentials being pasted in.
var secretLike = []string{"AKIA", "ASIA", "-----BEGIN", "aws_secret_access_key"}

// Validate rejects unknown providers and raw secrets in credential_ref.
func (c Connection) Validate() error {
	switch {
	case !c.Provider.Valid():
		return fmt.Errorf("%w: unknown provider", ErrInvalidConnection)
	case c.ProjectID == "" || c.AccountRef == "" || c.CredentialRef == "":
		return fmt.Errorf("%w: project_id, account_ref, credential_ref required", ErrInvalidConnection)
	}
	for _, s := range secretLike {
		if strings.Contains(c.CredentialRef, s) {
			return fmt.Errorf("%w: credential_ref must be a reference, not a secret", ErrInvalidConnection)
		}
	}
	return nil
}

// UsageRequest asks a provider for usage in a window.
type UsageRequest struct {
	Connection Connection
	From, To   time.Time
}

// RawUsage is provider-specific data handed to ingestion for normalization.
type RawUsage struct {
	Provider ProviderType
	Source   string // cost_explorer | cur | ...
	Payload  []byte
}

// CloudProvider is the plugin contract implemented by AWS, Azure and GCP.
type CloudProvider interface {
	Provider() ProviderType
	// Validate checks that granted access works (and is read-only).
	Validate(ctx context.Context, c Connection) error
	GetUsage(ctx context.Context, req UsageRequest) ([]RawUsage, error)
}

// Registry resolves providers by type.
type Registry struct {
	m map[ProviderType]CloudProvider
}

func NewRegistry(ps ...CloudProvider) *Registry {
	r := &Registry{m: map[ProviderType]CloudProvider{}}
	for _, p := range ps {
		r.m[p.Provider()] = p
	}
	return r
}

func (r *Registry) Get(t ProviderType) (CloudProvider, error) {
	p, ok := r.m[t]
	if !ok {
		return nil, fmt.Errorf("unsupported cloud provider %q", t)
	}
	return p, nil
}

// Repository is tenant-scoped (explicit tenantID + RLS).
type Repository interface {
	List(ctx context.Context, tenantID string) ([]Connection, error)
	Create(ctx context.Context, c Connection) (Connection, error)
}
