// Package application holds the cloud connection use-cases.
package application

import (
	"context"

	"github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/domain"
)

type Service struct {
	Repo      domain.Repository
	Providers *domain.Registry
}

func (s Service) List(ctx context.Context, tenantID string) ([]domain.Connection, error) {
	return s.Repo.List(ctx, tenantID)
}

// Connect validates input, verifies access through the provider plugin, then stores the reference.
func (s Service) Connect(ctx context.Context, c domain.Connection) (domain.Connection, error) {
	if err := c.Validate(); err != nil {
		return domain.Connection{}, err
	}
	p, err := s.Providers.Get(c.Provider)
	if err != nil {
		return domain.Connection{}, err
	}
	if err := p.Validate(ctx, c); err != nil {
		return domain.Connection{}, err
	}
	return s.Repo.Create(ctx, c)
}
