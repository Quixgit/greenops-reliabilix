// Package domain holds the Project aggregate, its SCI inputs and policies (no DB/HTTP imports).
package domain

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

type Project struct {
	ID             string    `json:"id"`
	TenantID       string    `json:"tenant_id"`
	Name           string    `json:"name"`
	FunctionalUnit string    `json:"functional_unit"` // SCI "R": what one unit of useful work is, e.g. "api_request"
	CreatedAt      time.Time `json:"created_at"`
}

var (
	ErrInvalidName   = errors.New("name must be 1-120 characters")
	ErrInvalidUnit   = errors.New("functional_unit must be 1-60 characters")
	ErrDuplicate     = errors.New("project already exists")
	ErrNotFound      = errors.New("not found")
	ErrInvalidPeriod = errors.New("invalid period")
	ErrInvalidPolicy = errors.New("invalid policy")
)

func New(tenantID, name, functionalUnit string, now time.Time) (Project, error) {
	name, functionalUnit = strings.TrimSpace(name), strings.TrimSpace(functionalUnit)
	if n := len([]rune(name)); n < 1 || n > 120 {
		return Project{}, ErrInvalidName
	}
	if n := len([]rune(functionalUnit)); n < 1 || n > 60 {
		return Project{}, ErrInvalidUnit
	}
	return Project{TenantID: tenantID, Name: name, FunctionalUnit: functionalUnit, CreatedAt: now}, nil
}

// FunctionalUnits is the number of functional units a project served in a period (SCI denominator R).
type FunctionalUnits struct {
	ID          string    `json:"id"`
	ProjectID   string    `json:"project_id"`
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"` // inclusive
	Units       float64   `json:"units"`
}

// Validate rejects empty/inverted periods, non-positive units and windows over 400 days.
func (f FunctionalUnits) Validate() error {
	if f.PeriodEnd.Before(f.PeriodStart) || f.PeriodEnd.Sub(f.PeriodStart) > 400*24*time.Hour {
		return ErrInvalidPeriod
	}
	if f.Units <= 0 {
		return fmt.Errorf("%w: units must be > 0", ErrInvalidPeriod)
	}
	return nil
}

// Policy holds the compliance and CI-gate settings of a project.
type Policy struct {
	ProjectID string `json:"project_id"`
	// AllowedRegions is the data-residency allow-list. While empty, residency is unknown and
	// no region_shift recommendation is ever produced for the project.
	AllowedRegions  []string `json:"allowed_regions"`
	CICarbonWarnKg  *float64 `json:"ci_carbon_warn_kg"`
	CICarbonBlockKg *float64 `json:"ci_carbon_block_kg"`
	CICostWarn      *float64 `json:"ci_cost_warn"`
	CICostBlock     *float64 `json:"ci_cost_block"`
}

var regionRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,30}[a-z0-9]$`)

// Validate normalizes and checks the policy.
func (p *Policy) Validate() error {
	if len(p.AllowedRegions) > 50 {
		return fmt.Errorf("%w: at most 50 regions", ErrInvalidPolicy)
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(p.AllowedRegions))
	for _, r := range p.AllowedRegions {
		r = strings.ToLower(strings.TrimSpace(r))
		if !regionRe.MatchString(r) {
			return fmt.Errorf("%w: bad region %q", ErrInvalidPolicy, r)
		}
		if !seen[r] {
			seen[r] = true
			out = append(out, r)
		}
	}
	p.AllowedRegions = out
	for _, v := range []*float64{p.CICarbonWarnKg, p.CICarbonBlockKg, p.CICostWarn, p.CICostBlock} {
		if v != nil && *v < 0 {
			return fmt.Errorf("%w: thresholds must be >= 0", ErrInvalidPolicy)
		}
	}
	if p.CICarbonWarnKg != nil && p.CICarbonBlockKg != nil && *p.CICarbonWarnKg > *p.CICarbonBlockKg {
		return fmt.Errorf("%w: carbon warn must not exceed block", ErrInvalidPolicy)
	}
	if p.CICostWarn != nil && p.CICostBlock != nil && *p.CICostWarn > *p.CICostBlock {
		return fmt.Errorf("%w: cost warn must not exceed block", ErrInvalidPolicy)
	}
	return nil
}

// Repository is tenant-scoped by construction (explicit tenantID) and backed by RLS.
type Repository interface {
	List(ctx context.Context, tenantID string) ([]Project, error)
	Get(ctx context.Context, tenantID, id string) (Project, error)
	Create(ctx context.Context, p Project) (Project, error)
	UpsertFunctionalUnits(ctx context.Context, tenantID string, f FunctionalUnits) error
	ListFunctionalUnits(ctx context.Context, tenantID, projectID string) ([]FunctionalUnits, error)
	FunctionalUnitsForDay(ctx context.Context, tenantID, projectID string, day time.Time) (float64, error)
	GetPolicy(ctx context.Context, tenantID, projectID string) (Policy, error)
	UpsertPolicy(ctx context.Context, tenantID string, p Policy) error
	ListForJobs(ctx context.Context) ([]JobRef, error)
}

// JobRef identifies a project for cross-tenant job fan-out.
type JobRef struct{ TenantID, ProjectID string }
