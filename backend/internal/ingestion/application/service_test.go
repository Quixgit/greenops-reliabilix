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

// taggedNormalizer yields one cost line and the tags of a payload.
type taggedNormalizer struct{ tags []focus.TaggedCost }

func (taggedNormalizer) Normalize(focus.RawBatch) ([]focus.Record, error) {
	d := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	return []focus.Record{{Provider: "aws", Source: "focus_export", ServiceName: "S3", ServiceCategory: "storage", RegionID: "eu-west-1", BilledCost: 5, EffectiveCost: 5, Currency: "USD", ChargePeriodStart: d, ChargePeriodEnd: d.AddDate(0, 0, 1)}}, nil
}
func (n taggedNormalizer) NormalizeTags(focus.RawBatch) ([]focus.TaggedCost, error) {
	return n.tags, nil
}

type taggedStore struct {
	fakeStore
	calls       int
	source      string
	from, to    time.Time
	rows        []focus.TaggedCost
	failReplace bool
}

func (s *taggedStore) ReplaceTagged(_ context.Context, _, _, source string, from, to time.Time, rows []focus.TaggedCost) error {
	s.calls++
	s.source, s.from, s.to, s.rows = source, from, to, rows
	if s.failReplace {
		return errors.New("db down")
	}
	return nil
}

func TestTaggedCostIsStoredForTheDaysReadAndNeverFailsTheCosts(t *testing.T) {
	d := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	good := focus.TaggedCost{Provider: "aws", ServiceName: "S3", Currency: "USD", Day: d, TagKey: "team", TagValue: "pay", BilledCost: 5, EffectiveCost: 5}
	bad := focus.TaggedCost{Provider: "aws", ServiceName: "S3", Currency: "USD", Day: d, TagKey: "", TagValue: "x"}
	store := &taggedStore{}
	s := Service{Normalizers: map[string]Normalizer{"aws/focus_export": taggedNormalizer{tags: []focus.TaggedCost{good, bad}}}, Store: store,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	if _, err := s.Ingest(context.Background(), "t", "p", "conn-1", []focus.RawBatch{{Provider: "aws", Source: "focus_export"}}); err != nil {
		t.Fatal(err)
	}
	if store.calls != 1 || store.source != "conn-1" || len(store.rows) != 1 || store.rows[0].TagKey != "team" || store.from != d || store.to != d {
		t.Fatalf("calls=%d source=%q rows=%+v window %v..%v (an invalid tag line is dropped, the source is the connection)", store.calls, store.source, store.rows, store.from, store.to)
	}
	if len(store.recs) != 1 {
		t.Fatal("the cost line is stored as ever")
	}
	// a source that reports tags but has none this time still clears its range: a tag that is gone from the bill is gone here too
	store2 := &taggedStore{}
	s.Store = store2
	s.Normalizers = map[string]Normalizer{"aws/focus_export": taggedNormalizer{}}
	if _, err := s.Ingest(context.Background(), "t", "p", "conn-1", []focus.RawBatch{{Provider: "aws", Source: "focus_export"}}); err != nil || store2.calls != 1 || len(store2.rows) != 0 {
		t.Fatalf("calls=%d rows=%v err=%v", store2.calls, store2.rows, err)
	}
	// a failure while storing tags is logged, not returned
	store3 := &taggedStore{failReplace: true}
	s.Store = store3
	if _, err := s.Ingest(context.Background(), "t", "p", "conn-1", []focus.RawBatch{{Provider: "aws", Source: "focus_export"}}); err != nil {
		t.Fatalf("tags must never fail a sync: %v", err)
	}
	// a source without tags never calls the tag store (Cost Explorer lines keep the rows of an export)
	store4 := &taggedStore{}
	s2 := svc(&fakeStore{}, nil)
	s2.Store = store4
	if _, err := s2.Ingest(context.Background(), "t", "p", "c", []focus.RawBatch{{Provider: "aws", Source: "cost_explorer", Payload: []byte(payload)}}); err != nil || store4.calls != 0 {
		t.Fatalf("calls=%d err=%v", store4.calls, err)
	}
}

func TestTagWindowReachesBackAsFarAsTheSourceSays(t *testing.T) {
	d := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC) // before the cost lines (1 September): the wider range wins
	store := &taggedStore{}
	s := Service{Normalizers: map[string]Normalizer{"aws/focus_export": taggedNormalizer{}}, Store: store, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	if _, err := s.Ingest(context.Background(), "t", "p", "c", []focus.RawBatch{{Provider: "aws", Source: "focus_export", TagFrom: d}}); err != nil {
		t.Fatal(err)
	}
	if store.from != d {
		t.Fatalf("the range a source's tags replace starts at its TagFrom, got %v", store.from)
	}
}
