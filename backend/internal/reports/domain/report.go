// Package domain defines reports: what can be requested and the neutral table form every format renders from.
package domain

import (
	"context"
	"errors"
	"time"
)

type Kind string

const (
	KindCarbon Kind = "carbon"
	KindSCI    Kind = "sci"
	KindFinops Kind = "finops"
)

type Format string

const (
	CSV  Format = "csv"
	JSON Format = "json"
	PDF  Format = "pdf"
)

type Status string

const (
	Pending Status = "pending"
	Ready   Status = "ready"
	Failed  Status = "failed"
)

var (
	ErrInvalidReport = errors.New("invalid report request")
	ErrNotFound      = errors.New("not found")
	ErrNotReady      = errors.New("report is not ready")
)

// Report is the metadata row; the file itself lives in object storage.
type Report struct {
	ID          string     `json:"id"`
	ProjectID   *string    `json:"project_id"`
	Kind        Kind       `json:"kind"`
	Format      Format     `json:"format"`
	PeriodStart time.Time  `json:"period_start"`
	PeriodEnd   time.Time  `json:"period_end"` // inclusive
	Status      Status     `json:"status"`
	ObjectKey   *string    `json:"-"`
	Error       *string    `json:"error"`
	RequestedBy string     `json:"requested_by"`
	CreatedAt   time.Time  `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at"`
}

// Validate rejects unknown kinds/formats and periods that are empty, inverted or longer than 400 days.
func (r Report) Validate() error {
	switch r.Kind {
	case KindCarbon, KindSCI, KindFinops:
	default:
		return ErrInvalidReport
	}
	switch r.Format {
	case CSV, JSON, PDF:
	default:
		return ErrInvalidReport
	}
	if r.PeriodEnd.Before(r.PeriodStart) || r.PeriodEnd.Sub(r.PeriodStart) > 400*24*time.Hour {
		return ErrInvalidReport
	}
	return nil
}

// Ext and ContentType describe the rendered file.
func (f Format) Ext() string { return string(f) }

func (f Format) ContentType() string {
	switch f {
	case CSV:
		return "text/csv; charset=utf-8"
	case JSON:
		return "application/json"
	default:
		return "application/pdf"
	}
}

// Table is the neutral, format-independent content of a report.
type Table struct {
	Title   string     `json:"title"`
	Period  string     `json:"period"`
	Notes   []string   `json:"notes"`
	Headers []string   `json:"headers"`
	Rows    [][]string `json:"rows"`
}

// Repository is tenant-scoped (explicit tenantID + RLS). Dataset builders are read-only views of other schemas.
type Repository interface {
	Insert(ctx context.Context, tenantID string, r Report) (Report, error)
	List(ctx context.Context, tenantID string) ([]Report, error)
	Get(ctx context.Context, tenantID, id string) (Report, error)
	SetReady(ctx context.Context, tenantID, id, objectKey string) error
	SetFailed(ctx context.Context, tenantID, id, message string) error
	CarbonTable(ctx context.Context, tenantID string, project *string, from, to time.Time) (Table, error)
	FinopsTable(ctx context.Context, tenantID string, project *string, from, to time.Time) (Table, error)
	SCITable(ctx context.Context, tenantID string, project *string, from, to time.Time) (Table, error)
}
