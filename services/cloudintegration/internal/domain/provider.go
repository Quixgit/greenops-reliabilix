// Package domain defines the plugin contract for cloud providers (ADR-0006).
package domain

import (
	"context"
	"fmt"
	"time"
)

// Account is a connected cloud account. It stores only a *reference* to
// credentials (role ARN + secret-manager path), never secrets (ADR-0010).
type Account struct {
	ID          string
	TenantID    string
	Provider    string
	ExternalID  string // AWS account id / Azure subscription id / GCP project id
	RoleRef     string // e.g. arn:aws:iam::123456789012:role/ReliabilixReadOnly
	ExternalKey string // sts:ExternalId, per-tenant, generated server-side
}

// UsageRequest asks a provider for usage in a window.
type UsageRequest struct {
	Account Account
	From    time.Time
	To      time.Time
}

// RawUsage is provider-specific data handed to ingestion for normalization.
type RawUsage struct {
	Provider string
	Source   string
	Payload  []byte
}

// CloudProvider is implemented by AWS, Azure and GCP connectors.
type CloudProvider interface {
	Name() string
	// Validate checks that the granted access works and is least-privilege.
	Validate(ctx context.Context, a Account) error
	FetchUsage(ctx context.Context, r UsageRequest) ([]RawUsage, error)
}

// Registry resolves providers by name.
type Registry struct{ m map[string]CloudProvider }

func NewRegistry(ps ...CloudProvider) *Registry {
	r := &Registry{m: map[string]CloudProvider{}}
	for _, p := range ps {
		r.m[p.Name()] = p
	}
	return r
}

func (r *Registry) Get(name string) (CloudProvider, error) {
	p, ok := r.m[name]
	if !ok {
		return nil, fmt.Errorf("unsupported cloud provider %q", name)
	}
	return p, nil
}
