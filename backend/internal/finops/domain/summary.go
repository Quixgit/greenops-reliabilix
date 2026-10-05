// Package domain holds cost-side concepts, kept separate from carbon.
package domain

import (
	"context"
	"time"
)

type Summary struct {
	HasData           bool    `json:"has_data"`
	TotalCost         float64 `json:"total_cost"`
	PreviousTotalCost float64 `json:"previous_total_cost"`
	Currency          string  `json:"currency"`
	ByService         []Line  `json:"by_service"`
}

type Line struct {
	Service string  `json:"service"`
	Cost    float64 `json:"cost"`
}

type Repository interface {
	Summary(ctx context.Context, tenantID string, project *string, from, to time.Time) (Summary, error)
}
