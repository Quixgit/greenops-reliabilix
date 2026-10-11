// Package application holds the automation use-cases: planning, approval and reporting of results.
// The platform never changes a customer's cloud itself; see ADR-0017.
package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/quixgit/greenops-reliabilix/backend/internal/automation/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/auth"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/notify"
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
	// Notify announces approved plans and reported results; optional and best effort.
	Notify notify.Notifier
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
	j, err := s.Repo.Transition(ctx, tenantID, id, domain.Approved, who.Subject, "", func(j *domain.Job) error {
		return s.Recs.Recheck(ctx, tenantID, j.RecommendationID)
	})
	if err == nil {
		s.announce(ctx, tenantID, notify.Event{Type: notify.TypeAutomationApproved, Severity: notify.Info,
			Title: "A change plan was approved", Body: j.Plan.Summary, Path: "/automation", DedupeKey: "automation:" + id + ":approved",
			Attrs: map[string]string{"job_id": id, "kind": string(j.Kind)}})
	}
	return j, err
}

// announce is best effort: a notification problem must not turn a recorded decision into an error.
func (s Service) announce(ctx context.Context, tenantID string, e notify.Event) {
	if s.Notify != nil {
		_ = s.Notify.Publish(ctx, tenantID, e)
	}
}

// History is the recorded steps of one job; the job must exist for the organization.
func (s Service) History(ctx context.Context, tenantID, id string) ([]domain.HistoryEntry, error) {
	if _, err := s.Repo.Get(ctx, tenantID, id); err != nil {
		return nil, err
	}
	return s.Repo.History(ctx, tenantID, id)
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
	j, err := s.Repo.Transition(ctx, tenantID, id, to, who.Subject, note, func(j *domain.Job) error {
		if to == domain.Completed {
			if err := j.CanExecute(); err != nil { // approved by a human, with a rollback plan
				return err
			}
			return s.Recs.MarkApplied(ctx, tenantID, j.RecommendationID, who.Subject)
		}
		return nil
	})
	if err == nil {
		sev, title := notify.Info, "A change plan was applied"
		switch to {
		case domain.Failed:
			sev, title = notify.Warning, "Applying a change plan failed"
		case domain.RolledBack:
			sev, title = notify.Warning, "A change was rolled back"
		}
		s.announce(ctx, tenantID, notify.Event{Type: notify.TypeAutomationResult, Severity: sev, Title: title, Body: j.Plan.Summary,
			Path: "/automation", DedupeKey: "automation:" + id + ":" + string(to), Attrs: map[string]string{"job_id": id, "outcome": string(to)}})
	}
	return j, err
}
