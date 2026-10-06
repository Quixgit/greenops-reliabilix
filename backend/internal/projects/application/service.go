// Package application holds the project use-cases.
package application

import (
	"context"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/projects/domain"
)

type Service struct{ Repo domain.Repository }

func (s Service) List(ctx context.Context, tenantID string) ([]domain.Project, error) {
	return s.Repo.List(ctx, tenantID)
}

func (s Service) Get(ctx context.Context, tenantID, id string) (domain.Project, error) {
	return s.Repo.Get(ctx, tenantID, id)
}

func (s Service) Create(ctx context.Context, tenantID, name, unit string) (domain.Project, error) {
	p, err := domain.New(tenantID, name, unit, time.Now().UTC())
	if err != nil {
		return domain.Project{}, err
	}
	return s.Repo.Create(ctx, p)
}

func (s Service) ReportFunctionalUnits(ctx context.Context, tenantID string, f domain.FunctionalUnits) error {
	if err := f.Validate(); err != nil {
		return err
	}
	return s.Repo.UpsertFunctionalUnits(ctx, tenantID, f)
}

func (s Service) ListFunctionalUnits(ctx context.Context, tenantID, projectID string) ([]domain.FunctionalUnits, error) {
	return s.Repo.ListFunctionalUnits(ctx, tenantID, projectID)
}

func (s Service) GetPolicy(ctx context.Context, tenantID, projectID string) (domain.Policy, error) {
	if _, err := s.Repo.Get(ctx, tenantID, projectID); err != nil {
		return domain.Policy{}, err
	}
	return s.Repo.GetPolicy(ctx, tenantID, projectID)
}

func (s Service) SetPolicy(ctx context.Context, tenantID string, p domain.Policy) (domain.Policy, error) {
	if err := p.Validate(); err != nil {
		return domain.Policy{}, err
	}
	return p, s.Repo.UpsertPolicy(ctx, tenantID, p)
}

// FunctionalUnitsForDay implements the carbon engine's SCI-denominator source.
func (s Service) FunctionalUnitsForDay(ctx context.Context, tenantID, projectID string, day time.Time) (float64, error) {
	return s.Repo.FunctionalUnitsForDay(ctx, tenantID, projectID, day)
}

// ListForJobs implements the scheduler fan-out over all projects.
func (s Service) ListForJobs(ctx context.Context) ([]domain.JobRef, error) {
	return s.Repo.ListForJobs(ctx)
}
