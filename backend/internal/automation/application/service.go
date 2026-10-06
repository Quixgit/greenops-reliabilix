// Package application holds the automation use-cases: planning, approval and reporting of results.
// The platform never changes a customer's cloud itself; see ADR-0017.
package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/quixgit/greenops-reliabilix/backend/internal/automation/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
)

// Recommendations is the port to the recommendations module (adapted in the composition root).
type Recommendations interface {
	Get(ctx context.Context, tenantID, id string) (domain.RecommendationInfo, error)
	// Recheck verifies the recommendation against the project's CURRENT policy (it may have changed).
	Recheck(ctx context.Context, tenantID, id string) error
	// MarkApplied records the recommendation as applied. It must be idempotent: a retry after a partial
	// failure must succeed.
	MarkApplied(ctx context.Context, tenantID, id, actor string) error
}

type Service struct {
	Repo Repository
	Recs Recommendations
}

// Repository is the persistence port (satisfied by repository.Postgres).
type Repository = domain.Repository

const maxNoteLen = 2000

// Plan creates a job for an APPROVED recommendation: it builds the change plan and risk assessment and stores
// them for review. Nothing is executed.
func (s Service) Plan(ctx context.Context, tenantID string, who auth.Claims, recommendationID string) (domain.Job, error) {
	info, err := s.Recs.Get(ctx, tenantID, recommendationID)
	if err != nil {
		return domain.Job{}, err
	}
	if info.Status != "approved" {
		return domain.Job{}, domain.ErrRecommendationNot
	}
	if err := s.Recs.Recheck(ctx, tenantID, recommendationID); err != nil {
		return domain.Job{}, err
	}
	pr, err := domain.BuildPlan(info)
	if err != nil {
		return domain.Job{}, err
	}
	return s.Repo.Insert(ctx, tenantID, domain.Job{ProjectID: info.ProjectID, RecommendationID: info.ID, Kind: pr.Kind, Plan: pr.Plan,
		Risk: pr.Risk, RollbackPlan: pr.RollbackPlan, CreatedBy: who.Subject})
}

func (s Service) List(ctx context.Context, tenantID string, limit int) ([]domain.Job, error) {
	return s.Repo.List(ctx, tenantID, limit)
}

func (s Service) Get(ctx context.Context, tenantID, id string) (domain.Job, error) {
	return s.Repo.Get(ctx, tenantID, id)
}

// Approve is the explicit human decision to apply the plan. The compliance check is repeated at this moment:
// the policy may have changed since the plan was made.
func (s Service) Approve(ctx context.Context, tenantID, id string, who auth.Claims) (domain.Job, error) {
	return s.Repo.Transition(ctx, tenantID, id, domain.Approved, who.Subject, "", func(j *domain.Job) error {
		return s.Recs.Recheck(ctx, tenantID, j.RecommendationID)
	})
}

// Cancel withdraws a plan that was not applied.
func (s Service) Cancel(ctx context.Context, tenantID, id string, who auth.Claims) (domain.Job, error) {
	return s.Repo.Transition(ctx, tenantID, id, domain.Cancelled, who.Subject, "", nil)
}

// Report records what the person who applied the plan observed. "completed" also marks the recommendation as
// applied; failed and rolled_back need a note so the audit trail explains them.
func (s Service) Report(ctx context.Context, tenantID, id string, who auth.Claims, outcome domain.Outcome, note string) (domain.Job, error) {
	to, ok := outcome.Status()
	if !ok {
		return domain.Job{}, fmt.Errorf("%w: unknown outcome", domain.ErrInvalidInput)
	}
	note = strings.TrimSpace(note)
	if len(note) > maxNoteLen {
		return domain.Job{}, fmt.Errorf("%w: note is longer than %d characters", domain.ErrInvalidInput, maxNoteLen)
	}
	if to != domain.Completed && note == "" {
		return domain.Job{}, domain.ErrNoteRequired
	}
	return s.Repo.Transition(ctx, tenantID, id, to, who.Subject, note, func(j *domain.Job) error {
		if to == domain.Completed {
			if err := j.CanExecute(); err != nil { // approved by a human, with a rollback plan
				return err
			}
			return s.Recs.MarkApplied(ctx, tenantID, j.RecommendationID, who.Subject)
		}
		return nil
	})
}
