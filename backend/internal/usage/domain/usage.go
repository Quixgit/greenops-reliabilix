// Package domain defines how stored usage is queried.
package domain

import (
	"context"
	"time"
)

type Record struct {
	ID              string    `json:"id"`
	ProjectID       string    `json:"project_id"`
	Provider        string    `json:"provider"`
	ResourceID      string    `json:"resource_id"`
	ResourceType    string    `json:"resource_type"`
	ServiceName     string    `json:"service_name"`
	ServiceCategory string    `json:"service_category"`
	Region          string    `json:"region"`
	UsageAmount     float64   `json:"usage_amount"`
	UsageUnit       string    `json:"usage_unit"`
	Cost            float64   `json:"cost"`
	Currency        string    `json:"currency"`
	RecordedAt      time.Time `json:"recorded_at"`
}

type Repository interface {
	List(ctx context.Context, tenantID string, from, to time.Time, limit int) ([]Record, error)
}
