package application

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/hibiken/asynq"

	"github.com/quixgit/greenops-reliabilix/backend/internal/ingestion/normalizers/costcsv"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/focus"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/queue"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/storage"
)

type memImports struct {
	rec      map[string]ImportRecord
	finished map[string]*ImportOutcome
	failure  map[string]string
	deleted  []string
	seq      int
}

func newMemImports() *memImports {
	return &memImports{rec: map[string]ImportRecord{}, finished: map[string]*ImportOutcome{}, failure: map[string]string{}}
}
func (m *memImports) CreateImport(_ context.Context, _ string, r ImportRecord) (ImportRecord, error) {
	m.seq++
	r.ID = "11111111-1111-4111-8111-00000000000" + string(rune('0'+m.seq))
	r.Status = "processing"
	m.rec[r.ID] = r
	return r, nil
}
func (m *memImports) GetImport(_ context.Context, _, id string) (ImportRecord, error) {
	r, ok := m.rec[id]
	if !ok {
		return r, ErrNotFound
	}
	return r, nil
}
func (m *memImports) ListImports(context.Context, string, int) ([]ImportRecord, error) { return nil, nil }
func (m *memImports) FinishImport(_ context.Context, _, id string, res *ImportOutcome, failure string) error {
	r := m.rec[id]
	if res != nil {
		r.Status, m.finished[id] = "done", res
	} else {
		r.Status, m.failure[id] = "failed", failure
	}
	m.rec[id] = r
	return nil
}
func (m *memImports) MarkImportDeleting(_ context.Context, _, id string) error {
	r, ok := m.rec[id]
	if !ok || (r.Status != "done" && r.Status != "failed") {
		return ErrNotFound
	}
	r.Status = "deleting"
	m.rec[id] = r
	return nil
}
func (m *memImports) DeleteImport(_ context.Context, _, id string) error {
	m.deleted = append(m.deleted, id)
	delete(m.rec, id)
	return nil
}

type recQueue struct {
	tasks []string
	err   error
}

func (q *recQueue) Enqueue(_ context.Context, task string, _ any, _ ...asynq.Option) error {
	q.tasks = append(q.tasks, task)
	return q.err
}

type usageSink struct {
	recs []focus.Record
	tags []focus.TaggedCost
	key  string
}

func (u *usageSink) Upsert(_ context.Context, _, _, _ string, r []focus.Record) error {
	u.recs = append(u.recs, r...)
	return nil
}
func (u *usageSink) ReplaceTagged(_ context.Context, _, _, key string, _, _ time.Time, rows []focus.TaggedCost) error {
	u.key, u.tags = key, rows
	return nil
}

const datadog = "ChargePeriodStart,ServiceName,BilledCost,Tags\n2026-09-01,Logs,10,\"{\"\"team\"\":\"\"pay\"\"}\"\n2026-09-02,APM,5,\n"

func imports(t *testing.T) (Imports, *memImports, *recQueue, *usageSink, *storage.Memory) {
	t.Helper()
	st, q, sink, files := newMemImports(), &recQueue{}, &usageSink{}, storage.NewMemory()
	return Imports{Store: st, Files: files, Queue: q,
		Pipeline: Service{Normalizers: map[string]Normalizer{"saas/cost_csv": costcsv.Normalizer{}}, Store: sink, Log: slog.New(slog.NewTextHandler(io.Discard, nil))},
		TagKeys:  func(context.Context, string) []string { return []string{"team"} }}, st, q, sink, files
}

func TestStartKeepsTheFileQueuesItAndAnswersWithAPreview(t *testing.T) {
	m, st, q, _, files := imports(t)
	rec, prev, err := m.Start(context.Background(), "T", "P", "alice", "dd.csv", " Datadog ", "eur", []byte(datadog))
	if err != nil {
		t.Fatal(err)
	}
	if rec.Vendor != "Datadog" || rec.Currency != "EUR" || rec.Status != "processing" || len(st.rec) != 1 {
		t.Fatalf("%+v", rec)
	}
	if prev.RowsRead != 2 || prev.Services != 2 || prev.Total != 15 || prev.Columns["cost"] != "BilledCost" {
		t.Fatalf("%+v", prev)
	}
	if len(q.tasks) != 1 || q.tasks[0] != queue.TaskImportCost || len(files.M) != 1 {
		t.Fatalf("queued %v, stored %d files", q.tasks, len(files.M))
	}
}

