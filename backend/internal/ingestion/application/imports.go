package application

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/hibiken/asynq"

	"github.com/quixgit/greenops-reliabilix/backend/internal/ingestion/normalizers/costcsv"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/focus"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/queue"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/storage"
)

// ImportRecord is one uploaded vendor cost file as the screen shows it.
type ImportRecord struct {
	ID           string     `json:"id"`
	ProjectID    string     `json:"project_id"`
	Vendor       string     `json:"vendor"`
	Filename     string     `json:"filename"`
	Currency     string     `json:"currency"`
	Status       string     `json:"status"`
	Error        string     `json:"error"`
	RowsRead     int        `json:"rows_read"`
	RowsImported int        `json:"rows_imported"`
	RowsSkipped  int        `json:"rows_skipped"`
	Services     int        `json:"services"`
	From         *string    `json:"from"`
	To           *string    `json:"to"`
	TotalCost    float64    `json:"total_cost"`
	Warnings     []string   `json:"warnings"`
	CreatedBy    string     `json:"created_by"`
	CreatedAt    time.Time  `json:"created_at"`
	FinishedAt   *time.Time `json:"finished_at"`
}

// ImportOutcome is what the job records after reading a file.
type ImportOutcome struct {
	RowsRead, RowsImported, RowsSkipped, Services int
	From, To                                      time.Time
	Currency                                      string
	TotalCost                                     float64
	Warnings                                      []string
}

// ImportStore is the port to the usage module's records of imports (wired in the composition root).
type ImportStore interface {
	CreateImport(ctx context.Context, tenantID string, r ImportRecord) (ImportRecord, error)
	GetImport(ctx context.Context, tenantID, id string) (ImportRecord, error)
	ListImports(ctx context.Context, tenantID string, limit int) ([]ImportRecord, error)
	FinishImport(ctx context.Context, tenantID, id string, res *ImportOutcome, failure string) error
	MarkImportDeleting(ctx context.Context, tenantID, id string) error
	DeleteImport(ctx context.Context, tenantID, id string) error
}

// ErrNotFound: no such import (or it is not in a state that allows the action).
var ErrNotFound = errors.New("import not found")

// ErrInvalidFile is a file that cannot be used; the message says why and is safe to show.
type ErrInvalidFile struct{ Reason string }

func (e ErrInvalidFile) Error() string { return e.Reason }

// Imports is the use-case of uploading a vendor's cost file: validate now, read in the worker, take back on request.
type Imports struct {
	Store    ImportStore
	Files    storage.Store
	Queue    queue.Enqueuer
	Pipeline Service
	// TagKeys are the organization's allocation keys (settings); nil keeps none.
	TagKeys func(ctx context.Context, tenantID string) []string
}

func (m Imports) keys(ctx context.Context, tenantID string) []string {
	if m.TagKeys == nil {
		return nil
	}
	return m.TagKeys(ctx, tenantID)
}

func fileKey(tenantID, id string) (string, error) { return storage.Key(tenantID, "imports", id+".csv") }

// Preview is what reading the file showed, handed back to the uploader at once.
type Preview struct {
	RowsRead, RowsSkipped, Services int
	From, To                        time.Time
	Currency                        string
	Total                           float64
	Columns                         map[string]string
	Warnings                        []string
}

