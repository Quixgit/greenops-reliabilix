// Package domain defines the provider-neutral usage model (ADR-0006).
package domain

import (
	"errors"
	"time"
)

// UsageRecord is the normalized form of AWS/Azure/GCP/Kubernetes usage.
type UsageRecord struct {
	ID              string
	TenantID        string
	ProjectID       string
	Provider        string // aws | azure | gcp | k8s
	Source          string // cost_explorer | cur | billing_export | kepler ...
	ResourceID      string
	ResourceType    string
	ServiceName     string
	ServiceCategory string // compute | storage | database | networking | other (FOCUS ServiceCategory)
	Region          string
	UsageAmount     float64
	UsageUnit       string
	Cost            float64
	Currency        string // ISO 4217
	PeriodStart     time.Time
	PeriodEnd       time.Time
}

var categories = map[string]bool{"compute": true, "storage": true, "database": true, "networking": true, "other": true}

var providers = map[string]bool{"aws": true, "azure": true, "gcp": true, "k8s": true}

// Validate enforces invariants before a record may be persisted.
func (u UsageRecord) Validate() error {
	switch {
	case u.TenantID == "":
		return errors.New("tenant_id required")
	case u.ProjectID == "":
		return errors.New("project_id required")
	case !providers[u.Provider]:
		return errors.New("unknown provider")
	case !categories[u.ServiceCategory]:
		return errors.New("unknown service_category")
	case u.ServiceName == "":
		return errors.New("service_name required")
	case u.Region == "":
		return errors.New("region required")
	case u.UsageAmount < 0 || u.Cost < 0:
		return errors.New("usage and cost must be non-negative")
	case len(u.Currency) != 3:
		return errors.New("currency must be ISO 4217")
	case !u.PeriodEnd.After(u.PeriodStart):
		return errors.New("period_end must be after period_start")
	}
	return nil
}
