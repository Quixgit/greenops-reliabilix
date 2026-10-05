// Package application holds the ingestion pipeline: archive raw -> normalize to FOCUS -> validate -> persist.
package application

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/focus"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/observability"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/storage"
)

// Normalizer converts one provider/source payload into FOCUS records.
type Normalizer interface {
	Normalize(raw focus.RawBatch) ([]focus.Record, error)
}

// UsageStore persists FOCUS records (implemented by the usage module).
type UsageStore interface {
	Upsert(ctx context.Context, tenantID, projectID, connectionID string, recs []focus.Record) error
}

type Service struct {
	Normalizers map[string]Normalizer // key: provider + "/" + source
	Store       UsageStore
	Archive     storage.Store // optional: raw payloads are kept so data can be re-normalized later
	Log         *slog.Logger
	Now         func() time.Time
}

func key(provider, source string) string { return provider + "/" + source }

// Ingest runs the pipeline for the batches of one sync. Invalid rows are skipped and counted, never fatal:
// one bad line must not block a tenant's whole sync.
func (s Service) Ingest(ctx context.Context, tenantID, projectID, connectionID string, batches []focus.RawBatch) (focus.IngestResult, error) {
	var res focus.IngestResult
	var all []focus.Record
	for _, b := range batches {
		n, ok := s.Normalizers[key(b.Provider, b.Source)]
		if !ok {
			return res, fmt.Errorf("ingestion: no normalizer for %s/%s", b.Provider, b.Source)
		}
		if err := s.archive(ctx, tenantID, connectionID, b); err != nil {
			s.Log.Warn("raw archive failed", "err", err) // archiving is best effort: ingestion must not depend on it
		}
		recs, err := n.Normalize(b)
		if err != nil {
			return res, err
		}
		for _, r := range recs {
			if err := r.Validate(); err != nil {
				s.Log.Warn("skipping invalid FOCUS record", "service", r.ServiceName, "err", err)
				continue
			}
			all = append(all, r)
		}
	}
	if len(all) == 0 {
		return res, nil
	}
	if err := s.Store.Upsert(ctx, tenantID, projectID, connectionID, all); err != nil {
		return res, fmt.Errorf("ingestion: persist: %w", err)
	}
	res.Records = len(all)
	res.From, res.To = all[0].ChargePeriodStart, all[0].ChargePeriodStart
	for _, r := range all {
		if r.ChargePeriodStart.Before(res.From) {
			res.From = r.ChargePeriodStart
		}
		if r.ChargePeriodStart.After(res.To) {
			res.To = r.ChargePeriodStart
		}
	}
	observability.UsageRecordsProcessed.Add(float64(res.Records))
	return res, nil
}

func (s Service) archive(ctx context.Context, tenantID, connectionID string, b focus.RawBatch) error {
	if s.Archive == nil {
		return nil
	}
	now := time.Now().UTC()
	if s.Now != nil {
		now = s.Now().UTC()
	}
	k, err := storage.Key(tenantID, "raw", fmt.Sprintf("%s/%s/%s.json", b.Provider, connectionID, now.Format("20060102T150405Z")))
	if err != nil {
		return err
	}
	return s.Archive.Put(ctx, k, bytes.NewReader(b.Payload))
}
