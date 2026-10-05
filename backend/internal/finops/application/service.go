// Package application holds the finops use-cases.
package application

import (
	"context"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/finops/domain"
)

type Service struct {
	Repo domain.Repository
	Now  func() time.Time
}

func (s Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s Service) Summary(ctx context.Context, tenantID string, project *string, from, to time.Time) (domain.Summary, error) {
	return s.Repo.Summary(ctx, tenantID, project, from, to)
}

func (s Service) CreateBudget(ctx context.Context, tenantID string, b domain.Budget) (domain.Budget, error) {
	if err := b.Validate(); err != nil {
		return domain.Budget{}, err
	}
	return s.Repo.CreateBudget(ctx, tenantID, b)
}

func (s Service) DeleteBudget(ctx context.Context, tenantID, id string) error {
	return s.Repo.DeleteBudget(ctx, tenantID, id)
}

// Budgets lists budgets with their consumption in the current period.
func (s Service) Budgets(ctx context.Context, tenantID string) ([]domain.BudgetStatus, error) {
	bs, err := s.Repo.ListBudgets(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	now := s.now()
	out := make([]domain.BudgetStatus, 0, len(bs))
	for _, b := range bs {
		start, end := domain.PeriodBounds(b.Period, now)
		spent, err := s.Repo.Spent(ctx, tenantID, b.ProjectID, b.Currency, start, end)
		if err != nil {
			return nil, err
		}
		out = append(out, domain.Evaluate(b, spent, now))
	}
	return out, nil
}

func (s Service) Anomalies(ctx context.Context, tenantID string, project *string, from, to time.Time) ([]domain.Anomaly, error) {
	return s.Repo.Anomalies(ctx, tenantID, project, from, to)
}