// Start checks the file at once (so a wrong file is answered with the reason, not discovered later), keeps it and queues the reading.
func (m Imports) Start(ctx context.Context, tenantID, projectID, actor, filename, vendor, currency string, data []byte) (ImportRecord, Preview, error) {
	vendor = strings.TrimSpace(vendor)
	res, err := costcsv.Parse(data, costcsv.Options{Vendor: vendor, Currency: currency, TagKeys: m.keys(ctx, tenantID)})
	if err != nil {
		return ImportRecord{}, Preview{}, ErrInvalidFile{Reason: err.Error()}
	}
	cur := strings.ToUpper(strings.TrimSpace(currency))
	if cur == "" {
		cur = "USD"
	}
	if len(filename) > 200 {
		filename = filename[:200]
	}
	rec, err := m.Store.CreateImport(ctx, tenantID, ImportRecord{ProjectID: projectID, Vendor: vendor, Filename: filename, Currency: cur, CreatedBy: actor})
	if err != nil {
		return ImportRecord{}, Preview{}, err
	}
	key, err := fileKey(tenantID, rec.ID)
	if err == nil {
		err = m.Files.Put(ctx, key, strings.NewReader(string(data)))
	}
	if err == nil {
		err = m.Queue.Enqueue(ctx, queue.TaskImportCost, queue.TenantPayload{TenantID: tenantID, ProjectID: projectID, RefID: rec.ID}, asynq.MaxRetry(3))
	}
	if err != nil {
		_ = m.Store.FinishImport(ctx, tenantID, rec.ID, nil, "the file could not be stored or queued")
		return rec, Preview{}, fmt.Errorf("import: %w", err)
	}
	return rec, Preview{RowsRead: res.RowsRead, RowsSkipped: res.RowsSkipped, Services: res.Services, From: res.From, To: res.To, Currency: res.Currency, Total: res.Total,
		Columns: res.Columns, Warnings: res.Warnings}, nil
}

func (m Imports) List(ctx context.Context, tenantID string) ([]ImportRecord, error) {
	return m.Store.ListImports(ctx, tenantID, 100)
}

// Remove marks the import for taking back and queues the removal of its lines.
func (m Imports) Remove(ctx context.Context, tenantID, id string) error {
	if err := m.Store.MarkImportDeleting(ctx, tenantID, id); err != nil {
		return ErrNotFound
	}
	return m.Queue.Enqueue(ctx, queue.TaskDeleteImport, queue.TenantPayload{TenantID: tenantID, RefID: id}, asynq.MaxRetry(5))
}

// RunImport is the job: read the stored file through the ingestion pipeline (normalize, validate, store the lines and their tags).
func (m Imports) RunImport(ctx context.Context, tenantID, projectID, id string) error {
	rec, err := m.Store.GetImport(ctx, tenantID, id)
	if err != nil {
		return nil // gone: nothing to read
	}
	if rec.Status != "processing" {
		return nil // a retry of a finished job
	}
	fail := func(msg string) error {
		_ = m.Store.FinishImport(ctx, tenantID, id, nil, msg)
		return nil // the person is told; retrying a file that cannot be read would not help
	}
	key, err := fileKey(tenantID, id)
	if err != nil {
		return fail("internal error")
	}
	rc, err := m.Files.Get(ctx, key)
	if err != nil {
		return err // storage trouble is worth a retry
	}
	data, err := io.ReadAll(io.LimitReader(rc, costcsv.MaxBytes+1))
	_ = rc.Close()
	if err != nil {
		return err
	}
	keys := m.keys(ctx, tenantID)
	res, err := costcsv.Parse(data, costcsv.Options{Vendor: rec.Vendor, Currency: rec.Currency, TagKeys: keys})
	if err != nil {
		return fail(err.Error())
	}
	batch := focus.RawBatch{Provider: "saas", Source: "cost_csv", Payload: data, Meta: map[string]string{
		"vendor": rec.Vendor, "currency": rec.Currency, "tag_keys": strings.Join(keys, ","), "import_id": id, "source_key": "import:" + id}}
	if _, err := m.Pipeline.Ingest(ctx, tenantID, rec.ProjectID, "", []focus.RawBatch{batch}); err != nil {
		return err
	}
	return m.Store.FinishImport(ctx, tenantID, id, &ImportOutcome{RowsRead: res.RowsRead, RowsImported: res.RowsImported, RowsSkipped: res.RowsSkipped, Services: res.Services,
		From: res.From, To: res.To, Currency: res.Currency, TotalCost: res.Total, Warnings: res.Warnings}, "")
}

// RunDelete is the job that takes an import back: its lines, tags, stored file and record.
func (m Imports) RunDelete(ctx context.Context, tenantID, id string) error {
	if err := m.Store.DeleteImport(ctx, tenantID, id); err != nil {
		return err
	}
	if key, err := fileKey(tenantID, id); err == nil {
		_ = m.Files.Delete(ctx, key) // best effort: the data is gone either way
	}
	return nil
}
