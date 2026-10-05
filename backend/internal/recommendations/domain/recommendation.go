// Package domain defines recommendations and their approval workflow.
package domain

import (
	"errors"
	"fmt"
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

// Recommendation carries BOTH impact estimates: the product shows cost and carbon side by side.
type Recommendation struct {
	ID                 string
	ProjectID          string
	Type               Type
	CarbonReductionPct float64
	CostImpact         float64 // negative = saving
	Confidence         float64 // 0..1
	ComplianceOK       bool
	Status             Status
}

var ErrInvalidTransition = errors.New("invalid status transition")

var allowed = map[Status][]Status{
	Open:     {Approved, Dismissed},
	Approved: {Applied, Dismissed},
}

// Transition moves the recommendation along open -> approved -> applied.
// Nothing reaches "applied" without passing "approved"; approval requires a
// passing compliance check.
func (r *Recommendation) Transition(to Status) error {
	ok := false
	for _, s := range allowed[r.Status] {
		ok = ok || s == to
	}
	if !ok {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, r.Status, to)
	}
	if to == Approved && !r.ComplianceOK {
		return fmt.Errorf("%w: compliance check not passed", ErrInvalidTransition)
	}
	r.Status = to
	return nil
}
