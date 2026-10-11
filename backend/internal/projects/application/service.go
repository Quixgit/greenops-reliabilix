// Package application holds the project use-cases.
package application

import (
	"context"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/entitlements"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/projects/domain"
)

type Service struct {
	Repo domain.Repository
	// Structure stores teams, environments and allocation rules; optional for callers that never touch them.
	Structure domain.StructureRepository
	// Limits is optional: when set, creating a project asks whether the organization has room for one more.
	Limits entitlements.Gate
}

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
	if s.Limits != nil {
		if err := s.Limits.Allow(ctx, tenantID, entitlements.Projects); err != nil {
			return domain.Project{}, err
		}
	}
	return s.Repo.Create(ctx, p)
}

// Delete removes a project that holds no cloud connection and no measured data.
func (s Service) Delete(ctx context.Context, tenantID, id string) error {
	return s.Repo.Delete(ctx, tenantID, id)
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

// CarbonLimits lists the projects that have a monthly carbon limit (for the alerts evaluation).
func (s Service) CarbonLimits(ctx context.Context, tenantID string) ([]domain.CarbonLimit, error) {
	return s.Repo.CarbonLimits(ctx, tenantID)
}

// ListForJobs implements the scheduler fan-out over all projects.
func (s Service) ListForJobs(ctx context.Context) ([]domain.JobRef, error) {
	return s.Repo.ListForJobs(ctx)
}