func TestAFileThatCannotBeUsedIsRefusedBeforeAnythingIsKept(t *testing.T) {
	m, st, q, _, files := imports(t)
	_, _, err := m.Start(context.Background(), "T", "P", "a", "x.csv", "Datadog", "", []byte("foo,bar\n1,2\n"))
	var bad ErrInvalidFile
	if !errors.As(err, &bad) || !strings.Contains(bad.Reason, "needs a date column") {
		t.Fatalf("%v", err)
	}
	if len(st.rec) != 0 || len(q.tasks) != 0 || len(files.M) != 0 {
		t.Fatal("a refused file leaves no record, no job and no copy")
	}
}

func TestRunImportReadsThroughThePipelineTagsAndSourceAndIsIdempotent(t *testing.T) {
	m, st, _, sink, _ := imports(t)
	rec, _, err := m.Start(context.Background(), "T", "P", "a", "dd.csv", "Datadog", "USD", []byte(datadog))
	if err != nil {
		t.Fatal(err)
	}
	if err := m.RunImport(context.Background(), "T", "P", rec.ID); err != nil {
		t.Fatal(err)
	}
	if len(sink.recs) != 2 || sink.recs[0].Provider != "saas" || sink.recs[0].Source != "import:"+rec.ID || sink.recs[0].RegionID != "global" {
		t.Fatalf("%+v", sink.recs)
	}
	if sink.key != "import:"+rec.ID || len(sink.tags) != 1 || sink.tags[0].TagKey != "team" || sink.tags[0].TagValue != "pay" || sink.tags[0].Provider != "saas" {
		t.Fatalf("tags %+v under %q: they are stored under the import so that taking it back removes them", sink.tags, sink.key)
	}
	out := st.finished[rec.ID]
	if out == nil || out.RowsImported != 2 || out.TotalCost != 15 || out.From.Format("2006-01-02") != "2026-09-01" {
		t.Fatalf("%+v", out)
	}
	// a retry of a finished job reads nothing again
	sink.recs = nil
	if err := m.RunImport(context.Background(), "T", "P", rec.ID); err != nil || len(sink.recs) != 0 {
		t.Fatalf("a finished import must not be read twice: %v %d", err, len(sink.recs))
	}
}

func TestRemoveOnlyFinishedImportsAndTheJobTakesEverythingBack(t *testing.T) {
	m, st, q, _, files := imports(t)
	rec, _, _ := m.Start(context.Background(), "T", "P", "a", "dd.csv", "Datadog", "USD", []byte(datadog))
	if err := m.Remove(context.Background(), "T", rec.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("an import still being read cannot be removed: %v", err)
	}
	_ = m.RunImport(context.Background(), "T", "P", rec.ID)
	q.tasks = nil
	if err := m.Remove(context.Background(), "T", rec.ID); err != nil || len(q.tasks) != 1 || q.tasks[0] != queue.TaskDeleteImport {
		t.Fatalf("%v %v", err, q.tasks)
	}
	if err := m.Remove(context.Background(), "T", rec.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("a second remove is refused")
	}
	if err := m.RunDelete(context.Background(), "T", rec.ID); err != nil || len(st.deleted) != 1 || len(files.M) != 0 {
		t.Fatalf("%v deleted %v files %d", err, st.deleted, len(files.M))
	}
}

func TestAnUnreadableStoredFileFailsTheImportWithTheReason(t *testing.T) {
	m, st, _, _, files := imports(t)
	rec, _, _ := m.Start(context.Background(), "T", "P", "a", "dd.csv", "Datadog", "USD", []byte(datadog))
	for k := range files.M {
		files.M[k] = []byte("nonsense")
	}
	if err := m.RunImport(context.Background(), "T", "P", rec.ID); err != nil {
		t.Fatalf("a file that cannot be read is the person's to fix, not a job to retry: %v", err)
	}
	if !strings.Contains(st.failure[rec.ID], "needs a date column") {
		t.Fatalf("failure %q", st.failure[rec.ID])
	}
}
