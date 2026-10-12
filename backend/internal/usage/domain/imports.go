package domain

import (
	"context"
	"errors"
	"time"
)

// Import is one uploaded vendor cost file and what became of it.
type Import struct {
	ID           string     `json:"id"`
	ProjectID    string     `json:"project_id"`
	Vendor       string     `json:"vendor"`
	Filename     string     `json:"filename"`
	Currency     string     `json:"currency"`
	Status       string     `json:"status"` // processing | done | failed | deleting
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

// ImportResult is what the worker records when the file has been read.
type ImportResult struct {
	RowsRead, RowsImported, RowsSkipped, Services int
	From, To                                      time.Time
	Currency                                      string
	TotalCost                                     float64
	Warnings                                      []string
}

var (
	ErrImportNotFound = errors.New("import not found")
	// ErrImportProject: the project does not exist in this organization.
	ErrImportProject = errors.New("unknown project")
)

// ImportRepository is tenant-scoped.
type ImportRepository interface {
	CreateImport(ctx context.Context, tenantID string, i Import) (Import, error)
	GetImport(ctx context.Context, tenantID, id string) (Import, error)
	ListImports(ctx context.Context, tenantID string, limit int) ([]Import, error)
	// FinishImport records the outcome of reading a file (status done, or failed with a message when result is nil).
	FinishImport(ctx context.Context, tenantID, id string, res *ImportResult, failure string) error
	// MarkImportDeleting says the file is being taken back (the screen shows it; a second delete is refused).
	MarkImportDeleting(ctx context.Context, tenantID, id string) error
	// DeleteImport removes the lines the file made, its tags and its record.
	DeleteImport(ctx context.Context, tenantID, id string) error
}
