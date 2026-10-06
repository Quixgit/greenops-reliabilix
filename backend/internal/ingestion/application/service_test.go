package application

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/ingestion/normalizers/awsce"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/focus"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/storage"
)

type fakeStore struct {
	recs []focus.Record
	err  error
}

func (f *fakeStore) Upsert(_ context.Context, _, _, _ string, r []focus.Record) error {
	f.recs = append(f.recs, r...)
	return f.err
}

const payload = `{"ResultsByTime":[
 {"TimePeriod":{"Start":"2026-09-01","End":"2026-09-02"},"Groups":[{"Keys":["Amazon Elastic Compute Cloud - Compute","eu-central-1"],"Metrics":{"UnblendedCost":{"Amount":"10","Unit":"USD"}}}]},
 {"TimePeriod":{"Start":"2026-09-03","End":"2026-09-04"},"Groups":[{"Keys":["AWS Lambda","us-east-1"],"Metrics":{"UnblendedCost":{"Amount":"2","Unit":"USD"}}},
                                                                  {"Keys":["Broken","us-east-1"],"Metrics":{"UnblendedCost":{"Amount":"1","Unit":"US"}}}]}]}`

func svc(store *fakeStore, arch storage.Store) Service {
	return Service{Normalizers: map[string]Normalizer{"aws/cost_explorer": awsce.Normalizer{}}, Store: store, Archive: arch,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)), Now: func() time.Time { return time.Date(2026, 10, 5, 1, 2, 3, 0, time.UTC) }}
}

func TestIngestNormalizesArchivesAndReportsWindow(t *testing.T) {
	store, mem := &fakeStore{}, storage.NewMemory()
	res, err := svc(store, mem).Ingest(context.Background(), "tenant-1", "p", "c1",
		[]focus.RawBatch{{Provider: "aws", Source: "cost_explorer", Payload: []byte(payload)}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Records != 2 || len(store.recs) != 2 {
		t.Fatalf("records = %d (stored %d), want 2: the invalid-currency row must be skipped, not fatal", res.Records, len(store.recs))
	}
	if res.From.Format(time.DateOnly) != "2026-09-01" || res.To.Format(time.DateOnly) != "2026-09-03" {
		t.Errorf("window = %v..%v", res.From, res.To)
	}
	if len(mem.M) != 1 {
		t.Fatalf("raw archive objects = %d", len(mem.M))
	}
	for k := range mem.M {
		if !strings.HasPrefix(k, "tenants/tenant-1/raw/aws/c1/") {
			t.Errorf("archive key not tenant-prefixed: %s", k)
		}
	}
}

func TestIngestErrors(t *testing.T) {
	ctx := context.Background()
	if _, err := svc(&fakeStore{}, nil).Ingest(ctx, "t", "p", "c", []focus.RawBatch{{Provider: "gcp", Source: "bq"}}); err == nil {
		t.Error("unknown normalizer accepted")
	}
	if _, err := svc(&fakeStore{}, nil).Ingest(ctx, "t", "p", "c", []focus.RawBatch{{Provider: "aws", Source: "cost_explorer", Payload: []byte("nope")}}); err == nil {
		t.Error("malformed payload accepted")
	}
	boom := errors.New("db down")
	if _, err := svc(&fakeStore{err: boom}, nil).Ingest(ctx, "t", "p", "c", []focus.RawBatch{{Provider: "aws", Source: "cost_explorer", Payload: []byte(payload)}}); !errors.Is(err, boom) {
		t.Errorf("store error lost: %v", err)
	}
	if res, err := svc(&fakeStore{}, nil).Ingest(ctx, "t", "p", "c", []focus.RawBatch{{Provider: "aws", Source: "cost_explorer", Payload: []byte(`{"ResultsByTime":[]}`)}}); err != nil || res.Records != 0 {
		t.Errorf("empty batch: %+v %v", res, err)
	}
}
