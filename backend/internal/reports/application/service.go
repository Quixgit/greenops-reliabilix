// Package application holds the report use-cases: request, generate (job) and download.
package application

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/hibiken/asynq"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/queue"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/storage"
	"github.com/quixgit/greenops-reliabilix/backend/internal/reports/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/reports/render"
)

type Service struct {
	Repo  domain.Repository
	Store storage.Store
	Queue queue.Enqueuer
	Now   func() time.Time
}

func (s Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func (s Service) Create(ctx context.Context, tenantID, requestedBy string, r domain.Report) (domain.Report, error) {
	if err := r.Validate(); err != nil {
		return domain.Report{}, err
	}
	r.RequestedBy = requestedBy
	created, err := s.Repo.Insert(ctx, tenantID, r)
	if err != nil {
		return domain.Report{}, err
	}
	if err := s.Queue.Enqueue(ctx, queue.TaskGenerateReport, queue.TenantPayload{TenantID: tenantID, RefID: created.ID}, asynq.MaxRetry(3)); err != nil {
		_ = s.Repo.SetFailed(ctx, tenantID, created.ID, "could not queue the report")
		return domain.Report{}, err
	}
	return created, nil
}

func (s Service) List(ctx context.Context, tenantID string) ([]domain.Report, error) {
	return s.Repo.List(ctx, tenantID)
}
func (s Service) Get(ctx context.Context, tenantID, id string) (domain.Report, error) {
	return s.Repo.Get(ctx, tenantID, id)
}

// table builds the dataset of a report. The period end is inclusive, the queries take an exclusive end.
func (s Service) table(ctx context.Context, tenantID string, r domain.Report) (domain.Table, error) {
	from, to := r.PeriodStart, r.PeriodEnd.AddDate(0, 0, 1)
	switch r.Kind {
	case domain.KindCarbon:
		return s.Repo.CarbonTable(ctx, tenantID, r.ProjectID, from, to)
	case domain.KindSCI:
		return s.Repo.SCITable(ctx, tenantID, r.ProjectID, from, to)
	case domain.KindSustainability:
		return s.Repo.SustainabilityTable(ctx, tenantID, r.ProjectID, from, to)
	default:
		return s.Repo.FinopsTable(ctx, tenantID, r.ProjectID, from, to)
	}
}

// Generate is the reports:generate job: build the dataset, render, store, mark ready.
func (s Service) Generate(ctx context.Context, tenantID, id string) error {
	r, err := s.Repo.Get(ctx, tenantID, id)
	if err != nil {
		if err == domain.ErrNotFound {
			return nil
		}
		return err
	}
	if r.Status == domain.Ready {
		return nil // already done (job redelivery)
	}
	tbl, err := s.table(ctx, tenantID, r)
	if err != nil {
		return err
	}
	b, err := render.Render(tbl, r.Format, s.now())
	if err != nil {
		return err
	}
	key, err := storage.Key(tenantID, "reports", id+"."+r.Format.Ext())
	if err != nil {
		return err
	}
	if err := s.Store.Put(ctx, key, bytes.NewReader(b)); err != nil {
		return fmt.Errorf("store report: %w", err)
	}
	return s.Repo.SetReady(ctx, tenantID, id, key)
}

// MarkFailed records a terminal failure after retries are exhausted.
func (s Service) MarkFailed(ctx context.Context, tenantID, id string) error {
	return s.Repo.SetFailed(ctx, tenantID, id, "Report generation failed. Try again or contact support.")
}

// Download is a stored report file opened for streaming.
type Download struct {
	Body        io.ReadCloser
	ContentType string
	Filename    string
}

// Open returns the file of a ready report. The tenant check happens in the repository (RLS), and the object
// key was built from the tenant id at generation time, so one tenant can never read another's file.
func (s Service) Open(ctx context.Context, tenantID, id string) (Download, error) {
	r, err := s.Repo.Get(ctx, tenantID, id)
	if err != nil {
		return Download{}, err
	}
	if r.Status != domain.Ready || r.ObjectKey == nil {
		return Download{}, domain.ErrNotReady
	}
	body, err := s.Store.Get(ctx, *r.ObjectKey)
	if err != nil {
		return Download{}, err
	}
	return Download{Body: body, ContentType: r.Format.ContentType(),
		Filename: fmt.Sprintf("reliabilix-%s-%s_%s.%s", r.Kind, r.PeriodStart.Format(time.DateOnly), r.PeriodEnd.Format(time.DateOnly), r.Format.Ext())}, nil
}
