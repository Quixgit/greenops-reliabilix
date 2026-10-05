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

func (s Service) Create(ctx context.Context, tenantID, name, unit string) (domain.Project, error) {
	p, err := domain.New(tenantID, name, unit, time.Now().UTC())
	if err != nil {
		return domain.Project{}, err
	}
	return s.Repo.Create(ctx, p)
}
