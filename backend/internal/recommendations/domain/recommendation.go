// Package domain defines recommendations, their generators and the human-approval workflow.
package domain

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type Type string

const (
	Rightsizing   Type = "rightsizing"
	RegionShift   Type = "region_shift"
	TimeShift     Type = "time_shift"
	SpotMigration Type = "spot_migration"
)

type Status string

const (
	Open      Status = "open"
	Approved  Status = "approved"
	Applied   Status = "applied"
	Dismissed Status = "dismissed"
)

// ComplianceCheck records the data-residency check a recommendation passed. region_shift is invalid
// without it: moving a workload to a region the customer has not allowed is a violation, not a saving.
type ComplianceCheck struct {
	Residency      string    `json:"residency"` // passed | failed | unknown
	AllowedRegions []string  `json:"allowed_regions,omitempty"`
	CheckedAt      time.Time `json:"checked_at"`
}

// Recommendation carries BOTH impact estimates, separately: carbon and cost.
type Recommendation struct {
	ID                 string          `json:"id"`
	ProjectID          string          `json:"project_id"`
	Type               Type            `json:"type"`
	Title              string          `json:"title"`
	Provider           string          `json:"provider"`
	ServiceName        string          `json:"service_name"`
	CurrentRegion      string          `json:"current_region"`
	RecommendedRegion  string          `json:"recommended_region"`
	CarbonReductionPct float64         `json:"estimated_carbon_reduction_pct"`
	CarbonReductionKg  float64         `json:"carbon_reduction_kg_month"`
	CostImpact         *float64        `json:"estimated_cost_impact"` // nil = not estimated; negative = saving
	CostBasis          string          `json:"cost_basis"`
	Confidence         float64         `json:"confidence"`
	Compliance         ComplianceCheck `json:"compliance_check"`
	Status             Status          `json:"status"`
	DecidedBy          *string         `json:"decided_by"`
	DecidedAt          *time.Time      `json:"decided_at"`
	AppliedAt          *time.Time      `json:"applied_at"`
	CreatedAt          time.Time       `json:"created_at"`
	// Details are structured facts for automation (resource id, instance types ...). Strings only.
	Details     map[string]string `json:"details"`
	Fingerprint string            `json:"-"`
}

var (
	ErrInvalidTransition = errors.New("invalid status transition")
	ErrComplianceChanged = errors.New("compliance no longer allows this recommendation")
	ErrNotFound          = errors.New("not found")
)

// ComplianceOK reports whether the recommendation may be approved from a compliance standpoint.
func (r Recommendation) ComplianceOK() bool {
	if r.Type == RegionShift {
		return r.Compliance.Residency == "passed"
	}
	return r.Compliance.Residency != "failed"
}

var allowed = map[Status][]Status{
	Open:     {Approved, Dismissed},
	Approved: {Applied, Dismissed},
}

// Transition moves the recommendation along open -> approved -> applied. Nothing reaches "applied"
// without passing "approved" (a human decision), and approval requires a passing compliance check.
func (r *Recommendation) Transition(to Status) error {
	ok := false
	for _, s := range allowed[r.Status] {
		ok = ok || s == to
	}
	if !ok {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, r.Status, to)
	}
	if to == Approved && !r.ComplianceOK() {
		return fmt.Errorf("%w: compliance check not passed", ErrInvalidTransition)
	}
	r.Status = to
	return nil
}

// Repository is tenant-scoped (explicit tenantID + RLS).
type Repository interface {
	Insert(ctx context.Context, tenantID string, r Recommendation) (created bool, err error)
	List(ctx context.Context, tenantID, status string, project *string, limit int) ([]Recommendation, error)
	Get(ctx context.Context, tenantID, id string) (Recommendation, error)
	// Transition locks the row, validates through check, applies the state change and audits it, atomically.
	Transition(ctx context.Context, tenantID, id string, to Status, actor string, check func(*Recommendation) error) (Recommendation, error)
	// Inputs of the generators (read-only views of carbon, finops and project policy).
	Workloads(ctx context.Context, tenantID, projectID string, from, to time.Time) (ws []Workload, dataDays int, err error)
	Intensities(ctx context.Context) (map[string]float64, error)
	PriceIndex(ctx context.Context, provider string) (map[string]float64, error)
	AllowedRegions(ctx context.Context, tenantID, projectID string) ([]string, error)
}
