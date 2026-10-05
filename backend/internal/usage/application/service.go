// Package application holds the usage use-cases.
package application

import (
	"context"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/focus"
	"github.com/quixgit/greenops-reliabilix/backend/internal/usage/domain"
)

type Service struct{ Repo domain.Repository }

func (s Service) Upsert(ctx context.Context, tenantID, projectID, connectionID string, recs []focus.Record) error {
	return s.Repo.Upsert(ctx, tenantID, projectID, connectionID, recs)
}

func (s Service) List(ctx context.Context, tenantID string, projectID *string, from, to time.Time, limit int) ([]domain.Record, error) {
	return s.Repo.List(ctx, tenantID, projectID, from, to, limit)
}

func (s Service) DailyAggregates(ctx context.Context, tenantID, projectID string, from, to time.Time) ([]domain.DailyAggregate, error) {
	return s.Repo.DailyAggregates(ctx, tenantID, projectID, from, to)
}
