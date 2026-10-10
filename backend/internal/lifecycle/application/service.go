// Package application holds the lifecycle use-cases: export, scheduled erasure and retention.
package application

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/lifecycle/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/settings"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/storage"
)

// Repo is the database side (implemented by repository.Postgres).
type Repo interface {
	OrganizationName(ctx context.Context, tenantID string) (string, error)
	Deletion(ctx context.Context, tenantID string) (domain.Deletion, error)
	Schedule(ctx context.Context, tenantID, by string, executeAfter time.Time) error
	Cancel(ctx context.Context, tenantID string) error
	Export(ctx context.Context, tenantID string, emit func(table string, row json.RawMessage) error) error
	DueDeletions(ctx context.Context) ([]string, error)
	Erase(ctx context.Context, tenantID string) (map[string]int64, error)
	DropOldPartitions(ctx context.Context, cutoff time.Time) (int, error)
}

type Service struct {
	Repo     Repo
	Store    storage.Store
	Settings settings.Reader
	Log      *slog.Logger
	Now      func() time.Time
}

func (s Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func (s Service) cfg() settings.Reader {
	if s.Settings != nil {
		return s.Settings
	}
	return settings.Defaults{}
}

// Status is the organization's erasure state.
func (s Service) Status(ctx context.Context, tenantID string) (domain.Deletion, error) {
	return s.Repo.Deletion(ctx, tenantID)
}

// ScheduleDeletion requires the organization's exact name and starts the grace period. Nothing is removed now.
func (s Service) ScheduleDeletion(ctx context.Context, tenantID, by, typedName string) (domain.Deletion, error) {
	name, err := s.Repo.OrganizationName(ctx, tenantID)
	if err != nil {
		return domain.Deletion{}, err
	}
	if !domain.ConfirmName(name, typedName) {
		return domain.Deletion{}, domain.ErrNameMismatch
	}
	days := s.cfg().Int(ctx, tenantID, settings.KeyDeletionGraceDays)
	if err := s.Repo.Schedule(ctx, tenantID, by, s.now().AddDate(0, 0, int(days))); err != nil {
		return domain.Deletion{}, err
	}
	return s.Repo.Deletion(ctx, tenantID)
}

func (s Service) CancelDeletion(ctx context.Context, tenantID string) error {
	return s.Repo.Cancel(ctx, tenantID)
}

// Export streams the organization's data as NDJSON lines; the first line describes the export.
func (s Service) Export(ctx context.Context, tenantID string, emit func(line []byte) error) error {
	head, _ := json.Marshal(map[string]any{"export": "reliabilix-organization-data", "format_version": 1,
		"organization_id": tenantID, "generated_at": s.now().Format(time.RFC3339),
		"note": "Secrets (API key hashes, invitation tokens, notification targets, AWS ExternalIds) and generated report files are not included."})
	if err := emit(append(head, '\n')); err != nil {
		return err
	}
	return s.Repo.Export(ctx, tenantID, func(table string, row json.RawMessage) error {
		line, err := json.Marshal(struct {
			Table string          `json:"table"`
			Row   json.RawMessage `json:"row"`
		}{table, row})
		if err != nil {
			return err
		}
		return emit(append(line, '\n'))
	})
}

// EraseDue erases every organization whose grace period is over: object-store files first, then the database in one
// transaction. An error on one organization is logged and does not stop the others; it is retried on the next run
// (the files step is idempotent, and the database step is all-or-nothing). Returns the number erased.
func (s Service) EraseDue(ctx context.Context) (int, error) {
	ids, err := s.Repo.DueDeletions(ctx)
	if err != nil {
		return 0, err
	}
	done := 0
	var firstErr error
	for _, id := range ids {
		if err := s.erase(ctx, id); err != nil {
			s.Log.Error("organization erasure failed", "tenant", id, "err", err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		done++
	}
	return done, firstErr
}

func (s Service) erase(ctx context.Context, tenantID string) error {
	if s.Store != nil {
		prefix, err := storage.TenantPrefix(tenantID)
		if err != nil {
			return err
		}
		if err := s.Store.DeletePrefix(ctx, prefix); err != nil {
			return fmt.Errorf("delete files: %w", err)
		}
	}
	counts, err := s.Repo.Erase(ctx, tenantID)
	if err != nil {
		return err
	}
	var rows int64
	for _, n := range counts {
		rows += n
	}
	s.Log.Info("organization erased", "tenant", tenantID, "rows", rows)
	return nil
}

// EnforceRetention drops monthly usage and carbon partitions past the platform's retention window. Returns the
// number of partitions dropped (0 when retention is off).
func (s Service) EnforceRetention(ctx context.Context) (int, error) {
	months := int(s.cfg().Int(ctx, "", settings.KeyDataRetentionMonths))
	cutoff, ok := domain.RetentionCutoff(s.now(), months)
	if !ok {
		return 0, nil
	}
	n, err := s.Repo.DropOldPartitions(ctx, cutoff)
	if err == nil && n > 0 {
		s.Log.Info("old partitions dropped", "partitions", n, "before", cutoff.Format(time.DateOnly))
	}
	return n, err
}
