// Package domain defines automation plans. Phase 3: the skeleton exists now so
// recommendations are designed against this seam; execution logic comes later.
package domain

import (
	"errors"
	"time"
)

var ErrNotApproved = errors.New("automation: plan has no human approval")

// Plan is a proposed infrastructure change derived from an approved recommendation.
type Plan struct {
	ID               string
	RecommendationID string
	ApprovedBy       string // human user id; empty = not approved
	ApprovedAt       *time.Time
	RollbackPlan     string
}

// CanExecute enforces the core safety rule: never change infrastructure without
// explicit human approval and a rollback plan.
func (p Plan) CanExecute() error {
	if p.ApprovedBy == "" || p.ApprovedAt == nil {
		return ErrNotApproved
	}
	if p.RollbackPlan == "" {
		return errors.New("automation: rollback plan required")
	}
	return nil
}
